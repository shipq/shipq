package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Paths.Migrations != "migrations" {
		t.Errorf("expected migrations path 'migrations', got %q", cfg.Paths.Migrations)
	}
	if cfg.Paths.Schematypes != "schematypes" {
		t.Errorf("expected schematypes path 'schematypes', got %q", cfg.Paths.Schematypes)
	}
	if cfg.Paths.QueriesIn != "querydef" {
		t.Errorf("expected queries_in path 'querydef', got %q", cfg.Paths.QueriesIn)
	}
	if cfg.Paths.QueriesOut != "queries" {
		t.Errorf("expected queries_out path 'queries', got %q", cfg.Paths.QueriesOut)
	}
}

func TestLoadConfigNoFile(t *testing.T) {
	// Create a temp directory with no config file
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use defaults
	if cfg.Paths.Migrations != "migrations" {
		t.Errorf("expected default migrations path, got %q", cfg.Paths.Migrations)
	}
}

func TestLoadConfigWithFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file
	iniContent := `[database]
url = postgres://localhost/testdb

[paths]
migrations = db/migrations
schematypes = gen/types
queries_in = sql/queries
queries_out = gen/sql
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.URL != "postgres://localhost/testdb" {
		t.Errorf("expected database URL 'postgres://localhost/testdb', got %q", cfg.Database.URL)
	}
	if cfg.Paths.Migrations != "db/migrations" {
		t.Errorf("expected migrations path 'db/migrations', got %q", cfg.Paths.Migrations)
	}
	if cfg.Paths.Schematypes != "gen/types" {
		t.Errorf("expected schematypes path 'gen/types', got %q", cfg.Paths.Schematypes)
	}
	if cfg.Paths.QueriesIn != "sql/queries" {
		t.Errorf("expected queries_in path 'sql/queries', got %q", cfg.Paths.QueriesIn)
	}
	if cfg.Paths.QueriesOut != "gen/sql" {
		t.Errorf("expected queries_out path 'gen/sql', got %q", cfg.Paths.QueriesOut)
	}
}

func TestLoadConfigDatabaseURLFallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Set DATABASE_URL env var
	oldEnv := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://localhost/envdb")
	defer os.Setenv("DATABASE_URL", oldEnv)

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.URL != "postgres://localhost/envdb" {
		t.Errorf("expected database URL from env, got %q", cfg.Database.URL)
	}
}

func TestLoadConfigFileOverridesEnv(t *testing.T) {
	tmpDir := t.TempDir()

	// Set DATABASE_URL env var
	oldEnv := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://localhost/envdb")
	defer os.Setenv("DATABASE_URL", oldEnv)

	// Create a config file with different URL
	iniContent := `[database]
url = postgres://localhost/filedb
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should override env
	if cfg.Database.URL != "postgres://localhost/filedb" {
		t.Errorf("expected database URL from file, got %q", cfg.Database.URL)
	}
}

func TestLoadConfigComments(t *testing.T) {
	tmpDir := t.TempDir()

	// Config file with comments
	iniContent := `# This is a comment
[database]
; This is also a comment
url = postgres://localhost/testdb

[paths]
# Another comment
migrations = custom_migrations
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.URL != "postgres://localhost/testdb" {
		t.Errorf("expected database URL 'postgres://localhost/testdb', got %q", cfg.Database.URL)
	}
	if cfg.Paths.Migrations != "custom_migrations" {
		t.Errorf("expected migrations path 'custom_migrations', got %q", cfg.Paths.Migrations)
	}
}

func TestLoadConfigPartialOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Config file that only sets some values
	iniContent := `[paths]
migrations = custom_migrations
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only migrations should be overridden
	if cfg.Paths.Migrations != "custom_migrations" {
		t.Errorf("expected migrations path 'custom_migrations', got %q", cfg.Paths.Migrations)
	}
	// Others should remain defaults
	if cfg.Paths.Schematypes != "schematypes" {
		t.Errorf("expected default schematypes path, got %q", cfg.Paths.Schematypes)
	}
	if cfg.Paths.QueriesIn != "querydef" {
		t.Errorf("expected default queries_in path, got %q", cfg.Paths.QueriesIn)
	}
	if cfg.Paths.QueriesOut != "queries" {
		t.Errorf("expected default queries_out path, got %q", cfg.Paths.QueriesOut)
	}
}
