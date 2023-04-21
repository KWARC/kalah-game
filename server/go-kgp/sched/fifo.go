// Connection Queue Handling
//
// Copyright (c) 2021, 2022, 2023  Philip Kaludercic
//
// This file is part of go-kgp.
//
// go-kgp is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License,
// version 3, as published by the Free Software Foundation.
//
// go-kgp is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License, version 3, along with go-kgp. If not, see
// <http://www.gnu.org/licenses/>

package sched

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"go-kgp"
	"go-kgp/bot"
	"go-kgp/cmd"
	"go-kgp/game"
	"go-kgp/proto"
)

var interval = 20 * time.Second

// The intent is not to have a secure source of random values, but
// just to avoid a predictive shuffling of north/south positions.
func init() { rand.Seed(time.Now().UnixMicro()) }

type fifo struct {
	add  chan kgp.Agent
	rem  chan kgp.Agent
	shut chan struct{}
	wait sync.WaitGroup
	q    []kgp.Agent
}

func (f *fifo) Start(st *cmd.State, conf *cmd.Conf) {
	var (
		bots []kgp.Agent
		av   = conf.Game.Open.Bots
	)

	bots = append(bots, bot.MakeRandom())
	for d, accs := range map[uint][]float64{
		2:  {1},
		4:  {0.4, 1},
		6:  {0.4, 0.75, 0.9, 1},
		8:  {0.4, 0.75, 0.9, 1},
		10: {0.4, 0.75, 0.9},
		12: {0.4, 0.75},
	} {
		for _, a := range accs {
			bots = append(bots, bot.MakeMinMax(d, a))
		}
	}

	// The actual scheduler runs every 20 seconds, so that clients
	// have time to gather in the queue and play against one
	// another, instead of just immediately falling back to a bot.
	tick := make(chan struct{}, 1)
	go func() {
		// Start the scheduler at a the beginning of a full minute, to
		// make the behaviour more predictable.
		wait := time.Until(time.Now().Round(interval)) + interval
		kgp.Debug.Println("Waiting", wait)
		time.Sleep(wait)

		for {
			tick <- struct{}{}
			time.Sleep(interval)
		}
	}()
	for {
		select {
		case <-tick:
			if len(f.q) == 0 {
				continue
			}
			kgp.Debug.Println("Running scheduler")
		case <-f.shut:
			// Stop accepting new connections
			return
		case a := <-f.add:
			if !bot.IsBot(a) {
				kgp.Debug.Println("Adding", a, "to the queue")
				f.q = append(f.q, a)
			} else {
				av++
			}
			continue
		case a := <-f.rem:
			kgp.Debug.Println("Remove", a, "from", f.q)

			i := -1
			for j, b := range f.q {
				if b != a {
					i = j
					break
				}
			}
			if i != -1 {
				// Based on the "Delete" Slice Trick
				//
				// https://github.com/golang/go/wiki/SliceTricks#delete
				f.q = append(f.q[:i], f.q[i+1:]...)
			}

			continue
		}

		// Remove all dead agents from the queue
		var (
			alive = make(chan kgp.Agent)
			next  = time.Now().Add(interval)
		)
		for _, a := range f.q {
			go func(a kgp.Agent) {
				if a != nil && a.Alive() {
					alive <- a
				} else {
					kgp.Debug.Println("Agent", a, "found to be dead")
					alive <- nil
				}
			}(a)
		}
		i := len(f.q)
		f.q = f.q[:0]
		for ; i > 0; i-- {
			if a := <-alive; a != nil {
				f.q = append(f.q, a)
			}
		}
		time.Sleep(time.Until(next))

		kgp.Debug.Println("Alive agents:", f.q)

		var rest []kgp.Agent
		for len(f.q) > 0 {
			// Select two agents, or two agents and a bot if only
			// one agent is available.
			var north, south kgp.Agent
			switch len(f.q) {
			case 0:
				panic("Broken look invariant")
			case 1:
				if av == 0 {
					goto done
				}
				south = f.q[0]
				f.q = nil

				// Pick a random bot
				north = bots[rand.Intn(len(bots))]
				av--
			default: // len(q) ≥ 2
				south = f.q[0]
				north = f.q[1]

				// Prevent an agent from playing against
				// himself (note that this does not prevent
				// two separate agents with the same token to
				// challenge one another)
				if north == south {
					panic("Duplicate agents")
				}

				// In case two agents have the same token, we want to
				// prevent them playing against one another over and
				// over again.
				ntok := north.User().Token
				stok := south.User().Token
				if ntok != "" && ntok == stok {
					// We have to distinguish between the two cases,
					// where two non-trivial agents are "just"
					// neighbouring one another in the queue, and when
					// they are the only two members of the queue.
					if len(f.q) == 2 {
						// If they are the only two, we will set aside
						// until the next scheduling tick one and let
						// the other play against a bot (this is done
						// by keeping it in the queue, and just
						// restarting the scheduler sub-cycle).
						f.q = f.q[:1]
						rest = append(rest, north)
					} else {
						// If they are just neighbouring, then move
						// one to the end and keep the other at the
						// front, restarting the scheduler sub-cycle
						f.q = append(f.q, south)
						f.q = f.q[1:]
						// To avoid infinite regression, we will set
						// aside all other agents with the same token
						// for now.
						for i := 1; i < len(f.q); i++ {
							if f.q[i].User().Token == ntok {
								rest = append(rest, f.q[i])
								f.q[i] = f.q[len(f.q)-1]
								f.q = f.q[:len(f.q)-1]
							}
						}
					}
					continue
				} else {
					f.q = f.q[2:]
				}
			}
			kgp.Debug.Println("Selected", north, "and", south, "for a match")

			// Ensure that we actually have two agents
			if north == nil || south == nil {
				panic("Illegal state")
			}

			// Start a game, but shuffle the order to avoid an
			// advantage for bots or non-bots.
			if rand.Intn(2) == 0 {
				north, south = south, north
			}

			f.wait.Add(1)
			go func(north, south kgp.Agent) {
				err := game.Play(&kgp.Game{
					Board: kgp.MakeBoard(
						conf.Game.Open.Size,
						conf.Game.Open.Init),
					South: north,
					North: south,
				}, st, conf)
				if err != nil {
					log.Print(err)
				}

				f.Schedule(south)
				f.Schedule(north)
				f.wait.Done()
			}(north, south)
		}

	done:
		if len(f.q) != 0 {
			panic("Queue is not empty")
		}
		f.q = rest
	}
}

func (f *fifo) Shutdown() {
	log.Println("Waiting for ongoing games to finish.")
	f.shut <- struct{}{}
	f.wait.Wait()
	for _, a := range f.q {
		if c, ok := a.(*proto.Client); ok {
			c.Kill()
		}
	}
}

func (f *fifo) Schedule(a kgp.Agent)   { f.add <- a }
func (f *fifo) Unschedule(a kgp.Agent) { f.rem <- a }
func (*fifo) String() string           { return "FIFO Scheduler" }

func MakeFIFO() cmd.Scheduler {
	return cmd.Scheduler(&fifo{
		add:  make(chan kgp.Agent, 16),
		rem:  make(chan kgp.Agent, 16),
		shut: make(chan struct{}),
	})
}
