// Shared State
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

package kgp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"go-kgp"
)

type Manager interface {
	fmt.Stringer
	Start(*State, *kgp.Conf)
	Shutdown()
}

type Scheduler interface {
	Manager

	Schedule(kgp.Agent)
	Unschedule(kgp.Agent)
}

type Database interface {
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

type State struct {
	Games   chan *kgp.Game
	Context context.Context
	Kill    context.CancelFunc
	Running bool

	Scheduler Scheduler
	Database  Database
	Managers  []Manager
}

func MakeMode() *State {
	ctx, kill := context.WithCancel(context.Background())
	return &State{
		Games:   make(chan *kgp.Game),
		Context: ctx,
		Kill:    kill,
	}
}

func (mode *State) Register(m Manager) {
	if mode.Running {
		panic(fmt.Sprintf("Late register: %#v", m))
	}

	switch s := m.(type) {
	case Database:
		mode.Database = s
	case Scheduler:
		mode.Scheduler = s
	}

	mode.Managers = append(mode.Managers, m)
}

func (mode *State) Start(c *kgp.Conf) {
	// Start the service
	for _, m := range mode.Managers {
		kgp.Debug.Printf("Starting %s", m)
		go m.Start(mode, c)
	}
	mode.Running = true

	// Catch an interrupt request...
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	select {
	case <-intr:
		log.Println("Caught interrupt")
	case <-mode.Context.Done():
		log.Println("Requested shutdown")
	}

	done := make(chan struct{})
	go func() {
		// ...and request all managers to shut down.
		kgp.Debug.Println("Waiting for managers to shutdown...")
		for i := len(mode.Managers) - 1; i >= 0; i-- {
			m := mode.Managers[i]
			kgp.Debug.Printf("Shutting %s down", m)
			m.Shutdown()
		}
		done <- struct{}{}
	}()

	select {
	case <-intr:
		log.Println("Forced shutdown")
	case <-done:
		log.Println("Shutting down regularly")
	}
}
