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
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

var (
	dCli  *client.Client
	dSync sync.Mutex
)

type Docker struct {
	id    string
	name  string
	pause uint32
	awake chan struct{}
}

func (d *Docker) Run(port string) error {
	d.awake = make(chan struct{})

	dSync.Lock()
	if dCli == nil {
		var err error
		dCli, err = client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			panic(err)
		}
	}
	dSync.Unlock()

	// The documentation for the library is sparse, but it is also
	// just a wrapper around a HTTP API.  To understand what this
	// configuration does, it is necessary to read
	// https://docs.docker.com/engine/api/v1.41/#operation/ContainerCreate
	ctx := context.Background()
	resp, err := dCli.ContainerCreate(ctx, &container.Config{
		NetworkDisabled: true,
		Image:           d.name,
	}, &container.HostConfig{
		Resources: container.Resources{
			CPUCount:   int64(conf.Tourn.Docker.CPUs),
			Memory:     int64(conf.Tourn.Docker.Memory),
			MemorySwap: int64(conf.Tourn.Docker.Swap),
		},
		ReadonlyRootfs: true,
		PortBindings: map[nat.Port][]nat.PortBinding{
			"2671/tcp": {{
				HostIP:   "localhost",
				HostPort: port,
			}},
		},
		NetworkMode: container.NetworkMode(conf.Tourn.Docker.Network),
	}, nil, nil, d.name)
	if err != nil {
		log.Print("Failed to create container ", d.name)
		return err
	}

	d.id = resp.ID
	if err := dCli.ContainerStart(ctx, d.id, types.ContainerStartOptions{}); err != nil {
		log.Print("Failed to start container ", d.name)
		return err
	}

	okC, errC := dCli.ContainerWait(ctx, d.id, container.WaitConditionNotRunning)
	select {
	case err := <-errC:
		log.Printf("Container %v signalled an error: %s", d.name, err)
		return err
	case <-okC:
		return nil
	}
}

func (d *Docker) Halt() error {
	ctx := context.Background()
	err := dCli.ContainerKill(ctx, d.id, "SIGKILL")
	if err != nil {
		log.Print("Failed to start container ", d.name)
	}
	return err
}

func (d *Docker) Pause() {
	// Indicate that the container will be paused
	atomic.StoreUint32(&d.pause, 1)

	// Connect to the
	ctx := context.Background()
	err := dCli.ContainerPause(ctx, d.id)
	if err != nil {
		log.Print("Failed to start container ", d.name)
	}
}

// Unpause the container
func (d *Docker) Unpause() {
	ctx := context.Background()
	err := dCli.ContainerUnpause(ctx, d.id)
	if err != nil {
		log.Print("Failed to start container ", d.name)
	}

	atomic.StoreUint32(&d.pause, 0)
	close(d.awake)
}

// Block until the container is running
func (d *Docker) Await() {
	// We don't need to block anything if the container is
	// running.  Otherwise we await that the container is
	// unpaused, give it some time to catch up and then make sure
	// the container wasn't suspended in the meantime.

	for atomic.LoadUint32(&d.pause) != 0 {
		<-d.awake
		time.Sleep(50 * time.Millisecond)
	}
}
