// Connection Queue Handling
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

package sched

import (
	random "math/rand"
	"time"

	"go-kgp"
	"go-kgp/bot"
	"go-kgp/conf"
	"go-kgp/game"
)

// The intent is not to have a secure source of random values, but
// just to avoid a predictive shuffling of north/south positions.
func init() { random.Seed(time.Now().UnixMicro()) }

type fifo struct {
	conf *conf.Conf
	add  chan kgp.Agent
	rem  chan kgp.Agent
}

func (f *fifo) Start() {
	var (
		bots []kgp.Agent
		q    []kgp.Agent
		bi   int // bot index
	)

	for _, d := range f.conf.BotTypes {
		bots = append(bots, bot.MakeMinMax(d))
	}

	// Idea: FIFO but get lucky and you might be pulled ahead
	//
	// Priority: Reduce average waiting time
	//
	// Non-Priority: Avoid frequent encounters
	for {
		select {
		case a := <-f.add:
			kgp.Debug.Println("Schedule", a)
			if !bot.IsBot(a) {
				q = append(q, a)
			}
		case a := <-f.rem:
			kgp.Debug.Println("Remove", a)
			for i := range q {
				if q[i] != a {
					continue
				}

				q[i] = q[len(q)-1]
				q = q[:len(q)-1]
			}
			continue
		}
		kgp.Debug.Print(q)

		// Remove all dead agents
		i := 0
		for _, a := range q {
			if a.Alive() {
				q[i] = a
				i++
			}
		}
		q = q[:i]

		// Select two agents, or two agents and a bot if only
		// one agent is available.
		var north, south kgp.Agent
		switch len(q) {
		case 0:
			continue
		case 1:
			south = q[0]
			q = nil

			// rotate through all bots
			bi = (bi + 1) % len(bots)
			north = bots[bi]
		default:
			south = q[0]
			north = q[1]
			q[0] = q[len(q)-1]
			q[1] = q[len(q)-2]
			q = q[:len(q)-2]
		}
		kgp.Debug.Println("Selected", north, south)

		// Ensure that we actually have two agents
		if north == nil || south == nil {
			panic("Illegal state")
		}

		// Start a game, but shuffle the order to avoid an
		// advantage for bots or non-bots.
		if random.Intn(2) == 0 {
			north, south = south, north
		}

		go func(north, south kgp.Agent) {
			game.Play(&kgp.Game{
				Board: kgp.MakeBoard(
					f.conf.BoardSize,
					f.conf.BoardInit),
				South: north,
				North: south,
			}, f.conf)
			time.Sleep(5 * time.Second)
			f.Schedule(south)
			f.Schedule(north)
		}(north, south)
	}
	panic("Quitting Random Scheduler")
}

func (*fifo) Shutdown() {
	select {}
}

func (f *fifo) Schedule(a kgp.Agent)   { f.add <- a }
func (f *fifo) Unschedule(a kgp.Agent) { f.rem <- a }
func (*fifo) String() string           { return "Random Scheduler" }

func MakeFIFO(config *conf.Conf) conf.GameManager {
	var man conf.GameManager = &fifo{
		add:  make(chan kgp.Agent, 1),
		rem:  make(chan kgp.Agent, 1),
		conf: config,
	}
	return man
}
