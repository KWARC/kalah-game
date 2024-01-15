// Process Handling
//
// Copyright (c) 2021, 2023, 2024  Philip Kaludercic
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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

var running map[uint64]*exec.Cmd = make(map[uint64]*exec.Cmd)

func start(cli *Client, id uint64, board *Board) {
	defer cli.Respond(id, "yield")
	_, ok := running[id]
	if ok {
		fmt.Fprintln(os.Stderr, "Duplicate ID", id)
		return
	}

	cmd := exec.Command(os.Args[2], os.Args[3:]...)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%d %d %d", len(board.northPits), board.south, board.north)
	for _, v := range board.southPits {
		fmt.Fprintf(&buf, " %d", v)
	}
	for _, v := range board.northPits {
		fmt.Fprintf(&buf, " %d", v)
	}
	fmt.Fprintln(&buf)
	cmd.Stdin = &buf
	cmd.Stderr = os.Stderr

	out, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	running[id] = cmd

	scanner := bufio.NewScanner(out)
	scanner.Split(bufio.ScanWords)
	last := -1
	for scanner.Scan() {
		word := scanner.Text()
		if debug {
			fmt.Fprintf(os.Stderr, "Responded with %v for %d\n", word, id)
		}
		move, err := strconv.Atoi(word)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot parse %v: %s\n", word, err)
			continue
		}
		if move == last {
			continue
		}
		if !board.Legal(SideSouth, move) {
			fmt.Fprintf(os.Stderr, "Attempted to make illegal move %d\n", move)
			continue
		}
		cli.Respond(id, "move", move+1)
		last = move
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input:", err)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}
func stop(id uint64) {
	if cmd, ok := running[id]; ok {
		cmd.Process.Kill()
		delete(running, id)
	}
}
