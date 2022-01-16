// Tournament handling via processes
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

// Process starts a client without isolation
type Process struct {
	prefix []string
	run    *exec.Cmd
	dir    string
}

// Run a process by calling "run.sh" and connect to PORT
//
// The output is redirected to a file.
func (p *Process) Start(port string) (err error) {
	if p.prefix != nil {
		args := append(p.prefix[1:], "./run.sh", port)
		p.run = exec.Command(p.prefix[0], args...)
	} else {
		p.run = exec.Command("./run.sh", port)
	}

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

	err = p.run.Start()
	if err != nil {
		log.Printf("Failed to start %v: %s", p.dir, err)
		return
	}

	err = p.run.Wait()
	if err.Error() == "signal: killed" {
		log.Printf("%s was killed", p.dir)
		err = nil
	}

	return
}

// Halt a process by killing it
func (p *Process) Halt() error {
	if p.run != nil {
		err := p.run.Process.Kill()
		p.run.Wait()
		return err
	}
	return nil
}

// Process is not paused
func (*Process) Pause() {}

// Process is not unpaused
func (*Process) Unpause() {}

// Process cannot be awaited, as it cannot be paused
func (*Process) Await() {}
