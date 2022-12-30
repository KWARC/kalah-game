// Game Model
//
// Copyright (c) 2021, 2022  Philip Kaludercic
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

package game

import (
	"context"
	"time"

	"go-kgp"
	"go-kgp/conf"
)

func Move(g *kgp.Game, m *kgp.Move) bool {
	if !g.Board.Legal(g.Current, m.Choice) {
		return false
	}

	if g.Current != g.Side(m.Agent) {
		panic("Unexpected side")
	}
	repeat := g.Board.Sow(g.Current, m.Choice)
	if !repeat {
		g.Current = !g.Current
	}
	return true
}

func MoveCopy(g *kgp.Game, m *kgp.Move) (*kgp.Game, bool) {
	c := &kgp.Game{
		Board:   g.Board.Copy(),
		South:   g.South,
		North:   g.North,
		Current: g.Current,
	}
	return c, Move(c, m)
}

func Play(g *kgp.Game, conf *conf.Conf) {
	dbg := conf.Debug.Printf
	bg := context.Background()

	g.State = kgp.ONGOING
	conf.DB.SaveGame(bg, g)
	for !g.Board.Over() {
		var m *kgp.Move

		count, last := g.Board.Moves(g.Current)
		dbg("Game %d: %s has %d moves",
			g.Id, g.State.String(), count)
		switch count {
		case 0:
			// If this happens, then Board.Over or
			// Board.Moves must be broken.
			panic("No moves even though game is not over")
		case 1:
			// Skip trivial moves
			m = &kgp.Move{
				Agent:   g.Active(),
				Comment: "[Auto-move]",
				Choice:  last,
				Game:    g,
				Stamp:   time.Now(),
			}
		default:
			var resign bool
			m, resign = g.Active().Request(g)
			if resign {
				dbg("Game %d: %s resigned", g.Id, g.Current)

				switch g.Current {
				case kgp.South:
					g.State = kgp.SOUTH_RESIGNED
				case kgp.North:
					g.State = kgp.NORTH_RESIGNED
				}

				goto save
			}
		}
		dbg("Game %d: %s made the move %d (%s)",
			g.Id, g.State.String(), m.Choice, m.Comment)

		side := g.Current
		if !Move(g, m) {
			dbg("Game %d: %s made illegal move %d",
				g.Id, g.Current, m.Choice)

			switch side {
			case kgp.South:
				g.State = kgp.SOUTH_RESIGNED
			case kgp.North:
				g.State = kgp.NORTH_RESIGNED
			}
			goto save
		}

		// Save the move in the database, and take as much
		// time as necessary.
		conf.DB.SaveMove(bg, m)
		dbg("Game %d: %s", g.Id, g.State.String())
	}

	switch g.Board.Outcome(kgp.South) {
	case kgp.WIN, kgp.LOSS:
		if g.Board.Store(kgp.North) > g.Board.Store(kgp.South) {
			g.State = kgp.NORTH_WON
		} else {
			g.State = kgp.SOUTH_WON
		}
	case kgp.DRAW:
		g.State = kgp.UNDECIDED
	}
save:
	conf.DB.SaveGame(bg, g)
	conf.Debug.Printf("Game %d finished (%s)", g.Id, &g.State)
}
