package main

import (
	"fmt"
	"log"
	"sync"
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
	game    *Game
	id      uint64
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
	yield chan<- *Client
	move  chan<- *Move
	// The two clients
	North   *Client
	South   *Client
	nchoice int
	schoice int
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

func (g *Game) choice() *int {
	switch g.side {
	case SideNorth:
		return &g.nchoice
	case SideSouth:
		return &g.schoice
	default:
		panic("Invalid state")
	}
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
	yield := make(chan *Client)
	move := make(chan *Move)
	g.yield = yield
	g.move = move

	if g.North.game != nil {
		panic("Already part of game")
	}
	g.North.game = g
	g.nchoice = -1
	if g.South.game != nil {
		panic("Already part of game")
	}
	g.South.game = g
	g.schoice = -1

	log.Printf("Start game between %s and %s", g.North, g.South)

	g.side = SideSouth
	g.last = g.South.Send("state", g)

	timer := time.NewTimer(time.Duration(conf.Game.Timeout) * time.Second)

	defer func() {
		fmt.Println("Game", g, "finished")

		if g.North.token != "" && g.South.token != "" {
			g.North.updateScore(g.South, g.Board.Outcome(SideNorth))
			g.South.updateScore(g.North, g.Board.Outcome(SideSouth))
		}

		var wait sync.WaitGroup
		wait.Add(1)
		dbact <- g.updateDatabase(&wait)
		wait.Wait()

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

	dbact <- g.updateDatabase(nil)

	for {
		next := false
		select {
		case m := <-move:
			if m.Client != g.Current() {
				m.Client.Error(m.id, "Not your turn")
			} else if !g.Board.Legal(g.side, m.Pit) {
				m.Client.Error(m.id, fmt.Sprintf("Illegal move %d", m.Pit+1))
			} else {
				*g.choice() = m.Pit
			}
		case cli := <-yield:
			if cli != g.Current() {
				break
			}
			// The client has indicated it does not intend
			// to use the remaining time.
			next = true
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

			choice := *g.choice()
			dbact <- saveMove(g, g.Current(), g.side, choice)

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

			if !again {
				g.side = !g.side
			}

			*g.choice() = -1
			g.last = g.Current().Send("state", g)

			timer.Reset(time.Duration(conf.Game.Timeout) * time.Second)
		}
	}
}
