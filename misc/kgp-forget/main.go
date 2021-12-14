// Copyright 2021, Philip Kaludercic

// Permission to use, copy, modify, and/or distribute this software
// for any purpose with or without fee is hereby granted, provided
// that the above copyright notice and this permission notice appear
// in all copies.

// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL
// WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE
// AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR
// CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
// OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT,
// NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN
// CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"

	"nhooyr.io/websocket"
)

func main() {
	var (
		rwc  io.ReadWriteCloser
		err  error
		dest string
	)

	if len(os.Args) <= 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [server address] [token ...]\n",
			os.Args[0])
		os.Exit(1)
	}

	if ok, _ := regexp.MatchString(`^wss?://`, dest); ok {
		ctx := context.Background()
		c, _, err := websocket.Dial(ctx, dest, nil)
		if err == nil {
			rwc = websocket.NetConn(ctx, c, websocket.MessageText)
		}
	} else {
		if ok, _ := regexp.MatchString(`^:\d+$`, dest); !ok {
			dest += ":2671"
		}
		rwc, err = net.Dial("tcp", dest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rwc.Close()

	var line string
	for {
		_, err := fmt.Fscanln(rwc, &line)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if strings.TrimSpace(line) != "" {
			break
		}
	}

	ok, err := regexp.MatchString(
		`^\s*(?:\d*\s+)?kgp\s+1\s+\d+\s+\d\s*$`,
		line)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if !ok {
		fmt.Fprintln(os.Stderr, "Unsupported KGP version")
		return
	}

	r := strings.NewReplacer("\"", "\\\"", "\n", "\\n")
	for _, tok := range os.Args[2:] {
		fmt.Fprintf(rwc, `set auth:forget "%s"\r\n`, r.Replace(tok))
	}
	fmt.Fprintln(rwc, "goodbye\r\n")
}
