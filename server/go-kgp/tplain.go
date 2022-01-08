// Tournament handling without isolation
//
// Copyright (c) 2022  Philip Kaludercic
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
	"io"
	"log"
	"os"
	"os/exec"
)

type Plain struct {
	run *exec.Cmd
	dir string
}

func (p *Plain) Run(port string) error {
	build := exec.Command("./build.sh")
	build.Dir = p.dir
	err := build.Run()
	if err != nil && !os.IsNotExist(err) {
		log.Print("Failed to build", p.dir)
		return err
	}

	p.run = exec.Command("./run.sh", port)

	var file *os.File
	file, err = os.Create(p.dir + ".stdout")
	if err != nil {
		log.Printf("Failed to redirect stdout for %s: %s",
			p.dir, err)
		p.run.Stdout = io.Discard
	} else {
		p.run.Stdout = file
		defer file.Close()
	}
	file, err = os.Create(p.dir + ".stderr")
	if err != nil {
		log.Printf("Failed to redirect stderr for %s: %s",
			p.dir, err)
		p.run.Stderr = io.Discard
	} else {
		p.run.Stderr = file
		defer file.Close()
	}
	p.run.Dir = p.dir

	err = p.run.Run()
	if err != nil {
		log.Printf("Failed to start %v: %s", p.dir, err)
		return err
	}
	return nil
}

func (p *Plain) Halt() error {
	if p.run != nil {
		return p.run.Process.Kill()
	}
	return nil
}

// Plain processes are not paused
func (Plain) Sleep() {}
func (Plain) Awake() {}
