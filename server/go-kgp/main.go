// Entry point
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
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	majorVersion = 1
	minorVersion = 0
	patchVersion = 0

	defConfName = "server.toml"
)

var (
	conf *Conf = &defaultConfig

	debug = log.New(io.Discard, "[debug] ", log.Ltime|log.Lshortfile|log.Lmicroseconds)

	version string
)

func listen(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		log.Printf("New connection from %s", conn.RemoteAddr())
		go (&Client{rwc: conn}).Handle()
	}
}

func main() {
	var (
		confFile = flag.String("conf", defConfName, "Name of configuration file")
		dumpConf = flag.Bool("dump-config", false, "Dump default configuration")
	)

	flag.UintVar(&conf.TCP.Port, "port",
		conf.TCP.Port,
		"Port for TCP connections")
	flag.StringVar(&conf.TCP.Host, "host",
		conf.TCP.Host,
		"Host for TCP connections")
	flag.UintVar(&conf.Web.Port, "webport",
		conf.Web.Port,
		"Port for HTTP connections")
	flag.StringVar(&conf.Web.Host, "webhost",
		conf.Web.Host,
		"Host for HTTP connections")
	flag.BoolVar(&conf.WS.Enabled, "websocket",
		conf.WS.Enabled,
		"Listen for websocket upgrades only")
	flag.StringVar(&conf.Database.File, "db",
		conf.Database.File,
		"Path to SQLite database")
	flag.UintVar(&conf.Game.Timeout, "timeout",
		conf.Game.Timeout,
		"Seconds to wait for a move to be made")
	flag.BoolVar(&conf.Debug, "debug",
		conf.Debug,
		"Print all network I/O")
	flag.StringVar(&conf.Web.About, "about",
		conf.Web.About,
		"A template for the about page")
	flag.Parse()

	if flag.NArg() != 0 {
		log.Fatal("Too many arguments")
	}

	if *dumpConf {
		enc := toml.NewEncoder(os.Stdout)
		err := enc.Encode(defaultConfig)
		if err != nil {
			log.Fatal("Failed to encode default configuration")
		}
		os.Exit(0)
	}

	newconf, err := openConf(*confFile)
	if err != nil && (!os.IsNotExist(err) || *confFile != defConfName) {
		log.Fatal(err)
	}
	if newconf != nil {
		conf = newconf
	}
	conf.init()

	if conf.TCP.Enabled {
		tcp := fmt.Sprintf("%s:%d", conf.TCP.Host, conf.TCP.Port)
		plain, err := net.Listen("tcp", tcp)
		if err != nil {
			log.Fatal(err)
		}
		debug.Printf("Listening on TCP %s", tcp)
		go listen(plain)
	}

	// Start match scheduler
	go queueManager()

	// Start database manager
	manageDatabase()
}
