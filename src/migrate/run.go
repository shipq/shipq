package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// Run executes all pending migrations from the plan.
// It is safe to call on every application startup - it only runs unapplied migrations.
func Run(ctx context.Context, db *sql.DB, plan *MigrationPlan, dialect string) error {
	// Ensure tracking table exists
	if err := EnsureTrackingTable(ctx, db, dialect); err != nil {
		return fmt.Errorf("failed to create tracking table: %w", err)
	}

	// Get already applied migrations
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a set of applied versions for fast lookup
	appliedSet := make(map[string]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Group migrations by their version (extracted from name)
	// For now, we execute all migrations in the plan that haven't been applied
	// The "version" in the tracking table corresponds to the migration name
	for i, migration := range plan.Migrations {
		// Use the migration index as a simple version for now
		// In practice, migrations from files will have timestamp versions
		version := fmt.Sprintf("%014d", i)

		if appliedSet[version] {
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

		// Execute the migration
		if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
		}

		// Record the migration
		if err := RecordMigration(ctx, db, dialect, version, migration.Name); err != nil {
			return err
		}
	}

	return nil
}

// RunWithVersions executes migrations using provided version strings.
// This is used by the CLI when running migrations from files with timestamps.
func RunWithVersions(ctx context.Context, db *sql.DB, migrations []VersionedMigration, dialect string) error {
	// Ensure tracking table exists
	if err := EnsureTrackingTable(ctx, db, dialect); err != nil {
		return fmt.Errorf("failed to create tracking table: %w", err)
	}

	// Get already applied migrations
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a set of applied versions for fast lookup
	appliedSet := make(map[string]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// Execute each unapplied migration
	for _, m := range migrations {
		if appliedSet[m.Version] {
			continue
		}

		// Get the SQL for this dialect
		var sqlStmt string
		switch dialect {
		case Postgres:
			sqlStmt = m.Instructions.Postgres
		case MySQL:
			sqlStmt = m.Instructions.MySQL
		case Sqlite:
			sqlStmt = m.Instructions.Sqlite
		default:
			return fmt.Errorf("unsupported dialect: %s", dialect)
		}

		// Execute the migration
		if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", m.Name, err)
		}

		// Record the migration
		if err := RecordMigration(ctx, db, dialect, m.Version, m.Name); err != nil {
			return err
		}
	}

	return nil
}

// VersionedMigration is a migration with an explicit version string.
type VersionedMigration struct {
	Version      string
	Name         string
	Instructions MigrationInstructions
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
