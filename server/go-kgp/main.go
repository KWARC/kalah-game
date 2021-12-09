package main

import (
	"flag"
	"fmt"
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

	defConfName = "server.toml"
)

var conf *Conf = &defaultConfig

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
		conf.TCP.Timeout,
		"Seconds to wait for a move to be made")
	flag.BoolVar(&conf.Debug, "debug",
		conf.Debug,
		"Print all network I/O")
	flag.Parse()

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

	if conf.Debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	if conf.WS.Enabled {
		http.HandleFunc("/socket", listenUpgrade)
		if conf.Debug {
			log.Print("Handling websocket on /socket")
		}

	}

	if conf.TCP.Enabled {
		tcp := fmt.Sprintf("%s:%d", conf.TCP.Host, conf.TCP.Port)
		plain, err := net.Listen("tcp", tcp)
		if err != nil {
			log.Fatal(err)
		}
		if conf.Debug {
			log.Printf("Listening on TCP %s", tcp)
		}
		go listen(plain)
	}

	// Start web server
	go func() {
		web := fmt.Sprintf("%s:%d", conf.Web.Host, conf.Web.Port)
		if conf.Debug {
			log.Printf("Listening via HTTP on %s", web)
		}
		log.Fatal(http.ListenAndServe(web, nil))
	}()

	// Start match scheduler
	go queueManager()

	// Start database manager
	manageDatabase()
}
