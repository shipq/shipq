package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	dbcodegen "github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

func handlerGenerateCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: 'shipq handler generate' requires a table name")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: shipq handler generate <table_name>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  shipq handler generate posts")
		fmt.Fprintln(os.Stderr, "  shipq handler generate users")
		os.Exit(1)
	}

	tableName := args[0]

	// Find project roots (supports monorepo setup)
	roots, err := project.FindProjectRoots()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to find project: %v\n", err)
		os.Exit(1)
	}

	modulePath, err := codegen.GetModulePath(roots.GoModRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you're in a Go project with a go.mod file.")
		os.Exit(1)
	}

	// Load the migration plan (schema.json) from shipq root
	schemaPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate", "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to read schema.json: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you've run 'shipq migrate up' first.")
		os.Exit(1)
	}

	plan, err := migrate.PlanFromJSON(schemaData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse schema.json: %v\n", err)
		os.Exit(1)
	}

	// Find the table
	table, ok := plan.Schema.Tables[tableName]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: table %q not found in schema\n", tableName)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Available tables:")
		for name := range plan.Schema.Tables {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
		os.Exit(1)
	}

	// Load CRUD config for scope settings (from shipq root)
	tableNames := make([]string, 0, len(plan.Schema.Tables))
	for name := range plan.Schema.Tables {
		tableNames = append(tableNames, name)
	}

	crudCfg, err := codegen.LoadCRUDConfigWithTables(roots.ShipqRoot, tableNames, plan.Schema.Tables)
	if err != nil {
		// If config doesn't exist, use defaults
		crudCfg = &codegen.CRUDConfig{
			TableOpts: make(map[string]dbcodegen.CRUDOptions),
		}
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
		fmt.Fprintf(os.Stderr, "error: failed to generate handlers: %v\n", err)
		os.Exit(1)
	}

	// Create the api/<table> directory (in shipq root)
	apiDir := filepath.Join(roots.ShipqRoot, "api", tableName)
	if err := codegen.EnsureDir(apiDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create directory %s: %v\n", apiDir, err)
		os.Exit(1)
	}

	// Write handler files
	for filename, content := range files {
		filePath := filepath.Join(apiDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if changed {
			fmt.Printf("Generated: %s\n", filePath)
		} else {
			fmt.Printf("Unchanged: %s\n", filePath)
		}
	}

	fmt.Println("")
	fmt.Printf("Handler files for %q generated in api/%s/\n", tableName, tableName)

	// Compile the registry (in shipq root)
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to compile registry: %v\n", err)
		// Don't exit - handler generation succeeded
	}
}
