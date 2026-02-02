package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// Exit codes
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitConfig  = 2
)

// CLI holds the command-line interface state.
type CLI struct {
	args    []string
	version string
	stdout  io.Writer
	stderr  io.Writer
}

// NewCLI creates a new CLI instance.
func NewCLI(args []string, version string) *CLI {
	return &CLI{
		args:    args,
		version: version,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
	}
}

// WithOutput sets custom output writers (useful for testing).
func (c *CLI) WithOutput(stdout, stderr io.Writer) *CLI {
	c.stdout = stdout
	c.stderr = stderr
	return c
}

// Run is the main entry point for the CLI.
func Run(args []string, version string) int {
	cli := NewCLI(args, version)
	return cli.Execute()
}

// Execute parses arguments and runs the appropriate command.
func (c *CLI) Execute() int {
	if len(c.args) == 0 {
		c.printHelp()
		return ExitSuccess
	}

	cmd := c.args[0]
	cmdArgs := c.args[1:]

	switch cmd {
	case "help", "--help", "-h":
		c.printHelp()
		return ExitSuccess

	case "version", "--version", "-v":
		c.printVersion()
		return ExitSuccess

	case "migrate":
		return c.runMigrate(cmdArgs)

	case "compile":
		return c.runCompile(cmdArgs)

	default:
		fmt.Fprintf(c.stderr, "Error: unknown command %q\n\n", cmd)
		c.printHelp()
		return ExitError
	}
}

// runMigrate handles the migrate subcommands.
func (c *CLI) runMigrate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(c.stderr, "Error: migrate requires a subcommand (new, up, reset)\n\n")
		c.printMigrateHelp()
		return ExitError
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "new":
		return c.runMigrateNew(subArgs)
	case "up":
		return c.runMigrateUp(subArgs)
	case "reset":
		return c.runMigrateReset(subArgs)
	case "help", "--help", "-h":
		c.printMigrateHelp()
		return ExitSuccess
	default:
		fmt.Fprintf(c.stderr, "Error: unknown migrate subcommand %q\n\n", subcmd)
		c.printMigrateHelp()
		return ExitError
	}
}

// runMigrateNew creates a new migration file.
func (c *CLI) runMigrateNew(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(c.stderr, "Error: migrate new requires a migration name\n")
		fmt.Fprintf(c.stderr, "Usage: portsql migrate new <name>\n")
		return ExitError
	}

	name := args[0]

	// Load config
	config, err := LoadConfig("")
	if err != nil {
		fmt.Fprintf(c.stderr, "Error: failed to load config: %v\n", err)
		return ExitConfig
	}

	// Create the migration file
	path, err := MigrateNew(config, name)
	if err != nil {
		fmt.Fprintf(c.stderr, "Error: %v\n", err)
		return ExitError
	}

	fmt.Fprintf(c.stdout, "Created: %s\n", path)
	return ExitSuccess
}

// runMigrateUp runs pending migrations.
func (c *CLI) runMigrateUp(args []string) int {
	// Load config
	config, err := LoadConfig("")
	if err != nil {
		fmt.Fprintf(c.stderr, "Error: failed to load config: %v\n", err)
		return ExitConfig
	}

	ctx := context.Background()
	if err := MigrateUp(ctx, config); err != nil {
		fmt.Fprintf(c.stderr, "Error: %v\n", err)
		return ExitError
	}

	return ExitSuccess
}

// runMigrateReset drops all tables and re-runs migrations.
func (c *CLI) runMigrateReset(args []string) int {
	// Load config
	config, err := LoadConfig("")
	if err != nil {
		fmt.Fprintf(c.stderr, "Error: failed to load config: %v\n", err)
		return ExitConfig
	}

	ctx := context.Background()
	if err := MigrateReset(ctx, config); err != nil {
		fmt.Fprintf(c.stderr, "Error: %v\n", err)
		return ExitError
	}

	return ExitSuccess
}

// runCompile compiles query definitions to SQL.
func (c *CLI) runCompile(args []string) int {
	// Load config
	config, err := LoadConfig("")
	if err != nil {
		fmt.Fprintf(c.stderr, "Error: failed to load config: %v\n", err)
		return ExitConfig
	}

	ctx := context.Background()
	if err := Compile(ctx, config); err != nil {
		fmt.Fprintf(c.stderr, "Error: %v\n", err)
		return ExitError
	}

	return ExitSuccess
}

// printHelp prints the main help message.
func (c *CLI) printHelp() {
	help := `portsql - Type-safe SQL query builder and migration tool

Usage:
  portsql <command> [arguments]

Commands:
  migrate new <name>   Create a new timestamped migration file
  migrate up           Run pending migrations, generate schematypes
  migrate reset        Drop all tables, re-run all migrations (localhost only)
  compile              Compile query definitions to SQL strings
  help                 Show this help message
  version              Show version information

Flags:
  --help, -h           Show help for a command
  --version, -v        Show version information

Configuration:
  portsql looks for a shipq.ini file in the current directory.
  If not found, it uses DATABASE_URL environment variable for the database URL.

  Example shipq.ini:
    [db]
    url = postgres://localhost/myapp
    migrations = migrations
    schematypes = schematypes
    queries_in = querydef
    queries_out = queries

Run 'portsql <command> --help' for more information on a command.
`
	fmt.Fprint(c.stdout, help)
}

// printMigrateHelp prints help for the migrate command.
func (c *CLI) printMigrateHelp() {
	help := `Usage: portsql migrate <subcommand> [arguments]

Subcommands:
  new <name>    Create a new timestamped migration file
                Example: portsql migrate new create_users
                Creates: migrations/YYYYMMDDHHMMSS_create_users.go

  up            Run all pending migrations and generate schematypes
                - Connects to the database
                - Runs unapplied migrations in timestamp order
                - Updates schema.json
                - Generates schematypes/tables.go

  reset         Drop all tables and re-run all migrations from scratch
                WARNING: This command only works on localhost databases.
                It will fail if the database host is not localhost/127.0.0.1.
`
	fmt.Fprint(c.stdout, help)
}

// printVersion prints the version string.
func (c *CLI) printVersion() {
	fmt.Fprintf(c.stdout, "portsql version %s\n", c.version)
}

// parseFlags is a simple flag parser that extracts flags from args.
// Returns the remaining non-flag args and a map of flag values.
func parseFlags(args []string) ([]string, map[string]string) {
	remaining := []string{}
	flags := make(map[string]string)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if idx := strings.Index(key, "="); idx != -1 {
				flags[key[:idx]] = key[idx+1:]
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags[key] = args[i+1]
				i++
			} else {
				flags[key] = "true"
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) == 2 {
			key := strings.TrimPrefix(arg, "-")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags[key] = args[i+1]
				i++
			} else {
				flags[key] = "true"
			}
		} else {
			remaining = append(remaining, arg)
		}
	}

	return remaining, flags
}
