// Round Robin Tournament
//
// Copyright (c) 2022, 2023  Philip Kaludercic
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

package sched

import (
	"fmt"
	"go-kgp/sched/isol"

	"go-kgp"
)

func MakeRoundRobin(size, init uint) Composable {
	return &scheduler{
		name: fmt.Sprintf("Round Robin (%d, %d)", size, init),
		desc: `All agents play against all other agents, on both sides
of a Kalah board.  If one agent definitively manages to beat another agent,
they are awarded two points, and the opponent is penalised by two points.
For a draw, both sides are granted a single point.  The final score of this
round is calculated by summing up the points for each game.  Agents with a
score of less than two are disqualified.`,
		schedule: func(agents []isol.ControlledAgent) (games []*kgp.Game) {
			// Prepare all games
			for _, a := range agents {
				for _, b := range agents {
					if a == b {
						continue
					}

					games = append(games, &kgp.Game{
						Board: kgp.MakeBoard(size, init),
						South: a, North: b,
					})
				}
			}
			return
		},
	}
}
