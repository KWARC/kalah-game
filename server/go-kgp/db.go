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
	"sync/atomic"
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

func (m *Move) updateDatabase(in *Game) DBAction {
	if !in.logged {
		return nil
	}

	when := time.Now()

	return func(db *sql.DB, ctx context.Context) (err error) {
		_, err = queries["insert-move"].Exec(
			in.Id,
			m.Client.Id,
			in.side,
			m.Pit,
			m.Comment,
			when)
		if err != nil {
			log.Print(err)
		}
		return
	}
}

func (game *Game) updateDatabase(wait *sync.WaitGroup) DBAction {
	if !game.logged {
		panic("Saving unlogged game")
	}

	return func(db *sql.DB, ctx context.Context) (err error) {
		defer wait.Done()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			log.Print(err)
			return err
		}
		defer tx.Rollback()

		res, err := tx.Stmt(queries["insert-game"]).ExecContext(ctx,
			len(game.Board.northPits),
			game.Board.init,
			game.North.Id,
			game.South.Id,
			game.Board.Outcome(SideSouth))
		if err != nil {
			log.Fatal(err)
			return
		}
		game.Id, err = res.LastInsertId()
		if err != nil {
			log.Print(err)
		}

		for _, move := range game.Moves {
			_, err = tx.Stmt(queries["insert-move"]).ExecContext(ctx,
				game.Id,
				move.Client.Id,
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

		return
	}
}

func registerTournament(name string, c chan<- int64) DBAction {
	return func(db *sql.DB, ctx context.Context) error {
		defer close(c)
		res, err := queries["insert-tournament"].ExecContext(ctx, name)
		if err == nil {
			id, err := res.LastInsertId()
			if err != nil {
				log.Print(err)
			} else {
				c <- id
			}
		}
		return err
	}
}

func (cli *Client) recordScore(game *Game, tid int64, score float64) DBAction {
	return func(db *sql.DB, ctx context.Context) (err error) {
		_, err = queries["insert-score"].ExecContext(ctx,
			cli.Id, game.Id, tid, score)
		return
	}
}

func (cli *Client) updateDatabase(wait *sync.WaitGroup, query bool) DBAction {
	return func(db *sql.DB, ctx context.Context) (err error) {
		var (
			name, descr *string
			score       *float64
		)
		defer wait.Done()

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

func queryGame(gid int, c chan<- *Game) DBAction {
	return func(db *sql.DB, ctx context.Context) error {
		defer close(c)
		row := queries["select-game"].QueryRowContext(ctx, gid)
		game, err := scanGame(ctx, row.Scan)
		if err != nil {
			log.Print(err)
			return err
		}

		rows, err := queries["select-moves"].QueryContext(ctx, gid)
		if err != nil {
			log.Print(err)
			return err
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
				return err
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
		return err
	}
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

func queryGames(aid int, c chan<- *Game, page int) DBAction {
	return func(db *sql.DB, ctx context.Context) (err error) {
		defer close(c)

		var rows *sql.Rows
		rows, err = queries["select-games-by"].QueryContext(ctx,
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
				return err
			}
			c <- game
		}
		if err = rows.Err(); err != nil {
			log.Print(err)
		}

		return
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

			err = rows.Scan(
				&agent.Rank,
				&agent.Id,
				&agent.Name,
				&agent.Author,
				&agent.Score,
				&agent.Games)
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

// Thread pool worker for database actions
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

		context, cancel := context.WithTimeout(context.Background(), conf.Database.Timeout*10000)
		act(db, context)
		if err := context.Err(); err != nil {
			log.Println(err)
		}
		cancel()
	}
}

// Initialise the database and database managers
func manageDatabase() {
	db, err := sql.Open("sqlite3", conf.Database.File+"?mode=rwc")
	if err != nil {
		log.Fatal(err, ": ", conf.Database.File)
	}
	defer db.Close()

	go func() {
		intr := make(chan os.Signal, 1)
		signal.Notify(intr, os.Interrupt)

		// The first interrupt signals the database managers to stop
		// accepting more requests
		<-intr
		go shutdown.Do(closeDB)

		// The second interrupt force-exits the process
		<-intr
		os.Exit(1)
	}()

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

	var wg sync.WaitGroup
	for id := uint(0); id < conf.Database.Threads; id++ {
		wg.Add(1)
		go databaseManager(id, db, &wg)
	}
	wg.Wait()
}

// Shutdown synchronisation object
//
// When the program is shutdown, use this to call closeDB.
var shutdown sync.Once

// Initiate a database shutdown
func closeDB() {
	debug.Print("Shutting down...")
	time.Sleep(conf.Database.Timeout * 2)

	// Wait for ongoing games to finish
	for atomic.LoadUint64(&playing) > 0 {
		time.Sleep(time.Second)
	}

	// Wait for the actual queue to empty itself, then terminate
	// the database managers
	for len(dbact) > 0 {
		time.Sleep(conf.Database.Timeout)
	}
	close(dbact)
}
