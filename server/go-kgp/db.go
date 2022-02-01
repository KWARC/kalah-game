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
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed sql
var sqlDir embed.FS

var (
	// The database connection
	db *sql.DB

	// The SQL queries are stored under ./sql/, and they are loaded by the
	// database manager.  These are prepared and stored in QUERIES, that
	// the database actions use.
	queries = make(map[string]*sql.Stmt)
)

func (game *Game) updateDatabase() {
	if !game.logged {
		panic("Saving unlogged game")
	}

	tx, err := db.Begin()
	if err != nil {
		log.Print(err)
		return
	}
	defer tx.Rollback()

	var scid, ncid *int64
	if game.South != nil {
		scid = &game.South.Id
	}
	if game.North != nil {
		ncid = &game.North.Id
	}

	res, err := tx.Stmt(queries["insert-game"]).Exec(
		len(game.Board.northPits),
		game.Board.init,
		ncid, scid,
		game.Outcome)
	if err != nil {
		log.Fatal(err)
		return
	}
	game.Id, err = res.LastInsertId()
	if err != nil {
		log.Print(err)
	}

	for _, move := range game.Moves {
		var cid *int64
		if move.Client != nil {
			cid = &move.Client.Id
		}
		_, err = tx.Stmt(queries["insert-move"]).Exec(
			game.Id,
			cid,
			game.Side(move.Client),
			move.Pit,
			move.Comment,
			move.when)
		if err != nil {
			log.Fatal(err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Print(err)
	}
}

func registerTournament(name string) int64 {
	res, err := queries["insert-tournament"].Exec(name)
	if err != nil {
		log.Fatal(err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	return id
}

func (cli *Client) recordScore(game *Game, tid int64, score float64) {
	if cli == nil {
		return
	}

	cli.Score += score

	_, err := queries["insert-score"].Exec(cli.Id, game.Id, tid, score)
	if err != nil {
		log.Print(err)
	}
}

func (cli *Client) updateDatabase(query bool) {
	if cli == nil {
		return
	}

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, conf.Database.Timeout)
	defer cancel()

	var (
		name, descr *string
		score       *float64
	)

	res, err := queries["insert-agent"].ExecContext(ctx,
		cli.token,
		cli.Name,
		cli.Descr,
		cli.Author,
		cli.Score)
	if err != nil {
		log.Print(err)
		return
	}
	cli.Id, err = res.LastInsertId()
	if err != nil {
		log.Print(err)
	}

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
}

func (cli *Client) forget(token []byte) {
	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, conf.Database.Timeout)
	defer cancel()

	_, err := queries["delete-agent"].ExecContext(ctx, token)
	if err != nil {
		log.Print(err)
	}
}

func queryAgent(aid int, c chan<- *Agent) {
	var agent Agent

	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, conf.Database.Timeout)
	defer cancel()

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
}

func queryGame(gid int, c chan<- *Game) {
	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, conf.Database.Timeout)
	defer cancel()

	defer close(c)
	row := queries["select-game"].QueryRowContext(ctx, gid)
	game, err := scanGame(ctx, row.Scan)
	if err != nil {
		log.Print(err)
		return
	}

	rows, err := queries["select-moves"].QueryContext(ctx, gid)
	if err != nil {
		log.Print(err)
		return
	}

	expected := SideSouth
	for rows.Next() {
		var (
			aid  int64
			comm string
			move int
			side Side
		)

		err = rows.Scan(&aid, &comm, &move)
		if err != nil {
			log.Print(err)
			return
		}
		switch aid {
		case game.North.Id:
			side = SideNorth
		case game.South.Id:
			side = SideSouth
		default:
			panic("Unknown ID")
		}
		if side != expected {
			panic(fmt.Sprintf("Unexpected side (%v, got %v)", expected, side))
		}

		// TODO Ensure the next move is on the right
		// side, by checking the return value in the
		// next iteration.
		if !game.Board.Sow(side, move) {
			expected = !expected
		}

		game.Moves = append(game.Moves, &Move{
			Pit:     move,
			Client:  game.Player(side),
			Comment: comm,
			State:   game.Board.Copy(),
		})
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
	}

	c <- game
}

func scanGame(ctx context.Context, scan func(dest ...interface{}) error) (*Game, error) {
	var (
		game         Game
		north, south Agent
		outcome      Outcome
		size, init   uint
	)
	err := scan(
		&game.Id,
		&size, &init,
		&north.Id,
		&south.Id,
		&outcome,
		&game.MoveCount)
	if err != nil {
		return nil, err
	}

	game.Board = makeBoard(size, init)
	err = queries["select-agent-id"].QueryRowContext(ctx, north.Id).Scan(
		&north.Name,
		&north.Descr,
		&north.Author,
		&north.Score)
	if err != nil {
		return nil, err
	}
	game.North = &Client{Agent: north}

	err = queries["select-agent-id"].QueryRowContext(ctx, south.Id).Scan(
		&south.Name,
		&south.Descr,
		&south.Author,
		&south.Score)
	if err != nil {
		return nil, err
	}
	game.South = &Client{Agent: south}

	return &game, nil
}

func queryGames(aid int, c chan<- *Game, page int) {
	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, conf.Database.Timeout)
	defer cancel()

	defer close(c)

	var rows *sql.Rows
	rows, err := queries["select-games-by"].QueryContext(ctx,
		aid, page)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return
	}
	defer rows.Close()

	for rows.Next() {
		game, err := scanGame(ctx, rows.Scan)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Print(err)
			}
			return
		}
		c <- game
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
	}
}

func queryAgents(c chan<- *Agent, page int) {
	bg := context.Background()
	ctx, cancel := context.WithTimeout(bg, conf.Database.Timeout)
	defer cancel()

	defer close(c)
	rows, err := queries["select-agents"].QueryContext(ctx, page, conf.Web.Limit)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return
	}
	defer rows.Close()

	for rows.Next() {
		var agent Agent

		err = rows.Scan(
			&agent.Rank,
			&agent.Id,
			&agent.Name,
			&agent.Author,
			&agent.Score,
			&agent.Games)
		if err != nil {
			log.Fatal(err)
			return
		}

		c <- &agent
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
		return
	}
}

// Initialise the database and database managers
func prepareDatabase() {
	var err error
	db, err = sql.Open("sqlite3", conf.Database.File+"?mode=rwc")
	if err != nil {
		log.Fatal(err, ": ", conf.Database.File)
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(1)

	for _, pragma := range []string{
		// https://www.sqlite.org/pragma.html#pragma_journal_mode
		"journal_mode = WAL",
		// https://www.sqlite.org/pragma.html#pragma_synchronous
		"synchronous = normal",
		// https://www.sqlite.org/pragma.html#pragma_temp_store
		"temp_store = memory",
		// https://www.sqlite.org/pragma.html#pragma_mmap_size
		"mmap_size = 268435456",
		// https://www.sqlite.org/pragma.html#pragma_foreign_keys
		"foreign_keys = on",
	} {
		debug.Printf("Run PRAGMA %v", pragma)
		_, err = db.Exec("PRAGMA " + pragma + ";")
		if err != nil {
			log.Fatal(err)
		}

	}

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
			debug.Printf("Execute %v", base)
			_, err = db.Exec(string(data))
		} else {
			debug.Printf("Prepare %v", base)
			queries[strings.TrimSuffix(base, ".sql")], err = db.Prepare(string(data))
		}
		if err != nil {
			log.Fatal(file, ": ", err)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if conf.Database.Optimise {
		go func() {
			for {
				time.Sleep(8 * time.Hour)
				// https://www.sqlite.org/pragma.html#pragma_optimize
				_, err = db.Exec("PRAGMA optimize;")
				if err != nil {
					log.Print(err)
				}
			}
		}()
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		log.Println("Shutting down...")
		_, err = db.Exec("PRAGMA optimize;")
		if err != nil {
			log.Print(err)
		}
		err := db.Close()
		if err != nil {
			log.Println(err)
		}
		os.Exit(0)
	}()
}
