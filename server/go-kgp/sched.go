// Connection Queue Handling
//
// Copyright (c) 2021, 2022  Philip Kaludercic
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

package main

import (
	"bytes"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

var (
	// Channel to append a client to the queue
	enqueue = make(chan *Client)
	// Channel to remove client from the queue
	forget = make(chan *Client)

	// Number of clients playing and waiting to play
	playing, waiting uint64
)

// A Scheduler updates the waiting queue and manages games
type Sched interface {
	// Initialise the scheduler
	Init() error
	// Notify the scheduler of a new client
	Add(*Client)
	// Notify the scheduler a client has died
	Remove(*Client)
	// Indicate if the scheduler will not schedule any more games
	Done() bool
}

type CompositeSched struct{ s []Sched }

func (cs *CompositeSched) Init() error {
	for _, s := range cs.s {
		if _, ok := s.(*QueueSched); ok {
			return errors.New("queue schedulers cannot be composed")
		}
	}
	return cs.s[0].Init()
}

// Notify the scheduler of a new client
func (cs *CompositeSched) Add(cli *Client) {
	if cs.s != nil {
		cs.s[0].Add(cli)
	}
}

// Notify the scheduler a client has died
func (cs *CompositeSched) Remove(cli *Client) {
	if cs.s != nil {
		cs.s[0].Remove(cli)
	}
}

// Indicate if the scheduler will not schedule any more games
func (cs *CompositeSched) Done() bool {
	if cs.s == nil {
		return true
	}

	if cs.s[0].Done() {
		if len(cs.s) == 1 {
			return true
		}

		if this, ok := cs.s[0].(*Tournament); ok && len(cs.s) >= 1 {
			if next, ok := cs.s[1].(*Tournament); ok {
				next.participants = this.participants
			}
		}

		cs.s = cs.s[1:]
		err := cs.s[0].Init()
		if err != nil {
			log.Fatal(err)
		}
		return cs.s[0].Done()
	}
	return false
}

type QueueSched struct {
	init  func()
	impl  func([]*Client) []*Client
	queue []*Client
}

// A queue scheduler might have a special initialisation, but must
// have a logic function
func (qs *QueueSched) Init() error {
	if qs.init != nil {
		qs.init()
	}
	if qs.impl == nil {
		return errors.New("queue scheduler without an implementation")
	}
	return nil
}

// Add a client to the queue, unless it is already waiting
func (qs *QueueSched) Add(cli *Client) {
	for _, c := range qs.queue {
		if c == cli {
			return
		}
	}
	qs.queue = append(qs.queue, cli)

	qs.queue = qs.impl(qs.queue)
}

// Remove all clients from the queue
func (qs *QueueSched) Remove(cli *Client) {
	for i := 0; i < len(qs.queue); i++ {
		if qs.queue[i] == cli {
			qs.queue[i] = qs.queue[len(qs.queue)-1]
			qs.queue[len(qs.queue)-1] = nil
			qs.queue = qs.queue[:len(qs.queue)-1]
		}
	}
}

// A Queue Scheduler never ends
func (qs *QueueSched) Done() bool {
	return false
}

func (QueueSched) Chain(Sched) {
	log.Fatal("Queue schedulers cannot be chained")
}

// The FIFO scheduler minimises the time a client remains in the
// queue, at the expense of the quality of a pairing.
func fifo(queue []*Client) []*Client {
	if len(queue) < 2 {
		return queue
	}

	north := queue[0]
	for i, cli := range queue[1:] {
		i += 1
		if !cli.Occupied() && cli.rwc == nil {
			queue = append(queue[:i], queue[i+1:]...)
			continue
		}
		if !bytes.Equal(cli.token, north.token) || cli.token == nil {
			south := cli
			queue[i] = queue[len(queue)-1]
			queue = queue[1 : len(queue)-1]

			if rand.Intn(2) == 0 {
				south, north = north, south
			}

			go func() {
				size := conf.Schedulers.FIFO.Sizes[rand.Intn(len(conf.Schedulers.FIFO.Sizes))]
				stones := conf.Schedulers.FIFO.Stones[rand.Intn(len(conf.Schedulers.FIFO.Stones))]

				g1 := &Game{
					Board: makeBoard(size, stones),
					North: north,
					South: south,
				}
				if died := g1.Play(); died != nil {
					other := g1.Other(died)
					if conf.Schedulers.FIFO.Endless {
						enqueue <- south
					} else {
						other.Kill()
					}
				}

				g2 := &Game{
					Board: makeBoard(size, stones),
					North: south,
					South: north,
				}
				if died := g2.Play(); died != nil {
					other := g1.Other(died)
					if conf.Schedulers.FIFO.Endless {
						enqueue <- south
					} else {
						other.Kill()
					}
				}

				o1 := g1.Outcome
				o2 := g2.Outcome
				if o1 != o2 || o1 == DRAW {
					if err := g1.updateElo(); err != nil {
						log.Print(err)
					}
				}

				if conf.Schedulers.FIFO.Endless {
					// In the "endless" mode, the client is just
					// added back to the waiting queue as soon as
					// the game is over.
					enqueue <- north
					enqueue <- south
				} else {
					north.Kill()
					south.Kill()
				}
			}()
			break
		}
	}

	return queue
}

// Using a scheduler, handle incoming events (requests to add and
// remove clients from the queue), to start games.
func schedule(sched Sched) {
	err := sched.Init()
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case cli := <-enqueue:
			sched.Add(cli)
		case cli := <-forget:
			sched.Remove(cli)
		case <-time.Tick(time.Second):
			// noop
		}

		if sched.Done() {
			return
		}

		// Update statistics (if using a queue scheduler)
		if qs, ok := sched.(*QueueSched); ok {
			w := uint64(len(qs.queue))
			atomic.StoreUint64(&waiting, w)
		}

	}
}

// Parse a scheduler specification into a scheduler
//
// The scheduler specification is described in the manual.
func parseSched(specs []string) Sched {
	var scheds []Sched
	if specs == nil {
		log.Fatal("Empty scheduler specification")
	}

	for _, spec := range specs {
		var sched Sched

		parts := strings.SplitN(spec, " ", 2)
		switch parts[0] {
		case "fifo":
			sched = &QueueSched{
				init: func() {
					fc := conf.Schedulers.FIFO
					listen(fc.Port)
					if fc.WebSocket {
						http.HandleFunc("/socket", listenUpgrade)
						debug.Print("Handling websocket on /socket")
					}
				},
				impl: fifo,
			}
		case "rand", "random":
			sched = makeTournament(&random{})
		case "rr", "round-robin":
			var n, p uint64
			if err := parse(parts[1], &n, &p); err != nil {
				log.Fatal("Invalid arguments")
			}
			if p < 1 {
				log.Fatal("Round robin needs to at least pick one agent")
			}
			sched = makeTournament(&roundRobin{
				size: uint(n),
				pick: uint(p),
			})
		default:
			log.Fatal("Unknown scheduler ", parts[0])
		}

		scheds = append(scheds, sched)
	}

	if len(scheds) == 1 {
		return scheds[0]
	} else {
		return &CompositeSched{scheds}
	}
}
