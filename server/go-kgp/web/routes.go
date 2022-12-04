// Web request handlers
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
	"context"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	"go-kgp"
)

const DB_TIMEOUT = 20 * time.Second // arbitrary choice

// Generate the index page
func (s *web) index(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, DB_TIMEOUT)
	defer cancel()

	w.Header().Add("Content-Type", "text/html")
	w.Header().Add("Cache-Control", "max-age=60")
	c := make(chan *kgp.Game)
	go s.conf.DB.QueryGames(ctx, -1, c, page-1)
	err = tmpl.ExecuteTemplate(w, "index.tmpl", struct {
		Games chan *kgp.Game
		Page  int
		User  *kgp.User // intentionally unused
	}{c, page, nil})
	if err != nil {
		s.conf.Log.Print(err)
	}
}

// Generate the index page
func (s *web) query(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		msg := "Form could not be parsed"
		http.Error(w, msg, http.StatusBadRequest)
	}

	token := r.PostFormValue("token")

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, DB_TIMEOUT)
	defer cancel()

	user := s.conf.DB.QueryUserToken(ctx, token)
	if user != nil && user.Id != 0 {
		http.Redirect(w, r, fmt.Sprintf("/agent/%d", user.Id), http.StatusSeeOther)
	} else {
		msg := fmt.Sprintf("No user found with the token %q", token)
		http.Error(w, msg, http.StatusNotFound)
	}
}

// Generate the about page
func (s *web) about(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	tmpl.ExecuteTemplate(w, "header.tmpl", nil)
	tmpl.ExecuteTemplate(w, "about.tmpl", struct{}{})
	tmpl.ExecuteTemplate(w, "footer.tmpl", nil)
}

// Generate a website to display an agent
func (s *web) showAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, DB_TIMEOUT)
	defer cancel()

	gc := make(chan *kgp.Game)
	user := s.conf.DB.QueryUser(ctx, id)
	if user == nil {
		msg := fmt.Sprintf("No user found with the id %q", id)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	go s.conf.DB.QueryGames(ctx, int(user.Id), gc, page-1)

	w.Header().Add("Content-Type", "text/html")
	err = tmpl.ExecuteTemplate(w, "show-agent.tmpl", struct {
		User  *kgp.User
		Games chan *kgp.Game
		Page  int
	}{user, gc, page})
	if err != nil {
		s.conf.Log.Print(err)
	}
}

// Generate a website to display an agent
func (s *web) showAgents(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, DB_TIMEOUT)
	defer cancel()

	uc := make(chan *kgp.User)
	go s.conf.DB.QueryUsers(ctx, uc, page-1)

	w.Header().Add("Content-Type", "text/html")
	err = tmpl.ExecuteTemplate(w, "list-agents.tmpl", struct {
		Users chan *kgp.User
		Page  int
	}{uc, page})
	if err != nil {
		s.conf.Log.Print(err)
	}
}

// Generate a website to display a game
func (s *web) showGame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, DB_TIMEOUT)
	defer cancel()

	gc := make(chan *kgp.Game, 1)
	mc := make(chan *kgp.Move, 4) // arbitrary
	go s.conf.DB.QueryGame(ctx, id, gc, mc)

	w.Header().Add("Content-Type", "text/html")
	w.Header().Add("Cache-Control", "max-age=604800")
	err = tmpl.ExecuteTemplate(w, "show-game.tmpl", struct {
		Game  *kgp.Game
		Moves chan *kgp.Move
	}{<-gc, mc})
	if err != nil {
		s.conf.Log.Print(err)
	}
}
