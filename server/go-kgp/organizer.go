package main

import (
	"sync"
	"time"
)

var (
	qlock = sync.Mutex{}
	qcond = sync.NewCond(&qlock)
	queue []*Client
)

// Add CLI to the queue
func enqueue(cli *Client) {
	time.Sleep(10 * time.Millisecond)
	qlock.Lock()
	queue = append(queue, cli)
	if len(queue) >= 2 {
		qcond.Signal()
	}
	qlock.Unlock()
}

// Remove CLI from the queue
func forget(cli *Client) {
	qlock.Lock()
	for i, c := range queue {
		if c == cli {
			queue = append(queue[:i], queue[i+1:]...)
			break
		}
	}
	qlock.Unlock()
}

// Move CLI up the queue
func promote(cli *Client) {
	qlock.Lock()
	for i, c := range queue {
		if c == cli {
			queue = append(queue[:i], queue[i+1:]...)
			queue = append([]*Client{cli}, queue...)
			break
		}
	}
	qlock.Unlock()
}

// Attempt to match clients for new games
func match() {
	north := queue[0]
	for i, cli := range queue[1:] {
		if cli.token != north.token || cli.token == "" {
			south := cli
			queue = append(queue[:i], queue[i+1:]...)
			queue = queue[1:]

			go (&Game{
				Board: makeBoard(defSize, defStones),
				North: north,
				South: south,
			}).Start()
			return
		}
	}
}

// Try to organise matches
func organizer() {
	for {
		qlock.Lock()
		for {
			qcond.Wait()
			if len(queue) < 2 {
				continue
			}
			match()
		}
	}
}
