package main

import "testing"

func boardEq(b1 *Board, b2 *Board) bool {
	if b1 == nil || b2 == nil {
		return false
	}

	if b1.north != b2.north {
		return false
	}
	if b1.south != b2.south {
		return false
	}

	for i := range b1.northPits {
		if b1.northPits[i] != b1.northPits[i] {
			return false
		}
	}

	for i := range b1.southPits {
		if b1.southPits[i] != b1.southPits[i] {
			return false
		}
	}

	return true
}

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
		},
	} {
		again := test.start.Sow(test.side, test.move)
		if test.again != again {
			t.Errorf("(%d) Didn't recognize repeat move", i)
		} else if !boardEq(test.start, test.end) {
			t.Errorf("(%d) Expected %s, got %s", i, test.end, test.start)
		}
	}
}

func TestOver(t *testing.T) {
	for _, test := range []struct {
		board *Board
		over  bool
	}{
		{
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			over: false,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{3, 3, 3},
			},
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{3, 3, 3},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			over: true,
		}, {
			board: &Board{
				north:     0,
				northPits: []uint{0, 0, 0},
				south:     0,
				southPits: []uint{0, 0, 0},
			},
			over: true,
		},
	} {
		if test.board.Over() != test.over {
			t.Fail()
		}
	}
}
