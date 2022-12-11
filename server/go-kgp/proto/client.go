// Client Communication Management
//
// Copyright (c) 2021, 2022  Philip Kaludercic
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

package proto

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-kgp"
	"go-kgp/conf"
)

var defaultUser = &kgp.User{Descr: `Pseudo-user of all unidentified agents.`}

type request struct {
	move chan<- *kgp.Move
	id   uint64
}

type response struct {
	move *kgp.Move
	id   uint64
}

// Client wraps a network connection into a player
type client struct {
	conf *conf.Conf

	// Agent Metadata
	user *kgp.User

	// protocol state
	iolock sync.Mutex // IO Lock
	glock  sync.Mutex // Game Lock
	rwc    io.ReadWriteCloser
	rid    uint64
	last   uint64
	kill   context.CancelFunc
	pinged uint32 // actually bool
	games  map[uint64]*kgp.Game
	req    chan *request
	resp   chan *response
	init   bool
	comm   string
}

func MakeClient(rwc io.ReadWriteCloser, conf *conf.Conf) {
	go (&client{
		user:  defaultUser,
		games: make(map[uint64]*kgp.Game),
		req:   make(chan *request, 1),
		resp:  make(chan *response, 1),
		rwc:   rwc,
		conf:  conf,
	}).handle()
}

func (cli *client) User() *kgp.User {
	return cli.user
}

// Request a client to make a move
func (cli *client) Request(game *kgp.Game) (*kgp.Move, bool) {
	if cli.rwc == nil {
		return nil, true
	}

	c := make(chan *kgp.Move, 1)
	state := game.State
	if game.North == cli {
		state = state.Mirror()
	}
	id := cli.send("state", state)
	defer cli.respond(id, "stop")

	cli.glock.Lock()
	cli.games[id] = game
	cli.glock.Unlock()

	cli.req <- &request{c, id}

	move := &kgp.Move{
		Choice:  game.State.Random(game.Side(cli)),
		Comment: "[random move]",
		Agent:   cli,
		Game:    game,
		Stamp:   time.Now(),
	}

	for {
		select {
		case <-time.After(cli.conf.MoveTimeout):
			return move, false
		case m := <-c:
			if m == nil {
				return move, false
			}
			move = m
		}
	}
}

// String will return a string representation for a client for
// internal use
func (cli *client) String() string {
	return fmt.Sprintf("%p (%q)", cli.rwc, cli.user.Token)
}

// Send is a shorthand to respond without a reference
func (cli *client) send(command string, args ...interface{}) uint64 {
	return cli.respond(0, command, args...)
}

// Error is a shorthand to respond with an error message
func (cli *client) error(to uint64, args ...interface{}) {
	cli.respond(to, "error", args...)
}

// Respond forwards a referenced message to the client
//
// Each element in ARGS is handled as an argument to COMMAND, and will
// use the concrete datatype for formatting.  Respond does not check
// if the arguments have the right types for COMMAND.
//
// If TO is 0, no reference will be added.
func (cli *client) respond(to uint64, command string, args ...interface{}) uint64 {
	if cli == nil {
		return 0
	}

	var (
		buf bytes.Buffer
		id  = atomic.AddUint64(&cli.rid, 2)
	)

	fmt.Fprint(&buf, id)
	if to > 0 {
		fmt.Fprintf(&buf, "@%d", to)
	}

	fmt.Fprintf(&buf, " %s", command)

	for _, arg := range args {
		fmt.Fprint(&buf, " ")
		switch v := arg.(type) {
		case string:
			fmt.Fprintf(&buf, "%#v", v)
		case int:
			fmt.Fprintf(&buf, "%d", v)
		case float64:
			fmt.Fprintf(&buf, "%f", v)
		case *kgp.Board:
			fmt.Fprint(&buf, v.String())
		case *kgp.Game:
			fmt.Fprint(&buf, v.State.String())
		default:
			panic(fmt.Sprintf("Unsupported type: %T", arg))
		}
	}

	// attempt to send this message before any other message is sent
	defer cli.iolock.Unlock()
	cli.iolock.Lock()

	if cli.rwc == nil {
		return 0
	}

	cli.conf.Debug.Println(cli, ">", buf.String())
	fmt.Fprint(&buf, "\r\n")
	_, err := io.Copy(cli.rwc, &buf)
	if err != nil {
		cli.conf.Debug.Print(err)
		return 0
	}

	return id
}

// Pinger regularly sends out a ping and checks if a pong was received.
func (cli *client) pinger(ctx context.Context) {
	if cli.conf.TCPTimeout == 0 {
		panic("TCP Timeout must be greater than 0")
	}
	ticker := time.NewTicker(cli.conf.TCPTimeout)
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

		// To prevent race conditions, we atomically check and
		// reset the pinged flag.  We try to set the flag, but
		// will fail if it is already set.  Failure will lead
		// to us aborting the connection.  Otherwise we send
		// the client a ping request.
		if atomic.CompareAndSwapUint32(&cli.pinged, 0, 1) {
			cli.send("ping")
		} else {
			// Attempt to send an error message, ignoring errors
			cli.iolock.Lock()
			if cli.rwc != nil {
				fmt.Fprint(cli.rwc, "error \"Received no pong\"\r\n")
			}
			cli.iolock.Unlock()

			cli.conf.Debug.Printf("%s did not respond to a ping in time", cli)
			cli.kill()
			break
		}
	}
}

// Handle coordinates a client
//
// It will start a ping thread (if the configuration requires it), a
// goroutine to handle and interpret input and then wait for the
// client to be killed.
func (cli *client) handle() {
	dbg := cli.conf.Debug.Println

	// Ensure that the client has a channel that is being
	// communicated upon.
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}
	defer cli.rwc.Close()

	var ctx context.Context
	ctx, cli.kill = context.WithCancel(context.Background())

	// Initiate the protocol with the client
	cli.send("kgp", majorVersion, minorVersion, patchVersion)

	// Optionally start a thread to periodically send ping
	// requests to the client
	var done context.CancelFunc
	if cli.conf.Ping {
		var pctx context.Context
		pctx, done = context.WithCancel(context.Background())
		go cli.pinger(pctx)
	}

	// Start a thread to read the user input from rwc
	dead := false
	go func() {
		scanner := bufio.NewScanner(cli.rwc)
		for scanner.Scan() {
			// Check if the client has been killed
			// by someone else
			if dead {
				break
			}

			// Interpret line
			input := scanner.Text()
			dbg(cli, "<", input)
			err := cli.interpret(input)
			if err != nil {
				cli.conf.Log.Print(err)
			}

		}

		// See https://github.com/golang/go/commit/e9ad52e46dee4b4f9c73ff44f44e1e234815800f
		err := scanner.Err()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			cli.conf.Log.Print(err)
		}
		cli.kill()
	}()

	var (
		// When the client is killed (a game has finished, the
		// client timed out, ...) we log this and mark the
		// client as dead for the input thread
		rwc = cli.rwc
		// Mappings of request IDs to requests/responses
		reqs  = make(map[uint64]*request)
		resps = make(map[uint64]*response)
	)

	for {
		select {
		case <-ctx.Done():
			dbg("Received shutdown signal for", cli)
			goto shutdown
		case req := <-cli.req:
			if resp, ok := resps[req.id]; ok {
				req.move <- resp.move
			} else {
				if _, ok := reqs[req.id]; ok {
					// we panic here because this
					// means the same request ID
					// has been used for multiple
					// state requests, which
					// violates the assumptions of
					// the protocol.
					panic("Request overridden before handled")
				}
				reqs[req.id] = req
			}
		case resp := <-cli.resp:
			if req, ok := reqs[resp.id]; ok {
				req.move <- resp.move
			}
			// otherwise we will ignore the response
		}
	}
shutdown:

	// Request for the client to be removed from the queue
	cli.conf.GM.Unschedule(cli)

	// Send a simple goodbye, ignoring errors if the network
	// connection was broken
	defer cli.iolock.Unlock()
	cli.iolock.Lock()
	fmt.Fprint(rwc, "goodbye\r\n")

	// Kill input processing thread
	dead = true

	// Kill ping thread if requested for the connection
	if done != nil {
		done()
	}

	// Unset the ReadWriteCloser
	cli.rwc = nil

	dbg("Closed connection to", cli)
}
