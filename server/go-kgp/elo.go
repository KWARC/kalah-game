package main

import (
	"log"
	"math"
)

type Outcome float64

const (
	MAX_DIFF = 400
	EPS      = 0.0001
	K        = 20

	WIN  = 1.0
	DRAW = 0.5
	LOSS = 0.0
)

func (cli *Client) updateScore(opp *Client, outcome Outcome) (err error) {
	// Calculate the new ELO rating for the current client
	// according to
	// https://de.wikipedia.org/wiki/Elo-Zahl#Erwartungswert
	diff := math.Max(-400, math.Min(opp.score-cli.score, 400))

	ea := 1 / (1 + math.Pow(10, diff/MAX_DIFF))
	eb := 1 / (1 + math.Pow(10, -diff/MAX_DIFF))

	if math.Abs((ea+eb)-1) > EPS {
		log.Printf("Numerical instability detected: %f + %f = %f != 1.0", ea, eb, ea+eb)
		return nil
	}

	cli.score = cli.score + K*(float64(outcome)-ea)

	// Send database manager a request to update the entry
	dbact <- cli.UpdateDatabase(nil)

	return nil
}
