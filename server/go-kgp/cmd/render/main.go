// Entry point
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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"go-kgp"

	"go-kgp/cmd"
	"go-kgp/db"
	"go-kgp/web"
)

type render struct{}

func (*render) String() string { return "Render" }
func (*render) Shutdown()      {} // noop

func (*render) Start(st *cmd.State, conf *cmd.Conf) {
	ctx := context.Background()
	games := make(chan *kgp.Game)
	go st.Database.QueryGames(ctx, 0, games, 0)

	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for game := range games {
				size, init := game.Board.Type()
				fn := fmt.Sprintf("game-%d-%d,%d-%s-%s.html",
					game.Id, size, init,
					game.South.User().Name,
					game.North.User().Name,
				)
				file, err := os.Create(fn)
				err = web.RenderGame(st, ctx, int(game.Id), file)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("Render", file.Name())
				file.Close()
			}
			wg.Done()
		}()
		wg.Add(1)
	}
	wg.Wait()
	os.Exit(0)
}

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Too many arguments passed to %s.\nUsage:\n",
			os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create a server mode (state) and load configuration
	var conf cmd.Conf
	st := cmd.MakeState()
	conf.Load()

	// Load database then the render engine
	db.Register(st, &conf)
	web.Register(st)
	st.Register(&render{})

	st.Start(&conf)
}
