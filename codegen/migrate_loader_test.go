package codegen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestDiscoverMigrations(t *testing.T) {
	t.Run("discovers valid migration files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create valid migration files
		migrations := []struct {
			filename string
			content  string
		}{
			{
				filename: "20260115120000_users.go",
				content: `package migrations

import "github.com/shipq/shipq/db/portsql/migrate"

func Migrate_20260115120000_users(plan *migrate.MigrationPlan) error {
	return nil
}
`,
			},
			{
				filename: "20260115120100_posts.go",
				content: `package migrations

import "github.com/shipq/shipq/db/portsql/migrate"

func Migrate_20260115120100_posts(plan *migrate.MigrationPlan) error {
	return nil
}
`,
			},
		}

		for _, m := range migrations {
			path := filepath.Join(tmpDir, m.filename)
			if err := os.WriteFile(path, []byte(m.content), 0644); err != nil {
				t.Fatalf("failed to write migration file: %v", err)
			}
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 2 {
			t.Errorf("DiscoverMigrations() found %d migrations, want 2", len(discovered))
		}

		// Verify first migration
		if discovered[0].Timestamp != "20260115120000" {
			t.Errorf("first migration timestamp = %q, want %q", discovered[0].Timestamp, "20260115120000")
		}
		if discovered[0].Name != "users" {
			t.Errorf("first migration name = %q, want %q", discovered[0].Name, "users")
		}
		if discovered[0].FuncName != "Migrate_20260115120000_users" {
			t.Errorf("first migration funcname = %q, want %q", discovered[0].FuncName, "Migrate_20260115120000_users")
		}

		// Verify second migration
		if discovered[1].Timestamp != "20260115120100" {
			t.Errorf("second migration timestamp = %q, want %q", discovered[1].Timestamp, "20260115120100")
		}
		if discovered[1].Name != "posts" {
			t.Errorf("second migration name = %q, want %q", discovered[1].Name, "posts")
		}
	})

	t.Run("returns sorted by timestamp", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create files in non-chronological order
		files := []string{
			"20260115130000_third.go",
			"20260115110000_first.go",
			"20260115120000_second.go",
		}

		for _, f := range files {
			path := filepath.Join(tmpDir, f)
			content := "package migrations\n"
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 3 {
			t.Fatalf("DiscoverMigrations() found %d migrations, want 3", len(discovered))
		}

		// Verify sorted order
		expected := []string{"20260115110000", "20260115120000", "20260115130000"}
		for i, m := range discovered {
			if m.Timestamp != expected[i] {
				t.Errorf("migration[%d] timestamp = %q, want %q", i, m.Timestamp, expected[i])
			}
		}
	})

	t.Run("ignores test files", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := []string{
			"20260115120000_users.go",
			"20260115120000_users_test.go", // Should be ignored
		}

		for _, f := range files {
			path := filepath.Join(tmpDir, f)
			if err := os.WriteFile(path, []byte("package migrations"), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 1 {
			t.Errorf("DiscoverMigrations() found %d migrations, want 1 (test file should be ignored)", len(discovered))
		}
	})

	t.Run("ignores non-go files", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := []string{
			"20260115120000_users.go",
			"20260115120000_readme.md",
			"20260115120000_config.json",
		}

		for _, f := range files {
			path := filepath.Join(tmpDir, f)
			if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 1 {
			t.Errorf("DiscoverMigrations() found %d migrations, want 1 (non-go files should be ignored)", len(discovered))
		}
	})

	t.Run("ignores files with invalid timestamp format", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := []string{
			"20260115120000_valid.go",   // Valid
			"2026011512_short.go",       // Timestamp too short
			"20260115abcd00_letters.go", // Non-digit in timestamp
			"users.go",                  // No timestamp at all
			"20260115120000users.go",    // Missing underscore
		}

		for _, f := range files {
			path := filepath.Join(tmpDir, f)
			if err := os.WriteFile(path, []byte("package migrations"), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 1 {
			t.Errorf("DiscoverMigrations() found %d migrations, want 1 (invalid formats should be ignored)", len(discovered))
		}

		if discovered[0].Name != "valid" {
			t.Errorf("discovered migration name = %q, want %q", discovered[0].Name, "valid")
		}
	})

	t.Run("ignores directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a subdirectory with migration-like name
		subDir := filepath.Join(tmpDir, "20260115120000_subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}

		// Create a valid migration file
		path := filepath.Join(tmpDir, "20260115120000_users.go")
		if err := os.WriteFile(path, []byte("package migrations"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 1 {
			t.Errorf("DiscoverMigrations() found %d migrations, want 1 (directories should be ignored)", len(discovered))
		}
	})

	t.Run("returns empty slice for nonexistent directory", func(t *testing.T) {
		discovered, err := codegen.DiscoverMigrations("/nonexistent/path")
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v, expected nil for nonexistent directory", err)
		}

		if discovered != nil && len(discovered) != 0 {
			t.Errorf("DiscoverMigrations() returned %v, want nil or empty slice", discovered)
		}
	})

	t.Run("returns empty slice for empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 0 {
			t.Errorf("DiscoverMigrations() found %d migrations, want 0", len(discovered))
		}
	})

	t.Run("handles migration names with underscores", func(t *testing.T) {
		tmpDir := t.TempDir()

		path := filepath.Join(tmpDir, "20260115120000_create_user_profiles.go")
		if err := os.WriteFile(path, []byte("package migrations"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Fatalf("DiscoverMigrations() error = %v", err)
		}

		if len(discovered) != 1 {
			t.Fatalf("DiscoverMigrations() found %d migrations, want 1", len(discovered))
		}

		if discovered[0].Name != "create_user_profiles" {
			t.Errorf("migration name = %q, want %q", discovered[0].Name, "create_user_profiles")
		}

		if discovered[0].FuncName != "Migrate_20260115120000_create_user_profiles" {
			t.Errorf("migration funcname = %q, want %q", discovered[0].FuncName, "Migrate_20260115120000_create_user_profiles")
		}
	})
}

func TestMigrationFile_Fields(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "20260115120000_users.go")
	if err := os.WriteFile(path, []byte("package migrations"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	discovered, err := codegen.DiscoverMigrations(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverMigrations() error = %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(discovered))
	}

	m := discovered[0]

	// Verify Path is full path
	if m.Path != path {
		t.Errorf("Path = %q, want %q", m.Path, path)
	}

	// Verify Timestamp
	if m.Timestamp != "20260115120000" {
		t.Errorf("Timestamp = %q, want %q", m.Timestamp, "20260115120000")
	}

	// Verify Name
	if m.Name != "users" {
		t.Errorf("Name = %q, want %q", m.Name, "users")
	}

	// Verify FuncName
	if m.FuncName != "Migrate_20260115120000_users" {
		t.Errorf("FuncName = %q, want %q", m.FuncName, "Migrate_20260115120000_users")
	}
}

// TestGeneratedRunnerCallsSetCurrentMigration verifies that the generated migration runner
// calls SetCurrentMigration before each migration function. This is critical for ensuring
// migration names are stable across rebuilds.
func TestGeneratedRunnerCallsSetCurrentMigration(t *testing.T) {
	migrations := []codegen.MigrationFile{
		{
			Path:      "/path/to/20260115120000_users.go",
			Timestamp: "20260115120000",
			Name:      "users",
			FuncName:  "Migrate_20260115120000_users",
		},
		{
			Path:      "/path/to/20260115130000_posts.go",
			Timestamp: "20260115130000",
			Name:      "posts",
			FuncName:  "Migrate_20260115130000_posts",
		},
	}

	code := codegen.GenerateMigrationRunnerForTest(migrations)

	// Verify SetCurrentMigration is called before each migration
	if !strings.Contains(code, `plan.SetCurrentMigration("20260115120000_users")`) {
		t.Error("Generated runner should call SetCurrentMigration with first migration name")
	}

	if !strings.Contains(code, `plan.SetCurrentMigration("20260115130000_posts")`) {
		t.Error("Generated runner should call SetCurrentMigration with second migration name")
	}

	// Verify the order: SetCurrentMigration should come before the migration function call
	setIdx := strings.Index(code, `plan.SetCurrentMigration("20260115120000_users")`)
	callIdx := strings.Index(code, `migrations.Migrate_20260115120000_users(plan)`)

	if setIdx == -1 || callIdx == -1 {
		t.Fatal("Could not find SetCurrentMigration or migration function call in generated code")
	}

	if setIdx > callIdx {
		t.Error("SetCurrentMigration must be called BEFORE the migration function")
	}
}

// TestGeneratedRunnerMigrationNamesMatchFilenames verifies that the migration names
// passed to SetCurrentMigration match the expected format: TIMESTAMP_name
func TestGeneratedRunnerMigrationNamesMatchFilenames(t *testing.T) {
	migrations := []codegen.MigrationFile{
		{
			Path:      "/path/to/20260204134211_accounts.go",
			Timestamp: "20260204134211",
			Name:      "accounts",
			FuncName:  "Migrate_20260204134211_accounts",
		},
	}

	code := codegen.GenerateMigrationRunnerForTest(migrations)

	// The migration name should be TIMESTAMP_name (from the filename)
	expectedName := "20260204134211_accounts"
	if !strings.Contains(code, `plan.SetCurrentMigration("`+expectedName+`")`) {
		t.Errorf("Generated runner should use migration name %q derived from filename", expectedName)
		t.Logf("Generated code:\n%s", code)
	}
}
