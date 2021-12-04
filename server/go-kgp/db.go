package main

import (
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

	_ "github.com/mattn/go-sqlite3"
)

// The database manager accepts "database actions", ie. functions that
// operate on a database.  These are sent to the database manager or
// managers via the channel DBACT, that executes the action and
// handles possible errors.

type DBAction func(*sql.DB)

var dbact = make(chan DBAction, 1)

// The SQL queries are stored under ./sql/, and they are loaded by the
// database manager.  These are prepared and stored in QUERIES, that
// the database actions use.

//go:embed sql
var sqlDir embed.FS

var queries = make(map[string]*sql.Stmt)

func (game *Game) updateDatabase(db *sql.DB) {
	var err error
	if game.IsOver() {
		fmt.Printf("Update (%s): %v\n", game, game.Board.Outcome(SideSouth))
		_, err = queries["update-game"].Exec(game.Board.Outcome(SideSouth), game.Id)
	} else {
		var res sql.Result
		res, err = queries["insert-game"].Exec(
			len(game.Board.northPits),
			game.Board.init,
			game.North.Id,
			game.South.Id)
		if err == nil {
			game.Id, err = res.LastInsertId()
		}
	}
	if err != nil {
		log.Println(err)
	}
	return
}

func saveMove(in *Game, by *Client, side Side, move int) DBAction {
	var aid *int64

	if by.token != "" {
		aid = &by.Id
	}

	return func(db *sql.DB) {
		_, err := queries["insert-move"].Exec(
			in.Id,
			aid,
			side,
			move,
			by.comment)
		if err != nil {
			log.Println(err)
		}
		return
	}
}

func (cli *Client) updateDatabase(wait *sync.WaitGroup) DBAction {
	return func(db *sql.DB) {
		_, err := queries["insert-agent"].Exec(
			cli.token, cli.Name, cli.Descr,
			cli.Name, cli.Descr, cli.Score)
		if err != nil {
			log.Println(err)
			return
		}

		var name, descr string
		err = queries["select-agent-token"].QueryRow(cli.token).Scan(
			&cli.Id, &name, &descr, &cli.Score)
		if err != nil {
			cli.killFunc()
		}
		if wait != nil {
			wait.Done()
		}
		return
	}
}

func queryAgent(aid int, c chan<- *Agent) DBAction {
	return func(db *sql.DB) {
		var agent Agent

		defer close(c)
		err := queries["select-agent-id"].QueryRow(aid).Scan(
			&agent.Name,
			&agent.Descr,
			&agent.Score)
		if err != nil {
			log.Println(err)
		} else {
			c <- &agent
		}

		return
	}
}

func queryGame(gid int, c chan<- *Game) DBAction {
	return func(db *sql.DB) {
		defer close(c)
		row := queries["select-game"].QueryRow(gid)
		game, err := scanGame(row.Scan)
		if err != nil {
			log.Println(err)
			return
		}

		rows, err := queries["select-moves"].Query(gid)
		if err != nil {
			log.Println(err)
			return
		}

		for rows.Next() {
			var (
				aid  int64
				comm string
				move int
				side Side
			)

			err = rows.Scan(&aid, &side, &comm, &move)
			if err != nil {
				log.Println(err)
				return
			}

			// TODO Ensure the next move is on the right
			// side, by checking the return value in the
			// next iteration.
			game.Board.Sow(side, move)

			game.Moves = append(game.Moves, &Move{
				Pit:     move,
				Client:  game.Player(side),
				game:    game,
				Comment: comm,
			})
		}
		if err = rows.Err(); err != nil {
			log.Println(err)
		}

		c <- game
		return
	}
}

func scanGame(scan func(dest ...interface{}) error) (*Game, error) {
	var (
		game         Game
		north, south Agent
		outcome      *uint8
		size, init   uint
	)

	err := scan(
		&game.Id,
		&size, &init,
		&north.Id,
		&south.Id,
		&outcome,
		&game.Started,
		&game.Ended)
	if err != nil {
		return nil, err
	}

	game.Board = makeBoard(size, init)

	if game.Ended != nil {
		game.Outcome = ONGOING
	} else if outcome == nil {
		game.Outcome = RESIGN
	} else {
		game.Outcome = Outcome(*outcome)
	}

	err = queries["select-agent-id"].QueryRow(north.Id).Scan(
		&north.Name,
		&north.Descr,
		&north.Score)
	if err != nil {
		return nil, err
	}
	game.North = &Client{Agent: north}

	err = queries["select-agent-id"].QueryRow(south.Id).Scan(
		&south.Name,
		&south.Descr,
		&south.Score)
	if err != nil {
		return nil, err
	}
	game.South = &Client{Agent: south}

	return &game, nil
}

func queryGames(c chan<- *Game, page int, aid *int) DBAction {
	return func(db *sql.DB) {
		var (
			rows *sql.Rows
			err  error
		)

		defer close(c)
		if aid == nil {
			rows, err = queries["select-games"].Query(page, conf.Web.Limit)
		} else {
			rows, err = queries["select-games-by"].Query(*aid, page)
		}
		if err != nil {
			if err != sql.ErrNoRows {
				log.Println(err)
			}
			return
		}
		defer rows.Close()

		var game *Game
		for rows.Next() {
			game, err = scanGame(rows.Scan)
			if err != nil {
				if err != sql.ErrNoRows {
					log.Println(err)
				}
				return
			}
			c <- game
		}
		if err = rows.Err(); err != nil {
			log.Println(err)
		}

		return
	}
}

func queryAgents(c chan<- *Agent, page int) DBAction {
	return func(db *sql.DB) {
		defer close(c)
		rows, err := queries["select-agents"].Query(page, conf.Web.Limit)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Println(err)
			}
			return
		}
		defer rows.Close()

		for rows.Next() {
			var agent Agent

			err = rows.Scan(&agent.Id, &agent.Name, &agent.Score, &agent.Games)
			if err != nil {
				log.Println(err)
				return
			}

			c <- &agent
		}
		if err = rows.Err(); err != nil {
			log.Println(err)
		}

		return
	}
}

func databaseManager(id uint, db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()

	for act := range dbact {
		if act == nil {
			continue
		}

		act(db)
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
		close(dbact)

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
