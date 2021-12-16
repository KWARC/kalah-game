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
	"math/rand"
)

var (
	enqueue = make(chan *Client) // append a client to the queue
	forget  = make(chan *Client) // remove a client from the queue
)

// Attempt to match clients for new games
func (gc *GameConf) match(queue []*Client) []*Client {
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

			go (&Game{
				Board: makeBoard(
					gc.Sizes[rand.Intn(len(gc.Sizes))],
					gc.Stones[rand.Intn(len(gc.Stones))]),
				North: north,
				South: south,
			}).Start()
			break
		}
	}

	return queue
}

// TODO (philip, 27Nov21): The operations ENQUEUE (append), PROMOTE
// (prepend) and FORGET (delete) should ensure that there is always at
// most one client in QUEUE.  If should therefore be possible to
// simplify the algorithm below that accounts for possible duplicates.

func (gc *GameConf) remove(cli *Client, queue []*Client) []*Client {
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
func (gc *GameConf) init() {
	debug.Print("Starting queue manager")

	var queue []*Client

	for {
		select {
		case cli := <-enqueue:
			queue = append(queue, cli)
		case cli := <-forget:
			queue = gc.remove(cli, queue)
		}

		if len(queue) >= 2 {
			queue = gc.match(queue)
		}
	}
}
