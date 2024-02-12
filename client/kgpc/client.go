// Communication Management
//
// Copyright (c) 2021, 2023, 2024  Philip Kaludercic
//
// This file is part of kgpc, based on go-kgp.
//
// kgpc is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License,
// version 3, as published by the Free Software Foundation.
//
// kgpc is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License, version 3, along with kgpc. If not, see
// <http://www.gnu.org/licenses/>

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
)

var debug = os.Getenv("DEBUG") != ""

// Client wraps a network connection into a player
type Client struct {
	rwc  io.ReadWriteCloser
	rid  uint64
	lock sync.Mutex
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
	var out io.Writer = cli.rwc
	if debug {
		fmt.Fprint(os.Stderr, "> ")
		out = io.MultiWriter(os.Stderr, out)
	}

	id := atomic.AddUint64(&cli.rid, 2)
	fmt.Fprint(out, id)
	if to > 0 {
		fmt.Fprintf(out, "@%d", to)
	}

	fmt.Fprintf(out, " %s", command)

	for _, arg := range args {
		fmt.Fprint(out, " ")
		switch arg.(type) {
		case string:
			fmt.Fprintf(out, "%#v", arg)
		case int:
			fmt.Fprintf(out, "%d", arg)
		case float64:
			fmt.Fprintf(out, "%f", arg)
		case *Board:
			fmt.Fprint(out, arg)
		default:
			panic("Unsupported type")
		}
	}

	// attempt to send this message before any other message is sent
	defer cli.lock.Unlock()
	cli.lock.Lock()

	if out == nil {
		return 0
	}

	fmt.Fprint(out, "\r\n")

	return id
}

// Handle controls a connection and reads user input
func (cli *Client) Handle() {
	cli.rid = 1

	// Ensure that the client has a channel that is being
	// communicated upon.
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}
	defer cli.rwc.Close()

	scanner := bufio.NewScanner(cli.rwc)
	for scanner.Scan() {
		err := cli.Interpret(scanner.Text())
		if err != nil {
			log.Println(err)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading protocol:", err)
	}

	// Try to send a goodbye message, ignoring any errors
	fmt.Fprintf(cli.rwc, "goodbye\r\n")
}
