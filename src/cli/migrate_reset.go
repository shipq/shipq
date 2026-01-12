package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/portsql/portsql/src/migrate"
)

// MigrateReset drops all tables and re-runs all migrations from scratch.
// This command only works on localhost databases for safety.
func MigrateReset(ctx context.Context, config *Config) error {
	// Validate config
	if config.Database.URL == "" {
		return fmt.Errorf("database URL not configured (set DATABASE_URL or add to portsql.ini)")
	}

	// Safety check: only allow on localhost
	if !IsLocalhostURL(config.Database.URL) {
		return fmt.Errorf(`migrate reset is not allowed on remote databases.
Host is not localhost.
This safety check prevents accidental data loss in production.

If you really need to reset a remote database, do it manually:
1. Drop all tables using your database client
2. Run 'portsql migrate up' to recreate them`)
	}

	// Parse dialect
	dialect := ParseDialect(config.Database.URL)
	if dialect == "" {
		return fmt.Errorf("unsupported database URL scheme: %s", config.Database.URL)
	}

	// Open database connection
	db, err := openDatabase(config.Database.URL, dialect)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	fmt.Println("Dropping all tables...")

	// Drop all tables
	if err := migrate.DropAllTables(ctx, db, dialect); err != nil {
		return fmt.Errorf("failed to drop tables: %w", err)
	}

	// Remove generated files
	schemaPath := filepath.Join(config.Paths.Migrations, "schema.json")
	if err := os.Remove(schemaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove schema.json: %w", err)
	}

	runnerPath := filepath.Join(config.Paths.Migrations, "runner.go")
	if err := os.Remove(runnerPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove runner.go: %w", err)
	}

	schemaTypesPath := filepath.Join(config.Paths.Schematypes, "tables.go")
	if err := os.Remove(schemaTypesPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove tables.go: %w", err)
	}

	fmt.Println("All tables dropped.")
	fmt.Println("Re-running migrations...")

	// Re-run all migrations
	return MigrateUp(ctx, config)
}
