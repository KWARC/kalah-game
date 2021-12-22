// Database management
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
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// The database manager accepts "database actions", ie. functions that
// operate on a database.  These are sent to the database manager or
// managers via the channel DBACT, that executes the action and
// handles possible errors.

type DBAction func(*sql.DB, context.Context) error

var dbact = make(chan DBAction, 1)

// The SQL queries are stored under ./sql/, and they are loaded by the
// database manager.  These are prepared and stored in QUERIES, that
// the database actions use.

//go:embed sql
var sqlDir embed.FS

var queries = make(map[string]*sql.Stmt)

func (cli *Client) updateDatabase(wait *sync.WaitGroup, query bool) DBAction {
	return func(db *sql.DB, ctx context.Context) (err error) {
		var (
			name, descr *string
			score       *float64
		)
		defer wait.Done()

		if query {
			err = queries["select-agent-token"].QueryRowContext(ctx, cli.token).Scan(
				&cli.Id, &name, &descr, &score)
			if err != nil && err != sql.ErrNoRows {
				log.Fatal(err)
			}

			if name != nil {
				cli.Name = *name
			}
			if descr != nil {
				cli.Descr = *descr
			}
			if score != nil {
				cli.Score = *score
			}
		}

		_, err = queries["insert-agent"].ExecContext(ctx,
			cli.token,
			cli.Name,
			cli.Descr,
			cli.Author,
			cli.Score)
		if err != nil {
			log.Print(err)
			return
		}

		return
	}
}

func (cli *Client) forget(token []byte) DBAction {
	return func(db *sql.DB, ctx context.Context) error {
		_, err := queries["delete-agent"].ExecContext(ctx, token)
		if err != nil {
			log.Print(err)
		}
		return err
	}
}

func queryAgent(aid int, c chan<- *Agent) DBAction {
	return func(db *sql.DB, ctx context.Context) error {
		var agent Agent

		defer close(c)
		err := queries["select-agent-id"].QueryRowContext(ctx, aid).Scan(
			&agent.Name,
			&agent.Descr,
			&agent.Author,
			&agent.Score)
		if err != nil {
			log.Fatal(err)
		} else {
			c <- &agent
		}
		return err
	}
}

func queryAgents(c chan<- *Agent, page int) DBAction {
	return func(db *sql.DB, ctx context.Context) error {
		defer close(c)
		rows, err := queries["select-agents"].QueryContext(ctx, page, conf.Web.Limit)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Print(err)
			}
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var agent Agent

			err = rows.Scan(&agent.Id, &agent.Name, &agent.Author, &agent.Score)
			if err != nil {
				log.Fatal(err)
				return err
			}

			c <- &agent
		}
		if err = rows.Err(); err != nil {
			log.Print(err)
			return err
		}

		return nil
	}
}

func databaseManager(id uint, db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()

	// The channel is copied so that when the server is requested
	// to terminate we can continue to process the remaining
	// actions, without an other thread writing on a closed
	// channel that would trigger a panic.
	dbact := dbact
	for act := range dbact {
		if act == nil {
			continue
		}

		context, cancel := context.WithTimeout(context.Background(), time.Millisecond*10000)
		defer cancel()
		act(db, context)
		if err := context.Err(); err != nil {
			log.Println(err)
		}
	}
}

func manageDatabase() {
	uri := fmt.Sprintf("%s?mode=%s&_journal=wal",
		conf.Database.File,
		conf.Database.Mode)
	db, err := sql.Open("sqlite3", uri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	go func() {
		intr := make(chan os.Signal, 1)
		signal.Notify(intr, os.Interrupt)

		// The first interrupt signals the database managers to stop
		// accepting more requests
		<-intr
		old := dbact
		dbact = make(chan DBAction)
		time.Sleep(conf.Database.Timeout)
		close(old)

		// The second interrupt force-exits the process
		<-intr
		os.Exit(1)
	}()

	err = fs.WalkDir(sqlDir, "sql", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		base := path.Base(file)
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := fs.ReadFile(sqlDir, file)
		if err != nil {
			log.Fatal(err)
		}

		if strings.HasPrefix(base, "create-") {
			_, err = db.Exec(string(data))
		} else {
			queries[strings.TrimSuffix(base, ".sql")], err = db.Prepare(string(data))
		}
		if err != nil {
			log.Fatal(err)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	for id := uint(0); id < conf.Database.Threads; id++ {
		wg.Add(1)
		go databaseManager(id, db, &wg)
	}
	wg.Wait()
}
