package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/portsql/portsql/src/codegen"
	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
	"github.com/portsql/portsql/src/query"
	"github.com/portsql/portsql/src/query/compile"
)

// Compile compiles query definitions into SQL strings and Go structs.
func Compile(ctx context.Context, config *Config) error {
	// Validate config
	if config.Database.URL == "" {
		return fmt.Errorf("database URL not configured (set DATABASE_URL or add to portsql.ini)")
	}

	// Parse dialect
	dialect := ParseDialect(config.Database.URL)
	if dialect == "" {
		return fmt.Errorf("unsupported database URL scheme: %s", config.Database.URL)
	}

	// Check if queries_in directory exists
	if _, err := os.Stat(config.Paths.QueriesIn); os.IsNotExist(err) {
		return fmt.Errorf("queries directory not found: %s\nCreate a package with query definitions using query.DefineOne/DefineMany/DefineExec()", config.Paths.QueriesIn)
	}

	// Generate and run temp program to extract registered queries
	queries, err := extractRegisteredQueries(ctx, config.Paths.QueriesIn)
	if err != nil {
		return fmt.Errorf("failed to extract queries: %w", err)
	}

	if len(queries) == 0 {
		return fmt.Errorf("no queries registered in %s\nUse query.DefineOne/DefineMany/DefineExec() in init() to register queries", config.Paths.QueriesIn)
	}

	fmt.Printf("Found %d registered queries\n", len(queries))

	// Compile each query for all dialects
	var compiledQueries []codegen.CompiledQuery
	var compiledWithDialects []codegen.CompiledQueryWithDialects
	for name, rq := range queries {
		fmt.Printf("  Compiling: %s (%s)\n", name, rq.ReturnType)

		// Compile for all dialects
		postgresSQL, _, err := compileQuery(rq.AST, "postgres")
		if err != nil {
			return fmt.Errorf("failed to compile query %s for postgres: %w", name, err)
		}
		mysqlSQL, _, err := compileQuery(rq.AST, "mysql")
		if err != nil {
			return fmt.Errorf("failed to compile query %s for mysql: %w", name, err)
		}
		sqliteSQL, _, err := compileQuery(rq.AST, "sqlite")
		if err != nil {
			return fmt.Errorf("failed to compile query %s for sqlite: %w", name, err)
		}

		// Get SQL for the current dialect (for backward-compatible queries.go)
		sql, _, err := compileQuery(rq.AST, dialect)
		if err != nil {
			return fmt.Errorf("failed to compile query %s: %w", name, err)
		}

		cq := codegen.CompiledQuery{
			Name:       name,
			SQL:        sql,
			Params:     codegen.ExtractParamInfo(rq.AST),
			Results:    codegen.ExtractResultInfo(rq.AST),
			ReturnType: string(rq.ReturnType),
		}
		compiledQueries = append(compiledQueries, cq)

		compiledWithDialects = append(compiledWithDialects, codegen.CompiledQueryWithDialects{
			CompiledQuery: cq,
			SQL: codegen.DialectSQL{
				Postgres: postgresSQL,
				MySQL:    mysqlSQL,
				SQLite:   sqliteSQL,
			},
		})
	}

	// Generate queries package
	if err := os.MkdirAll(config.Paths.QueriesOut, 0755); err != nil {
		return fmt.Errorf("failed to create queries output directory: %w", err)
	}

	// Generate types file (queries.go)
	code, err := codegen.GenerateQueriesPackage(compiledQueries, "queries")
	if err != nil {
		return fmt.Errorf("failed to generate queries package: %w", err)
	}

	outputPath := filepath.Join(config.Paths.QueriesOut, "queries.go")
	if err := os.WriteFile(outputPath, code, 0644); err != nil {
		return fmt.Errorf("failed to write queries.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", outputPath)

	// Generate runner file (runner.go)
	runnerCode, err := codegen.GenerateQueryRunner(compiledWithDialects, "queries")
	if err != nil {
		return fmt.Errorf("failed to generate query runner: %w", err)
	}

	runnerPath := filepath.Join(config.Paths.QueriesOut, "runner.go")
	if err := os.WriteFile(runnerPath, runnerCode, 0644); err != nil {
		return fmt.Errorf("failed to write runner.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", runnerPath)

	fmt.Printf("\nSuccessfully compiled %d queries.\n", len(compiledQueries))

	// --- CRUD Generation for AddTable tables ---

	// Load schema
	plan, err := loadSchema(config.Paths.Migrations)
	if err != nil {
		// Schema not found is not fatal - just skip CRUD generation
		fmt.Printf("\nNote: %v\nSkipping CRUD generation.\n", err)
		return nil
	}

	// Get tables that qualify for CRUD (created with AddTable)
	crudTables := getCRUDTables(plan)
	if len(crudTables) == 0 {
		fmt.Printf("\nNo tables found that qualify for CRUD generation.\n")
		fmt.Printf("Use plan.AddTable() to create tables with standard columns (id, public_id, created_at, updated_at, deleted_at).\n")
		return nil
	}

	fmt.Printf("\nGenerating CRUD for %d tables:\n", len(crudTables))

	// Build table options with scope configuration
	tableOpts := make(map[string]codegen.CRUDOptions)
	for _, table := range crudTables {
		scope := config.CRUD.GetScopeForTable(table.Name)
		opts := codegen.CRUDOptions{
			ScopeColumn: scope,
		}
		tableOpts[table.Name] = opts

		if scope != "" {
			fmt.Printf("  %s (scope: %s)\n", table.Name, scope)
		} else {
			fmt.Printf("  %s\n", table.Name)
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

	// Generate CRUD types
	crudTypesCode, err := codegen.GenerateCRUDPackageWithOptions(crudPlan, "queries", tableOpts)
	if err != nil {
		return fmt.Errorf("failed to generate CRUD types: %w", err)
	}

	crudTypesPath := filepath.Join(config.Paths.QueriesOut, "crud_types.go")
	if err := os.WriteFile(crudTypesPath, crudTypesCode, 0644); err != nil {
		return fmt.Errorf("failed to write crud_types.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", crudTypesPath)

	// Generate combined runner with both user-defined queries and CRUD methods
	combinedRunnerCode, err := codegen.GenerateCombinedRunner(compiledWithDialects, crudPlan, "queries", tableOpts)
	if err != nil {
		return fmt.Errorf("failed to generate combined runner: %w", err)
	}

	// Remove old runner.go if it exists (will be replaced by combined runner)
	os.Remove(filepath.Join(config.Paths.QueriesOut, "runner.go"))
	os.Remove(filepath.Join(config.Paths.QueriesOut, "crud_runner.go"))

	combinedRunnerPath := filepath.Join(config.Paths.QueriesOut, "runner.go")
	if err := os.WriteFile(combinedRunnerPath, combinedRunnerCode, 0644); err != nil {
		return fmt.Errorf("failed to write runner.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", combinedRunnerPath)

	fmt.Printf("\nSuccessfully generated CRUD for %d tables.\n", len(crudTables))

	return nil
}

// extractRegisteredQueries generates a temp program to extract queries from the registry.
func extractRegisteredQueries(ctx context.Context, queriesInPath string) (map[string]query.RegisteredQuery, error) {
	// Get the module path for imports
	modulePath, err := getModulePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get module path: %w", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "portsql-compile-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Get absolute path to queries directory
	absQueriesPath, err := filepath.Abs(queriesInPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Calculate the import path
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(cwd, absQueriesPath)
	if err != nil {
		return nil, err
	}

	// Convert to import path
	queriesImport := modulePath + "/" + filepath.ToSlash(relPath)

	// Generate the temp main.go
	mainGo := fmt.Sprintf(`package main

import (
	"encoding/json"
	"os"

	_ %q  // Import triggers DefineOne/DefineMany/DefineExec calls in init()
	"github.com/portsql/portsql/src/query"
)

func main() {
	queries := query.GetRegisteredQueries()
	if err := json.NewEncoder(os.Stdout).Encode(queries); err != nil {
		os.Stderr.WriteString("failed to encode queries: " + err.Error() + "\n")
		os.Exit(1)
	}
}
`, queriesImport)

	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainGo), 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp main.go: %w", err)
	}

	// Run go run
	cmd := exec.CommandContext(ctx, "go", "run", mainPath)
	cmd.Dir = cwd // Run from original directory for proper module resolution
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to extract queries: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run temp program: %w", err)
	}

	// Parse the JSON output
	var queries map[string]query.RegisteredQuery
	if err := json.Unmarshal(output, &queries); err != nil {
		return nil, fmt.Errorf("failed to parse queries output: %w", err)
	}

	return queries, nil
}

// compileQuery compiles an AST to SQL for the given dialect.
func compileQuery(ast *query.AST, dialect string) (string, []string, error) {
	switch dialect {
	case "postgres":
		return compile.CompilePostgres(ast)
	case "mysql":
		return compile.CompileMySQL(ast)
	case "sqlite":
		return compile.CompileSQLite(ast)
	default:
		return "", nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

// loadSchema loads the schema from migrations/schema.json.
func loadSchema(migrationsPath string) (*migrate.MigrationPlan, error) {
	schemaPath := filepath.Join(migrationsPath, "schema.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("schema.json not found at %s\nRun 'portsql migrate up' first to generate the schema", schemaPath)
		}
		return nil, fmt.Errorf("failed to read schema.json: %w", err)
	}

	var plan migrate.MigrationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse schema.json: %w", err)
	}

	return &plan, nil
}

// isAddTableTable returns true if the table was created with AddTable (has public_id AND deleted_at).
// Tables created with AddTable have the standard columns: id, public_id, created_at, updated_at, deleted_at.
func isAddTableTable(table ddl.Table) bool {
	hasPublicID := false
	hasDeletedAt := false

	for _, col := range table.Columns {
		switch col.Name {
		case "public_id":
			hasPublicID = true
		case "deleted_at":
			hasDeletedAt = true
		}
	}

	return hasPublicID && hasDeletedAt
}

// getCRUDTables returns tables that qualify for CRUD generation (created with AddTable).
func getCRUDTables(plan *migrate.MigrationPlan) []ddl.Table {
	var tables []ddl.Table
	for _, table := range plan.Schema.Tables {
		if isAddTableTable(table) {
			tables = append(tables, table)
		}
	}
	return tables
}
