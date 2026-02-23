package migrate

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// splitSQLStatements Tests
// =============================================================================

func TestSplitSQLStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single statement without semicolon",
			input: "CREATE TABLE users (id INT)",
			want:  []string{"CREATE TABLE users (id INT)"},
		},
		{
			name:  "single statement with semicolon",
			input: "CREATE TABLE users (id INT);",
			want:  []string{"CREATE TABLE users (id INT)"},
		},
		{
			name:  "two statements",
			input: "CREATE TABLE users (id INT); CREATE INDEX idx ON users (id)",
			want:  []string{"CREATE TABLE users (id INT)", "CREATE INDEX idx ON users (id)"},
		},
		{
			name:  "multiple statements with newlines",
			input: "CREATE TABLE users (id INT);\nCREATE INDEX idx ON users (id);\nCREATE INDEX idx2 ON users (id)",
			want:  []string{"CREATE TABLE users (id INT)", "CREATE INDEX idx ON users (id)", "CREATE INDEX idx2 ON users (id)"},
		},
		{
			name:  "statements with extra whitespace",
			input: "  CREATE TABLE users (id INT)  ;  CREATE INDEX idx ON users (id)  ;  ",
			want:  []string{"CREATE TABLE users (id INT)", "CREATE INDEX idx ON users (id)"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only semicolons and whitespace",
			input: "  ;  ;  ",
			want:  nil,
		},
		{
			name:  "MySQL style CREATE TABLE with indexes",
			input: "CREATE TABLE `accounts` (`id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY) ENGINE=InnoDB;\nCREATE UNIQUE INDEX `idx_accounts_id` ON `accounts` (`id`);\nCREATE UNIQUE INDEX `idx_accounts_email` ON `accounts` (`email`)",
			want: []string{
				"CREATE TABLE `accounts` (`id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY) ENGINE=InnoDB",
				"CREATE UNIQUE INDEX `idx_accounts_id` ON `accounts` (`id`)",
				"CREATE UNIQUE INDEX `idx_accounts_email` ON `accounts` (`email`)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSQLStatements(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitSQLStatements() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Multi-Statement Migration Tests (MySQL-style)
// =============================================================================

func TestRunMultiStatementMigration(t *testing.T) {
	// This test verifies that migrations containing multiple SQL statements
	// (like CREATE TABLE + CREATE INDEX) are executed correctly.
	// This was a bug where MySQL would fail with syntax error because
	// multiple statements were sent in a single Exec() call.

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a migration with multiple statements (like MySQL generates)
	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260204135551_create_accounts",
				Instructions: MigrationInstructions{
					// This mimics what generateMySQLCreateTable produces:
					// CREATE TABLE followed by CREATE INDEX statements
					Sqlite: `CREATE TABLE accounts (id INTEGER PRIMARY KEY, email TEXT NOT NULL);
CREATE UNIQUE INDEX idx_accounts_email ON accounts (email)`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err != nil {
		t.Fatalf("Run() failed on multi-statement migration: %v", err)
	}

	// Verify table was created
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}

	foundAccounts := false
	for _, table := range tables {
		if table == "accounts" {
			foundAccounts = true
		}
	}
	if !foundAccounts {
		t.Error("accounts table should exist")
	}

	// Verify index was created by trying to insert duplicate emails
	_, err = db.Exec("INSERT INTO accounts (email) VALUES ('test@example.com')")
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec("INSERT INTO accounts (email) VALUES ('test@example.com')")
	if err == nil {
		t.Error("second insert should fail due to unique index")
	}

	// Verify migration was recorded
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 1 || applied[0] != "20260204135551_create_accounts" {
		t.Errorf("expected migration to be recorded, got: %v", applied)
	}
}

func TestRunMultiStatementMigrationWithThreeIndexes(t *testing.T) {
	// Test with multiple indexes like the real accounts table might have

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260204135551_create_accounts",
				Instructions: MigrationInstructions{
					Sqlite: `CREATE TABLE accounts (id INTEGER PRIMARY KEY, first_name TEXT, last_name TEXT, email TEXT NOT NULL);
CREATE UNIQUE INDEX idx_accounts_id ON accounts (id);
CREATE UNIQUE INDEX idx_accounts_email ON accounts (email)`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err != nil {
		t.Fatalf("Run() failed on multi-statement migration with 3 indexes: %v", err)
	}

	// Verify migration was recorded
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 1 {
		t.Errorf("expected 1 migration recorded, got %d", len(applied))
	}
}

func TestRunMultiStatementMigrationRollsBackOnFailure(t *testing.T) {
	// If any statement in a multi-statement migration fails,
	// all statements should be rolled back

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	plan := &MigrationPlan{
		Schema: Schema{Tables: make(map[string]ddl.Table)},
		Migrations: []Migration{
			{
				Name: "20260204135551_create_accounts",
				Instructions: MigrationInstructions{
					// First statement succeeds, second fails (references non-existent column)
					Sqlite: `CREATE TABLE accounts (id INTEGER PRIMARY KEY, email TEXT);
CREATE UNIQUE INDEX idx_accounts_nonexistent ON accounts (nonexistent_column)`,
				},
			},
		},
	}

	err = Run(ctx, db, plan, Sqlite)
	if err == nil {
		t.Fatal("Run() should fail when index references non-existent column")
	}

	// The table should NOT exist (rolled back)
	tables, err := GetAllTables(ctx, db, Sqlite)
	if err != nil {
		t.Fatalf("GetAllTables failed: %v", err)
	}

	for _, table := range tables {
		if table == "accounts" {
			t.Error("accounts table should not exist after rollback")
		}
	}

	// Migration should NOT be recorded
	applied, err := GetAppliedMigrations(ctx, db)
	if err != nil {
		t.Fatalf("GetAppliedMigrations failed: %v", err)
	}

	if len(applied) != 0 {
		t.Errorf("no migrations should be recorded after rollback, got: %v", applied)
	}
}

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
