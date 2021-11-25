package main

import (
	"bytes"
	"fmt"
	"math/rand"
)

// Side represents a side of a board
type Side bool

const (
	// SideNorth is the northern side of the board
	SideNorth Side = false
	// SideSouth is the southern side of the board
	SideSouth Side = true
)

// String returns a string represenation for a side
func (b Side) String() string {
	if b {
		return "South"
	}
	return "North"

}

// Board represents a Kalah game
type Board struct {
	north, south uint
	northPits    []uint
	southPits    []uint
}

// create a new board with SIZE pits, each with INIT stones
func makeBoard(size, init uint) Board {
	board := Board{
		northPits: make([]uint, int(size)),
		southPits: make([]uint, int(size)),
	}

	for i := range board.northPits {
		board.northPits[i] = init
	}
	for i := range board.southPits {
		board.southPits[i] = init
	}

	return board
}

// Mirror returns a mirrored represenation of the board
func (b *Board) Mirror() *Board {
	return &Board{
		north:     b.south,
		south:     b.north,
		northPits: b.southPits,
		southPits: b.northPits,
	}
}

// String converts a board into a KGP representation
func (b *Board) String() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "<%d,%d,%d", len(b.northPits), b.south, b.north)
	for _, pit := range b.southPits {
		fmt.Fprintf(&buf, ",%d", pit)
	}
	for _, pit := range b.northPits {
		fmt.Fprintf(&buf, ",%d", pit)
	}
	fmt.Fprint(&buf, ">")

	return buf.String()
}

// Legal returns true if SIDE may play move PIT
func (b *Board) Legal(side Side, pit int) bool {
	size := len(b.northPits)

	if pit >= size || pit < 0 {
		return false
	}

	if side == SideNorth {
		return b.northPits[pit] > 0
	}
	return b.southPits[pit] > 0
}

// Random returns a random legal move for SIDE
func (b *Board) Random(side Side) (move int) {
	legal := make([]int, 0, len(b.northPits))

	for i := 0; i < len(b.northPits); i++ {
		if b.Legal(side, i) {
			legal = append(legal, i)
		}
	}

	// if len(legal) == true, rand.Intn panics.  This is ok, beacuse
	// Random shouldn't be called when the game is already over.
	return legal[rand.Intn(len(legal))]
}

// Sow modifies the board by sowing PIT for player SELF
func (b *Board) Sow(self Side, pit int) bool {
	if len(b.northPits) != len(b.southPits) {
		panic("Illegal board")
	}

	var (
		stones uint

		size = len(b.northPits)
		pos  = pit + 1
		side = self
	)

	if !b.Legal(self, pit) {
		panic("Illegal move")
	}

	// pick up stones from pit
	if self == SideNorth {
		stones = b.northPits[pit]
		b.northPits[pit] = 0
	} else {
		stones = b.southPits[pit]
		b.southPits[pit] = 0
	}

	// distribute all stones
	for stones > 0 {
		if int(pos) > size {
			panic("Out of bounds")
		} else if int(pos) == size {
			if side == self {
				if self == SideNorth {
					b.north++
				} else {
					b.south++
				}
				stones--
			}

			side = !side
			pos = 0
		} else {
			if side == SideNorth {
				b.northPits[pos]++
			} else {
				b.southPits[pos]++
			}
			pos++
			stones--
		}
	}

	// check for repeat- or collect-move
	if pos == 0 && side == !self {
		return true
	} else if side == self && pos > 0 {
		last := int(pos - 1)
		if side == SideNorth && b.northPits[last] == 1 {
			b.north += b.southPits[size-1-last] + 1
			b.southPits[size-1-last] = 0
			b.northPits[last] = 0
		} else if side == SideSouth && b.southPits[last] == 1 {
			b.south += b.northPits[size-1-last] + 1
			b.northPits[size-1-last] = 0
			b.southPits[last] = 0
		}
	}

	if b.Over() {
		b.Collect()
	}

	return false
}

// OverFor returns true if the game has finished for a side
func (b *Board) OverFor(side Side) bool {
	var pits []uint
	switch side {
	case SideNorth:
		pits = b.northPits
	case SideSouth:
		pits = b.southPits
	}

	for _, pit := range pits {
		if pit > 0 {
			return false
		}
	}
	return true
}

// Over returns true if the game is over for either side
func (b *Board) Over() bool {
	return b.OverFor(SideNorth) || b.OverFor(SideSouth)
}

func (b *Board) Outcome(side Side) Outcome {
	var (
		north = b.north
		south = b.south
	)

	for _, pit := range b.northPits {
		north += pit
	}
	for _, pit := range b.southPits {
		south += pit
	}

	switch {
	case north > south:
		if side == SideNorth {
			return WIN
		} else {
			return LOSS
		}
	case south < north:
		if side == SideNorth {
			return LOSS
		} else {
			return WIN
		}
	default:
		return DRAW
	}
}

// Move all stones for SIDE to the Kalah on SIDE
func (b *Board) Collect() {
	var north, south uint

	if !b.Over() {
		panic("Stones may not be collected")
	}

	for i, p := range b.northPits {
		north += p
		b.northPits[i] = 0
	}

	for i, p := range b.southPits {
		south += p
		b.southPits[i] = 0
	}

	b.north += north
	b.south += south
}
