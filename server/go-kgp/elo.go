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
	MAX_DIFF = 400
	EPS      = 0.0001
	K        = 20
)

var OutcomeToPoints = map[Outcome]float64{
	WIN:  1.0,
	DRAW: 0.5,
	LOSS: 0.0,
}

func (cli *Client) updateScore(opp *Client, outcome Outcome) (err error) {
	if cli.token == nil {
		panic("Cannot calculate score for anonymous agent")
	}

	// Calculate the new ELO rating for the current client
	// according to
	// https://de.wikipedia.org/wiki/Elo-Zahl#Erwartungswert
	diff := math.Max(-400, math.Min(opp.Score-cli.Score, 400))

	ea := 1 / (1 + math.Pow(10, diff/MAX_DIFF))
	eb := 1 / (1 + math.Pow(10, -diff/MAX_DIFF))

	if math.Abs((ea+eb)-1) > EPS {
		log.Printf("Numerical instability detected: %f + %f = %f != 1.0", ea, eb, ea+eb)
		return nil
	}

	log.Printf("Change %s score by %g", cli, K*(OutcomeToPoints[outcome]-ea))
	cli.Score = cli.Score + K*(OutcomeToPoints[outcome]-ea)
	opp.Score = opp.Score + K*(1-OutcomeToPoints[outcome]-eb)

	// Send database manager a request to update the entry
	var wait sync.WaitGroup
	wait.Add(1)
	dbact <- cli.updateDatabase(&wait)
	wait.Wait()

	return nil
}
