// Web interface manager
//
// Copyright (c) 2021, 2022, 2023  Philip Kaludercic
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

package web

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"go-kgp"
	cmd "go-kgp/cmd"
)

const about = `<p>This is a practice server for the AI1 Kalah Tournament.</p>`

type web struct {
	DB  cmd.Database
	mux *http.ServeMux
}

func (s *web) listen(conf *cmd.WebConf) {
	addr := fmt.Sprintf(":%d", conf.Port)
	log.Printf("Listening via HTTP on %s", addr)

	err := http.ListenAndServe(addr, s.mux)
	if err != nil {
		log.Print(err)
	}
}

func (s *web) drawGraphs(mode *cmd.State) {
	var (
		dbg  = kgp.Debug.Println
		draw = mode.Database.DrawGraph
	)

	gen := func() ([]byte, error) {
		bg := context.Background()
		ctx, cancel := context.WithCancel(bg)
		defer cancel()

		dbg("(Re-)generating dominance graph")
		cmd := exec.Command(`dot`, `-Tsvg`)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return nil, err
		}
		err = cmd.Start()
		if err != nil {
			return nil, err
		}

		go func() {
			err := draw(ctx, stdin)
			if err != nil {
				dbg(err)
				return
			}
			err = stdin.Close()
			if err != nil {
				dbg(err)
				return
			}
		}()

		data, err := io.ReadAll(stdout)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(io.Discard, io.TeeReader(stderr, os.Stderr))
		if err != nil {
			return nil, err
		}

		err = cmd.Wait()
		if err != nil {
			return data, err
		}
		dbg("Finished generating dominance graph")
		return data, nil
	}

	var (
		it   uint32
		next = time.Now()
		data []byte
	)

	h := func(w http.ResponseWriter, r *http.Request) {
		if time.Now().After(next) {
			if atomic.CompareAndSwapUint32(&it, 0, 1) {
				var err error
				data, err = gen()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				atomic.StoreUint32(&it, 0)
			}

			// Allow the graph to be regenerated on demand every minute
			next = time.Now().Add(time.Minute)
		}
		w.Header().Add("Cache-Control", "max-age=60")
		if _, err := w.Write(data); err != nil {
			log.Print(err)
		}
	}
	s.mux.HandleFunc("/graph", h)
}

func (s *web) Start(mode *cmd.State, conf *cmd.Conf) {
	w := &conf.Web
	s.DB = mode.Database

	// Prepare HTTP Multiplexer
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/about", s.about)
	s.mux.HandleFunc("/query", s.query)
	s.mux.HandleFunc("/agents", s.showAgents)
	s.mux.HandleFunc("/agent/", s.showAgent)
	s.mux.HandleFunc("/game/", s.showGame)
	s.mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow: /")
	})

	s.mux.Handle("/static/", http.FileServer(http.FS(static)))
	if w.Data != "" {
		if stat, err := os.Stat(w.Data); err != nil {
			log.Fatalf("Fail to access data directory %s: %s",
				w.Data, err)
		} else if !stat.IsDir() {
			log.Fatalf("Data directory is not a directory %s",
				w.Data)
		}
		log.Printf("Serving a /data/ directory (%s)", w.Data)
		dir := http.FileServer(http.Dir(w.Data))
		s.mux.Handle("/data/", http.StripPrefix("/data/", dir))
	}

	s.mux.HandleFunc("/", s.index)

	if _, err := exec.LookPath("dot"); err == nil {
		log.Print("Enabling graph generation")
		funcs["hasgraph"] = func() bool { return true }
		s.drawGraphs(mode)
	} else {
		funcs["hasgraph"] = func() bool { return false }
	}

	// Install the WebSocket handler
	if w.WebSocket {
		log.Print("Accepting websocket connections on /socket")
		s.mux.HandleFunc("/socket", upgrader(mode))
	}

	// Parse templates
	tmpl = template.Must(template.New("").Funcs(funcs).ParseFS(html, "*.tmpl"))
	var aboutpage string
	if w.About != "" {
		contents, err := os.ReadFile(w.About)
		if err != nil && os.IsNotExist(err) {
			log.Fatal(err)
		}
		aboutpage = string(contents)
	}
	if aboutpage == "" {
		aboutpage = about
	}
	_, err := tmpl.New("about.tmpl").Parse(string(aboutpage))
	if err != nil {
		log.Fatal(err)
	}

	s.listen(w)
}

// The web server can shut down immediately
func (*web) Shutdown() {}

func (*web) String() string { return "Web Server" }

func Register(mode *cmd.State) {
	mode.Register(&web{})
}
