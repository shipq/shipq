package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/inifile"
)

// migrateNewCmd handles "shipq migrate new <name> [columns...] [--global]"
func migrateNewCmd(args []string) {
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
	if err := ValidateMigrationName(migrationName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Load project config to get migrations path
	cfg, err := LoadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// Parse column specs
	columns, err := ParseColumnSpecs(columnArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Generate timestamp
	timestamp := GenerateTimestamp()

	// Load scope config from shipq.ini
	scopeColumn, scopeTable := loadScopeConfig(cfg)

	// Generate migration code
	migrationCfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: migrationName,
		Timestamp:     timestamp,
		Columns:       columns,
		ScopeColumn:   scopeColumn,
		ScopeTable:    scopeTable,
		IsGlobal:      isGlobal,
	}

	code, err := GenerateMigration(migrationCfg)
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
	fileName := GenerateMigrationFileName(timestamp, migrationName)
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
		table = codegen.InferScopeTable(column)
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
