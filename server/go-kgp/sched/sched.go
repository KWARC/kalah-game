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
	"fmt"
	"io"
	"log"
	"runtime"
	"sort"
	"sync"

	"go-kgp"
	"go-kgp/cmd"
	"go-kgp/game"
	"go-kgp/sched/isol"
)

type score struct{ w, l uint }

type scheduler struct {
	name   string
	wait   sync.WaitGroup
	agents []isol.ControlledAgent
	// Function to generate a schedule
	schedule func([]isol.ControlledAgent) []*kgp.Game
	// Mapping from an agent to everyone who it managed to defeat
	games []*kgp.Game
	score map[kgp.Agent]*score
}

func (s *scheduler) String() string {
	return s.name
}

func (s *scheduler) Start(mode *cmd.State, conf *cmd.Conf) {
	games := s.schedule(s.agents)
	sched := make(chan *kgp.Game, len(games))
	for _, g := range games {
		sched <- g
	}
	s.wait.Add(len(games))
	kgp.Debug.Println("Staring scheduler", s, "with", len(games), "games")

	var lock sync.Mutex
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for g := range sched {
				var (
					err   error
					south = g.South
					north = g.North
				)
				g.South, err = isol.Start(mode, conf, g.South)
				if err != nil {
					log.Print(err)
					goto skip
				}
				g.North, err = isol.Start(mode, conf, g.North)
				if err != nil {
					log.Print(err)
					goto skip
				}

				game.Play(g, mode, conf)
			skip:
				err = isol.Shutdown(g.North)
				if err != nil {
					log.Print(err)
				}
				err = isol.Shutdown(g.South)
				if err != nil {
					log.Print(err)
				}

				g.South = south
				g.North = north
				lock.Lock()
				s.games = append(s.games, g)
				lock.Unlock()

				s.wait.Done()
			}
		}()
	}
}

func (s *scheduler) Shutdown() {
	s.wait.Wait()
	kgp.Debug.Println("Completed scheduler", s)
}

func (s *scheduler) Schedule(a kgp.Agent)   {}
func (s *scheduler) Unschedule(a kgp.Agent) {}

func (s *scheduler) Take(a []isol.ControlledAgent) {
	s.agents = a
}

func (s *scheduler) Give() (next []isol.ControlledAgent) {
	s.wait.Wait()
	for _, a := range s.agents {
		w, l := s.Score(a)
		if 0 <= w-l {
			next = append(next, a)
		}
	}
	return
}

func (s *scheduler) Score(a isol.ControlledAgent) (int, int) {
	if s.score == nil {
		s.score = make(map[kgp.Agent]*score)

		for _, agent := range s.agents {
			s.score[agent] = &score{}
		}

		for _, game := range s.games {
			if S, ok := s.score[game.South]; ok {
				switch game.State {
				case kgp.NORTH_WON:
					S.l++
				case kgp.SOUTH_WON:
					S.w++
				}
			}
			if S, ok := s.score[game.North]; ok {
				switch game.State {
				case kgp.NORTH_WON:
					S.w++
				case kgp.SOUTH_WON:
					S.l++
				}
			}
		}
	}

	if sc, ok := s.score[a]; ok {
		return int(sc.w), int(sc.l)
	}
	return 0, 0
}

func (s *scheduler) PrintResults(st *cmd.State, W io.Writer) {
	fmt.Fprintln(W, `.NH 1`)
	fmt.Fprintf(W, "Stage %q\n", s.name)
	if len(s.games) == 0 {
		fmt.Fprintln(W, `.LP`)
		fmt.Fprintln(W, `No games took place.`)
		return
	}

	fmt.Fprintln(W, `.NH 2`)
	fmt.Fprintln(W, "Scores")

	fmt.Fprintln(W, `.TS`)
	fmt.Fprintln(W, `tab(/) box center;`)
	fmt.Fprintln(W, `c | c c | c`)
	fmt.Fprintln(W, `----`)
	fmt.Fprintln(W, `l | n n | n`)
	fmt.Fprintln(W, `.`)
	fmt.Fprintln(W, `Agent/Win/Loss/Score`)

	// Order agents in order of score
	sort.Slice(s.agents, func(i, j int) bool {
		iw, il := s.Score(s.agents[i])
		jw, jl := s.Score(s.agents[j])
		return (iw - il) < (jw - jl)
	})

	for _, a := range s.agents {
		w, l := s.Score(a)
		fmt.Fprintf(W, "%s/%d/%d/%d\n", a, w, l, w-l)
	}
	fmt.Fprintln(W, `.TE`)
}

var _ Composable = &scheduler{}
