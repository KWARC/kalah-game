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
	"log"
	"os"

	"go-kgp/conf"
	"go-kgp/db"
	"go-kgp/game"
	"go-kgp/proto"
	"go-kgp/web"
)

// Default file name for the configuration file
const defconf = "server.toml"

func main() {
	var (
		confFile = flag.String("conf", defconf, "Name of configuration file")
		dumpConf = flag.Bool("dump-config", false, "Dump default configuration")
		debug    = flag.Bool("debug", false, "Enable debugging mode")
	)

	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Load the configuration from disk (if available)
	config, err := conf.Open(*confFile, *debug)
	if err != nil && (!os.IsNotExist(err) || *confFile != defconf) {
		log.Fatal(err)
	} else {
		config = conf.Default(*debug)
	}
	config.Debug.Println("Debug logging has been enabled")

	// Dump the configuration onto the disk if requested
	if *dumpConf {
		err = config.Dump(os.Stdout)
		if err != nil {
			log.Fatalln("Failed to dump default configuration:", err)
		}
		os.Exit(0)
	}

	// Initialise the server components
	db.Prepare(config)
	game.Prepare(config)
	web.Prepare(config)
	proto.Prepare(config)

	// Launch the server
	config.Start()
}
