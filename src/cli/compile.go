package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/portsql/portsql/src/codegen"
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
		return fmt.Errorf("queries directory not found: %s\nCreate a package with query definitions using query.DefineQuery()", config.Paths.QueriesIn)
	}

	// Generate and run temp program to extract registered queries
	queries, err := extractRegisteredQueries(ctx, config.Paths.QueriesIn)
	if err != nil {
		return fmt.Errorf("failed to extract queries: %w", err)
	}

	if len(queries) == 0 {
		return fmt.Errorf("no queries registered in %s\nUse query.DefineQuery() in init() to register queries", config.Paths.QueriesIn)
	}

	fmt.Printf("Found %d registered queries\n", len(queries))

	// Compile each query
	var compiledQueries []codegen.CompiledQuery
	for name, ast := range queries {
		fmt.Printf("  Compiling: %s\n", name)

		sql, _, err := compileQuery(ast, dialect)
		if err != nil {
			return fmt.Errorf("failed to compile query %s: %w", name, err)
		}

		compiledQueries = append(compiledQueries, codegen.CompiledQuery{
			Name:    name,
			SQL:     sql,
			Params:  codegen.ExtractParamInfo(ast),
			Results: codegen.ExtractResultInfo(ast),
		})
	}

	// Generate queries package
	if err := os.MkdirAll(config.Paths.QueriesOut, 0755); err != nil {
		return fmt.Errorf("failed to create queries output directory: %w", err)
	}

	code, err := codegen.GenerateQueriesPackage(compiledQueries, "queries")
	if err != nil {
		return fmt.Errorf("failed to generate queries package: %w", err)
	}

	outputPath := filepath.Join(config.Paths.QueriesOut, "queries.go")
	if err := os.WriteFile(outputPath, code, 0644); err != nil {
		return fmt.Errorf("failed to write queries.go: %w", err)
	}

	fmt.Printf("Generated: %s\n", outputPath)
	fmt.Printf("\nSuccessfully compiled %d queries.\n", len(compiledQueries))

	return nil
}

// extractRegisteredQueries generates a temp program to extract queries from the registry.
func extractRegisteredQueries(ctx context.Context, queriesInPath string) (map[string]*query.AST, error) {
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

	_ %q  // Import triggers DefineQuery calls in init()
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
	var queries map[string]*query.AST
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
