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
	"sync"
	"sync/atomic"

	"go-kgp"
	cmd "go-kgp/cmd"
)

type composable interface {
	initialize(map[kgp.Agent]struct{})
	results() map[kgp.Agent]struct{}
}

type combo struct {
	wait sync.WaitGroup
	ag   map[kgp.Agent]struct{}
	ms   []cmd.Scheduler
	i    uint64
}

func (c *combo) Start(mode *cmd.State, conf *kgp.Conf) {
	c.wait.Add(1)

	next := c.ag
	for m := c.ms[0]; m != nil; m = c.ms[atomic.AddUint64(&c.i, 1)] {
		c, ok := m.(composable)
		if ok {
			c.initialize(next)
		}

		m.Start(mode, conf)
		m.Shutdown()

		if ok {
			next = c.results()
		}
	}
}

func (c *combo) Schedule(a kgp.Agent) {
	c.ms[atomic.LoadUint64(&c.i)].Schedule(a)
}

func (c *combo) Unschedule(a kgp.Agent) {
	c.ms[atomic.LoadUint64(&c.i)].Unschedule(a)
}

func (c *combo) Shutdown() {
	c.wait.Wait()
}

func (*combo) String() string { return "Round Robin" }

func MakeCombo(m ...cmd.Scheduler) cmd.Scheduler {
	return cmd.Scheduler(&combo{ms: m})
}
