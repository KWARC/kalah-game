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
	"sync"
	"sync/atomic"
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
	// Append a scheduler to use after this one is done
	Chain(Sched)
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
		return errors.New("Queue Scheduler without an implementation")
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

// The random scheduler has everyone play two games against a random agent
func random(queue []*Client) []*Client {
	for _, cli := range queue {
		go func(cli *Client) {
			var (
				size   = conf.Schedulers.Random.Size
				stones = conf.Schedulers.Random.Stones
			)

			g1 := &Game{
				Board: makeBoard(size, stones),
				North: cli,
				South: nil,
			}
			g2 := &Game{
				Board: makeBoard(size, stones),
				North: cli,
				South: nil,
			}

			g1.Play()
			g2.Play()

			o1 := g1.Outcome
			o2 := g2.Outcome
			if o1 == LOSS && o2 == WIN {
				cli.Score = 1
			} else {
				cli.Score = 0
			}

			var wait sync.WaitGroup
			wait.Add(1)
			dbact <- cli.updateDatabase(&wait, false)
			wait.Wait()
		}(cli)
	}
	return nil
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
				g1.Play()

				g2 := &Game{
					Board: makeBoard(size, stones),
					North: south,
					South: north,
				}
				g2.Play()

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
	sched.Init()

	for {
		select {
		case cli := <-enqueue:
			pause(cli)
			sched.Add(cli)
		case cli := <-forget:
			sched.Remove(cli)
		}

		if sched.Done() {
			shutdown.Do(closeDB)
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
func parseSched(spec interface{}) Sched {
	var specs []string
	switch v := spec.(type) {
	case string:
		// For the sake of convenience, the scheduler can also
		// just be a single string.
		specs = []string{v}
	case []string:
		specs = v
	}

	var scheds []Sched
	for _, spec := range specs {
		var sched Sched

		parts := strings.SplitN(spec, " ", 1)
		fn := parts[0]
		switch fn {
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
		case "rr", "round-robin":
			var n uint64
			if err := parse(parts[1], &n); err != nil {
				log.Fatal("Invalid ")
			}
			sched = makeTournament(&roundRobin{size: uint(n)})
		default:
			log.Fatal("Unknown scheduler", fn)
		}

		scheds = append(scheds, sched)
	}

	for i := 1; i < len(scheds); i++ {
		scheds[i-1].Chain(scheds[i])
	}

	return scheds[0]
}
