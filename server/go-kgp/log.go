// Shared logging
//
// Copyright (c) 2023  Philip Kaludercic
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

package kgp

import (
	"io"
	"log"
)

var Debug = log.New(io.Discard, "[debug] ", log.Ltime|log.Lshortfile|log.Lmicroseconds)
