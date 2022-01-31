// ELO Ranking calculation
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
	"log"
	"math"
)

const (
	MAX_DIFF = 700
	EPS      = 0.0001
	K        = 20
)

// Update the score of clients in game using the Elo system
func (g *Game) updateElo() (err error) {
	if g.North.token == nil || g.South.token == nil {
		return nil
	}

	diff := g.North.Score - g.South.Score

	// Calculate the new ELO rating for the current client
	// according to
	// https://de.wikipedia.org/wiki/Elo-Zahl#Erwartungswert
	ea := 1 / (1 + math.Pow(10, diff/MAX_DIFF))
	eb := 1 / (1 + math.Pow(10, -diff/MAX_DIFF))

	if math.Abs((ea+eb)-1) > EPS {
		log.Printf("Numerical instability detected: %f + %f = %f != 1.0", ea, eb, ea+eb)
		return nil
	}

	if g.Outcome == RESIGN {
		if g.Current() != g.North {
			g.South.Score += K * ea
			g.North.Score += K * -eb
		} else {
			g.South.Score += K * -ea
			g.North.Score += K * eb
		}
	} else {
		var points float64

		switch g.Outcome {
		case WIN:
			points = conf.Game.Win
		case DRAW:
			points = conf.Game.Draw
		case LOSS:
			points = conf.Game.Loss
		}

		g.South.Score += K * (points - ea)
		g.North.Score += K * (1 - points - eb)
	}
	g.South.Score = math.Max(0, g.South.Score)
	g.North.Score = math.Max(0, g.North.Score)

	// Send database manager a request to update the entry
	g.South.updateDatabase(false)
	g.North.updateDatabase(false)

	return nil
}
