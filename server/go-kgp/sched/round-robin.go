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
	var s *scheduler
	s = &scheduler{
		name: fmt.Sprint("Round Robin (%d, %d)", size, init),
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
	return s
}
