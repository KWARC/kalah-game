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
	"log"
	"math/rand"
	"sort"
	"strconv"
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
type Sched func([]*Client) ([]*Client, bool)

// Compose multiple scheduling systems into one
func compose(s []Sched) Sched {
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

// Filter out all predicates that don't satisfy PRED
func filter(pred func(*Client) bool) Sched {
	return func(queue []*Client) (new []*Client, _ bool) {
		for _, cli := range queue {
			if pred(cli) {
				new = append(new, cli)
			}
		}
		return new, true
	}
}

// Modify a scheduler to drop everyone with a score below SCORE
func bound(score float64) Sched {
	return filter(func(c *Client) bool {
		return c.Score >= score
	})
}

// Filter out worse than the best COUNT clients
func skim(count int) Sched {
	return func(queue []*Client) ([]*Client, bool) {
		if len(queue) < count {
			return queue, true
		}

		sort.Slice(queue, func(i, j int) bool {
			return queue[i].Score > queue[j].Score
		})

		n := count
		for n < len(queue) && queue[count-1].Score == queue[n].Score {
			n++
		}
		return queue[:n], true
	}
}

// The random scheduler has everyone play two games against a random agent
var random Sched = func(queue []*Client) ([]*Client, bool) {
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
	return nil, false
}

// The FIFO scheduler minimises the time a client remains in the
// queue, at the expense of the quality of a pairing.
var fifo Sched = func(queue []*Client) ([]*Client, bool) {
	if len(queue) < 2 {
		return queue, false
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

	return queue, false
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
			for i, c := range queue {
				if cli == c {
					queue = append(queue[:i], queue[i+1:]...)
					break
				}
			}
		}

		// Attempt to organise a match
		var over bool
		queue, over = sched(queue)
		if over {
			shutdown.Do(closeDB)
			return
		}

		// Update statistics
		atomic.StoreUint64(&waiting, uint64(len(queue)))
	}
}

// Parse a scheduler specification into a scheduler
//
// The scheduler specification is described in the manual.
func parseSched(spec string) Sched {
	var scheds []Sched

	for _, word := range strings.Split(spec, " ") {
		parse := strings.Split(word, ".")
		switch parse[0] {
		case "fifo":
			scheds = append(scheds, fifo)
		case "rand", "random":
			scheds = append(scheds, random)
		case "rr", "round-robin":
			if len(parse) != 2 {
				log.Fatal("Round robin requires a board sizes")
			}
			n, err := strconv.Atoi(parse[1])
			if err != nil || n < 0 {
				log.Fatal("Invalid size", parse[1])
			}
			rr := makeTournament(&roundRobin{size: uint(n)})
			scheds = append(scheds, rr)
		case "bound":
			score, err := strconv.ParseFloat(parse[1], 64)
			if err != nil || score < 0 {
				log.Fatal("Invalid score", parse[1])
			}
			scheds = append(scheds, bound(score))
		case "skim":
			count, err := strconv.Atoi(parse[1])
			if err != nil || count <= 0 {
				log.Fatal("Invalid count", parse[1])
			}
			scheds = append(scheds, skim(count))
		case "reset", "!":
			if len(parse) != 2 {
				log.Fatal("reset requires an argument")
			}
			score, err := strconv.ParseFloat(parse[1], 64)
			if err != nil || score < 0 {
				log.Fatal("Invalid score", parse[1])
			}
			scheds = append(scheds, reset(score))
		default:
			log.Fatal("Unknown word", word)
		}
	}

	switch len(scheds) {
	case 0:
		log.Fatal("Failed to parse scheduler spec")
		return nil
	case 1:
		return scheds[0]
	default:
		for i := 0; i < len(scheds)/2; i++ {
			j := len(scheds) - i - 1
			scheds[i], scheds[j] = scheds[j], scheds[i]
		}
		return compose(scheds)
	}
}
