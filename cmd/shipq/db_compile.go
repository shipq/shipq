package main

import (
	"os"
	"path/filepath"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	portsqlcodegen "github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/query"
)

// dbCompileCmd implements the "shipq db compile" command.
// It generates type-safe query runner code from user-defined queries.
func dbCompileCmd() {
	cwd, err := os.Getwd()
	if err != nil {
		cli.FatalErr("failed to get current directory", err)
	}

	// 1. Load project configuration
	cfg, err := codegen.LoadDBPackageConfig(cwd)
	if err != nil {
		cli.FatalErr("failed to load project config", err)
	}

	cli.Infof("Compiling queries for %s dialect...", cfg.Dialect)

	// 2. Discover querydefs packages
	pkgs, err := codegen.DiscoverQuerydefsPackages(cwd, cfg.ModulePath)
	if err != nil {
		cli.FatalErr("failed to discover querydefs packages", err)
	}

	if len(pkgs) == 0 {
		cli.Warn("No querydefs packages found. Only CRUD operations will be generated.")
	} else {
		cli.Infof("Found %d querydefs package(s)", len(pkgs))
	}

	// 3. Generate and write compile program
	programCfg := codegen.CompileProgramConfig{
		ModulePath:    cfg.ModulePath,
		QuerydefsPkgs: pkgs,
	}

	if err := codegen.WriteCompileProgram(cwd, programCfg); err != nil {
		cli.FatalErr("failed to write compile program", err)
	}

	// 4. Build and run compile program to extract query definitions
	var userQueries []query.SerializedQuery
	if len(pkgs) > 0 {
		queries, err := codegen.RunCompileProgram(cwd)
		if err != nil {
			cli.FatalErr("failed to extract queries", err)
		}
		userQueries = queries
		cli.Infof("Found %d user-defined query(ies)", len(userQueries))
	}

	// 5. Load schema for CRUD generation
	plan, err := codegen.LoadMigrationPlan(cwd)
	if err != nil {
		cli.Warn("Could not load schema: " + err.Error())
		cli.Warn("CRUD operations will not be generated.")
		plan = nil
	}

	// 5.5. Apply scope filtering based on actual table schemas
	// This ensures scope is only applied to tables that have the scope column
	tableOpts := cfg.GetTableOpts()
	if plan != nil && cfg.CRUDConfig != nil {
		// Build a map of table names to tables for filtering
		tables := make(map[string]ddl.Table)
		for name, table := range plan.Schema.Tables {
			tables[name] = table
		}

		// Reload config with actual table names
		tableNames := make([]string, 0, len(tables))
		for name := range tables {
			tableNames = append(tableNames, name)
		}

		// Re-load CRUD config with actual tables and apply filtering
		if updatedCfg, err := codegen.LoadCRUDConfigWithTables(cfg.ProjectRoot, tableNames, tables); err == nil {
			tableOpts = updatedCfg.TableOpts
		}
	}

	// 5.6. Warn about tables lacking cursor pagination support
	if plan != nil {
		cursorWarnings := portsqlcodegen.CheckAllTablesCursorSupport(plan)
		for _, w := range cursorWarnings {
			cli.Warnf("Table %q lacks %s - using offset pagination (cursor pagination requires both)",
				w.TableName, joinMissing(w.MissingColumns))
		}
	}

	// 6. Create output directories
	queriesDir := filepath.Join(cwd, "shipq", "queries")
	if err := codegen.EnsureDir(queriesDir); err != nil {
		cli.FatalErr("failed to create queries directory", err)
	}

	dialectDir := filepath.Join(queriesDir, cfg.Dialect)
	if err := codegen.EnsureDir(dialectDir); err != nil {
		cli.FatalErr("failed to create dialect directory", err)
	}

	// 7. Generate and write types.go
	runnerCfg := codegen.UnifiedRunnerConfig{
		ModulePath:  cfg.ModulePath,
		Dialect:     cfg.Dialect,
		UserQueries: userQueries,
		Schema:      plan,
		TableOpts:   tableOpts,
	}

	typesCode, err := codegen.GenerateSharedTypes(runnerCfg)
	if err != nil {
		cli.FatalErr("failed to generate types.go", err)
	}

	typesPath := filepath.Join(queriesDir, "types.go")
	written, err := codegen.WriteFileIfChanged(typesPath, typesCode)
	if err != nil {
		cli.FatalErr("failed to write types.go", err)
	}
	if written {
		cli.Info("  Generated shipq/queries/types.go")
	}

	// 8. Generate and write runner.go
	runnerCode, err := codegen.GenerateUnifiedRunner(runnerCfg)
	if err != nil {
		cli.FatalErr("failed to generate runner.go", err)
	}

	runnerPath := filepath.Join(dialectDir, "runner.go")
	written, err = codegen.WriteFileIfChanged(runnerPath, runnerCode)
	if err != nil {
		cli.FatalErr("failed to write runner.go", err)
	}
	if written {
		cli.Infof("  Generated shipq/queries/%s/runner.go", cfg.Dialect)
	}

	// 9. Clean up compile artifacts
	if err := codegen.CleanCompileArtifacts(cwd); err != nil {
		cli.Warn("Failed to clean compile artifacts: " + err.Error())
	}

	// Report success
	queryCount := len(userQueries)
	tableCount := 0
	if plan != nil {
		tableCount = len(plan.Schema.Tables)
	}

	cli.Success("Query compilation complete")
	cli.Infof("  User queries: %d", queryCount)
	cli.Infof("  CRUD tables: %d", tableCount)
}

// joinMissing joins missing column names with " and ".
func joinMissing(columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	if len(columns) == 1 {
		return columns[0]
	}
	return columns[0] + " and " + columns[1]
}
