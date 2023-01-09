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
	// Function to determine if an agent passed
	judge func(kgp.Agent, map[kgp.Agent][]kgp.Agent) bool
	// Mapping from an agent to everyone who it managed to defeat
	results map[kgp.Agent][]kgp.Agent
	score   map[kgp.Agent]*score
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

func (s *scheduler) Take(a []isol.ControlledAgent) {
	s.agents = a
}

func (s *scheduler) Give() (next []isol.ControlledAgent) {
	for _, a := range s.agents {
		w, l := s.Score(a)
		if 0 < w-l {
			next = append(next, a)
		}
	}
	return
}

func (s *scheduler) Score(a isol.ControlledAgent) (int, int) {
	if s.score == nil {
		s.score = make(map[kgp.Agent]*score)
		for a, d := range s.results {
			s.score[a] = &score{w: uint(len(d))}
		}
		for _, d := range s.results {
			for _, b := range d {
				var draw bool

				if !draw {
					s.score[b].l++
				}
			}
		}
	}

	S := s.score[a]
	return int(S.w), int(S.l)
}

func (s *scheduler) PrintResults(st *cmd.State, W io.Writer) {
	fmt.Fprintln(W, `.NH 1`)
	fmt.Fprintf(W, "Stage %q\n", s.name)
	fmt.Fprintln(W, `.NH 2`)
	fmt.Fprintf(W, "Scores")

	fmt.Fprintln(W, `.TS`)
	fmt.Fprintln(W, `tab(/) box center;`)
	fmt.Fprintln(W, `c | c c c | c`)
	fmt.Fprintln(W, `-`)
	fmt.Fprintln(W, `l | n n n | n`)
	fmt.Fprintln(W, `.`)
	fmt.Fprintln(W, `Agent/Win/Loss/Draw/Score`)

	// Order agents in order of score
	sort.Slice(s.agents, func(i, j int) bool {
		iw, il := s.Score(s.agents[i])
		jw, jl := s.Score(s.agents[j])
		return (iw - il) < (jw - jl)
	})

	for _, a := range s.agents {
		w, l := s.Score(a)
		d := len(s.agents) - w - l - 1
		fmt.Fprintf(W, `%s/%d/%d/%d/%d\n`, a, w, l, d, w-d)
	}
	fmt.Fprintln(W, `.TE`)
}

var _ Composable = &scheduler{}
