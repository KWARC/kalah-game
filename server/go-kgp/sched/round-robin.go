// Round Robin Tournament
//
// Copyright (c) 2022, 2023  Philip Kaludercic
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
	"sync"

	"go-kgp"
	cmd "go-kgp/cmd"
)

type rr struct {
	agents map[kgp.Agent]struct{}
	sched  *scheduler
	wait   sync.WaitGroup
	size   uint
	init   uint
}

func (r *rr) Start(mode *cmd.State, conf *kgp.Conf) {
	var games []*kgp.Game

	// Prepare all games
	for a := range r.agents {
		for b := range r.agents {
			if a == b {
				continue
			}

			games = append(games, &kgp.Game{
				Board: kgp.MakeBoard(r.size, r.init),
				South: a,
				North: b,
			})
		}
	}

	r.sched = &scheduler{
		games: games,
	}
	r.sched.run(&r.wait, mode, conf)
}

func (r *rr) Shutdown() {
	r.wait.Wait()
}

func (r *rr) Schedule(a kgp.Agent)   {}
func (r *rr) Unschedule(a kgp.Agent) {}
func (*rr) String() string           { return "Round Robin" }

func (r *rr) initialize(agents map[kgp.Agent]struct{}) {
	r.agents = agents
}

func (r *rr) results() map[kgp.Agent]struct{} {
	next := make(map[kgp.Agent]struct{})
	for a, d := range r.sched.results {
		if d != nil {
			next[a] = struct{}{}
		}
	}
	return next
}

func MakeRoundRobin(size, init uint) cmd.Scheduler {
	return cmd.Scheduler(&rr{
		agents: make(map[kgp.Agent]struct{}),
		size:   size,
		init:   init,
	})
}
