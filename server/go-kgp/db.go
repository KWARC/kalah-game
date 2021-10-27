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

func (mov *Move) UpdateDatabase(db *sql.DB) error {
	_, err := db.Exec(`INSERT INTO move(comment, agent, game, played)
                           VALUES (?, ?, ?, DATETIME('now'))`,
		mov.cli.comment,
		mov.cli.dbid,
		mov.game.dbid)
	return err
}

var dbact = make(chan DBAction, 64)

func manageDatabase(file string) {
	db, err := sql.Open("sqlite3", file+"?mode=rwc&_journal=wal")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	defer close(dbact)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS agent (
                            id INTEGER PRIMARY KEY AUTOINCREMENT,
                            token TEXT UNIQUE,
                            name TEXT,
                            descr TEXT,
                            score REAL
                          );`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS game (
                            id INTEGER PRIMARY KEY AUTOINCREMENT,
                            north REFERENCES agent(id),
                            south REFERENCES agent(id),
                            result INTEGER,
                            start DATETIME
                          );`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS move (
                            comment TEXT,
                            agent REFERENCES agent(id),
                            game REFERENCES game(id),
                            played DATETIME
                          );`)
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
