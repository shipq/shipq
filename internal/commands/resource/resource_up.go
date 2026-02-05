package resource

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/crud"
	"github.com/shipq/shipq/codegen/handlergen"
	dbcodegen "github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

func createBaseHandlers() error {
	// Find project roots (supports monorepo setup)
	roots, err := project.FindProjectRoots()
	if err != nil {
		return fmt.Errorf("failed to find project: %w", err)
	}

	// Step 1: Run migrations
	fmt.Println("Running migrations...")
	up.MigrateUpCmd()

	// Step 2: Regenerate handlers for all tables
	fmt.Println("")
	fmt.Println("Regenerating handlers...")

	modulePath, err := codegen.GetModulePath(roots.GoModRoot)
	if err != nil {
		return fmt.Errorf("%w\nMake sure you're in a Go project with a go.mod file.", err)
	}

	// Load the migration plan (schema.json) from shipq root
	schemaPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate", "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema.json: %w\nMake sure migrations have been run.", err)
	}

	plan, err := migrate.PlanFromJSON(schemaData)
	if err != nil {
		return fmt.Errorf("failed to parse schema.json: %w", err)
	}

	// Load CRUD config for scope settings (from shipq root)
	tableNames := handlergen.SortedTableNames(plan.Schema.Tables)

	crudCfg, err := crud.LoadCRUDConfigWithTables(roots.ShipqRoot, tableNames, plan.Schema.Tables)
	if err != nil {
		// If config doesn't exist, use defaults
		crudCfg = &crud.CRUDConfig{
			TableOpts: make(map[string]dbcodegen.CRUDOptions),
		}
	}

	// Generate handlers for each table
	generatedCount := 0
	skippedCount := 0

	for _, tableName := range tableNames {
		table := plan.Schema.Tables[tableName]

		// Skip junction tables - they don't need handlers
		if table.IsJunctionTable {
			fmt.Printf("Skipping %s (junction table)\n", tableName)
			skippedCount++
			continue
		}

		// Check for opt-out marker file (in shipq root)
		optOutPath := filepath.Join(roots.ShipqRoot, "api", tableName, ".shipq-no-regen")
		if _, err := os.Stat(optOutPath); err == nil {
			fmt.Printf("Skipping %s (opted out via .shipq-no-regen)\n", tableName)
			skippedCount++
			continue
		}

		// Get scope column for this table
		scopeColumn := ""
		if opts, ok := crudCfg.TableOpts[tableName]; ok {
			scopeColumn = opts.ScopeColumn
		}

		// Generate handler files
		cfg := handlergen.HandlerGenConfig{
			ModulePath:  modulePath,
			TableName:   tableName,
			Table:       table,
			Schema:      plan.Schema.Tables,
			ScopeColumn: scopeColumn,
		}

		files, err := handlergen.GenerateHandlerFiles(cfg)
		if err != nil {
			return fmt.Errorf("failed to generate handlers for %s: %w", tableName, err)
		}

		// Create the api/<table> directory (in shipq root)
		apiDir := filepath.Join(roots.ShipqRoot, "api", tableName)
		if err := codegen.EnsureDir(apiDir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", apiDir, err)
		}

		// Write handler files
		changedFiles := 0
		for filename, content := range files {
			filePath := filepath.Join(apiDir, filename)
			changed, err := codegen.WriteFileIfChanged(filePath, content)
			if err != nil {
				return fmt.Errorf("failed to write %s: %w", filePath, err)
			}
			if changed {
				changedFiles++
			}
		}

		// Write .shipq-no-regen marker file to prevent future regeneration
		markerPath := filepath.Join(apiDir, ".shipq-no-regen")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			markerContent := "# This file prevents shipq from regenerating handlers in this directory.\n# Delete this file if you want shipq to regenerate the handlers.\n"
			if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", markerPath, err)
			}
		}

		if changedFiles > 0 {
			fmt.Printf("Generated %s (%d files updated)\n", tableName, changedFiles)
		} else {
			fmt.Printf("Generated %s (no changes)\n", tableName)
		}
		generatedCount++
	}

	// Summary
	fmt.Println("")
	fmt.Printf("Done! Generated handlers for %d tables", generatedCount)
	if skippedCount > 0 {
		fmt.Printf(", skipped %d", skippedCount)
	}
	fmt.Println(".")

	// Compile the registry (in shipq root)
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		return fmt.Errorf("failed to compile registry: %w", err)
	}

	return nil
}

func ResourceUpCmd() {
	if err := createBaseHandlers(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
