package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a simple plan
	plan := NewPlan()
	plan.Migrations = []Migration{
		{
			Name: "create_users",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
			},
		},
		{
			Name: "create_posts",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT)`,
			},
		},
	}

	// Run migrations
	if err := Run(ctx, db, plan, Sqlite); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify tables were created
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}

	// Should have users, posts, and _portsql_migrations
	if len(tables) != 3 {
		t.Errorf("expected 3 tables, got %d: %v", len(tables), tables)
	}

	// Run again - should be idempotent
	if err := Run(ctx, db, plan, Sqlite); err != nil {
		t.Fatalf("Run (second call) failed: %v", err)
	}

	// Still should have 3 tables
	tables, err = GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}
	if len(tables) != 3 {
		t.Errorf("expected 3 tables after second run, got %d", len(tables))
	}
}

func TestRunWithVersions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	migrations := []VersionedMigration{
		{
			Version: "20260111153000",
			Name:    "20260111153000_create_users",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
			},
		},
		{
			Version: "20260111160000",
			Name:    "20260111160000_create_posts",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT)`,
			},
		},
	}

	// Run migrations
	if err := RunWithVersions(ctx, db, migrations, Sqlite); err != nil {
		t.Fatalf("RunWithVersions failed: %v", err)
	}

	// Verify migrations were recorded with correct versions
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(applied))
	}

	if applied[0] != "20260111153000" || applied[1] != "20260111160000" {
		t.Errorf("unexpected versions: %v", applied)
	}

	// Run again - should skip already applied
	if err := RunWithVersions(ctx, db, migrations, Sqlite); err != nil {
		t.Fatalf("RunWithVersions (second call) failed: %v", err)
	}

	// Add a new migration
	migrations = append(migrations, VersionedMigration{
		Version: "20260111170000",
		Name:    "20260111170000_create_comments",
		Instructions: MigrationInstructions{
			Sqlite: `CREATE TABLE comments (id INTEGER PRIMARY KEY, body TEXT)`,
		},
	})

	// Run again - should only run the new one
	if err := RunWithVersions(ctx, db, migrations, Sqlite); err != nil {
		t.Fatalf("RunWithVersions (third call) failed: %v", err)
	}

	applied, err = GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 3 {
		t.Fatalf("expected 3 applied migrations, got %d", len(applied))
	}
}

func TestRunMigrationOrder(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Provide migrations out of order
	migrations := []VersionedMigration{
		{
			Version: "20260111160000",
			Name:    "20260111160000_second",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE second_table (id INTEGER PRIMARY KEY)`,
			},
		},
		{
			Version: "20260111153000",
			Name:    "20260111153000_first",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE first_table (id INTEGER PRIMARY KEY)`,
			},
		},
	}

	// Run migrations - should execute in version order
	if err := RunWithVersions(ctx, db, migrations, Sqlite); err != nil {
		t.Fatalf("RunWithVersions failed: %v", err)
	}

	// Verify they were recorded in order
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(applied))
	}

	// Should be sorted
	if applied[0] != "20260111153000" || applied[1] != "20260111160000" {
		t.Errorf("expected sorted order, got: %v", applied)
	}
}
