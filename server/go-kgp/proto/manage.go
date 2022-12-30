// TCP interface
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
	"fmt"
	"net"
	"strconv"
	"strings"

	"go-kgp/conf"
)

type Listener struct {
	conf    *conf.Conf
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
		t.conf.Log.Fatal(err)
	}
	if t.port == 0 {
		// Extract port number the operating system bound the listener
		// to, since port 0 is redirected to a "random" open port
		addr := t.conn.Addr().String()
		i := strings.LastIndexByte(addr, ':')
		if i == -1 && i+1 == len(addr) {
			t.conf.Log.Fatal("Invalid address ", addr)
		}
		port, err := strconv.ParseUint(addr[i+1:], 10, 16)
		if err != nil {
			t.conf.Log.Fatal("Unexpected error ", err)
		}
		t.port = uint16(port)

	}
}

func (t *Listener) Start() {
	if t.conf.GM == nil {
		panic("No game manager")
	}
	t.init()

	t.conf.Debug.Printf("Accepting connections on :%d", t.port)
	for {
		conn, err := t.conn.Accept()
		if err != nil {
			continue
		}

		if t.handler(MakeClient(conn, t.conf)) {
			break
		}
	}
}

func (t *Listener) Port() uint16 {
	return t.port
}

func (t *Listener) Shutdown() {
	if err := t.conn.Close(); err != nil {
		t.conf.Log.Print(err)
	}
}

func launch(c *Client) bool {
	go c.Connect()
	return false
}

func MakeListner(conf *conf.Conf, port uint16) *Listener {
	return &Listener{conf: conf, port: port, handler: launch}
}

func StartListner(conf *conf.Conf, handler func(*Client) bool) *Listener {
	l := &Listener{conf: conf, handler: handler}
	l.init()
	go l.Start()
	return l
}

func Prepare(conf *conf.Conf) {
	conf.Register(MakeListner(conf, uint16(conf.TCPPort)))
}
