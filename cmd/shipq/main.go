package main

import (
	"fmt"
	"os"

	authcmd "github.com/shipq/shipq/internal/commands/auth"
	dbcmd "github.com/shipq/shipq/internal/commands/db"
	dockercmd "github.com/shipq/shipq/internal/commands/docker"
	emailcmd "github.com/shipq/shipq/internal/commands/email"
	filescmd "github.com/shipq/shipq/internal/commands/files"
	handlercmd "github.com/shipq/shipq/internal/commands/handler"
	initcmd "github.com/shipq/shipq/internal/commands/init"
	killcmd "github.com/shipq/shipq/internal/commands/kill"
	llmcmd "github.com/shipq/shipq/internal/commands/llm"
	"github.com/shipq/shipq/internal/commands/migrate/new"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	nixcmd "github.com/shipq/shipq/internal/commands/nix"
	resourcecmd "github.com/shipq/shipq/internal/commands/resource"
	seedcmd "github.com/shipq/shipq/internal/commands/seed"
	signupcmd "github.com/shipq/shipq/internal/commands/signup"
	startcmd "github.com/shipq/shipq/internal/commands/start"
	statuscmd "github.com/shipq/shipq/internal/commands/status"
	workerscmd "github.com/shipq/shipq/internal/commands/workers"
)

const usage = `shipq - A database migration and code generation tool

Usage:
  shipq <command> [arguments]

Commands:
  status            Show project status and available next steps
  nix               Generate shell.nix with latest stable nixpkgs
  docker            Generate production Dockerfiles (server + optional worker)
  init              Initialize a new shipq project (creates go.mod and shipq.ini)
  auth              Generate authentication system (tables, handlers, tests)
  auth google       Add Google OAuth login to an existing auth system
  auth github       Add GitHub OAuth login to an existing auth system
  signup            Generate signup handler (run after auth)
  email             Add email verification and password reset (run after auth + workers)
  seed              Run all seed files in seeds/ directory
  start <service>   Start a dev service (postgres|mysql|sqlite|redis|minio|centrifugo|server|worker)
                    For server/worker: hot reload is on by default; use --no-watch to disable
  kill-port <port>  Kill the process bound to <port>
  kill-defaults     Kill all default dev-service ports
  db setup          Set up the database (create database and configure shipq.ini)
  db compile        Generate type-safe query runner code from user-defined queries
  db reset          Drop and recreate dev/test databases, re-run migrations (alias for migrate reset)
  migrate new <name>  Create a new migration
  migrate up        Run all pending migrations
  migrate reset     Drop and recreate dev/test databases, re-run migrations
  files             Generate S3-compatible file upload system (tables, handlers, helpers)
  workers           Bootstrap the workers system (channels, Centrifugo, task queue)
  workers compile   Recompile channel codegen without full bootstrap
  resource <table> <op>  Generate CRUD handler(s) for a table (create|get_one|list|update|delete|all)
  handler generate <table>  Generate CRUD handlers for a table
  handler compile           Compile handler registry and run codegen
  llm compile               Compile LLM tool registries, persister, migrations, and querydefs

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

	case "status":
		statuscmd.StatusCmd()

	case "nix":
		nixcmd.NixCmd()

	case "docker":
		dockercmd.DockerCmd()

	case "init":
		initcmd.InitCmd()

	case "auth":
		if len(os.Args) >= 3 {
			switch os.Args[2] {
			case "google", "github":
				authcmd.AuthOAuthCmd(os.Args[2])
			case "-h", "--help", "help":
				fmt.Println("shipq auth - Authentication commands")
				fmt.Println("")
				fmt.Println("Usage:")
				fmt.Println("  shipq auth            Generate the base authentication system (tables, handlers, tests)")
				fmt.Println("  shipq auth google     Add Google OAuth login to an existing auth system")
				fmt.Println("  shipq auth github     Add GitHub OAuth login to an existing auth system")
				os.Exit(0)
			default:
				fmt.Fprintf(os.Stderr, "error: unknown auth subcommand: %s\n", os.Args[2])
				fmt.Fprintln(os.Stderr, "Run 'shipq auth --help' for usage.")
				os.Exit(1)
			}
		} else {
			authcmd.AuthCmd()
		}

	case "signup":
		signupcmd.SignupCmd()

	case "email":
		emailcmd.EmailCmd()

	case "files":
		filescmd.FilesCmd()

	case "seed":
		seedcmd.SeedCmd()

	case "kill-port":
		if len(os.Args) < 3 || os.Args[2] == "--help" || os.Args[2] == "-h" || os.Args[2] == "help" {
			fmt.Println("shipq kill-port - Kill the process occupying a TCP port")
			fmt.Println("")
			fmt.Println("Usage: shipq kill-port <port>")
			fmt.Println("")
			fmt.Println("Sends SIGTERM to the process(es) bound to <port>.")
			fmt.Println("If nothing is on the port the command exits cleanly.")
			fmt.Println("SIGKILL is sent if the process does not exit within 3 s.")
			if len(os.Args) < 3 {
				os.Exit(1)
			}
			os.Exit(0)
		}
		killcmd.KillPortCmd(os.Args[2])

	case "kill-defaults":
		killcmd.KillDefaultsCmd()

	case "start":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq start' requires a service name")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Usage: shipq start <service>")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Services:")
			fmt.Fprintln(os.Stderr, "  postgres    Start a PostgreSQL server")
			fmt.Fprintln(os.Stderr, "  mysql       Start a MySQL server")
			fmt.Fprintln(os.Stderr, "  sqlite      Initialise the SQLite database file")
			fmt.Fprintln(os.Stderr, "  redis       Start a Redis server")
			fmt.Fprintln(os.Stderr, "  minio       Start a MinIO S3-compatible object store")
			fmt.Fprintln(os.Stderr, "  centrifugo  Start Centrifugo (WebSocket hub)")
			fmt.Fprintln(os.Stderr, "  server      Run the application server (go run ./cmd/server)")
			fmt.Fprintln(os.Stderr, "  worker      Run the background worker (go run ./cmd/worker)")
			os.Exit(1)
		}
		startcmd.StartCmd(os.Args[2], os.Args[3:])

	case "db":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq db' requires a subcommand")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Available subcommands:")
			fmt.Fprintln(os.Stderr, "  setup    Set up the database")
			fmt.Fprintln(os.Stderr, "  compile  Generate type-safe query runner code")
			fmt.Fprintln(os.Stderr, "  reset    Drop and recreate databases, re-run all migrations")
			os.Exit(1)
		}

		subCmd := os.Args[2]
		switch subCmd {
		case "setup":
			dbcmd.DBSetupCmd()

		case "compile":
			dbcmd.DBCompileCmd()

		case "reset":
			up.MigrateResetCmd() // Alias for user convenience

		case "-h", "--help", "help":
			fmt.Println("shipq db - Database management commands")
			fmt.Println("")
			fmt.Println("Subcommands:")
			fmt.Println("  setup    Set up the database (create database and configure shipq.ini)")
			fmt.Println("  compile  Generate type-safe query runner code from user-defined queries")
			fmt.Println("  reset    Drop and recreate databases, re-run all migrations")
			fmt.Println("")
			fmt.Println("To start a database server use: shipq start <postgres|mysql|sqlite|redis|minio>")
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
			new.MigrateNewCmd(os.Args[3:])

		case "up":
			up.MigrateUpCmd()

		case "reset":
			up.MigrateResetCmd()

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
			handlercmd.HandlerGenerateCmd(os.Args[3:])

		case "compile":
			handlercmd.HandlerCompileCmd()

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

	case "workers":
		if len(os.Args) < 3 {
			workerscmd.WorkersCmd()
			break
		}

		switch os.Args[2] {
		case "compile":
			workerscmd.WorkersCompileCmd()

		case "-h", "--help", "help":
			fmt.Println("shipq workers - Workers system commands")
			fmt.Println("")
			fmt.Println("Usage:")
			fmt.Println("  shipq workers           Bootstrap the workers system (channels, Centrifugo, task queue)")
			fmt.Println("  shipq workers compile    Recompile channel codegen without full bootstrap")
			fmt.Println("")
			fmt.Println("The 'compile' subcommand is useful after editing channel definitions.")
			fmt.Println("It performs only codegen steps (channel discovery, typed channels,")
			fmt.Println("worker main, Centrifugo config, TypeScript client, querydefs, and")
			fmt.Println("handler registry compilation) without running migrations, go mod tidy,")
			fmt.Println("prerequisite checks, or embedding.")
			fmt.Println("")
			fmt.Println("To start individual services use:")
			fmt.Println("  shipq start redis       # in one terminal")
			fmt.Println("  shipq start centrifugo  # in another terminal")
			fmt.Println("  shipq start worker      # in another terminal")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "error: unknown workers subcommand: %s\n", os.Args[2])
			fmt.Fprintln(os.Stderr, "Run 'shipq workers --help' for usage.")
			os.Exit(1)
		}

	case "llm":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq llm' requires a subcommand")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Available subcommands:")
			fmt.Fprintln(os.Stderr, "  compile    Compile LLM tool registries, persister, migrations, and querydefs")
			os.Exit(1)
		}

		switch os.Args[2] {
		case "compile":
			llmcmd.LLMCompileCmd()

		case "-h", "--help", "help":
			fmt.Println("shipq llm - LLM integration commands")
			fmt.Println("")
			fmt.Println("Subcommands:")
			fmt.Println("  compile    Compile LLM tool registries, persister, migrations, and querydefs")
			fmt.Println("")
			fmt.Println("The 'compile' subcommand:")
			fmt.Println("  1. Runs static analysis on tool packages to find Register() and app.Tool() calls")
			fmt.Println("  2. Builds and runs a temporary program to extract tool metadata via reflection")
			fmt.Println("  3. Generates typed tool dispatchers + per-package Registry() functions")
			fmt.Println("  4. Generates the llmpersist adapter (wraps queries.Runner → llm.Persister)")
			fmt.Println("  5. Generates database migration for llm_conversations + llm_messages tables")
			fmt.Println("  6. Generates querydefs for LLM persistence")
			fmt.Println("  7. Detects LLM-enabled channels and writes a marker for channel compile")
			fmt.Println("  8. Recompiles queries and handler registry")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "error: unknown llm subcommand: %s\n", os.Args[2])
			fmt.Fprintln(os.Stderr, "Run 'shipq llm --help' for usage.")
			os.Exit(1)
		}

	case "resource":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: 'shipq resource' requires a table name and operation")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Usage: shipq resource <table> <operation> [--public]")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Operations: create, get_one, list, update, delete, all")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Examples:")
			fmt.Fprintln(os.Stderr, "  shipq resource books create")
			fmt.Fprintln(os.Stderr, "  shipq resource books all")
			fmt.Fprintln(os.Stderr, "  shipq resource books all --public")
			os.Exit(1)
		}

		tableName := os.Args[2]

		if tableName == "-h" || tableName == "--help" || tableName == "help" {
			fmt.Println("shipq resource - Per-operation handler generation")
			fmt.Println("")
			fmt.Println("Usage: shipq resource <table> <operation> [--public]")
			fmt.Println("")
			fmt.Println("Operations:")
			fmt.Println("  create    Generate create handler + test")
			fmt.Println("  get_one   Generate get-one handler + test")
			fmt.Println("  list      Generate list handler + test (with pagination)")
			fmt.Println("  update    Generate update handler + test")
			fmt.Println("  delete    Generate soft-delete handler + test")
			fmt.Println("  all       Generate all 5 CRUD handlers + tests + register.go")
			fmt.Println("")
			fmt.Println("Flags:")
			fmt.Println("  --public  Skip auth protection for generated routes")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  shipq resource books create")
			fmt.Println("  shipq resource books all")
			fmt.Println("  shipq resource books all --public")
			os.Exit(0)
		}

		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "error: 'shipq resource' requires an operation")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Usage: shipq resource <table> <operation>")
			fmt.Fprintln(os.Stderr, "Operations: create, get_one, list, update, delete, all")
			os.Exit(1)
		}

		operation := os.Args[3]

		// Validate operation
		validOp := false
		for _, op := range resourcecmd.ValidOperations {
			if operation == op {
				validOp = true
				break
			}
		}
		if !validOp {
			fmt.Fprintf(os.Stderr, "error: unknown operation %q\n", operation)
			fmt.Fprintln(os.Stderr, "Valid operations: create, get_one, list, update, delete, all")
			os.Exit(1)
		}

		resourcecmd.ResourceCmd(tableName, operation, os.Args[4:])

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Run 'shipq --help' for usage.")
		os.Exit(1)
	}
}
