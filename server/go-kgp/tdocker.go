// Tournament handling using Docker (https://www.docker.com/)
//
// Copyright (c) 2022  Philip Kaludercic
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
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Hostname of the current system
var hostname string

func init() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}
}

// Docker isolates a client within a docker container
type Docker struct {
	id   string
	name string
	cli  *client.Client
}

// Start an isolating docker container and connect to PORT
func (d *Docker) Start(port string) error {
	var err error
	d.cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	// The documentation for the library is sparse, but it is also
	// just a wrapper around a HTTP API.  To understand what this
	// configuration does, it is necessary to read
	// https://docs.docker.com/engine/api/v1.41/#operation/ContainerCreate
	ctx := context.Background()
	resp, err := d.cli.ContainerCreate(ctx, &container.Config{
		Image: d.name,
		Cmd:   []string{hostname, port},
	}, &container.HostConfig{
		Resources: container.Resources{
			CPUCount:   int64(conf.Tourn.Docker.CPUs),
			Memory:     int64(conf.Tourn.Docker.Memory),
			MemorySwap: int64(conf.Tourn.Docker.Swap),
		},
		ReadonlyRootfs: true,
		NetworkMode:    container.NetworkMode(conf.Tourn.Docker.Network),
		AutoRemove:     true,
	}, nil, nil, fmt.Sprintf("%s-%d", d.name, time.Now().UnixNano()))
	if err != nil {
		log.Fatal("Failed to create container ", d.name, ": ", err)
		return err
	}
	defer d.Halt()

	d.id = resp.ID
	if err := d.cli.ContainerStart(ctx, d.id, types.ContainerStartOptions{}); err != nil {
		log.Fatal("Failed to start container ", d.name, ": ", err)
		return err
	}

	okC, errC := d.cli.ContainerWait(ctx, d.id, container.WaitConditionNotRunning)
	select {
	case err := <-errC:
		log.Printf("Container %v signalled an error: %s", d.name, err)
		return err
	case <-okC:
		return nil
	}
}

// Kill the isolating Docker container
func (d *Docker) Halt() error {
	ctx := context.Background()
	err := d.cli.ContainerKill(ctx, d.id, "SIGKILL")
	if err != nil {
		log.Print("Failed to kill container ", d.name, ": ", err)
		return err
	}
	return nil
}
