// Configuration Specification and Management
//
// Copyright (c) 2021  Philip Kaludercic
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
	"context"
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
)

type GameConf struct {
	Sizes   []uint `toml:"sizes"`
	Stones  []uint `toml:"stones"`
	Timeout uint   `toml:"timeout"`
}

type WSConf struct {
	Enabled bool `toml:"enabled"`
}

type WebConf struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    uint   `toml:"port"`
	Limit   uint   `toml:"limit"`
	About   string `toml:"about"`
	server  *http.Server
	WS      WSConf `toml:"websocket"`

	dbact chan<- DBAction
}

type TCPConf struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    uint   `toml:"port"`
	Ping    bool   `toml:"ping"`
	Timeout uint   `toml:"timeout"`
	Retries uint   `toml:"retries"`
	Endless bool   `toml:"endless"`

	cancel context.CancelFunc
}

type DBConf struct {
	File    string `toml:"file"`
	Threads uint   `toml:"threads"`
	Mode    string `toml:"mode"`

	act chan DBAction
	db  *sql.DB
}

type Conf struct {
	Debug    bool     `toml:"debug"`
	Database DBConf   `toml:"database"`
	Game     GameConf `toml:"game"`
	Web      WebConf  `toml:"web"`
	TCP      TCPConf  `toml:"tcp"`
	EarlyWin bool     `toml:"earlywin"`

	file string
}

var defaultConfig = Conf{
	Debug:    false,
	EarlyWin: true,
	Game: GameConf{
		Sizes:   []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		Stones:  []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		Timeout: 5,
	},
	Web: WebConf{
		Enabled: true,
		Host:    "0.0.0.0",
		Port:    8080,
		Limit:   50,
		About:   "",
		WS: WSConf{
			Enabled: false,
		},
	},
	Database: DBConf{
		File:    "kalah.sql",
		Threads: 1,
		Mode:    "rwc",
	},
	TCP: TCPConf{
		Enabled: true,
		Host:    "0.0.0.0",
		Port:    2671,
		Ping:    true,
		Timeout: 20,
		Retries: 8,
		Endless: true,
	},
}

func readConf(name string, conf *Conf) error {
	debug.Print("Loading configuration")

	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = toml.NewDecoder(file).Decode(conf)
	conf.file = name
	return err
}

func openConf(name string) (*Conf, error) {
	var conf Conf
	err := readConf(name, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func start(conf *Conf) {
	c := make(chan os.Signal, 1)
	go signal.Notify(c, syscall.SIGUSR1, os.Interrupt)

	for {
		// Enabled or disable debugging
		if conf.Debug {
			debug.SetOutput(os.Stderr)
			debug.Print("Enabled debugging output")
		} else {
			debug.Print("Disabling debugging output")
			debug.SetOutput(io.Discard)
		}

		// XXX: Setting a global variable here goes against
		// the centralised, non-global configuration design.
		// A way should be found to circumvent this direct
		// approach, and only have games end early that were
		// started when EarlyWin was enabled (and vice versa).
		earlyWin = conf.EarlyWin

		// Start the queue manager
		go conf.Game.init()
		// Start the database manager
		go conf.Database.init()
		// Start accepting TCP requests
		go conf.TCP.init()
		// Start the web-server and websocket
		go conf.Web.init()

		sig := <-c
		// Stop accepting TCP connections
		conf.TCP.deinit()
		// Shut down webserver
		conf.Web.deinit()
		// Close database
		conf.Database.deinit()

		// Terminate on an interrupt
		if sig == os.Interrupt {
			return
		}

		// Read configuration file if necessary
		if conf.file != "" {
			nconf, err := openConf(conf.file)
			if err == nil {
				conf = nconf
			} else {
				log.Println(err)
			}
		}

	}
}
