package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

func (st *State) DrawGraph(lang string) ([]byte, error) {
	bg := context.Background()
	ctx, cancel := context.WithCancel(bg)
	defer cancel()

	cmd := exec.Command(`dot`, fmt.Sprintf("-T%s", lang))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	go func() {
		err := st.Database.DrawGraph(ctx, stdin)
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

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(io.Discard, io.TeeReader(stderr, os.Stderr))
	if err != nil {
		return nil, err
	}

	return data, err
}
