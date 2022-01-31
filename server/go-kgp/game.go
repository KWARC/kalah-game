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
	_ = iota
	WIN
	DRAW
	LOSS
	RESIGN
)

func (o Outcome) String() string {
	switch o {
	case WIN:
		return "Win"
	case DRAW:
		return "Draw"
	case LOSS:
		return "Loss"
	case RESIGN:
		return "Resign"
	default:
		return "???"
	}
}

// Move is an Action to set the next move
type Move struct {
	Pit     int
	Client  *Client
	Comment string
	Yield   bool
	id, ref uint64
	when    time.Time
	State   *Board
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
	// or yield.  These are processed in .Play().
	move  chan<- *Move
	death chan<- *Client
	// The two clients
	North *Client
	South *Client
	// Is this game logged in the database?
	logged bool
	// Data for the web interface.
	//
	// These fields are usually empty, unless a Game object has
	// been queried from the database and passed on to a template.
	Id int64
	// List of moves made in a game, in order of occurance
	Moves []*Move
	// The result of the game (from south's perspective)
	Outcome Outcome
	// Number of moves made in a game (used only for database
	// queries, otherwise Moves is used)
	MoveCount int64
}

// String generates a KGP board representation for the current player
func (g *Game) String() string {
	if g.side == SideNorth {
		return g.Board.Mirror().String()
	}
	return g.Board.String()
}

// IsOver checks if the current board is in a final state
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

// Return the side CLI is playing as
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
func (g *Game) IsCurrent(cli *Client, ref uint64) bool {
	if g == nil {
		return false
	}
	if cli.simple && cli.nstop != cli.nyield {
		// If the client has sent us a move even
		// though he has not responded to a previous
		// "stop" command via "yield" we must conclude
		// that the client has misunderstood the
		// communication or is too slow.
		return false
	}
	if g.Current() != cli {
		return false
	}
	return atomic.LoadUint64(&g.last) == ref || ref == 0
}

// Other returns the opponent of CLI, or nil if CLI is not playing a
// game
func (g *Game) Other(cli *Client) *Client {
	if g == nil {
		return nil
	}
	switch cli {
	case g.North:
		return g.South
	case g.South:
		return g.North
	default:
		panic(fmt.Sprintf("%s is not part of %s", cli, g))
	}
}

// Return the result and who it relates to
func (g *Game) Result() (Outcome, *Client) {
	switch g.Outcome {
	case WIN:
		return LOSS, g.North
	case LOSS:
		return LOSS, g.South
	case DRAW:
		return DRAW, nil
	case RESIGN:
		return RESIGN, g.Current()
	default:
		panic("Request result of an active game")
	}
}

// Semaphore-like channel to limit the number of concurrent games
//
// If nil (as by default), there is no upper bound.  This variable is
// initialised in main according to conf.Game.Slots.
var slots chan struct{}

// Send the current side a state command
func (g *Game) SendState() {
	if g.Current() == nil {
		return
	}
	atomic.StoreUint64(&g.last, g.Current().Send("state", g))
	g.Current().lock.Lock()
	g.Current().games[g.last] = g
	g.Current().lock.Unlock()
}

// Play manages a game between the north and south client
//
// If the return value is non-nil, it returns the client that died or
// resigned, leading to a premature end of the game.
func (g *Game) Play() *Client {
	atomic.AddUint64(&playing, 2)
	if slots != nil {
		// Attempt to reserve a slot
		<-slots
	}

	defer func() {
		// Indicate an available slot
		if slots != nil {
			slots <- struct{}{}
		}

		// Update game data
		g.Outcome = g.Board.Outcome(SideSouth)
		debug.Printf("%s vs. %s (%s): %s", g.South, g.North, g, g.Outcome)

		// Remove all ID references from both clients
		g.North.Forget(g)
		g.South.Forget(g)

		// Decrement the number of active games
		atomic.AddUint64(&playing, ^uint64(1))

		// Save the game (along with the moves) in the
		// database
		if g.logged {
			g.updateDatabase()
		}
	}()

	move := make(chan *Move)
	death := make(chan *Client, 2)
	g.move = move
	g.death = death

	if g.North != nil && g.North.simple {
		if g.North.game != nil {
			panic(fmt.Sprintf("Already %s part of game %s (%s, %s) while entering %s (%s, %s)",
				g.North,
				g.North.game, g.North.game.South, g.North.game.North,
				g, g.South, g.North))
		}
		g.North.game = g
	}
	if g.South != nil && g.South.simple {
		if g.South.game != nil {
			panic(fmt.Sprintf("Already %s part of game %s (%s, %s) while entering %s (%s, %s)",
				g.South,
				g.South.game, g.South.game.South, g.South.game.South,
				g, g.South, g.North))
		}
		g.South.game = g
	}

	if (g.North == nil || g.North.token != nil) && (g.South == nil || g.South.token != nil) {
		g.logged = true
		g.updateDatabase()
	}

	g.side = SideSouth
	g.SendState()

	timer := time.NewTimer(time.Duration(conf.Game.Timeout) * time.Second)

	var choice *Move
	for !g.Board.Over() {
		var next bool

		// Random move generator
		//
		// If a client is nil, we interpret it as a random
		// move client.
		if g.Current() == nil {
			choice := &Move{Pit: g.Board.Random(g.side)}
			g.Moves = append(g.Moves, choice)

			if !g.Board.Sow(g.side, choice.Pit) {
				g.side = !g.side
			}
			if g.Current() != nil {
				g.SendState()
			}

			continue
		}

		select {
		case m := <-move:
			if !g.IsCurrent(m.Client, m.ref) {
				break
			}
			if m.Yield {
				// The client has indicated it does not intend
				// to use the remaining time.
				next = true
			} else if !g.Board.Legal(g.side, m.Pit) {
				m.Client.Error(m.id, fmt.Sprintf("Illegal move %d", m.Pit+1))
			} else {
				m.when = time.Now()
				m.Comment = m.Client.comment
				m.Client.comment = ""
				choice = m
			}
		case cli := <-death:
			if cli == nil {
				g.Current().Respond(g.last, "stop")
				return nil
			}
			if g.North != cli && g.South != cli {
				log.Print("Unrelated death")
				return cli
			}
			opp := g.Other(cli)

			// Leave enough time for the queue to be
			// updated and all traces of the opponent to
			// be removed.
			time.Sleep(time.Second)

			if g.Current() == opp {
				opp.Respond(g.last, "stop")
			}
			return cli
		case <-timer.C:
			// The time allocated for the current player
			// is over, and we proceed to the next round.
			next = true
		}

		if next {
			g.Current().Respond(g.last, "stop")
			atomic.AddUint64(&g.Current().nstop, 1)

			// We generate a random move to replace
			// whatever the current choice is, either if
			// no choice was made (denoted by a -1) or if
			// the client is playing in simple mode and
			// there are pending stop requests that have
			// to be responded to with a yield
			if choice == nil || (g.Current().simple && g.Current().nstop != g.Current().nyield) {
				choice = &Move{
					Client:  g.Current(),
					Pit:     g.Board.Random(g.side),
					when:    time.Now(),
					Comment: "Timeout. Move randomly generated by server",
				}
				debug.Printf("%s made no move, randomly chose %d", g.Current(), choice.Pit)
			}

			for {
				again := g.Board.Sow(g.side, choice.Pit)
				if g.Board.Over() {
					goto over
				}
				g.Moves = append(g.Moves, choice)

				if !again {
					g.side = !g.side
				}

				count, last := g.Board.Moves(g.side)
				if count == 0 {
					// g.Board.Over must be broken
					panic("No moves even though game is not over")
				} else if count == 1 && conf.Game.SkipTriv {
					// Skip trivial moves
					choice = &Move{
						Client: g.Current(),
						Pit:    last,
						when:   time.Now(),
					}
				} else {
					break
				}
			}

			g.SendState()
			timer.Reset(time.Duration(conf.Game.Timeout) * time.Second)
			choice = nil
		}
	}
over:

	return nil
}
