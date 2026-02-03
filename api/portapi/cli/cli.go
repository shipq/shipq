// Package cli provides the command-line interface for the ShipQ API HTTP generator.
package cli

import (
	"fmt"
	"io"
	"os"
)

// Exit codes
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitConfig  = 2
)

// Options configures the CLI behavior.
type Options struct {
	// Stdout is the writer for standard output. Defaults to os.Stdout if nil.
	Stdout io.Writer
	// Stderr is the writer for standard error. Defaults to os.Stderr if nil.
	Stderr io.Writer
	// Version is the version string to display. Defaults to "dev" if empty.
	Version string
}

// defaults returns Options with default values for nil fields.
func (o Options) defaults() Options {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	return o
}

// Run executes the ShipQ API generator CLI with the given arguments.
// It returns an exit code (0 for success, non-zero for errors).
// This function does NOT call os.Exit - the caller is responsible for that.
func Run(args []string, opts Options) int {
	opts = opts.defaults()

	// Handle no arguments - run the default generator
	if len(args) == 0 {
		if err := runGenerator(opts.Stdout, opts.Stderr); err != nil {
			fmt.Fprintf(opts.Stderr, "error: %v\n", err)
			return ExitError
		}
		return ExitSuccess
	}

	// Handle commands and flags
	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "--help", "-h":
		printHelp(opts.Stdout)
		return ExitSuccess

	case "version", "--version", "-v":
		printVersion(opts.Stdout, opts.Version)
		return ExitSuccess

	case "resource":
		return runResource(cmdArgs, opts)

	default:
		// If the first arg isn't a known command, assume it's a flag
		// and run the default generator (for backward compatibility)
		if err := runGenerator(opts.Stdout, opts.Stderr); err != nil {
			fmt.Fprintf(opts.Stderr, "error: %v\n", err)
			return ExitError
		}
		return ExitSuccess
	}
}

// runResource handles the `resource` subcommand.
func runResource(args []string, opts Options) int {
	// Handle help flag
	if len(args) > 0 {
		switch args[0] {
		case "help", "--help", "-h":
			printResourceHelp(opts.Stdout)
			return ExitSuccess
		}
	}

	// Require table name argument
	if len(args) == 0 {
		fmt.Fprintf(opts.Stderr, "Error: resource requires a table name\n")
		fmt.Fprintf(opts.Stderr, "Usage: shipq api resource <table>\n\n")
		fmt.Fprintf(opts.Stderr, "Run 'shipq api resource --help' for more information.\n")
		return ExitError
	}

	tableName := args[0]

	// Parse optional flags
	resourceOpts := ResourceOptions{
		TableName: tableName,
		Prefix:    "", // Default: no prefix
		OutDir:    "api/resources",
	}

	// Default: generate runtime after resource creation
	generateRuntime := true

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--prefix" && i+1 < len(args):
			resourceOpts.Prefix = args[i+1]
			i++
		case arg == "--out" && i+1 < len(args):
			resourceOpts.OutDir = args[i+1]
			i++
		case arg == "--no-runtime":
			generateRuntime = false
		case arg == "--runtime":
			generateRuntime = true
		}
	}

	// Load config to get DB integration settings
	cfg, err := LoadConfig("")
	if err == nil {
		// Pass queries output path for DB integration
		resourceOpts.QueriesOut = cfg.QueriesOut
		resourceOpts.QueriesImport = cfg.DBRunnerImport
		resourceOpts.DBRunnerImport = cfg.DBRunnerImport
		// Use first dialect for runner package (e.g., "sqlite", "postgres")
		if len(cfg.Dialects) > 0 {
			resourceOpts.DBRunnerPackage = cfg.Dialects[0]
		}
	}
	// Config load failure is not fatal - just means no DB integration

	// Run the resource generator
	if err := GenerateResource(resourceOpts, opts.Stdout, opts.Stderr); err != nil {
		fmt.Fprintf(opts.Stderr, "error: %v\n", err)
		return ExitError
	}

	// Generate HTTP runtime (zz_generated_http.go) unless --no-runtime was specified
	if generateRuntime {
		fmt.Fprintf(opts.Stdout, "\nGenerating HTTP runtime...\n")
		if err := runGenerator(opts.Stdout, opts.Stderr); err != nil {
			fmt.Fprintf(opts.Stderr, "error generating HTTP runtime: %v\n", err)
			return ExitError
		}
	}

	return ExitSuccess
}

// printHelp prints the main help message.
func printHelp(w io.Writer) {
	help := `shipq api - HTTP handler code generator for ShipQ API endpoints

Usage:
  shipq api [command] [flags]

Commands:
  resource <table>   Generate REST API endpoints for a database table
  help               Show this help message
  version            Show version information

When run without a command, generates HTTP handlers from endpoint definitions.

Flags:
  --help, -h       Show this help message
  --version, -v    Show version information

Configuration (shipq.ini):
  [api]
  package = ./api                    # Output package path (required)
  middleware_package = ./middleware  # Middleware package path (optional)
  openapi = true                     # Generate OpenAPI spec (optional)
  openapi_output = openapi.json      # OpenAPI output filename
  docs_ui = true                     # Generate docs UI (optional)
  docs_path = /docs                  # Docs UI route path
  test_client = true                 # Generate test client (optional)

Examples:
  # Generate handlers (run from project root with shipq.ini)
  shipq api

  # Generate REST endpoints for a table
  shipq api resource users

  # Show help
  shipq api --help

Run 'shipq api <command> --help' for more information on a specific command.
`
	fmt.Fprint(w, help)
}

// printResourceHelp prints the help message for the resource subcommand.
func printResourceHelp(w io.Writer) {
	help := `shipq api resource - Generate REST API endpoints for a database table

Usage:
  shipq api resource <table> [flags]

Arguments:
  <table>    The database table name (plural, snake_case, e.g. "users", "purchase_orders")

Flags:
  --prefix <path>   URL prefix for endpoints (default: "")
  --out <dir>       Output directory (default: "api/resources")
  --no-runtime      Skip HTTP runtime generation (useful in CI or when generating multiple resources)
  --runtime         Generate HTTP runtime (default behavior)
  --help, -h        Show this help message

Description:
  Generates HTTP handler code that wraps the CRUD operations generated by
  'shipq db compile'. The generated endpoints follow RESTful conventions:

    GET    /<table>/{public_id}   Fetch a single record
    GET    /<table>               List records (paginated)
    POST   /<table>               Create a new record
    PUT    /<table>/{public_id}   Update a record
    DELETE /<table>/{public_id}   Soft-delete a record

  Resources can only be generated for tables created with plan.AddTable()
  in migrations. These tables have public_id and deleted_at columns that
  enable safe public identifiers and soft deletion.

DB Integration:
  If queries_out is configured in shipq.ini, handlers will be generated with
  full DB integration:

    [db]
    queries_out = queries

  DB-backed handlers use RegisterWithDB(app, db) for registration:

    import "myapp/api/resources/users"
    users.RegisterWithDB(app, db)

  Run migrations at app startup using the runner package:

    import "myapp/db/generated"
    generated.Run(ctx, db, "postgres")

Examples:
  # Generate REST endpoints for the users table
  shipq api resource users

  # Generate with a URL prefix
  shipq api resource users --prefix /api/v1

  # Generate to a custom output directory
  shipq api resource users --out internal/handlers

After generating a resource:
  The HTTP runtime (zz_generated_http.go) is automatically regenerated.
  If you use --no-runtime, run 'shipq api' manually to regenerate the HTTP runtime.

  Register the resource in your app:
    - With DB: users.RegisterWithDB(app, db)
    - Without DB: users.Register(app)
`
	fmt.Fprint(w, help)
}

// printVersion prints the version string.
func printVersion(w io.Writer, version string) {
	fmt.Fprintf(w, "shipq api version %s\n", version)
}
