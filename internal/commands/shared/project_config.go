package shared

import (
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

const (
	// DefaultMigrationsDir is the default directory for migration files.
	DefaultMigrationsDir = "migrations"
)

// ProjectConfig holds the loaded project configuration.
// This is the shared superset used by auth, files, migrate/new, workers, etc.
type ProjectConfig struct {
	GoModRoot      string // Directory containing go.mod
	ShipqRoot      string // Directory containing shipq.ini
	ModulePath     string // Full import path from go.mod (e.g., "myapp")
	MigrationsPath string // Absolute path to migrations directory
	DatabaseURL    string // from shipq.ini [db] database_url
	Dialect        string // inferred from DatabaseURL ("postgres", "mysql", "sqlite")
	ScopeColumn    string // from shipq.ini [db] scope (e.g., "organization_id")
}

// LoadProjectConfig finds project roots and loads configuration from go.mod
// and shipq.ini. It populates all fields of ProjectConfig including database
// dialect inference.
func LoadProjectConfig() (*ProjectConfig, error) {
	roots, err := project.FindProjectRoots()
	if err != nil {
		return nil, err
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		return nil, err
	}
	modulePath := moduleInfo.FullImportPath("")

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return nil, err
	}

	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	migrationsPath := filepath.Join(roots.ShipqRoot, migrationsDir)

	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
	}

	scopeColumn := ini.Get("db", "scope")

	return &ProjectConfig{
		GoModRoot:      roots.GoModRoot,
		ShipqRoot:      roots.ShipqRoot,
		ModulePath:     modulePath,
		MigrationsPath: migrationsPath,
		DatabaseURL:    databaseURL,
		Dialect:        dialect,
		ScopeColumn:    scopeColumn,
	}, nil
}

// TestDatabaseURL derives the test database URL from the configured DatabaseURL.
// Returns an empty string if no DatabaseURL is configured or derivation fails.
func (c *ProjectConfig) TestDatabaseURL() string {
	if c.DatabaseURL == "" {
		return ""
	}
	u, err := dburl.TestDatabaseURL(c.DatabaseURL)
	if err != nil {
		return ""
	}
	return u
}
