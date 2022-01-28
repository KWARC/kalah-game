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

// Local client isolation abstraction
type Isolation interface {
	// Start a client and connect to port, and block until the
	// client terminates
	Start(port string) error
	// Kill the client and block until it dies
	Halt() error
}

// Helper function to halt a client
func (cli *Client) Halt() error {
	debug.Print("To halt ", cli)

	if cli == nil {
		return nil
	}

	defer cli.lock.Unlock()
	cli.lock.Lock()

	debug.Print("Unpausing ", cli)
	if cli.isol == nil {
		log.Panic("Client is not isolated")
	}

	err := cli.isol.Halt()
	if err != nil {
		log.Print(err)
	}
	// cli.rwc = nil
	return err
}

// Restart an isolated client
func (cli *Client) Restart() bool {
	debug.Print("Restarting ", cli)

	if cli == nil {
		return true
	}

	cli.lock.Lock()
	if cli.rwc != nil {
		cli.lock.Unlock()
		return true
	}

	debug.Print("Ensuring ", cli)
	if cli == nil || cli.isol == nil {
		log.Panic("Client is not isolated")
	}

	var (
		// response channel for successful connections
		c = make(chan *Client)
		// indicator channel for failed connection
		fail = make(chan string)
	)

	cli.Halt()
	cli.lock.Unlock()
	connect(cli, c, fail)
	select {
	case <-c:
		// everything is ok
		return true
	case <-fail:
		forget <- cli
		return false
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
	// Games to start
	start chan *Game
	// List of active games
	active map[*Game]struct{}
	// Is manually initialised
	manual bool
}

func connect(cli *Client, c chan<- *Client, fail chan<- string) {
	if cli.isol == nil {
		panic("No isolation method for client")
	}

	// Start a new TCP server with a random port for this client
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

	go func() {
		var (
			name = string(cli.token)
			dir  = path.Base(name)
			err  error
		)

		// Wait for the client to connect
		cli.lock.Lock()
		if cli.rwc != nil {
			cli.rwc.Close()
		}
		cli.rwc, err = ln.Accept()
		cli.lock.Unlock()
		if err != nil {
			log.Print("Failed to connect to ", dir)
			fail <- name
			return
		}
		debug.Print("Connected to ", dir)

		// Handle the connection
		cli.notify = c
		cli.Handle()
	}()

	// Initiate the client in the isolation
	go func() {
		err := cli.isol.Start(port)
		if err != nil {
			fail <- string(cli.token)
			log.Print(err)
		}
	}()

	cli.updateDatabase(true)
}

// Helper function to launch a client with NAME
//
// The function starts a separate server, creates an isolated client,
// and returns the client via the passed channel
func launch(name string, c chan<- *Client, fail chan<- string) {
	debug.Println("Launching ", name)

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

	// Create a client and connect to a new server
	connect(&Client{
		token: []byte(name),
		isol:  isol,
	}, c, fail)
}

// Convert a tournament system into a scheduler
func makeTournament(sys System) Sched {
	return &Tournament{
		active: make(map[*Game]struct{}),
		start:  make(chan *Game, 4),
		system: sys,
	}
}

// Start and manage games
func (t *Tournament) Manage() {
	id := registerTournament(t.system.String())

	for game := range t.start {
		t.Lock()
		t.active[game] = struct{}{}
		t.Unlock()

		debug.Print("To start ", game)
		go func(game *Game) {
			var died *Client

			// Create a second game with reversed positions
			size := uint(len(game.Board.northPits))
			emag := &Game{
				Board: makeBoard(size, game.Board.init),
				North: game.South,
				South: game.North,
			}

			if !game.South.Restart() {
				log.Println("Failed to restart", game.South)
				enqueue <- game.North
				return
			}
			if !game.North.Restart() {
				log.Println("Failed to restart", game.North)
				enqueue <- game.South
				return
			}
			log.Printf("Start %s vs. %s (%s)", game.South, game.North, game)
			died = game.Play()
			if died != nil {
				log.Printf("Cancelled %s vs. %s (%s)", game.South, game.North, game)
				enqueue <- emag.Other(died)
				return
			}

			if !game.South.Restart() {
				log.Println("Failed to restart", game.South)
				enqueue <- game.North
				return
			}
			if !game.North.Restart() {
				log.Println("Failed to restart", game.North)
				enqueue <- game.South
				return
			}
			log.Printf("Start %s vs. %s (%s, rev)", emag.South, emag.North, emag)
			died = emag.Play()
			if died != nil {
				log.Printf("Cancelled %s vs. %s (%s, rev)", emag.South, emag.North, emag)
				enqueue <- emag.Other(died)
				return
			}

			t.Lock()
			switch game.Outcome {
			case WIN:
				if LOSS != emag.Outcome {
					log.Printf("%s was undecided %s", game.South, game.North)
					break
				}
				log.Printf("%s won against %s", game.South, game.North)
				game.South.recordScore(game, id, conf.Game.Win)
				game.North.recordScore(game, id, conf.Game.Loss)
			case LOSS:
				if WIN != emag.Outcome {
					log.Printf("%s was undecided %s", game.North, game.South)
					break
				}
				log.Printf("%s won against %s", game.North, game.South)
				game.South.recordScore(game, id, conf.Game.Loss)
				game.North.recordScore(game, id, conf.Game.Win)
			case DRAW:
				log.Printf("%s played a draw against %s", game.South, game.North)
				game.South.recordScore(game, id, conf.Game.Draw)
				game.North.recordScore(game, id, conf.Game.Draw)
			}

			t.system.Record(t, game)
			delete(t.active, game)
			t.Unlock()

			enqueue <- game.North
			enqueue <- game.South
		}(game)
	}
}

func (t *Tournament) Init() error {
	if !t.manual {
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
			// response channel for successful connections
			c = make(chan *Client)
			// indicator channel for failed connection
			fail = make(chan string)
			// number of successful connections
			s uint
			// connections not yet established
			w = uint(len(names))
		)

		for _, name := range names {
			launch(name, c, fail)
		}

		wait := make(chan struct{})
		go func() {
			// Await every response from the client channel.  The channel
			// cannot be closed, because we may still be waiting for a
			// response from a client.
			for w > 0 {
				select {
				case cli := <-c:
					t.Lock()
					t.participants = append(t.participants, cli)
					t.Unlock()
					s++
				case name := <-fail:
					log.Print(name, " failed to connect")
				}
				w--
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
	}
	t.Lock()
	for _, cli := range t.participants {
		cli.Score = 0
	}
	t.Unlock()

	go t.Manage()
	return nil
}

func (t *Tournament) Add(cli *Client) {
	defer t.Unlock()
	t.Lock()

	t.system.Ready(t, cli)
}

func (t *Tournament) Remove(cli *Client) {
	defer t.Unlock()
	t.Lock()

	for i := 0; i < len(t.participants); i++ {
		if t.participants[i] == cli {
			t.participants[i] = t.participants[len(t.participants)-1]
			t.participants = t.participants[:len(t.participants)-1]
		}
	}
	for game := range t.active {
		if game.North == cli || game.South == cli {
			game.death <- cli
		}
	}
	t.system.Forget(t, cli)
}

func (t *Tournament) Done() bool {
	defer t.Unlock()
	t.Lock()

	return t.system.Over(t) && len(t.active) == 0
}
