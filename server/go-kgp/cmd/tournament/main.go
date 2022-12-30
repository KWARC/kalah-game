// Entry point
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

package main

import (
	"flag"
	"fmt"
	"os"

	"go-kgp/conf"
	"go-kgp/db"
	"go-kgp/sched"
)

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Too many arguments passed to %s.\nUsage:\n",
			os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load the configuration from disk (if available)
	config := conf.Load()
	config.Debug.Println("Debug logging has been enabled")

	// Enable the database
	db.Prepare(config)

	// Create a time schedule
	comb := sched.MakeCombo(
		config,
		sched.MakeSanityCheck(config),
		sched.MakeRoundRobin(config, 6, 6),
		sched.MakeRoundRobin(config, 8, 8),
		sched.MakeRoundRobin(config, 10, 10),
		sched.MakeRoundRobin(config, 12, 12),
	)
	config.Register(comb)

	// Start the tournament
	config.Start()
}
