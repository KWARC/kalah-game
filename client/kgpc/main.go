// Protocol Handling
//
// Copyright (c) 2021  Philip Kaludercic
//
// This file is part of kgpc.
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
	"context"
	"fmt"
	"net"
	"os"
	"regexp"

	"nhooyr.io/websocket"
)

var (
	token  = os.Getenv("TOKEN")
	author = os.Getenv("AUTHOR")
	name   = os.Getenv("NAME")
)

func main() {
	var (
		cli  Client
		err  error
		dest string
	)

	if len(os.Args) <= 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [server address] [command ...]\n",
			os.Args[0])
		os.Exit(1)
	}

	if ok, _ := regexp.MatchString(`^wss?://`, dest); ok {
		ctx := context.Background()
		c, _, err := websocket.Dial(ctx, dest, nil)
		if err == nil {
			cli.rwc = websocket.NetConn(ctx, c, websocket.MessageText)
		}
	} else {
		if ok, _ := regexp.MatchString(`^:\d+$`, dest); !ok {
			dest += ":2671"
		}
		cli.rwc, err = net.Dial("tcp", dest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cli.Handle()
}
