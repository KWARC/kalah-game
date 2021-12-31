// Communication Management
//
// Copyright (c) 2021  Philip Kaludercic
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
	"sync"
	"sync/atomic"
)

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
	id := atomic.AddUint64(&cli.rid, 2)

	fmt.Fprint(cli.rwc, id)
	if to > 0 {
		fmt.Fprintf(cli.rwc, "@%d", to)
	}

	fmt.Fprintf(cli.rwc, " %s", command)

	for _, arg := range args {
		fmt.Fprint(cli.rwc, " ")
		switch arg.(type) {
		case string:
			fmt.Fprintf(cli.rwc, "%#v", arg)
		case int:
			fmt.Fprintf(cli.rwc, "%d", arg)
		case float64:
			fmt.Fprintf(cli.rwc, "%f", arg)
		case *Board:
			fmt.Fprint(cli.rwc, arg)
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

	fmt.Fprint(cli.rwc, "\r\n")

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
		input := scanner.Text()
		err := cli.Interpret(input)
		if err != nil {
			log.Println(err)
		}
	}

	// Try to send a goodbye message, ignoring any errors
	fmt.Fprintf(cli.rwc, "goodbye\r\n")
}
