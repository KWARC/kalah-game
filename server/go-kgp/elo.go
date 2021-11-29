package main

import (
	"log"
	"math"
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
	if cli.token == "" {
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

	// Send database manager a request to update the entry
	dbact <- cli.updateDatabase(nil)

	return nil
}
