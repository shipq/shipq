package main

import (
	"fmt"
	"os"

	"github.com/shipq/shipq/registry"
)

func handlerCompileCmd() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Run(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
