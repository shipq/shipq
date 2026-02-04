package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/inifile"
)

func setupTestProjectWithMigrations(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory and resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, _ := filepath.EvalSymlinks(t.TempDir())

	// Create go.mod
	goModContent := "module testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create shipq.ini with database_url
	shipqIniContent := "[db]\ndatabase_url = sqlite:///tmp/test.db\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to create shipq.ini: %v", err)
	}

	// Create migrations directory
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("failed to create migrations directory: %v", err)
	}

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		os.Chdir(origDir)
	}

	return tmpDir, cleanup
}

func TestGetMigrationsPath(t *testing.T) {
	tmpDir, cleanup := setupTestProjectWithMigrations(t)
	defer cleanup()

	// Load ini file
	shipqIniPath := filepath.Join(tmpDir, "shipq.ini")
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	path := getMigrationsPath(ini, tmpDir)
	expected := filepath.Join(tmpDir, "migrations")

	if path != expected {
		t.Errorf("getMigrationsPath() = %q, want %q", path, expected)
	}
}

func TestGetMigrationsPath_CustomPath(t *testing.T) {
	tmpDir, cleanup := setupTestProjectWithMigrations(t)
	defer cleanup()

	// Update shipq.ini with custom migrations path
	shipqIniContent := "[db]\ndatabase_url = sqlite:///tmp/test.db\nmigrations = db/migrations\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to update shipq.ini: %v", err)
	}

	// Load ini file
	shipqIniPath := filepath.Join(tmpDir, "shipq.ini")
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	path := getMigrationsPath(ini, tmpDir)
	expected := filepath.Join(tmpDir, "db/migrations")

	if path != expected {
		t.Errorf("getMigrationsPath() = %q, want %q", path, expected)
	}
}

func TestBuildTestDatabaseURL_Postgres(t *testing.T) {
	devURL := "postgres://user@localhost:5432/myapp"
	testURL, err := buildTestDatabaseURL(devURL, "postgres")
	if err != nil {
		t.Fatalf("buildTestDatabaseURL() error = %v", err)
	}

	if !strings.Contains(testURL, "myapp_test") {
		t.Errorf("test URL should contain myapp_test, got %s", testURL)
	}
}

func TestBuildTestDatabaseURL_MySQL(t *testing.T) {
	devURL := "mysql://user@localhost:3306/myapp"
	testURL, err := buildTestDatabaseURL(devURL, "mysql")
	if err != nil {
		t.Fatalf("buildTestDatabaseURL() error = %v", err)
	}

	if !strings.Contains(testURL, "myapp_test") {
		t.Errorf("test URL should contain myapp_test, got %s", testURL)
	}
}

func TestBuildTestDatabaseURL_SQLite(t *testing.T) {
	devURL := "sqlite:///path/to/myapp.db"
	testURL, err := buildTestDatabaseURL(devURL, "sqlite")
	if err != nil {
		t.Fatalf("buildTestDatabaseURL() error = %v", err)
	}

	if !strings.Contains(testURL, "myapp_test") {
		t.Errorf("test URL should contain myapp_test, got %s", testURL)
	}
}

func TestBuildTestDatabaseURL_EmptyDBName(t *testing.T) {
	devURL := "postgres://user@localhost:5432/"
	_, err := buildTestDatabaseURL(devURL, "postgres")
	if err == nil {
		t.Error("buildTestDatabaseURL() should error with empty database name")
	}
}

func TestURLToDSNWithDriver_Postgres(t *testing.T) {
	url := "postgres://user@localhost:5432/mydb"
	dsn, driver, err := urlToDSNWithDriver(url, "postgres")

	if err != nil {
		t.Fatalf("urlToDSNWithDriver() error = %v", err)
	}
	if driver != "pgx" {
		t.Errorf("driver = %q, want %q", driver, "pgx")
	}
	if dsn != url {
		t.Errorf("dsn = %q, want %q", dsn, url)
	}
}

func TestURLToDSNWithDriver_MySQL(t *testing.T) {
	url := "mysql://user@localhost:3306/mydb"
	dsn, driver, err := urlToDSNWithDriver(url, "mysql")

	if err != nil {
		t.Fatalf("urlToDSNWithDriver() error = %v", err)
	}
	if driver != "mysql" {
		t.Errorf("driver = %q, want %q", driver, "mysql")
	}
	// DSN should be in format user@tcp(host:port)/dbname
	if !strings.Contains(dsn, "@tcp(") {
		t.Errorf("dsn should contain @tcp(, got %q", dsn)
	}
}

func TestURLToDSNWithDriver_SQLite(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantDSN string
	}{
		{
			name:    "sqlite:// prefix",
			url:     "sqlite:///path/to/db.sqlite",
			wantDSN: "/path/to/db.sqlite",
		},
		{
			name:    "sqlite: prefix",
			url:     "sqlite:/path/to/db.sqlite",
			wantDSN: "/path/to/db.sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, driver, err := urlToDSNWithDriver(tt.url, "sqlite")
			if err != nil {
				t.Fatalf("urlToDSNWithDriver() error = %v", err)
			}
			if driver != "sqlite" {
				t.Errorf("driver = %q, want %q", driver, "sqlite")
			}
			if dsn != tt.wantDSN {
				t.Errorf("dsn = %q, want %q", dsn, tt.wantDSN)
			}
		})
	}
}

func TestURLToDSNWithDriver_UnsupportedDialect(t *testing.T) {
	_, _, err := urlToDSNWithDriver("oracle://localhost/db", "oracle")
	if err == nil {
		t.Error("urlToDSNWithDriver() should error for unsupported dialect")
	}
}

func TestMigrateUp_GeneratesDBPackage(t *testing.T) {
	tmpDir, cleanup := setupTestProjectWithMigrations(t)
	defer cleanup()

	// Generate DB package directly using codegen
	err := codegen.EnsureDBPackage(tmpDir)
	if err != nil {
		t.Fatalf("EnsureDBPackage() error = %v", err)
	}

	// Verify shipq/db/db.go was created
	dbFilePath := filepath.Join(tmpDir, "shipq", "db", "db.go")
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		t.Error("shipq/db/db.go was not created")
	}

	// Verify content
	content, err := os.ReadFile(dbFilePath)
	if err != nil {
		t.Fatalf("failed to read db.go: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "package db") {
		t.Error("db.go missing package declaration")
	}
	if !strings.Contains(contentStr, `const Dialect = "sqlite"`) {
		t.Error("db.go missing Dialect constant")
	}
	if !strings.Contains(contentStr, "func DB()") {
		t.Error("db.go missing DB() function")
	}
}

func TestMigrateUp_DiscoversMigrations(t *testing.T) {
	tmpDir, cleanup := setupTestProjectWithMigrations(t)
	defer cleanup()

	// Create a migration file
	migrationsDir := filepath.Join(tmpDir, "migrations")
	migrationContent := `package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260115120000_users(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	return err
}
`
	migrationPath := filepath.Join(migrationsDir, "20260115120000_users.go")
	if err := os.WriteFile(migrationPath, []byte(migrationContent), 0644); err != nil {
		t.Fatalf("failed to write migration file: %v", err)
	}

	// Discover migrations
	migrations, err := codegen.DiscoverMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("DiscoverMigrations() error = %v", err)
	}

	if len(migrations) != 1 {
		t.Errorf("discovered %d migrations, want 1", len(migrations))
	}

	if len(migrations) > 0 {
		m := migrations[0]
		if m.Timestamp != "20260115120000" {
			t.Errorf("migration timestamp = %q, want %q", m.Timestamp, "20260115120000")
		}
		if m.Name != "users" {
			t.Errorf("migration name = %q, want %q", m.Name, "users")
		}
		if m.FuncName != "Migrate_20260115120000_users" {
			t.Errorf("migration funcname = %q, want %q", m.FuncName, "Migrate_20260115120000_users")
		}
	}
}

func TestMigrateUp_GeneratesMigrateRunner(t *testing.T) {
	modulePath := "testproject"

	content, err := codegen.GenerateMigrateRunner(modulePath)
	if err != nil {
		t.Fatalf("GenerateMigrateRunner() error = %v", err)
	}

	contentStr := string(content)

	// Check essential parts
	if !strings.Contains(contentStr, "package migrate") {
		t.Error("runner.go missing package declaration")
	}
	if !strings.Contains(contentStr, "//go:embed schema.json") {
		t.Error("runner.go missing embed directive")
	}
	if !strings.Contains(contentStr, "func Plan()") {
		t.Error("runner.go missing Plan() function")
	}
	if !strings.Contains(contentStr, "func Run(") {
		t.Error("runner.go missing Run() function")
	}
	if !strings.Contains(contentStr, "func RunWithDB(") {
		t.Error("runner.go missing RunWithDB() function")
	}
	if !strings.Contains(contentStr, `db "testproject/shipq/db"`) {
		t.Error("runner.go missing correct db import path")
	}
}

func TestMigrateUp_NoMigrations(t *testing.T) {
	tmpDir, cleanup := setupTestProjectWithMigrations(t)
	defer cleanup()

	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Discover migrations (should be empty)
	migrations, err := codegen.DiscoverMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("DiscoverMigrations() error = %v", err)
	}

	if len(migrations) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(migrations))
	}
}

func TestMigrateUp_MultipleMigrations(t *testing.T) {
	tmpDir, cleanup := setupTestProjectWithMigrations(t)
	defer cleanup()

	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create multiple migration files
	files := []struct {
		name    string
		content string
	}{
		{
			name: "20260115120000_users.go",
			content: `package migrations

import "github.com/shipq/shipq/db/portsql/migrate"

func Migrate_20260115120000_users(plan *migrate.MigrationPlan) error {
	return nil
}
`,
		},
		{
			name: "20260115120100_posts.go",
			content: `package migrations

import "github.com/shipq/shipq/db/portsql/migrate"

func Migrate_20260115120100_posts(plan *migrate.MigrationPlan) error {
	return nil
}
`,
		},
		{
			name: "20260115120200_comments.go",
			content: `package migrations

import "github.com/shipq/shipq/db/portsql/migrate"

func Migrate_20260115120200_comments(plan *migrate.MigrationPlan) error {
	return nil
}
`,
		},
	}

	for _, f := range files {
		path := filepath.Join(migrationsDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", f.name, err)
		}
	}

	// Discover migrations
	migrations, err := codegen.DiscoverMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("DiscoverMigrations() error = %v", err)
	}

	if len(migrations) != 3 {
		t.Errorf("discovered %d migrations, want 3", len(migrations))
	}

	// Verify they're in order
	expectedTimestamps := []string{"20260115120000", "20260115120100", "20260115120200"}
	for i, m := range migrations {
		if m.Timestamp != expectedTimestamps[i] {
			t.Errorf("migration[%d] timestamp = %q, want %q", i, m.Timestamp, expectedTimestamps[i])
		}
	}
}
