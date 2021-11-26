package main

import (
	"fmt"
	"log"
	"time"
)

type Outcome uint8

const (
	_ = iota
	WIN
	DRAW
	LOSS
)

// An Action is sent from a client to game to change the latters state
type Action interface {
	Do(*Game, Side) bool
}

// Move is an Action to set the next move
type Move struct {
	pit  int
	cli  *Client
	game *Game
	comm string
}

// Do ensures a move is valid and then sets it
func (m Move) Do(game *Game, side Side) bool {
	if !game.Board.Legal(side, m.pit) {
		game.Current().Send("error", fmt.Sprintf("Illegal move %d", m.pit+1))
	} else {
		game.Player(side).choice = m.pit
		dbact <- m.UpdateDatabase
	}
	return false
}

// Yield is an Action to give up the remaining time.
type Yield struct{}

// Do gives up the remaining time
func (y Yield) Do(g *Game, side Side) bool {
	return true
}

// Game represents a game between two players
type Game struct {
	Board  Board
	last   uint64 // id of last state command
	side   Side
	ctrl   chan Action
	North  *Client
	South  *Client
	Result bool
	Id     int64
	start  time.Time
	IsOver bool
	Moves  []Move
}

// String generates a KGP board representation for the current player
func (g *Game) String() string {
	if g.side == SideNorth {
		return g.Board.Mirror().String()
	}
	return g.Board.String()
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
	g.ctrl = make(chan Action)

	if g.North.game != nil {
		panic("Already part of game")
	}
	g.North.game = g
	if g.South.game != nil {
		panic("Already part of game")
	}
	g.South.game = g

	log.Printf("Start game between %s and %s", g.North, g.South)

	g.side = SideSouth
	g.last = g.South.Send("state", g)
	g.Current().choice = -1

	timer := time.After(time.Duration(conf.Game.Timeout) * time.Second)
	next := false

	defer func() {
		fmt.Println("Game", g, "finished")

		g.North.updateScore(g.South, g.Board.Outcome(SideNorth))
		g.South.updateScore(g.North, g.Board.Outcome(SideSouth))

		dbact <- g.UpdateDatabase

		if conf.Endless {
			// In the "endless" mode, the client is just
			// added back to the waiting queue as soon as
			// the game is over.
			g.North.game = nil
			g.South.game = nil
			enqueue(g.North)
			enqueue(g.South)
		} else {
			g.North.killFunc()
			g.South.killFunc()
		}
	}()

	dbact <- g.UpdateDatabase

	for {
		select {
		case act := <-g.ctrl:
			next = act.Do(g, g.side)
		case <-timer:
			// the timer is delaying the game
			next = true
		}

		if g.IsOver {
			break
		}

		if next {
			choice := g.Current().choice
			g.Current().choice = -1

			if g.Board.Over() {
				return
			} else if !g.Board.Legal(g.side, choice) {
				oldchoice := choice
				choice = g.Board.Random(g.side)
				msg := fmt.Sprintf("Move %d illegal, used %d",
					oldchoice, choice)
				if g.side == SideNorth {
					g.North.Respond(g.last, "error", msg)
				} else {
					g.South.Respond(g.last, "error", msg)
				}
			}

			again := g.Board.Sow(g.side, choice)

			if g.side == SideNorth {
				g.North.Respond(g.last, "stop")
			} else {
				g.South.Respond(g.last, "stop")
			}

			if !again {
				g.side = !g.side
			}

			if g.side == SideNorth {
				g.last = g.North.Send("state", g)
			} else {
				g.last = g.South.Send("state", g)
			}

			timer = time.After(time.Duration(conf.Game.Timeout) * time.Second)
		}
	}
}
