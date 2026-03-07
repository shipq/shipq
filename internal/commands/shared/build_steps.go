package shared

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/registry"
)

// GoModTidy runs `go mod tidy` in the given directory (typically GoModRoot).
// It prints progress and returns an error on failure instead of calling os.Exit.
func GoModTidy(goModRoot string) error {
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = goModRoot
	if tidyOut, tidyErr := tidyCmd.CombinedOutput(); tidyErr != nil {
		return fmt.Errorf("go mod tidy failed: %v\n%s", tidyErr, tidyOut)
	}
	fmt.Println("  go mod tidy done")
	return nil
}

// CompileAndBuildRegistry runs the standard three-step tail shared by most
// feature commands:
//  1. db.DBCompileCmd()  — recompile queries
//  2. GoModTidy          — resolve new imports
//  3. registry.Run       — regenerate api/server.go, test client, etc.
//
// Set runTidy to false if the caller has already run go mod tidy or wants to
// skip that step for speed.
func CompileAndBuildRegistry(shipqRoot, goModRoot string, runTidy bool) error {
	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	if runTidy {
		fmt.Println("")
		if err := GoModTidy(goModRoot); err != nil {
			return err
		}
	}

	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(shipqRoot, goModRoot); err != nil {
		return fmt.Errorf("failed to compile registry: %w", err)
	}

	return nil
}

// CompileAndBuildRegistryOrExit is a convenience wrapper around
// CompileAndBuildRegistry that prints the error and exits on failure.
func CompileAndBuildRegistryOrExit(shipqRoot, goModRoot string, runTidy bool) {
	if err := CompileAndBuildRegistry(shipqRoot, goModRoot, runTidy); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
