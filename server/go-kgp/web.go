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
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
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

var (
	T     *template.Template
	funcs = template.FuncMap{
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
			return t.Format(time.Stamp)
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
		"version": func() string {
			if version == "" {
				return "unknown"
			}
			return version
		},
		"hasMore": func(i int) bool {
			return i%int(conf.Web.Limit) != 0
		},
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
	}
)

var (
	// The static file system as a HTTP Handler
	static http.Handler

	// A lock to synchronise the restarting of a web server on
	// configuration reload
	weblock sync.Mutex

	// The cached state of the front-page
	indexPage []byte
)

func init() {
	staticfs, err := fs.Sub(html, "html/static")
	if err != nil {
		log.Fatal(err)
	}
	static = http.FileServer(http.FS(staticfs))

	go func() {
		time.Sleep(10 * time.Second)
		var buf bytes.Buffer
		err := genIndex(0, &buf)
		if err != nil {
			log.Println(err)
		} else {
			indexPage = buf.Bytes()
		}

		time.Sleep(10 * time.Minute)
	}()
}

func (wc *WebConf) init() {
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
	mux.HandleFunc("/", index)
	mux.HandleFunc("/agent/", showAgent)
	mux.HandleFunc("/about", about)
	mux.Handle("/static/", http.StripPrefix("/static/", static))

	if conf.WS.Enabled {
		mux.HandleFunc("/socket", listenUpgrade)
		debug.Print("Handling websocket on /socket")
	}

	// Parse templates
	var err error
	T = template.Must(template.New("").Funcs(funcs).ParseFS(html, "html/*.tmpl"))
	if conf.Web.About != "" {
		about, err := os.ReadFile(conf.Web.About)
		if err != nil {
			log.Fatal(err)
		}
		_, err = T.New("about.tmpl").Parse(string(about))
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

func genIndex(page int, w io.Writer) error {
	c := make(chan *Agent)
	dbact <- queryAgents(c, page-1)
	err := T.ExecuteTemplate(w, "index.tmpl", struct {
		Agents chan *Agent
		Page   int
	}{c, page})
	return err
}

func index(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	w.Header().Add("Content-Type", "text/html")
	w.Header().Add("Cache-Control", "max-age=60")
	if !conf.Debug && page == 1 {
		_, err = w.Write(indexPage)
	} else {
		err = genIndex(page, w)
	}
	if err != nil {
		log.Print(err)
	}
}

func about(w http.ResponseWriter, r *http.Request) {
	if conf.Web.About == "" {
		http.Error(w, "No about page", http.StatusNoContent)
		return
	}
	w.Header().Add("Content-Type", "text/html")
	T.ExecuteTemplate(w, "header.tmpl", nil)
	T.ExecuteTemplate(w, "about.tmpl", struct{}{})
	T.ExecuteTemplate(w, "footer.tmpl", nil)
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

	w.Header().Add("Content-Type", "text/html")
	err = T.ExecuteTemplate(w, "show-agent.tmpl", struct {
		Agent chan *Agent
		Page  int
	}{c, page})
	if err != nil {
		log.Print(err)
	}
}
