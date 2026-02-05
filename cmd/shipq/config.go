package main

import (
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

const (
	// DefaultMigrationsDir is the default directory for migration files.
	DefaultMigrationsDir = "migrations"
)

// ProjectConfig holds the loaded project configuration.
// In a monorepo setup, GoModRoot and ShipqRoot may be different directories.
type ProjectConfig struct {
	GoModRoot      string // Directory containing go.mod
	ShipqRoot      string // Directory containing shipq.ini
	ModulePath     string // Module path from go.mod
	MigrationsPath string // Absolute path to migrations directory
	// ProjectRoot is kept for backward compatibility (same as ShipqRoot)
	ProjectRoot string
}

// LoadProjectConfig finds both project roots and loads the configuration from shipq.ini.
// In a monorepo, the go.mod may be in a parent directory of shipq.ini.
// Returns the project config or an error if not in a shipq project.
func LoadProjectConfig() (*ProjectConfig, error) {
	// Find both roots (shipq.ini may be in a subdirectory of go.mod)
	roots, err := project.FindProjectRoots()
	if err != nil {
		return nil, err
	}

	// Get module path from go.mod
	modulePath, err := codegen.GetModulePath(roots.GoModRoot)
	if err != nil {
		return nil, err
	}

	// Parse shipq.ini
	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return nil, err
	}

	// Get migrations path from [db] section (default: "migrations")
	// Migrations are relative to shipq root, not go.mod root
	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	// Build absolute path (relative to shipq root)
	migrationsPath := filepath.Join(roots.ShipqRoot, migrationsDir)

	return &ProjectConfig{
		GoModRoot:      roots.GoModRoot,
		ShipqRoot:      roots.ShipqRoot,
		ModulePath:     modulePath,
		MigrationsPath: migrationsPath,
		ProjectRoot:    roots.ShipqRoot, // Backward compatibility
	}, nil
}

// LoadMigrationsPath is a convenience function that returns just the migrations path.
// Returns the absolute path to the migrations directory.
func LoadMigrationsPath() (string, error) {
	cfg, err := LoadProjectConfig()
	if err != nil {
		return "", err
	}
	return cfg.MigrationsPath, nil
}
