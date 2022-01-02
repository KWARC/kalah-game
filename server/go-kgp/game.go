// Game Coordinator
//
// Copyright (c) 2021  Philip Kaludercic
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
	"sync/atomic"
	"time"
)

type Outcome uint8

const (
	ONGOING = iota
	WIN
	DRAW
	LOSS
	RESIGN
)

// Move is an Action to set the next move
type Move struct {
	Pit     int
	Client  *Client
	Comment string
	Yield   bool
	id      uint64
	when    time.Time
}

// Game represents a game between two players
type Game struct {
	Board Board
	// The ID of the last state command, used to verify if a
	// move/yield response should be ignored or not in freeplay
	// mode.
	last uint64
	// The side of the board that is currently deciding to make a
	// move.  See .North and .South.
	side Side
	// The control channel that is used to send actions like move
	// or yield.  These are processed in .Start().
	move  chan<- *Move
	death chan<- *Client
	// The two clients
	North   *Client
	South   *Client
	nchoice int
	schoice int
	// Is this game logged in the database?
	logged bool
	// Data for the web interface.
	//
	// These fields are usually empty, unless a Game object has
	// been queried from the database and passed on to a template.
	Id      int64
	Moves   []*Move
	Outcome Outcome // For south
	Ended   *time.Time
	Started *time.Time
}

// String generates a KGP board representation for the current player
func (g *Game) String() string {
	if g.side == SideNorth {
		return g.Board.Mirror().String()
	}
	return g.Board.String()
}

func (g *Game) IsOver() bool {
	return g.Board.Over()
}

// Player returns the player on SIDE of the board
func (g *Game) Player(side Side) *Client {
	switch side {
	case SideNorth:
		return g.North
	case SideSouth:
		return g.South
	default:
		panic("Invalid state")
	}
}

func (g *Game) Side(cli *Client) Side {
	switch cli {
	case g.North:
		return SideNorth
	case g.South:
		return SideSouth
	default:
		panic("Unknown client")
	}
}

// Current returns the player who's turn it is
func (g *Game) Current() *Client {
	return g.Player(g.side)
}

// IsCurrent returns true, if CLI the game is currently waiting for
// CLI to answer
func (g *Game) IsCurrent(cli *Client) bool {
	if g == nil {
		return false
	}

	return g.Current() == cli
}

// Other returns the opponent of CLI, or nil if CLI is not playing a
// game
func (g *Game) Other(cli *Client) *Client {
	if g == nil {
		return nil
	}
	switch cli {
	case g.North:
		if g.North.game == nil {
			return nil
		}
		return g.South
	case g.South:
		if g.South.game == nil {
			return nil
		}
		return g.North
	default:
		panic(fmt.Sprintf("%s is not part of %s", cli, g))
	}
}

// Start manages a game between the north and south client
func (g *Game) Start() {
	move := make(chan *Move)
	death := make(chan *Client)
	g.move = move
	g.death = death

	if g.North.game != nil {
		panic("Already part of game")
	}
	g.North.game = g
	if g.South.game != nil {
		panic("Already part of game")
	}
	g.South.game = g

	log.Printf("Start game (%d, %d) between %s and %s",
		len(g.Board.northPits), g.Board.init,
		g.North, g.South)

	g.side = SideSouth
	g.last = g.South.Send("state", g)

	timer := time.NewTimer(time.Duration(conf.Game.Timeout) * time.Second)

	if g.North.token != nil && g.South.token != nil {
		g.logged = true
	}

	for {
		var (
			choice *Move
			next   bool
		)

		select {
		case m := <-move:
			if m.Yield {
				if m.Client != g.Current() {
					break
				}
				// The client has indicated it does not intend
				// to use the remaining time.
				next = true
			} else if m.Client.simple && m.Client.pending >= 1 {
				// If the client has sent us a move even
				// though he has not responded to a previous
				// "stop" command via "yield" we must conclude
				// that the client has misunderstood the
				// communication or is too slow.
			} else if !g.Board.Legal(g.side, m.Pit) {
				m.Client.Error(m.id, fmt.Sprintf("Illegal move %d", m.Pit+1))
			} else {
				m.when = time.Now()
				choice = m
			}
		case cli := <-death:
			if g.North != cli && g.South != cli {
				log.Print("Unrelated death")
				return
			}
			opp := g.Other(cli)

			// Leave enough time for the queue to be
			// updated and all traces of the opponent to
			// be removed.
			time.Sleep(time.Second)

			if conf.Endless {
				if g.Current() == opp {
					opp.Respond(g.last, "stop")
				}
				opp.game = nil
				enqueue <- opp
			} else {
				opp.killFunc()
			}

			return
		case <-timer.C:
			// The time allocated for the current player
			// is over, and we proceed to the next round.
			next = true
		}

		if g.IsOver() {
			break
		}

		if next {
			g.Current().Respond(g.last, "stop")
			atomic.AddInt64(&g.Current().pending, 1)

			if choice.Yield {
				g.side = !g.side
			} else {

				// We generate a random move to replace
				// whatever the current choice is, either if
				// no choice was made (denoted by a -1) or if
				// the client is playing in simple mode and
				// there are pending stop requests that have
				// to be responded to with a yield
				if choice == nil || (g.Current().simple && g.Current().pending > 0) {
					choice.Pit = g.Board.Random(g.side)
				}

				g.Moves = append(g.Moves, choice)

				again := g.Board.Sow(g.side, choice.Pit)
				if g.Board.Over() {
					break
				}

				if !again {
					g.side = !g.side
				}
			}

			g.last = g.Current().Send("state", g)

			timer.Reset(time.Duration(conf.Game.Timeout) * time.Second)
		}
	}

	if g.logged {
		g.updateScore()
	}

	if conf.Endless {
		// In the "endless" mode, the client is just
		// added back to the waiting queue as soon as
		// the game is over.
		if g.North.game == g {
			g.North.game = nil
			enqueue <- g.North
		}
		if g.South.game == g {
			g.South.game = nil
			enqueue <- g.South
		}
	} else {
		g.North.killFunc()
		g.South.killFunc()
	}
}
