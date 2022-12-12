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

type Mode uint8

const (
	MODE_OPEN = iota
	MODE_TOURNAMENT
)

// Public configuration
type Conf struct {
	Mode  Mode
	Log   *log.Logger
	Debug *log.Logger

	// Protocol Configuration
	TCPPort    uint16        // Port for accepting connections
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
	WebPort      uint16 // Port that the web server listens on

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
	Debug: log.New(io.Discard, "", 0),

	// Protocol Configuration
	TCPPort:    2671,
	Ping:       true,
	TCPTimeout: time.Minute,
	WebSocket:  true,

	// Database configuration
	Database: "kgp.db",

	// Game Configuration
	MoveTimeout: time.Second * 5,
	Mode:        MODE_OPEN,

	// Public Tournament configuration
	BoardInit: 8,
	BoardSize: 8,
	BotTypes:  map[uint]uint{2: 4, 4: 4, 6: 4, 8: 4},

	// Website configuration
	WebInterface: true,
	WebPort:      8080,
	About:        "about.html",
}
