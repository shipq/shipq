package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/channelcompile"
	"github.com/shipq/shipq/codegen/embed"
	"github.com/shipq/shipq/codegen/llmcompile"
	"github.com/shipq/shipq/codegen/llmgen"
	codegenMigrate "github.com/shipq/shipq/codegen/migrate"
	"github.com/shipq/shipq/config"
	portsqlcodegen "github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
	registrypkg "github.com/shipq/shipq/registry"
)

// LLMCompileCmd implements the "shipq llm compile" subcommand.
// It runs the full LLM compilation pipeline:
//
//  1. Load and parse shipq.ini
//  2. Parse [llm] section
//  3. Validate prerequisites (workers, Redis, Centrifugo, database)
//  4. Run static analysis + compile program to extract tool metadata
//  5. Run all code generators (tool registries, persister, migration, querydefs, stream types)
//  6. Compile queries (so the generated querydefs are usable)
//  7. Recompile handler registry (so new routes are picked up)
func LLMCompileCmd() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project roots", err)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdLLMCompile, roots.ShipqRoot) {
		os.Exit(1)
	}

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	// ── Step 1: Parse [llm] section ──────────────────────────────────

	llmCfg := config.ParseLLMConfig(ini)
	if llmCfg == nil {
		fmt.Fprintln(os.Stderr, "error: no [llm] section in shipq.ini")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Add an [llm] section to enable LLM support:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  [llm]")
		fmt.Fprintln(os.Stderr, "  tool_pkgs = myapp/tools/weather, myapp/tools/calendar")
		os.Exit(1)
	}

	// ── Step 2: Validate prerequisites ───────────────────────────────

	if err := llmcompile.ValidatePrerequisites(ini); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run `shipq workers` first, then add [llm] configuration to shipq.ini.")
		os.Exit(1)
	}

	// ── Step 3: Resolve module info ──────────────────────────────────

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		cli.FatalErr("failed to determine Go module info", err)
	}
	importPrefix := moduleInfo.FullImportPath("")

	scopeColumn := ini.Get("db", "scope")
	hasTenancy := scopeColumn != ""
	hasAuth := ini.Section("auth") != nil

	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
	}

	cli.Infof("Project: %s", importPrefix)
	cli.Infof("Tool packages: %d configured", len(llmCfg.ToolPkgs))

	// ── Step 3b: Embed LLM library packages ──────────────────────────
	//
	// The LLM core package, provider implementations (anthropic, openai),
	// and testing utilities must be present in the generated project before
	// the compile program can build (it imports shipq/lib/llm via tool packages).

	filesEnabled := ini.Section("files") != nil
	fmt.Println("")
	fmt.Println("Embedding LLM library packages...")
	if err := embed.EmbedAllPackages(roots.ShipqRoot, importPrefix, embed.EmbedOptions{
		FilesEnabled:   filesEnabled,
		WorkersEnabled: true, // LLM requires workers
		LLMEnabled:     true,
	}); err != nil {
		cli.FatalErr("failed to embed LLM library packages", err)
	}

	// ── Step 4: Run compile pipeline (static analysis + compile program) ─

	fmt.Println("")
	fmt.Println("Running LLM compile pipeline...")

	// Record whether LLM migration existed before we run generators.
	migrationExistedBefore := llmMigrationExists(filepath.Join(roots.ShipqRoot, "migrations"))

	var toolPackages []llmcompile.SerializedToolPackage

	if len(llmCfg.ToolPkgs) > 0 {
		compileCfg := llmcompile.CompileLLMConfig{
			ToolPkgs:   llmCfg.ToolPkgs,
			ModulePath: importPrefix,
			GoModRoot:  roots.GoModRoot,
			ShipqRoot:  roots.ShipqRoot,
			DBDialect:  dialect,
			HasTenancy: hasTenancy,
			HasAuth:    hasAuth,
			ModuleInfo: moduleInfo,
		}

		toolPackages, err = llmcompile.CompileLLM(compileCfg)
		if err != nil {
			cli.FatalErr("LLM compile pipeline failed", err)
		}

		totalTools := 0
		for _, pkg := range toolPackages {
			totalTools += len(pkg.Tools)
		}
		cli.Infof("  Found %d tool(s) across %d package(s)", totalTools, len(toolPackages))
	} else {
		fmt.Println("  No tool packages configured (persistence + streaming only)")
	}

	// ── Step 5: Discover channel packages (for stream type detection) ─

	var channelPkgs []string
	if ini.Section("workers") != nil {
		// Try to discover channel packages for stream type detection.
		// This is best-effort — if discovery fails we just skip stream type detection.
		channels, discoverErr := channelcompile.BuildAndRunChannelCompileProgram(roots.GoModRoot, roots.ShipqRoot, moduleInfo)
		if discoverErr == nil {
			for _, ch := range channels {
				if ch.PackagePath != "" {
					channelPkgs = append(channelPkgs, ch.PackagePath)
				}
			}
		}
	}

	// ── Step 6: Run all code generators ──────────────────────────────

	fmt.Println("")
	fmt.Println("Generating LLM code...")

	genCfg := llmgen.GenerateAllLLMConfig{
		ToolPackages: toolPackages,
		ModulePath:   importPrefix,
		GoModRoot:    roots.GoModRoot,
		ShipqRoot:    roots.ShipqRoot,
		DBDialect:    dialect,
		HasTenancy:   hasTenancy,
		HasAuth:      hasAuth,
		ChannelPkgs:  channelPkgs,
	}

	if err := llmgen.GenerateAllLLM(genCfg); err != nil {
		cli.FatalErr("LLM code generation failed", err)
	}

	// Report what was generated.
	for _, pkg := range toolPackages {
		if len(pkg.Tools) > 0 {
			fmt.Printf("  Generated %s/zz_generated_registry.go (%d tools)\n", pkg.PackageName, len(pkg.Tools))
		}
	}
	fmt.Println("  Generated shipq/lib/llmpersist/zz_generated_persister.go")
	fmt.Println("  Generated querydefs/llm/queries.go")

	if !migrationExistedBefore {
		fmt.Println("  Generated migrations/*_llm_tables.go")
	} else {
		fmt.Println("  LLM migration already exists (skipped)")
	}

	if len(channelPkgs) > 0 {
		fmt.Printf("  Wrote .shipq/llm_channels.json (%d channel(s) scanned)\n", len(channelPkgs))
	}

	// ── Step 7: Rebuild migration plan & regenerate schema package ────
	//
	// The LLM migration was just generated (step 6). The cached
	// schema.json is stale — it was written by a prior `migrate up` and
	// does not include the new llm_conversations / llm_messages tables.
	//
	// We rebuild the plan from the Go migration source files, rewrite
	// schema.json, and regenerate the schema Go package so that
	// `schema.LlmConversations` and `schema.LlmMessages` are visible
	// to the query compiler in step 8.

	fmt.Println("")
	fmt.Println("Rebuilding migration plan & schema package...")

	migrationsPath := filepath.Join(roots.ShipqRoot, "migrations")
	migrations, discoverErr := codegenMigrate.DiscoverMigrations(migrationsPath)
	if discoverErr != nil {
		cli.FatalErr("failed to discover migrations", discoverErr)
	}

	rawModulePath, rawErr := codegen.GetModulePath(roots.GoModRoot)
	if rawErr != nil {
		cli.FatalErr("failed to read raw module path", rawErr)
	}

	planJSON, buildErr := codegenMigrate.BuildMigrationPlan(
		roots.GoModRoot, rawModulePath, importPrefix, migrationsPath, migrations,
	)
	if buildErr != nil {
		cli.FatalErr("failed to rebuild migration plan", buildErr)
	}

	// Write the updated schema.json so future LoadMigrationPlan calls see the LLM tables.
	migratePkgPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate")
	if mkErr := codegen.EnsureDir(migratePkgPath); mkErr != nil {
		cli.FatalErr("failed to create migrate directory", mkErr)
	}
	schemaJSONPath := filepath.Join(migratePkgPath, "schema.json")
	if _, wErr := codegen.WriteFileIfChanged(schemaJSONPath, planJSON); wErr != nil {
		cli.FatalErr("failed to write schema.json", wErr)
	}

	plan, parseErr := migrate.PlanFromJSON(planJSON)
	if parseErr != nil {
		cli.FatalErr("failed to parse rebuilt migration plan", parseErr)
	}

	if err := regenerateSchemaPackage(roots.ShipqRoot, importPrefix, plan); err != nil {
		cli.FatalErr("failed to regenerate schema package", err)
	}
	fmt.Println("  Rebuilt schema.json and shipq/db/schema/schema.go")

	// ── Step 8: Compile queries ──────────────────────────────────────

	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// ── Step 9: Compile handler registry ─────────────────────────────

	fmt.Println("")
	fmt.Println("Compiling handler registry...")

	if err := registrypkg.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		cli.FatalErr("failed to compile registry", err)
	}
	fmt.Println("  Handler registry compiled")

	// ── Done ─────────────────────────────────────────────────────────

	fmt.Println("")
	cli.Success("LLM compile completed successfully!")
}

// llmMigrationExists checks whether an LLM migration file already exists
// in the migrations directory.
func llmMigrationExists(migrationsDir string) bool {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_llm_tables.go") {
			return true
		}
	}

	return false
}

// regenerateSchemaPackage regenerates the shipq/db/schema package from the
// current migration plan. This is needed after generating a new migration
// (e.g. the LLM tables migration) so that the schema Go types are available
// for the query compiler.
func regenerateSchemaPackage(shipqRoot, modulePath string, plan *migrate.MigrationPlan) error {
	schemaDir := filepath.Join(shipqRoot, "shipq", "db", "schema")
	if err := codegen.EnsureDir(schemaDir); err != nil {
		return fmt.Errorf("failed to create schema directory: %w", err)
	}

	queryPkgPath := modulePath + "/shipq/lib/db/portsql/query"
	schemaCode, err := portsqlcodegen.GenerateSchemaPackage(plan, queryPkgPath)
	if err != nil {
		return fmt.Errorf("failed to generate schema package: %w", err)
	}

	schemaPath := filepath.Join(schemaDir, "schema.go")
	if _, err := codegen.WriteFileIfChanged(schemaPath, schemaCode); err != nil {
		return fmt.Errorf("failed to write schema.go: %w", err)
	}

	return nil
}
