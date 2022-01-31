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
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
)

var (
	// Regular expression to destruct a command
	parser = regexp.MustCompile(`^[[:space:]]*` +
		`(?:([[:digit:]]*)(?:@([[:digit:]]+))?[[:space:]]+)?` +
		`([[:alnum:]]+)(?:[[:space:]]+(.*))?` +
		`[[:space:]]*$`)

	// Regular expression to match escaped chararchters
	unescape = regexp.MustCompile(`\\.`)

	// Error to return if a message couldn't be parsed
	errArgumentMismatch = errors.New("argument mismatch")
)

func descape(str string) string {
	switch str[1] {
	case 'n':
		return "\n"
	default:
		return str[1:]
	}
}

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
			*param = unescape.ReplaceAllStringFunc(arg, descape)
		case *uint64:
			*param, err = strconv.ParseUint(arg, 10, 64)
			if err != nil {
				return err
			}
		default:
			panic("Unsupported argument type")
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
		cli.updateDatabase(true)
	case "info:authors", "info:author":
		cli.Author = val
		cli.updateDatabase(true)
	case "info:description":
		cli.Descr = val
		cli.updateDatabase(true)
	case "info:comment":
		cli.comment = val
	case "auth:token":
		if cli.token == nil {
			hash := sha256.New()
			fmt.Fprint(hash, val)
			cli.token = hash.Sum(nil)
			cli.Score = 1000.0
			cli.updateDatabase(true)
		}
	case "auth:forget":
		hash := sha256.New()
		fmt.Fprint(hash, val)
		token := hash.Sum(nil)

		cli.forget(token)
	}

	return nil
}

// Interpret parses and evaluates INPUT
func (cli *Client) Interpret(input string) error {
	if strings.TrimSpace(input) == "" {
		// Ignore empty lines
		return nil
	}

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
		game *Game
	)
	if matches[1] != "" {
		id, err = strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			debug.Printf("Error while parsing ID in %v: %s", input, err)
			return nil
		}
	}
	if matches[2] != "" {
		ref, err = strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			debug.Printf("Error while parsing ref in %v: %s", input, err)
			return nil
		}
	}
	if cli.simple {
		game = cli.game
	} else {
		cli.lock.Lock()
		game = cli.games[ref]
		cli.lock.Unlock()
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
			if cli.notify != nil {
				cli.notify <- cli
			}
			enqueue <- cli
			cli.Respond(id, "ok")
		default:
			cli.Error(id, "Unsupported mode %q", mode)
		}
	case "move":
		if !game.IsCurrent(cli, ref) {
			debug.Printf("%s ignored move (ref: %d, game: %s)", cli, ref, game)
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
			ref:    ref,
		}
	case "yield":
		new := atomic.AddUint64(&cli.nyield, 1)
		if cli.simple && new-1 > cli.nstop {
			cli.Error(id, "Preemptive yield")
			cli.Kill()
		}

		if !game.IsCurrent(cli, ref) {
			debug.Printf("%s ignored yield (ref: %d, game: %s)", cli, ref, game)
			return nil
		}

		game.move <- &Move{
			Yield:  true,
			Client: cli,
			id:     id,
			ref:    ref,
		}
	case "ok", "error":
		// We do not expect the client to confirm or reject anything,
		// so we can ignore these response messages.
	case "pong":
		atomic.StoreUint32(&cli.pinged, 0)
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
		cli.Kill()
	}

	return nil
}
