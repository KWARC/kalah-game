// MinMax Implementation Tests
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
// License, version 3, along with go-kgp. If not, see
// <http://www.gnu.org/licenses/>

package bot

import (
	"testing"

	"go-kgp"
)

func TestSearch(t *testing.T) {
	for i, test := range []struct {
		state    string
		side     kgp.Side
		depth    uint
		expected uint
	}{
		{
			state:    "<2, 0,0, 0,1, 0,0>",
			side:     kgp.South,
			depth:    5,
			expected: 1,
		},
		{
			state:    "<2, 0,1, 2,0, 0,0>",
			side:     kgp.South,
			depth:    5,
			expected: 0,
		},
		{
			state:    "<2, 0,1, 2,0, 1,0>",
			side:     kgp.South,
			depth:    5,
			expected: 0,
		},
		{
			state:    "<2, 0,1, 2,0, 1,0>",
			side:     kgp.South,
			depth:    5,
			expected: 0,
		},
		{
			state:    "<3, 0,0, 0,0,1, 0,0,0>",
			side:     kgp.South,
			depth:    5,
			expected: 2,
		},
		{
			state:    "<3, 0,0, 3,0,0, 1,1,1>",
			side:     kgp.South,
			depth:    10,
			expected: 0,
		},
		{
			state:    "<3, 0,0, 0,2,0, 1,1,1>",
			side:     kgp.South,
			depth:    10,
			expected: 1,
		},
		{
			state:    "<3, 0,0, 3,1,0, 1,1,1>",
			side:     kgp.South,
			depth:    10,
			expected: 0,
		},
		{
			state:    "<4, 0,0, 0,3,1,0, 1,1,1,1>",
			side:     kgp.South,
			depth:    10,
			expected: 1,
		},
	} {
		state, err := kgp.Parse(test.state)
		if err != nil {
			t.Fatalf("Parse error: %s", err)
		}
		size, _ := state.Type()

		move, ev := search(state, test.side, test.depth)
		if move >= size {
			t.Errorf("[%d] Proposed impossible move %d given %s (%d)",
				i, move, state, ev)
		} else if !state.Legal(test.side, move) {
			t.Errorf("[%d] Proposed illegal move %d given %s (%d)",
				i, move, state, ev)
		} else if test.expected != move {
			t.Errorf("[%d] Expected move %d, but got %d (%d)",
				i, test.expected, move, ev)
		}
	}
}
