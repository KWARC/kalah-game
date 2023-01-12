// Kalah Board Implementation
//
// Copyright (c) 2021  Philip Kaludercic
//
// This file is part of kgpc, based on go-kgp.
//
// kgpc is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License,
// version 3, as published by the Free Software Foundation.
//
// kgpc is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License, version 3, along with kgpc . If not, see
// <http://www.gnu.org/licenses/>

package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
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
	init         uint
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
	board.init = init

	return board
}

func parseBoard(str string) *Board {
	str = strings.TrimPrefix(str, "<")
	str = strings.TrimSuffix(str, ">")

	raw := strings.Split(str, ",")
	if len(raw) < 5 {
		return nil
	}
	data := make([]uint, len(raw))
	for i, r := range raw {
		v, err := strconv.Atoi(r)
		if err != nil {
			return nil
		}
		data[i] = uint(v)
	}

	size := data[0]
	if int(size)*2+3 != len(data) {
		return nil
	}

	board := makeBoard(size, 0)
	board.north = data[1]
	board.south = data[2]
	board.southPits = data[3 : 3+size]
	board.northPits = data[3+size:]

	return &board
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
