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

import "fmt"

// A tournament system decides what games to play, and records results
//
// All methods are called in a synchronised context, and do not have
// to be thread-safe.
type System interface {
	fmt.Stringer
	// Register a client as ready
	Ready(*Tournament, *Client)
	// Mark a client as dead
	Forget(*Tournament, *Client)
	// Record the outcome of a game
	Record(*Tournament, *Game)
	// Check if a tournament is over
	Over(*Tournament) bool
}

// roundRobin tournaments let every participant play with every other
// participant.
type roundRobin struct {
	// Board size for this tournament
	size uint
	// Set of games that we are expecting to play
	games map[*Game]struct{}
	// Set of clients that are ready to play a game
	ready []*Client
}

// Generate a name for the current tournament
func (rr *roundRobin) String() string {
	return fmt.Sprintf("round-robin-%d", rr.size)
}

// Mark a client as ready and attempt to start a game
func (rr *roundRobin) Ready(t *Tournament, cli *Client) {
	if rr.games == nil {
		rr.games = make(map[*Game]struct{})
		for _, a := range t.participants {
			for _, b := range t.participants {
				if a == b {
					continue
				}
				rr.games[&Game{
					Board: makeBoard(rr.size, rr.size),
					North: a,
					South: b,
				}] = struct{}{}
			}
		}
	}

	// Loop over all the ready clients to check if we still need
	// to organise a game between the new client and someone else.
	for i, ilc := range rr.ready {
		for game := range rr.games {
			b1 := game.North == cli && game.South == ilc
			b2 := game.North == ilc && game.South == cli

			// In case the new client and an existing
			// client both still have to play a game, we
			// will remove the waiting client from the
			// ready list and start the game
			if b1 || b2 {
				// Slice trick: "Delete without
				// preserving order (GC)"
				rr.ready[i] = rr.ready[len(rr.ready)-1]
				rr.ready[len(rr.ready)-1] = nil
				rr.ready = rr.ready[:len(rr.ready)]
				delete(rr.games, game)

				t.start <- game
				return
			}
		}
	}

	// If the client didn't find a match, mark it as ready and do
	// nothing more.
	rr.ready = append(rr.ready, cli)
}

// Remove all games that CLI should have participated in
func (rr roundRobin) Forget(_ *Tournament, cli *Client) {
	for game := range rr.games {
		if game.North == cli || game.South == cli {
			delete(rr.games, game)
		}
	}
}

// The result of a game is not relevant for round robin
func (*roundRobin) Record(*Tournament, *Game) {}

// A round robin tournament is over as soon as everyone has played a
// game against every other participant.  For n participants, this
// means every one has had n-1 games, ie. there have been n-1 rounds.
func (rr *roundRobin) Over(t *Tournament) bool {
	return rr.games != nil && len(rr.games) == 0
}
