package main

import (
	"flag"
	"log"
	"net"
	"net/http"
)

const (
	majorVersion = 1
	minorVersion = 0
	patchVersion = 0
)

var (
	defSize   uint
	defStones uint
	timeout   uint
	debug     bool
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
		socket string
		dbf    string
		web    string
		ws     bool
	)

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.UintVar(&defSize, "size", 7, "Size of new boards")
	flag.UintVar(&defStones, "stones", 7, "Number of stones to use")
	flag.StringVar(&socket, "socket", ":2671", "Address to listen on for socket connections")
	flag.BoolVar(&ws, "websocket", false, "Listen for websocket upgrades only")
	flag.StringVar(&dbf, "db", "kalah.sql", "Path to SQLite database")
	flag.UintVar(&timeout, "timeout", 5, "Seconds to wait for a move to be made")
	flag.StringVar(&web, "http", ":8080", "Address to have web server listen on")
	flag.BoolVar(&debug, "debug", false, "Print all network I/O")
	flag.Parse()

	if ws {
		http.HandleFunc("/socket", listenUpgrade)
		log.Println("Listening for upgrades on /socket")
	} else {
		plain, err := net.Listen("tcp", socket)
		if err != nil {
			log.Fatal(err)
		}
		go listen(plain)
		log.Printf("Listening on socket %s", socket)
	}

	// Start web server
	go func() {
		log.Printf("Web interface on %s", web)
		log.Fatal(http.ListenAndServe(web, nil))
	}()

	// start match scheduler
	go organizer()

	// start database manager
	manageDatabase(dbf)
}
