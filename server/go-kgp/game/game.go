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
	"go-kgp/sched"
)

type coord struct{ conf *conf.Conf }

func (*coord) String() string { return "Coordinator" }

func (m *coord) Start() {
	for game := range m.conf.Play {
		m.conf.Debug.Println("Starting", game)
		go Play(game, m.conf)
	}
}

func (m *coord) Shutdown() {}

func Prepare(config *conf.Conf) {
	var man conf.GameManager

	switch config.Mode {
	case conf.MODE_OPEN:
		man = sched.MakeRandom(config)
	case conf.MODE_TOURNAMENT:
		panic("Not implemented")
	}
	config.Register(man)
	config.Register(&coord{config})
}

func Move(g *kgp.Game, m *kgp.Move) bool {
	if !g.State.Legal(g.Current, m.Choice) {
		return false
	}

	if g.Current != g.Side(m.Agent) {
		panic("Unexpected side")
	}
	repeat := g.State.Sow(g.Current, m.Choice)
	if !repeat {
		g.Current = !g.Current
	}
	return true
}

func MoveCopy(g *kgp.Game, m *kgp.Move) (*kgp.Game, bool) {
	c := &kgp.Game{
		State:   g.State.Copy(),
		South:   g.South,
		North:   g.North,
		Current: g.Current,
	}
	return c, Move(c, m)
}

func Play(g *kgp.Game, conf *conf.Conf) {
	dbg := conf.Debug.Printf
	bg := context.Background()

	conf.DB.SaveGame(bg, g)

	g.Outcome = kgp.ONGOING
	for !g.State.Over() {
		var m *kgp.Move

		count, last := g.State.Moves(g.Current)
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

				g.Outcome = kgp.RESIGN
				goto save
			}
		}
		dbg("Game %d: %s made the move %d (%s)",
			g.Id, g.State.String(), m.Choice, m.Comment)

		if !Move(g, m) {
			dbg("Game %d: %s made illegal move %d",
				g.Id, g.Current, m.Choice)

			// TODO: Assign a more accurate outcome
			g.Outcome = kgp.RESIGN
			goto save
		}

		// Save the move in the database, and take as much
		// time as necessary.
		conf.DB.SaveMove(bg, m)
		dbg("Game %d: %s", g.Id, g.State.String())
	}

	g.Outcome = g.State.Outcome(kgp.South)
save:
	conf.DB.SaveGame(bg, g)
	conf.Debug.Printf("Game %d finished (%s)", g.Id, g.Outcome)

	if g.South != nil {
		conf.GM.Schedule(g.South)
	}
	if g.North != nil {
		conf.GM.Schedule(g.North)
	}
}
