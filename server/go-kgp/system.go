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

import (
	"fmt"
	"log"
)

// A tournament system decides what games to play, and records results
//
// All methods are called in a synchronised context, and do not have
// to be thread-safe.
type System interface {
	fmt.Stringer
	// Register a client as ready
	Ready(*Tournament, *Client)
	// Record the outcome of a game
	Record(*Tournament, *Game)
	// Check if a tournament is over
	Over(*Tournament) bool
}

// roundRobin tournaments let every participant play with every other
// participant.
type roundRobin struct{ size, round, ready uint }

func (rr *roundRobin) String() string {
	return fmt.Sprintf("round-robin-%d", rr.size)
}

// Notify that a client is ready
func (rr *roundRobin) Ready(t *Tournament, _ *Client) {
	rr.ready++

	// If everyone is done, proceed to the next round
	//
	// FIXME: We do not need to wait for everyone to finish before
	// organising the next round.  Tournaments can be accelerated
	// by allowing clients to play as soon as they are done.  This
	// could be done by scheduling all matches ({ game(a, b) | a ∈
	// P, b ∈ P, a ≠ b}) a priori, then starting games from this
	// schedule as soon as all the necessary participants for a
	// game are ready.
	if rr.ready == uint(len(t.participants)) {
		rr.nextRound(t)

		// If we have an odd number of participants, one will
		// not be able to play.  He is always regarded as
		// ready.
		if len(t.participants)%2 == 1 {
			rr.ready = 1
		} else {
			rr.ready = 1
		}

	}
}

// The result of a game is not relevant for round robin
func (*roundRobin) Record(*Tournament, *Game) {}

// A round robin tournament is over as soon as everyone has played a
// game against every other participant.  For n participants, this
// means every one has had n-1 games, ie. there have been n-1 rounds.
func (rr *roundRobin) Over(t *Tournament) bool {
	return rr.round >= uint(len(t.participants))
}

func (rr *roundRobin) nextRound(t *Tournament) {
	if rr.Over(t) {
		log.Printf("[%p] Finished round robin tournament", rr)
		return
	}
	rr.round++
	log.Printf("[%p] Start round robin round %d", rr, rr.round)

	// Calculate the size of the board/number of stones for this
	// round of the tournament

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
		j += int(rr.round) - 1
		j %= len(t.participants) - 1

		circle[i] = t.participants[1+j]
	}

	n := len(circle)
	if n%2 == 1 {
		// Ensure n is even
		n--
	}

	for i := 0; i < len(circle)/2; i++ {
		t.games <- &Game{
			Board: makeBoard(rr.size, rr.size),
			North: circle[i],
			South: circle[n-i-1],
		}
	}
}
