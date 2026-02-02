package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	portapicli "github.com/shipq/shipq/api/portapi/cli"
	portsqlcli "github.com/shipq/shipq/db/portsql/cli"
	"github.com/shipq/shipq/internal/initcmd"
	"github.com/shipq/shipq/internal/project"
)

// run dispatches commands and returns an exit code.
func run(args []string) int {
	return runWithOutput(args, os.Stdout, os.Stderr)
}

// runWithOutput dispatches commands with custom output writers.
func runWithOutput(args []string, stdout, stderr io.Writer) int {
	// Parse global flags first
	var projectPath string
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle --project flag
		if arg == "--project" {
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "Error: --project requires a path argument")
				return 1
			}
			projectPath = args[i+1]
			i++ // Skip the value
			continue
		}
		if strings.HasPrefix(arg, "--project=") {
			projectPath = strings.TrimPrefix(arg, "--project=")
			continue
		}

		// Once we hit a non-global-flag argument, everything else is the command and its args
		remaining = args[i:]
		break
	}

	if len(remaining) == 0 {
		printHelp(stdout)
		return 0
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	switch cmd {
	case "help", "--help", "-h":
		printHelp(stdout)
		return 0

	case "version", "--version", "-v":
		printVersion(stdout)
		return 0

	case "init":
		// init operates on CWD by default, doesn't need project discovery
		return runInit(cmdArgs, stdout, stderr)

	case "db":
		return runDBWithProject(cmdArgs, projectPath, stdout, stderr)

	case "api":
		return runAPIWithProject(cmdArgs, projectPath, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Error: unknown command %q\n\n", cmd)
		printHelp(stderr)
		return 1
	}
}

// runDB delegates to the PortSQL CLI.
func runDB(args []string, stdout, stderr io.Writer) int {
	c := portsqlcli.NewCLI(args, Version)
	c.WithOutput(stdout, stderr)
	return c.Execute()
}

// runDBWithProject runs the PortSQL CLI after resolving and changing to the project root.
func runDBWithProject(args []string, projectPath string, stdout, stderr io.Writer) int {
	if err := changeToProjectRoot(projectPath, stderr); err != nil {
		return 1
	}
	return runDB(args, stdout, stderr)
}

// runAPI delegates to the PortAPI generator CLI.
func runAPI(args []string, stdout, stderr io.Writer) int {
	return portapicli.Run(args, portapicli.Options{
		Stdout:  stdout,
		Stderr:  stderr,
		Version: Version,
	})
}

// runAPIWithProject runs the PortAPI CLI after resolving and changing to the project root.
func runAPIWithProject(args []string, projectPath string, stdout, stderr io.Writer) int {
	if err := changeToProjectRoot(projectPath, stderr); err != nil {
		return 1
	}
	return runAPI(args, stdout, stderr)
}

// changeToProjectRoot resolves the project root and changes the working directory.
// If projectPath is empty, it searches upward from the current directory.
func changeToProjectRoot(projectPath string, stderr io.Writer) error {
	root, err := project.ResolveWithOverride(projectPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return err
	}

	if err := os.Chdir(root.Dir); err != nil {
		fmt.Fprintf(stderr, "Error: failed to change to project directory: %v\n", err)
		return err
	}

	return nil
}

// runInit scaffolds a new ShipQ project.
func runInit(args []string, stdout, stderr io.Writer) int {
	return initcmd.Run(args, initcmd.Options{
		Stdout: stdout,
		Stderr: stderr,
	})
}

// printHelp prints the top-level help message.
func printHelp(w io.Writer) {
	help := `shipq - Unified CLI for ShipQ tools

Usage:
  shipq <command> [arguments]

Commands:
  init              Initialize a new ShipQ project (creates shipq.ini and directories)
  db                Run database commands (migrations, queries, start, setup)
  api               Run API generator commands
  help              Show this help message
  version           Show version information

Global Flags:
  --project <path>  Path to ShipQ project root (default: search upward for shipq.ini)
  --help, -h        Show help for a command
  --version, -v     Show version information

Examples:
  shipq init                      Create a new project with default settings
  shipq init --database postgres  Create a new project with PostgreSQL dialect

  shipq db start postgres             Start a local Postgres server
  shipq db start mysql                Start a local MySQL server
  shipq db setup                      Create dev and test databases

  shipq db migrate new create_users   Create a new migration file
  shipq db migrate up                 Run pending migrations
  shipq db compile                    Compile query definitions

  shipq api                           Generate HTTP handlers from endpoint definitions

  # Run from any subdirectory (auto-discovers project root)
  cd api && shipq db compile

  # Explicitly specify project path
  shipq --project /path/to/myproject db migrate up

Run 'shipq <command> --help' for more information on a specific command.

Note: 'shipq db' exposes the PortSQL command set.
      'shipq api' exposes the PortAPI generator command set.
`
	fmt.Fprint(w, help)
}

// printVersion prints the version string.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "shipq version %s\n", Version)
}
