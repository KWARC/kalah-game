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
	"sync"
)

const (
	MAX_DIFF = 700
	EPS      = 0.0001
	K        = 10
)

var OutcomeToPoints = map[Outcome]float64{
	WIN:  1.0,
	DRAW: 0.5,
	LOSS: 0.0,
}

func (g *Game) updateScore() (err error) {
	diff := g.North.Score - g.South.Score
	if math.Abs(diff) > MAX_DIFF {
		return nil
	}

	// Calculate the new ELO rating for the current client
	// according to
	// https://de.wikipedia.org/wiki/Elo-Zahl#Erwartungswert
	ea := 1 / (1 + math.Pow(10, diff/MAX_DIFF))
	eb := 1 / (1 + math.Pow(10, -diff/MAX_DIFF))

	if math.Abs((ea+eb)-1) > EPS {
		log.Printf("Numerical instability detected: %f + %f = %f != 1.0", ea, eb, ea+eb)
		return nil
	}

	outcome := g.Board.Outcome(SideSouth)
	g.South.Score += K * (OutcomeToPoints[outcome] - ea)
	g.South.Score = math.Max(0, g.South.Score)
	g.North.Score += K * (1 - OutcomeToPoints[outcome] - eb)
	g.North.Score = math.Max(0, g.North.Score)

	// Send database manager a request to update the entry
	var wait sync.WaitGroup
	wait.Add(3)
	g.dbact <- g.South.updateDatabase(&wait, false)
	g.dbact <- g.North.updateDatabase(&wait, false)
	g.dbact <- g.updateDatabase(&wait)
	wait.Wait()

	return nil
}
