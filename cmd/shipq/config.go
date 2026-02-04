package main

import (
	"path/filepath"

	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

const (
	// DefaultMigrationsDir is the default directory for migration files.
	DefaultMigrationsDir = "migrations"
)

// ProjectConfig holds the loaded project configuration.
type ProjectConfig struct {
	ProjectRoot    string
	MigrationsPath string
}

// LoadProjectConfig finds the project root and loads the configuration from shipq.ini.
// Returns the project config or an error if not in a shipq project.
func LoadProjectConfig() (*ProjectConfig, error) {
	// Find project root
	projectRoot, err := project.FindProjectRoot()
	if err != nil {
		return nil, err
	}

	// Validate that this is a shipq project
	if err := project.ValidateProjectRoot(projectRoot); err != nil {
		return nil, err
	}

	// Parse shipq.ini
	shipqIniPath := filepath.Join(projectRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return nil, err
	}

	// Get migrations path from [db] section (default: "migrations")
	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	// Build absolute path
	migrationsPath := filepath.Join(projectRoot, migrationsDir)

	return &ProjectConfig{
		ProjectRoot:    projectRoot,
		MigrationsPath: migrationsPath,
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
