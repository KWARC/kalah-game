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
	"fmt"

	"go-kgp"
)

type challenge struct {
	board *kgp.Board
	move  uint
}

func (cli *Client) challenge() {
	b := kgp.MakeRandomBoard()
	m := b.Random(kgp.South)

	id := cli.send("problem", b, m)
	cli.chall[id] = &challenge{board: b, move: m}
}

func (cli *Client) verify(id, ref uint64, cmd, args string) {
	chall := cli.chall[ref]
	switch cmd {
	case "solution":
		if chall == nil {
			cli.error(id, "Invalid reference")
			return
		}
		delete(cli.chall, ref)

		var (
			state  *kgp.Board
			repeat bool
		)
		parse(args, &state, &repeat)
		isrepeat := chall.board.Sow(kgp.South, chall.move)
		switch {
		case !state.Equal(chall.board):
			cli.error(id, fmt.Sprintf("Expected state %s", chall.board))
		case isrepeat != repeat:
			if isrepeat {
				cli.error(id, "Was a repeat move")
			} else {
				cli.error(id, "Was not a repeat move")
			}
		default:
			cli.respond(id, "ok")
		}

		cli.challenge() // next challenge
	default:
		cli.error(id, "Unknown command")
	}
}
