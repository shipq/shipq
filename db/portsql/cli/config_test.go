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

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error when shipq.ini is missing")
	}

	// Error should mention shipq.ini
	if !contains(err.Error(), "shipq.ini") {
		t.Errorf("error should mention 'shipq.ini', got: %v", err)
	}
}

func TestLoadConfigWithFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a shipq.ini config file
	iniContent := `[db]
url = postgres://localhost/testdb
migrations = db/migrations
schematypes = gen/types
queries_in = sql/queries
queries_out = gen/sql
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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

	// Create an empty shipq.ini (required)
	iniPath := filepath.Join(tmpDir, "shipq.ini")
	if err := os.WriteFile(iniPath, []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

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
	iniContent := `[db]
url = postgres://localhost/filedb
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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
[db]
; This is also a comment
url = postgres://localhost/testdb

# Another comment
migrations = custom_migrations
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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
	iniContent := `[db]
migrations = custom_migrations
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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

	iniContent := `[db]
url = sqlite:./test.db
scope = org_id
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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

	iniContent := `[db]
url = sqlite:./test.db

[crud.orders]
scope = user_id

[crud.products]
scope = vendor_id
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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

	iniContent := `[db]
url = sqlite:./test.db
scope = org_id

[crud.public_logs]
scope =
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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

	// No [crud] section at all, just minimal [db]
	iniContent := `[db]
url = sqlite:./test.db
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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
			"orders":      "user_id", // Different scope
			"public_logs": "",        // No scope (empty override)
		},
	}

	tests := []struct {
		table    string
		expected string
	}{
		{"orders", "user_id"},  // Table-specific override
		{"public_logs", ""},    // Empty override
		{"users", "org_id"},    // Falls back to global
		{"products", "org_id"}, // Falls back to global
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
	iniContent := `[db]
url = postgres://localhost/myapp
scope = organization_id

[crud.users]
scope =

[crud.user_sessions]
scope = user_id

[crud.audit_logs]
scope = organization_id
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
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
		{"users", ""},                     // Explicitly no scope
		{"user_sessions", "user_id"},      // User-scoped
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

// =============================================================================
// Order Direction Config Tests
// =============================================================================

func TestCRUDConfig_OrderDirection_GlobalDefault(t *testing.T) {
	// Global order direction setting
	content := `[db]
url = sqlite:test.db
order = asc
`
	tmpDir := t.TempDir()
	iniPath := filepath.Join(tmpDir, "shipq.ini")
	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Global order should be "asc"
	if cfg.CRUD.GlobalOrder != "asc" {
		t.Errorf("expected GlobalOrder 'asc', got %q", cfg.CRUD.GlobalOrder)
	}

	// GetOrderForTable should return the global order
	if order := cfg.CRUD.GetOrderForTable("users"); order != "asc" {
		t.Errorf("GetOrderForTable('users') = %q, want 'asc'", order)
	}
}

func TestCRUDConfig_OrderDirection_PerTableOverride(t *testing.T) {
	// Per-table order direction override
	content := `[db]
url = sqlite:test.db
order = desc

[crud.audit_logs]
order = asc
`
	tmpDir := t.TempDir()
	iniPath := filepath.Join(tmpDir, "shipq.ini")
	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Global order should be "desc"
	if cfg.CRUD.GlobalOrder != "desc" {
		t.Errorf("expected GlobalOrder 'desc', got %q", cfg.CRUD.GlobalOrder)
	}

	// audit_logs should use "asc" (override)
	if order := cfg.CRUD.GetOrderForTable("audit_logs"); order != "asc" {
		t.Errorf("GetOrderForTable('audit_logs') = %q, want 'asc'", order)
	}

	// users should use "desc" (global default)
	if order := cfg.CRUD.GetOrderForTable("users"); order != "desc" {
		t.Errorf("GetOrderForTable('users') = %q, want 'desc'", order)
	}
}

func TestCRUDConfig_OrderDirection_DefaultIsDesc(t *testing.T) {
	// When no order is specified, default should be desc (newest first)
	content := `[db]
url = sqlite:test.db
`
	tmpDir := t.TempDir()
	iniPath := filepath.Join(tmpDir, "shipq.ini")
	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Default order should be "desc" when not specified
	if order := cfg.CRUD.GetOrderForTable("users"); order != "desc" {
		t.Errorf("GetOrderForTable('users') = %q, want 'desc' (default)", order)
	}
}

// =============================================================================
// Dialect Validation Tests
// =============================================================================

func TestNormalizeDialect_ValidDialects(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"postgres", "postgres"},
		{"mysql", "mysql"},
		{"sqlite", "sqlite"},
		// Case normalization
		{"Postgres", "postgres"},
		{"POSTGRES", "postgres"},
		{"MySQL", "mysql"},
		{"MYSQL", "mysql"},
		{"SQLite", "sqlite"},
		{"SQLITE", "sqlite"},
		// Whitespace handling
		{" postgres ", "postgres"},
		{"  mysql  ", "mysql"},
		{"\tsqlite\t", "sqlite"},
		// Empty returns empty (for filtering)
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeDialect(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("normalizeDialect(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeDialect_InvalidDialect(t *testing.T) {
	tests := []struct {
		input          string
		wantErrContain string
	}{
		{"postgre", `invalid dialect "postgre"`},
		{"postgress", `invalid dialect "postgress"`},
		{"pg", `invalid dialect "pg"`},
		{"maria", `invalid dialect "maria"`},
		{"sql", `invalid dialect "sql"`},
		{"oracle", `invalid dialect "oracle"`},
		{"mssql", `invalid dialect "mssql"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := normalizeDialect(tt.input)
			if err == nil {
				t.Fatalf("expected error for invalid dialect %q, got nil", tt.input)
			}

			errStr := err.Error()

			// Check error contains the invalid value
			if !contains(errStr, tt.wantErrContain) {
				t.Errorf("error should contain %q, got: %s", tt.wantErrContain, errStr)
			}

			// Check error mentions supported dialects
			if !contains(errStr, "postgres") || !contains(errStr, "mysql") || !contains(errStr, "sqlite") {
				t.Errorf("error should list supported dialects, got: %s", errStr)
			}

			// Check error has helpful hint
			if !contains(errStr, "Hint:") {
				t.Errorf("error should contain hint, got: %s", errStr)
			}
		})
	}
}

func TestLoadConfig_DialectValidation(t *testing.T) {
	tests := []struct {
		name           string
		dialects       string
		wantDialects   []string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:         "valid single dialect",
			dialects:     "postgres",
			wantDialects: []string{"postgres"},
		},
		{
			name:         "valid multiple dialects",
			dialects:     "postgres, mysql, sqlite",
			wantDialects: []string{"postgres", "mysql", "sqlite"},
		},
		{
			name:         "case normalization",
			dialects:     "Postgres, MySQL, SQLite",
			wantDialects: []string{"postgres", "mysql", "sqlite"},
		},
		{
			name:         "trailing comma filtered",
			dialects:     "postgres, sqlite,",
			wantDialects: []string{"postgres", "sqlite"},
		},
		{
			name:         "leading comma filtered",
			dialects:     ", postgres, sqlite",
			wantDialects: []string{"postgres", "sqlite"},
		},
		{
			name:         "multiple commas filtered",
			dialects:     "postgres,, ,sqlite",
			wantDialects: []string{"postgres", "sqlite"},
		},
		{
			name:           "invalid dialect returns error",
			dialects:       "postgres, postgress",
			wantErr:        true,
			wantErrContain: "invalid",
		},
		{
			name:           "typo in dialect returns helpful error",
			dialects:       "sqlit",
			wantErr:        true,
			wantErrContain: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			content := "[db]\n" +
				"url = sqlite:test.db\n" +
				"dialects = " + tt.dialects + "\n"

			iniPath := filepath.Join(tmpDir, "shipq.ini")
			if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write ini file: %v", err)
			}

			cfg, err := LoadConfig(tmpDir)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error should contain %q, got: %s", tt.wantErrContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(cfg.Database.Dialects) != len(tt.wantDialects) {
				t.Fatalf("got %d dialects, want %d: %v", len(cfg.Database.Dialects), len(tt.wantDialects), cfg.Database.Dialects)
			}

			for i, want := range tt.wantDialects {
				if cfg.Database.Dialects[i] != want {
					t.Errorf("dialect[%d] = %q, want %q", i, cfg.Database.Dialects[i], want)
				}
			}
		})
	}
}

// =============================================================================
// Error Message Tests
// =============================================================================

func TestLoadConfig_ErrorMentionsShipqIni(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config with invalid dialect
	iniContent := `[db]
dialects = invalid_dialect
`
	iniPath := filepath.Join(tmpDir, "shipq.ini")
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid dialect")
	}

	errStr := err.Error()
	if !contains(errStr, "shipq.ini") {
		t.Errorf("error should mention 'shipq.ini', got: %v", err)
	}
}

// contains checks if s contains substr (case-sensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
