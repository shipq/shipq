package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const trackingTableName = "_portsql_migrations"

// EnsureTrackingTable creates the _portsql_migrations table if it doesn't exist.
func EnsureTrackingTable(ctx context.Context, db *sql.DB, dialect string) error {
	var createSQL string

	switch dialect {
	case Postgres:
		createSQL = `
			CREATE TABLE IF NOT EXISTS _portsql_migrations (
				version    VARCHAR(14) PRIMARY KEY,
				name       VARCHAR(255) NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
	case MySQL:
		createSQL = `
			CREATE TABLE IF NOT EXISTS _portsql_migrations (
				version    VARCHAR(14) PRIMARY KEY,
				name       VARCHAR(255) NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
	case Sqlite:
		createSQL = `
			CREATE TABLE IF NOT EXISTS _portsql_migrations (
				version    TEXT PRIMARY KEY,
				name       TEXT NOT NULL,
				applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
	default:
		return fmt.Errorf("unsupported dialect: %s", dialect)
	}

	_, err := db.ExecContext(ctx, createSQL)
	return err
}

// GetAppliedMigrations returns the list of applied migration versions, sorted by version.
func GetAppliedMigrations(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT version FROM _portsql_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		versions = append(versions, version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migrations: %w", err)
	}

	return versions, nil
}

// RecordMigration inserts a migration into the tracking table.
func RecordMigration(ctx context.Context, db *sql.DB, dialect, version, name string) error {
	var insertSQL string
	var args []interface{}

	switch dialect {
	case Postgres:
		insertSQL = `INSERT INTO _portsql_migrations (version, name, applied_at) VALUES ($1, $2, $3)`
		args = []interface{}{version, name, time.Now()}
	case MySQL:
		insertSQL = `INSERT INTO _portsql_migrations (version, name, applied_at) VALUES (?, ?, ?)`
		args = []interface{}{version, name, time.Now()}
	case Sqlite:
		insertSQL = `INSERT INTO _portsql_migrations (version, name, applied_at) VALUES (?, ?, ?)`
		args = []interface{}{version, name, time.Now().Format(time.RFC3339)}
	default:
		return fmt.Errorf("unsupported dialect: %s", dialect)
	}

	_, err := db.ExecContext(ctx, insertSQL, args...)
	if err != nil {
		return fmt.Errorf("failed to record migration %s: %w", version, err)
	}

	return nil
}

// GetAllTables returns the list of all table names in the database.
func GetAllTables(ctx context.Context, db *sql.DB, dialect string) ([]string, error) {
	var querySQL string

	switch dialect {
	case Postgres:
		querySQL = `
			SELECT tablename FROM pg_tables 
			WHERE schemaname = 'public'
			ORDER BY tablename`
	case MySQL:
		querySQL = `SHOW TABLES`
	case Sqlite:
		querySQL = `
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name NOT LIKE 'sqlite_%'
			ORDER BY name`
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}

	rows, err := db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	return tables, nil
}

// DropAllTables drops all tables in the database including the tracking table.
func DropAllTables(ctx context.Context, db *sql.DB, dialect string) error {
	tables, err := GetAllTables(ctx, db, dialect)
	if err != nil {
		return err
	}

	for _, table := range tables {
		var dropSQL string
		switch dialect {
		case Postgres:
			dropSQL = fmt.Sprintf(`DROP TABLE IF EXISTS %q CASCADE`, table)
		case MySQL:
			dropSQL = fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)
		case Sqlite:
			dropSQL = fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, table)
		default:
			return fmt.Errorf("unsupported dialect: %s", dialect)
		}

		if _, err := db.ExecContext(ctx, dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	return nil
}
