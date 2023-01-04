// Configuration loading and dumping
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

package conf

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"go-kgp"

	"github.com/BurntSushi/toml"
)

const (
	defconf = "go-kgp.toml"
)

var (
	debug  bool   = false
	silent bool   = false
	dump   bool   = false
	cfile  string = defconf
)

// Parse a configuration from R into CONF
func load(r io.Reader) (*Conf, error) {
	// Load configuration data
	var data conf
	_, err := toml.NewDecoder(r).Decode(&data)
	if err != nil {
		return nil, err
	}

	// Create a configuration object
	c := defaultConfig

	// Apply configuration requests
	c.TCPPort = data.Proto.Port
	c.TCPTimeout = time.Duration(data.Proto.Timeout) * time.Millisecond
	c.Ping = data.Proto.Ping
	c.WebSocket = data.Proto.Websocket
	c.Database = data.Database.File
	c.MoveTimeout = time.Duration(data.Game.Timeout) * time.Millisecond
	c.WebInterface = data.Web.Enabled
	c.About = data.Web.About
	c.WebPort = uint(data.Proto.Port)
	data.Game.Open.Init = c.BoardInit
	data.Game.Open.Size = c.BoardSize
	c.BotTypes = data.Game.Open.Bots

	return &c, nil
}

// Open a configuration file and return it
func Load() (c *Conf) {
	file, err := os.Open(cfile)
	if err != nil {
		if !os.IsNotExist(err) || cfile != defconf {
			log.Fatal(err)
		} else {
			c = &defaultConfig
		}
	} else {
		c, err = load(file)
		if err != nil {
			log.Print(err)
			c = &defaultConfig
		}
	}
	defer file.Close()

	switch {
	case debug:
		kgp.Debug.SetOutput(os.Stderr)
		log.Default().SetFlags(log.LstdFlags | log.Lshortfile)
	case silent:
		log.Default().SetOutput(io.Discard)
	}
	c.Play = make(chan *kgp.Game, 1)
	c.Ctx, c.Kill = context.WithCancel(context.Background())

	// Dump the configuration onto the disk if requested
	if dump {
		err = c.Dump(os.Stdout)
		if err != nil {
			log.Fatalln("Failed to dump default configuration:", err)
		}
		os.Exit(0)
	}

	return c
}

// Serialise the configuration into a writer
func (c *Conf) Dump(wr io.Writer) error {
	var data conf

	data.Database.File = c.Database
	data.Proto.Ping = c.Ping
	data.Proto.Timeout = uint(c.TCPTimeout / time.Millisecond)
	data.Proto.Port = uint(c.TCPPort)
	data.Proto.Websocket = c.WebSocket
	data.Game.Timeout = uint(c.MoveTimeout / time.Millisecond)
	data.Game.Open.Init = c.BoardInit
	data.Game.Open.Size = c.BoardSize
	data.Game.Open.Bots = c.BotTypes
	data.Web.Enabled = c.WebInterface
	data.Web.About = c.About
	data.Web.Port = uint(c.WebPort)

	return toml.NewEncoder(wr).Encode(data)
}
