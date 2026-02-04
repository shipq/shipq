package main

import (
	"fmt"
	"os"
)

const usage = `shipq - A database migration and code generation tool

Usage:
  shipq <command> [arguments]

Commands:
  init              Initialize a new shipq project (creates go.mod and shipq.ini)
  db start <type>   Start a local database server (postgres|mysql|sqlite)
  db setup          Set up the database (create database and configure shipq.ini)
  db compile        Generate type-safe query runner code from user-defined queries
  db reset          Drop and recreate dev/test databases, re-run migrations (alias for migrate reset)
  migrate new <name>  Create a new migration
  migrate up        Run all pending migrations
  migrate reset     Drop and recreate dev/test databases, re-run migrations
  resource up       Run migrations and regenerate all handlers
  handler generate <table>  Generate CRUD handlers for a table
  handler compile           Compile handler registry and run codegen

Options:
  -h, --help    Show this help message

Run 'shipq <command> --help' for more information on a specific command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usage)
		os.Exit(0)

	case "init":
		initCmd()

	case "db":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq db' requires a subcommand")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Available subcommands:")
			fmt.Fprintln(os.Stderr, "  start <type>  Start a local database server (postgres|mysql|sqlite)")
			fmt.Fprintln(os.Stderr, "  setup         Set up the database")
			os.Exit(1)
		}

		subCmd := os.Args[2]
		switch subCmd {
		case "start":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "error: 'shipq db start' requires a database type")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Usage: shipq db start <type>")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Types:")
				fmt.Fprintln(os.Stderr, "  postgres  Start a PostgreSQL server")
				fmt.Fprintln(os.Stderr, "  mysql     Start a MySQL server")
				fmt.Fprintln(os.Stderr, "  sqlite    Initialize SQLite database file")
				os.Exit(1)
			}
			dbType := os.Args[3]
			dbStartCmd(dbType)

		case "setup":
			dbSetupCmd()

		case "compile":
			dbCompileCmd()

		case "reset":
			migrateResetCmd() // Alias for user convenience

		case "-h", "--help", "help":
			fmt.Println("shipq db - Database management commands")
			fmt.Println("")
			fmt.Println("Subcommands:")
			fmt.Println("  start <type>  Start a local database server (postgres|mysql|sqlite)")
			fmt.Println("  setup         Set up the database (create database and configure shipq.ini)")
			fmt.Println("  compile       Generate type-safe query runner code from user-defined queries")
			fmt.Println("  reset         Drop and recreate databases, re-run all migrations")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "error: unknown db subcommand: %s\n", subCmd)
			fmt.Fprintln(os.Stderr, "Run 'shipq db --help' for usage.")
			os.Exit(1)
		}

	case "migrate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq migrate' requires a subcommand")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Available subcommands:")
			fmt.Fprintln(os.Stderr, "  new <name> [columns...]  Create a new migration")
			fmt.Fprintln(os.Stderr, "  up                       Run all pending migrations")
			os.Exit(1)
		}

		subCmd := os.Args[2]
		switch subCmd {
		case "new":
			migrateNewCmd(os.Args[3:])

		case "up":
			migrateUpCmd()

		case "reset":
			migrateResetCmd()

		case "-h", "--help", "help":
			fmt.Println("shipq migrate - Migration management commands")
			fmt.Println("")
			fmt.Println("Subcommands:")
			fmt.Println("  new <name> [columns...]  Create a new migration")
			fmt.Println("  up                       Run all pending migrations")
			fmt.Println("  reset                    Drop and recreate databases, re-run all migrations")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  shipq migrate new users")
			fmt.Println("  shipq migrate new users name:string email:string")
			fmt.Println("  shipq migrate new posts title:string user_id:references:users")
			fmt.Println("")
			fmt.Println("Column types: string, text, int, bigint, bool, float, decimal, datetime, timestamp, binary, json")
			fmt.Println("References: <column>:references:<table>")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "error: unknown migrate subcommand: %s\n", subCmd)
			fmt.Fprintln(os.Stderr, "Run 'shipq migrate --help' for usage.")
			os.Exit(1)
		}

	case "handler":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq handler' requires a subcommand")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Available subcommands:")
			fmt.Fprintln(os.Stderr, "  generate <table>  Generate CRUD handlers for a table")
			fmt.Fprintln(os.Stderr, "  compile           Compile handler registry and run codegen")
			os.Exit(1)
		}

		subCmd := os.Args[2]
		switch subCmd {
		case "generate":
			handlerGenerateCmd(os.Args[3:])

		case "compile":
			handlerCompileCmd()

		case "-h", "--help", "help":
			fmt.Println("shipq handler - Handler generation commands")
			fmt.Println("")
			fmt.Println("Subcommands:")
			fmt.Println("  generate <table>  Generate CRUD handlers for a table")
			fmt.Println("  compile           Compile handler registry and run codegen")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  shipq handler generate posts")
			fmt.Println("  shipq handler generate users")
			fmt.Println("")
			fmt.Println("This generates handler files in api/<table>/ including:")
			fmt.Println("  - create.go      POST /<table>")
			fmt.Println("  - get_one.go     GET /<table>/:id")
			fmt.Println("  - list.go        GET /<table>")
			fmt.Println("  - update.go      PATCH /<table>/:id")
			fmt.Println("  - soft_delete.go DELETE /<table>/:id")
			fmt.Println("  - register.go    Handler registration function")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "error: unknown handler subcommand: %s\n", subCmd)
			fmt.Fprintln(os.Stderr, "Run 'shipq handler --help' for usage.")
			os.Exit(1)
		}

	case "resource":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq resource' requires a subcommand")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Available subcommands:")
			fmt.Fprintln(os.Stderr, "  up  Run migrations and regenerate all handlers")
			os.Exit(1)
		}

		subCmd := os.Args[2]
		switch subCmd {
		case "up":
			resourceUpCmd()

		case "-h", "--help", "help":
			fmt.Println("shipq resource - Resource management commands")
			fmt.Println("")
			fmt.Println("Subcommands:")
			fmt.Println("  up  Run migrations and regenerate all handlers")
			fmt.Println("")
			fmt.Println("The 'resource up' command:")
			fmt.Println("  1. Runs all pending migrations (same as 'shipq migrate up')")
			fmt.Println("  2. Regenerates CRUD handlers for all tables in the schema")
			fmt.Println("")
			fmt.Println("To opt out of handler regeneration for a specific table,")
			fmt.Println("create a file: api/<table>/.shipq-no-regen")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "error: unknown resource subcommand: %s\n", subCmd)
			fmt.Fprintln(os.Stderr, "Run 'shipq resource --help' for usage.")
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Run 'shipq --help' for usage.")
		os.Exit(1)
	}
}
