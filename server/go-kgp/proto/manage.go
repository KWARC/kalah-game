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

	"go-kgp/conf"
)

type tcp struct {
	conf *conf.Conf
	conn net.Listener
}

func (*tcp) String() string {
	return "TCP Handler"
}

func (t *tcp) Start() {
	if t.conf.GM == nil {
		panic("No game manager")
	}

	var err error
	tcp := fmt.Sprintf(":%d", t.conf.TCPPort)
	t.conn, err = net.Listen("tcp", tcp)
	if err != nil {
		t.conf.Log.Fatal(err)
	}

	t.conf.Debug.Printf("Accepting connections on %s", tcp)
	for {
		conn, err := t.conn.Accept()
		if err != nil {
			continue
		}

		MakeClient(conn, t.conf)
	}
}

func (t *tcp) Shutdown() {
	if err := t.conn.Close(); err != nil {
		t.conf.Log.Print(err)
	}
}

func Prepare(conf *conf.Conf) {
	conf.Register(&tcp{conf: conf})
}
