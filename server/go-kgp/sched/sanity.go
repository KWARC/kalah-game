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
	"sync"

	"go-kgp"
	"go-kgp/bot"
)

func MakeSanityCheck() Composable {
	return &scheduler{
		name:   "Sanity Test",
		wait:   sync.WaitGroup{},
		agents: []kgp.Agent{},
		schedule: func(a []kgp.Agent) (games []*kgp.Game) {
			adv := bot.MakeRandom()
			for _, agent := range a {
				games = append(games, &kgp.Game{Board: kgp.MakeBoard(6, 6), South: agent, North: adv}, &kgp.Game{Board: kgp.MakeBoard(6, 6), South: adv, North: agent})
			}
			return
		},
		judge: func(a kgp.Agent, m map[kgp.Agent][]kgp.Agent) bool {
			return len(m[a]) > 0
		},
		results: map[kgp.Agent][]kgp.Agent{},
	}
}
