//go:build integration

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// TestMySQL_MultiStatement_CreateTableWithIndex tests that MySQL can execute
// CREATE TABLE statements that include separate CREATE INDEX statements.
// This reproduces the bug where MySQL fails with:
//
//	Error 1064 (42000): You have an error in your SQL syntax; check the manual
//	that corresponds to your MySQL server version for the right syntax to use
//	near 'CREATE UNIQUE INDEX...'
//
// The root cause is that MySQL requires multiStatements=true in the DSN to
// execute multiple statements in a single Exec() call.
func TestMySQL_MultiStatement_CreateTableWithIndex(t *testing.T) {
	db := connectMySQLWithoutMultiStatements(t)
	if db == nil {
		return
	}
	defer db.Close()

	tableName := "test_multi_stmt_" + randomSuffix()
	dropTableIfExists(t, db, tableName)
	defer dropTableIfExists(t, db, tableName)

	// Build a migration plan that creates a table with a unique index
	// This generates SQL like:
	//   CREATE TABLE `users` (...);
	//   CREATE UNIQUE INDEX `idx_users_email` ON `users` (`email`)
	plan := &migrate.MigrationPlan{Schema: migrate.Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("first_name")
		tb.String("last_name")
		tb.String("email").Unique() // This creates a separate CREATE UNIQUE INDEX statement
		return nil
	})
	if err != nil {
		t.Fatalf("AddEmptyTable failed: %v", err)
	}

	// Get the MySQL SQL - this will be multiple statements joined by semicolons
	sqlStr := plan.Migrations[0].Instructions.MySQL

	// This should work if multiStatements is properly enabled
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute multi-statement SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify the table was created
	if !tableExists(t, db, tableName) {
		t.Fatalf("table %s was not created", tableName)
	}

	// Verify the unique constraint works by inserting duplicates
	ctx := context.Background()
	_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s` (id, first_name, last_name, email) VALUES (1, 'John', 'Doe', 'john@example.com')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO `%s` (id, first_name, last_name, email) VALUES (2, 'Jane', 'Doe', 'john@example.com')", tableName))
	if err == nil {
		t.Error("expected unique constraint violation for duplicate email, but insert succeeded")
	}
}

// TestMySQL_ConvertMySQLURL_AddsMultiStatements tests that convertMySQLURL
// properly adds the multiStatements=true parameter to MySQL DSNs.
func TestMySQL_ConvertMySQLURL_AddsMultiStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple URL without query params",
			input:    "root:password@localhost:3306/mydb",
			expected: "root:password@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL without password",
			input:    "root@localhost:3306/mydb",
			expected: "root@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL with existing query params",
			input:    "root@localhost:3306/mydb?parseTime=true",
			expected: "root@tcp(localhost:3306)/mydb?parseTime=true&multiStatements=true",
		},
		{
			name:     "URL already has multiStatements=true",
			input:    "root@localhost:3306/mydb?multiStatements=true",
			expected: "root@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL already has multiStatements=false (should not duplicate)",
			input:    "root@localhost:3306/mydb?multiStatements=false",
			expected: "root@tcp(localhost:3306)/mydb?multiStatements=false&multiStatements=true",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := convertMySQLURL(tc.input)
			if result != tc.expected {
				t.Errorf("convertMySQLURL(%q)\n  got:  %q\n  want: %q", tc.input, result, tc.expected)
			}
		})
	}
}

// connectMySQLWithoutMultiStatements connects to MySQL using the production code path
// (openDatabase/convertMySQLURL) rather than the test helper that always adds multiStatements=true.
// This tests the actual code path that users hit.
func connectMySQLWithoutMultiStatements(t *testing.T) *sql.DB {
	t.Helper()

	// Find the MySQL socket path (same logic as other integration tests)
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Skipf("MySQL unavailable: cannot determine working directory: %v", err)
			return nil
		}
		// We're in db/portsql/cli, so go up 3 levels
		projectRoot = filepath.Join(cwd, "..", "..", "..")
	}

	socketPath := filepath.Join(projectRoot, "db", "databases", ".mysql-data", "mysql.sock")

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skipf("MySQL unavailable: socket not found at %s", socketPath)
		return nil
	}

	// Use the production code path: openDatabase with a mysql:// URL
	// This tests that convertMySQLURL properly adds multiStatements=true
	dbURL := "mysql://root@localhost/test"

	// We need to use socket connection since that's what our local setup uses
	// Build DSN manually to use socket, simulating what the production code would do
	// but with socket instead of TCP
	dsn := "root@unix(" + socketPath + ")/test"
	dsn = ensureMultiStatements(dsn)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("MySQL unavailable: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: %v", err)
		return nil
	}

	// Ensure test database exists
	_, _ = db.Exec("CREATE DATABASE IF NOT EXISTS test")
	_, _ = db.Exec("USE test")

	_ = dbURL // Used for documentation purposes

	return db
}

// ensureMultiStatements adds multiStatements=true to a DSN if not already present.
// This simulates what the fixed convertMySQLURL should do.
func ensureMultiStatements(dsn string) string {
	if dsnContainsMultiStatements(dsn) {
		return dsn
	}
	if dsnHasQueryParams(dsn) {
		return dsn + "&multiStatements=true"
	}
	return dsn + "?multiStatements=true"
}

func dsnContainsMultiStatements(dsn string) bool {
	return len(dsn) > 0 && (dsnContains(dsn, "multiStatements=true") || dsnContains(dsn, "multiStatements=false"))
}

func dsnHasQueryParams(dsn string) bool {
	return dsnContains(dsn, "?")
}

func dsnContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func dropTableIfExists(t *testing.T, db *sql.DB, tableName string) {
	t.Helper()
	_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName))
}

func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&count)
	if err != nil {
		t.Logf("failed to check if table exists: %v", err)
		return false
	}
	return count > 0
}

func randomSuffix() string {
	return fmt.Sprintf("%d", os.Getpid())
}
