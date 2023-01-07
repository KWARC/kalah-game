// Sanity Test
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
	"go-kgp/bot"
	"go-kgp/conf"
)

type san struct {
	agents map[kgp.Agent]struct{}
	sched  *scheduler
	wait   sync.WaitGroup
	conf   *conf.Conf
}

func (s *san) Start() {
	var (
		games = make([]*kgp.Game, 2*len(s.agents))
		adv   = bot.MakeRandom()
	)

	for agent := range s.agents {
		games = append(games, &kgp.Game{
			Board: kgp.MakeBoard(6, 6),
			South: agent,
			North: adv,
		}, &kgp.Game{
			Board: kgp.MakeBoard(6, 6),
			South: adv,
			North: agent,
		})
	}

	s.sched = &scheduler{
		conf:  s.conf,
		games: games,
	}
	s.sched.run(&s.wait)
}

func (s *san) Shutdown() {
	s.wait.Wait()
}

func (*san) Schedule(a kgp.Agent)   {}
func (*san) Unschedule(a kgp.Agent) {}

func (*san) String() string { return "Sanity Test" }

func MakeSanityCheck(config *conf.Conf) conf.GameManager {
	var man conf.GameManager = &san{
		agents: make(map[kgp.Agent]struct{}),
		conf:   config,
	}
	return man
}
