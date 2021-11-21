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
	var c chan *Game

	dbact <- QueryGame(0, c)
	err := T.ExecuteTemplate(w, "show-game.tmpl", <-c)
	if err != nil {
		log.Print(err)
	}
}

func showAgent(w http.ResponseWriter, r *http.Request) {
	var c chan *Client

	dbact <- QueryAgent(0, c)
	err := T.ExecuteTemplate(w, "show-agent.tmpl", <-c)
	if err != nil {
		log.Print(err)
	}
}

func listGames(w http.ResponseWriter, r *http.Request) {
	gchan := make(chan *Game)
	dbact <- QueryGames(gchan, 0)
	err := T.ExecuteTemplate(w, "list-games.tmpl", gchan)
	if err != nil {
		log.Print(err)
	}
}

func listAgents(w http.ResponseWriter, r *http.Request) {
	achan := make(chan *Agent)
	dbact <- QueryAgents(achan, 0)
	err := T.ExecuteTemplate(w, "list-agents.tmpl", achan)
	if err != nil {
		log.Print(err)
	}
}
