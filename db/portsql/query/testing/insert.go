//go:build integration

package testing

import (
	"context"
	"testing"
)

// InsertAuthor inserts an author into all databases
func (dbs *TestDBs) InsertAuthor(t *testing.T, publicID, name, email string, bio *string, active bool) {
	t.Helper()

	ctx := context.Background()

	// Postgres
	if _, err := dbs.Postgres.Exec(ctx,
		`INSERT INTO test_authors (public_id, name, email, bio, active) VALUES ($1, $2, $3, $4, $5)`,
		publicID, name, email, bio, active,
	); err != nil {
		t.Fatalf("postgres insert author failed: %v", err)
	}

	// MySQL - convert bool to int
	activeInt := 0
	if active {
		activeInt = 1
	}
	if _, err := dbs.MySQL.Exec(
		"INSERT INTO test_authors (public_id, name, email, bio, active) VALUES (?, ?, ?, ?, ?)",
		publicID, name, email, bio, activeInt,
	); err != nil {
		t.Fatalf("mysql insert author failed: %v", err)
	}

	// SQLite
	if _, err := dbs.SQLite.Exec(
		`INSERT INTO test_authors (public_id, name, email, bio, active) VALUES (?, ?, ?, ?, ?)`,
		publicID, name, email, bio, activeInt,
	); err != nil {
		t.Fatalf("sqlite insert author failed: %v", err)
	}
}

// TryInsertAuthorEach tries to insert an author into each database independently,
// returning the error (if any) for each dialect. This allows tests to verify that
// all databases behave consistently for edge cases.
func (dbs *TestDBs) TryInsertAuthorEach(publicID, name, email string, bio *string, active bool) map[Dialect]error {
	ctx := context.Background()
	results := make(map[Dialect]error)

	activeInt := 0
	if active {
		activeInt = 1
	}

	// Try Postgres
	_, results[DialectPostgres] = dbs.Postgres.Exec(ctx,
		`INSERT INTO test_authors (public_id, name, email, bio, active) VALUES ($1, $2, $3, $4, $5)`,
		publicID, name, email, bio, active,
	)

	// Try MySQL
	_, results[DialectMySQL] = dbs.MySQL.Exec(
		"INSERT INTO test_authors (public_id, name, email, bio, active) VALUES (?, ?, ?, ?, ?)",
		publicID, name, email, bio, activeInt,
	)

	// Try SQLite
	_, results[DialectSQLite] = dbs.SQLite.Exec(
		`INSERT INTO test_authors (public_id, name, email, bio, active) VALUES (?, ?, ?, ?, ?)`,
		publicID, name, email, bio, activeInt,
	)

	return results
}

// InsertBook inserts a book into all databases using the author's public_id
// to look up the correct author_id in each database (since auto-increment IDs differ)
func (dbs *TestDBs) InsertBook(t *testing.T, publicID string, authorPublicID string, title string, price *string) {
	t.Helper()

	ctx := context.Background()

	// Postgres - look up author_id by public_id
	if _, err := dbs.Postgres.Exec(ctx,
		`INSERT INTO test_books (public_id, author_id, title, price) 
		 SELECT $1, id, $2, $3 FROM test_authors WHERE public_id = $4`,
		publicID, title, price, authorPublicID,
	); err != nil {
		t.Fatalf("postgres insert book failed: %v", err)
	}

	// MySQL - look up author_id by public_id
	if _, err := dbs.MySQL.Exec(
		`INSERT INTO test_books (public_id, author_id, title, price) 
		 SELECT ?, id, ?, ? FROM test_authors WHERE public_id = ?`,
		publicID, title, price, authorPublicID,
	); err != nil {
		t.Fatalf("mysql insert book failed: %v", err)
	}

	// SQLite - look up author_id by public_id
	if _, err := dbs.SQLite.Exec(
		`INSERT INTO test_books (public_id, author_id, title, price) 
		 SELECT ?, id, ?, ? FROM test_authors WHERE public_id = ?`,
		publicID, title, price, authorPublicID,
	); err != nil {
		t.Fatalf("sqlite insert book failed: %v", err)
	}
}

// ClearAllData removes all test data from all databases
func (dbs *TestDBs) ClearAllData(t *testing.T) {
	t.Helper()

	ctx := context.Background()

	// Delete in order due to foreign keys
	dbs.Postgres.Exec(ctx, "DELETE FROM test_books")
	dbs.Postgres.Exec(ctx, "DELETE FROM test_authors")

	dbs.MySQL.Exec("DELETE FROM test_books")
	dbs.MySQL.Exec("DELETE FROM test_authors")

	dbs.SQLite.Exec("DELETE FROM test_books")
	dbs.SQLite.Exec("DELETE FROM test_authors")
}

// SoftDeleteAuthor marks an author as deleted in all databases
func (dbs *TestDBs) SoftDeleteAuthor(t *testing.T, publicID string) {
	t.Helper()

	ctx := context.Background()

	// Postgres
	if _, err := dbs.Postgres.Exec(ctx,
		"UPDATE test_authors SET deleted_at = NOW() WHERE public_id = $1",
		publicID,
	); err != nil {
		t.Fatalf("postgres soft delete failed: %v", err)
	}

	// MySQL
	if _, err := dbs.MySQL.Exec(
		"UPDATE test_authors SET deleted_at = NOW() WHERE public_id = ?",
		publicID,
	); err != nil {
		t.Fatalf("mysql soft delete failed: %v", err)
	}

	// SQLite
	if _, err := dbs.SQLite.Exec(
		"UPDATE test_authors SET deleted_at = datetime('now') WHERE public_id = ?",
		publicID,
	); err != nil {
		t.Fatalf("sqlite soft delete failed: %v", err)
	}
}
