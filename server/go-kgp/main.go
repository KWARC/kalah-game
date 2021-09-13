package main

import (
	"flag"
	"fmt"
	"log"
	"net"
)

const (
	majorVersion = 1
	minorVersion = 0
	patchVersion = 0
)

var (
	port      uint
	defSize   uint
	defStones uint
	warmup    uint
	timeout   uint
)

func main() {
	flag.UintVar(&defSize, "size", 7, "Size of new boards")
	flag.UintVar(&defStones, "stones", 7, "Number of stones to use")
	flag.UintVar(&port, "port", 2671, "Port number to listen on")
	flag.UintVar(&warmup, "warmup", 5, "Seconds to wait before starting game")
	flag.UintVar(&timeout, "timeout", 5, "Seconds to wait for a move to be made")
	flag.Parse()

	// open server socket
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	// start match scheduler
	go scheduler()

	// accept incoming connections
	log.Print("Listening on port 2671")
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
