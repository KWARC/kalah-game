// Protocol Handling
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
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

var (
	parser = regexp.MustCompile(`^[[:space:]]*` +
		`(?:([[:digit:]]*)(?:@([[:digit:]]+))?[[:space:]]+)?` +
		`([[:alnum:]]+)(?:[[:space:]]+(.*))?` +
		`[[:space:]]*$`)
	errArgumentMismatch = errors.New("argument mismatch")
)

// parse destructs RAW and tries to assign the parts to PARAMS
func parse(raw string, params ...interface{}) error {
	var (
		inquotes bool
		escape   bool
		err      error

		i   int
		arg string
	)

	for i, arg = range strings.FieldsFunc(raw, func(c rune) bool {
		if inquotes {
			if escape {
				escape = false
				return false
			} else if c == '"' {
				inquotes = false
				return true
			} else {
				escape = c == '\\'
				return false
			}
		} else {
			inquotes = c == '"'
			return unicode.IsSpace(c) || inquotes
		}
	}) {
		if i >= len(params) {
			return errArgumentMismatch
		}

		switch param := params[i].(type) {
		case *string:
			*param = arg
		case *uint64:
			*param, err = strconv.ParseUint(arg, 10, 64)
			if err != nil {
				return err
			}
		}
	}

	if i+1 != len(params) {
		return errArgumentMismatch
	}

	return nil
}

// Handle setting KEY to VAL for CLI
func (cli *Client) Set(key, val string) error {
	switch key {
	case "info:name":
		cli.Name = val
	case "info:authors", "info:author":
		cli.Author = val
	case "info:description":
		cli.Descr = val
	case "info:comment":
		cli.comment = val
	case "auth:token":
		if cli.token == nil {
			hash := sha256.New()
			fmt.Fprint(hash, val)
			cli.token = hash.Sum(nil)
			cli.Score = 1000.0

			var wg sync.WaitGroup
			wg.Add(1)
			cli.dbact <- cli.updateDatabase(&wg, true)
			wg.Wait()
		}
	}

	return nil
}

// Interpret parses and evaluates INPUT
func (cli *Client) Interpret(input string) error {
	matches := parser.FindStringSubmatch(input)
	if matches == nil {
		debug.Printf("Malformed input: %v", input)
		return nil
	}

	var (
		id, ref uint64
		err     error

		cmd  = matches[3]
		args = matches[4]
		game = cli.game
	)
	if matches[1] != "" {
		id, err = strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			return nil
		}
	}
	if matches[2] != "" {
		ref, err = strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			return nil
		}
	}

	switch cmd {
	case "mode":
		if game != nil {
			return nil
		}

		var mode string
		err = parse(args, &mode)
		if err != nil {
			return err
		}

		switch mode {
		case "simple":
			cli.simple = true
			fallthrough
		case "freeplay":
			enqueue <- cli
			cli.Respond(id, "ok")
		default:
			cli.Error(id, "Unsupported mode %q", mode)
		}
	case "move":
		if game == nil ||
			!game.IsCurrent(cli) ||
			(ref != game.last && ref != 0) {
			return nil
		}

		var pit uint64
		err = parse(args, &pit)
		if err != nil {
			return err
		}

		game.move <- &Move{
			Pit:    int(pit) - 1,
			Client: cli,
			id:     id,
		}
	case "yield":
		new := atomic.AddInt64(&cli.pending, -1)
		if cli.simple && new < -1 {
			cli.Error(id, "Preemptive yield")
			cli.killFunc()
		}

		if game == nil ||
			!game.IsCurrent(cli) ||
			(ref != game.last && ref != 0) {
			return nil
		}

		game.yield <- cli
	case "ok", "error":
		// We do not expect the client to confirm or reject anything,
		// so we can ignore these response messages.
	case "pong":
		cli.pinged = false
	case "set":
		// Note that VAL doesn't have to be a string per spec,
		// but we will parse it as such to keep it in it's
		// intermediate representation. If we need to convert
		// it to something else later on, we will do so then.
		var key, val string
		err := parse(args, &key, &val)
		if err != nil {
			return err
		}

		return cli.Set(key, val)
	case "goodbye":
		cli.killFunc()
	}

	return nil
}

func (tc *TCPConf) deinit() {
	if tc.cancel != nil {
		tc.cancel()
	}
}

func (tc *TCPConf) init() {
	debug.Print("Starting TCP listener")

	if !tc.Enabled {
		return
	}

	var (
		conns chan net.Conn
		ctx   context.Context
		dead  bool
	)

	ctx, tc.cancel = context.WithCancel(context.Background())
	tcp := fmt.Sprintf("%s:%d", tc.Host, tc.Port)
	ln, err := net.Listen("tcp", tcp)
	if err != nil {
		log.Fatal(err)
	}
	debug.Printf("Listening on TCP %s", tcp)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if dead {
					return
				}
				log.Print(err)
				continue
			}

			conns <- conn
		}
	}()

	for {
		select {
		case conn := <-conns:
			log.Printf("New connection from %s", conn.RemoteAddr())
			go (&Client{rwc: conn, conf: tc}).Handle()
		case <-ctx.Done():
			dead = true
			err = ln.Close()
			if err != nil {
				log.Println(err)
			}
			return
		}
	}
}
