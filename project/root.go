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
	ErrNotInProject    = errors.New("not in a Go project (no go.mod found)")
	ErrNoGoMod         = errors.New("go.mod not found in project root")
	ErrNoShipqIni      = errors.New("shipq.ini not found in project root")
	ErrNotShipqProject = errors.New("not a shipq project (missing go.mod or shipq.ini)")
)

// FindProjectRoot walks up from the current working directory looking for go.mod.
// Returns the directory containing go.mod, or an error if not found.
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindProjectRootFrom(cwd)
}

// FindProjectRootFrom walks up from the given directory looking for go.mod.
// Returns the directory containing go.mod, or an error if not found.
func FindProjectRootFrom(startDir string) (string, error) {
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

// ValidateProjectRoot checks that the given path contains both go.mod and shipq.ini.
// Returns an error describing what's missing, or nil if both files exist.
func ValidateProjectRoot(path string) error {
	goModPath := filepath.Join(path, GoModFile)
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return ErrNoGoMod
	} else if err != nil {
		return err
	}

	shipqIniPath := filepath.Join(path, ShipqIniFile)
	if _, err := os.Stat(shipqIniPath); os.IsNotExist(err) {
		return ErrNoShipqIni
	} else if err != nil {
		return err
	}

	return nil
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
