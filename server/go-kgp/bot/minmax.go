// Primitive MinMax Agent
//
// Copyright (c) 2022  Philip Kaludercic
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
	"os"
	"time"

	"go-kgp"
)

// We need a random
var nonce string = os.Getenv("NONCE")

type minmax struct {
	depth uint      // ply cutoff
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

func (m *minmax) Request(g *kgp.Game) (*kgp.Move, bool) {
	if g.Board.Over() {
		panic("Unexpected final state")
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
func (m *minmax) String() string  { return fmt.Sprintf("MM%d", m.depth) }
func (*minmax) IsBot()            {}
func (*minmax) Alive() bool       { return true } // bots never die

func MakeMinMax(depth uint) kgp.Agent {
	return &minmax{
		user: &kgp.User{
			Token: fmt.Sprintf("%s-mm%d", nonce,
				depth),
			Name:  fmt.Sprintf("MinMax-%d", depth),
			Descr: `Simple reference implementation for a MinMax agent.`,
		},
		depth: depth,
	}
}
