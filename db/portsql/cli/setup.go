// Package cli provides the PortSQL command-line interface.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Default connection settings for local database servers started by `shipq db start`
const (
	// Postgres defaults (matches initdb --username=postgres)
	defaultPostgresURL = "postgres://postgres@localhost:5432/postgres"

	// MySQL defaults (uses socket from start mysql)
	// Note: For MySQL we use TCP since socket path varies by project
	defaultMySQLURL = "mysql://root@localhost:3306/"
)

// Setup creates the dev and test databases based on configuration.
// It only works with localhost databases for safety.
func Setup(cfg *Config, stdout, stderr io.Writer) error {
	// Get the database URL, with fallback to dialect-based defaults
	dbURL := cfg.Database.URL
	if dbURL == "" {
		// Try to infer URL from configured dialects
		dbURL = inferDefaultURL(cfg, stderr)
	}
	if dbURL == "" {
		return fmt.Errorf("no database URL configured\n" +
			"  Set db.url in shipq.ini, or run with a dialect that has a running local server:\n" +
			"    shipq db start postgres  # then run setup\n" +
			"    shipq db start mysql     # then run setup")
	}

	// Parse the URL
	parsed, err := ParseDBURL(dbURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Enforce localhost safety
	if !parsed.IsLocalhost() {
		return fmt.Errorf("shipq db setup only supports localhost databases for safety\n"+
			"  Got host: %s\n"+
			"  Hint: Use localhost, 127.0.0.1, or ::1 as the host", parsed.Host)
	}

	// Get project folder name for default database names
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectFolder := filepath.Base(projectRoot)

	// Get config overrides
	baseName := cfg.Database.Name
	devNameOverride := cfg.Database.DevName
	testNameOverride := cfg.Database.TestName

	// Derive database names
	devName, testName := DeriveDBNames(baseName, devNameOverride, testNameOverride, projectFolder)

	// Validate database names
	if err := ValidateDBName(devName); err != nil {
		return fmt.Errorf("invalid dev database name: %w", err)
	}
	if err := ValidateDBName(testName); err != nil {
		return fmt.Errorf("invalid test database name: %w", err)
	}

	fmt.Fprintf(stdout, "Setting up databases for project: %s\n", projectFolder)
	fmt.Fprintf(stdout, "  Dev database:  %s\n", devName)
	fmt.Fprintf(stdout, "  Test database: %s\n\n", testName)

	// Dispatch to dialect-specific setup
	switch parsed.Dialect {
	case "postgres":
		return setupPostgres(parsed, devName, testName, stdout, stderr)
	case "mysql":
		return setupMySQL(parsed, devName, testName, stdout, stderr)
	case "sqlite":
		return setupSQLite(parsed, devName, testName, stdout, stderr)
	default:
		return fmt.Errorf("unsupported dialect for setup: %s", parsed.Dialect)
	}
}

// inferDefaultURL attempts to determine a default database URL based on:
// 1. Configured dialects in shipq.ini
// 2. Detection of running local database servers
func inferDefaultURL(cfg *Config, stderr io.Writer) string {
	dialects := cfg.Database.GetDialects()

	// If dialects are explicitly configured, use the first one
	if len(dialects) > 0 {
		switch dialects[0] {
		case "postgres":
			fmt.Fprintf(stderr, "Note: No db.url configured, using default Postgres URL for local server\n")
			return defaultPostgresURL
		case "mysql":
			fmt.Fprintf(stderr, "Note: No db.url configured, using default MySQL URL for local server\n")
			return defaultMySQLURL
		case "sqlite":
			// SQLite doesn't need setup in the same way
			return ""
		}
	}

	// Try to detect which server might be running by checking for data directories
	projectRoot, err := os.Getwd()
	if err != nil {
		return ""
	}

	postgresData := filepath.Join(projectRoot, postgresDataDir)
	mysqlData := filepath.Join(projectRoot, mysqlDataDir)

	// Check if postgres data dir exists (suggests postgres is/was used)
	if _, err := os.Stat(postgresData); err == nil {
		fmt.Fprintf(stderr, "Note: Found Postgres data directory, using default Postgres URL\n")
		return defaultPostgresURL
	}

	// Check if mysql data dir exists
	if _, err := os.Stat(mysqlData); err == nil {
		fmt.Fprintf(stderr, "Note: Found MySQL data directory, using default MySQL URL\n")
		return defaultMySQLURL
	}

	return ""
}

// setupPostgres creates Postgres databases.
func setupPostgres(parsed *ParsedDBURL, devName, testName string, stdout, stderr io.Writer) error {
	// Connect to the maintenance database (usually "postgres")
	maintURL := parsed.MaintenanceURL()

	// Convert URL to connection string format that lib/pq expects
	connStr := postgresURLToConnStr(maintURL)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to Postgres: %w\n"+
			"  Is the server running? Try: shipq db start postgres", err)
	}
	defer db.Close()

	// Test connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to connect to Postgres: %w\n"+
			"  Is the server running? Try: shipq db start postgres", err)
	}

	// Create dev database
	devCreated, err := createPostgresDB(ctx, db, devName)
	if err != nil {
		return fmt.Errorf("failed to create dev database %q: %w", devName, err)
	}
	if devCreated {
		fmt.Fprintf(stdout, "Created database: %s\n", devName)
	} else {
		fmt.Fprintf(stdout, "Database already exists: %s\n", devName)
	}

	// Create test database
	testCreated, err := createPostgresDB(ctx, db, testName)
	if err != nil {
		return fmt.Errorf("failed to create test database %q: %w", testName, err)
	}
	if testCreated {
		fmt.Fprintf(stdout, "Created database: %s\n", testName)
	} else {
		fmt.Fprintf(stdout, "Database already exists: %s\n", testName)
	}

	// Print connection info
	fmt.Fprintf(stdout, "\nConnection URLs:\n")
	fmt.Fprintf(stdout, "  Dev:  %s\n", parsed.WithDatabase(devName))
	fmt.Fprintf(stdout, "  Test: %s\n", parsed.WithDatabase(testName))

	return nil
}

// createPostgresDB creates a Postgres database if it doesn't exist.
// Returns (created bool, error).
func createPostgresDB(ctx context.Context, db *sql.DB, dbName string) (bool, error) {
	// Check if database exists
	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbName,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		return false, nil
	}

	// Create the database
	// Note: CREATE DATABASE cannot use parameters, so we use string formatting
	// The dbName has already been validated by ValidateDBName
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(dbName)))
	if err != nil {
		return false, err
	}

	return true, nil
}

// setupMySQL creates MySQL databases.
func setupMySQL(parsed *ParsedDBURL, devName, testName string, stdout, stderr io.Writer) error {
	// Try socket connection first (matches `shipq db start mysql` behavior)
	// then fall back to TCP if socket doesn't exist
	var db *sql.DB
	var err error
	var connMethod string

	// Get project root to find socket
	projectRoot, _ := os.Getwd()
	socketPath := filepath.Join(projectRoot, mysqlDataDir, "mysql.sock")

	// Try socket first if it exists
	if _, statErr := os.Stat(socketPath); statErr == nil {
		// Socket exists, try to connect via socket
		socketDSN := buildMySQLSocketDSN(parsed, socketPath)
		db, err = sql.Open("mysql", socketDSN)
		if err == nil {
			if pingErr := db.Ping(); pingErr == nil {
				connMethod = "socket"
			} else {
				db.Close()
				db = nil
			}
		}
	}

	// Fall back to TCP if socket connection failed
	if db == nil {
		connStr := mysqlURLToConnStr(parsed.MaintenanceURL())
		db, err = sql.Open("mysql", connStr)
		if err != nil {
			return fmt.Errorf("failed to connect to MySQL: %w\n"+
				"  Is the server running? Try: shipq db start mysql", err)
		}
		connMethod = "TCP"
	}
	defer db.Close()

	_ = connMethod // Used for potential debug output

	// Test connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w\n"+
			"  Is the server running? Try: shipq db start mysql", err)
	}

	// Create dev database (MySQL supports IF NOT EXISTS)
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoteIdentifierMySQL(devName)))
	if err != nil {
		return fmt.Errorf("failed to create dev database %q: %w", devName, err)
	}
	fmt.Fprintf(stdout, "Ensured database exists: %s\n", devName)

	// Create test database
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoteIdentifierMySQL(testName)))
	if err != nil {
		return fmt.Errorf("failed to create test database %q: %w", testName, err)
	}
	fmt.Fprintf(stdout, "Ensured database exists: %s\n", testName)

	// Print connection info
	fmt.Fprintf(stdout, "\nConnection URLs:\n")
	fmt.Fprintf(stdout, "  Dev:  %s\n", parsed.WithDatabase(devName))
	fmt.Fprintf(stdout, "  Test: %s\n", parsed.WithDatabase(testName))

	return nil
}

// setupSQLite handles SQLite "setup" (minimal - just ensures directory exists).
func setupSQLite(parsed *ParsedDBURL, devName, testName string, stdout, stderr io.Writer) error {
	fmt.Fprintf(stdout, "SQLite databases are created automatically when accessed.\n")
	fmt.Fprintf(stdout, "No setup required.\n\n")

	// Derive file paths
	devPath := devName + ".db"
	testPath := testName + ".db"

	fmt.Fprintf(stdout, "Expected database files:\n")
	fmt.Fprintf(stdout, "  Dev:  %s\n", devPath)
	fmt.Fprintf(stdout, "  Test: %s\n", testPath)

	return nil
}

// postgresURLToConnStr converts a postgres:// URL to a lib/pq connection string.
func postgresURLToConnStr(dbURL string) string {
	// lib/pq can actually accept postgres:// URLs directly
	return dbURL
}

// mysqlURLToConnStr converts a mysql:// URL to a go-sql-driver/mysql DSN.
func mysqlURLToConnStr(dbURL string) string {
	// Parse the URL
	parsed, err := ParseDBURL(dbURL)
	if err != nil {
		return dbURL // Return as-is and let the driver handle errors
	}

	// Build DSN: [user[:password]@][protocol[(address)]]/dbname[?param=value]
	var dsn strings.Builder

	// User and password
	if parsed.User != "" {
		dsn.WriteString(parsed.User)
		if parsed.Password != "" {
			dsn.WriteString(":")
			dsn.WriteString(parsed.Password)
		}
		dsn.WriteString("@")
	}

	// Protocol and address
	if parsed.Host != "" {
		dsn.WriteString("tcp(")
		dsn.WriteString(parsed.Host)
		if parsed.Port != "" {
			dsn.WriteString(":")
			dsn.WriteString(parsed.Port)
		}
		dsn.WriteString(")")
	}

	// Database name
	dsn.WriteString("/")
	dsn.WriteString(parsed.Database)

	// Query params
	if parsed.Query != "" {
		dsn.WriteString("?")
		dsn.WriteString(parsed.Query)
	}

	return dsn.String()
}

// quoteIdentifier quotes a Postgres identifier (table name, database name, etc.)
// to prevent SQL injection.
func quoteIdentifier(name string) string {
	// Double any existing double quotes and wrap in double quotes
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// quoteIdentifierMySQL quotes a MySQL identifier using backticks.
func quoteIdentifierMySQL(name string) string {
	// Double any existing backticks and wrap in backticks
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// buildMySQLSocketDSN builds a MySQL DSN for socket connection.
func buildMySQLSocketDSN(parsed *ParsedDBURL, socketPath string) string {
	var dsn strings.Builder

	// User and password
	if parsed.User != "" {
		dsn.WriteString(parsed.User)
		if parsed.Password != "" {
			dsn.WriteString(":")
			dsn.WriteString(parsed.Password)
		}
		dsn.WriteString("@")
	}

	// Socket connection
	dsn.WriteString("unix(")
	dsn.WriteString(socketPath)
	dsn.WriteString(")/")

	// Database (empty for maintenance)
	dsn.WriteString(parsed.Database)

	// Query params
	if parsed.Query != "" {
		dsn.WriteString("?")
		dsn.WriteString(parsed.Query)
	}

	return dsn.String()
}
