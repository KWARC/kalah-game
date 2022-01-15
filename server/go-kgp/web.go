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
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync/atomic"
	"time"
)

//go:embed html
var html embed.FS

var (
	// Template manager
	T *template.Template

	// Custom template functions
	funcs = template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
		"dec": func(i int) int {
			return i - 1
		},
		"timefmt": func(t time.Time) string {
			return t.Format(time.Stamp)
		},
		"result": func(out Outcome) string {
			switch out {
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
		"hasMore": func(i int) bool {
			return i%int(conf.Web.Limit) != 0
		},
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
		"waiting": func() uint64 {
			return atomic.LoadUint64(&waiting)
		},
		"playing": func() uint64 {
			return atomic.LoadUint64(&playing)
		},
		"are": func(n uint64) string {
			if n == 1 {
				return "is"
			}
			return "are"
		},
		"board": func(b *Board) string {
			size := len(b.northPits)
			init := b.init
			return fmt.Sprintf("(%d, %d)", size, init)
		},
		"draw": func(m *Move, g *Game) template.HTML {
			var (
				buf  bytes.Buffer
				B    = &buf
				b    = m.State
				size = len(b.northPits)
				u    = 50.0
				w    = (u*2 + u*float64(size))
			)

			circle := func(x, y float64, n uint, hl bool) {
				// https://developer.mozilla.org/en-US/docs/Web/SVG/Element/circle
				if x < 0 {
					x = float64(size) - x
				}
				color := "sienna"
				if hl {
					color = "seagreen"
				}
				fmt.Fprintf(B, `<circle fill="%s" cx="%g" cy="%g" r="%g" />`,
					color, u*x+u/2, u*y+u/2, u*0.8/2)
				// https://developer.mozilla.org/en-US/docs/Web/SVG/Element/text
				d := u * .4
				if n >= 10 {
					d = u * 0.3
				} else if n >= 100 {
					d = u * 0.2
				}
				fmt.Fprintf(B, `<text x="%g" y="%g">%d</text>`,
					u*x+d, 0.4*u+u*y+u*.2, n)
			}

			fmt.Fprintf(B, `<svg width="%g" height="%g">`, w, 2*u)

			// https://developer.mozilla.org/en-US/docs/Web/SVG/Element/rect
			fmt.Fprintf(B, `<rect x="0" y="0" rx="10" ry="10" width="%g" height="%g" fill="burlywood" />`,
				w, 2*u)
			for i := len(b.northPits) - 1; i >= 0; i-- {
				n := b.northPits[0]
				hl := g.North == m.Client && m.Pit == i
				circle(float64(1+i), 0, n, hl)
			}
			for i, s := range b.southPits {
				hl := g.South == m.Client && m.Pit == i
				circle(float64(1+i), 1, s, hl)
			}
			circle(0.1, 0.5, b.north, false)
			circle(-0.9, 0.5, b.south, false)

			fmt.Fprintf(B, `</svg>`)

			return template.HTML(buf.String())
		},
	}

	// The static file system as a HTTP Handler
	static http.Handler
)

// Parse the embedded file system and create a HTTP file system
func init() {
	staticfs, err := fs.Sub(html, "html/static")
	if err != nil {
		log.Fatal(err)
	}
	static = http.FileServer(http.FS(staticfs))
}

// Initialise the web server
//
// If a web server was already running, wait for it to be killed.
func (wc *WebConf) init() {
	// Install HTTP handlers
	http.HandleFunc("/", index)
	http.HandleFunc("/agent/", showAgent)
	http.HandleFunc("/game/", showGame)
	http.HandleFunc("/about", about)
	http.Handle("/static/", http.StripPrefix("/static/", static))
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow:")
	})

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

	addr := fmt.Sprintf(":%d", conf.Web.Port)
	debug.Printf("Listening via HTTP on %s", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Print(err)
	}
}

// Generate the index page
func index(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	w.Header().Add("Content-Type", "text/html")
	w.Header().Add("Cache-Control", "max-age=60")
	c := make(chan *Agent)
	dbact <- queryAgents(c, page-1)
	err = T.ExecuteTemplate(w, "index.tmpl", struct {
		Agents chan *Agent
		Page   int
	}{c, page})
	if err != nil {
		log.Print(err)
	}
}

// Generate the about page
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

// Generate a website to display an agent
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

	ac := make(chan *Agent)
	gc := make(chan *Game)
	dbact <- queryAgent(id, ac)
	dbact <- queryGames(id, gc, page-1)

	w.Header().Add("Content-Type", "text/html")
	err = T.ExecuteTemplate(w, "show-agent.tmpl", struct {
		Agent chan *Agent
		Games chan *Game
		Page  int
	}{ac, gc, page})
	if err != nil {
		log.Print(err)
	}
}

// Generate a website to display a game
func showGame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	c := make(chan *Game)
	dbact <- queryGame(id, c)

	w.Header().Add("Content-Type", "text/html")
	w.Header().Add("Cache-Control", "max-age=604800")
	err = T.ExecuteTemplate(w, "show-game.tmpl", <-c)
	if err != nil {
		log.Print(err)
	}
}
