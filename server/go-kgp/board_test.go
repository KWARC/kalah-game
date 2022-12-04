// Kalah Board Implementation Tests
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

package kgp

import (
	"reflect"
	"testing"
)

func TestLegal(t *testing.T) {
	for i, test := range []struct {
		start *Board
		move  uint
		side  Side
		legal bool
	}{
		{
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  North,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{5, 5, 5, 5},
				south:     0,
				southPits: []uint{5, 5, 5, 5},
			},
			move:  2,
			side:  North,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  2,
			side:  North,
			legal: true,
		}, {
			start: &Board{
				north:     1,
				northPits: []uint{3, 3, 3},
				south:     1,
				southPits: []uint{3, 3, 3},
			},
			move:  1,
			side:  South,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{9, 9, 9},
				south:     0,
				southPits: []uint{9, 9, 9},
			},
			move:  0,
			side:  North,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{1, 0, 3},
			},
			move:  0,
			side:  South,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  North,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{0, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  North,
			legal: false,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{0, 0, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  North,
			legal: false,
		},
	} {
		legal := test.start.Legal(test.side, test.move)
		if test.legal != legal {
			t.Errorf("(%d) Didn't recognize illegla move", i)
		}
	}
}

func TestSow(t *testing.T) {
	// FIXME: adapt test cases

	for i, test := range []struct {
		start, end *Board
		move       uint
		side       Side
		again      bool
	}{
		{
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			end: &Board{
				north:     1,
				northPits: []uint{0, 4, 4},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  North,
			again: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{5, 5, 5, 5},
				south:     0,
				southPits: []uint{5, 5, 5, 5},
			},
			end: &Board{
				north:     1,
				northPits: []uint{5, 5, 0, 6},
				south:     0,
				southPits: []uint{6, 6, 6, 5},
			},
			move: 2,
			side: North,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			end: &Board{
				north:     1,
				northPits: []uint{3, 3, 0},
				south:     0,
				southPits: []uint{4, 4, 3},
			},
			move: 2,
			side: North,
		}, {
			start: &Board{
				north:     1,
				northPits: []uint{3, 3, 3},
				south:     1,
				southPits: []uint{3, 3, 3},
			},
			end: &Board{
				north:     1,
				northPits: []uint{4, 3, 3},
				south:     2,
				southPits: []uint{3, 0, 4},
			},
			move: 1,
			side: South,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{9, 9, 9},
				south:     0,
				southPits: []uint{9, 9, 9},
			},
			end: &Board{
				north:     1,
				northPits: []uint{1, 11, 11},
				south:     0,
				southPits: []uint{10, 10, 10},
			},
			move: 0,
			side: North,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{1, 0, 3},
			},
			end: &Board{
				north:     0,
				northPits: []uint{3, 0, 3},
				south:     4,
				southPits: []uint{0, 0, 3},
			},
			move: 0,
			side: South,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{7, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			end: &Board{
				north:     6,
				northPits: []uint{0, 4, 4},
				south:     0,
				southPits: []uint{4, 4, 0},
			},
			move: 0,
			side: North,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{1, 0, 1},
				south:     0,
				southPits: []uint{0, 0, 1},
			},
			end: &Board{
				north:     0,
				northPits: []uint{0, 1, 1},
				south:     0,
				southPits: []uint{0, 0, 1},
			},
			move: 0,
			side: North,
		},
	} {
		again := test.start.Sow(test.side, test.move)
		if test.again != again {
			t.Errorf("(%d) Didn't recognize repeat move", i)
		} else if !reflect.DeepEqual(test.start, test.end) {
			t.Errorf("(%d) Expected %s, got %s", i, test.end, test.start)
		}
	}
}

func TestOverFor(t *testing.T) {
	for _, test := range []struct {
		board *Board
		side  Side
		over  bool
	}{
		{
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: North,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: South,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: North,
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: South,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: North,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: South,
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: North,
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: South,
			over: true,
		},
	} {
		over, _ := test.board.OverFor(test.side)
		if over != test.over {
			t.Fail()
		}
	}
}

func TestOutcome(t *testing.T) {
	for i, test := range []struct {
		board   *Board
		outcome Outcome
	}{
		{
			board: &Board{
				north: 0,
				south: 1,
			},
			outcome: WIN,
		}, {
			board: &Board{
				north: 1,
				south: 0,
			},
			outcome: LOSS,
		}, {
			board: &Board{
				north: 0,
				south: 0,
			},
			outcome: DRAW,
		}, {
			board: &Board{
				north: 1,
				south: 1,
			},
			outcome: DRAW,
		}, {
			board: &Board{
				north:     2,
				south:     1,
				southPits: []uint{1, 1, 1},
			},
			outcome: WIN,
		}, {
			board: &Board{
				north:     0,
				south:     2,
				northPits: []uint{1, 1, 1},
			},
			outcome: LOSS,
		},
	} {
		outcome := test.board.Outcome(South)
		if outcome != test.outcome {
			t.Errorf("(%d) Expected %d, got %d", i, test.outcome, outcome)
		}
	}
}

func TestParse(t *testing.T) {
	for i, test := range []struct {
		input  string
		output *Board
		fail   bool
	}{
		{
			input: "<3,0,0,0,0,0,3,3,3>", output: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
		},
		{
			input: "<2, 0,0, 0,1, 0,0>",
			output: &Board{
				north:     0,
				northPits: []uint{0, 0},
				south:     0,
				southPits: []uint{0, 1},
			},
		},
		{
			input: "<2,1,2,3,4,5,6>", output: &Board{
				north:     2,
				northPits: []uint{5, 6},
				south:     1,
				southPits: []uint{3, 4},
			},
		},
		{
			input: "<5,1,2,3,4,5,6,7,8,9,10,11,12>", output: &Board{
				north:     2,
				northPits: []uint{8, 9, 10, 11, 12},
				south:     1,
				southPits: []uint{3, 4, 5, 6, 7},
			},
		},
		{
			input: "<1,0,0,0,0>", output: &Board{
				north:     0,
				northPits: []uint{0},
				south:     0,
				southPits: []uint{0},
			},
		},
		{
			input: " <1	,0 , 0,0 , 	 0>  ", output: &Board{
				north:     0,
				northPits: []uint{0},
				south:     0,
				southPits: []uint{0},
			},
		},
		{
			input: " < 1 , 0 , 0 , 0 , 0 > ", output: &Board{
				north:     0,
				northPits: []uint{0},
				south:     0,
				southPits: []uint{0},
			},
		},
		{input: "<0>", fail: true},
		{input: "<0,1,1>", fail: true},
		{input: "<1,1,1,1>", fail: true},
		{input: "<1,1,1,1,1,1>", fail: true},
		{input: "1,1,1,1,1", fail: true},
		{input: "<1,1,1,1,1", fail: true},
		{input: "1,1,1,1,1>", fail: true},
		{input: "<1,1,1,1,a>", fail: true},
	} {
		parse, err := Parse(test.input)
		if test.fail {
			if err == nil {
				t.Errorf("(%d) Expected error", i)
			}
		} else {
			if err != nil {
				t.Errorf("(%d) Failed with %q", i, err)
			} else if parse.String() != test.output.String() {
				t.Errorf("(%d) Expected %s, got %s",
					i, test.output, parse)
			}
		}
	}
}
