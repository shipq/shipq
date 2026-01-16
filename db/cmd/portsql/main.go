// Package main provides the portsql CLI entry point.
package main

import (
	"os"

	"github.com/shipq/shipq/db/portsql/cli"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	code := cli.Run(os.Args[1:], Version)
	os.Exit(code)
}
