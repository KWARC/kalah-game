// Scheduler Combinator
//
// Copyright (c) 2021, 2022  Philip Kaludercic
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
	"go-kgp/conf"
)

type composable interface {
	initialize(map[kgp.Agent]struct{})
	results() map[kgp.Agent]struct{}
}

type combo struct {
	conf *conf.Conf
	wait sync.WaitGroup
	ag   map[kgp.Agent]struct{}
	ms   []conf.GameManager
	i    atomic.Uint64
}

func (c *combo) Start() {
	c.wait.Add(1)

	next := c.ag
	for m := c.ms[0]; m != nil; m = c.ms[c.i.Add(1)] {
		c, ok := m.(composable)
		if ok {
			c.initialize(next)
		}

		m.Start()
		m.Shutdown()

		if ok {
			next = c.results()
		}
	}
}

func (c *combo) Schedule(a kgp.Agent) {
	c.ms[c.i.Load()].Schedule(a)
}

func (c *combo) Unschedule(a kgp.Agent) {
	c.ms[c.i.Load()].Unschedule(a)
}

func (c *combo) Shutdown() {
	c.wait.Wait()
}

func (*combo) String() string { return "Round Robin" }

func MakeCombo(config *conf.Conf, m ...conf.GameManager) conf.GameManager {
	var man conf.GameManager = &combo{
		conf: config,
		ms:   m,
	}
	return man
}
