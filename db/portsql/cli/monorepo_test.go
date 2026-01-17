package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Monorepo Integration Tests
// =============================================================================

// TestMonorepo_ModulePathFromSubpackages verifies that getModulePath() correctly
// finds go.mod in a monorepo structure and returns the proper module root.
func TestMonorepo_ModulePathFromSubpackages(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create monorepo structure
	monorepo := setupTestMonorepo(t)
	defer os.RemoveAll(monorepo.rootDir)

	tests := []struct {
		name         string
		subDir       string
		wantModule   string
		wantRoot     string
		wantDialects []string
	}{
		{
			name:         "service-mysql subpackage",
			subDir:       "service-mysql",
			wantModule:   "github.com/test/monorepo",
			wantRoot:     monorepo.rootDir,
			wantDialects: []string{"mysql"},
		},
		{
			name:         "service-sqlite subpackage",
			subDir:       "service-sqlite",
			wantModule:   "github.com/test/monorepo",
			wantRoot:     monorepo.rootDir,
			wantDialects: []string{"sqlite"},
		},
		{
			name:         "nested subpackage",
			subDir:       filepath.Join("service-mysql", "migrations"),
			wantModule:   "github.com/test/monorepo",
			wantRoot:     monorepo.rootDir,
			wantDialects: nil, // no portsql.ini in migrations dir
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetDir := filepath.Join(monorepo.rootDir, tt.subDir)
			if err := os.Chdir(targetDir); err != nil {
				t.Fatalf("failed to change to %s: %v", targetDir, err)
			}

			// Test module path discovery
			modulePath, moduleRoot, err := getModulePath()
			if err != nil {
				t.Fatalf("getModulePath() error: %v", err)
			}

			if modulePath != tt.wantModule {
				t.Errorf("modulePath = %q, want %q", modulePath, tt.wantModule)
			}

			if moduleRoot != tt.wantRoot {
				t.Errorf("moduleRoot = %q, want %q", moduleRoot, tt.wantRoot)
			}
		})
	}
}

// TestMonorepo_ImportPathCalculation verifies that import paths are correctly
// calculated relative to the module root, not the CWD.
func TestMonorepo_ImportPathCalculation(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create monorepo structure
	monorepo := setupTestMonorepo(t)
	defer os.RemoveAll(monorepo.rootDir)

	tests := []struct {
		name           string
		subDir         string
		relPath        string // path relative to subDir
		wantImportPath string
	}{
		{
			name:           "service-mysql queries",
			subDir:         "service-mysql",
			relPath:        "queries",
			wantImportPath: "github.com/test/monorepo/service-mysql/queries",
		},
		{
			name:           "service-sqlite migrations",
			subDir:         "service-sqlite",
			relPath:        "migrations",
			wantImportPath: "github.com/test/monorepo/service-sqlite/migrations",
		},
		{
			name:           "service-mysql schematypes",
			subDir:         "service-mysql",
			relPath:        "schematypes",
			wantImportPath: "github.com/test/monorepo/service-mysql/schematypes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetDir := filepath.Join(monorepo.rootDir, tt.subDir)
			if err := os.Chdir(targetDir); err != nil {
				t.Fatalf("failed to change to %s: %v", targetDir, err)
			}

			// Get module info
			modulePath, moduleRoot, err := getModulePath()
			if err != nil {
				t.Fatalf("getModulePath() error: %v", err)
			}

			// Calculate import path the same way the CLI does
			absPath, err := filepath.Abs(tt.relPath)
			if err != nil {
				t.Fatalf("filepath.Abs() error: %v", err)
			}

			relFromModule, err := filepath.Rel(moduleRoot, absPath)
			if err != nil {
				t.Fatalf("filepath.Rel() error: %v", err)
			}

			importPath := modulePath + "/" + filepath.ToSlash(relFromModule)

			if importPath != tt.wantImportPath {
				t.Errorf("importPath = %q, want %q", importPath, tt.wantImportPath)
			}
		})
	}
}

// TestMonorepo_DialectConfiguration verifies that each subpackage can have
// its own dialect configuration via portsql.ini.
func TestMonorepo_DialectConfiguration(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create monorepo structure
	monorepo := setupTestMonorepo(t)
	defer os.RemoveAll(monorepo.rootDir)

	tests := []struct {
		name         string
		subDir       string
		wantDialects []string
	}{
		{
			name:         "service-mysql uses mysql dialect",
			subDir:       "service-mysql",
			wantDialects: []string{"mysql"},
		},
		{
			name:         "service-sqlite uses sqlite dialect",
			subDir:       "service-sqlite",
			wantDialects: []string{"sqlite"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetDir := filepath.Join(monorepo.rootDir, tt.subDir)
			if err := os.Chdir(targetDir); err != nil {
				t.Fatalf("failed to change to %s: %v", targetDir, err)
			}

			// Load config from current directory
			config, err := LoadConfig("")
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}

			dialects := config.Database.GetDialects()

			if len(dialects) != len(tt.wantDialects) {
				t.Fatalf("got %d dialects, want %d", len(dialects), len(tt.wantDialects))
			}

			for i, d := range dialects {
				if d != tt.wantDialects[i] {
					t.Errorf("dialect[%d] = %q, want %q", i, d, tt.wantDialects[i])
				}
			}
		})
	}
}

// testMonorepo holds paths for a test monorepo structure.
type testMonorepo struct {
	rootDir       string
	serviceMysql  string
	serviceSqlite string
}

// setupTestMonorepo creates a temporary monorepo directory structure for testing.
func setupTestMonorepo(t *testing.T) *testMonorepo {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "portsql-monorepo-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create root go.mod
	goModContent := "module github.com/test/monorepo\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create service-mysql structure
	serviceMysql := filepath.Join(tmpDir, "service-mysql")
	if err := os.MkdirAll(filepath.Join(serviceMysql, "migrations"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(serviceMysql, "queries"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(serviceMysql, "schematypes"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	mysqlConfig := `[database]
dialects = mysql

[paths]
migrations = migrations
schematypes = schematypes
queries_in = querydef
queries_out = queries
`
	if err := os.WriteFile(filepath.Join(serviceMysql, "portsql.ini"), []byte(mysqlConfig), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create service-sqlite structure
	serviceSqlite := filepath.Join(tmpDir, "service-sqlite")
	if err := os.MkdirAll(filepath.Join(serviceSqlite, "migrations"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(serviceSqlite, "queries"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(serviceSqlite, "schematypes"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	sqliteConfig := `[database]
dialects = sqlite

[paths]
migrations = migrations
schematypes = schematypes
queries_in = querydef
queries_out = queries
`
	if err := os.WriteFile(filepath.Join(serviceSqlite, "portsql.ini"), []byte(sqliteConfig), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	return &testMonorepo{
		rootDir:       tmpDir,
		serviceMysql:  serviceMysql,
		serviceSqlite: serviceSqlite,
	}
}
