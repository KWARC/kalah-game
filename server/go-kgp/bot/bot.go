// Bot Interface
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
// License, version 3, along with go-kgp . If not, see
// <http://www.gnu.org/licenses/>

package bot

import "go-kgp"

type Bot interface {
	kgp.Agent
	IsBot()
}

func IsBot(a kgp.Agent) bool {
	_, ok := a.(Bot)
	return ok
}
