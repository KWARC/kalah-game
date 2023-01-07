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
	"strconv"
	"time"

	"go-kgp"
	cmd "go-kgp/cmd"
	"go-kgp/proto"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

type docker struct {
	name string
	id   string
	cont *client.Client
	cli  *proto.Client
	lis  *proto.Listener
}

func (d *docker) Alive() bool {
	ctx := context.Background()
	resp, err := d.cont.ContainerInspect(ctx, d.id)
	if err != nil {
		kgp.Debug.Print(err)
		return false
	}
	return !resp.State.Dead // XXX: Is this enough?
}

func (d *docker) Request(g *kgp.Game) (*kgp.Move, bool) {
	return d.cli.Request(g)
}

func (d *docker) User() *kgp.User {
	return d.cli.User()
}

func (d *docker) Start(mode *cmd.State) (kgp.Agent, error) {
	var err error
	d.cont, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	// We start a new server on a random port (=0) for each client
	// to make coordinating client-server connections header.  The
	// sub-client has to confirm a connection before the
	// isolated/docker client is regarded as having started up.
	wait := make(chan *proto.Client)
	d.lis = proto.StartListner(mode, func(cli *proto.Client) bool {
		wait <- cli
		return true
	})

	// Figure out what port the server is listening on
	port := strconv.FormatUint(uint64(d.lis.Port()), 10)

	// The documentation for the library is sparse, but it is also
	// just a wrapper around a HTTP API.  To understand what this
	// configuration does, it is necessary to read
	// https://docs.docker.com/engine/api/v1.41/#operation/ContainerCreate
	ctx := context.Background()
	resp, err := d.cont.ContainerCreate(ctx, &container.Config{
		Image: d.name,
	}, &container.HostConfig{
		Resources: container.Resources{
			CPUCount: 1,
			Memory:   1024 * 1024 * 1024,
		},
		PortBindings: nat.PortMap{
			nat.Port("2671/tcp"): []nat.PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: port,
			}},
		},
		ReadonlyRootfs: true,
		AutoRemove:     true,
	}, nil, nil, fmt.Sprintf("%s-%d", d.name, time.Now().UnixNano()))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create container %s", d.name)
	}

	d.id = resp.ID
	if err := d.cont.ContainerStart(ctx, d.id, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrapf(err, "Failed to start container %s", d.name)
	}

	okC, errC := d.cont.ContainerWait(ctx, d.id, container.WaitConditionNotRunning)
	select {
	case err := <-errC:
		return nil, errors.Wrapf(err, "Container %v signalled an error", d.name)
	case <-okC:
		d.cli = <-wait
		return d.cli, nil
	}
}

func (d *docker) Shutdown() error {
	d.lis.Shutdown()
	ctx := context.Background()
	err := d.cont.ContainerKill(ctx, d.id, "SIGKILL")
	if err != nil {
		return errors.Wrapf(err, "Failed to kill container %s", d.name)
	}
	return nil
}

// Check if docker implements ControlledAgent
var _ ControlledAgent = &docker{}
