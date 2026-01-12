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

// =============================================================================
// CRUD Config Tests
// =============================================================================

func TestCRUDConfig_GlobalScope(t *testing.T) {
	tmpDir := t.TempDir()

	iniContent := `[database]
url = sqlite:./test.db

[crud]
scope = org_id
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.CRUD.GlobalScope != "org_id" {
		t.Errorf("expected GlobalScope 'org_id', got %q", cfg.CRUD.GlobalScope)
	}
}

func TestCRUDConfig_PerTableScope(t *testing.T) {
	tmpDir := t.TempDir()

	iniContent := `[database]
url = sqlite:./test.db

[crud.orders]
scope = user_id

[crud.products]
scope = vendor_id
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if scope := cfg.CRUD.TableScopes["orders"]; scope != "user_id" {
		t.Errorf("expected orders scope 'user_id', got %q", scope)
	}
	if scope := cfg.CRUD.TableScopes["products"]; scope != "vendor_id" {
		t.Errorf("expected products scope 'vendor_id', got %q", scope)
	}
}

func TestCRUDConfig_EmptyScopeOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	iniContent := `[database]
url = sqlite:./test.db

[crud]
scope = org_id

[crud.public_logs]
scope = 
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Global scope should be set
	if cfg.CRUD.GlobalScope != "org_id" {
		t.Errorf("expected GlobalScope 'org_id', got %q", cfg.CRUD.GlobalScope)
	}

	// public_logs should have empty scope (overriding global)
	if scope, exists := cfg.CRUD.TableScopes["public_logs"]; !exists {
		t.Error("expected public_logs to be in TableScopes")
	} else if scope != "" {
		t.Errorf("expected public_logs scope to be empty, got %q", scope)
	}

	// GetScopeForTable should return empty for public_logs (override)
	if scope := cfg.CRUD.GetScopeForTable("public_logs"); scope != "" {
		t.Errorf("expected GetScopeForTable('public_logs') to return empty, got %q", scope)
	}

	// GetScopeForTable should return global scope for other tables
	if scope := cfg.CRUD.GetScopeForTable("users"); scope != "org_id" {
		t.Errorf("expected GetScopeForTable('users') to return 'org_id', got %q", scope)
	}
}

func TestCRUDConfig_NoConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// No [crud] section at all
	iniContent := `[database]
url = sqlite:./test.db
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GlobalScope should be empty
	if cfg.CRUD.GlobalScope != "" {
		t.Errorf("expected empty GlobalScope, got %q", cfg.CRUD.GlobalScope)
	}

	// GetScopeForTable should return empty for any table
	if scope := cfg.CRUD.GetScopeForTable("users"); scope != "" {
		t.Errorf("expected GetScopeForTable to return empty without config, got %q", scope)
	}
}

func TestCRUDConfig_GetScopeForTable_Priority(t *testing.T) {
	cfg := CRUDConfig{
		GlobalScope: "org_id",
		TableScopes: map[string]string{
			"orders":      "user_id",  // Different scope
			"public_logs": "",         // No scope (empty override)
		},
	}

	tests := []struct {
		table    string
		expected string
	}{
		{"orders", "user_id"},       // Table-specific override
		{"public_logs", ""},         // Empty override
		{"users", "org_id"},         // Falls back to global
		{"products", "org_id"},      // Falls back to global
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			got := cfg.GetScopeForTable(tt.table)
			if got != tt.expected {
				t.Errorf("GetScopeForTable(%q) = %q, want %q", tt.table, got, tt.expected)
			}
		})
	}
}

func TestCRUDConfig_HasTableOverride(t *testing.T) {
	cfg := CRUDConfig{
		GlobalScope: "org_id",
		TableScopes: map[string]string{
			"orders":      "user_id",
			"public_logs": "", // Empty is still an override
		},
	}

	if !cfg.HasTableOverride("orders") {
		t.Error("expected HasTableOverride('orders') to be true")
	}
	if !cfg.HasTableOverride("public_logs") {
		t.Error("expected HasTableOverride('public_logs') to be true (empty override)")
	}
	if cfg.HasTableOverride("users") {
		t.Error("expected HasTableOverride('users') to be false")
	}
}

func TestCRUDConfig_ComplexMultiTenant(t *testing.T) {
	tmpDir := t.TempDir()

	// Multi-tenant SaaS example
	iniContent := `[database]
url = postgres://localhost/myapp

[crud]
scope = organization_id

[crud.users]
scope = 

[crud.user_sessions]
scope = user_id

[crud.audit_logs]
scope = organization_id
`
	iniPath := filepath.Join(tmpDir, "portsql.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify each table's scope
	tests := []struct {
		table    string
		expected string
	}{
		{"users", ""},                // Explicitly no scope
		{"user_sessions", "user_id"}, // User-scoped
		{"audit_logs", "organization_id"}, // Same as global
		{"products", "organization_id"},   // Falls back to global
		{"orders", "organization_id"},     // Falls back to global
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			got := cfg.CRUD.GetScopeForTable(tt.table)
			if got != tt.expected {
				t.Errorf("GetScopeForTable(%q) = %q, want %q", tt.table, got, tt.expected)
			}
		})
	}
}
