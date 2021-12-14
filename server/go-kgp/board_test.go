// Kalah Board Implementation Tests
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
	"reflect"
	"testing"
)

func TestLegal(t *testing.T) {
	for i, test := range []struct {
		start *Board
		move  int
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
			side:  SideNorth,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{5, 5, 5, 5},
				south:     0,
				southPits: []uint{5, 5, 5, 5},
			},
			move:  2,
			side:  SideNorth,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  2,
			side:  SideNorth,
			legal: true,
		}, {
			start: &Board{
				north:     1,
				northPits: []uint{3, 3, 3},
				south:     1,
				southPits: []uint{3, 3, 3},
			},
			move:  1,
			side:  SideSouth,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{9, 9, 9},
				south:     0,
				southPits: []uint{9, 9, 9},
			},
			move:  0,
			side:  SideNorth,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{1, 0, 3},
			},
			move:  0,
			side:  SideSouth,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{3, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  SideNorth,
			legal: true,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{0, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  SideNorth,
			legal: false,
		}, {
			start: &Board{
				north:     0,
				northPits: []uint{0, 0, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			move:  0,
			side:  SideNorth,
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
	conf.Game.EarlyWin = false

	for i, test := range []struct {
		start, end *Board
		move       int
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
			side:  SideNorth,
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
			side: SideNorth,
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
			side: SideNorth,
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
			side: SideSouth,
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
			side: SideNorth,
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
			side: SideSouth,
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
			side: SideNorth,
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
			side: SideNorth,
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
			side: SideNorth,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: SideSouth,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: SideNorth,
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			side: SideSouth,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: SideNorth,
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: SideSouth,
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: SideNorth,
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			side: SideSouth,
			over: true,
		},
	} {
		if test.board.OverFor(test.side) != test.over {
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
		outcome := test.board.Outcome(SideSouth)
		if outcome != test.outcome {
			t.Errorf("(%d) Expected %d, got %d", i, test.outcome, outcome)
		}
	}
}
