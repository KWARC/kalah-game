// Entry point
//
// Copyright (c) 2024  Philip Kaludercic
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
	"strconv"

	cmd "go-kgp/cmd"
	"go-kgp/db"
	"go-kgp/proto"
	"go-kgp/sched"
	"go-kgp/web"
)

func main() {
	depth := flag.Uint64("search-depth", 8, "Plies the MinMax bot should search")
	accuracy := flag.Float64("search-accuracy", 1.0, "Percentage of moves that should be random")
	flag.Parse()

	// Create a server mode (state) and load configuration
	var conf cmd.Conf
	st := cmd.MakeState()
	conf.Load()

	// Load components
	db.Register(st, &conf)
	st.Register(sched.MakeSpecificFifo(uint(*depth), *accuracy))
	proto.Register(st, &conf)
	web.Register(st)

	// Launch the server
	st.Start(&conf)
}
