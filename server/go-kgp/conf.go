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
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
)

type GameConf struct {
	Sizes    []uint `toml:"sizes"`
	Stones   []uint `toml:"stones"`
	Timeout  uint   `toml:"timeout"`
	EarlyWin bool   `toml:"earlywin"`
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
}

type TCPConf struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    uint   `toml:"port"`
	Ping    bool   `toml:"ping"`
	Timeout uint   `toml:"timeout"`
	Retries uint   `toml:"retries"`
}

type DBConf struct {
	File    string `toml:"file"`
	Threads uint   `toml:"threads"`
	Mode    string `toml:"mode"`
}

type Conf struct {
	Debug    bool     `toml:"debug"`
	Endless  bool     `toml:"endless"`
	Database DBConf   `toml:"database"`
	Game     GameConf `toml:"game"`
	Web      WebConf  `toml:"web"`
	WS       WSConf   `toml:"websocket"`
	TCP      TCPConf  `toml:"tcp"`
	file     string
}

var defaultConfig = Conf{
	Debug: false,
	Database: DBConf{
		File:    "kalah.sql",
		Threads: 1,
		Mode:    "rwc",
	},
	Endless: true,
	Game: GameConf{
		Sizes:    []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		Stones:   []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		EarlyWin: true,
	},
	Web: WebConf{
		Enabled: true,
		Host:    "0.0.0.0",
		Port:    8080,
		Limit:   50,
		About:   "",
	},
	WS: WSConf{
		Enabled: false,
	},
	TCP: TCPConf{
		Enabled: true,
		Host:    "0.0.0.0",
		Port:    2671,
		Ping:    true,
		Timeout: 20,
		Retries: 8,
	},
}

func (conf *Conf) init() {
	go func() {
		var (
			rc  io.ReadCloser
			err error
		)

		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGUSR1)

		for range c {
			if conf.file == "" {
				goto init
			}
			rc, err = os.Open(conf.file)
			if err != nil {
				log.Println(err)
				continue
			}

			err = parseConf(rc, conf)
			if err != nil {
				log.Println(err)
			}
			rc.Close()

		init:
			conf.init()
		}
	}()

	if conf.Debug {
		debug.SetOutput(os.Stderr)
	} else {
		debug.SetOutput(io.Discard)
	}

	conf.Web.init()
}

func parseConf(r io.Reader, conf *Conf) error {
	_, err := toml.NewDecoder(r).Decode(conf)
	return err
}

func openConf(name string) (*Conf, error) {
	var conf Conf

	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	conf.file = name
	return &conf, parseConf(file, &conf)
}
