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

// An Action is sent from a client to game to change the latters state
type Action interface {
	// Returns true if the game should proceed to the next round
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
		dbact <- m.updateDatabase
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
	ctrl chan<- Action
	// The two clients
	North *Client
	South *Client
	// Data for the web interface.
	//
	// These fields are usually empty, unless a Game object has
	// been queried from the database and passed on to a template.
	Id      int64
	Moves   []Move
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
	return g.Outcome != ONGOING
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
	ctrl := make(chan Action)
	g.ctrl = ctrl

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

	defer func() {
		fmt.Println("Game", g, "finished")

		g.North.updateScore(g.South, g.Board.Outcome(SideNorth))
		g.South.updateScore(g.North, g.Board.Outcome(SideSouth))

		dbact <- g.updateDatabase

		if conf.Endless {
			// In the "endless" mode, the client is just
			// added back to the waiting queue as soon as
			// the game is over.
			g.North.game = nil
			g.South.game = nil
			enqueue <- g.North
			enqueue <- g.South
		} else {
			g.North.killFunc()
			g.South.killFunc()
		}
	}()

	dbact <- g.updateDatabase

	g.Current().choice = -1
	for {
		next := false
		select {
		case act := <-ctrl:
			next = act.Do(g, g.side)
		case <-timer:
			// the timer is delaying the game
			next = true
		}

		if g.IsOver() {
			break
		}

		if next {
			choice := g.Current().choice

			// We generate a random move to replace
			// whatever the current choice is, either if
			// no choice was made (denoted by a -1) or if
			// the client is playing in simple mode and
			// there are pending stop requests that have
			// to be responded to with a yield
			if choice == -1 || (g.Current().simple && g.Current().pending > 0) {
				choice = g.Board.Random(g.side)
			}
			again := g.Board.Sow(g.side, choice)
			if g.Board.Over() {
				return
			}

			g.Current().Respond(g.last, "stop")
			atomic.AddInt64(&g.Current().pending, 1)

			if !again {
				g.side = !g.side
			}

			g.Current().Send("state", g)
			g.Current().choice = -1

			timer = time.After(time.Duration(conf.Game.Timeout) * time.Second)
		}
	}
}
