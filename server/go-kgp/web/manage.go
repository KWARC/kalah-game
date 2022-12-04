// Web interface manager
//
// Copyright (c) 2021, 2022  Philip Kaludercic
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
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"go-kgp/conf"
)

const about = `<p>This is a practice server for the AI1 Kalah Tournament.</p>`

type web struct {
	conf *conf.Conf
	mux  *http.ServeMux
}

func (s *web) listen() {
	addr := fmt.Sprintf(":%d", s.conf.WebPort)
	s.conf.Debug.Printf("Listening via HTTP on %s", addr)

	err := http.ListenAndServe(addr, s.mux)
	if err != nil {
		s.conf.Log.Print(err)
	}
}

func (s *web) Start() {
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
	s.mux.HandleFunc("/", s.index)

	s.mux.Handle("/static/", http.FileServer(http.FS(static)))
	if s.conf.Data != "" {
		dir := http.FileServer(http.Dir(s.conf.Data))
		s.mux.Handle("/data/", http.StripPrefix("/data/", dir))
	}

	// Install the WebSocket handler
	if s.conf.WebSocket {
		s.conf.Debug.Print("Accepting websocket connections on /socket")
		s.mux.HandleFunc("/socket", upgrader(s.conf))
	}

	// Parse templates
	tmpl = template.Must(template.New("").Funcs(funcs).ParseFS(html, "*.tmpl"))
	var aboutpage string
	if s.conf.About != "" {
		contents, err := os.ReadFile(s.conf.About)
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

	s.listen()
}

// The web server can shut down immediately
func (*web) Shutdown() {}

func (*web) String() string { return "Web Server" }

func Prepare(conf *conf.Conf) {
	if !conf.WebInterface {
		return
	}

	conf.Register(&web{conf: conf})
}
