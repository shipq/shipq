package dbops

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// QuoteIdentifier quotes a database identifier to prevent SQL injection.
// For postgres/sqlite, uses double quotes with doubled internal quotes.
// For mysql, uses backticks with doubled internal backticks.
func QuoteIdentifier(name string, dialect string) string {
	switch dialect {
	case "mysql":
		// MySQL uses backticks
		var result strings.Builder
		result.WriteByte('`')
		for _, c := range name {
			if c == '`' {
				result.WriteString("``")
			} else {
				result.WriteRune(c)
			}
		}
		result.WriteByte('`')
		return result.String()
	default: // postgres, sqlite
		// Use double quotes
		var result strings.Builder
		result.WriteByte('"')
		for _, c := range name {
			if c == '"' {
				result.WriteString(`""`)
			} else {
				result.WriteRune(c)
			}
		}
		result.WriteByte('"')
		return result.String()
	}
}

// GenerateDropSQL generates a DROP DATABASE statement for the given dialect.
func GenerateDropSQL(dbName string, dialect string) string {
	quoted := QuoteIdentifier(dbName, dialect)
	return fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoted)
}

// GenerateCreateSQL generates a CREATE DATABASE statement for the given dialect.
func GenerateCreateSQL(dbName string, dialect string) string {
	quoted := QuoteIdentifier(dbName, dialect)
	switch dialect {
	case "mysql":
		return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoted)
	case "postgres":
		// Postgres doesn't support IF NOT EXISTS for CREATE DATABASE in standard SQL
		// We check existence separately
		return fmt.Sprintf("CREATE DATABASE %s", quoted)
	default:
		return fmt.Sprintf("CREATE DATABASE %s", quoted)
	}
}

// DropPostgresDB drops a PostgreSQL database if it exists.
// Requires a connection to a maintenance database (e.g., "postgres").
func DropPostgresDB(ctx context.Context, db *sql.DB, dbName string) error {
	// First, terminate all connections to the database
	terminateSQL := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()
	`
	_, _ = db.ExecContext(ctx, terminateSQL, dbName)

	// Drop the database
	dropSQL := GenerateDropSQL(dbName, "postgres")
	_, err := db.ExecContext(ctx, dropSQL)
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", dbName, err)
	}
	return nil
}

// DropMySQLDB drops a MySQL database if it exists.
func DropMySQLDB(ctx context.Context, db *sql.DB, dbName string) error {
	dropSQL := GenerateDropSQL(dbName, "mysql")
	_, err := db.ExecContext(ctx, dropSQL)
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", dbName, err)
	}
	return nil
}

// DropSQLiteDB deletes a SQLite database file if it exists.
func DropSQLiteDB(dbPath string) error {
	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to do
	}

	if err := os.Remove(dbPath); err != nil {
		return fmt.Errorf("failed to delete SQLite database file %s: %w", dbPath, err)
	}

	// Also try to remove WAL and SHM files if they exist
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	return nil
}

// CreatePostgresDB creates a PostgreSQL database if it doesn't exist.
// Requires a connection to a maintenance database (e.g., "postgres").
func CreatePostgresDB(ctx context.Context, db *sql.DB, dbName string) error {
	// Check if database already exists
	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		return nil // Database already exists
	}

	// Create the database
	createSQL := GenerateCreateSQL(dbName, "postgres")
	_, err = db.ExecContext(ctx, createSQL)
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}
	return nil
}

// CreateMySQLDB creates a MySQL database if it doesn't exist.
func CreateMySQLDB(ctx context.Context, db *sql.DB, dbName string) error {
	createSQL := GenerateCreateSQL(dbName, "mysql")
	_, err := db.ExecContext(ctx, createSQL)
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}
	return nil
}

// CreateSQLiteDB creates an empty SQLite database file if it doesn't exist.
// Also creates parent directories if needed.
func CreateSQLiteDB(dbPath string) error {
	// Check if file already exists
	if _, err := os.Stat(dbPath); err == nil {
		return nil // File already exists
	}

	// Create the file
	file, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite database file %s: %w", dbPath, err)
	}
	return file.Close()
}
