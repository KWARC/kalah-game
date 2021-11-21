package main

import (
	"flag"
	"fmt"
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
		port uint
		dbf  string
		web  string
	)

	flag.UintVar(&defSize, "size", 7, "Size of new boards")
	flag.UintVar(&defStones, "stones", 7, "Number of stones to use")
	flag.UintVar(&port, "port", 2671, "Port number of plain connections")
	flag.StringVar(&dbf, "db", "kalah.sql", "Path to SQLite database")
	flag.UintVar(&timeout, "timeout", 5, "Seconds to wait for a move to be made")
	flag.StringVar(&web, "http", ":8080", "Address to have web server listen on")
	flag.Parse()

	// open server socket
	plain, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	go listen(plain)
	log.Printf("Listening on port %d", port)

	// Start web server
	go log.Fatal(http.ListenAndServe(web, nil))

	// start database manager
	go manageDatabase(dbf)

	// start match scheduler
	organizer()
}
