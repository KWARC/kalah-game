package main

import (
	"errors"
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
	errArgumentMismatch = errors.New("Argument mismatch")
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
		cli.name = val
	case "info:authors":
		cli.author = val
	case "info:description":
		cli.descr = val
	case "info:comment":
		cli.comment = val
	case "auth:token":
		if cli.token == "" {
			cli.token = val
			dbact <- cli.UpdateDatabase
		}
	}

	return nil
}

// Interpret parses and evaluates INPUT
func (cli *Client) Interpret(input string) error {
	matches := parser.FindStringSubmatch(input)
	if matches == nil {
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
		if game != nil || cli.waiting {
			return nil
		}

		var mode string
		err = parse(args, &mode)
		if err != nil {
			return err
		}

		switch mode {
		case "simple", "freeplay":
			cli.waiting = true
			enqueue(cli)
			cli.Respond(id, "ok")
		default:
			cli.Respond(id, "error", "Unsupported Mode")
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

		game.ctrl <- Move{
			pit: int(pit),
			cli: cli,
		}
	case "yield":
		if game == nil ||
			!game.IsCurrent(cli) ||
			(ref != game.last && ref != 0) {
			return nil
		}

		game.ctrl <- Yield(false)
		// cli.Respond(game.last, "stop")
	case "ok", "fail", "error":
		var msg string
		parse(args, &msg) // parsing errors are ignored

		if cmd == "fail" && game != nil {
			game.ctrl <- Yield(true)
		}
	case "pong":
		cli.pinged = false
		if cli.waiting {
			boost(cli)
		}
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
	}

	return nil
}
