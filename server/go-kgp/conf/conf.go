// Configuration Specification and Management
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
	"context"
	"flag"
	"io"
	"log"
	"time"

	"go-kgp"
)

// Internal representation
type conf struct {
	Debug    bool `toml:"debug"`
	Database struct {
		File string `toml:"file"`
	} `toml:"database"`
	Proto struct {
		Port      uint `toml:"port"`
		Ping      bool `toml:"ping"`
		Timeout   uint `toml:"timeout"`
		Websocket bool `toml:"websocket"`
	} `toml:"proto"`
	Game struct {
		Timeout uint   `toml:"timeout"`
		Mode    string `toml:"mode"`
		Open    struct {
			Init uint   `toml:"init"`
			Size uint   `toml:"init"`
			Bots []uint `toml:"bots"`
		} `toml:"open"`
	} `toml:"game"`
	Web struct {
		Enabled bool   `toml:"enabled"`
		Port    uint   `toml:"port"`
		About   string `toml:"about"`
		Data    string `toml:"data"`
	} `toml:"web"`
}

// Public configuration
type Conf struct {
	Log   *log.Logger
	Debug *log.Logger
	Ctx   context.Context
	Kill  context.CancelFunc

	// Protocol Configuration
	TCPPort    uint          // Port for accepting connections
	Ping       bool          // Should KGP send ping requests
	TCPTimeout time.Duration // Disconnect after this timeout
	WebSocket  bool          // Are Websocket connection enabled

	// Database Configuration
	Database string // File to store the database
	DB       DatabaseManager

	// Game Configuration
	MoveTimeout time.Duration
	Play        chan *kgp.Game
	GM          GameManager

	// Website configuration
	WebInterface bool   // Has the web interface been enabled?
	Data         string // Path to a data directory
	About        string // Path to a template file containing the "about" site
	WebPort      uint   // Port that the web server listens on

	// Public Tournament configuration
	BoardInit uint
	BoardSize uint
	BotTypes  map[uint]uint

	// Internal state
	man []Manager // List of system managers
	run bool      // Running flag
}

// Configuration object used by default
var defaultConfig = Conf{
	Log:   log.Default(),
	Debug: log.New(io.Discard, "[debug]", log.Ltime|log.Lshortfile|log.Lmicroseconds),

	// Protocol Configuration
	TCPPort:    2671,
	Ping:       true,
	TCPTimeout: time.Second * 20,
	WebSocket:  true,

	// Database configuration
	Database: "data.db",

	// Game Configuration
	MoveTimeout: time.Second * 5,

	// Public Tournament configuration
	BoardInit: 8,
	BoardSize: 8,
	BotTypes:  map[uint]uint{2: 4, 4: 4, 6: 4, 8: 4},

	// Website configuration
	WebInterface: true,
	WebPort:      8080,
	About:        "",
}

func init() {
	flag.StringVar(&defaultConfig.About, "about", defaultConfig.About,
		"File to use for the about template")
	flag.UintVar(&defaultConfig.WebPort, "wwwport", defaultConfig.WebPort,
		"Port to use for the HTTP server")
	flag.UintVar(&defaultConfig.BoardInit, "board-init", defaultConfig.BoardInit,
		"Default number of stones to use for Kalah boards")
	flag.UintVar(&defaultConfig.BoardSize, "board-size", defaultConfig.BoardSize,
		"Default size to use for Kalah boards")
	flag.StringVar(&defaultConfig.Database, "db", defaultConfig.Database,
		"File to use for the database")
	flag.BoolVar(&defaultConfig.Ping, "ping", defaultConfig.Ping,
		"Enable ping as a keepalive check")
	flag.BoolVar(&defaultConfig.WebSocket, "websocket", defaultConfig.WebSocket,
		"Enable WebSocket connections")
	flag.UintVar(&defaultConfig.TCPPort, "tcpport", defaultConfig.TCPPort,
		"Port to use for TCP connections")
	flag.StringVar(&defaultConfig.Data, "data", defaultConfig.Data,
		"Directory to use for hosting /data/ requests")
	flag.BoolVar(&debug, "debug", dump, "Enable debug output")
	flag.BoolVar(&dump, "dump-config", dump, "Dump configuration to standard output")
	flag.StringVar(&cfile, "conf", cfile, "Path to configuration file")
}
