// General Isolation
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

package isol

import (
	"go-kgp"
)

type ControlledAgent interface {
	kgp.Agent
	Start() (kgp.Agent, error)
	Shutdown() error
}

func Start(a kgp.Agent) (kgp.Agent, error) {
	if ca, ok := a.(ControlledAgent); ok {
		return ca.Start()
	}
	return a, nil
}

func Shutdown(a kgp.Agent) error {
	if ca, ok := a.(ControlledAgent); ok {
		return ca.Shutdown()
	}
	return nil
}
