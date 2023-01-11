// Sanity Test
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
	"go-kgp"
	"go-kgp/bot"
	"go-kgp/sched/isol"
)

func MakeSanityCheck() Composable {
	return &scheduler{
		name: "Sanity Test",
		desc: `All agents are made to compete once against a random bot
on a (6, 6) board.  As the agent is allowed to make the first move, we
know that they must be able to win the game, since this configuration of
Kalah is solved.  To pass this stage, one has to definitively win against
the random bot, otherwise one is disqualified immediately.`,
		schedule: func(a []isol.ControlledAgent) (games []*kgp.Game) {
			adv := bot.MakeRandom()
			for _, agent := range a {
				game := &kgp.Game{Board: kgp.MakeBoard(6, 6), South: agent, North: adv}
				games = append(games, game)
			}
			return
		},
	}
}
