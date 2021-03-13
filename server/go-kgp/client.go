package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Client wraps a network connection into a player
type Client struct {
	game    *Game
	rwc     io.ReadWriteCloser
	name    string
	lock    sync.Mutex
	rid     uint64
	input   chan string
	waiting bool
}

// Send forwards an unreferenced message to the client
func (cli *Client) Send(command string, args ...interface{}) uint64 {
	return cli.Respond(0, command, args...)
}

// Respond forwards a referenced message to the client
func (cli *Client) Respond(to uint64, command string, args ...interface{}) uint64 {
	var (
		buf = bytes.NewBuffer(nil)
		id  = atomic.AddUint64(&cli.rid, 2)
	)

	fmt.Fprint(buf, id)
	if to > 0 {
		fmt.Fprintf(buf, "@%d", to)
	}

	fmt.Fprintf(buf, " %s", command)
	for _, arg := range args {
		fmt.Fprint(buf, " ")
		switch arg.(type) {
		case string:
			fmt.Fprintf(buf, "%#v", arg)
		case int:
			fmt.Fprintf(buf, "%d", arg)
		case float64:
			fmt.Fprintf(buf, "%f", arg)
		case *Game:
			fmt.Fprintf(buf, "%s", arg)
		default:
			panic("Unsupported type")
		}
	}
	fmt.Fprint(buf, "\r\n")

	// attempt to send this message before any other message is sent
	defer cli.lock.Unlock()
	cli.lock.Lock()

	i := 8 // allow 8 unsuccesful retries
retry:
	n, err := io.Copy(cli.rwc, buf)
	if err != nil {
		log.Println(i, err)
		nerr, ok := err.(net.Error)
		if i > 0 && (!ok || (ok && nerr.Temporary())) {
			time.Sleep(10 * time.Millisecond)
			if n > 0 {
				// discard first n bytes
				buf = bytes.NewBuffer(buf.Bytes()[n:])
			}
			i--
			goto retry
		} else {
			cli.game.ctrl <- Yield(true)
		}
	}

	return id
}

// Handle controls a connection and reads user input
func (cli *Client) Handle() {
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}

	defer cli.rwc.Close()
	cli.input = make(chan string)
	cli.Send("kgp", majorVersion, minorVersion, patchVersion)

	go func() {
		scanner := bufio.NewScanner(cli.rwc)
		for scanner.Scan() {
			cli.input <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Print(err)
			cli.game.ctrl <- Yield(true)
		}
	}()

	for line := range cli.input {
		err := cli.Interpret(line)
		if err != nil {
			cli.Send("error", err.Error())
		}
	}

	log.Printf("Close connection for %p", cli)
}
