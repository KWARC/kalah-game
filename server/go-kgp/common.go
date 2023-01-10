// Common Interfaces and constants
//
// Copyright (c) 2021, 2022, 2023  Philip Kaludercic
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
	"errors"
	"fmt"
	"time"
)

type (
	Side    bool
	Outcome uint8
	State   uint8
)

const (
	// Possible agent modes
	South, North Side = false, true

	// Possible game outcomes
	_ Outcome = iota
	WIN
	LOSS
	DRAW

	// Possible game states
	ONGOING State = iota
	NORTH_WON
	SOUTH_WON
	NORTH_RESIGNED
	SOUTH_RESIGNED
	UNDECIDED
	ABORTED
)

func (o Outcome) String() string {
	switch o {
	case WIN:
		return "Win"
	case DRAW:
		return "Draw"
	case LOSS:
		return "Loss"
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

func (s *State) String() string {
	switch *s {
	case ONGOING:
		return "o"
	case NORTH_WON:
		return "nw"
	case SOUTH_WON:
		return "sw"
	case NORTH_RESIGNED:
		return "nr"
	case SOUTH_RESIGNED:
		return "sr"
	case UNDECIDED:
		return "u"
	case ABORTED:
		return "a"
	default:
		panic(fmt.Sprintf("Illegal state: %d", *s))
	}
}

func (s *State) Scan(src interface{}) error {
	str, ok := src.(string)
	if !ok {
		return errors.New(`invalid type`)
	}

	switch str {
	case "o":
		*s = ONGOING
	case "nw":
		*s = NORTH_WON
	case "sw":
		*s = SOUTH_WON
	case "nr":
		*s = NORTH_RESIGNED
	case "sr":
		*s = SOUTH_RESIGNED
	case "u":
		*s = UNDECIDED
	case "a":
		*s = ABORTED
	default:
		return errors.New(`unknown state`)
	}
	return nil
}

type Agent interface {
	Request(*Game) (*Move, bool)
	User() *User
	Alive() bool
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
	Board     *Board
	Id        uint64
	North     Agent
	South     Agent
	Current   Side
	State     State
	MoveCount uint
	LastMove  time.Time
}

func (g *Game) Side(a Agent) Side {
	switch a {
	case g.North:
		return North
	case g.South:
		return South
	default:
		panic(fmt.Sprintf("Unknown Agent %s : %T (neither %s, nor %s)",
			a, a, g.South, g.North))
	}
}

func (g *Game) Player(s Side) Agent {
	switch s {
	case North:
		return g.North
	case South:
		return g.South
	default:
		panic("Invalid state")
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
