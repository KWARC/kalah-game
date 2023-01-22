// Client Communication Management
//
// Copyright (c) 2021, 2022, 2023  Philip Kaludercic
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
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-kgp"
	"go-kgp/cmd"
)

var defaultUser = &kgp.User{
	Name:  "Anonymous",
	Descr: `Pseudo-user of all unidentified agents.`,
}

type request struct {
	move chan<- *kgp.Move
	id   uint64
}

type response struct {
	move *kgp.Move
	id   uint64
}

// Client wraps a network connection into a player
type Client struct {
	conf *cmd.Conf

	// Agent Metadata
	user *kgp.User

	// protocol state
	iolock sync.Mutex // IO Lock
	glock  sync.Mutex // Game Lock
	rwc    io.ReadWriteCloser
	rid    uint64
	Kill   context.CancelFunc
	games  map[uint64]*kgp.Game
	req    chan *request
	resp   chan *response
	alive  chan struct{}
	init   bool
	dead   uint32
	comm   string
}

func MakeClient(rwc io.ReadWriteCloser, conf *cmd.Conf) *Client {
	return &Client{
		user:  defaultUser,
		games: make(map[uint64]*kgp.Game),
		req:   make(chan *request, 1),
		resp:  make(chan *response, 1),
		alive: make(chan struct{}, 1),
		rwc:   rwc,
		conf:  conf,
	}
}

func (cli *Client) User() *kgp.User {
	return cli.user
}

// Request a client to make a move
func (cli *Client) Request(game *kgp.Game) (*kgp.Move, bool) {
	if atomic.LoadUint32(&cli.dead) != 0 {
		return nil, true
	}

	c := make(chan *kgp.Move, 1)
	board := game.Board
	if game.North == cli {
		board = board.Mirror()
	}
	id := cli.send("state", board)
	defer cli.respond(id, "stop")

	cli.glock.Lock()
	cli.games[id] = game
	cli.glock.Unlock()

	cli.req <- &request{c, id}

	move := &kgp.Move{
		Choice:  game.Board.Random(game.Side(cli)),
		Comment: "[random move]",
		Agent:   cli,
		State:   &kgp.Board{},
		Game:    game,
	}

	for {
		select {
		case <-time.After(cli.conf.Game.Timeout):
			ok := cli.ping()
			return move, !ok
		case m := <-c:
			if m == nil {
				return move, false
			}
			move = m
		}
	}
}

func (cli *Client) Alive() bool {
	return cli.ping()
}

// String will return a string representation for a client for
// internal use
func (cli *Client) String() string {
	if cli.user.Token != "" {
		return fmt.Sprintf("%s", cli.user.Token)
	} else {
		return fmt.Sprintf("%p", cli)
	}
}

// Send is a shorthand to respond without a reference
func (cli *Client) send(command string, args ...interface{}) uint64 {
	return cli.respond(0, command, args...)
}

// Error is a shorthand to respond with an error message
func (cli *Client) error(to uint64, args ...interface{}) {
	cli.respond(to, "error", args...)
}

// Respond forwards a referenced message to the client
//
// Each element in ARGS is handled as an argument to COMMAND, and will
// use the concrete datatype for formatting.  Respond does not check
// if the arguments have the right types for COMMAND.
//
// If TO is 0, no reference will be added.
func (cli *Client) respond(to uint64, command string, args ...interface{}) uint64 {
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

	if atomic.LoadUint32(&cli.dead) != 0 {
		return 0
	}

	kgp.Debug.Println(cli, ">", buf.String())

	// attempt to send this message before any other message is sent
	defer cli.iolock.Unlock()
	cli.iolock.Lock()
	fmt.Fprint(&buf, "\r\n")
	_, err := io.Copy(cli.rwc, &buf)
	if err != nil {
		kgp.Debug.Print(err)
		return 0
	}

	return id
}

// Ping a client, block and return if it is still alive
func (cli *Client) ping() bool {
	if atomic.LoadUint32(&cli.dead) != 0 {
		return false
	}
	if !cli.conf.Proto.Ping {
		return true
	}

	id := cli.send("ping")
	if id == 0 {
		return false
	}

	select {
	case <-time.After(cli.conf.Proto.Timeout):
		cli.error(id, "received no pong")
		for cli.Kill == nil {
			time.Sleep(time.Millisecond * 10)
		}
		if cli.Kill != nil {
			cli.Kill()
		}
		return false
	case <-cli.alive:
		return true
	}
}

// Handle coordinates a client
//
// It will start a ping thread (if the configuration requires it), a
// goroutine to handle and interpret input and then wait for the
// client to be killed.
func (cli *Client) Connect(st *cmd.State) {
	dbg := kgp.Debug.Println

	// Ensure that the client has a channel that is being
	// communicated upon.
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}
	defer cli.rwc.Close()

	var ctx context.Context
	ctx, cli.Kill = context.WithCancel(context.Background())

	// Initiate the protocol with the client
	cli.send("kgp", majorVersion, minorVersion, patchVersion)

	// Start a thread to read the user input from rwc
	go func() {
		scanner := bufio.NewScanner(cli.rwc)
		for scanner.Scan() {
			// Check if the client has been killed
			// by someone else
			if atomic.LoadUint32(&cli.dead) != 0 {
				break
			}

			// Interpret line
			input := scanner.Text()
			dbg(cli, "<", input)
			err := cli.interpret(input, st)
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
	st.Scheduler.Unschedule(cli)

	// Send a simple goodbye, ignoring errors if the network
	// connection was broken
	defer cli.iolock.Unlock()
	cli.iolock.Lock()
	fmt.Fprint(rwc, "goodbye\r\n")

	// Mark client as dead
	atomic.StoreUint32(&cli.dead, 1)

	dbg("Closed connection to", cli)
}
