package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed html/static
var static embed.FS

//go:embed html
var templates embed.FS

var T *template.Template

func init() {
	// Install HTTP handlers
	http.HandleFunc("/", listGames)
	http.HandleFunc("/games", listGames)
	http.HandleFunc("/agents", listAgents)
	http.HandleFunc("/game", showGame)
	http.HandleFunc("/agent", showAgent)
	static := http.StripPrefix("/static/", http.FileServer(http.FS(static)))
	http.Handle("/static/", static)

	// Parse templates
	T = template.Must(template.ParseFS(templates, "html/*.tmpl"))
}

func showGame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err := T.ExecuteTemplate(w, "show-game.tmpl", <-c)
	c := make(chan *Game)
	dbact <- queryGame(id, c)
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

	dbact <- QueryAgent(0, c)
	err := T.ExecuteTemplate(w, "show-agent.tmpl", <-c)
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	c := make(chan *Agent)
	dbact <- queryAgent(id, c)
	games := make(chan *Game)
	dbact <- queryGames(games, page-1, &id)

	if err != nil {
		log.Print(err)
	}
}

func listGames(w http.ResponseWriter, r *http.Request) {
	err := T.ExecuteTemplate(w, "list-games.tmpl", gchan)
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	c := make(chan *Game)
	dbact <- queryGames(c, page-1, nil)
	if err != nil {
		log.Print(err)
	}
}

func listAgents(w http.ResponseWriter, r *http.Request) {
	err := T.ExecuteTemplate(w, "list-agents.tmpl", achan)
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	c := make(chan *Agent)
	dbact <- queryAgents(c, page-1)
	if err != nil {
		log.Print(err)
	}
}
