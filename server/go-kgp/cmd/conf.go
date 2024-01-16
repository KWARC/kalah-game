// Configuration
//
// Copyright (c) 2021, 2022, 2023, 2024  Philip Kaludercic
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

package cmd

import (
	"flag"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	"go-kgp"

	"github.com/BurntSushi/toml"
)

const defconf = "go-kgp.toml"

func init() {
	def := &defaultConfig

	flag.StringVar(&def.Web.About, "about", def.Web.About,
		"File to use for the about template")
	flag.UintVar(&def.Web.Port, "wwwport", def.Web.Port,
		"Port to use for the HTTP server")
	flag.StringVar(&def.Web.Data, "data", def.Web.Data,
		"Directory to use for hosting /data/ requests")
	flag.BoolVar(&def.Web.WebSocket, "websocket", def.Web.WebSocket,
		"Enable WebSocket connections")

	flag.UintVar(&def.Game.Open.Bots, "bots", def.Game.Open.Bots,
		"Number of concurrent bots the server provides")
	flag.UintVar(&def.Game.Open.Init, "board-init", def.Game.Open.Init,
		"Default number of stones to use for Kalah boards")
	flag.UintVar(&def.Game.Open.Size, "board-size", def.Game.Open.Size,
		"Default size to use for Kalah boards")
	flag.DurationVar(&def.Game.Timeout, "timeout", def.Game.Timeout,
		"Time a client has to make a move")

	flag.StringVar(&def.Database.File, "db", def.Database.File,
		"File to use for the database")
	flag.DurationVar(&def.Database.Cleanup, "cleanup", def.Database.Cleanup,
		"How frequently to clean up old games (older than a week)")

	flag.BoolVar(&def.Proto.Ping, "ping", def.Proto.Ping,
		"Enable ping as a keepalive check")
	flag.UintVar(&def.Proto.Port, "tcpport", def.Proto.Port,
		"Port to use for TCP connections")

	flag.BoolVar(&debug, "debug", dump, "Enable debug output")
	flag.BoolVar(&silent, "silent", silent, "Enable verbose output")
	flag.BoolVar(&dump, "dump-config", dump, "Dump configuration to standard output")
	flag.StringVar(&cfile, "conf", cfile, "Path to configuration file")
}

type DatabaseConf struct {
	File    string        `toml:"file"`
	Cleanup time.Duration `toml:"cleanup"`
}

type ProtoConf struct {
	Port    uint          `toml:"port"`
	Ping    bool          `toml:"ping"`
	Timeout time.Duration `toml:"timeout"`
}

type OpenGameConf struct {
	Bots uint `toml:"bots"`
	Init uint `toml:"init"`
	Size uint `toml:"size"`
}

type ClosedGameConf struct {
	Images []string      `toml:"images"`
	Stages []string      `toml:"stages"`
	Result string        `toml:"result"`
	Sanity bool          `toml:"sanity"`
	Warmup time.Duration `toml:"warmup"`
	Memory uint          `toml:"memory"`
	CPUs   uint          `toml:"cpus"`
}

type GameConf struct {
	Timeout time.Duration  `toml:"timeout"`
	Open    OpenGameConf   `toml:"open"`
	Closed  ClosedGameConf `toml:"closed"`
}

type WebConf struct {
	Enabled   bool   `toml:"enabled"`
	Port      uint   `toml:"port"`
	WebSocket bool   `toml:"websocket"`
	About     string `toml:"about,omitempty"`
	Data      string `toml:"data,omitempty"`
}

// Internal representation
type Conf struct {
	Database DatabaseConf `toml:"database"`
	Proto    ProtoConf    `toml:"proto"`
	Game     GameConf     `toml:"game"`
	Web      WebConf      `toml:"web"`
}

// Configuration object used by default
var defaultConfig = Conf{
	Proto: ProtoConf{
		Port:    2671,
		Ping:    true,
		Timeout: time.Second * 20,
	},
	Database: DatabaseConf{
		File:    "data.db",
		Cleanup: time.Hour * 4,
	},
	Game: GameConf{
		Timeout: time.Second * 5,
		Open: OpenGameConf{
			Bots: uint(runtime.NumCPU()/2 + 1),
			Init: 8,
			Size: 8,
		},
		Closed: ClosedGameConf{
			Stages: []string{"6,6", "8,8", "10,10", "12,12"},
			Sanity: true,
			Result: "result.pdf",
			Warmup: 10 * time.Second,
			Memory: 1024 * 1024 * 1024,
			CPUs:   1,
		},
	},
	Web: WebConf{
		Enabled:   true,
		WebSocket: true,
		Port:      8080,
	},
}

var (
	debug  = false
	silent = false
	dump   = false
	cfile  = defconf
)

// Open a configuration file and return it
func (c *Conf) Load() {
	file, err := os.Open(cfile)
	if err != nil {
		if !os.IsNotExist(err) || cfile != defconf {
			log.Fatal(err)
		} else {
			*c = defaultConfig
		}
	} else {
		*c = defaultConfig
		_, err := toml.NewDecoder(file).Decode(&c)
		if err != nil {
			log.Print(err)
			*c = defaultConfig
		}
	}
	defer file.Close()

	switch {
	case debug:
		kgp.Debug.SetOutput(os.Stderr)
		log.Default().SetFlags(log.LstdFlags | log.Lshortfile)
		kgp.Debug.Println("Debug logging has been enabled")
	case silent:
		log.Default().SetOutput(io.Discard)
	}

	// Dump the configuration onto the disk if requested
	if dump {
		err = c.Dump(os.Stdout)
		if err != nil {
			log.Fatalln("Failed to dump default configuration:", err)
		}
		os.Exit(0)
	}
}

// Serialise the configuration into a writer
func (c *Conf) Dump(wr io.Writer) error {
	return toml.NewEncoder(wr).Encode(c)
}
