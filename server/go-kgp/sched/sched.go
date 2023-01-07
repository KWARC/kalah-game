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
	"log"
	"runtime"
	"sync"

	"go-kgp"
	cmd "go-kgp/cmd"
	"go-kgp/game"
	"go-kgp/sched/isol"
)

type scheduler struct {
	games []*kgp.Game
	// Mapping from an agent to everyone who it managed to defeat
	results map[kgp.Agent][]kgp.Agent
}

func (s *scheduler) run(wait *sync.WaitGroup, mode *cmd.State, conf *kgp.Conf) {
	s.results = make(map[kgp.Agent][]kgp.Agent)
	sched := make(chan *kgp.Game, len(s.games))
	for _, g := range s.games {
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
				wait.Done()
			}
		}()
	}
}
