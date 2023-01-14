// TCP interface
//
// Copyright (c) 2021, 2022, 2023, 2023  Philip Kaludercic
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
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"go-kgp/cmd"
)

type Listener struct {
	conn    net.Listener
	port    uint16
	handler func(*Client) bool
}

func (*Listener) String() string {
	return "TCP Handler"
}

// Initialise a listener, unless it has already been initialised
func (t *Listener) init() {
	if t.conn != nil {
		return
	}

	var err error
	tcp := fmt.Sprintf(":%d", t.port)
	t.conn, err = net.Listen("tcp", tcp)
	if err != nil {
		log.Fatal(err)
	}
	if t.port == 0 {
		// Extract port number the operating system bound the listener
		// to, since port 0 is redirected to a "random" open port
		addr := t.conn.Addr().String()
		i := strings.LastIndexByte(addr, ':')
		if i == -1 && i+1 == len(addr) {
			log.Fatal("Invalid address ", addr)
		}
		port, err := strconv.ParseUint(addr[i+1:], 10, 16)
		if err != nil {
			log.Fatal("Unexpected error ", err)
		}
		t.port = uint16(port)
	}
}

func (t *Listener) Start(st *cmd.State, conf *cmd.Conf) {
	if st.Scheduler == nil {
		panic("No game scheduler")
	}
	t.init()

	log.Printf("Accepting connections on :%d", t.port)
	for {
		conn, err := t.conn.Accept()
		if err != nil {
			continue
		}

		if tcp, ok := conn.(*net.TCPConn); ok {
			err = tcp.SetKeepAlive(true)
			if err != nil {
				log.Println("Failed to enable keepalive:", err)
			}
		}

		if t.handler(MakeClient(conn, conf)) {
			break
		}
	}
}

func (t *Listener) Port() uint16 {
	return t.port
}

func (t *Listener) Shutdown() {
	if err := t.conn.Close(); err != nil {
		log.Print(err)
	}
}

func MakeListner(st *cmd.State, port uint) *Listener {
	return &Listener{
		handler: func(c *Client) bool {
			go c.Connect(st)
			return false
		},
		port: uint16(port),
	}
}

func StartListner(st *cmd.State, conf *cmd.Conf, handler func(*Client) bool) *Listener {
	l := &Listener{handler: handler, port: 0}
	l.init()
	go l.Start(st, conf)
	return l
}

func Register(st *cmd.State, conf *cmd.Conf) {
	st.Register(MakeListner(st, conf.Proto.Port))
}
