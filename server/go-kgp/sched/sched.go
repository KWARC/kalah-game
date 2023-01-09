// Generic Scheduler Pool
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
	"io"
	"log"
	"runtime"
	"sync"

	"go-kgp"
	cmd "go-kgp/cmd"
	"go-kgp/game"
	"go-kgp/sched/isol"
)

type scheduler struct {
	name   string
	wait   sync.WaitGroup
	agents []kgp.Agent
	// Function to generate a schedule
	schedule func([]kgp.Agent) []*kgp.Game
	// Function to determine if an agent passed
	judge func(kgp.Agent, map[kgp.Agent][]kgp.Agent) bool
	// Mapping from an agent to everyone who it managed to defeat
	results map[kgp.Agent][]kgp.Agent
}

func (s *scheduler) String() string {
	return s.name
}

func (s *scheduler) Start(mode *cmd.State, conf *cmd.Conf) {
	games := s.schedule(s.agents)
	s.results = make(map[kgp.Agent][]kgp.Agent)
	sched := make(chan *kgp.Game, len(games))
	for _, g := range games {
		sched <- g
	}

	var lock sync.Mutex
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for g := range sched {
				var err error
				g.South, err = isol.Start(mode, g.South)
				if err != nil {
					log.Print(err)
					goto skip
				}
				g.North, err = isol.Start(mode, g.North)
				if err != nil {
					log.Print(err)
					goto skip
				}

				game.Play(g, mode, conf)

				lock.Lock()
				switch g.State {
				case kgp.NORTH_WON:
					s.results[g.North] = append(s.results[g.North], g.South)
				case kgp.SOUTH_WON:
					s.results[g.South] = append(s.results[g.South], g.North)
				case kgp.UNDECIDED:
					s.results[g.North] = append(s.results[g.North], g.South)
					s.results[g.South] = append(s.results[g.South], g.North)
				case kgp.ONGOING:
					// This should not be possible
					// after game.Play has
					// returned.
					panic("Encountered an ongoing game")
				}
				lock.Unlock()

			skip:
				err = isol.Shutdown(g.North)
				if err != nil {
					log.Print(err)
				}
				err = isol.Shutdown(g.South)
				if err != nil {
					log.Print(err)
				}
				s.wait.Done()
			}
		}()
	}
}

func (s *scheduler) Shutdown() {
	s.wait.Wait()
}

func (s *scheduler) Schedule(a kgp.Agent)   {}
func (s *scheduler) Unschedule(a kgp.Agent) {}

func (s *scheduler) Take(a []kgp.Agent) {
	s.agents = a
}

func (s *scheduler) Give() (next []kgp.Agent) {
	if s.judge == nil {
		return s.agents
	}
	for _, a := range s.agents {
		if s.judge(a, s.results) {
			next = append(next, a)
		}
	}
	return
}

func (*scheduler) PrintResults(io.Writer) {
	panic("unimplemented")
}
