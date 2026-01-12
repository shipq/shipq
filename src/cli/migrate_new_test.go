package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsValidMigrationName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "create_users", true},
		{"single word", "users", true},
		{"with numbers", "add_user_v2", true},
		{"empty", "", false},
		{"starts with number", "2users", false},
		{"has uppercase", "Create_users", false},
		{"has dash", "create-users", false},
		{"has space", "create users", false},
		{"has dot", "create.users", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidMigrationName(tt.input)
			if got != tt.want {
				t.Errorf("isValidMigrationName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMigrateNew(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")

	config := &Config{
		Paths: PathsConfig{
			Migrations: migrationsDir,
		},
	}

	// Create a migration
	path, err := MigrateNew(config, "create_users")
	if err != nil {
		t.Fatalf("MigrateNew failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("migration file was not created: %s", path)
	}

	// Read the file content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	// Verify content contains expected elements
	contentStr := string(content)
	if !strings.Contains(contentStr, "package migrations") {
		t.Error("migration file should have package migrations")
	}
	if !strings.Contains(contentStr, "func Migrate_") {
		t.Error("migration file should have Migrate_ function")
	}
	if !strings.Contains(contentStr, "create_users") {
		t.Error("migration file should contain migration name")
	}
	if !strings.Contains(contentStr, "*migrate.MigrationPlan") {
		t.Error("migration file should have MigrationPlan parameter")
	}
}

func TestMigrateNewInvalidName(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		Paths: PathsConfig{
			Migrations: tmpDir,
		},
	}

	_, err := MigrateNew(config, "Invalid-Name")
	if err == nil {
		t.Error("expected error for invalid migration name")
	}
}

func TestParseMigrationFilename(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		wantTimestamp string
		wantName      string
		wantOk        bool
	}{
		{
			name:          "valid",
			filename:      "20260111153000_create_users.go",
			wantTimestamp: "20260111153000",
			wantName:      "create_users",
			wantOk:        true,
		},
		{
			name:          "valid with numbers",
			filename:      "20260111153000_add_field_v2.go",
			wantTimestamp: "20260111153000",
			wantName:      "add_field_v2",
			wantOk:        true,
		},
		{
			name:     "no extension",
			filename: "20260111153000_create_users",
			wantOk:   false,
		},
		{
			name:     "wrong extension",
			filename: "20260111153000_create_users.txt",
			wantOk:   false,
		},
		{
			name:     "too short",
			filename: "20260111_x.go",
			wantOk:   false,
		},
		{
			name:     "no underscore after timestamp",
			filename: "20260111153000create_users.go",
			wantOk:   false,
		},
		{
			name:     "no name",
			filename: "20260111153000_.go",
			wantOk:   false,
		},
		{
			name:     "non-numeric timestamp",
			filename: "2026011115300a_create_users.go",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp, name, ok := ParseMigrationFilename(tt.filename)
			if ok != tt.wantOk {
				t.Errorf("ParseMigrationFilename(%q) ok = %v, want %v", tt.filename, ok, tt.wantOk)
				return
			}
			if ok {
				if timestamp != tt.wantTimestamp {
					t.Errorf("timestamp = %q, want %q", timestamp, tt.wantTimestamp)
				}
				if name != tt.wantName {
					t.Errorf("name = %q, want %q", name, tt.wantName)
				}
			}
		})
	}
}
