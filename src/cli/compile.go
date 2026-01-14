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

	// Get dialects to generate
	dialects := config.Database.GetDialects()
	if len(dialects) == 0 {
		return fmt.Errorf("no dialects configured (set dialects in portsql.ini or check database URL)")
	}

	fmt.Printf("Generating for dialects: %v\n", dialects)

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

		// Compile for all dialects - capture paramOrder for each
		postgresSQL, postgresParamOrder, err := compileQuery(rq.AST, "postgres")
		if err != nil {
			return fmt.Errorf("failed to compile query %s for postgres: %w", name, err)
		}
		mysqlSQL, mysqlParamOrder, err := compileQuery(rq.AST, "mysql")
		if err != nil {
			return fmt.Errorf("failed to compile query %s for mysql: %w", name, err)
		}
		sqliteSQL, sqliteParamOrder, err := compileQuery(rq.AST, "sqlite")
		if err != nil {
			return fmt.Errorf("failed to compile query %s for sqlite: %w", name, err)
		}

		cq := codegen.CompiledQuery{
			Name:       name,
			SQL:        postgresSQL, // Use Postgres as default for shared types (doesn't matter for types)
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
			ParamOrder: codegen.DialectParamOrder{
				Postgres: postgresParamOrder,
				MySQL:    mysqlParamOrder,
				SQLite:   sqliteParamOrder,
			},
		})
	}

	// Generate queries output directory
	if err := os.MkdirAll(config.Paths.QueriesOut, 0755); err != nil {
		return fmt.Errorf("failed to create queries output directory: %w", err)
	}

	// --- Load schema for CRUD generation ---
	var crudPlan *migrate.MigrationPlan
	tableOpts := make(map[string]codegen.CRUDOptions)

	plan, err := loadSchema(config.Paths.Migrations)
	if err == nil {
		// Get tables that qualify for CRUD (created with AddTable)
		crudTables := getCRUDTables(plan)
		if len(crudTables) > 0 {
			fmt.Printf("\nFound %d CRUD tables:\n", len(crudTables))

			// Build table options with scope configuration
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
			crudPlan = &migrate.MigrationPlan{
				Schema: migrate.Schema{
					Tables: make(map[string]ddl.Table),
				},
			}
			for _, table := range crudTables {
				crudPlan.Schema.Tables[table.Name] = table
			}
		}
	}

	// --- Generate shared types (types.go) ---
	typesCode, err := codegen.GenerateSharedTypes(compiledQueries, crudPlan, "queries", tableOpts)
	if err != nil {
		return fmt.Errorf("failed to generate shared types: %w", err)
	}

	typesPath := filepath.Join(config.Paths.QueriesOut, "types.go")
	if err := os.WriteFile(typesPath, typesCode, 0644); err != nil {
		return fmt.Errorf("failed to write types.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", typesPath)

	// --- Remove old files from previous architecture ---
	os.Remove(filepath.Join(config.Paths.QueriesOut, "queries.go"))
	os.Remove(filepath.Join(config.Paths.QueriesOut, "runner.go"))
	os.Remove(filepath.Join(config.Paths.QueriesOut, "crud_types.go"))
	os.Remove(filepath.Join(config.Paths.QueriesOut, "crud_runner.go"))

	// --- Generate dialect-specific runners ---

	// Get module path for types import
	modulePath, err := getModulePath()
	if err != nil {
		return fmt.Errorf("failed to get module path: %w", err)
	}

	// Get the relative path of queries output from the module root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	absQueriesOut, err := filepath.Abs(config.Paths.QueriesOut)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	relQueriesOut, err := filepath.Rel(cwd, absQueriesOut)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}
	typesImportPath := modulePath + "/" + filepath.ToSlash(relQueriesOut)

	for _, dialect := range dialects {
		// Create dialect subdirectory
		dialectPath := filepath.Join(config.Paths.QueriesOut, dialect)
		if err := os.MkdirAll(dialectPath, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dialect, err)
		}

		// Generate dialect-specific runner
		runnerCode, err := codegen.GenerateDialectRunner(
			compiledWithDialects,
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
		fmt.Printf("Generated: %s\n", runnerPath)
	}

	fmt.Printf("\nSuccessfully compiled %d queries for %d dialect(s).\n", len(compiledQueries), len(dialects))
	if crudPlan != nil {
		fmt.Printf("Generated CRUD for %d tables.\n", len(crudPlan.Schema.Tables))
	}

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
		return compile.NewCompiler(compile.Postgres).Compile(ast)
	case "mysql":
		return compile.NewCompiler(compile.MySQL).Compile(ast)
	case "sqlite":
		return compile.NewCompiler(compile.SQLite).Compile(ast)
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
