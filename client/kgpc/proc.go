// Process Handling
//
// Copyright (c) 2021  Philip Kaludercic
//
// This file is part of kgpc.
//
// kgpc is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License,
// version 3, as published by the Free Software Foundation.
//
// kgpc is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License, version 3, along with kgpc. If not, see
// <http://www.gnu.org/licenses/>

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

var running map[uint64]*os.Process

func start(cli *Client, id uint64, board *Board) {
	_, ok := running[id]
	if ok {
		panic("Duplicate ID")
	}

	args := flag.Args()
	cmd := exec.Command(args[1], args[2:]...)
	running[id] = cmd.Process

	in, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	cmd.Start()

	go func() {
		scanner := bufio.NewScanner(out)
		scanner.Split(bufio.ScanWords)
		for scanner.Scan() {
			word := scanner.Text()
			move, err := strconv.Atoi(word)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cannot parse \"%s\"\n", word)
				continue
			}
			if !board.Legal(SideSouth, move) {
				fmt.Fprintf(os.Stderr, "Attempted to make illegal move %d\n", move)
				continue
			}
			cli.Respond(id, "move", move)
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading input:", err)
		}
	}()

	fmt.Fprintf(in, "%d\n", len(board.northPits))
	fmt.Fprintf(in, "%d\n%d\n", board.south, board.north)
	for _, v := range board.southPits {
		fmt.Fprintf(in, "%d\n", v)
	}
	for _, v := range board.northPits {
		fmt.Fprintf(in, "%d\n", v)
	}

	cmd.Wait()
	cli.Respond(id, "yield")
}

func stop(id uint64) {
	proc, ok := running[id]
	if !ok {
		return
	}

	proc.Kill()
	delete(running, id)
}
