package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type DBAction func(*sql.DB) error

func (cli *Client) UpdateDatabase(db *sql.DB) error {
	res, err := db.Exec(`INSERT INTO agent(token, name, descr)
                             VALUES (?, ?, ?)
                               ON CONFLICT (token) DO UPDATE SET
                                 name = ?, descr = ?`,
		cli.token, cli.name, cli.descr,
		cli.name, cli.descr)
	if err != nil {
		return err
	}
	cli.dbid, err = res.LastInsertId()
	return err
}

func (game *Game) UpdateDatabase(db *sql.DB) error {
	res, err := db.Exec(`INSERT INTO game(north, south, start)
                             VALUES (?, ?, DATETIME('now'))`,
		game.north.dbid, game.south.dbid)
	if err != nil {
		return err
	}
	game.dbid, err = res.LastInsertId()
	return err
}

//go:embed sql/insert-game.sql
var sqlInsertGameSrc string
var sqlInsertGame *sql.Stmt

func (mov *Move) UpdateDatabase(db *sql.DB) error {
	// Do not save a move if the game has been invalidated
	if mov.game == nil {
		return nil
	}
	_, err := sqlInsertGame.Exec(mov.cli.comment, mov.cli.dbid, mov.game.dbid)
	return err
}

var dbact = make(chan DBAction, 64)

//go:embed sql/create-agent.sql
var sqlCreateAgentSrc string
var sqlCreateAgent *sql.Stmt

//go:embed sql/create-game.sql
var sqlCreateGameSrc string
var sqlCreateGame *sql.Stmt

//go:embed sql/create-move.sql
var sqlCreateMoveSrc string
var sqlCreateMove *sql.Stmt

func manageDatabase(file string) {
	db, err := sql.Open("sqlite3", file+"?mode=rwc&_journal=wal")
	if err != nil {
		log.Fatal(err)
	}
	defer close(dbact)
	defer db.Close()

	// Prepare statements
	for _, ent := range []struct {
		sql  string
		stmt **sql.Stmt
	}{
		{sqlInsertMoveSrc, &sqlInsertMove},
		{sqlInsertGameSrc, &sqlInsertGame},
		{sqlInsertAgentSrc, &sqlInsertAgent},
		{sqlSelectAgentSrc, &sqlSelectAgent},
		{sqlCreateAgentSrc, &sqlCreateAgent},
		{sqlCreateGameSrc, &sqlCreateGame},
		{sqlCreateMoveSrc, &sqlCreateMove},
	} {
		*ent.stmt, err = db.Prepare(ent.sql)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create tables
	_, err = sqlCreateAgent.Exec()
	if err != nil {
		log.Fatal(err)
	}
	_, err = sqlCreateGame.Exec()
	if err != nil {
		log.Fatal(err)
	}
	_, err = sqlCreateMove.Exec()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Waiting for Database actions")
	for act := range dbact {
		err := act(db)
		if err != nil {
			log.Print(err)
		}
	}
	log.Print("Terminating Database manager")
}
