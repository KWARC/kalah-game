// Configuration loading and dumping
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

package conf

import (
	"io"
	"os"
	"time"

	"go-kgp"

	"github.com/BurntSushi/toml"
)

var debug bool = false

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
	if debug {
		c.Log.SetOutput(os.Stderr)
	}
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
	for _, d := range data.Game.Open.Bots {
		if _, ok := c.BotTypes[d]; !ok {
			c.BotTypes[d] = 0
		}
		c.BotTypes[d]++
	}

	return &c, nil
}

// Open a configuration file and return it
func Open(name string) (*Conf, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	c, err := load(file)
	c.Play = make(chan *kgp.Game, 1)
	return c, err
}

// Return a reference to the default configuration
func Default() *Conf {
	conf := &defaultConfig
	conf.Play = make(chan *kgp.Game, 1)
	if debug {
		conf.Log.SetOutput(os.Stderr)
	}

	return conf
}

// Serialise the configuration into a writer
func (c *Conf) Dump(wr io.Writer) error {
	var data conf

	data.Database.File = c.Database
	data.Proto.Ping = c.Ping
	data.Proto.Timeout = uint(c.TCPTimeout / time.Millisecond)
	data.Proto.Port = uint(c.TCPPort)
	data.Game.Timeout = uint(c.MoveTimeout / time.Millisecond)
	data.Game.Open.Init = c.BoardInit
	data.Game.Open.Size = c.BoardSize
	for d, n := range c.BotTypes {
		for i := uint(0); i < n; i++ {
			data.Game.Open.Bots = append(data.Game.Open.Bots, d)
		}
	}
	data.Web.Enabled = c.WebInterface
	data.Web.About = c.About
	data.Web.Port = uint(c.WebPort)

	return toml.NewEncoder(wr).Encode(data)
}
