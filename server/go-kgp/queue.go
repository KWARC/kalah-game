package main

import (
	"fmt"
	"math/rand"
)

var (
	enqueue = make(chan *Client) // append a client to the queue
	promote = make(chan *Client) // promote a client to the front of the queue
	forget  = make(chan *Client) // remove a client from the queue
)

// Attempt to match clients for new games
func match(queue []*Client) []*Client {
	north := queue[0]
	for i, cli := range queue[1:] {
		if cli.token != north.token || cli.token == "" {
			south := cli
			queue = append(queue[:i], queue[i+1:]...)
			queue = queue[1:]

			if rand.Intn(2) == 0 {
				south, north = north, south
			}

			go (&Game{
				Board: makeBoard(
					conf.Game.Sizes[rand.Intn(len(conf.Game.Sizes))],
					conf.Game.Stones[rand.Intn(len(conf.Game.Stones))]),
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
			if cli.game != nil {
				panic("Enqueuing client that is already playing")
			}
			queue = remove(cli, queue)
			queue = append(queue, cli)
		case cli := <-promote:
			if cli.game == nil {
				continue
			}
			queue = remove(cli, queue)
			queue = append([]*Client{cli}, queue...)
		case cli := <-forget:
			queue = remove(cli, queue)
		}

		if len(queue) >= 2 {
			queue = match(queue)
		}
	}
}
