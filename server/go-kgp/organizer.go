package main

import "log"
import "sync"

var (
	qlock = sync.Mutex{}
	qcond = sync.NewCond(&qlock)
	queue []*Client
)

// Add CLI to the queue
func enqueue(cli *Client) {
	qlock.Lock()
	queue = append(queue, cli)
	qlock.Unlock()
	if len(queue) >= 2 {
		qcond.Signal()
	}
}

// Remove CLI from the queue
func unqueue(cli *Client) {
	qlock.Lock()
	for i, c := range queue {
		if c == cli {
			queue = append(queue[:i], queue[i+1:]...)
		}
	}
	qlock.Unlock()
}

// Move CLI up the queue
func boost(cli *Client) {
	qlock.Lock()
	for i, c := range queue {
		if c == cli {
			queue = append(queue[:i], queue[i+1:]...)
			queue = append([]*Client{cli}, queue...)
		}
	}
	qlock.Unlock()
}

// Wait for games to be organizable
func organizer() {
	for {
		qlock.Lock()
		for len(queue) < 2 {
			qcond.Wait()
		}
		go (&Game{
			board: makeBoard(defSize, defStones),
			north: queue[0],
			south: queue[1],
		}).Start()
		queue = queue[2:]
		qlock.Unlock()

		log.Printf("Start new game")
	}

}
