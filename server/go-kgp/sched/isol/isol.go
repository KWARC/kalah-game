// General Isolation
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
	"fmt"

	"go-kgp"
	"go-kgp/cmd"
)

type ControlledAgent interface {
	kgp.Agent
	fmt.Stringer
	Start(*cmd.State, *cmd.Conf) (kgp.Agent, error)
	Shutdown() error
}

func Start(st *cmd.State, conf *cmd.Conf, a kgp.Agent) (kgp.Agent, error) {
	kgp.Debug.Println("Starting", a)
	if ca, ok := a.(ControlledAgent); ok {
		return ca.Start(st, conf)
	}
	return a, nil
}

func Shutdown(a kgp.Agent) error {
	kgp.Debug.Println("Shutting down", a)
	if ca, ok := a.(ControlledAgent); ok {
		return ca.Shutdown()
	}
	return nil
}
