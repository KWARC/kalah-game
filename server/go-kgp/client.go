// Client Communication Management
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
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// An Agent is the core of a Client, as stored in the database
type Agent struct {
	Name   string
	Author string
	Descr  string
	Score  float64
	Games  int64
	Rank   int64
	Id     int64
}

// Client wraps a network connection into a player
type Client struct {
	Agent
	games   map[uint64]*Game
	rwc     io.ReadWriteCloser
	lock    sync.Mutex
	iolock  sync.Mutex
	rid     uint64
	killFn  context.CancelFunc
	pinged  uint32
	token   []byte
	comment string
	notify  chan<- *Client

	// Tournament isolation
	isol Isolation

	// Simple mode state management
	simple bool
	game   *Game
	nyield uint64
	nstop  uint64
}

// Kill will try to close the connection for a client
func (cli *Client) Kill() {
	debug.Println("To kill", cli)
	if cli != nil && cli.killFn != nil {
		cli.killFn()
	}
}

// String will return a string representation for a client for
// internal use
func (cli *Client) String() string {
	if cli == nil {
		return "RND"
	}

	if cli.isol != nil {
		return string(cli.token)
	}

	hash := base64.StdEncoding.EncodeToString(cli.token)
	if conn, ok := cli.rwc.(net.Conn); ok {
		return fmt.Sprintf("%s (%q)", conn.RemoteAddr(), hash)
	}
	if conn, ok := cli.rwc.(*wsrwc); ok {
		return fmt.Sprintf("%s (%q)", conn.RemoteAddr(), hash)
	}
	return fmt.Sprintf("%p (%q)", cli, hash)
}

// Send is a shorthand to respond without a reference
func (cli *Client) Send(command string, args ...interface{}) uint64 {
	return cli.Respond(0, command, args...)
}

// Error is a shorthand to respond with an error message
func (cli *Client) Error(to uint64, args ...interface{}) {
	cli.Respond(to, "error", args...)
}

// Respond forwards a referenced message to the client
//
// Each element in ARGS is handled as an argument to COMMAND, and will
// use the concrete datatype for formatting.  Respond does not check
// if the arguments have the right types for COMMAND.
//
// If TO is 0, no reference will be added.
func (cli *Client) Respond(to uint64, command string, args ...interface{}) uint64 {
	if cli == nil {
		return 0
	}

	var (
		buf = bytes.NewBuffer(nil)
		id  = atomic.AddUint64(&cli.rid, 2)
	)

	fmt.Fprint(buf, id)
	if to > 0 {
		fmt.Fprintf(buf, "@%d", to)
	}

	fmt.Fprintf(buf, " %s", command)

	for _, arg := range args {
		fmt.Fprint(buf, " ")
		switch arg.(type) {
		case string:
			fmt.Fprintf(buf, "%#v", arg)
		case int:
			fmt.Fprintf(buf, "%d", arg)
		case float64:
			fmt.Fprintf(buf, "%f", arg)
		case *Game:
			fmt.Fprintf(buf, "%s", arg)
		default:
			panic("Unsupported type")
		}
	}

	// attempt to send this message before any other message is sent
	defer cli.iolock.Unlock()
	cli.iolock.Lock()

	if cli.rwc == nil {
		return 0
	}

	debug.Print(cli, " > ", buf.String())
	fmt.Fprint(buf, "\r\n")

	i := conf.TCP.Retries // allow 8 unsuccesful retries
retry:
	n, err := io.Copy(cli.rwc, buf)
	if err != nil {
		_, isWS := cli.rwc.(*wsrwc)
		if isWS {
			return id
		}

		nerr, ok := err.(net.Error)
		if i > 0 && (!ok || (ok && nerr.Temporary())) {
			wait := time.Millisecond
			wait <<= (conf.TCP.Retries - i)
			wait *= 10
			time.Sleep(wait)
			if n > 0 {
				// discard first n bytes
				buf = bytes.NewBuffer(buf.Bytes()[n:])
			}
			i--
			goto retry
		} else {
			log.Print(cli, err)
			cli.Kill()
		}
	}

	return id
}

// Check if the client cannot play a game
func (cli *Client) Occupied() bool {
	// Only simple mode clients can be occupied, as they are
	// limited to only play one game at a time
	return cli.simple && cli.game != nil
}

// Remove all of CLIs references to GAME
func (cli *Client) Forget(game *Game) {
	defer cli.lock.Unlock()
	cli.lock.Lock()

	if cli.simple {
		if cli.game == game {
			cli.game = nil
		} else {
			panic("Forgetting wrong game")
		}
	} else {
		for id, g := range cli.games {
			if g == game {
				delete(cli.games, id)
			}
		}
	}
}

// Pinger regularly sends out a ping and checks if a pong was received.
func (cli *Client) Pinger(ctx context.Context) {
	if conf.TCP.Timeout == 0 {
		panic("TCP Timeout must be greater than 0")
	}
	ticker := time.NewTicker(time.Duration(conf.TCP.Timeout) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// If the timer fired, check the ping flag and
			// kill the client if it is still set
			if cli.rwc == nil {
				continue
			}
		}

		if cli.isol != nil {
			// If we are managing the client, and know
			// that it is asleep, we do not expect a ping.
			cli.isol.Await()
		}

		// To prevent race conditions, we atomically check and
		// reset the pinged flag.  We try to set the flag, but
		// will fail if it is already set.  Failure will lead
		// to us aborting the connection.  Otherwise we send
		// the client a ping request.
		if atomic.CompareAndSwapUint32(&cli.pinged, 0, 1) {
			cli.Send("ping")
		} else {
			// Attempt to send an error message, ignoring errors
			cli.iolock.Lock()
			fmt.Fprint(cli.rwc, "error \"Received no pong\"\r\n")
			cli.iolock.Unlock()

			debug.Printf("%s did not respond to a ping in time", cli)
			cli.Kill()
			break
		}
	}
}

// Handle coordinates a client
//
// It will start a ping thread (if the configuration requires it), a
// goroutine to handle and interpret input and then wait for the
// client to be killed.
func (cli *Client) Handle() {
	// Ensure that the client has a channel that is being
	// communicated upon.
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}
	defer cli.rwc.Close()

	cli.games = make(map[uint64]*Game)

	var ctx context.Context
	ctx, cli.killFn = context.WithCancel(context.Background())

	// Initiate the protocol with the client
	cli.Send("kgp", majorVersion, minorVersion, patchVersion)

	// Optionally start a thread to periodically send ping
	// requests to the client
	var done context.CancelFunc
	if conf.TCP.Ping {
		var pCtx context.Context
		pCtx, done = context.WithCancel(context.Background())
		go cli.Pinger(pCtx)
	}

	// Start a thread to read the user input from rwc
	dead := false
	go func() {
		scanner := bufio.NewScanner(cli.rwc)
		for scanner.Scan() {
			// Prevent flooding by waiting a for a moment
			// between lines
			time.Sleep(time.Microsecond)

			// Check if the client has been killed
			// by someone else
			if dead {
				break
			}

			// Interpret line
			input := scanner.Text()
			debug.Print(cli, " < ", input)
			err := cli.Interpret(input)
			if err != nil {
				log.Print(err)
			}

		}
		// See https://github.com/golang/go/commit/e9ad52e46dee4b4f9c73ff44f44e1e234815800f
		err := scanner.Err()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Print(err)
		}
		cli.Kill()
	}()

	// When the client is killed (a game has finished, the client
	// timed out, ...) we log this and mark the client as dead for
	// the input thread
	<-ctx.Done()

	// Request for the client to be removed from the queue
	forget <- cli

	// Send a simple goodbye, ignoring errors if the network
	// connection was broken
	cli.iolock.Lock()
	fmt.Fprint(cli.rwc, "goodbye\r\n")
	cli.iolock.Unlock()

	// Kill input processing thread
	dead = true

	// Kill ping thread if requested for the connection
	if done != nil {
		done()
	}

	// Unset the ReadWriteCloser
	cli.rwc = nil

	// If the client was currently playing a game, we have to
	// consider what our opponent is doing.  We notify the game
	// that the client is gone.
	if cli.simple {
		cli.game.death <- cli
	} else {
		cli.lock.Lock()
		for _, game := range cli.games {
			game.death <- cli
		}
		cli.lock.Unlock()
	}

	debug.Print("Closed connection to ", cli)
}
