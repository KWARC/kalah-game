// Connection Queue Handling
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

package sched

import (
	random "math/rand"
	"time"

	"go-kgp"
	"go-kgp/bot"
	"go-kgp/conf"
)

// The intent is not to have a secure source of random values, but
// just to avoid a predictive shuffling of north/south positions.
func init() { random.Seed(time.Now().UnixMicro()) }

type rand struct {
	conf *conf.Conf
	add  chan kgp.Agent
	rem  chan kgp.Agent
}

type Bot interface {
	kgp.Agent
	IsBot()
}

func isBot(a kgp.Agent) bool {
	_, ok := a.(Bot)
	return ok
}

func (f *rand) Start() {
	var q []kgp.Agent

	for d, n := range f.conf.BotTypes {
		for i := uint(0); i < n; i++ {
			f.conf.Debug.Printf("Add MinMax bot with depth %d", d)
			q = append(q, bot.MakeMinMax(d))
		}
	}

	// Idea: FIFO but get lucky and you might be pulled ahead
	//
	// Priority: Reduce average waiting time
	//
	// Non-Priority: Avoid frequent encounters
	for {
		nonbots := 0
		for _, a := range q {
			if !isBot(a) {
				nonbots++
			}
		}
		if nonbots >= 2 {
			goto skip
		}
		select {
		case a := <-f.add:
			f.conf.Debug.Println("Schedule", a)
			q = append(q, a)
		case a := <-f.rem:
			f.conf.Debug.Println("Remove", a)
			for i := range q {
				if q[i] != a {
					continue
				}

				q[i] = q[len(q)-1]
				q = q[:len(q)-1]
			}
			continue
		}
	skip:
		f.conf.Debug.Print(q)

		// Try and select two agents, where at least one is
		// not a bot
		var (
			north, south kgp.Agent
			ni, si       int = -1, -1
		)
		for i, a := range q {
			if !isBot(a) {
				north = a
				ni = i
				break
			}
		}
		for i, a := range q {
			if i != ni {
				south = a
				si = i
				break
			}
		}
		f.conf.Debug.Println("Selected", north, south)

		// Only proceed if we actually found two agents
		if north == nil || south == nil {
			continue
		}
		q[ni] = q[len(q)-1]
		q[si] = q[len(q)-2]
		q = q[:len(q)-2]

		// Start a game, but shuffle the order to avoid an
		// advantage for bots or non-bots.
		if random.Intn(2) == 0 {
			north, south = south, north
		}

		board := kgp.MakeBoard(
			f.conf.BoardSize,
			f.conf.BoardInit)
		f.conf.Play <- &kgp.Game{
			State: board,
			South: north,
			North: south,
		}
	}
	panic("Quitting Random Scheduler")
}

func (f *rand) Schedule(a kgp.Agent)   { f.add <- a }
func (f *rand) Unschedule(a kgp.Agent) { f.rem <- a }
func (*rand) Shutdown()                {}
func (*rand) String() string           { return "Random Scheduler" }

func MakeRandom(config *conf.Conf) conf.GameManager {
	var man conf.GameManager = &rand{
		add:  make(chan kgp.Agent, 1),
		rem:  make(chan kgp.Agent, 1),
		conf: config,
	}
	return man
}
