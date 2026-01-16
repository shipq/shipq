package migrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// Order Validation Tests (TDD - these should fail initially)
// =============================================================================

func TestRunRejectsOutOfOrderMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a plan with migrations out of order (second timestamp comes before first)
	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260111170700_second",
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE second_table (id INTEGER PRIMARY KEY)`,
				},
			},
			{
				Name: "20260111170656_first", // This comes AFTER but has EARLIER timestamp
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE first_table (id INTEGER PRIMARY KEY)`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err == nil {
		t.Error("Run() should reject out-of-order migrations")
		return
	}

	if !strings.Contains(strings.ToLower(err.Error()), "order") {
		t.Errorf("error should mention 'order', got: %v", err)
	}
}

func TestRunRejectsDuplicateTimestamps(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a plan with duplicate migration names
	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260111170656_create_users",
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY)`,
				},
			},
			{
				Name: "20260111170656_create_users", // Duplicate!
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE users2 (id INTEGER PRIMARY KEY)`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err == nil {
		t.Error("Run() should reject duplicate migration names")
		return
	}

	if !strings.Contains(strings.ToLower(err.Error()), "order") && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Errorf("error should mention 'order' or 'duplicate', got: %v", err)
	}
}

func TestRunRejectsInvalidMigrationNames(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a plan with invalid migration name (no timestamp prefix)
	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "create_users", // Invalid - no timestamp prefix
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY)`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err == nil {
		t.Error("Run() should reject migrations without timestamp prefix")
		return
	}

	if !strings.Contains(strings.ToLower(err.Error()), "timestamp") {
		t.Errorf("error should mention 'timestamp', got: %v", err)
	}
}

// =============================================================================
// Transaction Rollback Tests (TDD - these should fail initially)
// =============================================================================

func TestRunRollsBackFailedMigration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a plan where the second migration will fail
	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260111170656_create_users",
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
				},
			},
			{
				Name: "20260111170700_bad_migration",
				Instructions: MigrationInstructions{
					Sqlite: `THIS IS INVALID SQL THAT WILL FAIL`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err == nil {
		t.Fatal("Run() should fail on invalid SQL")
	}

	// The first migration should have succeeded and been recorded
	// (we do per-migration transactions, not all-or-nothing)
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	// First migration should be recorded
	if len(applied) != 1 {
		t.Errorf("expected 1 applied migration (first one succeeded), got %d", len(applied))
	}

	if len(applied) > 0 && applied[0] != "20260111170656_create_users" {
		t.Errorf("expected first migration to be recorded, got: %v", applied)
	}

	// The bad migration should NOT be recorded
	for _, name := range applied {
		if strings.Contains(name, "bad_migration") {
			t.Error("bad_migration should not be recorded")
		}
	}

	// Verify first table was created
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}

	foundUsers := false
	for _, table := range tables {
		if table == "users" {
			foundUsers = true
		}
	}
	if !foundUsers {
		t.Error("users table should exist (first migration succeeded)")
	}
}

func TestRunTransactionRollbackOnSQLError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a migration with multiple SQL statements where the second fails
	// The whole migration should be rolled back
	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260111170656_multi_statement",
				Instructions: MigrationInstructions{
					// First statement succeeds, second fails - both should be rolled back
					Sqlite: `CREATE TABLE test1 (id INTEGER PRIMARY KEY);
CREATE TABLE test1 (id INTEGER PRIMARY KEY)`, // Duplicate table name - will fail
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err == nil {
		t.Fatal("Run() should fail on duplicate table creation")
	}

	// The migration should NOT be recorded (rolled back)
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 0 {
		t.Errorf("no migrations should be recorded after rollback, got: %v", applied)
	}

	// The test1 table should NOT exist (rolled back)
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}

	for _, table := range tables {
		if table == "test1" {
			t.Error("test1 table should not exist after rollback")
		}
	}
}

func TestRunMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a simple plan with timestamped migration names
	plan := NewPlan()
	plan.Migrations = []Migration{
		{
			Name: "20260111153000_create_users",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
			},
		},
		{
			Name: "20260111160000_create_posts",
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

// TestRunWithAddedMigration tests adding a new migration to an existing plan
func TestRunWithAddedMigration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Initial plan with two migrations
	plan := NewPlan()
	plan.Migrations = []Migration{
		{
			Name: "20260111153000_create_users",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
			},
		},
		{
			Name: "20260111160000_create_posts",
			Instructions: MigrationInstructions{
				Sqlite: `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT)`,
			},
		},
	}

	// Run migrations
	if err := Run(ctx, db, plan, Sqlite); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify migrations were recorded
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(applied))
	}

	// Add a new migration to the plan
	plan.Migrations = append(plan.Migrations, Migration{
		Name: "20260111170000_create_comments",
		Instructions: MigrationInstructions{
			Sqlite: `CREATE TABLE comments (id INTEGER PRIMARY KEY, body TEXT)`,
		},
	})

	// Run again - should only run the new one
	if err := Run(ctx, db, plan, Sqlite); err != nil {
		t.Fatalf("Run (second call) failed: %v", err)
	}

	applied, err = GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 3 {
		t.Fatalf("expected 3 applied migrations, got %d", len(applied))
	}
}
