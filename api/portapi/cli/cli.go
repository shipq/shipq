// Package cli provides the command-line interface for the PortAPI HTTP generator.
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

// Run executes the PortAPI generator CLI with the given arguments.
// It returns an exit code (0 for success, non-zero for errors).
// This function does NOT call os.Exit - the caller is responsible for that.
func Run(args []string, opts Options) int {
	opts = opts.defaults()

	// Handle help and version flags
	if len(args) > 0 {
		switch args[0] {
		case "help", "--help", "-h":
			printHelp(opts.Stdout)
			return ExitSuccess
		case "version", "--version", "-v":
			printVersion(opts.Stdout, opts.Version)
			return ExitSuccess
		}
	}

	// Run the generator
	if err := runGenerator(opts.Stdout, opts.Stderr); err != nil {
		fmt.Fprintf(opts.Stderr, "error: %v\n", err)
		return ExitError
	}

	return ExitSuccess
}

// printHelp prints the help message.
func printHelp(w io.Writer) {
	help := `portapi - HTTP handler code generator for PortAPI endpoints

Usage:
  portapi [flags]

The generator reads configuration from shipq.ini in the current directory
and generates HTTP handler code based on discovered endpoint definitions.

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
  portapi

  # Show help
  portapi --help
`
	fmt.Fprint(w, help)
}

// printVersion prints the version string.
func printVersion(w io.Writer, version string) {
	fmt.Fprintf(w, "portapi version %s\n", version)
}
