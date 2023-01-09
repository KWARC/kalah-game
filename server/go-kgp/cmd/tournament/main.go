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
	"io"
	"log"
	"os"
	"regexp"
	"strconv"

	"go-kgp/cmd"
	"go-kgp/db"
	"go-kgp/sched"
	"go-kgp/sched/isol"
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

	// Create schedule
	var (
		prog = []sched.Composable{sched.MakeSanityCheck()}
		pat  = regexp.MustCompile(`^(\d+),(\d+)(?:,(?:(0?\.\d+)|(\d+)))?$`)
	)
	for i, st := range conf.Game.Closed.Stages {
		mat := pat.FindStringSubmatch(st)
		if mat == nil {
			log.Fatalf("Invalid stage %v (%d)", st, i+1)
		}

		n, err := strconv.Atoi(mat[1])
		if err != nil {
			log.Panic(err)
		}
		m, err := strconv.Atoi(mat[1])
		if err != nil {
			log.Panic(err)
		}

		rr := sched.MakeRoundRobin(uint(n), uint(m))
		prog = append(prog, rr)
	}
	combo := sched.MakeCombo(prog...)

	// Check if the -dir flag was used and handle it
	if *dir != "" {
		dent, err := os.ReadDir(*dir)
		if err != nil {
			log.Fatal(err)
		}

		for _, ent := range dent {
			if !ent.IsDir() {
				continue
			}

			a := isol.MakeDockerAgent(ent.Name())
			combo.AddAgent(a)
		}

		if len(conf.Game.Closed.Images) > 0 {
			log.Print("Ignoring image list from configuration")
		}
	} else {
		for _, name := range conf.Game.Closed.Images {
			a := isol.MakeDockerAgent(name)
			combo.AddAgent(a)
		}
	}

	// Load components
	db.Register(mode, conf)
	mode.Register(combo)

	// Print results
	var out io.Writer
	if res := conf.Game.Closed.Result; res != "" {
		file, err := os.Open(res)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		out = file
	} else {
		out = os.Stdout
	}

	// Start the tournament
	mode.Start(conf)

	// Print results
	combo.PrintResults(mode, out)
}
