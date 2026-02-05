package new

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/internal/commands/migrate/generator"
	"github.com/shipq/shipq/internal/commands/migrate/parser"
	"github.com/shipq/shipq/project"
)

func setupTestProject(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory and resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, _ := filepath.EvalSymlinks(t.TempDir())

	// Create go.mod
	goModContent := "module testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create shipq.ini
	shipqIniContent := "[db]\nurl = sqlite://test.db\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to create shipq.ini: %v", err)
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

func TestLoadProjectConfig(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	cfg, err := loadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ProjectRoot != tmpDir {
		t.Errorf("ProjectRoot = %q, want %q", cfg.ProjectRoot, tmpDir)
	}

	expectedMigrationsPath := filepath.Join(tmpDir, "migrations")
	if cfg.MigrationsPath != expectedMigrationsPath {
		t.Errorf("MigrationsPath = %q, want %q", cfg.MigrationsPath, expectedMigrationsPath)
	}
}

func TestLoadProjectConfig_CustomMigrationsPath(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Update shipq.ini with custom migrations path
	shipqIniContent := "[db]\nurl = sqlite://test.db\nmigrations = db/migrations\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to update shipq.ini: %v", err)
	}

	cfg, err := loadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedMigrationsPath := filepath.Join(tmpDir, "db/migrations")
	if cfg.MigrationsPath != expectedMigrationsPath {
		t.Errorf("MigrationsPath = %q, want %q", cfg.MigrationsPath, expectedMigrationsPath)
	}
}

func TestLoadProjectConfig_NotInProject(t *testing.T) {
	// Create temp directory without go.mod
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	_, err := loadProjectConfig()
	if err == nil {
		t.Fatal("expected error when not in a project")
	}
}

func TestLoadProjectConfig_MissingShipqIni(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only go.mod
	goModContent := "module testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	_, err := loadProjectConfig()
	if err == nil {
		t.Fatal("expected error when shipq.ini is missing")
	}

	if err != project.ErrNoShipqIni {
		t.Errorf("expected ErrNoShipqIni, got %v", err)
	}
}

// Helper to create migration and return file path
func createMigrationHelper(t *testing.T, tmpDir string, name string, columns []parser.ColumnSpec) string {
	t.Helper()

	cfg := &ProjectConfig{
		ProjectRoot:    tmpDir,
		MigrationsPath: filepath.Join(tmpDir, "migrations"),
	}

	timestamp := generator.GenerateTimestamp()

	migrationCfg := generator.MigrationConfig{
		PackageName:   "migrations",
		MigrationName: name,
		Timestamp:     timestamp,
		Columns:       columns,
	}

	code, err := generator.GenerateMigration(migrationCfg)
	if err != nil {
		t.Fatalf("failed to generate migration: %v", err)
	}

	// Create migrations directory
	if err := os.MkdirAll(cfg.MigrationsPath, 0755); err != nil {
		t.Fatalf("failed to create migrations directory: %v", err)
	}

	// Write migration file
	fileName := generator.GenerateMigrationFileName(timestamp, name)
	filePath := filepath.Join(cfg.MigrationsPath, fileName)

	if err := os.WriteFile(filePath, code, 0644); err != nil {
		t.Fatalf("failed to write migration file: %v", err)
	}

	return filePath
}

func TestMigrateNew_CreatesFile(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	filePath := createMigrationHelper(t, tmpDir, "users", []parser.ColumnSpec{
		{Name: "name", Type: "string"},
		{Name: "email", Type: "string"},
	})

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("migration file was not created at %s", filePath)
	}

	// Verify file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for expected content
	if !strings.Contains(contentStr, "package migrations") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(contentStr, "func Migrate_") {
		t.Error("missing migration function")
	}
	if !strings.Contains(contentStr, `plan.AddTable("users"`) {
		t.Error("missing AddTable call")
	}
	if !strings.Contains(contentStr, `tb.String("name")`) {
		t.Error("missing name column")
	}
	if !strings.Contains(contentStr, `tb.String("email")`) {
		t.Error("missing email column")
	}
}

func TestMigrateNew_CreatesDirectory(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Verify migrations directory doesn't exist yet
	if _, err := os.Stat(migrationsDir); !os.IsNotExist(err) {
		t.Fatal("migrations directory should not exist yet")
	}

	// Create migration
	createMigrationHelper(t, tmpDir, "users", []parser.ColumnSpec{})

	// Verify migrations directory was created
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Fatal("migrations directory should have been created")
	}
}

func TestMigrateNew_FileNameFormat(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	filePath := createMigrationHelper(t, tmpDir, "users", []parser.ColumnSpec{})

	// Get just the filename
	fileName := filepath.Base(filePath)

	// Should match pattern: YYYYMMDDHHMMSS_users.go
	if len(fileName) < 20 { // 14 digit timestamp + _ + users + .go
		t.Errorf("filename too short: %s", fileName)
	}

	// Check timestamp part (first 14 chars should be digits)
	for i := 0; i < 14; i++ {
		if fileName[i] < '0' || fileName[i] > '9' {
			t.Errorf("timestamp char %d should be digit, got %c in %s", i, fileName[i], fileName)
		}
	}

	// Check underscore
	if fileName[14] != '_' {
		t.Errorf("expected underscore at position 14, got %c in %s", fileName[14], fileName)
	}

	// Check name and extension
	if !strings.HasSuffix(fileName, "_users.go") {
		t.Errorf("filename should end with _users.go, got %s", fileName)
	}
}

func TestMigrateNew_WithReferences(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	filePath := createMigrationHelper(t, tmpDir, "posts", []parser.ColumnSpec{
		{Name: "title", Type: "string"},
		{Name: "user_id", Type: "references", References: "users"},
	})

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for table reference lookup
	if !strings.Contains(contentStr, `usersRef, err := plan.Table("users")`) {
		t.Errorf("missing users table reference lookup in:\n%s", contentStr)
	}

	// Check for reference column
	if !strings.Contains(contentStr, `tb.Bigint("user_id").References(usersRef)`) {
		t.Errorf("missing user_id reference column in:\n%s", contentStr)
	}
}

func TestMigrateNew_EmptyMigration(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t)
	defer cleanup()

	filePath := createMigrationHelper(t, tmpDir, "empty", []parser.ColumnSpec{})

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Should still have valid structure
	if !strings.Contains(contentStr, "package migrations") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(contentStr, `plan.AddTable("empty"`) {
		t.Error("missing AddTable call")
	}
	if !strings.Contains(contentStr, "return nil") {
		t.Error("missing return statement in table builder")
	}
}

func TestMigrateNew_GeneratedCodeIsValidGo(t *testing.T) {
	_, cleanup := setupTestProject(t)
	defer cleanup()

	// Create several migrations with different configurations
	testCases := []struct {
		name    string
		columns []parser.ColumnSpec
	}{
		{"empty", []parser.ColumnSpec{}},
		{"simple", []parser.ColumnSpec{{Name: "name", Type: "string"}}},
		{"multi_col", []parser.ColumnSpec{
			{Name: "name", Type: "string"},
			{Name: "age", Type: "int"},
			{Name: "active", Type: "bool"},
		}},
		{"with_ref", []parser.ColumnSpec{
			{Name: "title", Type: "string"},
			{Name: "user_id", Type: "references", References: "users"},
		}},
		{"multi_ref", []parser.ColumnSpec{
			{Name: "from_id", Type: "references", References: "accounts"},
			{Name: "to_id", Type: "references", References: "accounts"},
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := generator.MigrationConfig{
				PackageName:   "migrations",
				MigrationName: tc.name,
				Timestamp:     generator.GenerateTimestamp(),
				Columns:       tc.columns,
			}

			// GenerateMigration already runs go/format which validates syntax
			_, err := generator.GenerateMigration(cfg)
			if err != nil {
				t.Errorf("generated invalid Go code: %v", err)
			}
		})
	}
}

func TestValidateMigrationName_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"a", false},
		{"_", false},
		{"_a", false},
		{"a1", false},
		{"create_users_table", false},
		{"V1", false},
		{"CamelCase", false},
		{"", true},
		{" ", true},
		{"1", true},
		{"1abc", true},
		{"has-dash", true},
		{"has.dot", true},
		{"has space", true},
		{"has\ttab", true},
		{"has\nnewline", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateMigrationName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMigrationName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
