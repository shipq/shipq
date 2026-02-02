// Package project provides utilities for finding and working with ShipQ project roots.
//
// A ShipQ project root is identified by the presence of a shipq.ini configuration file.
// This package provides functions to locate the project root from any subdirectory,
// enabling commands like `shipq db migrate up` to work when run from nested directories.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/internal/config"
)

// ErrNotFound is returned when no project root can be found.
var ErrNotFound = errors.New("shipq project not found")

// Root contains information about a located ShipQ project.
type Root struct {
	// Dir is the absolute path to the project root directory.
	Dir string

	// ConfigPath is the absolute path to the shipq.ini file.
	ConfigPath string
}

// FindRoot searches upward from startDir looking for a shipq.ini file.
// If startDir is empty, the current working directory is used.
//
// Returns the Root if found, or (nil, false, nil) if not found.
// Returns an error only for filesystem errors (not for "not found").
func FindRoot(startDir string) (*Root, bool, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, false, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Convert to absolute path
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, false, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	dir := absDir
	for {
		configPath := filepath.Join(dir, config.ConfigFilename)
		info, err := os.Stat(configPath)
		if err == nil && !info.IsDir() {
			// Found it
			return &Root{
				Dir:        dir,
				ConfigPath: configPath,
			}, true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			// Unexpected error (permission denied, etc.)
			return nil, false, fmt.Errorf("failed to check %s: %w", configPath, err)
		}

		// Move up to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return nil, false, nil
		}
		dir = parent
	}
}

// Resolve searches upward from startDir and returns the project root.
// Unlike FindRoot, this returns an error if no project root is found.
//
// If startDir is empty, the current working directory is used.
func Resolve(startDir string) (*Root, error) {
	root, found, err := FindRoot(startDir)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w: no %s found\n"+
			"  Run from your ShipQ project root or use --project to specify the path",
			ErrNotFound, config.ConfigFilename)
	}
	return root, nil
}

// ResolveWithOverride returns the project root, using the override path if provided.
// If override is non-empty, it validates that the path contains shipq.ini.
// If override is empty, it searches upward from the current directory.
func ResolveWithOverride(override string) (*Root, error) {
	if override != "" {
		// Validate the override path
		absDir, err := filepath.Abs(override)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve project path: %w", err)
		}

		info, err := os.Stat(absDir)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project path does not exist: %s", override)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to access project path: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("project path is not a directory: %s", override)
		}

		configPath := filepath.Join(absDir, config.ConfigFilename)
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found in %s\n"+
				"  Ensure you're pointing to a valid ShipQ project directory",
				config.ConfigFilename, override)
		} else if err != nil {
			return nil, fmt.Errorf("failed to access config file: %w", err)
		}

		return &Root{
			Dir:        absDir,
			ConfigPath: configPath,
		}, nil
	}

	return Resolve("")
}

// MustResolve is like Resolve but returns the current working directory
// as a fallback if no project root is found. This is useful for commands
// that can work without a project (like `init`).
//
// If startDir is empty, the current working directory is used.
func MustResolve(startDir string) (string, error) {
	root, found, err := FindRoot(startDir)
	if err != nil {
		return "", err
	}
	if found {
		return root.Dir, nil
	}

	// Fallback to current directory
	if startDir == "" {
		return os.Getwd()
	}
	return filepath.Abs(startDir)
}
