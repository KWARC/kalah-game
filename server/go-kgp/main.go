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
	"net/http"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	majorVersion = 1
	minorVersion = 0
	patchVersion = 0

	// Default file name for the configuration file
	defConfName = "server.toml"
)

var (
	// Active configuration object
	conf *Conf = &defaultConfig

	// Logger used for debug output
	debug = log.New(io.Discard, "[debug] ", log.Ltime|log.Lshortfile|log.Lmicroseconds)
)

func listen(port uint) {
	tcp := fmt.Sprintf(":%d", port)
	plain, err := net.Listen("tcp", tcp)
	if err != nil {
		log.Fatal(err)
	}

	debug.Printf("Listening on TCP %s", tcp)
	go func() {
		for {
			conn, err := plain.Accept()
			if err != nil {
				log.Print(err)
				continue
			}

			log.Printf("New connection from %s", conn.RemoteAddr())
			go (&Client{rwc: conn}).Handle()
		}
	}()
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
	flag.StringVar(&conf.Sched, "sched",
		conf.Sched,
		"Set game scheduler.")
	flag.StringVar(&conf.Tourn.Isolation, "isolate",
		conf.Tourn.Isolation,
		"Isolation mechanism used for the tournament.")
	flag.Parse()

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
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

	// In case an upper bound of concurrent games has been
	// specified, we prepare the "slots" channel to be used as a
	// semaphore.
	if conf.Game.Slots > 0 {
		slots = make(chan struct{}, conf.Game.Slots)
		for i := uint(0); i < conf.Game.Slots; i++ {
			slots <- struct{}{}
		}
	}

	// The ongoing WaitGroup coordinates the number of active
	// games.  Before terminating the program, we want to ensure
	// that all games have finished.  WaitGroups have the
	// constraint that they are not allowed to reach a value of
	// "0" more than once (else triggering a "WaitGroup is reused
	// before previous Wait has returned" panic).  To prevent
	// this, a pseudo-Add is called before the rest of the program
	// is starts, that will be undone in the function closeDB.
	ongoing.Add(1)

	// Generate match scheduler from the scheduler specification
	// and start it is a separate goroutine.
	//
	// Note that the scheduler specification must already be
	// prepared in this new goroutine, as arguments to a function
	// are evaluated in the initial goroutine.  If the scheduler
	// depends on the database (as it does for tournament
	// schedulers), the program would deadlock.
	go func() {
		sched := parseSched(conf.Sched)
		switch &sched {
		case &fifo:
			fc := conf.Schedulers.FIFO
			listen(fc.Port)
			if fc.WebSocket {
				http.HandleFunc("/socket", listenUpgrade)
				debug.Print("Handling websocket on /socket")
			}
		case &random:
		}
		schedule(sched)
	}()

	// Start database manager
	manageDatabase()
}
