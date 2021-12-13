// Web interface generator
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
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

//go:embed html
var html embed.FS

var T *template.Template
var funcs = template.FuncMap{
	"inc": func(i int) int {
		return i + 1
	},
	"dec": func(i int) int {
		return i - 1
	},
	"isOver": func(g Game) bool {
		return g.IsOver()
	},
	"timefmt": func(t time.Time) string {
		return t.Format(time.RFC822)
	},
	"result": func(out Outcome) string {
		switch out {
		case ONGOING:
			return "Ongoing"
		case WIN:
			return "South won"
		case DRAW:
			return "Draw"
		case LOSS:
			return "North won"
		case RESIGN:
			return "Resignation"
		default:
			return "???"
		}
	},
	"hasAbout": func() bool {
		return conf.Web.About != ""
	},
}

var (
	// The static file system as a HTTP Handler
	static http.Handler

	// A lock to synchronise the restarting of a web server on
	// configuration reload
	weblock sync.Mutex
)

func init() {
	staticfs, err := fs.Sub(html, "html/static")
	if err != nil {
		log.Fatal(err)
	}
	static = http.FileServer(http.FS(staticfs))
}

func (wc *WebConf) init() {
	var about []byte

	if !wc.Enabled {
		return
	}

	if wc.server != nil {
		wc.server.Shutdown(context.Background())
	}

	mux := http.NewServeMux()
	weblock.Lock()
	defer weblock.Unlock()

	// Install HTTP handlers
	mux.HandleFunc("/games", listGames)
	mux.HandleFunc("/agents", listAgents)
	mux.HandleFunc("/game/", showGame)
	mux.HandleFunc("/agent/", showAgent)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			err := T.ExecuteTemplate(w, "index.tmpl", struct{}{})
			if err != nil {
				log.Print(err)
			}

		case "/about":
			if about == nil {
				http.Error(w, "No about page", http.StatusNoContent)
				return
			}
			T.ExecuteTemplate(w, "header.tmpl", nil)
			w.Write(about)
			T.ExecuteTemplate(w, "footer.tmpl", nil)
		default:
			static.ServeHTTP(w, r)
		}
	})

	if conf.WS.Enabled {
		mux.HandleFunc("/socket", listenUpgrade)
		debug.Print("Handling websocket on /socket")
	}

	// Parse templates
	var err error
	T = template.Must(template.New("").Funcs(funcs).ParseFS(html, "html/*.tmpl"))
	if conf.Web.About != "" {
		about, err = os.ReadFile(conf.Web.About)
		if err != nil {
			log.Fatal(err)
		}
	}

	addr := fmt.Sprintf("%s:%d", conf.Web.Host, conf.Web.Port)
	log.Printf("Listening via HTTP on %s", addr)
	wc.server = &http.Server{Addr: addr, Handler: mux}
	err = wc.server.ListenAndServe()
	if err != nil {
		log.Print(err)
	}
}

func showGame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	c := make(chan *Game)
	dbact <- queryGame(id, c)
	err = T.ExecuteTemplate(w, "show-game.tmpl", <-c)
	if err != nil {
		log.Print(err)
	}
}

func showAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	c := make(chan *Agent)
	dbact <- queryAgent(id, c)
	games := make(chan *Game)
	dbact <- queryGames(games, page-1, &id)

	err = T.ExecuteTemplate(w, "show-agent.tmpl", struct {
		Agent chan *Agent
		Games chan *Game
		Page  int
	}{c, games, page})
	if err != nil {
		log.Print(err)
	}
}

func listGames(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	c := make(chan *Game)
	dbact <- queryGames(c, page-1, nil)
	err = T.ExecuteTemplate(w, "list-games.tmpl", struct {
		Games chan *Game
		Page  int
	}{c, page})
	if err != nil {
		log.Print(err)
	}
}

func listAgents(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	c := make(chan *Agent)
	dbact <- queryAgents(c, page-1)
	err = T.ExecuteTemplate(w, "list-agents.tmpl", struct {
		Agents chan *Agent
		Page   int
	}{c, page})
	if err != nil {
		log.Print(err)
	}
}
