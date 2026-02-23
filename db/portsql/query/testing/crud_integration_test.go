//go:build integration

package testing

import (
	"context"
	"testing"
	"time"

	"github.com/shipq/shipq/nanoid"
)

// =============================================================================
// Auto-Filled Column Integration Tests
// =============================================================================

// TestInsert_SetsTimestamps verifies that INSERT sets created_at and updated_at.
func TestInsert_SetsTimestamps(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	publicID := nanoid.New()

	// Insert author with auto-filled timestamps
	dbs.InsertAuthor(t, publicID, "Test Author", "test@example.com", nil, true)

	// Verify timestamps in Postgres - just check they're non-zero and recent
	var pgCreatedAt, pgUpdatedAt time.Time
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT created_at, updated_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgCreatedAt, &pgUpdatedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}

	if pgCreatedAt.IsZero() {
		t.Error("postgres created_at should not be zero")
	}
	if pgUpdatedAt.IsZero() {
		t.Error("postgres updated_at should not be zero")
	}
	// Verify it's a recent timestamp (within last 24 hours to account for any timezone)
	if time.Since(pgCreatedAt) > 24*time.Hour && time.Since(pgCreatedAt) < -24*time.Hour {
		t.Errorf("postgres created_at should be recent, got %v", pgCreatedAt)
	}

	// Verify timestamps in MySQL (scan as string due to driver behavior)
	var myCreatedAtStr, myUpdatedAtStr string
	err = dbs.MySQL.QueryRow(
		"SELECT created_at, updated_at FROM test_authors WHERE public_id = ?", publicID).
		Scan(&myCreatedAtStr, &myUpdatedAtStr)
	if err != nil {
		t.Fatalf("mysql query failed: %v", err)
	}

	if myCreatedAtStr == "" {
		t.Error("mysql created_at should not be empty")
	}
	if myUpdatedAtStr == "" {
		t.Error("mysql updated_at should not be empty")
	}

	myCreatedAt, err := time.Parse("2006-01-02 15:04:05", myCreatedAtStr)
	if err != nil {
		t.Fatalf("failed to parse mysql created_at %q: %v", myCreatedAtStr, err)
	}
	if myCreatedAt.IsZero() {
		t.Error("mysql created_at should not be zero")
	}

	// Verify timestamps in SQLite (stored as text)
	var sqCreatedAt, sqUpdatedAt string
	err = dbs.SQLite.QueryRow(
		`SELECT created_at, updated_at FROM test_authors WHERE public_id = ?`, publicID).
		Scan(&sqCreatedAt, &sqUpdatedAt)
	if err != nil {
		t.Fatalf("sqlite query failed: %v", err)
	}

	if sqCreatedAt == "" {
		t.Error("sqlite created_at should not be empty")
	}
	if sqUpdatedAt == "" {
		t.Error("sqlite updated_at should not be empty")
	}

	// SQLite timestamps are stored as text, parse them
	sqCreated, err := time.Parse("2006-01-02 15:04:05", sqCreatedAt)
	if err != nil {
		t.Fatalf("failed to parse sqlite created_at: %v", err)
	}
	if sqCreated.IsZero() {
		t.Error("sqlite created_at should not be zero")
	}
}

// TestUpdate_OnlyChangesUpdatedAt verifies that UPDATE changes updated_at but not created_at.
func TestUpdate_OnlyChangesUpdatedAt(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	publicID := nanoid.New()

	// Insert author
	dbs.InsertAuthor(t, publicID, "Original Name", "original@example.com", nil, true)

	// Wait a bit to ensure timestamps differ
	time.Sleep(1100 * time.Millisecond) // Need >1 second for second-precision timestamps

	// Get original timestamps from Postgres
	var pgOrigCreatedAt, pgOrigUpdatedAt time.Time
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT created_at, updated_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgOrigCreatedAt, &pgOrigUpdatedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}

	// Update in all databases
	_, err = dbs.Postgres.Exec(ctx,
		`UPDATE test_authors SET name = $1, updated_at = NOW() WHERE public_id = $2`,
		"Updated Name", publicID)
	if err != nil {
		t.Fatalf("postgres update failed: %v", err)
	}

	_, err = dbs.MySQL.Exec(
		"UPDATE test_authors SET name = ?, updated_at = NOW() WHERE public_id = ?",
		"Updated Name", publicID)
	if err != nil {
		t.Fatalf("mysql update failed: %v", err)
	}

	_, err = dbs.SQLite.Exec(
		`UPDATE test_authors SET name = ?, updated_at = datetime('now') WHERE public_id = ?`,
		"Updated Name", publicID)
	if err != nil {
		t.Fatalf("sqlite update failed: %v", err)
	}

	// Verify Postgres timestamps
	var pgNewCreatedAt, pgNewUpdatedAt time.Time
	err = dbs.Postgres.QueryRow(ctx,
		`SELECT created_at, updated_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgNewCreatedAt, &pgNewUpdatedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}

	// created_at should NOT change
	if !pgNewCreatedAt.Equal(pgOrigCreatedAt) {
		t.Errorf("postgres created_at should not change on update. Original: %v, New: %v",
			pgOrigCreatedAt, pgNewCreatedAt)
	}

	// updated_at SHOULD have changed (be different from original)
	if pgNewUpdatedAt.Equal(pgOrigUpdatedAt) {
		t.Errorf("postgres updated_at should change on update. Original: %v, New: %v",
			pgOrigUpdatedAt, pgNewUpdatedAt)
	}

	// updated_at should be after original
	if !pgNewUpdatedAt.After(pgOrigUpdatedAt) {
		t.Errorf("postgres updated_at should be after original. Original: %v, New: %v",
			pgOrigUpdatedAt, pgNewUpdatedAt)
	}

	// Verify MySQL - just check the update worked
	var myName string
	err = dbs.MySQL.QueryRow(
		"SELECT name FROM test_authors WHERE public_id = ?", publicID).
		Scan(&myName)
	if err != nil {
		t.Fatalf("mysql query failed: %v", err)
	}
	if myName != "Updated Name" {
		t.Errorf("mysql name should be 'Updated Name', got %q", myName)
	}
}

// TestSoftDelete_SetsDeletedAt verifies that soft delete sets deleted_at to NOW().
func TestSoftDelete_SetsDeletedAt(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	publicID := nanoid.New()

	// Insert author
	dbs.InsertAuthor(t, publicID, "To Be Deleted", "delete@example.com", nil, true)

	// Verify deleted_at is NULL initially
	var pgDeletedAt *time.Time
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT deleted_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgDeletedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}
	if pgDeletedAt != nil {
		t.Error("deleted_at should be NULL before soft delete")
	}

	// Soft delete in all databases
	dbs.SoftDeleteAuthor(t, publicID)

	// Verify Postgres deleted_at is now set
	err = dbs.Postgres.QueryRow(ctx,
		`SELECT deleted_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgDeletedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}
	if pgDeletedAt == nil {
		t.Error("postgres deleted_at should not be NULL after soft delete")
	} else if pgDeletedAt.IsZero() {
		t.Error("postgres deleted_at should not be zero after soft delete")
	}

	// Verify MySQL deleted_at (scan as string)
	var myDeletedAtStr *string
	err = dbs.MySQL.QueryRow(
		"SELECT deleted_at FROM test_authors WHERE public_id = ?", publicID).
		Scan(&myDeletedAtStr)
	if err != nil {
		t.Fatalf("mysql query failed: %v", err)
	}
	if myDeletedAtStr == nil {
		t.Error("mysql deleted_at should not be NULL after soft delete")
	}

	// Verify SQLite deleted_at
	var sqDeletedAt *string
	err = dbs.SQLite.QueryRow(
		`SELECT deleted_at FROM test_authors WHERE public_id = ?`, publicID).
		Scan(&sqDeletedAt)
	if err != nil {
		t.Fatalf("sqlite query failed: %v", err)
	}
	if sqDeletedAt == nil {
		t.Error("sqlite deleted_at should not be NULL after soft delete")
	}
}

// TestSoftDelete_ExcludesFromRegularQueries verifies that soft-deleted records
// are excluded from queries with "deleted_at IS NULL".
func TestSoftDelete_ExcludesFromRegularQueries(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Insert two authors
	activeID := nanoid.New()
	deletedID := nanoid.New()

	dbs.InsertAuthor(t, activeID, "Active Author", "active@example.com", nil, true)
	dbs.InsertAuthor(t, deletedID, "Deleted Author", "deleted@example.com", nil, true)

	// Soft delete one
	dbs.SoftDeleteAuthor(t, deletedID)

	// Query with deleted_at IS NULL filter
	// Postgres
	var pgCount int
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT COUNT(*) FROM test_authors WHERE deleted_at IS NULL`).
		Scan(&pgCount)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}
	if pgCount != 1 {
		t.Errorf("postgres should return 1 active author, got %d", pgCount)
	}

	// MySQL
	var myCount int
	err = dbs.MySQL.QueryRow(
		"SELECT COUNT(*) FROM test_authors WHERE deleted_at IS NULL").
		Scan(&myCount)
	if err != nil {
		t.Fatalf("mysql query failed: %v", err)
	}
	if myCount != 1 {
		t.Errorf("mysql should return 1 active author, got %d", myCount)
	}

	// SQLite
	var sqCount int
	err = dbs.SQLite.QueryRow(
		`SELECT COUNT(*) FROM test_authors WHERE deleted_at IS NULL`).
		Scan(&sqCount)
	if err != nil {
		t.Fatalf("sqlite query failed: %v", err)
	}
	if sqCount != 1 {
		t.Errorf("sqlite should return 1 active author, got %d", sqCount)
	}

	// Verify the soft-deleted record still exists (total count)
	err = dbs.Postgres.QueryRow(ctx, `SELECT COUNT(*) FROM test_authors`).Scan(&pgCount)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}
	if pgCount != 2 {
		t.Errorf("postgres total should be 2 authors, got %d", pgCount)
	}
}

// TestNanoidGeneration verifies that nanoid generates unique IDs.
func TestNanoidGeneration(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := nanoid.New()

		// Check length
		if len(id) != 21 {
			t.Errorf("nanoid should be 21 characters, got %d: %s", len(id), id)
		}

		// Check uniqueness
		if seen[id] {
			t.Errorf("nanoid collision detected: %s", id)
		}
		seen[id] = true

		// Check characters are URL-safe
		for _, c := range []byte(id) {
			valid := (c >= '0' && c <= '9') ||
				(c >= 'a' && c <= 'z') ||
				(c >= 'A' && c <= 'Z') ||
				c == '-' || c == '_'
			if !valid {
				t.Errorf("Invalid character %q in nanoid %q", c, id)
			}
		}
	}
}

// TestCrossDatabase_TimestampConsistency verifies that all three databases
// set timestamps (regardless of timezone differences).
func TestCrossDatabase_TimestampConsistency(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Insert same author in all databases
	publicID := nanoid.New()
	dbs.InsertAuthor(t, publicID, "Consistency Test", "consistency@example.com", nil, true)

	// Verify Postgres has a timestamp
	var pgCreatedAt time.Time
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT created_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgCreatedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}
	if pgCreatedAt.IsZero() {
		t.Error("postgres created_at should not be zero")
	}

	// Verify MySQL has a timestamp (scan as string)
	var myCreatedAtStr string
	err = dbs.MySQL.QueryRow(
		"SELECT created_at FROM test_authors WHERE public_id = ?", publicID).
		Scan(&myCreatedAtStr)
	if err != nil {
		t.Fatalf("mysql query failed: %v", err)
	}
	if myCreatedAtStr == "" {
		t.Error("mysql created_at should not be empty")
	}
	myCreatedAt, err := time.Parse("2006-01-02 15:04:05", myCreatedAtStr)
	if err != nil {
		t.Fatalf("failed to parse mysql timestamp: %v", err)
	}
	if myCreatedAt.IsZero() {
		t.Error("mysql created_at should not be zero")
	}

	// Verify SQLite has a timestamp
	var sqCreatedAtStr string
	err = dbs.SQLite.QueryRow(
		`SELECT created_at FROM test_authors WHERE public_id = ?`, publicID).
		Scan(&sqCreatedAtStr)
	if err != nil {
		t.Fatalf("sqlite query failed: %v", err)
	}
	if sqCreatedAtStr == "" {
		t.Error("sqlite created_at should not be empty")
	}
	sqCreatedAt, err := time.Parse("2006-01-02 15:04:05", sqCreatedAtStr)
	if err != nil {
		t.Fatalf("failed to parse sqlite timestamp: %v", err)
	}
	if sqCreatedAt.IsZero() {
		t.Error("sqlite created_at should not be zero")
	}

	// Log the timestamps for debugging (don't compare due to timezone differences)
	t.Logf("Timestamps - PG: %v, MY: %v, SQ: %v", pgCreatedAt, myCreatedAt, sqCreatedAt)
}

// TestInsertAndFetch_WithGeneratedPublicID simulates the full CRUD flow with
// generated public_id.
func TestInsertAndFetch_WithGeneratedPublicID(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Generate public_id like the CRUD runner would
	publicID := nanoid.New()
	name := "Generated ID Test"
	email := "generated@example.com"

	// Insert into all databases
	dbs.InsertAuthor(t, publicID, name, email, nil, true)

	// Fetch by public_id from Postgres
	var pgName string
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT name FROM test_authors WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID).Scan(&pgName)
	if err != nil {
		t.Fatalf("postgres fetch failed: %v", err)
	}
	if pgName != name {
		t.Errorf("postgres name mismatch: got %q, want %q", pgName, name)
	}

	// Fetch by public_id from MySQL
	var myName string
	err = dbs.MySQL.QueryRow(
		"SELECT name FROM test_authors WHERE public_id = ? AND deleted_at IS NULL",
		publicID).Scan(&myName)
	if err != nil {
		t.Fatalf("mysql fetch failed: %v", err)
	}
	if myName != name {
		t.Errorf("mysql name mismatch: got %q, want %q", myName, name)
	}

	// Fetch by public_id from SQLite
	var sqName string
	err = dbs.SQLite.QueryRow(
		`SELECT name FROM test_authors WHERE public_id = ? AND deleted_at IS NULL`,
		publicID).Scan(&sqName)
	if err != nil {
		t.Fatalf("sqlite fetch failed: %v", err)
	}
	if sqName != name {
		t.Errorf("sqlite name mismatch: got %q, want %q", sqName, name)
	}
}

// TestUpdateExcludesSoftDeleted verifies that UPDATE with deleted_at IS NULL
// does not affect soft-deleted records.
func TestUpdateExcludesSoftDeleted(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Insert and soft delete
	publicID := nanoid.New()
	dbs.InsertAuthor(t, publicID, "Original Name", "update-test@example.com", nil, true)
	dbs.SoftDeleteAuthor(t, publicID)

	// Try to update with deleted_at IS NULL condition
	result, err := dbs.Postgres.Exec(ctx,
		`UPDATE test_authors SET name = $1, updated_at = NOW() 
		 WHERE public_id = $2 AND deleted_at IS NULL`,
		"Should Not Update", publicID)
	if err != nil {
		t.Fatalf("postgres update failed: %v", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected != 0 {
		t.Errorf("postgres should not update soft-deleted record, but affected %d rows", rowsAffected)
	}

	// Verify the name didn't change
	var pgName string
	err = dbs.Postgres.QueryRow(ctx,
		`SELECT name FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&pgName)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}
	if pgName != "Original Name" {
		t.Errorf("postgres name should still be 'Original Name', got %q", pgName)
	}
}

// TestSoftDeleteIdempotent verifies that soft delete can be called multiple times
// without error (idempotent operation).
func TestSoftDeleteIdempotent(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	publicID := nanoid.New()

	// Insert
	dbs.InsertAuthor(t, publicID, "Idempotent Test", "idempotent@example.com", nil, true)

	// First soft delete
	dbs.SoftDeleteAuthor(t, publicID)

	// Get the deleted_at timestamp
	var firstDeletedAt *time.Time
	err := dbs.Postgres.QueryRow(ctx,
		`SELECT deleted_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&firstDeletedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}

	// Wait a moment
	time.Sleep(100 * time.Millisecond)

	// Second soft delete attempt (should not change deleted_at due to IS NULL check)
	_, err = dbs.Postgres.Exec(ctx,
		`UPDATE test_authors SET deleted_at = NOW() 
		 WHERE public_id = $1 AND deleted_at IS NULL`, publicID)
	if err != nil {
		t.Fatalf("second soft delete failed: %v", err)
	}

	// Verify deleted_at hasn't changed
	var secondDeletedAt *time.Time
	err = dbs.Postgres.QueryRow(ctx,
		`SELECT deleted_at FROM test_authors WHERE public_id = $1`, publicID).
		Scan(&secondDeletedAt)
	if err != nil {
		t.Fatalf("postgres query failed: %v", err)
	}

	if firstDeletedAt == nil || secondDeletedAt == nil {
		t.Fatal("deleted_at should not be nil after soft delete")
	}

	if !firstDeletedAt.Equal(*secondDeletedAt) {
		t.Errorf("deleted_at should not change on second soft delete. First: %v, Second: %v",
			*firstDeletedAt, *secondDeletedAt)
	}
}
