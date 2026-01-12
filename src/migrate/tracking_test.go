package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestTrackingTable(t *testing.T) {
	// Use SQLite for unit testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test EnsureTrackingTable
	if err := EnsureTrackingTable(ctx, db, Sqlite); err != nil {
		t.Fatalf("EnsureTrackingTable failed: %v", err)
	}

	// Calling it again should be idempotent
	if err := EnsureTrackingTable(ctx, db, Sqlite); err != nil {
		t.Fatalf("EnsureTrackingTable (second call) failed: %v", err)
	}

	// Test GetAppliedMigrations (should be empty)
	versions, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 applied migrations, got %d", len(versions))
	}

	// Test RecordMigration
	if err := RecordMigration(ctx, db, Sqlite, "20260111153000", "create_users"); err != nil {
		t.Fatalf("RecordMigration failed: %v", err)
	}

	// Verify it was recorded
	versions, err = GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 applied migration, got %d", len(versions))
	}
	if versions[0] != "20260111153000" {
		t.Errorf("expected version '20260111153000', got %q", versions[0])
	}

	// Add another migration
	if err := RecordMigration(ctx, db, Sqlite, "20260111160000", "create_posts"); err != nil {
		t.Fatalf("RecordMigration failed: %v", err)
	}

	// Verify both are returned in order
	versions, err = GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(versions))
	}
	if versions[0] != "20260111153000" || versions[1] != "20260111160000" {
		t.Errorf("unexpected migration order: %v", versions)
	}
}

func TestGetAllTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create some tables
	_, err = db.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE posts (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	// Get all tables
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}

	// Tables should be sorted
	if tables[0] != "posts" || tables[1] != "users" {
		t.Errorf("unexpected table order: %v", tables)
	}
}

func TestDropAllTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create some tables
	_, err = db.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE posts (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	// Drop all tables
	if err := DropAllTables(ctx, db, Sqlite); err != nil {
		t.Fatalf("DropAllTables failed: %v", err)
	}

	// Verify tables are gone
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("expected 0 tables after drop, got %d", len(tables))
	}
}

func TestUnsupportedDialect(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test unsupported dialect
	if err := EnsureTrackingTable(ctx, db, "unsupported"); err == nil {
		t.Error("expected error for unsupported dialect")
	}

	if err := RecordMigration(ctx, db, "unsupported", "123", "test"); err == nil {
		t.Error("expected error for unsupported dialect")
	}

	if _, err := GetAllTables(ctx, db, "unsupported"); err == nil {
		t.Error("expected error for unsupported dialect")
	}

	if err := DropAllTables(ctx, db, "unsupported"); err == nil {
		t.Error("expected error for unsupported dialect")
	}
}
