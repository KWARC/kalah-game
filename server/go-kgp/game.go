package main

import (
	"fmt"
	"log"
	"time"
)

// An Action is sent from a client to game to change the latters state
type Action interface {
	Do(*Game, Side) bool
}

// Move is an Action to set the next move
type Move int64

// Do ensures a move is valid and then sets it
func (m Move) Do(game *Game, side Side) bool {
	if m < 0 || m >= Move(len(game.board.northPits)) {
		game.Current().Send("error", "illegal move")
	} else {
		game.choice = m
	}
	return false
}

// Yield is an Action to give up the remaining time.  If Yield wrapps
// a true bool value, the entire game is cancelled
type Yield bool

// Do gives up the remaining time
func (y Yield) Do(g *Game, _ Side) bool {
	if y == true {
		g.north.Send("fail", "game over")
		g.south.Send("fail", "game over")
		g.dead = true
	}
	return true
}

// Game represents a game between two players
type Game struct {
	board  Board
	last   uint64 // id of last state command
	choice Move
	side   Side
	ctrl   chan Action
	north  *Client
	south  *Client
	dead   bool
}

// String generates a KGP board representation for the current player
func (g *Game) String() string {
	if g.side == SideSouth {
		return g.board.Mirror().String()
	}
	return g.board.String()
}

// Current returns the player who's turn it is
func (g *Game) Current() *Client {
	switch g.side {
	case SideNorth:
		return g.north
	case SideSouth:
		return g.south
	default:
		panic("Invalid state")
	}
}

// IsCurrent returns true, if CLI the game is currently waiting for
// CLI to answer
func (g *Game) IsCurrent(cli *Client) bool {
	if g == nil {
		return false
	}

	return g.Current() == cli
}

// Start beings coordinating a game between NORTH and SOUTH
func (g *Game) Start() {
	g.ctrl = make(chan Action)

	if g.north.game != nil {
		panic("Already part of game")
	}
	g.north.game = g
	g.north.waiting = false
	if g.south.game != nil {
		panic("Already part of game")
	}
	g.south.game = g
	g.south.waiting = false

	size := len(g.board.northPits)
	g.north.Send("init", size)
	g.south.Send("init", size)
	time.Sleep(time.Duration(warmup) * time.Second)
	g.last = g.north.Send("state", g)

	timer := time.After(time.Duration(timeout) * time.Second)
	next := false

	defer close(g.north.input)
	defer close(g.south.input)

	for {
		select {
		case act := <-g.ctrl:
			next = act.Do(g, g.side)
		case <-timer:
			next = true
		}

		if g.dead {
			break
		}

		if next {
			if g.board.Over() {
				return
			} else if !g.board.Legal(g.side, uint(g.choice)) {
				g.choice = g.board.Random(g.side)
				msg := fmt.Sprintf("No move legal move, defaulted to %d", g.choice)
				if g.side == SideNorth {
					g.north.Respond(g.last, "error", msg)
				} else {
					g.south.Respond(g.last, "error", msg)
				}
			}

			again := g.board.Sow(g.side, uint(g.choice))

			g.choice = Move(-1)
			if g.side == SideNorth {
				g.north.Respond(g.last, "stop")
				g.last = g.south.Send("state", g)
			} else {
				g.south.Respond(g.last, "stop")
				g.last = g.north.Send("state", g)
			}

			if !again {
				g.side = !g.side
			}

			timer = time.After(time.Duration(timeout) * time.Second)
		}
	}

	log.Println("Finished game", g)
}
