// Primitive MinMax Agent
//
// Copyright (c) 2022, 2023  Philip Kaludercic
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
// License, version 3, along with go-kgp . If not, see
// <http://www.gnu.org/licenses/>

package bot

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"go-kgp"
)

// We need a random
var nonce string = os.Getenv("NONCE")

type minmax struct {
	depth uint      // ply cutoff
	acc   float64   // accuracy
	user  *kgp.User // database entry
}

func search(Σ *kgp.Board, π kgp.Side, Δ uint) (uint, int64) {
	λ, _ := Σ.Type()
	var it func(*kgp.Board, kgp.Side, uint, int64, int64) (uint, int64)

	// NOTE: The usage of Alpha-Beta Pruning in this
	// implementation has no advantage other than reducing the
	// computational load imposed on the server.  Bots to not have
	// time restrictions (unlike network agents, see
	// `proto.client') so they are guaranteed to finish traversing
	// the entire tree.  Given this relaxation MinMax and
	// AlphaBeta always choose the same move, so the name remains
	// valid.
	it = func(σ *kgp.Board, ω kgp.Side, δ uint, α, β int64) (uint, int64) {
		var (
			Φ int64 // best evaluation
			μ uint  // best move
		)
		if ω == π { // maximising
			Φ = math.MinInt
		} else { // minimising
			Φ = math.MaxInt
		}

		for m := uint(0); m < λ; m++ {
			if !σ.Legal(ω, m) {
				continue
			}

			// Create a new copy to avoid destructively
			// modifying parent or sibling states (TODO:
			// Provide a "Unsow" method to avoid
			// unnecessary allocations)
			n := σ.Copy()
			// Progress to the next state
			rep := n.Sow(ω, m)
			// Evaluate the state, either immediately by
			// guesstimating the value of the current
			// state if final or we have reached the
			// maximal recursion depth, or by invoking the
			// function recursively.
			var φ int64
			if over := n.Over(); δ == 0 || over {
				if over {
					n.Collect()
				}
				φ = int64(n.Store(π)) - int64(n.Store(!π))
			} else {
				// NOTE: We are xor'ing the state with
				// side-repetition flag.
				_, φ = it(n, kgp.Side(bool(ω) != !rep), δ-1, α, β)
			}

			if ω == π { // maximising
				if φ > Φ {
					Φ = φ
					μ = m
				}
				if Φ > α {
					α = Φ
				}
				if Φ >= β {
					break
				}
			} else { // minimising
				if φ < Φ {
					Φ = φ
					μ = m
				}
				if Φ < β {
					β = Φ
				}
				if Φ <= α {
					break
				}
			}
		}

		return μ, Φ
	}
	return it(Σ, π, Δ, math.MinInt, math.MaxInt)
}

// From Moby Thesaurus II by Grady Ward
var randomly = []string{
	"accidental", "adventitious", "adventitiously", "aimless", "aleatoric",
	"aleatory", "amorphous", "any which way", "anyhow", "anywise", "arbitrarily",
	"arbitrary", "around", "at random", "blobby", "blurred", "blurry", "broad",
	"by chance", "capricious", "casual", "casually", "causeless", "chance",
	"chance-medley", "chancy", "chaotic", "confused", "designless", "desultory",
	"disarticulated", "discontinuous", "disjunct", "disordered", "dispersed",
	"disproportionate", "driftless", "dysteleological", "erratic", "erratically",
	"fitful", "foggy", "formless", "fortuitous", "fortuitously", "frivolous",
	"fuzzy", "general", "gratuitous", "haphazard", "haphazardly", "hazy",
	"helter-skelter", "hit-or-miss", "ill-defined", "immethodical", "imprecise",
	"inaccurate", "inchoate", "incidental", "incidentally", "incoherent",
	"indecisive", "indefinable", "indefinite", "indefinitely", "indeterminable",
	"indeterminate", "indiscriminate", "indiscriminately", "indistinct",
	"inexact", "inexplicable", "irregular", "irregularly", "lax", "loose",
	"meaningless", "mindless", "misshapen", "nonspecific", "nonsymmetrical",
	"nonsystematic", "nonuniform", "obscure", "occasional", "occasionally", "odd",
	"orderless", "planless", "potluck", "promiscuous", "purposeless",
	"random shot", "randomly", "senseless", "serendipitous", "serendipitously",
	"shadowed forth", "shadowy", "shapeless", "spasmodic", "sporadic",
	"stochastic", "straggling", "straggly", "stray", "sweeping", "systemless",
	"unaccountable", "unarranged", "uncalculated", "unclassified", "unclear",
	"undefined", "undestined", "undetermined", "undirected", "ungraded",
	"unjoined", "unmethodical", "unmotivated", "unordered", "unorganized",
	"unplain", "unplanned", "unpremeditated", "unpremeditatedly", "unsorted",
	"unspecific", "unspecified", "unsymmetrical", "unsystematic",
	"unsystematically", "ununiform", "vague", "veiled", "wandering",
}

func (m *minmax) Request(g *kgp.Game) (*kgp.Move, bool) {
	if g.Board.Over() {
		panic("Unexpected final state")
	}
	if m.acc == 0 || rand.Float64() > m.acc {
		r := randomly[rand.Intn(len(randomly))]
		return &kgp.Move{
			Choice:  g.Board.Random(g.Side(m)),
			Comment: fmt.Sprintf("*%s*", r),
			Agent:   m,
			State:   g.Board,
			Game:    g,
			Stamp:   time.Now(),
		}, false
	}
	move, ev := search(g.Board, g.Side(m), m.depth)
	if !g.Board.Legal(g.Side(m), move) {
		panic(fmt.Sprintf("Proposing illegal move %d for %s given %s",
			move, g.Side(m), g.Board))
	}
	return &kgp.Move{
		Choice:  move,
		Comment: fmt.Sprintf("Evaluation: %d", ev),
		Agent:   m,
		State:   g.Board,
		Game:    g,
		Stamp:   time.Now(),
	}, false
}

func (m *minmax) User() *kgp.User { return m.user }
func (m *minmax) String() string  { return fmt.Sprintf("MM%d/%f", m.depth, m.acc) }
func (*minmax) IsBot()            {}
func (*minmax) Alive() bool       { return true } // bots never die

func MakeMinMax(depth uint, acc float64) kgp.Agent {
	if acc < 0 || acc > 1 {
		panic("Invalid accuracy")
	}

	var user *kgp.User
	switch acc {
	case 0:
		user = &kgp.User{
			Token: fmt.Sprintf("%s-rand", nonce),
			Name:  "Random",
			Descr: `A reference agent that will always make a random legal move.`,
		}
	case 1:
		user = &kgp.User{
			Token: fmt.Sprintf("%s-mm%d", nonce, depth),
			Name:  fmt.Sprintf("MinMax-%d", depth),
			Descr: fmt.Sprintf(`
Simple reference implementation for a MinMax agent.

This agent is a bot and is provided by the practice server to make
comparing the performance easier.  Note that bots are not time-bound
and will always complete their search.  This agent will always search
%d plies ahead and return the best move it can find.`, depth),
		}
	default:
		// Idea stolen from https://www.youtube.com/watch?v=DpXy041BIlA
		user = &kgp.User{
			Token: fmt.Sprintf("%s-mm%d/%f", nonce, depth, acc),
			Name:  fmt.Sprintf("MinMax-%d (%.0f%%)", depth, 100*acc),
			Descr: fmt.Sprintf(`
Simple reference implementation for a tainted MinMax agent.

This agent is a bot and is provided by the practice server to make
comparing the performance easier.  Note that bots are not time-bound
and will always complete their search.  %.0f percent of the
time, this agent will always search %d plies ahead and return the
best move it can find.  Otherwise it will revert to a random move.`,
				100*acc, depth),
		}
	}

	return &minmax{user: user, depth: depth, acc: acc}
}

var random = MakeMinMax(0, 0.0)

func MakeRandom() kgp.Agent {
	return random
}
