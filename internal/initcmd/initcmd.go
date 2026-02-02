// Package initcmd implements the 'shipq init' command for scaffolding new ShipQ projects.
package initcmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/internal/config"
)

// Options configures the init command execution.
type Options struct {
	Stdout io.Writer
	Stderr io.Writer
}

// directories to create during init
var directories = []string{
	"migrations",
	"schematypes",
	"querydef",
	"queries",
	"api",
}

// Run executes the init command with the given arguments.
// Returns an exit code (0 for success, 1 for error).
func Run(args []string, opts Options) int {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	// Parse flags
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(opts.Stderr)

	database := fs.String("database", "mysql", "database dialect (mysql, postgres, sqlite)")
	force := fs.Bool("force", false, "overwrite existing shipq.ini")
	help := fs.Bool("help", false, "show help for init command")
	helpShort := fs.Bool("h", false, "show help for init command")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpShort {
		printHelp(opts.Stdout)
		return 0
	}

	// Validate database dialect
	dialect := strings.ToLower(*database)
	if !isValidDialect(dialect) {
		fmt.Fprintf(opts.Stderr, "Error: invalid database dialect %q\n", *database)
		fmt.Fprintf(opts.Stderr, "  Supported dialects: mysql, postgres, sqlite\n")
		return 1
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(opts.Stderr, "Error: failed to get current directory: %v\n", err)
		return 1
	}

	// Execute the init
	if err := execute(cwd, dialect, *force, opts.Stdout, opts.Stderr); err != nil {
		fmt.Fprintf(opts.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}

// execute performs the actual initialization.
func execute(targetDir, dialect string, force bool, stdout, stderr io.Writer) error {
	configPath := filepath.Join(targetDir, config.ConfigFilename)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		if !force {
			return fmt.Errorf("%s already exists\n"+
				"  Use --force to overwrite", config.ConfigFilename)
		}
	}

	// Check for conflicting paths (files where directories should be)
	for _, dir := range directories {
		dirPath := filepath.Join(targetDir, dir)
		info, err := os.Stat(dirPath)
		if err == nil && !info.IsDir() {
			return fmt.Errorf("expected directory but found file: %s", dir)
		}
	}

	// Create directories
	createdDirs := []string{}
	for _, dir := range directories {
		dirPath := filepath.Join(targetDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
			createdDirs = append(createdDirs, dir)
		}
	}

	// Generate and write config file
	configContent := renderShipqINI(dialect)

	// Write atomically: write to temp file then rename
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize config file: %w", err)
	}

	// Print success message
	printSuccess(stdout, createdDirs, dialect, force)

	return nil
}

// renderShipqINI generates the shipq.ini content for the given dialect.
func renderShipqINI(dialect string) string {
	var sb strings.Builder

	sb.WriteString("# ShipQ configuration file\n")
	sb.WriteString("# This file marks the root of your ShipQ project.\n")
	sb.WriteString("\n")

	// [db] section
	sb.WriteString("[db]\n")
	sb.WriteString("# Database connection URL.\n")
	sb.WriteString("# You can also set the DATABASE_URL environment variable instead.\n")

	// Write commented example URL based on dialect
	switch dialect {
	case "mysql":
		sb.WriteString("# url = mysql://user:pass@tcp(localhost:3306)/dbname\n")
	case "postgres":
		sb.WriteString("# url = postgres://user:pass@localhost:5432/dbname?sslmode=disable\n")
	case "sqlite":
		sb.WriteString("# url = sqlite://./database.db\n")
	}
	sb.WriteString("url =\n")
	sb.WriteString("\n")

	sb.WriteString("# Database dialect(s) to generate code for.\n")
	sb.WriteString(fmt.Sprintf("dialects = %s\n", dialect))
	sb.WriteString("\n")

	sb.WriteString("# Paths for database-related files (relative to this config file).\n")
	sb.WriteString("migrations = migrations\n")
	sb.WriteString("schematypes = schematypes\n")
	sb.WriteString("queries_in = querydef\n")
	sb.WriteString("queries_out = queries\n")
	sb.WriteString("\n")

	sb.WriteString("# Optional: Global CRUD scope filter (e.g., tenant_id).\n")
	sb.WriteString("# scope =\n")
	sb.WriteString("\n")

	sb.WriteString("# Optional: Global CRUD ordering.\n")
	sb.WriteString("# order =\n")
	sb.WriteString("\n")

	// [api] section
	sb.WriteString("[api]\n")
	sb.WriteString("# Package path for generated API code.\n")
	sb.WriteString("package = ./api\n")
	sb.WriteString("\n")

	sb.WriteString("# Optional: Package for middleware.\n")
	sb.WriteString("# middleware_package =\n")
	sb.WriteString("\n")

	sb.WriteString("# OpenAPI generation settings.\n")
	sb.WriteString("# openapi = true\n")
	sb.WriteString("# openapi_output = openapi.json\n")
	sb.WriteString("# openapi_title =\n")
	sb.WriteString("# openapi_version = 0.0.0\n")
	sb.WriteString("# openapi_description =\n")
	sb.WriteString("# openapi_servers =\n")
	sb.WriteString("\n")

	sb.WriteString("# Docs UI settings.\n")
	sb.WriteString("# docs_ui = false\n")
	sb.WriteString("# docs_path = /docs\n")
	sb.WriteString("# openapi_json_path = /openapi.json\n")
	sb.WriteString("\n")

	sb.WriteString("# Test client generation.\n")
	sb.WriteString("# test_client = false\n")
	sb.WriteString("# test_client_filename = zz_generated_testclient_test.go\n")

	return sb.String()
}

// isValidDialect checks if the given dialect is supported.
func isValidDialect(dialect string) bool {
	for _, d := range config.ValidDialects {
		if d == dialect {
			return true
		}
	}
	return false
}

// printSuccess prints the success message with next steps.
func printSuccess(w io.Writer, createdDirs []string, dialect string, wasForce bool) {
	fmt.Fprintln(w)
	if wasForce {
		fmt.Fprintln(w, "✓ Overwrote shipq.ini")
	} else {
		fmt.Fprintln(w, "✓ Created shipq.ini")
	}

	if len(createdDirs) > 0 {
		fmt.Fprintf(w, "✓ Created directories: %s\n", strings.Join(createdDirs, ", "))
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  1. Configure your database connection:")
	fmt.Fprintf(w, "     • Edit shipq.ini and set [db] url, or\n")
	fmt.Fprintf(w, "     • Set the DATABASE_URL environment variable\n")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  2. Create your first migration:")
	fmt.Fprintln(w, "     shipq db migrate new init")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  3. Run migrations:")
	fmt.Fprintln(w, "     shipq db migrate up")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Database dialect: %s\n", dialect)
	fmt.Fprintln(w)
}

// printHelp prints the help message for the init command.
func printHelp(w io.Writer) {
	help := `shipq init - Initialize a new ShipQ project

Usage:
  shipq init [flags]

Flags:
  --database <dialect>  Database dialect to use (default: mysql)
                        Supported: mysql, postgres, sqlite
  --force               Overwrite existing shipq.ini
  --help, -h            Show this help message

Description:
  Creates a new ShipQ project in the current directory with:
  • shipq.ini     - Configuration file (single source of truth)
  • migrations/   - Database migration files
  • schematypes/  - Generated schema type definitions
  • querydef/     - Query definitions
  • queries/      - Generated query code
  • api/          - Generated API code

Examples:
  shipq init                      # Initialize with MySQL (default)
  shipq init --database postgres  # Initialize with PostgreSQL
  shipq init --database sqlite    # Initialize with SQLite
  shipq init --force              # Reinitialize, overwriting config
`
	fmt.Fprint(w, help)
}

// ErrConfigExists is returned when shipq.ini already exists and --force is not set.
var ErrConfigExists = errors.New("shipq.ini already exists")
