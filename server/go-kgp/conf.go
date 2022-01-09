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
	"time"

	"github.com/BurntSushi/toml"
)

type GameConf struct {
	Sizes    []uint `toml:"sizes"`
	Stones   []uint `toml:"stones"`
	Timeout  uint   `toml:"timeout"`
	EarlyWin bool   `toml:"earlywin"`
	Slots    uint   `toml:"slots"`
	SkipTriv bool   `toml:"skiptriv"`
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
	Cache   bool   `toml:"cache"`
	Base    string `toml:"base"`
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
	File    string        `toml:"file"`
	Threads uint          `toml:"threads"`
	Mode    string        `toml:"mode"`
	Timeout time.Duration `toml:"timeout"`
}

type TournConf struct {
	Directory string   `toml:"directory"`
	Rounds    uint     `toml:"rounds"`
	System    string   `toml:"system"`
	Isolation string   `toml:"isolation"`
	Warmup    uint     `toml:"warmup"`
	Names     []string `toml:"names"`
}

type Conf struct {
	Sched    string    `toml:"sched"`
	Debug    bool      `toml:"debug"`
	Endless  bool      `toml:"endless"`
	Database DBConf    `toml:"database"`
	Tourn    TournConf `toml:"tournament"`
	Game     GameConf  `toml:"game"`
	Web      WebConf   `toml:"web"`
	WS       WSConf    `toml:"websocket"`
	TCP      TCPConf   `toml:"tcp"`
	file     string
}

var defaultConfig = Conf{
	Debug: false,
	Sched: "fifo",
	Tourn: TournConf{
		Directory: ".",
		Rounds:    1,
		System:    "round-robin",
		Isolation: "none",
		Warmup:    60 * 10,
	},
	Database: DBConf{
		File:    "kalah.sql",
		Threads: 1,
		Timeout: 100 * time.Millisecond,
	},
	Endless: true,
	Game: GameConf{
		Sizes:    []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		Stones:   []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		EarlyWin: true,
		SkipTriv: true,
		Timeout:  5,
		Slots:    0, // unlimited
	},
	Web: WebConf{
		Enabled: true,
		Host:    "0.0.0.0",
		Port:    8080,
		Limit:   50,
		About:   "",
		Cache:   false,
		Base:    "",
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
				log.Print(err)
				continue
			}

			err = parseConf(rc, conf)
			if err != nil {
				log.Print(err)
			}
			rc.Close()

		init:
			go conf.Web.init()
		}
	}()

	if conf.Debug {
		debug.SetOutput(os.Stderr)
		log.SetFlags(log.Flags() | log.Lshortfile)
	} else {
		debug.SetOutput(io.Discard)
	}

	go conf.Web.init()
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
