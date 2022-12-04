// Common Interfaces and constants
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
	"fmt"
	"time"
)

type (
	Side    bool
	Outcome uint8
)

const (
	// Possible agent modes
	South, North Side = false, true
	// Possible game states
	ONGOING Outcome = iota
	WIN
	DRAW
	LOSS
	RESIGN
)

func (o Outcome) String() string {
	switch o {
	case ONGOING:
		return "Ongoing"
	case WIN:
		return "Win"
	case DRAW:
		return "Draw"
	case LOSS:
		return "Loss"
	case RESIGN:
		return "Resign"
	default:
		panic(fmt.Sprintf("Illegal outcome: %d", o))
	}
}

func (b Side) String() string {
	switch b {
	case South:
		return "South"
	case North:
		return "North"
	}
	panic("Illegal side")
}

type Agent interface {
	Request(*Game) (*Move, bool)
	User() *User
}

type User struct {
	Id     int64
	Token  string
	Name   string
	Descr  string
	Author string
	Games  uint64
}

type Game struct {
	// The board the game is being played on
	State     *Board
	Id        uint64
	North     Agent
	South     Agent
	Current   Side
	Outcome   Outcome
	MoveCount uint
}

func (g *Game) Side(a Agent) Side {
	switch a {
	case g.North:
		return North
	case g.South:
		return South
	default:
		panic("Unknown Agent")
	}
}

func (g *Game) Player(s Side) Agent {
	switch s {
	case North:
		return g.North
	case South:
		return g.South
	default:
		panic("Unknown Agent")
	}
}

func (g *Game) Active() Agent {
	return g.Player(g.Current)
}

type Move struct {
	Choice  uint
	Comment string
	Agent   Agent
	State   *Board
	Game    *Game
	Stamp   time.Time
}
