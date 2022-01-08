// Tournament Systems
//
// Copyright (c) 2022  Philip Kaludercic
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
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type System func(*Tournament) []*Game

var roundRobin System = func(t *Tournament) (games []*Game) {
	// Check of the tournament is over
	if int(t.round) >= len(t.participants) {
		return nil
	}

	// Calculate the size of the board/number of stones for this
	// round of the tournament
	r := (float64(t.round) / float64(len(t.participants)))
	size := conf.Game.Sizes[int(float64(len(conf.Game.Sizes))*r)]
	stones := conf.Game.Stones[int(float64(len(conf.Game.Stones))*r)]

	// Collect all games for the current round, using the circle
	// method:
	// https://en.wikipedia.org/wiki/Round-robin_tournament#Circle_method
	circle := make([]*Client, len(t.participants))
	copy(circle, t.participants)

	for i := 1; i < len(t.participants); i++ {
		// Starting from the current position...
		j := i
		// The circle method rotates the 2nd to last
		// participant by one place for each round.  This
		// calculates the assignments directly for the nth
		// round:
		j += int(t.round) - 1
		j %= len(t.participants) - 1

		circle[i] = t.participants[1+j]
	}

	n := len(circle)
	if n%2 == 1 {
		// Ensure n is even
		n--
	}
	for i := 0; i < len(circle)/2; i++ {
		games = append(games, &Game{
			Board: makeBoard(size, stones),
			North: circle[i],
			South: circle[n-i-1],
		})
	}

	return
}

type Tournament struct {
	sync.Mutex
	// What tournament system is being used (swiss, round-robin,
	// single-elimination, ...).
	system System
	// How many games are we waiting to finish.
	waiting int64
	// What are the clients we are expecting to participate in
	// this tournament.
	participants []*Client
	// Record of victories, mapping a winner to a list of looses.
	record map[*Client][]*Client
	// What round of the tournament is being played (1-indexed)
	round uint
}

func launch(dir string, c chan<- *Client) {
	debug.Println("Launching", dir)

	// Start a new TCP listener for this client
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}

	// Extract port number the operating system bound the listener
	// to, since port 0 is redirected to a "random" open port
	addr := ln.Addr().String()
	i := strings.LastIndexByte(addr, ':')
	if i == -1 && i+1 == len(addr) {
		log.Fatal("Invalid address", addr)
	}
	port := addr[i+1:]

	// Create a client and wait for an incoming connection
	var run *exec.Cmd
	cli := &Client{
		notify: c,
		token:  []byte(dir),
		tourn:  true,
	}
	go func() {
		var (
			dir = path.Base(dir)
			err error
		)

		// Wait for the client to connect
		cli.rwc, err = ln.Accept()
		if err != nil {
			log.Print("Failed to connect to", dir)
			c <- nil
			return
		}
		debug.Println("Connected to", dir)

		// Handle the connection
		cli.Handle()

		// Attempt to kill the process, if a process exists
		if run != nil {
			err = run.Process.Kill()
			if err != nil {
				log.Println(err)
			}
		}
	}()

	// Initialise and prepare a command for the client depending
	// on the requested isolation mechanism.
	switch conf.Tourn.Isolation {
	case "none": // In a regular process, without any isolation
		build := exec.Command("./build.sh")
		build.Dir = dir
		err = build.Run()
		if err != nil && !os.IsNotExist(err) {
			log.Print("Failed to build", dir)
			c <- nil
			return
		}

		run = exec.Command("./run.sh", port)
		run.Dir = dir
	case "guix": // In a Guix shell
		run = exec.Command("guix", "shell",
			"--container", "--no-cwd",
			// FIXME: map localhost:2761 in the container
			// to the port of the client's listener
			"--network",
			fmt.Sprintf("--expose=%s=/kalah", dir),
			"--", "/kalah/run.sh", port)
	default:
		log.Fatal("Unknown isolation system", conf.Tourn.Isolation)
	}

	var file *os.File
	file, err = os.Create(dir + ".stdout")
	if err != nil {
		log.Printf("Failed to redirect stdout for %s: %s",
			dir, err)
		run.Stdout = io.Discard
	} else {
		run.Stdout = file
		defer file.Close()
	}
	file, err = os.Create(dir + ".stderr")
	if err != nil {
		log.Printf("Failed to redirect stderr for %s: %s",
			dir, err)
		run.Stderr = io.Discard
	} else {
		run.Stderr = file
		defer file.Close()
	}

	err = run.Run()
	if err != nil {
		log.Printf("Failed to start %v: %s", dir, err)
	}
	if cli.kill != nil {
		// cli.kill is set in Client.Handle, that might not
		// have been called if the connection failed and the
		// processed aborted.
		cli.kill()
	}
}

func makeTournament(sys System) Sched {
	t := &Tournament{
		record: make(map[*Client][]*Client),
		system: sys,
	}

	dir, err := os.ReadDir(conf.Tourn.Directory)
	if err != nil {
		log.Fatal(err)
	}

	var (
		clich   = make(chan *Client, len(dir))
		c, s, i uint
	)

	for _, ent := range dir {
		if !ent.IsDir() {
			continue
		}

		// Attempt to launch the client in ent.
		go launch(ent.Name(), clich)
		c++
	}

	wait := make(chan struct{})
	go func() {
		// Await every response from the client channel.  The channel
		// cannot be closed, because we may still be waiting for a
		// response from a client.
		for cli := range clich {
			// We will receive a non-nil client, if the client
			// failed to initialise, in which case we add nothing
			// to the participant list.  A client is successfully
			// initialised, as soon as it requests to play a game.
			if cli != nil {
				t.participants = append(t.participants, cli)
				s++
			}

			// Check if we have received a response for every
			// launch invocation.
			i++
			if i == c {
				break
			}
		}

		wait <- struct{}{}
	}()

	select {
	case <-wait:
		log.Printf("Tournament successfully initialised (%d/%d)", s, c)
	case <-time.After(time.Duration(conf.Tourn.Warmup) * time.Second):
		log.Printf("Tournament warm-up time exceeded (%d/%d/%d)", s, i, c)
	}

	return t.Match
}

func (t *Tournament) Match(queue []*Client) []*Client {
	debug.Printf("Ongoing games %d, waiting clients %d", t.waiting, len(queue))
	if queue == nil {
		shutdown()
	}
	if t.waiting != 0 {
		return queue
	}

	clients := make(map[*Client]struct{})
	for _, c := range queue {
		clients[c] = struct{}{}
	}

	for _, c := range t.participants {
		_, ok := clients[c]
		if !ok {
			// If a client still hasn't connected, we will
			// not start a match
			return queue
		}
	}

	t.round++
	games := t.system(t)
	if games == nil {
		log.Print("Tournament has finished")
		shutdown()
		return nil
	} else {
		log.Print("Starting round ", t.round)
	}

	t.waiting = int64(len(games))
	for _, game := range games {
		go func(game *Game) {
			var wait sync.WaitGroup
			wait.Add(2)
			dbact <- game.South.updateDatabase(&wait, true)
			dbact <- game.North.updateDatabase(&wait, true)
			wait.Wait()

			// Create a second game with reversed positions
			size := uint(len(game.Board.northPits))
			emag := &Game{
				Board: makeBoard(size, game.Board.init),
				North: game.South,
				South: game.North,
			}

			game.Start()
			emag.Start()

			defer t.Unlock()
			t.Lock()
			switch game.Outcome {
			case WIN:
				if game.Outcome != emag.Outcome {
					log.Printf("%s was undecided %s", game.South, game.North)
					break
				}
				log.Printf("%s won against %s", game.South, game.North)
				t.record[game.South] = append(t.record[game.South], game.North)
				if err := game.updateScore(); err != nil {
					log.Println(err)
				}
			case LOSS:
				if game.Outcome != emag.Outcome {
					log.Printf("%s was undecided %s", game.North, game.South)
					break
				}
				log.Printf("%s won against %s", game.North, game.South)
				t.record[game.North] = append(t.record[game.North], game.South)
				if err := game.updateScore(); err != nil {
					log.Println(err)
				}
			case DRAW:
				log.Printf("%s played a draw against %s", game.South, game.North)
				t.record[game.South] = append(t.record[game.South], game.North)
				t.record[game.North] = append(t.record[game.North], game.South)
				if err := game.updateScore(); err != nil {
					log.Println(err)
				}
				if err := emag.updateScore(); err != nil {
					log.Println(err)
				}
			}

			atomic.AddInt64(&t.waiting, -1)

			enqueue <- game.North
			enqueue <- game.South
		}(game)
	}

	return nil
}
