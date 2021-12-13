// Websocket utility functions
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
	"context"
	"log"
	"net/http"

	ws "nhooyr.io/websocket"
)

func listenUpgrade(w http.ResponseWriter, r *http.Request) {
	// upgrade to websocket or bail out
	c, err := ws.Accept(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to establish websocket connection", http.StatusBadRequest)
		return
	}
	defer c.Close(ws.StatusInternalError, "Error in websocket connection")

	conn := ws.NetConn(context.Background(), c, ws.MessageText)
	log.Printf("New connection from %s", conn.RemoteAddr())
	(&Client{rwc: conn, isWS: true}).Handle()
}
