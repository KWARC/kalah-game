// Tournament Managment
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
	"log"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type Isolation interface {
	Run(port string) error
	Halt() error
	Unpause()
	Pause()
	Await()
}

// Helper function to pause an isolated client
func pause(cli *Client) {
	if cli != nil && cli.isol != nil {
		cli.isol.Pause()
	}
}

// Helper function to unpause an isolated client
func unpause(cli *Client) {
	if cli != nil && cli.isol != nil {
		cli.isol.Unpause()
	}
}

// A tournament is a scheduler that matches participants via a system
type Tournament struct {
	sync.Mutex
	// What tournament system is being used (swiss, round-robin,
	// single-elimination, ...).
	system System
	// What are the clients we are expecting to participate in
	// this tournament.
	participants []*Client
	// Record of victories, mapping a winner to a list of looses.
	record map[*Client][]*Client
	// Games to start
	games chan *Game
}

// Helper function to launch a client with NAME
//
// The function starts a separate server, creates an isolated client,
// and returns the client via the passed channel
func launch(name string, c chan<- *Client) {
	debug.Println("Launching ", name)

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
		log.Fatal("Invalid address ", addr)
	}
	port := addr[i+1:]

	// Initialise and prepare a command for the client depending
	// on the requested isolation mechanism.
	var isol Isolation
	switch conf.Tourn.Isolation {
	case "none": // In a regular process, without any isolation
		isol = &Process{dir: name}
	case "docker": // In a docker container
		isol = &Docker{name: name}
	default:
		log.Fatal("Unknown isolation system ", conf.Tourn.Isolation)
	}

	// Create a client and wait for an incoming connection
	cli := &Client{
		notify: c,
		token:  []byte(name),
		isol:   isol,
	}

	go func() {
		var (
			dir = path.Base(name)
			err error
		)

		// Wait for the client to connect
		cli.rwc, err = ln.Accept()
		if err != nil {
			log.Print("Failed to connect to ", dir)
			c <- nil
			return
		}
		debug.Println("Connected to ", dir)

		// Handle the connection
		cli.Handle()

		// As soon as the connection is terminated, kill the
		// isolated process as well.
		err = isol.Halt()
		if err != nil {
			log.Println(err)
		}
	}()

	var wait sync.WaitGroup
	wait.Add(1)
	dbact <- cli.updateDatabase(&wait, true)
	wait.Wait()

	err = isol.Run(port)
	if err != nil {
		log.Fatal(err)
	}
	cli.Kill()
}

// Convert a tournament system into a scheduler
func makeTournament(sys System) Sched {
	t := &Tournament{
		record: make(map[*Client][]*Client),
		system: sys,
	}

	names := conf.Tourn.Names
	if names == nil {
		dir, err := os.ReadDir(conf.Tourn.Directory)
		if err != nil {
			log.Fatal(err)
		}
		for _, ent := range dir {
			if !ent.IsDir() {
				continue
			}

			names = append(names, ent.Name())
		}
	}

	var (
		// response channel (nil for a failed and non-nil for
		// a successful connection)
		c = make(chan *Client)
		// number of successful connections
		s uint
		// connections not yet established
		w = uint(len(names))
	)

	for _, name := range names {
		go launch(name, c)
	}

	wait := make(chan struct{})
	go func() {
		// Await every response from the client channel.  The channel
		// cannot be closed, because we may still be waiting for a
		// response from a client.
		for cli := range c {
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
			w--
			if w == 0 {
				break
			}
		}

		close(wait)
	}()

	select {
	case <-wait:
		log.Printf("Tournament successfully initialised (%d/%d)",
			s, len(names))
	case <-time.After(time.Duration(conf.Tourn.Warmup) * time.Second):
		log.Printf("Tournament warm-up time exceeded (%d/%d/%d)",
			s, len(names)-int(w), len(names))
	}

	t.games = make(chan *Game)
	go t.Manage()
	return t.Schedule
}

// A scheduler adaptor for a tournament
func (t *Tournament) Manage() {
	c := make(chan int64)
	dbact <- registerTournament(t.system.String(), c)
	id := <-c

	for game := range t.games {
		go func(game *Game) {
			// Create a second game with reversed positions
			size := uint(len(game.Board.northPits))
			emag := &Game{
				Board: makeBoard(size, game.Board.init),
				North: game.South,
				South: game.North,
			}

			log.Printf("Start %s vs. %s (%s)", game.South, game.North, game)
			game.Play()
			log.Printf("Start %s vs. %s (%s, rev)", emag.South, emag.North, emag)
			emag.Play()

			t.Lock()
			switch game.Outcome {
			case WIN:
				if game.Outcome != emag.Outcome {
					log.Printf("%s was undecided %s", game.South, game.North)
					goto norecord
				}
				log.Printf("%s won against %s", game.South, game.North)
				t.record[game.South] = append(t.record[game.South], game.North)
				dbact <- game.South.recordScore(game, id, 1)
				dbact <- game.North.recordScore(game, id, -1)
			case LOSS:
				if game.Outcome != emag.Outcome {
					log.Printf("%s was undecided %s", game.North, game.South)
					goto norecord
				}
				log.Printf("%s won against %s", game.North, game.South)
				t.record[game.North] = append(t.record[game.North], game.South)
				dbact <- game.South.recordScore(game, id, -1)
				dbact <- game.North.recordScore(game, id, 1)
			case DRAW:
				log.Printf("%s played a draw against %s", game.South, game.North)
				t.record[game.South] = append(t.record[game.South], game.North)
				t.record[game.North] = append(t.record[game.North], game.South)
				dbact <- game.South.recordScore(game, id, 0)
				dbact <- game.North.recordScore(game, id, 0)
			}
			t.system.Record(t, game)

		norecord:
			t.Unlock()

			enqueue <- game.North
			enqueue <- game.South
		}(game)
	}
}

func (t *Tournament) Schedule(queue []*Client) ([]*Client, bool) {
	for _, cli := range queue {
		t.system.Ready(t, cli)
	}

	t.Lock()
	over := t.system.Over(t)
	t.Unlock()
	if over {
		for _, cli := range t.participants {
			err := cli.isol.Halt()
			if err != nil {
				log.Println(err)
			}
		}
	}
	return nil, over
}
