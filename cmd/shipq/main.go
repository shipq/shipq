// Package main provides the shipq CLI entry point.
package main

import (
	"os"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	code := run(os.Args[1:])
	os.Exit(code)
}
