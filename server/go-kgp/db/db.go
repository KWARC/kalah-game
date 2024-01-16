// Database management
//
// Copyright (c) 2021, 2022, 2023, 2024  Philip Kaludercic
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

package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"go-kgp"
	"go-kgp/cmd"
	"go-kgp/game"
)

//go:embed *.sql
var sql_dir embed.FS

type db struct {
	// The database connections
	read  *sql.DB
	write *sql.DB

	// The SQL queries are stored under ./sql/, and they are
	// loaded by the database manager.  QUERIES are the commands
	// handle by READ, and COMMANDS are the queries handled by
	// WRITE.
	queries  map[string]*sql.Stmt
	commands map[string]*sql.Stmt
}

type user kgp.User

func (u *user) Request(*kgp.Game) (*kgp.Move, bool) {
	panic("Cannot request a move from a user")
}

func (u *user) User() *kgp.User {
	return (*kgp.User)(u)
}

func (u *user) Alive() bool {
	return false // users aren't live agents
}

func (db *db) Forget(ctx context.Context, token []byte) {
	_, err := db.commands["delete-agent"].ExecContext(ctx, token)
	if err != nil {
		log.Print(err)
	}
}

func (db *db) QueryUserToken(ctx context.Context, token string) *kgp.User {
	var u kgp.User
	err := db.queries["select-agent-token"].QueryRowContext(ctx, token).Scan(
		&u.Id,
		&u.Name,
		&u.Descr)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Print(err)
		}
		return nil
	}
	return &u
}

func (db *db) queryUser(ctx context.Context, id int) (*kgp.User, error) {
	u := kgp.User{Id: int64(id)}
	return &u, db.queries["select-agent-id"].QueryRowContext(ctx, id).Scan(
		&u.Name,
		&u.Descr,
		&u.Author,
		&u.Games)
}

func (db *db) QueryUser(ctx context.Context, id int) *kgp.User {
	u, err := db.queryUser(ctx, id)
	if err != nil {
		log.Print(err)
		return nil
	}
	return u
}

func (db *db) QueryGame(ctx context.Context, gid int, gc chan<- *kgp.Game, mc chan<- *kgp.Move) {
	defer close(gc)
	defer close(mc)
	row := db.queries["select-game"].QueryRowContext(ctx, gid)
	g, err := db.scanGame(ctx, row.Scan)
	if err != nil {
		log.Print(err)
		return
	}
	gc <- g

	rows, err := db.queries["select-moves"].QueryContext(ctx, gid)
	if err != nil {
		log.Print(err)
		return
	}

	for rows.Next() {
		var (
			m    = &kgp.Move{}
			side bool
		)
		err = rows.Scan(&side, &m.Comment, &m.Choice, &m.Stamp)
		if err != nil {
			log.Print(err)
			return
		}
		m.Agent = g.Player(kgp.Side(side))

		if next, repeat := game.MoveCopy(g, m); !repeat {
			log.Printf("Illegal move %d on %s", m.Choice, g.Board)
			break
		} else {
			g = next
		}
		m.State = g.Board.Copy()

		mc <- m
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
	}
}

func (db *db) scanGame(ctx context.Context, scan func(dest ...interface{}) error) (game *kgp.Game, err error) {
	var (
		nid, sid   int
		size, init uint
		lastmove   sql.NullString
	)

	game = &kgp.Game{}
	err = scan(
		&game.Id,
		&size, &init,
		&nid, &sid,
		&game.State,
		&game.MoveCount,
		&lastmove)
	if err != nil {
		return
	}
	game.Board = kgp.MakeBoard(size, init)

	var south, north *kgp.User
	south, err = db.queryUser(ctx, sid)
	if err != nil {
		return
	}
	north, err = db.queryUser(ctx, nid)
	if err != nil {
		return
	}

	game.North = (*user)(north)
	game.South = (*user)(south)

	if lastmove.Valid {
		// 2023-01-04 12:35:09.0648946+01:00
		// tfmt := `2006-01-02 15:04:05.9999999+07:00`
		tfmt := `2006-01-02 15:04:05.999999999-07:00`
		game.LastMove, err = time.Parse(tfmt, lastmove.String)
	}

	return
}

func (db *db) QueryGames(ctx context.Context, aid int, c chan<- *kgp.Game, page int) {
	defer close(c)

	var (
		rows *sql.Rows
		err  error
	)
	switch {
	case aid < 0:
		rows, err = db.queries["select-games"].QueryContext(ctx, page)
	case aid == 0:
		rows, err = db.queries["select-all-games"].QueryContext(ctx)
	default:
		rows, err = db.queries["select-games-by"].QueryContext(ctx,
			aid, page)
	}
	if err != nil {
		log.Print(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		game, err := db.scanGame(ctx, rows.Scan)
		if err != nil {
			log.Print(err)
			continue
		}
		c <- game
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
	}
}

func (db *db) QueryUsers(ctx context.Context, c chan<- *kgp.User, page int) {
	defer close(c)
	rows, err := db.queries["select-agents"].QueryContext(ctx, page)
	if err != nil {
		log.Print(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var u kgp.User

		err = rows.Scan(
			&u.Id,
			&u.Name,
			&u.Author,
			&u.Games)
		if err != nil {
			log.Print(err)
			continue
		}

		c <- &u
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
		return
	}
}

func (db *db) SaveGame(ctx context.Context, game *kgp.Game) (err error) {
	var tx *sql.Tx
	tx, err = db.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to begin transaction: %w", err)
	}
	defer func() {
		terr := tx.Rollback()
		if terr != nil && !errors.Is(terr, sql.ErrTxDone) {
			err = fmt.Errorf("Failed to rollback transaction: %w, %w", terr, err)
		}
	}()

	if game.South != nil && game.South.User() != nil {
		err = db.saveUser(ctx, tx, game.South.User())
		if err != nil {
			return
		}
	}
	if game.North != nil && game.North.User() != nil {
		err = db.saveUser(ctx, tx, game.North.User())
		if err != nil {
			return
		}
	}
	if err = db.saveGame(ctx, tx, game); err != nil {
		return
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit transaction: %w", err)
	}
	return nil
}

func (db *db) saveGame(ctx context.Context, tx *sql.Tx, game *kgp.Game) error {
	if game.Id == 0 {
		north, south := game.North.User(), game.South.User()

		size, init := game.Board.Type()
		kgp.Debug.Printf("Saving game with SID %d and NID %d",
			south.Id, north.Id)
		res, err := tx.Stmt(db.commands["insert-game"]).ExecContext(ctx,
			size, init, north.Id, south.Id, game.State.String())
		if err != nil {
			log.Print(err)
			return err
		}

		id, err := res.LastInsertId()
		if err != nil {
			log.Print(err)
			return err
		}
		game.Id = uint64(id)
	} else {
		_, err := tx.Stmt(db.commands["update-game"]).ExecContext(ctx,
			game.State.String(), game.Id)
		if err != nil {
			log.Print(err)
			return err
		}
	}

	return nil
}

func (db *db) saveUser(ctx context.Context, tx *sql.Tx, u *kgp.User) error {
	if u.Id != 0 {
		return nil
	}

	if u.Token != "" {
		var id sql.NullInt64
		var name, desc sql.NullString
		res, err := db.queries["select-agent-token"].QueryContext(ctx, u.Token)
		if err != nil {
			// FIXME: The user should be allowed to update
			//        their metadata.
			kgp.Debug.Print(err)
			goto insert
		}
		if !res.Next() {
			goto insert
		}
		err = res.Scan(&id, &name, &desc)
		if err == nil {
			if id.Valid {
				u.Id = id.Int64
			}
			if name.Valid {
				if u.Name != name.String {
					goto insert
				}
				u.Name = name.String
			}
			if desc.Valid {
				if u.Descr != desc.String {
					goto insert
				}
				u.Descr = desc.String
			}
			return nil
		} else {
			return err
		}
	}
insert:

	kgp.Debug.Printf("Saving user with %q token %q", u.Name, u.Token)
	res, err := tx.Stmt(db.commands["insert-agent"]).ExecContext(ctx,
		u.Token, u.Name, u.Descr, u.Author)
	if err != nil {
		return fmt.Errorf("Failed to save agent %q: %w", u.Name, err)
	}
	u.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("Failed to fetch agent ID: %w", err)
	}
	kgp.Debug.Printf("Assigned user %q ID %d", u.Name, u.Id)

	return nil
}

func (db *db) SaveMove(ctx context.Context, move *kgp.Move) (err error) {
	var tx *sql.Tx
	tx, err = db.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to begin transaction: %w", err)
	}
	defer func() {
		terr := tx.Rollback()
		if terr != nil && !errors.Is(terr, sql.ErrTxDone) {
			err = fmt.Errorf("Failed to rollback transaction: %w, %w", terr, err)
		}
	}()

	game := move.Game
	south, north := game.South.User(), game.North.User()
	if err = db.saveUser(ctx, tx, south); err != nil {
		return fmt.Errorf("Failed to save south user: %w", err)
	}
	if err = db.saveUser(ctx, tx, north); err != nil {
		return fmt.Errorf("Failed to save north user: %w", err)
	}
	if err = db.saveGame(ctx, tx, game); err != nil {
		return fmt.Errorf("Failed to save game between %q and %q: %w",
			south.Id, north.Id, err)
	}

	_, err = tx.Stmt(db.commands["insert-move"]).ExecContext(ctx,
		game.Id,
		move.Agent.User().Id,
		game.Side(move.Agent),
		move.Choice,
		move.Comment)
	if err != nil {
		return fmt.Errorf("Failed to save move in game %q: %w",
			game.Id, err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit transaction: %w", err)
	}
	return nil
}

func (db *db) QueryGraph(ctx context.Context, g chan<- *kgp.Game) error {
	defer close(g)
	res, err := db.queries["select-graph"].QueryContext(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Println("Empty response")
			return nil
		}
		return err
	}
	defer res.Close()

	for res.Next() {
		var (
			w, l user
			sid  int64
		)
		err = res.Scan(&w.Name, &w.Id, &l.Name, &l.Id, &sid)
		if err != nil {
			return err
		}

		switch sid {
		case w.Id: // winner is on the sound
			g <- &kgp.Game{
				State: kgp.SOUTH_WON,
				South: &w,
				North: &l,
			}
		case l.Id: // winner is on the north
			g <- &kgp.Game{
				State: kgp.NORTH_WON,
				South: &l,
				North: &w,
			}
		default:
			log.Panicln("SID", sid, "is neither", w.Id, "or", l.Id)
			continue
		}
	}

	return nil
}

func (db *db) Start(st *cmd.State, conf *cmd.Conf) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	tick := make(chan struct{})
	go func() {
		for {
			tick <- struct{}{}
			time.Sleep(conf.Database.Cleanup)
		}
	}()
	for {
		var err error
		select {
		case <-c:
			kgp.Debug.Print("Vacuuming database")
			// https://www.sqlite.org/lang_vacuum.html
			_, err = db.write.Exec("VACUUM;")
		case <-tick:
			log.Println("Deleting games")
			var res sql.Result
			res, err = db.commands["delete-games"].Exec()
			if err != nil {
				log.Print(err)
				break
			}

			var n int64
			n, err = res.RowsAffected()
			if err != nil {
				log.Print(err)
				break
			}
			kgp.Debug.Println("Deleted", n, "old games")
			// https://www.sqlite.org/pragma.html#pragma_optimize
			_, err = db.write.Exec("PRAGMA optimize;")
		}
		if err != nil {
			log.Print(err)
		}
	}
}

func (db *db) Shutdown() {
	var err error

	// https://www.sqlite.org/pragma.html#pragma_optimize
	_, err = db.write.Exec("PRAGMA optimize;")
	if err != nil {
		log.Print(err)
	}

	err = db.write.Close()
	if err != nil {
		log.Print(err)
	}

	err = db.read.Close()
	if err != nil {
		log.Print(err)
	}
}

func (*db) String() string { return "Database Manager" }

// Initialise the database and database managers
func Register(st *cmd.State, conf *cmd.Conf) {
	kgp.Debug.Println("Opening Database", conf.Database.File)
	read, err := sql.Open("sqlite3", conf.Database.File)
	if err != nil {
		log.Fatal(err, ": ", conf.Database)
	}
	read.SetConnMaxLifetime(0)
	read.SetMaxIdleConns(1)

	write, err := sql.Open("sqlite3", conf.Database.File)
	if err != nil {
		log.Fatal(err, ": ", conf.Database)
	}
	write.SetConnMaxLifetime(0)
	write.SetMaxIdleConns(1)
	write.SetMaxOpenConns(1)

	db := &db{
		queries:  make(map[string]*sql.Stmt),
		commands: make(map[string]*sql.Stmt),
		write:    write,
		read:     read,
	}

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
		kgp.Debug.Printf("Run PRAGMA %v", pragma)
		_, err = db.write.Exec("PRAGMA " + pragma + ";")
		if err != nil {
			log.Fatal(err)
		}
	}

	entries, err := sql_dir.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || strings.HasPrefix(".", entry.Name()) {
			continue
		}

		base := path.Base(entry.Name())
		data, err := fs.ReadFile(sql_dir, entry.Name())
		if err != nil {
			log.Fatal(err)
		}

		if strings.HasPrefix(base, "create-") || strings.HasPrefix(base, "run-") {
			_, err = db.write.Exec(string(data))
			kgp.Debug.Printf("Executed query %v", base)
		} else {
			query := strings.TrimSuffix(base, ".sql")
			if strings.HasPrefix(query, "select-") {
				db.queries[query], err = db.read.Prepare(string(data))
				kgp.Debug.Printf("Registered query %v", query)
			} else {
				db.commands[query], err = db.write.Prepare(string(data))
				kgp.Debug.Printf("Registered command %v", query)
			}
		}
		if err != nil {
			log.Fatal(entry.Name(), ": ", err)
		}
	}

	if len(db.queries) == 0 {
		panic("No queries loaded")
	}

	st.Register(cmd.Database(db))
}
