// Configuration Management
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

package conf

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"go-kgp"
)

type Manager interface {
	fmt.Stringer
	Start()
	Shutdown()
}

type GameManager interface {
	Manager

	Schedule(kgp.Agent)
	Unschedule(kgp.Agent)
}

type DatabaseManager interface {
	Manager

	// Access interface
	QueryUsers(context.Context, chan<- *kgp.User, int)
	QueryUser(context.Context, int) *kgp.User
	QueryUserToken(context.Context, string) *kgp.User
	QueryGames(context.Context, int, chan<- *kgp.Game, int)
	QueryGame(context.Context, int, chan<- *kgp.Game, chan<- *kgp.Move)

	// Store interface
	SaveMove(context.Context, *kgp.Move)
	SaveGame(context.Context, *kgp.Game)

	// Miscellaneous
	DrawGraph(context.Context, io.Writer) error
}

func (c *Conf) Register(m Manager) {
	if c.run {
		panic(fmt.Sprintf("Late register: %#v", m))
	}

	switch s := m.(type) {
	case DatabaseManager:
		c.DB = s
	case GameManager:
		c.GM = s
	}

	c.man = append(c.man, m)
}

func (c *Conf) Start() {
	// Start the service
	for _, m := range c.man {
		c.Debug.Printf("Starting %s", m)
		go m.Start()
	}
	c.run = true

	// Catch an interrupt request...
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	select {
	case <-intr:
		c.Debug.Println("Caught interrupt")
	case <-c.Ctx.Done():
		c.Debug.Println("Requested shutdown")
	}

	// ...and request all managers to shut down.
	c.Debug.Println("Waiting for managers to shutdown...")
	for _, m := range c.man {
		c.Debug.Printf("Shutting %s down", m)
		m.Shutdown()
	}
	c.Debug.Println("Shutting down")
}
