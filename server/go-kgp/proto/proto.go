// Protocol Handling
//
// Copyright (c) 2021, 2022, 2023  Philip Kaludercic
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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"go-kgp"
	"go-kgp/cmd"
)

const (
	majorVersion = 1
	minorVersion = 0
	patchVersion = 1
)

var (
	// Regular expression to destruct a command
	tokenizer = regexp.MustCompile(`^[[:space:]]*` +
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
	case 't':
		return "\t"
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
		case **kgp.Board:
			*param, err = kgp.Parse(arg)
			if err != nil {
				return err
			}
		case *bool:
			switch strings.ToLower(arg) {
			case "true", "t", "yes":
				*param = true
			case "false", "f", "no":
				*param = false
			default:
				return errors.New("Malformed bool")
			}
		}
	}

	if i+1 != len(params) {
		return errArgumentMismatch
	}

	return nil
}

// Interpret parses and evaluates INPUT
func (cli *Client) interpret(input string, st *cmd.State) error {
	input = strings.TrimSpace(input)
	if input == "" { // Ignore empty lines
		return nil
	}

	matches := tokenizer.FindStringSubmatch(input)
	if matches == nil {
		kgp.Debug.Printf("Malformed input: %v", input)
		return nil
	}

	var (
		id, ref uint64
		err     error

		cmd  = matches[3]
		args = matches[4]
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
		if cli.init {
			cli.error(id, "Duplicate \"mode\" request")
			cli.Kill()
			return nil
		}

		var mode string
		err = parse(args, &mode)
		if err != nil {
			return err
		}

		switch mode {
		case "freeplay":
			st.Scheduler.Schedule(cli)
			cli.respond(id, "ok")
			atomic.StoreInt32((*int32)(&cli.mode), int32(freeplay))
		case "verify", "go-kgp:verify":
			cli.chall = make(map[uint64]*challenge)
			atomic.StoreInt32((*int32)(&cli.mode), int32(verify))
			cli.challenge()
		default:
			cli.error(id, "Unsupported mode %q", mode)
		}
	case "ok", "error":
		// We do not expect the client to confirm or reject anything,
		// so we can ignore these response messages.
	case "pong":
		cli.alive <- struct{}{}
	case "set":
		// Note that VAL doesn't have to be a string per spec,
		// but we will parse it as such to keep it in it's
		// intermediate representation. If we need to convert
		// it to something else later on, we will do so then.
		var key, val string
		err := parse(args, &key, &val)
		if err != nil {
			return fmt.Errorf("Parse error (%s): %w on input %q", cli, err, input)
		}

		switch key {
		case "info:name":
			if cli.user == defaultUser {
				cli.user = &kgp.User{Token: val}
			}
			cli.user.Name = val
		case "info:authors", "info:author":
			if cli.user == defaultUser {
				cli.user = &kgp.User{Token: val}
			}
			cli.user.Author = val
		case "info:description":
			if cli.user == defaultUser {
				cli.user = &kgp.User{Token: val}
			}
			cli.user.Descr = val
		case "info:comment":
			cli.comm = val
		case "auth:token":
			cli.user = &kgp.User{
				Name:   cli.user.Name,
				Author: cli.user.Author,
				Descr:  cli.user.Descr,
				Token:  val,
			}
			if cli.user.Descr == defaultUser.Descr {
				cli.user.Descr = ""
			}
		}
	case "goodbye":
		cli.Kill()
	default:
		switch cli.mode {
		case freeplay:
			cli.freeplay(id, ref, cmd, args)
		case verify:
			cli.verify(id, ref, cmd, args)
		}
		kgp.Debug.Printf("Invalid command %q", input)
	}

	return nil
}

func (cli *Client) freeplay(id, ref uint64, cmd, args string) {
	cli.glock.Lock()
	game := cli.games[ref]
	cli.glock.Unlock()

	switch cmd {
	case "move":
		if game == nil {
			cli.error(id, "No state associated with id")

			return
		}

		var pit uint64
		err := parse(args, &pit)
		if err != nil {
			return
		}

		cli.resp <- &response{
			move: &kgp.Move{
				Game:    game,
				Agent:   cli,
				Choice:  uint(pit) - 1,
				Comment: cli.comm,
				Stamp:   time.Now(),
			},
			id: ref,
		}
		cli.comm = ""
	case "yield":
		if game == nil {
			cli.error(id, "No state associated with id")
			return
		}

		cli.resp <- &response{
			move: nil,
			id:   ref,
		}
		cli.comm = ""
	default:
		cli.error(id, "Unknown command")
	}
}
