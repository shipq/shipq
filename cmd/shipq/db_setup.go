package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/dbops"
	"github.com/shipq/shipq/project"
)

// Default connection URLs for local database servers
const (
	defaultMySQLURL    = "mysql://root@localhost:3306/"
	defaultPostgresURL = "postgres://postgres@localhost:5432/postgres"
)

// commandExists checks if a command is available on the system PATH.
// This is a variable so it can be overridden in tests.
var commandExists = func(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// detectDatabaseDialect determines which database to use based on what's available.
// Priority order: MySQL -> Postgres -> SQLite
func detectDatabaseDialect() string {
	if commandExists("mysqld") {
		return dburl.DialectMySQL
	}
	if commandExists("postgres") {
		return dburl.DialectPostgres
	}
	return dburl.DialectSQLite
}

// inferDatabaseURL returns a default database URL based on detected dialect and project name.
func inferDatabaseURL(projectRoot, projectName string) (string, string) {
	dialect := detectDatabaseDialect()

	switch dialect {
	case dburl.DialectMySQL:
		cli.Infof("Detected mysqld on PATH, using MySQL")
		return defaultMySQLURL, dialect
	case dburl.DialectPostgres:
		cli.Infof("Detected postgres on PATH, using PostgreSQL")
		return defaultPostgresURL, dialect
	default:
		cli.Infof("No MySQL or PostgreSQL found, using SQLite")
		// For SQLite, we build the full path immediately
		dataDir := filepath.Join(projectRoot, ".shipq", "data")
		dbPath := filepath.Join(dataDir, projectName+".db")
		return dburl.BuildSQLiteURL(dbPath), dburl.DialectSQLite
	}
}

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

	// Get project name for database names
	projectName := project.GetProjectName(projectRoot)

	// Get DATABASE_URL from environment, or infer a default
	databaseURL := os.Getenv("DATABASE_URL")
	var dialect string

	if databaseURL == "" {
		databaseURL, dialect = inferDatabaseURL(projectRoot, projectName)
	} else {
		// Infer dialect from provided URL
		dialect, err = dburl.InferDialectFromDBUrl(databaseURL)
		if err != nil {
			cli.FatalErr("failed to determine database type", err)
		}
	}

	// Validate localhost (skip for SQLite since it's always local)
	if dialect != dburl.DialectSQLite && !dburl.IsLocalhost(databaseURL) {
		cli.Fatal("DATABASE_URL must point to localhost for safety")
	}

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
	// Open maintenance connection to postgres database
	db, err := dbops.OpenMaintenanceDB(databaseURL, "postgres")
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctx := context.Background()

	// Create main database
	dbName := projectName
	if err := dbops.CreatePostgresDB(ctx, db, dbName); err != nil {
		return "", err
	}
	cli.Successf("Created database: %s", dbName)

	// Create test database
	testDBName := projectName + "_test"
	if err := dbops.CreatePostgresDB(ctx, db, testDBName); err != nil {
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

// setupMySQL creates MySQL databases and returns the connection URL.
func setupMySQL(databaseURL, projectName string) (string, error) {
	// Open maintenance connection
	db, err := dbops.OpenMaintenanceDB(databaseURL, "mysql")
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctx := context.Background()

	// Create main database
	dbName := projectName
	if err := dbops.CreateMySQLDB(ctx, db, dbName); err != nil {
		return "", err
	}
	cli.Successf("Created database: %s", dbName)

	// Create test database
	testDBName := projectName + "_test"
	if err := dbops.CreateMySQLDB(ctx, db, testDBName); err != nil {
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

// setupSQLite ensures the SQLite database file exists and returns the URL.
func setupSQLite(projectRoot, projectName string) (string, error) {
	dataDir := filepath.Join(projectRoot, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	// Main database
	dbPath := filepath.Join(dataDir, projectName+".db")
	if err := dbops.CreateSQLiteDB(dbPath); err != nil {
		return "", err
	}
	cli.Successf("Created database file: %s", dbPath)

	// Test database
	testDBPath := filepath.Join(dataDir, projectName+"_test.db")
	if err := dbops.CreateSQLiteDB(testDBPath); err != nil {
		return "", err
	}
	cli.Successf("Created database file: %s", testDBPath)

	return dburl.BuildSQLiteURL(dbPath), nil
}
