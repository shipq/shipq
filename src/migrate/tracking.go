package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const trackingTableName = "_portsql_migrations"

// EnsureTrackingTable creates the _portsql_migrations table if it doesn't exist.
// The table uses `name` (full migration name like "20260111170700_create_users") as the
// primary key to allow multiple migrations with the same timestamp but different names.
func EnsureTrackingTable(ctx context.Context, db *sql.DB, dialect string) error {
	var createSQL string

	switch dialect {
	case Postgres:
		createSQL = `
			CREATE TABLE IF NOT EXISTS _portsql_migrations (
				name       VARCHAR(255) PRIMARY KEY,
				version    VARCHAR(14) NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
	case MySQL:
		createSQL = `
			CREATE TABLE IF NOT EXISTS _portsql_migrations (
				name       VARCHAR(255) PRIMARY KEY,
				version    VARCHAR(14) NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
	case Sqlite:
		createSQL = `
			CREATE TABLE IF NOT EXISTS _portsql_migrations (
				name       TEXT PRIMARY KEY,
				version    TEXT NOT NULL,
				applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`
	default:
		return fmt.Errorf("unsupported dialect: %s", dialect)
	}

	_, err := db.ExecContext(ctx, createSQL)
	return err
}

// GetAppliedMigrations returns the list of applied migration names, sorted by version then name.
// The name is the full migration identifier like "20260111170700_create_users".
func GetAppliedMigrations(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT name FROM _portsql_migrations ORDER BY version, name")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan migration name: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migrations: %w", err)
	}

	return names, nil
}

// RecordMigration inserts a migration into the tracking table.
// The name is the full migration identifier like "20260111170700_create_users".
// The version is just the timestamp portion for ordering.
func RecordMigration(ctx context.Context, db *sql.DB, dialect, version, name string) error {
	var insertSQL string
	var args []interface{}

	switch dialect {
	case Postgres:
		insertSQL = `INSERT INTO _portsql_migrations (name, version, applied_at) VALUES ($1, $2, $3)`
		args = []interface{}{name, version, time.Now()}
	case MySQL:
		insertSQL = `INSERT INTO _portsql_migrations (name, version, applied_at) VALUES (?, ?, ?)`
		args = []interface{}{name, version, time.Now()}
	case Sqlite:
		insertSQL = `INSERT INTO _portsql_migrations (name, version, applied_at) VALUES (?, ?, ?)`
		args = []interface{}{name, version, time.Now().Format(time.RFC3339)}
	default:
		return fmt.Errorf("unsupported dialect: %s", dialect)
	}

	_, err := db.ExecContext(ctx, insertSQL, args...)
	if err != nil {
		return fmt.Errorf("failed to record migration %s: %w", name, err)
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
