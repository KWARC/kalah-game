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
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	enqueue = make(chan *Client) // append a client to the queue
	forget  = make(chan *Client) // remove a client from the queue

	playing, waiting uint64
)

// A Scheduler updates the waiting queue and manages games
type Sched func([]*Client) ([]*Client, bool)

// Discard a tournament
func noop(queue []*Client) ([]*Client, bool) {
	for _, cli := range queue {
		cli.kill()
	}
	return nil, false
}

// Compose multiple scheduling systems into one
func compose(s ...Sched) Sched {
	return func(queue []*Client) ([]*Client, bool) {
		for {
			if len(s) == 0 {
				return nil, true
			}

			step, over := s[0](queue)
			if !over {
				return step, false
			}
			s = s[1:]
		}
	}
}

// Limit the number of times a scheduler can be used
func limit(s Sched, n uint) Sched {
	return func(queue []*Client) ([]*Client, bool) {
		if n == 0 {
			return nil, true
		}
		n--

		return s(queue)
	}
}

// Fun FN on every client in the queue and update the database
func foreach(fn func(*Client)) Sched {
	return func(queue []*Client) ([]*Client, bool) {
		var wg sync.WaitGroup

		for _, cli := range queue {
			wg.Add(1)
			fn(cli)
			dbact <- cli.updateDatabase(&wg, false)
		}

		wg.Done()
		return queue, true
	}
}

// Reset the score of every client to SCORE
func reset(score float64) Sched {
	return foreach(func(cli *Client) {
		cli.Score = score
	})
}

// Split clients into two different schedulers via PRED
//
// If PRED returns true for a client, add it to SA, else SB.
func partition(sa Sched, sb Sched, pred func(*Client) bool) Sched {
	return func(queue []*Client) ([]*Client, bool) {
		aqueue := make([]*Client, 0, len(queue))
		bqueue := make([]*Client, 0, len(queue))

		for _, cli := range queue {
			if pred(cli) {
				aqueue = append(aqueue, cli)
			} else {
				bqueue = append(bqueue, cli)
			}
		}

		ra, aover := sa(aqueue)
		rb, bover := sb(bqueue)
		return append(ra, rb...), aover && bover
	}
}

func filter(s Sched, pred func(*Client) bool) Sched {
	return partition(s, noop, pred)
}

// Modify a scheduler to drop everyone with a score below SCORE
func bound(s Sched, score float64) Sched {
	return filter(s, func(c *Client) bool {
		return c.Score >= score
	})
}

var random Sched = func(queue []*Client) ([]*Client, bool) {
	for _, cli := range queue {
		go func(cli *Client) {
			size := conf.Game.Sizes[rand.Intn(len(conf.Game.Sizes))]
			stones := conf.Game.Stones[rand.Intn(len(conf.Game.Stones))]

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
	return nil, false
}

// The FIFO scheduler minimises the time a client remains in the
// queue, at the expense of the quality of a pairing.
var fifo Sched = func(queue []*Client) ([]*Client, bool) {
	north := queue[0]
	for i, cli := range queue[1:] {
		i += 1
		if cli.game != nil || cli.rwc == nil {
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
				size := conf.Game.Sizes[rand.Intn(len(conf.Game.Sizes))]
				stones := conf.Game.Stones[rand.Intn(len(conf.Game.Stones))]

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
					if err := g1.updateScore(); err != nil {
						log.Print(err)
					}
					if err := g2.updateScore(); err != nil {
						log.Print(err)
					}
				}

				if conf.Endless {
					// In the "endless" mode, the client is just
					// added back to the waiting queue as soon as
					// the game is over.
					enqueue <- north
					enqueue <- south
				} else {
					north.kill()
					south.kill()
				}
			}()
			break
		}
	}

	return queue, false
}

func remove(queue []*Client, cli *Client) []*Client {
	// Traverse the queue and replace any reference to CLI with a nil
	// pointer.
	found := false
	for i := range queue {
		if queue[i] == nil {
			panic(fmt.Sprintf("Nil reference found in queue (%d): %v", i, queue))
		}
		if queue[i] == cli {
			queue[i] = nil
			found = true
		}
	}

	// In case CLI was not in the queue, we return it unmodified
	if !found {
		return queue
	}

	// To avoid copying the contents of the entire queue, we will just
	// pull clients from the back of the queue to the beginning.  The
	// queue therefore is does not preserve a real "real" FIFO
	// ordering, but this is not necessary and might be argued to
	// introduce some dynamism into the system.

	// We traverse the queue from the beginning to the end, pulling
	// clients from the end of the queue to current position (i) to
	// fill empty spots.
	for i := 0; i < len(queue); i++ {

		// Before checking if the entry at the current position is
		// nil, we want to ensure that the last element in the queue
		// is not nil.
		for len(queue) > i && queue[len(queue)-1] == nil {
			queue = queue[:len(queue)-1]
		}

		// In case we have removed all elements, we can stop immediately
		if len(queue) == 0 {
			break
		}

		// If the current element is empty, we have either reached the
		// end of the queue when LEN == I, or there is an element at
		// the end of the queue (QUEUE[LEN-1]) that can be pulled
		// ahead.
		if queue[i] == nil {
			if len(queue) == i {
				queue = queue[:len(queue)-1]
				break
			} else {
				queue[i] = queue[len(queue)-1]
				queue = queue[:len(queue)-1]
			}
		}
	}

	return queue
}

// Using a scheduler, handle incoming events (requests to add and
// remove clients from the queue), to start games.
func schedule(sched Sched) {
	var queue []*Client

	for {
		select {
		case cli := <-enqueue:
			pause(cli)
			vacant := true
			for _, c := range queue {
				if cli == c {
					vacant = false
					break
				}
			}
			if vacant {
				queue = append(queue, cli)
			}
		case cli := <-forget:
			queue = remove(queue, cli)
		}

		// Attempt to organise a match
		var over bool
		queue, over = sched(queue)
		if over {
			shutdown()
			return
		}

		// Update statistics
		atomic.StoreUint64(&waiting, uint64(len(queue)))
	}
}

// Parse a scheduler specification into a scheduler
//
// Each specification is a list of tokens operating on a scheduler
// stack.
//
// The simplest case is a single word denoting a primitive scheduler:
//
//    "random"
//    "round-robin"
//
// A more complicated example might combine schedulers.  For example
//
//    "round-robin random compose"
//
// will add a round-robin scheduler to the stack, followed by a random
// scheduler, then combine these into a sequential scheduler that
// first executes the random scheduler, then the random scheduler.
//
// Some tokens may use arguments, separated by periods, to modify
// their behaviour (here using the abbreviated syntax):
//
//    "rr !.1000 $ |.1 > >"
//
// Would start a random scheduler, eliminate all clients that lost a
// random match (ie. have a score of 0, as opposed to 1), proceed to
// reset the score for all remaining clients and finally start a
// round-robin tournament.
func parseSched(spec string) Sched {
	var st []Sched

	for i, word := range strings.Split(spec, " ") {
		parse := strings.Split(word, ".")
		switch parse[0] {
		// Scheduler primitives
		case "fifo":
			st = append(st, fifo)
		case "rand", "random", "$":
			st = append(st, random)
		case "noop":
			st = append(st, noop)
		case "rr", "round-robin":
			st = append(st, makeTournament(&roundRobin{}))

			// Scheduler combinators
		case "seq", "compose", "+", ">":
			if len(st) < 2 {
				log.Fatal("Stack underflow at", i)
			}
			a := st[len(st)-1]
			b := st[len(st)-2]
			st = append(st[:len(st)-2], compose(a, b))
		case "limit", "=":
			n, err := strconv.Atoi(parse[1])
			if err != nil || n < 0 {
				log.Fatal("Invalid limit", parse[1])
			}
			s := limit(st[len(st)-1], uint(n))
			st = append(st[:len(st)-1], s)
		case "bound", "filter", "|":
			score, err := strconv.ParseFloat(parse[1], 64)
			if err != nil || score < 0 {
				log.Fatal("Invalid score", parse[1])
			}
			st = append(st[:len(st)-1], bound(st[len(st)-1], score))
		case "reset", "!":
			score, err := strconv.ParseFloat(parse[1], 64)
			if err != nil || score < 0 {
				log.Fatal("Invalid score", parse[1])
			}
			st = append(st[:len(st)-1], reset(score))
		default:
			log.Fatal("Unknown word", word)
		}
	}

	if len(st) == 0 {
		log.Fatal("Failed to parse scheduler spec")
	}
	return st[len(st)-1] // Pop top-of-stack
}
