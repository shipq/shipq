// Package main provides the portsql-api-httpgen CLI entry point.
// This is a thin wrapper around the api/portapi/cli package.
package main

import (
	"os"

	"github.com/shipq/shipq/api/portapi/cli"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	code := cli.Run(os.Args[1:], cli.Options{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: Version,
	})
	os.Exit(code)
}
