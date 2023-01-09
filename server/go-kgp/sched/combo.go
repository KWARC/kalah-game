// Scheduler Combinator
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
	"fmt"
	"io"
	"sort"
	"sync/atomic"

	"go-kgp"
	cmd "go-kgp/cmd"
	"go-kgp/sched/isol"
)

type Composable interface {
	cmd.Scheduler
	PrintResults(*cmd.State, io.Writer)
	Take([]isol.ControlledAgent)
	Give() []isol.ControlledAgent
	Score(isol.ControlledAgent) (int, int)
}

type Combo struct {
	conf   *cmd.ClosedGameConf
	agents []isol.ControlledAgent
	scheds []Composable
	now    uint64
}

func (c *Combo) Start(mode *cmd.State, conf *cmd.Conf) {
	next := c.agents
	for {
		m := c.scheds[atomic.LoadUint64(&c.now)]
		m.Take(next)
		m.Start(mode, conf)
		m.Shutdown()
		next = m.Give()

		if atomic.AddUint64(&c.now, 1) >= uint64(len(c.scheds)) {
			break
		}
	}
}

func (c *Combo) Shutdown() {}

func (c *Combo) Schedule(a kgp.Agent) {
	c.scheds[atomic.LoadUint64(&c.now)].Schedule(a)
}

func (c *Combo) Unschedule(a kgp.Agent) {
	c.scheds[atomic.LoadUint64(&c.now)].Unschedule(a)
}

func (c *Combo) String() string {
	return c.scheds[atomic.LoadUint64(&c.now)].String()
}

func (c *Combo) PrintResults(st *cmd.State, W io.Writer) {
	fmt.Fprintln(W, ".RP")
	fmt.Fprintln(W, ".TL")
	fmt.Fprintln(W, "Results of the AI1 Kalah Tournament")
	fmt.Fprintln(W, ".AB")
	fmt.Fprintln(W, `This report contains the results of the closed AI1
Kalah tournament.  All teams that manage to pass the first stage will
receive bonus points.  The top ten teams receive additional bonus points.
The tournament consists of multiple stages, where agents are disqualified
if they don't perform well enough.  The final score is calculated by
summing up the total number of games won, and subtracting the total number
of games an agent lost.`)
	fmt.Fprintln(W, ".AE")
	fmt.Fprintln(W, ".NH 1")

	for i, s := range c.scheds {
		s.PrintResults(st, W)

		var (
			prev []isol.ControlledAgent
			curr []isol.ControlledAgent
		)
		if i == 0 {
			prev = c.agents
		} else {
			prev = c.scheds[i-1].Give()
		}
		curr = s.Give()

		fmt.Fprintln(W, ".PP")
		fmt.Fprintln(W, `The following agents were disqualified
for failing to meet the necessary criteria for proceeding to the
next round:`)
		c := 0
		for _, a := range prev {
			for _, b := range curr {
				if a == b {
					goto skip
				}
			}

			fmt.Fprintln(W, `.IP \(bu`)
			fmt.Fprintln(W, a.String())
			c++
		skip:
			// the agent was not disqualified
		}
		if c == 0 {
			fmt.Fprintln(W, ".QP")
			fmt.Fprintln(W, ".I none ) (")
		}
	}

	fmt.Fprintln(W, "Final score")
	fmt.Fprintln(W, ".PP")
	fmt.Fprintln(W, "The top ten agents are as follows:")

	cache := make(map[isol.ControlledAgent]int)
	score := func(a isol.ControlledAgent) int {
		s, ok := cache[a]
		if ok {
			return s
		}

		var score int
		for _, s := range c.scheds {
			w, l := s.Score(a)
			score += w - l
		}

		cache[a] = score
		return score
	}

	sort.Slice(c.agents, func(i, j int) bool {
		return score(c.agents[i]) < score(c.agents[j])
	})

	var (
		last = score(c.agents[0])
		nr   = 1
	)
	for i, a := range c.agents {
		if i >= 10 {
			break
		}

		sc := score(a)
		if sc < last {
			nr++
		}
		last = sc
		fmt.Fprintf(W, `.IP %d\n%s (Score: %d)`, nr, a.String(), sc)
	}

	fmt.Fprintln(W, ".PP")
	fmt.Fprintln(W, "Congratulations to all participating teams!")
}

func (c *Combo) AddAgent(a isol.ControlledAgent) {
	c.agents = append(c.agents, a)
}

func MakeCombo(m ...Composable) *Combo {
	return &Combo{scheds: m}
}
