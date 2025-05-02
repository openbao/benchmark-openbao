// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/openbao/benchmark-openbao/command"
)

func init() {
	// This doesn't need to be in an init, just putting it here to call it out.
	rand.Seed(time.Now().UnixNano())
}

func main() {
	os.Exit(command.Run(os.Args[1:]))
}
