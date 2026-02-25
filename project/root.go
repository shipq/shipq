package project

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	GoModFile    = "go.mod"
	ShipqIniFile = "shipq.ini"
)

var (
	ErrNotInProject = errors.New("not in a Go project (no go.mod found)")
	ErrNoShipqIni   = errors.New("shipq.ini not found")
)

// ProjectRoots holds both the Go module root and the shipq project root.
// In a monorepo setup, these may be different directories (shipq.ini in a subdirectory).
// In a standard setup, they are the same directory.
type ProjectRoots struct {
	GoModRoot string // Directory containing go.mod
	ShipqRoot string // Directory containing shipq.ini
}

// FindGoModRoot walks up from the current working directory looking for go.mod.
// Returns the directory containing go.mod, or an error if not found.
func FindGoModRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindGoModRootFrom(cwd)
}

// FindGoModRootFrom walks up from the given directory looking for go.mod.
// Returns the directory containing go.mod, or an error if not found.
func FindGoModRootFrom(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, GoModFile)
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", ErrNotInProject
		}
		dir = parent
	}
}

// FindShipqRoot walks up from the current working directory looking for shipq.ini.
// Returns the directory containing shipq.ini, or an error if not found.
func FindShipqRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindShipqRootFrom(cwd)
}

// FindShipqRootFrom walks up from the given directory looking for shipq.ini.
// Returns the directory containing shipq.ini, or an error if not found.
func FindShipqRootFrom(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		shipqIniPath := filepath.Join(dir, ShipqIniFile)
		if _, err := os.Stat(shipqIniPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", ErrNoShipqIni
		}
		dir = parent
	}
}

// FindProjectRoots finds both the Go module root and shipq project root from CWD.
// The shipq root must be at or below the Go module root (shipq.ini can be in a subdirectory).
// Returns an error if either root cannot be found.
func FindProjectRoots() (*ProjectRoots, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return FindProjectRootsFrom(cwd)
}

// FindProjectRootsFrom finds both the Go module root and shipq project root.
// The shipq root must be at or below the Go module root (shipq.ini can be in a subdirectory).
// Returns an error if either root cannot be found.
func FindProjectRootsFrom(startDir string) (*ProjectRoots, error) {
	// First find the shipq root (shipq.ini)
	shipqRoot, err := FindShipqRootFrom(startDir)
	if err != nil {
		return nil, err
	}

	// Then find the go.mod root, which may be at shipqRoot or an ancestor
	goModRoot, err := FindGoModRootFrom(shipqRoot)
	if err != nil {
		return nil, err
	}

	return &ProjectRoots{
		GoModRoot: goModRoot,
		ShipqRoot: shipqRoot,
	}, nil
}

// GetProjectName returns the folder name of the project root.
// This is used as the default database name and other identifiers.
func GetProjectName(projectRoot string) string {
	return filepath.Base(projectRoot)
}

// HasGoMod returns true if the given directory contains a go.mod file.
func HasGoMod(dir string) bool {
	goModPath := filepath.Join(dir, GoModFile)
	_, err := os.Stat(goModPath)
	return err == nil
}

// HasShipqIni returns true if the given directory contains a shipq.ini file.
func HasShipqIni(dir string) bool {
	shipqIniPath := filepath.Join(dir, ShipqIniFile)
	_, err := os.Stat(shipqIniPath)
	return err == nil
}
