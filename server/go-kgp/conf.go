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
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type DBConf struct {
	// Path to the SQLite database
	File string `toml:"file"`
	// Timeout to execute a database action
	Timeout time.Duration `toml:"timeout"`
	// Periodically optimise the database
	Optimise bool `toml:"optimise"`
}

type DockerConf struct {
	// Bytes in bytes of memory to grant a Docker container
	Memory uint `toml:"memory"`
	// Bytes in bytes of swap to grant a Docker container
	Swap uint `toml:"swap"`
	// Number of CPUs to grant a Docker container
	CPUs uint `toml:"cpus"`
	// Name of the Docker network to connect the container to
	Network string `toml:"network"`
}

type TournamentConf struct {
	// Directory containing participants
	Directory string `toml:"directory"`
	// Isolation mechanism used to start a client
	Isolation string `toml:"isolation"`
	// Number of seconds allowed for clients to connect
	Warmup uint `toml:"warmup"`
	// List of participant names
	Names []string `toml:"names"`
	// Configuration for Docker isolation
	Docker DockerConf `toml:"docker"`
}

type WebConf struct {
	// Is the web interface enabled
	Enabled bool `toml:"enabled"`
	// Hostname of the public server the website is accessible
	// over (just the domain)
	Host string `toml:"host"`
	// Port to bind the the webserver
	Port uint `toml:"port"`
	// Limit is the number of entries any table may list
	Limit uint `toml:"limit"`
	// Path to the about template.  If an empty string, no about
	// page will be generated.
	About string `toml:"about"`
}

type GameConf struct {
	// Time granted to make a move or yield
	Timeout uint `toml:"timeout"`
	// Should games end if there is no possibility for one side to
	// win (i.e. one side has already collected more than half the
	// stones in their store)
	EarlyWin bool `toml:"earlywin"`
	// Number of concurrent games (0 if no limit)
	Slots uint `toml:"slots"`
	// Should trivial moves (where there is only one choice) be
	// made for the client, without an additional query.
	SkipTriv bool `toml:"skiptriv"`
	// Points for winning
	Win float64 `toml:"win"`
	// Points for loosing
	Loss float64 `toml:"loose"`
	// Points for a draw
	Draw float64 `toml:"draw"`
}

type TCPConf struct {
	// Enabled keepalive checks via "ping"
	Ping bool `toml:"ping"`
	// Number of seconds until a "ping" expires, and the
	// connection is regarded to be dead
	Timeout uint `toml:"timeout"`
}

type Scheduler struct{ s Sched }

type Conf struct {
	// Scheduler specification
	Sched Scheduler `toml:"sched"`
	// Enable debug logging
	Debug bool `toml:"debug"`
	// Database configuration
	Database DBConf `toml:"database"`
	// Tournament configuration
	Tourn TournamentConf `toml:"tournament"`
	// General Game configuration
	Game GameConf `toml:"game"`
	// Web interface configuration
	Web WebConf `toml:"web"`
	// General TCP configuration
	TCP TCPConf `toml:"tcp"`
}

// Configuration object used by default
var defaultConfig = Conf{
	Debug: false,
	Tourn: TournamentConf{
		Directory: ".",
		Isolation: "none",
		Warmup:    60 * 10,
		Docker: DockerConf{
			CPUs:    1,
			Memory:  1 << 30, // 1GiB
			Swap:    1 << 30, // 1GiB
			Network: "none",
		},
	},
	Database: DBConf{
		File:     "kalah.sql",
		Timeout:  100 * time.Millisecond,
		Optimise: true,
	},
	Game: GameConf{
		EarlyWin: true,
		SkipTriv: true,
		Timeout:  5,
		Slots:    0, // unlimited
		Win:      1,
		Loss:     -1,
		Draw:     0,
	},
	Web: WebConf{
		Enabled: true,
		Host:    "",
		Port:    8080,
		Limit:   50,
		About:   "",
	},
	TCP: TCPConf{
		Ping:    true,
		Timeout: 20,
	},
}

func (conf *Conf) init() {
	if conf.Debug {
		debug.SetOutput(os.Stderr)
		log.SetFlags(log.Flags() | log.Lshortfile)
	} else {
		debug.SetOutput(io.Discard)
	}

	go conf.Web.init()
}

// Parse a configuration from R into CONF
func parseConf(r io.Reader, conf *Conf) (err error) {
	_, err = toml.NewDecoder(r).Decode(conf)
	return
}

// Open a configuration file and return it
func openConf(name string) (*Conf, error) {
	var conf Conf

	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return &conf, parseConf(file, &conf)
}

func (sched *Scheduler) UnmarshalTOML(s interface{}) error {
	switch v := s.(type) {
	case string:
		switch v {
		case "ro", "noop":
			sched.s = &QueueSched{impl: noop}
			return nil
		case "fifo":
			sched.s = &QueueSched{
				init: func() {
					go listen(2671)
					http.HandleFunc("/socket", listenUpgrade)
					debug.Print("Handling websocket on /socket")
				},
				impl: fifo,
			}
		case "random", "rand":
			sched.s = makeTournament(&random{size: 6})
		case "rr", "round-robin":
			sched.s = makeTournament(&roundRobin{size: 6})
		default:
			return fmt.Errorf("Unknown scheduler %s", v)
		}
	case map[string]interface{}:
		rtyp, ok := v["type"]
		if !ok {
			return fmt.Errorf("Scheduler has no type %s", v)
		}
		typ, ok := rtyp.(string)
		if !ok {
			return fmt.Errorf("Scheduler type is not a string %s", rtyp)
		}

		switch typ {
		case "rand":
			rand := &random{}

			size, ok := v["size"]
			if ok {
				rand.size, ok = size.(uint)
				if !ok {
					return fmt.Errorf("Invalid size %s", size)
				}
			} else {
				rand.size = 6
			}

			sched.s = makeTournament(rand)
		case "rr", "round-robin":
			rr := &roundRobin{}

			size, ok := v["size"]
			if ok {
				rr.size, ok = size.(uint)
				if !ok {
					return fmt.Errorf("Invalid size %s", size)
				}
			} else {
				rr.size = 6
			}

			pick, ok := v["pick"]
			if ok {
				rr.pick, ok = pick.(uint)
				if !ok {
					return fmt.Errorf("Invalid pick value %s", pick)
				}
			}

			sched.s = makeTournament(rr)
		case "se", "single-elimination":
			se := &singleElim{}

			size, ok := v["size"]
			if ok {
				se.size, ok = size.(uint)
				if !ok {
					return fmt.Errorf("invalid size %s", size)
				}
			} else {
				se.size = 6
			}

			sched.s = makeTournament(se)
		default:
			return fmt.Errorf("Unknown type %v", v)
		}
	case []interface{}:
		var comp []Sched
		for _, s := range v {
			var sched Scheduler
			err := sched.UnmarshalTOML(s)
			if err != nil {
				return err
			}
			comp = append(comp, sched.s)
		}
		switch len(comp) {
		case 0:
			return fmt.Errorf("Empty Scheduler list")
		case 1:
			sched.s = comp[0]
		default:
			sched.s = &CompositeSched{s: comp}
		}
	default:
		return fmt.Errorf("Unknown scheduler %#v", s)
	}
	return nil
}
