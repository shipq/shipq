package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing shipq.ini")
	}

	if !contains(err.Error(), "shipq.ini not found") {
		t.Errorf("error should mention 'shipq.ini not found', got: %v", err)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", "")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have defaults
	if cfg.DB.Migrations != "migrations" {
		t.Errorf("expected default migrations path 'migrations', got %q", cfg.DB.Migrations)
	}
	if cfg.DB.Schematypes != "schematypes" {
		t.Errorf("expected default schematypes path 'schematypes', got %q", cfg.DB.Schematypes)
	}
	if cfg.DB.QueriesIn != "querydef" {
		t.Errorf("expected default queries_in path 'querydef', got %q", cfg.DB.QueriesIn)
	}
	if cfg.DB.QueriesOut != "queries" {
		t.Errorf("expected default queries_out path 'queries', got %q", cfg.DB.QueriesOut)
	}
}

func TestLoad_DBSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[db]
url = postgres://localhost/mydb
dialects = postgres, mysql
migrations = db/migrations
schematypes = db/schema
queries_in = db/querydef
queries_out = db/queries
scope = tenant_id
order = desc
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DB.URL != "postgres://localhost/mydb" {
		t.Errorf("expected URL 'postgres://localhost/mydb', got %q", cfg.DB.URL)
	}

	if len(cfg.DB.Dialects) != 2 {
		t.Errorf("expected 2 dialects, got %d", len(cfg.DB.Dialects))
	} else {
		if cfg.DB.Dialects[0] != "postgres" {
			t.Errorf("expected first dialect 'postgres', got %q", cfg.DB.Dialects[0])
		}
		if cfg.DB.Dialects[1] != "mysql" {
			t.Errorf("expected second dialect 'mysql', got %q", cfg.DB.Dialects[1])
		}
	}

	if cfg.DB.Migrations != "db/migrations" {
		t.Errorf("expected migrations 'db/migrations', got %q", cfg.DB.Migrations)
	}
	if cfg.DB.Schematypes != "db/schema" {
		t.Errorf("expected schematypes 'db/schema', got %q", cfg.DB.Schematypes)
	}
	if cfg.DB.QueriesIn != "db/querydef" {
		t.Errorf("expected queries_in 'db/querydef', got %q", cfg.DB.QueriesIn)
	}
	if cfg.DB.QueriesOut != "db/queries" {
		t.Errorf("expected queries_out 'db/queries', got %q", cfg.DB.QueriesOut)
	}
	if cfg.DB.GlobalScope != "tenant_id" {
		t.Errorf("expected global scope 'tenant_id', got %q", cfg.DB.GlobalScope)
	}
	if cfg.DB.GlobalOrder != "desc" {
		t.Errorf("expected global order 'desc', got %q", cfg.DB.GlobalOrder)
	}
}

func TestLoad_DialectValidation(t *testing.T) {
	tests := []struct {
		name         string
		dialects     string
		wantErr      bool
		wantDialects []string
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
			name:         "case insensitive",
			dialects:     "POSTGRES, MySQL, SQLite",
			wantDialects: []string{"postgres", "mysql", "sqlite"},
		},
		{
			name:         "extra spaces",
			dialects:     "  postgres  ,  mysql  ",
			wantDialects: []string{"postgres", "mysql"},
		},
		{
			name:         "trailing comma",
			dialects:     "postgres,",
			wantDialects: []string{"postgres"},
		},
		{
			name:     "invalid dialect",
			dialects: "pg",
			wantErr:  true,
		},
		{
			name:     "mixed valid and invalid",
			dialects: "postgres, mariadb",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "shipq.ini", "[db]\ndialects = "+tt.dialects)

			cfg, err := Load(dir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(cfg.DB.Dialects) != len(tt.wantDialects) {
				t.Errorf("expected %d dialects, got %d", len(tt.wantDialects), len(cfg.DB.Dialects))
				return
			}

			for i, d := range tt.wantDialects {
				if cfg.DB.Dialects[i] != d {
					t.Errorf("dialect[%d]: expected %q, got %q", i, d, cfg.DB.Dialects[i])
				}
			}
		})
	}
}

func TestLoad_InvalidDialect_ErrorMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", "[db]\ndialects = pg")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid dialect")
	}

	errStr := err.Error()
	if !contains(errStr, "shipq.ini") {
		t.Errorf("error should mention 'shipq.ini', got: %v", err)
	}
	if !contains(errStr, "db.dialects") {
		t.Errorf("error should mention 'db.dialects', got: %v", err)
	}
	if !contains(errStr, "pg") {
		t.Errorf("error should mention the invalid value 'pg', got: %v", err)
	}
}

func TestLoad_APISection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
middleware_package = ./middleware
openapi = true
openapi_output = api.json
openapi_title = My API
openapi_version = 1.0.0
openapi_description = API description
openapi_servers = http://localhost:8080, https://api.example.com
docs_ui = true
docs_path = /docs
openapi_json_path = /api.json
test_client = true
test_client_filename = client_test.go
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.API.Package != "./api" {
		t.Errorf("expected package './api', got %q", cfg.API.Package)
	}
	if cfg.API.MiddlewarePackage != "./middleware" {
		t.Errorf("expected middleware_package './middleware', got %q", cfg.API.MiddlewarePackage)
	}
	if !cfg.API.OpenAPIEnabled {
		t.Error("expected openapi to be enabled")
	}
	if cfg.API.OpenAPIOutput != "api.json" {
		t.Errorf("expected openapi_output 'api.json', got %q", cfg.API.OpenAPIOutput)
	}
	if cfg.API.OpenAPITitle != "My API" {
		t.Errorf("expected openapi_title 'My API', got %q", cfg.API.OpenAPITitle)
	}
	if cfg.API.OpenAPIVersion != "1.0.0" {
		t.Errorf("expected openapi_version '1.0.0', got %q", cfg.API.OpenAPIVersion)
	}
	if cfg.API.OpenAPIDescription != "API description" {
		t.Errorf("expected openapi_description 'API description', got %q", cfg.API.OpenAPIDescription)
	}
	if len(cfg.API.OpenAPIServers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(cfg.API.OpenAPIServers))
	} else {
		if cfg.API.OpenAPIServers[0] != "http://localhost:8080" {
			t.Errorf("expected first server 'http://localhost:8080', got %q", cfg.API.OpenAPIServers[0])
		}
		if cfg.API.OpenAPIServers[1] != "https://api.example.com" {
			t.Errorf("expected second server 'https://api.example.com', got %q", cfg.API.OpenAPIServers[1])
		}
	}
	if !cfg.API.DocsUIEnabled {
		t.Error("expected docs_ui to be enabled")
	}
	if cfg.API.DocsPath != "/docs" {
		t.Errorf("expected docs_path '/docs', got %q", cfg.API.DocsPath)
	}
	if cfg.API.OpenAPIJSONPath != "/api.json" {
		t.Errorf("expected openapi_json_path '/api.json', got %q", cfg.API.OpenAPIJSONPath)
	}
	if !cfg.API.TestClientEnabled {
		t.Error("expected test_client to be enabled")
	}
	if cfg.API.TestClientFilename != "client_test.go" {
		t.Errorf("expected test_client_filename 'client_test.go', got %q", cfg.API.TestClientFilename)
	}
}

func TestLoad_APIDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", "[api]\npackage = ./api")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.API.OpenAPIOutput != "openapi.json" {
		t.Errorf("expected default openapi_output 'openapi.json', got %q", cfg.API.OpenAPIOutput)
	}
	if cfg.API.OpenAPIVersion != "0.0.0" {
		t.Errorf("expected default openapi_version '0.0.0', got %q", cfg.API.OpenAPIVersion)
	}
	if cfg.API.OpenAPIJSONPath != "/openapi.json" {
		t.Errorf("expected default openapi_json_path '/openapi.json', got %q", cfg.API.OpenAPIJSONPath)
	}
	if cfg.API.TestClientFilename != "zz_generated_testclient_test.go" {
		t.Errorf("expected default test_client_filename 'zz_generated_testclient_test.go', got %q", cfg.API.TestClientFilename)
	}
}

func TestLoad_DocsUIImpliesOpenAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
docs_ui = true
docs_path = /docs
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.API.OpenAPIEnabled {
		t.Error("docs_ui=true should imply openapi=true")
	}
}

func TestLoad_DocsPathRequired(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
docs_ui = true
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when docs_ui is enabled without docs_path")
	}

	if !contains(err.Error(), "docs_path") {
		t.Errorf("error should mention 'docs_path', got: %v", err)
	}
}

func TestLoad_DocsPathValidation(t *testing.T) {
	tests := []struct {
		name     string
		docsPath string
		wantErr  bool
		wantPath string
	}{
		{
			name:     "valid path",
			docsPath: "/docs",
			wantPath: "/docs",
		},
		{
			name:     "trailing slash removed",
			docsPath: "/docs/",
			wantPath: "/docs",
		},
		{
			name:     "root path rejected",
			docsPath: "/",
			wantErr:  true,
		},
		{
			name:     "missing leading slash",
			docsPath: "docs",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
docs_ui = true
docs_path = `+tt.docsPath)

			cfg, err := Load(dir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.API.DocsPath != tt.wantPath {
				t.Errorf("expected docs_path %q, got %q", tt.wantPath, cfg.API.DocsPath)
			}
		})
	}
}

func TestLoad_OpenAPIJSONPathValidation(t *testing.T) {
	tests := []struct {
		name     string
		jsonPath string
		wantErr  bool
	}{
		{
			name:     "valid path",
			jsonPath: "/openapi.json",
		},
		{
			name:     "missing leading slash",
			jsonPath: "openapi.json",
			wantErr:  true,
		},
		{
			name:     "trailing slash",
			jsonPath: "/openapi/",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = `+tt.jsonPath)

			_, err := Load(dir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoad_TestClientFilenameValidation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
test_client_filename = client.go
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid test_client_filename")
	}

	if !contains(err.Error(), "_test.go") {
		t.Errorf("error should mention '_test.go', got: %v", err)
	}
}

func TestLoad_BooleanParsing(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"false", "false", false},
		{"FALSE", "FALSE", false},
		{"0", "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
test_client = `+tt.value)

			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.API.TestClientEnabled != tt.want {
				t.Errorf("expected test_client=%v, got %v", tt.want, cfg.API.TestClientEnabled)
			}
		})
	}
}

func TestLoad_InvalidBoolean(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
test_client = yes
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid boolean value")
	}

	if !contains(err.Error(), "test_client") {
		t.Errorf("error should mention the key, got: %v", err)
	}
}

func TestLoad_CRUDSections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[db]
scope = org_id
order = desc

[crud.users]
scope =
order = asc

[crud.posts]
scope = user_id
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DB.GlobalScope != "org_id" {
		t.Errorf("expected global scope 'org_id', got %q", cfg.DB.GlobalScope)
	}
	if cfg.DB.GlobalOrder != "desc" {
		t.Errorf("expected global order 'desc', got %q", cfg.DB.GlobalOrder)
	}

	// users table has empty scope (overrides global)
	if scope, ok := cfg.DB.TableScopes["users"]; !ok {
		t.Error("expected users table scope to exist")
	} else if scope != "" {
		t.Errorf("expected users table scope to be empty, got %q", scope)
	}

	// users table has order override
	if order, ok := cfg.DB.TableOrders["users"]; !ok {
		t.Error("expected users table order to exist")
	} else if order != "asc" {
		t.Errorf("expected users table order 'asc', got %q", order)
	}

	// posts table has scope override
	if scope, ok := cfg.DB.TableScopes["posts"]; !ok {
		t.Error("expected posts table scope to exist")
	} else if scope != "user_id" {
		t.Errorf("expected posts table scope 'user_id', got %q", scope)
	}
}

func TestLoad_DatabaseURLEnvFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", "[db]")

	// Set env var
	oldEnv := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://env/mydb")
	defer os.Setenv("DATABASE_URL", oldEnv)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DB.URL != "postgres://env/mydb" {
		t.Errorf("expected URL from env, got %q", cfg.DB.URL)
	}
}

func TestLoad_ConfigOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[db]
url = postgres://config/mydb
`)

	// Set env var
	oldEnv := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://env/mydb")
	defer os.Setenv("DATABASE_URL", oldEnv)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DB.URL != "postgres://config/mydb" {
		t.Errorf("expected URL from config, got %q", cfg.DB.URL)
	}
}

func TestLoad_ConfigDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", "")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ConfigDir != dir {
		t.Errorf("expected ConfigDir %q, got %q", dir, cfg.ConfigDir)
	}
}

func TestLoad_OpenAPIOutputAbsolutePathRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./api
openapi = true
openapi_output = /absolute/path.json
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for absolute openapi_output path")
	}

	if !contains(err.Error(), "relative path") {
		t.Errorf("error should mention 'relative path', got: %v", err)
	}
}

func TestLoad_DeriveAPITitle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shipq.ini", `
[api]
package = ./internal/api
openapi = true
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.API.OpenAPITitle != "api" {
		t.Errorf("expected derived title 'api', got %q", cfg.API.OpenAPITitle)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()

	// Should not exist initially
	exists, err := Exists(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected shipq.ini to not exist")
	}

	// Create the file
	writeFile(t, dir, "shipq.ini", "")

	// Should exist now
	exists, err = Exists(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected shipq.ini to exist")
	}
}

// Helper functions

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
