package main

import (
	"math/rand"
	"os"
	"os/signal"
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
			return
		}
	}
}

// Try to organise matches
func organizer() {
	go func() {
		intr := make(chan os.Signal)
		signal.Notify(intr, os.Interrupt)
		<-intr
		os.Exit(1)
	}()

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
