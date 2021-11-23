package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// An Agent
type Agent struct {
	Name   string
	Author string
	Descr  string
	Score  float64
	Id     int64
}

// Client wraps a network connection into a player
type Client struct {
	Agent
	game     *Game
	rwc      io.ReadWriteCloser
	lock     sync.Mutex
	choice   int
	rid      uint64
	killFunc context.CancelFunc
	waiting  bool
	pinged   bool
	token    string
	comment  string
}

func (cli *Client) String() string {
	if conn, ok := cli.rwc.(net.Conn); ok {
		return fmt.Sprint(conn.RemoteAddr())
	}
	return fmt.Sprintf("%p", cli)
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
	if debug {
		log.Print(cli, " > ", buf.String())
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
		} else if cli.game != nil {
			cli.game.ctrl <- Yield(true)
		}
	}

	return id
}

// Handle controls a connection and reads user input
func (cli *Client) Handle() {

	// Ensure that the client has a channel that is being
	// communicated upon.
	if cli.rwc == nil {
		panic("No ReadWriteCloser")
	}
	defer forget(cli)
	defer cli.rwc.Close()

	context, cancel := context.WithCancel(context.Background())
	cli.killFunc = cancel

	// Initiate the protocol with the client
	cli.Send("kgp", majorVersion, minorVersion, patchVersion)

	// Start a thread to periodically send ping requests to the
	// client
	ticker := time.NewTicker(time.Duration(1+timeout) * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			// If the timer fired, check the ping flag and
			// kill the client if it is still set
			if cli.pinged {
				log.Printf("%s did not respond to a ping in time", cli)
				cli.killFunc()
				break
			}
			// In case it was not set, ping the client
			// again and reset the flag
			cli.Send("ping")
			cli.pinged = true
		}
	}()

	// Start a thread to read the user input from rwc
	dead := false
	go func() {
		scanner := bufio.NewScanner(cli.rwc)
		for scanner.Scan() {
			// Prevent flooding by waiting a for a moment
			// between lines
			time.Sleep(time.Microsecond)

			// Check if the client has been killed
			// by someone else
			if dead {
				break
			}

			// Interpret line
			input := scanner.Text()
			if debug {
				log.Print(cli, " < ", input)
			}
			err := cli.Interpret(input)
			if err != nil {
				log.Println(err)
			}

		}
		// See https://github.com/golang/go/commit/e9ad52e46dee4b4f9c73ff44f44e1e234815800f
		err := scanner.Err()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Print(err)
		}
		cli.killFunc()
	}()

	// When the client is killed (a game has finished, the client
	// timed out, ...) we log this and mark the client as dead for
	// the input thread
	<-context.Done()
	log.Printf("Close connection for %s", cli)
	dead = true
}
