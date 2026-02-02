package migrate

import (
	"context"
	"database/sql"
	"fmt"
)

// Run executes all pending migrations from the plan.
// It is safe to call on every application startup - it only runs unapplied migrations.
//
// Migration names must follow the TIMESTAMP_name format (e.g., "20260111170656_create_users")
// and must be in strictly ascending lexicographic order (which equals timestamp order).
func Run(ctx context.Context, db *sql.DB, plan *MigrationPlan, dialect string) error {
	// Validate all migration names and ensure they're in order
	var prevName string
	for _, migration := range plan.Migrations {
		// Validate name format
		if err := ValidateMigrationName(migration.Name); err != nil {
			return fmt.Errorf("invalid migration: %w", err)
		}

		// Validate ordering (must be strictly ascending)
		if migration.Name <= prevName {
			return fmt.Errorf("migrations out of order: %q must come after %q", migration.Name, prevName)
		}
		prevName = migration.Name
	}

	// Ensure tracking table exists
	if err := EnsureTrackingTable(ctx, db, dialect); err != nil {
		return fmt.Errorf("failed to create tracking table: %w", err)
	}

	// Get already applied migrations (returns full names)
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a set of applied names for fast lookup
	appliedSet := make(map[string]bool)
	for _, name := range applied {
		appliedSet[name] = true
	}

	// Execute all migrations in the plan that haven't been applied
	for _, migration := range plan.Migrations {
		if appliedSet[migration.Name] {
			continue
		}

		// Get the SQL for this dialect
		var sqlStmt string
		switch dialect {
		case Postgres:
			sqlStmt = migration.Instructions.Postgres
		case MySQL:
			sqlStmt = migration.Instructions.MySQL
		case Sqlite:
			sqlStmt = migration.Instructions.Sqlite
		default:
			return fmt.Errorf("unsupported dialect: %s", dialect)
		}

		// Execute migration in a transaction
		if err := runMigrationInTransaction(ctx, db, dialect, migration.Name, sqlStmt); err != nil {
			return err
		}
	}

	return nil
}

// runMigrationInTransaction executes a single migration within a transaction.
// Both the SQL execution and the tracking record are within the same transaction.
func runMigrationInTransaction(ctx context.Context, db *sql.DB, dialect, name, sqlStmt string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction for migration %s: %w", name, err)
	}
	defer tx.Rollback() // no-op if committed

	// Execute the migration SQL
	if _, err := tx.ExecContext(ctx, sqlStmt); err != nil {
		return fmt.Errorf("failed to execute migration %s: %w", name, err)
	}

	// Extract version (timestamp) from the name for the version column
	version := name[:14]

	// Record the migration within the same transaction
	if err := RecordMigrationTx(ctx, tx, dialect, version, name); err != nil {
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %s: %w", name, err)
	}

	return nil
}

// DetectDialect attempts to detect the database dialect from a *sql.DB.
// It uses the driver name to determine the dialect.
func DetectDialect(db *sql.DB) (string, error) {
	// Get the driver name
	// This is a bit hacky, but it works for the common drivers
	var result string
	err := db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		// Try to detect from the error message or continue
	}

	// Try postgres-specific query
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err == nil {
		if len(version) > 0 && (version[0] == 'P' || version[0] == 'p') {
			return Postgres, nil
		}
	}

	// Try mysql-specific query
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err == nil {
		// MySQL version strings often contain "MySQL" or "MariaDB"
		return MySQL, nil
	}

	// Try sqlite-specific query
	err = db.QueryRow("SELECT sqlite_version()").Scan(&version)
	if err == nil {
		return Sqlite, nil
	}

	return "", fmt.Errorf("could not detect database dialect")
}
