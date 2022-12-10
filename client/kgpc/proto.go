// Protocol Handling
//
// Copyright (c) 2021  Philip Kaludercic
//
// This file is part of go-kgp, based on go-kgp.
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
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	parser = regexp.MustCompile(`^[[:space:]]*` +
		`(?:([[:digit:]]+)(?:@([[:digit:]]+))?[[:space:]]+)?` +
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
		case *int64:
			*param, err = strconv.ParseInt(arg, 10, 64)
			if err != nil {
				return err
			}
		case *Board:
			param = parseBoard(arg)
			if param != nil {
				return errors.New("malformed board")
			}
		}
	}

	if i+1 != len(params) {
		return errArgumentMismatch
	}

	return nil
}

// Interpret parses and evaluates INPUT
func (cli *Client) Interpret(input string) error {
	matches := parser.FindStringSubmatch(input)
	if matches == nil {
		return errors.New("malformed input")
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
	case "kgp":
		var major, minor, patch int64
		err = parse(args, &major, &minor, &patch)
		if err != nil {
			return err
		}

		if major != 1 {
			return errors.New("unsupported version")
		}
		if token != "" {
			cli.Send("set", "auth:token", token)
		}
		if name != "" {
			cli.Send("set", "info:name", name)
		}
		if author != "" {
			cli.Send("set", "info:author", author)
		}
		cli.Send("mode", "freeplay")
	case "state":
		var board *Board
		err = parse(args, &board)
		if err != nil {
			return err
		}
		start(cli, id, board)
	case "stop":
		stop(ref)
	case "ping":
		cli.Respond(id, "pong")
	case "goodbye":
		os.Exit(0)
	}

	return nil
}
