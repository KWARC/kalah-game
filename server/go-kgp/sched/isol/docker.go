// Docker-Based Client Isolation
//
// Copyright (c) 2022, 2023  Philip Kaludercic
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

package isol

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"

	"go-kgp"
	"go-kgp/cmd"
	"go-kgp/proto"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/pkg/errors"
)

var (
	hostname string
	c        int64
)

func init() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}
}

type docker struct {
	name string
	id   int64
}

type cli struct {
	C *client.Client // container
	l *proto.Listener
	c *proto.Client
	d *docker
	i string // container ID
	u *kgp.User
}

func MakeDockerAgent(name, build string) ControlledAgent {
	if build != "" {
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			log.Fatal(err)
		}

		tar, err := archive.TarWithOptions(build, &archive.TarOptions{})
		if err != nil {
			log.Fatal(err)
		}
		opts := types.ImageBuildOptions{
			Dockerfile: "Dockerfile",
			Tags:       []string{name},
			Remove:     true,
		}
		ctx := context.Background()
		res, err := cli.ImageBuild(ctx, tar, opts)
		if err != nil {
			log.Fatal(err)
		}
		l, err := os.Create(fmt.Sprintf("%s.%d.log", build, time.Now().Unix()))
		if err != nil {
			log.Println(err)
			goto skip
		}
		_, err = io.Copy(l, res.Body)
		if err != nil {
			log.Println(err)
		}
		err = l.Close()
		if err != nil {
			log.Println(err)
		}
	skip:
	}
	return &docker{
		id:   atomic.AddInt64(&c, 1),
		name: name,
	}
}

func (d *docker) String() string { return d.name }
func (*docker) Alive() bool      { return false }

func (d *docker) Request(g *kgp.Game) (*kgp.Move, bool) {
	panic("A docker client cannot make a move")
}

func (d *docker) User() *kgp.User {
	return &kgp.User{
		Id:   d.id,
		Name: d.name,
	}
}

func (d *docker) Start(st *cmd.State, conf *cmd.Conf) (kgp.Agent, error) {
	cont, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	// We start a new server on a random port (=0) for each client
	// to make coordinating client-server connections header.  The
	// sub-client has to confirm a connection before the
	// isolated/docker client is regarded as having started up.
	wait := make(chan *proto.Client)
	listener := proto.StartListner(st, conf, func(cli *proto.Client) bool {
		go cli.Connect(st)
		wait <- cli
		return true
	})
	kgp.Debug.Println("Connect", d, "to port", listener.Port())

	// The documentation for the library is sparse, but it is also
	// just a wrapper around a HTTP API.  To understand what this
	// configuration does, it is necessary to read
	// https://docs.docker.com/engine/api/v1.41/#operation/ContainerCreate
	ctx := context.Background()
	kgp.Debug.Println("Creating container for", d)
	resp, err := cont.ContainerCreate(ctx, &container.Config{
		Env: []string{
			fmt.Sprintf("KGP_HOST=%s", hostname),
			fmt.Sprintf("KGP_PORT=%d", listener.Port()),
		},
		Image: d.name,
	}, &container.HostConfig{
		Resources: container.Resources{
			CPUCount: int64(conf.Game.Closed.CPUs),
			Memory:   int64(conf.Game.Closed.Memory),
		},
		NetworkMode:    container.NetworkMode("host"),
		ReadonlyRootfs: true,
		AutoRemove:     true,
	}, nil, nil, fmt.Sprintf("%s-%d", d.name, time.Now().UnixNano()))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create container %s", d.name)
	}

	id := resp.ID
	kgp.Debug.Println("Starting container for", d)
	if err := cont.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrapf(err, "Failed to start container %s", d.name)
	}

	_, errC := cont.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	kgp.Debug.Println("Waiting for container", d)

	warmup := time.After(conf.Game.Closed.Warmup)
	select {
	case <-warmup:
		err := cont.ContainerKill(ctx, id, `SIGKILL`)
		if err != nil {
			log.Print(err)
		}
		return nil, errors.New("Timeout during initialisation")
	case err := <-errC:
		return nil, errors.Wrapf(err, "Container %v signalled an error", d.name)
	case client := <-wait:
		kgp.Debug.Println(d, "Connected to port", listener.Port())

		return &cli{
			l: listener,
			c: client,
			C: cont,
			i: id,
			d: d,
			u: &kgp.User{
				Name:  d.name,
				Token: fmt.Sprint(d.id),
			},
		}, nil
	}
}

func (*docker) Shutdown() error { return nil }

func (c *cli) String() string { return c.i }

func (d *cli) Start(*cmd.State, *cmd.Conf) (kgp.Agent, error) {
	panic("Cannot start a client")
}

func (c *cli) Alive() bool {
	ctx := context.Background()
	resp, err := c.C.ContainerInspect(ctx, c.i)
	if err != nil {
		kgp.Debug.Print(err)
		return false
	}
	return !resp.State.Dead // XXX: Is this enough?
}

func (c *cli) Request(g *kgp.Game) (*kgp.Move, bool) {
	s := &g.South
	if g.Side(c) == kgp.North {
		s = &g.North
	}

	// HACK: In case the game wants to check what side of the board
	// the agent is playing on, we need to ensure that the correct
	// kgp.Agent is assigned.  We will quickly swap the actual agent
	// for the sub-client and then let the sub-client act as if it
	// were the actual agent.  After this is done, we claim the
	// sub-clients works as our own by overwriting the agent reference
	// in the move
	*s = c.c
	m, r := c.c.Request(g)
	*s = c
	if m != nil {
		m.Agent = c
	}
	return m, r
}

func (c *cli) User() *kgp.User {
	return c.u
}

func (c *cli) Shutdown() error {
	c.l.Shutdown()
	ctx := context.Background()
	err := c.C.ContainerKill(ctx, c.i, "SIGKILL")
	if err != nil {
		return errors.Wrapf(err, "Failed to kill container %s", c.d.name)
	}

	return nil
}

// Check if docker and cli implements ControlledAgent
var (
	_ ControlledAgent = &docker{}
	_ ControlledAgent = &cli{}
)
