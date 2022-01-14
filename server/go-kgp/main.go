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
	confFile := flag.String("conf", defConfName, "Name of configuration file")
	dumpConf := flag.Bool("dump-config", false, "Dump default configuration")
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
			rc := conf.Schedulers.Random
			listen(rc.Port)
			if rc.WebSocket {
				http.HandleFunc("/socket", listenUpgrade)
				debug.Print("Handling websocket on /socket")
			}
		}
		schedule(sched)
	}()

	// Start database manager
	manageDatabase()
	debug.Print("Terminating")
}
