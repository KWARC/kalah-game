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
	"log"
	"sort"
	"sync/atomic"

	"go-kgp"
	"go-kgp/cmd"
	"go-kgp/sched/isol"
)

type Composable interface {
	cmd.Scheduler
	PrintResults(*cmd.State, io.Writer)
	Take([]isol.ControlledAgent)
	Give() []isol.ControlledAgent
	Score(isol.ControlledAgent) (int, int, int)
}

type Combo struct {
	conf   *cmd.ClosedGameConf
	agents []isol.ControlledAgent
	scheds []Composable
	now    uint64
}

func (c *Combo) Start(st *cmd.State, conf *cmd.Conf) {
	if len(c.agents) == 0 {
		log.Fatal("No agents to run the tournament with")
	}

	next := c.agents
	for len(c.agents) > 0 {
		m := c.scheds[c.now]
		kgp.Debug.Println("Starting ", m, "round with", c.agents)
		m.Take(next)
		kgp.Debug.Println("Starting", m)
		m.Start(st, conf)
		kgp.Debug.Println("Shutting down", m)
		m.Shutdown()
		next = m.Give()

		if atomic.AddUint64(&c.now, 1) >= uint64(len(c.scheds)) {
			break
		}
	}
	kgp.Debug.Println("Ending combo scheduler")
	st.Kill()
}

func (c *Combo) Shutdown() {}

func (c *Combo) Schedule(a kgp.Agent) {
	c.scheds[atomic.LoadUint64(&c.now)].Schedule(a)
}

func (c *Combo) Unschedule(a kgp.Agent) {
	c.scheds[atomic.LoadUint64(&c.now)].Unschedule(a)
}

func (c *Combo) String() string {
	i := atomic.LoadUint64(&c.now)
	if i < uint64(len(c.scheds)) {
		current := c.scheds[i].String()
		return fmt.Sprintf("Combo Scheduler (%s)", current)
	}
	return "Combo Scheduler"
}

func (c *Combo) PrintResults(st *cmd.State, W io.Writer) {
	if atomic.LoadUint64(&c.now) < uint64(len(c.scheds)) {
		return
	}

	fmt.Fprintln(W, ".TL")
	fmt.Fprintln(W, "Results of the AI1 Kalah Tournament")
	fmt.Fprintln(W, ".AB")
	fmt.Fprintln(W, `This report contains the results of the closed AI1
Kalah tournament.  All teams that manage to pass the first stage will
receive bonus points.  The top ten teams receive additional bonus points.
The tournament consists of multiple stages, where agents are disqualified
if they don't perform well enough.`)
	fmt.Fprintln(W, ".AE")

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
		fmt.Fprintln(W, `These agents were disqualified
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

	fmt.Fprintln(W, `.NH 1`)
	fmt.Fprintln(W, "Final score")
	fmt.Fprintln(W, ".LP")
	fmt.Fprintln(W, "The top ten agents are as follows:")

	cache := make(map[isol.ControlledAgent]int)
	score := func(a isol.ControlledAgent) int {
		s, ok := cache[a]
		if ok {
			return s
		}

		var score int
		for _, s := range c.scheds {
			w, l, d := s.Score(a)
			score += 2*w - 0*l + d
		}

		cache[a] = score
		return score
	}

	sort.SliceStable(c.agents, func(i, j int) bool {
		return score(c.agents[i]) > score(c.agents[j])
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
		fmt.Fprintf(W, ".IP %d\n%s (Score: %d)\n", nr, a.String(), sc)
	}

	fmt.Fprintln(W, ".LP")
	fmt.Fprintln(W, "Congratulations to all participating teams!")
}

func (c *Combo) AddAgent(a isol.ControlledAgent) {
	log.Println("Registered agent", a)
	c.agents = append(c.agents, a)
}

func MakeCombo(m ...Composable) *Combo {
	return &Combo{scheds: m}
}
