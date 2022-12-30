// Random Agent
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
	"time"

	"go-kgp"
)

type rand struct {
	user *kgp.User // database entry
}

func (r *rand) Request(g *kgp.Game) (*kgp.Move, bool) {
	if g.Board.Over() {
		panic("Unexpected final state")
	}

	return &kgp.Move{
		Choice: g.Board.Random(g.Side(r)),
		Agent:  r,
		State:  g.Board,
		Game:   g,
		Stamp:  time.Now(),
	}, false
}

func (m *rand) User() *kgp.User { return m.user }
func (m *rand) String() string  { return "random" }
func (*rand) IsBot()            {}
func (*rand) Alive() bool       { return true } // bots never die

func MakeRandom() kgp.Agent {
	return &rand{
		user: &kgp.User{
			Name:  "random",
			Descr: "An agent that only makes random moves",
		},
	}
}
