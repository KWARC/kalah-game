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
	"os"

	"go-kgp"
	"go-kgp/conf"
	"go-kgp/db"
	"go-kgp/proto"
	"go-kgp/sched"
	"go-kgp/web"
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
	kgp.Debug.Println("Debug logging has been enabled")

	// Enable the database
	db.Prepare(config)

	// Enable the web interface
	web.Prepare(config)

	// Allow TCP connections
	proto.Prepare(config)

	// Use the random scheduler
	config.Register(conf.GameManager(sched.MakeFIFO(config)))

	// Launch the server
	config.Start()
}
