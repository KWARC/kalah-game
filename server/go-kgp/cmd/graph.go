package cmd

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"go-kgp"
)

func genGraph(g <-chan *kgp.Game, w io.Writer) error {
	seen := make(map[int64]struct{})
	node := func(id int64, name string) (string, error) {
		var err error
		node := fmt.Sprintf("n%d", id)
		if _, ok := seen[id]; ok {
			return node, nil
		}
		if name == "" {
			name = fmt.Sprintf("Unnamed (%d)", id)
		}
		name = strings.ReplaceAll(name, `"`, `\"`)
		_, err = fmt.Fprintf(w, `%s [label="%s" href="/agent/%d"];`,
			node, name, id)
		if err != nil {
			return "", err
		}
		return node, nil
	}

	_, err := fmt.Fprintf(w, `strict digraph dominance { ratio = compress ;`)
	if err != nil {
		return err
	}

	for game := range g {
		var win, loss *kgp.User
		switch game.State {
		case kgp.NORTH_WON:
			win, loss = game.North.User(), game.South.User()
		case kgp.SOUTH_WON:
			loss, win = game.North.User(), game.South.User()
		default:
			continue
		}

		t, err := node(loss.Id, loss.Name)
		if err != nil {
			return err
		}
		f, err := node(win.Id, win.Name)
		if err != nil {
			return err
		}

		_, err = fmt.Fprint(w, f, "->", t, ";")
		if err != nil {
			return err
		}
	}

	_, err = fmt.Fprint(w, `}`)
	if err != nil {
		return err
	}

	return nil
}

func (st *State) DrawGraph(g <-chan *kgp.Game, opts ...string) ([]byte, error) {
	cmd := exec.Command(`dot`, opts...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	go func() {
		err := genGraph(g, stdin)
		if err != nil {
			log.Print(err)
			return
		}
		err = stdin.Close()
		if err != nil {
			log.Print(err)
			return
		}
	}()

	out, err := io.ReadAll(stdout)
	if err := cmd.Wait(); err != nil {
		log.Println(err)
		return nil, err
	}
	return out, err
}
