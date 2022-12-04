// Kalah Board Implementation
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
// License, version 3, along with go-kgp . If not, see
// <http://www.gnu.org/licenses/>

package kgp

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

var repr = regexp.MustCompile(`^\s*<\s*(\d+(\s*,\s*\d+)+)\s*>\s*$`)

// Board represents a Kalah game
type Board struct {
	// The north and south store
	north, south uint
	// The northern pits (from right to left)
	northPits []uint
	// The southern pits (from left to right)
	southPits []uint
	// The initial board size
	init uint
}

func (b *Board) Type() (size, init uint) {
	return uint(len(b.northPits)), b.init
}

func (b *Board) Pit(side Side, pit uint) uint {
	if pit >= uint(len(b.northPits)) {
		panic("Illegal access")
	}

	if side {
		return b.northPits[pit]
	} else {
		return b.southPits[pit]
	}
}

func (b *Board) Store(side Side) uint {
	if side {
		return b.north
	} else {
		return b.south
	}
}

// create a new board with SIZE pits, each with INIT stones
func MakeBoard(size, init uint) *Board {
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
	board.init = init

	return &board
}

func Parse(spec string) (*Board, error) {
	match := repr.FindStringSubmatch(spec)
	if match == nil {
		return nil, errors.New("invalid specification")
	}

	var data []uint
	for _, part := range strings.Split(match[1], ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return nil, err
		}
		data = append(data, uint(n))
	}

	size := data[0]
	if size == 0 || uint(len(data)) != 1+2+size*2 {
		return nil, errors.New("invalid size")
	}

	b := MakeBoard(size, math.MaxUint)
	b.south = data[1]
	b.north = data[2]
	for i := uint(0); i < size; i++ {
		b.southPits[i] = data[3+i]
		b.northPits[i] = data[3+size+i]
	}
	return b, nil

}

// Mirror returns a mirrored represenation of the board
func (b *Board) Mirror() *Board {
	return &Board{
		north:     b.south,
		south:     b.north,
		northPits: b.southPits,
		southPits: b.northPits,
		init:      b.init,
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
func (b *Board) Legal(side Side, pit uint) bool {
	if pit >= uint(len(b.northPits)) {
		panic("Illegal access")
	}
	if side == North {
		return b.northPits[pit] > 0
	}
	return b.southPits[pit] > 0
}

func (b *Board) Moves(side Side) (count, last uint) {
	for i := uint(0); i < uint(len(b.northPits)); i++ {
		if b.Legal(side, i) {
			last = i
			count++
		}
	}

	return
}

// Random returns a random legal move for SIDE
func (b *Board) Random(side Side) (move uint) {
	legal := make([]uint, 0, len(b.northPits))

	for i := uint(0); i < uint(len(b.northPits)); i++ {
		if b.Legal(side, i) {
			legal = append(legal, i)
		}
	}

	// if len(legal) == true, rand.Intn panics.  This is ok, because
	// Random shouldn't be called when the game is already over.
	return legal[rand.Intn(len(legal))]
}

// Sow modifies the board by sowing PIT for player SELF
func (b *Board) Sow(self Side, pit uint) bool {
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
		panic(fmt.Sprintf("Illegal move %d by %s in %s",
			pit, self, b))
	}

	// pick up stones from pit
	if self == North {
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
				if self == North {
					b.north++
				} else {
					b.south++
				}
				stones--
			}

			side = !side
			pos = 0
		} else {
			if side == North {
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
		if b.Over() {
			b.Collect()
		}

		return true
	} else if side == self && pos > 0 {
		last := int(pos - 1)
		if side == North && b.northPits[last] == 1 && b.southPits[size-1-last] > 0 {
			b.north += b.southPits[size-1-last] + 1
			b.southPits[size-1-last] = 0
			b.northPits[last] = 0
		} else if side == South && b.southPits[last] == 1 && b.northPits[size-1-last] > 0 {
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

// OverFor returns true if the game has finished for a SIDE
//
// The second argument designates the right-most pit that would be a
// legal move, iff the game is not over for SIDE.
func (b *Board) OverFor(side Side) (bool, uint) {
	var pits []uint
	switch side {
	case North:
		pits = b.northPits
	case South:
		pits = b.southPits
	}

	for i := range pits {
		if pits[i] > 0 {
			return false, uint(i)
		}
	}
	return true, 0
}

// Over returns true if the game is over for either side
func (b *Board) Over() bool {
	var stones uint

	for _, pit := range b.northPits {
		stones += pit
	}
	for _, pit := range b.southPits {
		stones += pit
	}
	stones += b.north
	stones += b.south

	if b.north > stones/2 || b.south > stones/2 {
		return true
	}

	north, _ := b.OverFor(North)
	south, _ := b.OverFor(South)
	return north || south
}

// Calculate the outcome for SIDE
func (b *Board) Outcome(side Side) Outcome {
	var north, south uint

	for _, pit := range b.northPits {
		north += pit
	}
	for _, pit := range b.southPits {
		south += pit
	}

	if !b.Over() {
		return RESIGN
	}

	north += b.north
	south += b.south

	switch {
	case north > south:
		if side == North {
			return WIN
		} else {
			return LOSS
		}
	case north < south:
		if side == North {
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

// Deep copy of the board
func (b *Board) Copy() *Board {
	north := make([]uint, len(b.southPits))
	south := make([]uint, len(b.northPits))
	if copy(north, b.northPits) != len(b.northPits) {
		panic("Illegal board state")
	}
	if copy(south, b.southPits) != len(b.southPits) {
		panic("Illegal board state")
	}
	return &Board{
		north:     b.north,
		south:     b.south,
		northPits: north,
		southPits: south,
		init:      b.init,
	}
}
