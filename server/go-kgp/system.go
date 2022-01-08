// Tournament Systems
//
// Copyright (c) 2022  Philip Kaludercic
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

type System func(*Tournament) []*Game

func roundRobin(t *Tournament) (games []*Game) {
	// Check of the tournament is over
	if int(t.round) >= len(t.participants) {
		return nil
	}

	// Calculate the size of the board/number of stones for this
	// round of the tournament
	r := (float64(t.round) / float64(len(t.participants)))
	size := conf.Game.Sizes[int(float64(len(conf.Game.Sizes))*r)]
	stones := conf.Game.Stones[int(float64(len(conf.Game.Stones))*r)]

	// Collect all games for the current round, using the circle
	// method:
	// https://en.wikipedia.org/wiki/Round-robin_tournament#Circle_method
	circle := make([]*Client, len(t.participants))
	copy(circle, t.participants)

	for i := 1; i < len(t.participants); i++ {
		// Starting from the current position...
		j := i
		// The circle method rotates the 2nd to last
		// participant by one place for each round.  This
		// calculates the assignments directly for the nth
		// round:
		j += int(t.round) - 1
		j %= len(t.participants) - 1

		circle[i] = t.participants[1+j]
	}

	n := len(circle)
	if n%2 == 1 {
		// Ensure n is even
		n--
	}
	for i := 0; i < len(circle)/2; i++ {
		games = append(games, &Game{
			Board: makeBoard(size, stones),
			North: circle[i],
			South: circle[n-i-1],
		})
	}

	return
}
