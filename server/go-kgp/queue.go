// Client Waiting Queue
//
// Copyright (c) 2021  Philip Kaludercic
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
)

var (
	enqueue = make(chan *Client) // append a client to the queue
	forget  = make(chan *Client) // remove a client from the queue

	playing, waiting int64
)

// Attempt to match clients for new games
func match(queue []*Client) []*Client {
	north := queue[0]
	for i, cli := range queue[1:] {
		i += 1
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
				g1.Start()

				g2 := &Game{
					Board: makeBoard(size, stones),
					North: south,
					South: north,
				}
				g2.Start()

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

	return queue
}

func remove(cli *Client, queue []*Client) []*Client {
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

// Try to organise matches
func queueManager() {
	var queue []*Client

	for {
		select {
		case cli := <-enqueue:
			queue = append(queue, cli)
		case cli := <-forget:
			queue = remove(cli, queue)
		}

		if len(queue) >= 2 {
			queue = match(queue)
		}
		waiting = int64(len(queue))
	}
}
