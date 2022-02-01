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

type CompositeSched struct {
	s []Sched
}

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
	if cs.s == nil {
		panic("Composite Scheduler is empty")
	}
	cs.s[0].Add(cli)
}

// Notify the scheduler a client has died
func (cs *CompositeSched) Remove(cli *Client) {
	if cs.s == nil {
		panic("Composite Scheduler is empty")
	}
	cs.s[0].Remove(cli)
}

// Indicate if the scheduler will not schedule any more games
func (cs *CompositeSched) Done() bool {
	if cs.s == nil {
		panic("Composite Scheduler is empty")
	}

	done := cs.s[0].Done()
	if done {
		if len(cs.s) == 1 {
			return true
		}

		var cs0 Sched
		cs0, cs.s = cs.s[0], cs.s[1:]
		if prev, ok := cs0.(*Tournament); ok {
			prev.system.Deinit(prev)
			if curr, ok := cs.s[0].(*Tournament); prev != nil && ok {
				curr.participants = prev.participants
			}
		}

		err := cs.s[0].Init()
		if err != nil {
			log.Fatal(err)
		}

		if curr, ok := cs.s[0].(*Tournament); ok {
			for _, cli := range curr.participants {
				curr.Add(cli)
			}
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
				var size, stones uint = 7, 7

				g1 := &Game{
					Board: makeBoard(size, stones),
					North: north,
					South: south,
				}
				if died := g1.Play(); died != nil {
					other := g1.Other(died)
					enqueue <- other
				}

				g2 := &Game{
					Board: makeBoard(size, stones),
					North: south,
					South: north,
				}
				if died := g2.Play(); died != nil {
					other := g1.Other(died)
					enqueue <- other
				}

				o1 := g1.Outcome
				o2 := g2.Outcome
				if o1 != o2 || o1 == DRAW {
					if err := g1.updateElo(); err != nil {
						log.Print(err)
					}
				}

				enqueue <- north
				enqueue <- south
			}()
			break
		}
	}

	return queue
}

func noop(queue []*Client) []*Client {
	for _, cli := range queue {
		cli.Kill()
	}
	return nil
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
