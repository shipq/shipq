package new

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/crud"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/migrate/generator"
	"github.com/shipq/shipq/internal/commands/migrate/parser"
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

// loadProjectConfig finds both project roots and loads the configuration from shipq.ini.
// In a monorepo, the go.mod may be in a parent directory of shipq.ini.
// Returns the project config or an error if not in a shipq project.
func loadProjectConfig() (*ProjectConfig, error) {
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

// MigrateNewCmd handles "shipq migrate new <name> [columns...] [--global]"
func MigrateNewCmd(args []string) {
	// Require migration name
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: migration name required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: shipq migrate new <name> [columns...] [--global]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  shipq migrate new users")
		fmt.Fprintln(os.Stderr, "  shipq migrate new users name:string email:string")
		fmt.Fprintln(os.Stderr, "  shipq migrate new posts title:string user_id:references:users")
		fmt.Fprintln(os.Stderr, "  shipq migrate new accounts name:string --global  # Skip scope injection")
		os.Exit(1)
	}

	// Check for --global flag and extract it from args
	isGlobal := false
	var filteredArgs []string
	for _, arg := range args {
		if arg == "--global" {
			isGlobal = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	if len(filteredArgs) < 1 {
		fmt.Fprintln(os.Stderr, "error: migration name required")
		os.Exit(1)
	}

	migrationName := filteredArgs[0]
	columnArgs := filteredArgs[1:]

	// Validate migration name
	if err := parser.ValidateMigrationName(migrationName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Load project config to get migrations path
	cfg, err := loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// Parse column specs
	columns, err := parser.ParseColumnSpecs(columnArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Generate timestamp
	timestamp := generator.GenerateTimestamp()

	// Load scope config from shipq.ini
	scopeColumn, scopeTable := loadScopeConfig(cfg)

	// Generate migration code
	migrationCfg := generator.MigrationConfig{
		PackageName:   "migrations",
		MigrationName: migrationName,
		Timestamp:     timestamp,
		Columns:       columns,
		ScopeColumn:   scopeColumn,
		ScopeTable:    scopeTable,
		IsGlobal:      isGlobal,
	}

	code, err := generator.GenerateMigration(migrationCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate migration: %v\n", err)
		os.Exit(1)
	}

	// Create migrations directory if needed
	if err := os.MkdirAll(cfg.MigrationsPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create migrations directory: %v\n", err)
		os.Exit(1)
	}

	// Generate file name and path
	fileName := generator.GenerateMigrationFileName(timestamp, migrationName)
	filePath := filepath.Join(cfg.MigrationsPath, fileName)

	// Write migration file
	if err := os.WriteFile(filePath, code, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write migration file: %v\n", err)
		os.Exit(1)
	}

	// Calculate relative path from project root for display
	relPath, err := filepath.Rel(cfg.ProjectRoot, filePath)
	if err != nil {
		relPath = filePath
	}

	fmt.Printf("Created migration: %s\n", relPath)

	// Notify user if scope was injected
	if scopeColumn != "" && !isGlobal {
		fmt.Printf("  (auto-added %s scope column from shipq.ini)\n", scopeColumn)
	}
}

// loadScopeConfig loads the scope configuration from shipq.ini.
// Returns (column, table) where both are empty if no scope is configured.
func loadScopeConfig(cfg *ProjectConfig) (string, string) {
	shipqIniPath := filepath.Join(cfg.ProjectRoot, "shipq.ini")
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return "", ""
	}

	column := ini.Get("db", "scope")
	if column == "" {
		return "", ""
	}

	table := ini.Get("db", "scope_table")
	if table == "" {
		// Infer from column name: organization_id -> organizations
		table = crud.InferScopeTable(column)
	}

	return column, table
}

// inferScopeTable infers the referenced table name from a scope column name.
// This is a fallback if codegen.InferScopeTable is not available.
func inferScopeTable(column string) string {
	if strings.HasSuffix(column, "_id") {
		singular := strings.TrimSuffix(column, "_id")
		return singular + "s" // Simple pluralization
	}
	return column + "s"
}
