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
	"go-kgp"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"

	"go-kgp/cmd"
	"go-kgp/db"
	"go-kgp/sched"
	"go-kgp/sched/isol"
)

func main() {
	dir := flag.String("dir", "", "Agent directory")
	auto := flag.Bool("auto", false, "Build containers in the agent directory.")

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
		m, err := strconv.Atoi(mat[2])
		if err != nil {
			log.Panic(err)
		}

		rr := sched.MakeRoundRobin(uint(n), uint(m))
		prog = append(prog, rr)
	}
	combo := sched.MakeCombo(prog...)

	// Check if the -dir flag was used and handle it
	st.Register(sched.MakeNoOp())
	if *dir != "" {
		dent, err := os.ReadDir(*dir)
		if err != nil {
			log.Fatal(err)
		}

		for _, ent := range dent {
			if !ent.IsDir() {
				continue
			}

			var build string
			if *auto {
				build = path.Join(*dir, ent.Name())
			}
			a := isol.MakeDockerAgent(ent.Name(), build)

			// Check if the container works
			i, err := isol.Start(st, &conf, a)
			if err != nil {
				log.Println(err)
				continue
			}
			err = isol.Shutdown(i)
			if err != nil {
				log.Println(err)
				continue
			}

			combo.AddAgent(a)
		}

		if len(conf.Game.Closed.Images) > 0 {
			log.Print("Ignoring image list from configuration")
		}
	} else {
		for _, name := range conf.Game.Closed.Images {
			a := isol.MakeDockerAgent(name, "")

			// Check if the container works
			i, err := isol.Start(st, &conf, a)
			if err != nil {
				log.Println(err)
				continue
			}
			err = isol.Shutdown(i)
			if err != nil {
				log.Println(err)
				continue
			}

			combo.AddAgent(a)
		}
	}

	// Load components
	db.Register(st, &conf)
	st.Register(combo)

	// Print results
	var (
		cmd *exec.Cmd
		out io.Writer
	)
	if res := conf.Game.Closed.Result; res != "" {
		kgp.Debug.Println("Writing results to", res)
		file, err := os.Create(res)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		out = file

		var dev string
		switch path.Ext(res) {
		case ".pdf":
			dev = "-Tpdf"
		case ".ps":
			dev = "-Tps"
		case ".html":
			dev = "-Txhtml"
		case ".txt":
			dev = "-Tutf8"
		default:
			goto skip
		}
		kgp.Debug.Println("Preparing groff with", dev)
		cmd = exec.Command("groff", dev, "-ms", "-t")

		cmd.Stdout = file
		out, err = cmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		out = os.Stdout
	}
skip:

	// Start the tournament
	st.Start(&conf)

	// Print results
	if cmd != nil {
		cmd.Start()
	}
	combo.PrintResults(st, out)
	if cmd != nil {
		cmd.Wait()
	}
}
