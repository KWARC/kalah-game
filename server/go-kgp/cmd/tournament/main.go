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
	"flag"
	"fmt"
	"log"
	"os"

	cmd "go-kgp/cmd"
	"go-kgp/db"
	"go-kgp/sched"
)

func main() {
	dir := flag.String("dir", ".", "Agent directory")

	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Too many arguments passed to %s.\nUsage:\n",
			os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create a server mode (state) and load configuration
	mode := cmd.MakeMode()
	conf := cmd.LoadConf()

	// Check if the -dir flag was used and handle it
	if dir != nil {
		dent, err := os.ReadDir(*dir)
		if err != nil {
			log.Fatal(err)
		}

		for _, ent := range dent {
			if !ent.IsDir() {
				continue
			}
		}
	}

	// Load components
	db.Register(mode)
	mode.Register(sched.MakeCombo(
		sched.MakeSanityCheck(),
		sched.MakeRoundRobin(6, 6),
		sched.MakeRoundRobin(8, 8),
		sched.MakeRoundRobin(10, 10),
		sched.MakeRoundRobin(12, 12),
	))

	// Start the tournament
	mode.Start(conf)
}
