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

	cmd "go-kgp/cmd"
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

	// Create a server mode (state) and load configuration
	mode := cmd.MakeMode()
	conf := cmd.LoadConf()

	// Load components
	db.Register(mode)
	mode.Register(sched.MakeFIFO())
	proto.Register(mode, conf)
	web.Register(mode)

	// Launch the server
	mode.Start(conf)
}
