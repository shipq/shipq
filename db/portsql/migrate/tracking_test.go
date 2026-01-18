package migrate

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
	names, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 applied migrations, got %d", len(names))
	}

	// Test RecordMigration - now uses full name as unique key
	if err := RecordMigration(ctx, db, Sqlite, "20260111153000", "20260111153000_create_users"); err != nil {
		t.Fatalf("RecordMigration failed: %v", err)
	}

	// Verify it was recorded
	names, err = GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("expected 1 applied migration, got %d", len(names))
	}
	if names[0] != "20260111153000_create_users" {
		t.Errorf("expected name '20260111153000_create_users', got %q", names[0])
	}

	// Add another migration
	if err := RecordMigration(ctx, db, Sqlite, "20260111160000", "20260111160000_create_posts"); err != nil {
		t.Fatalf("RecordMigration failed: %v", err)
	}

	// Verify both are returned in order (sorted by version, then name)
	names, err = GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(names))
	}
	if names[0] != "20260111153000_create_users" || names[1] != "20260111160000_create_posts" {
		t.Errorf("unexpected migration order: %v", names)
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

func TestSameTimestampDifferentNames(t *testing.T) {
	// Test that two migrations with the same timestamp but different names
	// can both be recorded without error. This simulates migrations created
	// within the same second.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create tracking table
	if err := EnsureTrackingTable(ctx, db, Sqlite); err != nil {
		t.Fatalf("EnsureTrackingTable failed: %v", err)
	}

	// Record first migration
	if err := RecordMigration(ctx, db, Sqlite, "20260111170700", "20260111170700_create_tags"); err != nil {
		t.Fatalf("RecordMigration (tags) failed: %v", err)
	}

	// Record second migration with SAME timestamp but different name
	// This should NOT fail - the name is used as the unique key now
	if err := RecordMigration(ctx, db, Sqlite, "20260111170700", "20260111170700_create_users"); err != nil {
		t.Fatalf("RecordMigration (users) failed - same timestamp should be allowed: %v", err)
	}

	// Verify both are recorded
	names, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(names))
	}

	// Migrations should be returned sorted by version (timestamp) then name
	// Both have same version, so they're sorted by name
	expected := []string{"20260111170700_create_tags", "20260111170700_create_users"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("expected migration %d to be %q, got %q", i, expected[i], name)
		}
	}
}

// =============================================================================
// UTC Timestamp Tests
// =============================================================================

func TestRecordMigration_AlwaysUTC(t *testing.T) {
	// Test that recorded timestamps are always in UTC, regardless of local timezone.
	// This ensures cross-database consistency.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create tracking table
	if err := EnsureTrackingTable(ctx, db, Sqlite); err != nil {
		t.Fatalf("EnsureTrackingTable failed: %v", err)
	}

	// Record time before migration (truncate to second since RFC3339 loses sub-second)
	beforeUTC := time.Now().UTC().Truncate(time.Second)

	// Record a migration
	if err := RecordMigration(ctx, db, Sqlite, "20260115120000", "20260115120000_test_utc"); err != nil {
		t.Fatalf("RecordMigration failed: %v", err)
	}

	// Record time after migration (add 1 second buffer for truncation)
	afterUTC := time.Now().UTC().Add(time.Second).Truncate(time.Second)

	// Query the applied_at timestamp directly
	var appliedAtStr string
	err = db.QueryRowContext(ctx,
		"SELECT applied_at FROM _portsql_migrations WHERE name = ?",
		"20260115120000_test_utc",
	).Scan(&appliedAtStr)
	if err != nil {
		t.Fatalf("failed to query applied_at: %v", err)
	}

	// Parse the timestamp (SQLite stores as RFC3339 text)
	appliedAt, err := time.Parse(time.RFC3339, appliedAtStr)
	if err != nil {
		t.Fatalf("failed to parse applied_at timestamp %q: %v", appliedAtStr, err)
	}

	// Verify the timestamp is in UTC (RFC3339 with Z suffix or +00:00)
	if appliedAt.Location() != time.UTC {
		t.Errorf("expected applied_at to be in UTC, got location %v", appliedAt.Location())
	}

	// Verify timestamp is within expected range (between before and after, accounting for truncation)
	if appliedAt.Before(beforeUTC) || appliedAt.After(afterUTC) {
		t.Errorf("applied_at %v is not between %v and %v", appliedAt, beforeUTC, afterUTC)
	}

	// Additionally verify the stored string ends with Z (UTC indicator)
	if appliedAtStr[len(appliedAtStr)-1] != 'Z' {
		t.Errorf("expected timestamp to end with 'Z' (UTC), got %q", appliedAtStr)
	}
}

func TestRecordMigrationTx_AlwaysUTC(t *testing.T) {
	// Test that RecordMigrationTx also uses UTC timestamps
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create tracking table
	if err := EnsureTrackingTable(ctx, db, Sqlite); err != nil {
		t.Fatalf("EnsureTrackingTable failed: %v", err)
	}

	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Record time before migration (truncate to second since RFC3339 loses sub-second)
	beforeUTC := time.Now().UTC().Truncate(time.Second)

	// Record a migration within transaction
	if err := RecordMigrationTx(ctx, tx, Sqlite, "20260115130000", "20260115130000_test_utc_tx"); err != nil {
		tx.Rollback()
		t.Fatalf("RecordMigrationTx failed: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Record time after migration (add 1 second buffer for truncation)
	afterUTC := time.Now().UTC().Add(time.Second).Truncate(time.Second)

	// Query the applied_at timestamp
	var appliedAtStr string
	err = db.QueryRowContext(ctx,
		"SELECT applied_at FROM _portsql_migrations WHERE name = ?",
		"20260115130000_test_utc_tx",
	).Scan(&appliedAtStr)
	if err != nil {
		t.Fatalf("failed to query applied_at: %v", err)
	}

	// Parse and verify UTC
	appliedAt, err := time.Parse(time.RFC3339, appliedAtStr)
	if err != nil {
		t.Fatalf("failed to parse applied_at timestamp %q: %v", appliedAtStr, err)
	}

	if appliedAt.Location() != time.UTC {
		t.Errorf("expected applied_at to be in UTC, got location %v", appliedAt.Location())
	}

	if appliedAt.Before(beforeUTC) || appliedAt.After(afterUTC) {
		t.Errorf("applied_at %v is not between %v and %v", appliedAt, beforeUTC, afterUTC)
	}
}

// =============================================================================
// Identifier Escaping Tests
// =============================================================================

func TestEscapeIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dialect  string
		expected string
	}{
		// Postgres (uses double quotes)
		{name: "postgres simple", input: "users", dialect: Postgres, expected: `"users"`},
		{name: "postgres with space", input: "user data", dialect: Postgres, expected: `"user data"`},
		{name: "postgres with double quote", input: `user"name`, dialect: Postgres, expected: `"user""name"`},
		{name: "postgres with multiple quotes", input: `a"b"c`, dialect: Postgres, expected: `"a""b""c"`},

		// MySQL (uses backticks)
		{name: "mysql simple", input: "users", dialect: MySQL, expected: "`users`"},
		{name: "mysql with space", input: "user data", dialect: MySQL, expected: "`user data`"},
		{name: "mysql with backtick", input: "user`name", dialect: MySQL, expected: "`user``name`"},
		{name: "mysql with multiple backticks", input: "a`b`c", dialect: MySQL, expected: "`a``b``c`"},

		// SQLite (uses double quotes like Postgres)
		{name: "sqlite simple", input: "users", dialect: Sqlite, expected: `"users"`},
		{name: "sqlite with space", input: "user data", dialect: Sqlite, expected: `"user data"`},
		{name: "sqlite with double quote", input: `user"name`, dialect: Sqlite, expected: `"user""name"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeIdentifier(tt.input, tt.dialect)
			if got != tt.expected {
				t.Errorf("escapeIdentifier(%q, %q) = %q, want %q", tt.input, tt.dialect, got, tt.expected)
			}
		})
	}
}

func TestTimestampFormat_SQLite_RFC3339UTC(t *testing.T) {
	// Verify that SQLite timestamps are stored in RFC3339 format with UTC timezone
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create tracking table
	if err := EnsureTrackingTable(ctx, db, Sqlite); err != nil {
		t.Fatalf("EnsureTrackingTable failed: %v", err)
	}

	// Record multiple migrations
	migrations := []string{
		"20260115140000_first",
		"20260115140001_second",
		"20260115140002_third",
	}

	for _, name := range migrations {
		version := name[:14]
		if err := RecordMigration(ctx, db, Sqlite, version, name); err != nil {
			t.Fatalf("RecordMigration(%s) failed: %v", name, err)
		}
	}

	// Query all timestamps
	rows, err := db.QueryContext(ctx, "SELECT name, applied_at FROM _portsql_migrations")
	if err != nil {
		t.Fatalf("failed to query migrations: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, appliedAtStr string
		if err := rows.Scan(&name, &appliedAtStr); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}

		// Verify format is valid RFC3339
		parsedTime, err := time.Parse(time.RFC3339, appliedAtStr)
		if err != nil {
			t.Errorf("migration %s: timestamp %q is not valid RFC3339: %v", name, appliedAtStr, err)
			continue
		}

		// Verify it's UTC (ends with Z or has +00:00 offset)
		if parsedTime.Location() != time.UTC {
			t.Errorf("migration %s: expected UTC, got location %v (timestamp: %s)",
				name, parsedTime.Location(), appliedAtStr)
		}

		// Verify the string representation ends with 'Z'
		if appliedAtStr[len(appliedAtStr)-1] != 'Z' {
			t.Errorf("migration %s: expected timestamp to end with 'Z', got %q", name, appliedAtStr)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error iterating rows: %v", err)
	}
}
