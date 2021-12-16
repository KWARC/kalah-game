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

// An Agent
type Agent struct {
	Name   string
	Author string
	Descr  string
	Score  float64
	Id     int64
	Games  uint
}

// Client wraps a network connection into a player
type Client struct {
	Agent
	game     *Game
	rwc      io.ReadWriteCloser
	lock     sync.Mutex
	rid      uint64
	killFunc context.CancelFunc
	pinged   bool
	token    []byte
	comment  string
	simple   bool
	pending  int64
	conf     *TCPConf
	dbact    chan<- DBAction
}

func (cli *Client) String() string {
	hash := base64.StdEncoding.EncodeToString(cli.token)
	if conn, ok := cli.rwc.(net.Conn); ok {
		return fmt.Sprintf("%s (%q)", conn.RemoteAddr(), hash)
	}
	if conn, ok := cli.rwc.(*wsrwc); ok {
		return fmt.Sprintf("%s (%q)", conn.RemoteAddr(), hash)
	}
	return fmt.Sprintf("%p (%q)", cli, hash)
}

// Send forwards an unreferenced message to the client
func (cli *Client) Send(command string, args ...interface{}) uint64 {
	return cli.Respond(0, command, args...)
}

func (cli *Client) Error(to uint64, args ...interface{}) {
	cli.Respond(to, "error", args...)
}

// Respond forwards a referenced message to the client
func (cli *Client) Respond(to uint64, command string, args ...interface{}) uint64 {
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
	defer cli.lock.Unlock()
	cli.lock.Lock()

	if cli.rwc == nil {
		return 0
	}

	debug.Print(cli, " > ", buf.String())
	fmt.Fprint(buf, "\r\n")

	i := cli.conf.Retries // allow 8 unsuccesful retries
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
			wait <<= cli.conf.Retries - i
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
			cli.killFunc()
		}
	}

	return id
}

func (cli *Client) Pinger(done <-chan struct{}) {
	if cli.conf.Timeout == 0 {
		panic("TCP Timeout must be greater than 0")
	}
	ticker := time.NewTicker(time.Duration(cli.conf.Timeout) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}

		// If the timer fired, check the ping flag and
		// kill the client if it is still set
		if cli.pinged {
			log.Printf("%s did not respond to a ping in time", cli)
			cli.killFunc()
			break
		}
		// In case it was not set, ping the client
		// again and reset the flag
		cli.Send("ping")
		cli.pinged = true
	}
}

// Handle controls a connection and reads user input
func (cli *Client) Handle() {
	// Ensure that the client has a channel that is being
	// communicated upon.
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}
	defer cli.rwc.Close()

	context, cancel := context.WithCancel(context.Background())
	cli.killFunc = cancel

	// Initiate the protocol with the client
	cli.Send("kgp", majorVersion, minorVersion, patchVersion)

	// Optionally start a thread to periodically send ping
	// requests to the client
	var done chan struct{}
	if cli.conf.Ping {
		done = make(chan (struct{}))
		go cli.Pinger(done)
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
		cli.killFunc()
	}()

	// When the client is killed (a game has finished, the client
	// timed out, ...) we log this and mark the client as dead for
	// the input thread
	<-context.Done()

	// Request for the client to be removed from the queue
	forget <- cli

	// To avoid concurrency issues, the client lock is reserved
	// for the rest of the function/goroutine's lifetime
	cli.lock.Lock()
	defer cli.lock.Unlock()

	// Send a simple goodbye, ignoring errors if the network
	// connection was broken
	fmt.Fprint(cli.rwc, "goodbye\r\n")

	// Kill input processing thread
	dead = true

	// Kill ping thread if requested for the connection
	if done != nil {
		close(done)
	}

	// Unset the ReadWriteCloser
	cli.rwc = nil

	// If the client was currently playing a game, we have to
	// consider what our opponent is doing.  We notify the game
	// that the client is gone.
	if cli.game != nil {
		cli.game.death <- cli
	}

	debug.Print("Closed connection to", cli)
}
