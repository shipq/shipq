package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// GeneratePackagesOptions holds options for package generation.
type GeneratePackagesOptions struct {
	// SkipQueries skips generating the queries package
	SkipQueries bool
	// SkipRunner skips generating the db/generated package
	SkipRunner bool
	// Stdout is where informational messages are written
	Stdout io.Writer
	// Stderr is where warning/error messages are written
	Stderr io.Writer
}

// GeneratePackages generates the queries package and db/generated package.
// This is called automatically after migrate up and db setup to ensure
// the generated packages are always up to date.
//
// It will:
// 1. Generate the queries package (types.go, dialect runners) if schema exists
// 2. Generate the db/generated package (runner.go, schema.json, db.go) if schema exists
//
// Errors are logged but don't cause the function to fail - the caller
// (migrate up, setup) should still succeed even if package generation fails.
func GeneratePackages(ctx context.Context, config *Config, opts GeneratePackagesOptions) {
	stdout := opts.Stdout
	stderr := opts.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	// Check if schema.json exists
	schemaPath := filepath.Join(config.Paths.Migrations, "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		// No schema yet, print note and skip generation
		fmt.Fprintf(stderr, "\nNote: schema.json not found at %s\n", schemaPath)
		fmt.Fprintf(stderr, "  Run 'shipq db migrate up' first to generate the schema,\n")
		fmt.Fprintf(stderr, "  then run 'shipq db setup' again to generate the packages.\n")
		return
	}

	// Generate queries package
	if !opts.SkipQueries {
		if err := generateQueriesPackage(ctx, config, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "\nWarning: could not generate queries package: %v\n", err)
			fmt.Fprintf(stderr, "  You can generate it later by running 'shipq db compile'\n")
		}
	}

	// Generate db/generated package (runner package)
	if !opts.SkipRunner {
		if err := generateRunnerPackage(config, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "\nWarning: could not generate runner package: %v\n", err)
			fmt.Fprintf(stderr, "  You can generate it later by running 'shipq db setup'\n")
		}
	}
}

// generateQueriesPackage generates the queries package with types and dialect runners.
// This is a simplified version of Compile that focuses on CRUD generation.
func generateQueriesPackage(ctx context.Context, config *Config, stdout, stderr io.Writer) error {
	// Load schema
	plan, err := loadSchema(config.Paths.Migrations)
	if err != nil {
		return err
	}

	// Get dialects to generate
	dialects := config.Database.GetDialects()
	if len(dialects) == 0 {
		// Default to sqlite if no dialects configured
		dialects = []string{"sqlite"}
	}

	// Get tables that qualify for CRUD (created with AddTable)
	crudTables := migrate.GetCRUDTables(plan)
	if len(crudTables) == 0 {
		// No CRUD tables, nothing to generate
		return nil
	}

	// Build table options with scope and order configuration
	tableOpts := make(map[string]codegen.CRUDOptions)
	for _, table := range crudTables {
		scope := config.CRUD.GetScopeForTable(table.Name)
		order := config.CRUD.GetOrderForTable(table.Name)
		tableOpts[table.Name] = codegen.CRUDOptions{
			ScopeColumn: scope,
			OrderAsc:    order == "asc",
		}
	}

	// Create a filtered plan with only CRUD-eligible tables
	crudPlan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}
	for _, table := range crudTables {
		crudPlan.Schema.Tables[table.Name] = table
	}

	// Generate queries output directory
	queriesOut := config.Paths.QueriesOut
	if queriesOut == "" {
		queriesOut = "queries"
	}
	if err := os.MkdirAll(queriesOut, 0755); err != nil {
		return fmt.Errorf("failed to create queries output directory: %w", err)
	}

	// Generate shared types (types.go)
	typesCode, err := codegen.GenerateSharedTypes(nil, crudPlan, "queries", tableOpts)
	if err != nil {
		return fmt.Errorf("failed to generate shared types: %w", err)
	}

	typesPath := filepath.Join(queriesOut, "types.go")
	if err := os.WriteFile(typesPath, typesCode, 0644); err != nil {
		return fmt.Errorf("failed to write types.go: %w", err)
	}
	fmt.Fprintf(stdout, "Generated: %s\n", typesPath)

	// Get module path for types import
	modulePath, moduleRoot, err := getModulePath()
	if err != nil {
		return fmt.Errorf("failed to get module path: %w", err)
	}

	// Get the relative path of queries output from the module root
	absQueriesOut, err := filepath.Abs(queriesOut)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	relQueriesOut, err := filepath.Rel(moduleRoot, absQueriesOut)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}
	typesImportPath := modulePath + "/" + filepath.ToSlash(relQueriesOut)

	// Generate dialect-specific runners
	for _, dialect := range dialects {
		// Create dialect subdirectory
		dialectPath := filepath.Join(queriesOut, dialect)
		if err := os.MkdirAll(dialectPath, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dialect, err)
		}

		// Generate dialect-specific runner
		runnerCode, err := codegen.GenerateDialectRunner(
			nil, // No user-defined queries
			crudPlan,
			dialect,
			typesImportPath,
			tableOpts,
		)
		if err != nil {
			return fmt.Errorf("failed to generate %s runner: %w", dialect, err)
		}

		runnerPath := filepath.Join(dialectPath, "runner.go")
		if err := os.WriteFile(runnerPath, runnerCode, 0644); err != nil {
			return fmt.Errorf("failed to write %s/runner.go: %w", dialect, err)
		}
		fmt.Fprintf(stdout, "Generated: %s\n", runnerPath)
	}

	return nil
}
