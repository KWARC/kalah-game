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
	"math/rand"
	"runtime"
	"sort"
	"sync"

	"go-kgp"
	"go-kgp/cmd"
	"go-kgp/game"
	"go-kgp/sched/isol"
)

type score struct{ w, l, d uint }

type scheduler struct {
	name   string
	desc   string
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

func (s *scheduler) Start(st *cmd.State, conf *cmd.Conf) {
	games := s.schedule(s.agents)
	rand.Shuffle(len(games), func(i, j int) {
		games[i], games[j] = games[j], games[i]
	})
	sched := make(chan *kgp.Game, len(games))
	for _, g := range games {
		sched <- g
	}
	s.wait.Add(len(games))
	kgp.Debug.Println("Staring scheduler", s, "with", len(games), "games")

	var (
		lock sync.Mutex
		done uint
	)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for g := range sched {
				var (
					err   error
					south = g.South
					north = g.North
				)
				g.South, err = isol.Start(st, conf, g.South)
				if err != nil {
					log.Print(err)
					goto skip
				}
				g.North, err = isol.Start(st, conf, g.North)
				if err != nil {
					log.Print(err)
					goto skip
				}

				err = game.Play(g, st, conf)
				if err != nil {
					log.Print(err)
				}
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
				done++
				log.Printf("%d/%d (%s vs. %s) -> %s", done, len(games), south, north, g.State.String())
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
		w, l, d := s.Score(a)
		if w+d >= l {
			next = append(next, a)
		}
	}
	return
}

func (s *scheduler) Score(a isol.ControlledAgent) (int, int, int) {
	if s.score == nil {
		s.score = make(map[kgp.Agent]*score)

		for _, agent := range s.agents {
			s.score[agent] = &score{}
		}

		for _, game := range s.games {
			if S, ok := s.score[game.South]; ok {
				switch game.State {
				case kgp.NORTH_WON:
					S.l += 1
				case kgp.SOUTH_WON:
					S.w += 1
				case kgp.UNDECIDED:
					S.d += 1
				case kgp.SOUTH_RESIGNED:
					S.l += 1
				}
			}
			if S, ok := s.score[game.North]; ok {
				switch game.State {
				case kgp.NORTH_WON:
					S.w += 1
				case kgp.SOUTH_WON:
					S.l += 1
				case kgp.UNDECIDED:
					S.d += 1
				case kgp.NORTH_RESIGNED:
					S.l += 1
				}
			}
		}
	}

	if sc, ok := s.score[a]; ok {
		return int(sc.w), int(sc.l), int(sc.d)
	}
	return 0, 0, 0
}

func (s *scheduler) PrintResults(st *cmd.State, W io.Writer) {
	fmt.Fprintln(W, `.NH 1`)
	fmt.Fprintf(W, "Stage %q\n", s.name)
	if len(s.games) == 0 {
		fmt.Fprintln(W, `.LP`)
		fmt.Fprintln(W, `No games took place.`)
		return
	}
	fmt.Fprintln(W, `.LP`)
	fmt.Fprintln(W, s.desc)

	// Order agents in order of score
	sort.SliceStable(s.agents, func(i, j int) bool {
		iw, il, id := s.Score(s.agents[i])
		is := 2*iw - 2*il + id
		jw, jl, jd := s.Score(s.agents[j])
		js := 2*jw - 2*jl + jd
		return is < js
	})

	fmt.Fprintln(W, `.NH 2`)
	fmt.Fprintln(W, "Scores")

	fmt.Fprintln(W, `.TS`)
	fmt.Fprintln(W, `tab(/) box center;`)
	fmt.Fprintln(W, `c | c c c | c`)
	fmt.Fprintln(W, `-----`)
	fmt.Fprintln(W, `l | n n n | n`)
	fmt.Fprintln(W, `.`)
	fmt.Fprintln(W, `Agent/Win/Loss/Draw/Score`)

	for _, a := range s.agents {
		w, l, d := s.Score(a)
		s := 2*w - 2*l + d
		fmt.Fprintf(W, "%s/%d/%d/%d/%d\n", a, w, l, d, s)
	}
	fmt.Fprintln(W, `.TE`)

	fmt.Fprintln(W, `.NH 2`)
	fmt.Fprintln(W, "Game Log")

	fmt.Fprintln(W, `.TS H`)
	fmt.Fprintln(W, `tab(/) box center;`)
	fmt.Fprintln(W, `c | c c | c c c`)
	fmt.Fprintln(W, `------`)
	fmt.Fprintln(W, `n | l l | n n n`)
	fmt.Fprintln(W, `.`)
	fmt.Fprintln(W, `.TH`)
	fmt.Fprintln(W, `Nr./South Agent/North Agent/South/North/Diff.`)

	for i, g := range s.games {
		south := int(g.Board.Store(kgp.South))
		north := int(g.Board.Store(kgp.North))
		fmt.Fprintf(W, "%d/%s/%s/%d/%d/%d\n", i+1,
			g.South.User().Name,
			g.North.User().Name,
			south, north, south-north)
	}
	fmt.Fprintln(W, `.TE`)
}

var _ Composable = &scheduler{}
