// Websocket interface
//
// Copyright (c) 2021, 2022, 2023  Philip Kaludercic
// Copyright (c) 2021  Tom Wiesing
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

package web

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"go-kgp"
	"go-kgp/cmd"
	"go-kgp/proto"

	ws "nhooyr.io/websocket"
)

// wsrwc is a read-write-closer using websockets
type wsrwc struct{ conn *ws.Conn }

// Convert a write call to a Websocket message
func (c *wsrwc) Write(p []byte) (int, error) {
	err := c.conn.Write(context.Background(), ws.MessageText, p)
	return len(p), err
}

// Convert a read call into a Websocket query
func (c *wsrwc) Read(p []byte) (int, error) {
	t, s, err := c.conn.Read(context.Background())
	if err != nil {
		return 0, err
	}
	if t != ws.MessageText {
		return 0, fmt.Errorf("wrong message type")
	}
	return copy(p, s), nil

}

// Convert a close call into a Websocket command
func (c *wsrwc) Close() error {
	return c.conn.Close(ws.StatusNormalClosure, "Connection closed")
}

// Upgrade a HTTP connection to a WebSocket and handle it
func upgrader(st *cmd.State, conf *cmd.Conf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, nil)
		if err != nil {
			kgp.Debug.Printf("Unable to upgrade connection: %s", err)
			w.WriteHeader(500)
			return
		}

		log.Printf("New connection from %s", r.RemoteAddr)
		cli := proto.MakeClient(&wsrwc{conn}, conf)
		go cli.Connect(st)
	}
}
