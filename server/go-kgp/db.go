package main

import (
	"database/sql"
	"log"
	"sync"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

type DBAction func(*sql.DB) error

//go:embed sql/insert-move.sql
var sqlInsertMoveSrc string
var sqlInsertMove *sql.Stmt

func (game *Game) UpdateDatabase(db *sql.DB) error {
	res, err := sqlInsertGame.Exec(game.North.Id, game.South.Id)
	if err != nil {
		return err
	}
	game.Id, err = res.LastInsertId()
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
	_, err := sqlInsertGame.Exec(mov.cli.comment, mov.cli.Id, mov.game.Id)
	return err
}

//go:embed sql/insert-agent.sql
var sqlInsertAgentSrc string
var sqlInsertAgent *sql.Stmt

//go:embed sql/select-agent.sql
var sqlSelectAgentSrc string
var sqlSelectAgent *sql.Stmt

func (cli *Client) UpdateDatabase(wait *sync.WaitGroup) DBAction {
	log.Print("Request to save", cli)
	return func(db *sql.DB) error {
		log.Print("Starting to save", cli)
		res, err := sqlInsertAgent.Exec(
			cli.token, cli.Name, cli.Descr,
			cli.Name, cli.Descr, cli.Score)
		if err != nil {
			return err
		}
		cli.Id, err = res.LastInsertId()

		err = sqlSelectAgent.QueryRow(cli.Id).Scan(&cli.Score, nil, nil)
		if err != nil {
			cli.kill <- true
		}
		if wait != nil {
			wait.Done()
		}
		return nil
	}
}

func QueryAgent(aid uint, c chan<- *Client) DBAction {
	return func(db *sql.DB) error {
		var cli Client

		err := sqlSelectAgent.QueryRow(aid).Scan(&cli.Score, nil, nil)
		if err != nil {
			close(c)
		} else {
			c <- &cli
		}

		return err
	}
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
