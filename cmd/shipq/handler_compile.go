package main

import (
	"fmt"
	"os"

	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

func handlerCompileCmd() {
	// Find project roots (supports monorepo setup)
	roots, err := project.FindProjectRoots()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to find project: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
