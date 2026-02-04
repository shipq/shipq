package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	dbcodegen "github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/registry"
)

func createBaseHandlers() error {
	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Step 1: Run migrations
	fmt.Println("Running migrations...")
	migrateUpCmd()

	// Step 2: Regenerate handlers for all tables
	fmt.Println("")
	fmt.Println("Regenerating handlers...")

	modulePath, err := codegen.GetModulePath(projectRoot)
	if err != nil {
		return fmt.Errorf("%w\nMake sure you're in a Go project with a go.mod file.", err)
	}

	// Load the migration plan (schema.json)
	schemaPath := filepath.Join(projectRoot, "shipq", "migrate", "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema.json: %w\nMake sure migrations have been run.", err)
	}

	plan, err := migrate.PlanFromJSON(schemaData)
	if err != nil {
		return fmt.Errorf("failed to parse schema.json: %w", err)
	}

	// Load CRUD config for scope settings
	tableNames := codegen.SortedTableNames(plan.Schema.Tables)

	crudCfg, err := codegen.LoadCRUDConfigWithTables(projectRoot, tableNames, plan.Schema.Tables)
	if err != nil {
		// If config doesn't exist, use defaults
		crudCfg = &codegen.CRUDConfig{
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

		// Check for opt-out marker file
		optOutPath := filepath.Join(projectRoot, "api", tableName, ".shipq-no-regen")
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
		cfg := codegen.HandlerGenConfig{
			ModulePath:  modulePath,
			TableName:   tableName,
			Table:       table,
			Schema:      plan.Schema.Tables,
			ScopeColumn: scopeColumn,
		}

		files, err := codegen.GenerateHandlerFiles(cfg)
		if err != nil {
			return fmt.Errorf("failed to generate handlers for %s: %w", tableName, err)
		}

		// Create the api/<table> directory
		apiDir := filepath.Join(projectRoot, "api", tableName)
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

	// Compile the registry
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(projectRoot); err != nil {
		return fmt.Errorf("failed to compile registry: %w", err)
	}

	return nil
}

func resourceUpCmd() {
	if err := createBaseHandlers(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
