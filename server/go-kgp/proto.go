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

		game.ctrl <- Move(pit)
	case "yield":
		if game == nil ||
			!game.IsCurrent(cli) ||
			(ref != game.last && ref != 0) {
			return nil
		}

		game.ctrl <- Yield(false)
		cli.Respond(game.last, "stop")
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
	default:
		cli.Respond(id, "error", "Invalid command")
	}

	return nil
}
