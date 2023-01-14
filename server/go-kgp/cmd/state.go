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

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"go-kgp"
)

type Manager interface {
	fmt.Stringer
	Start(*State, *Conf)
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
	QueryGraph(ctx context.Context, g chan<- *kgp.Game) error
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

func MakeState() *State {
	ctx, kill := context.WithCancel(context.Background())
	return &State{
		Games:   make(chan *kgp.Game),
		Context: ctx,
		Kill:    kill,
	}
}

func (st *State) Register(m Manager) {
	if st.Running {
		panic(fmt.Sprintf("Late register: %#v", m))
	}

	switch s := m.(type) {
	case Database:
		st.Database = s
	case Scheduler:
		st.Scheduler = s
	}

	st.Managers = append(st.Managers, m)
}

func (st *State) Start(c *Conf) {
	// Start the service
	for _, m := range st.Managers {
		log.Printf("Starting %s", m)
		go m.Start(st, c)
	}
	st.Running = true

	// Catch an interrupt request...
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	select {
	case <-intr:
		log.Println("Caught interrupt")
	case <-st.Context.Done():
		log.Println("Requested shutdown")
	}

	done := make(chan struct{})
	go func() {
		// ...and request all managers to shut down.
		kgp.Debug.Println("Waiting for managers to shutdown...")
		for i := len(st.Managers) - 1; i >= 0; i-- {
			m := st.Managers[i]
			log.Printf("Shutting %s down", m)
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
