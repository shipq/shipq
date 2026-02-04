package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

// dbSetupCmd implements the "shipq db setup" command.
// It creates the database and configures shipq.ini.
func dbSetupCmd() {
	// Find and validate project root
	projectRoot, err := project.FindProjectRoot()
	if err != nil {
		cli.FatalErr("failed to find project root", err)
	}

	if err := project.ValidateProjectRoot(projectRoot); err != nil {
		cli.FatalErr("invalid project", err)
	}

	// Get DATABASE_URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		cli.Fatal("DATABASE_URL environment variable is required")
	}

	// Infer dialect from URL
	dialect, err := dburl.InferDialectFromDBUrl(databaseURL)
	if err != nil {
		cli.FatalErr("failed to determine database type", err)
	}

	// Validate localhost
	if !dburl.IsLocalhost(databaseURL) {
		cli.Fatal("DATABASE_URL must point to localhost for safety")
	}

	// Get project name for database names
	projectName := project.GetProjectName(projectRoot)

	// Handle each dialect
	var finalDatabaseURL string
	switch dialect {
	case dburl.DialectPostgres:
		finalDatabaseURL, err = setupPostgres(databaseURL, projectName)
	case dburl.DialectMySQL:
		finalDatabaseURL, err = setupMySQL(databaseURL, projectName)
	case dburl.DialectSQLite:
		finalDatabaseURL, err = setupSQLite(projectRoot, projectName)
	default:
		cli.Fatal(fmt.Sprintf("unsupported database dialect: %s", dialect))
	}

	if err != nil {
		cli.FatalErr("failed to set up database", err)
	}

	// Update shipq.ini
	shipqIniPath := filepath.Join(projectRoot, project.ShipqIniFile)
	iniFile, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	iniFile.Set("db", "database_url", finalDatabaseURL)

	if err := iniFile.WriteFile(shipqIniPath); err != nil {
		cli.FatalErr("failed to write shipq.ini", err)
	}

	cli.Success("Database setup complete")
	cli.Infof("  Database: %s", projectName)
	cli.Infof("  Test database: %s_test", projectName)
	cli.Infof("  Updated shipq.ini with database_url")
}

// setupPostgres creates PostgreSQL databases and returns the connection URL.
func setupPostgres(databaseURL, projectName string) (string, error) {
	// Connect to postgres database using pgx stdlib driver
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return "", fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Create main database
	dbName := projectName
	if err := createPostgresDB(ctx, db, dbName); err != nil {
		return "", err
	}
	cli.Successf("Created database: %s", dbName)

	// Create test database
	testDBName := projectName + "_test"
	if err := createPostgresDB(ctx, db, testDBName); err != nil {
		return "", err
	}
	cli.Successf("Created database: %s", testDBName)

	// Build final URL with database name
	finalURL, err := dburl.WithDatabaseName(databaseURL, dbName)
	if err != nil {
		return "", fmt.Errorf("failed to build database URL: %w", err)
	}

	return finalURL, nil
}

// createPostgresDB creates a PostgreSQL database if it doesn't exist.
func createPostgresDB(ctx context.Context, db *sql.DB, dbName string) error {
	// Check if database exists
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		cli.Infof("Database %s already exists", dbName)
		return nil
	}

	// Create database (can't use prepared statement for CREATE DATABASE)
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(dbName)))
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	return nil
}

// setupMySQL creates MySQL databases and returns the connection URL.
func setupMySQL(databaseURL, projectName string) (string, error) {
	// Parse the URL to get connection parameters
	// MySQL driver expects: user:password@tcp(host:port)/dbname
	dsn, err := mysqlURLToDSN(databaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse MySQL URL: %w", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return "", fmt.Errorf("failed to ping MySQL: %w", err)
	}

	// Create main database
	dbName := projectName
	if err := createMySQLDB(ctx, db, dbName); err != nil {
		return "", err
	}
	cli.Successf("Created database: %s", dbName)

	// Create test database
	testDBName := projectName + "_test"
	if err := createMySQLDB(ctx, db, testDBName); err != nil {
		return "", err
	}
	cli.Successf("Created database: %s", testDBName)

	// Build final URL with database name
	finalURL, err := dburl.WithDatabaseName(databaseURL, dbName)
	if err != nil {
		return "", fmt.Errorf("failed to build database URL: %w", err)
	}

	return finalURL, nil
}

// createMySQLDB creates a MySQL database if it doesn't exist.
func createMySQLDB(ctx context.Context, db *sql.DB, dbName string) error {
	// MySQL supports IF NOT EXISTS
	_, err := db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName))
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}
	return nil
}

// mysqlURLToDSN converts a mysql:// URL to a MySQL DSN.
func mysqlURLToDSN(mysqlURL string) (string, error) {
	// Parse mysql://user@host:port/dbname to user@tcp(host:port)/dbname
	importURL := mysqlURL
	if len(importURL) > 8 && importURL[:8] == "mysql://" {
		importURL = importURL[8:]
	}

	// Find @ separator
	atIdx := -1
	for i, c := range importURL {
		if c == '@' {
			atIdx = i
			break
		}
	}

	if atIdx == -1 {
		return "", fmt.Errorf("invalid MySQL URL: missing @ separator")
	}

	user := importURL[:atIdx]
	rest := importURL[atIdx+1:]

	// Find / separator for database
	slashIdx := -1
	for i, c := range rest {
		if c == '/' {
			slashIdx = i
			break
		}
	}

	var hostPort, dbName string
	if slashIdx == -1 {
		hostPort = rest
		dbName = ""
	} else {
		hostPort = rest[:slashIdx]
		dbName = rest[slashIdx+1:]
	}

	// Build DSN: user@tcp(host:port)/dbname
	return fmt.Sprintf("%s@tcp(%s)/%s", user, hostPort, dbName), nil
}

// setupSQLite ensures the SQLite database file exists and returns the URL.
func setupSQLite(projectRoot, projectName string) (string, error) {
	dataDir := filepath.Join(projectRoot, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	// Main database
	dbPath := filepath.Join(dataDir, projectName+".db")
	if err := touchFile(dbPath); err != nil {
		return "", err
	}
	cli.Successf("Created database file: %s", dbPath)

	// Test database
	testDBPath := filepath.Join(dataDir, projectName+"_test.db")
	if err := touchFile(testDBPath); err != nil {
		return "", err
	}
	cli.Successf("Created database file: %s", testDBPath)

	return dburl.BuildSQLiteURL(dbPath), nil
}

// touchFile creates an empty file if it doesn't exist.
func touchFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", path, err)
		}
		file.Close()
	}
	return nil
}

// quoteIdentifier quotes a PostgreSQL identifier to prevent SQL injection.
func quoteIdentifier(name string) string {
	// Simple quoting - in production, use a proper escaping function
	result := ""
	for _, c := range name {
		if c == '"' {
			result += "\"\""
		} else {
			result += string(c)
		}
	}
	return "\"" + result + "\""
}
