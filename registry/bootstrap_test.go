package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen/embed"
)

// ── bootstrapQueryPackages tests ─────────────────────────────────────────────

func TestBootstrapQueryPackages_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-sqlite"

	if err := bootstrapQueryPackages(tmpDir, modulePath, "sqlite"); err != nil {
		t.Fatalf("bootstrapQueryPackages failed: %v", err)
	}

	// types.go should exist
	typesPath := filepath.Join(tmpDir, "shipq", "queries", "types.go")
	typesContent, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("types.go not found: %v", err)
	}
	typesStr := string(typesContent)

	// Should declare package queries
	if !strings.Contains(typesStr, "package queries") {
		t.Error("types.go missing 'package queries'")
	}

	// Should have Runner interface with BeginTx
	if !strings.Contains(typesStr, "Runner interface") {
		t.Error("types.go missing Runner interface")
	}
	if !strings.Contains(typesStr, "BeginTx") {
		t.Error("types.go missing BeginTx method")
	}

	// Should have TxRunner struct
	if !strings.Contains(typesStr, "TxRunner") {
		t.Error("types.go missing TxRunner struct")
	}

	// Should have context helpers
	if !strings.Contains(typesStr, "NewContextWithRunner") {
		t.Error("types.go missing NewContextWithRunner")
	}
	if !strings.Contains(typesStr, "RunnerFromContext") {
		t.Error("types.go missing RunnerFromContext")
	}

	// runner.go should exist in dialect directory
	runnerPath := filepath.Join(tmpDir, "shipq", "queries", "sqlite", "runner.go")
	runnerContent, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("runner.go not found: %v", err)
	}
	runnerStr := string(runnerContent)

	// Should declare dialect package
	if !strings.Contains(runnerStr, "package sqlite") {
		t.Error("runner.go missing 'package sqlite'")
	}

	// Should have QueryRunner struct
	if !strings.Contains(runnerStr, "QueryRunner") {
		t.Error("runner.go missing QueryRunner struct")
	}

	// Should have NewQueryRunner constructor
	if !strings.Contains(runnerStr, "NewQueryRunner") {
		t.Error("runner.go missing NewQueryRunner")
	}

	// Should have WithTx method
	if !strings.Contains(runnerStr, "WithTx") {
		t.Error("runner.go missing WithTx")
	}
}

func TestBootstrapQueryPackages_Postgres(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-postgres"

	if err := bootstrapQueryPackages(tmpDir, modulePath, "postgres"); err != nil {
		t.Fatalf("bootstrapQueryPackages failed: %v", err)
	}

	// runner.go should exist in postgres directory
	runnerPath := filepath.Join(tmpDir, "shipq", "queries", "postgres", "runner.go")
	runnerContent, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("runner.go not found: %v", err)
	}

	if !strings.Contains(string(runnerContent), "package postgres") {
		t.Error("runner.go missing 'package postgres'")
	}
}

func TestBootstrapQueryPackages_MySQL(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-mysql"

	if err := bootstrapQueryPackages(tmpDir, modulePath, "mysql"); err != nil {
		t.Fatalf("bootstrapQueryPackages failed: %v", err)
	}

	// runner.go should exist in mysql directory
	runnerPath := filepath.Join(tmpDir, "shipq", "queries", "mysql", "runner.go")
	runnerContent, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("runner.go not found: %v", err)
	}

	if !strings.Contains(string(runnerContent), "package mysql") {
		t.Error("runner.go missing 'package mysql'")
	}
}

func TestBootstrapQueryPackages_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-idempotent"

	// Call twice — should not error on second call
	if err := bootstrapQueryPackages(tmpDir, modulePath, "sqlite"); err != nil {
		t.Fatalf("first bootstrapQueryPackages failed: %v", err)
	}

	// Read first-run content
	typesPath := filepath.Join(tmpDir, "shipq", "queries", "types.go")
	firstContent, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("types.go not found after first call: %v", err)
	}

	if err := bootstrapQueryPackages(tmpDir, modulePath, "sqlite"); err != nil {
		t.Fatalf("second bootstrapQueryPackages failed: %v", err)
	}

	// Content should be identical
	secondContent, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("types.go not found after second call: %v", err)
	}

	if string(firstContent) != string(secondContent) {
		t.Error("types.go content changed between calls — expected idempotent behavior")
	}
}

func TestBootstrapQueryPackages_ModulePathInTypes(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "github.com/company/myapp"

	if err := bootstrapQueryPackages(tmpDir, modulePath, "sqlite"); err != nil {
		t.Fatalf("bootstrapQueryPackages failed: %v", err)
	}

	// The runner.go should reference the module's queries package
	runnerPath := filepath.Join(tmpDir, "shipq", "queries", "sqlite", "runner.go")
	runnerContent, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("runner.go not found: %v", err)
	}

	// The runner imports the queries package for TxRunner
	if !strings.Contains(string(runnerContent), modulePath) {
		t.Errorf("runner.go should reference module path %q, got:\n%s", modulePath, runnerContent)
	}
}

// ── bootstrapPackages tests ──────────────────────────────────────────────────

func TestBootstrapPackages_CreatesLibPackages(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-full"

	// Create a minimal shipq.ini so EnsureDBPackage can read it
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create a minimal go.mod so GetModuleInfo works
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("bootstrapPackages failed: %v", err)
	}

	// Check that key lib packages exist
	requiredDirs := []string{
		filepath.Join("shipq", "lib", "handler"),
		filepath.Join("shipq", "lib", "httpserver"),
		filepath.Join("shipq", "lib", "httputil"),
		filepath.Join("shipq", "lib", "httperror"),
		filepath.Join("shipq", "lib", "logging"),
		filepath.Join("shipq", "lib", "crypto"),
		filepath.Join("shipq", "lib", "nanoid"),
	}

	for _, dir := range requiredDirs {
		fullPath := filepath.Join(tmpDir, dir)
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			t.Errorf("expected %s to exist after bootstrap", dir)
			continue
		}
		if err != nil {
			t.Errorf("error checking %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	// Check that handler directory has at least one .go file
	handlerDir := filepath.Join(tmpDir, "shipq", "lib", "handler")
	entries, err := os.ReadDir(handlerDir)
	if err != nil {
		t.Fatalf("failed to read handler dir: %v", err)
	}
	hasGoFile := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") {
			hasGoFile = true
			break
		}
	}
	if !hasGoFile {
		t.Error("shipq/lib/handler/ should contain at least one .go file")
	}
}

func TestBootstrapPackages_CreatesDBPackage(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-db"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("bootstrapPackages failed: %v", err)
	}

	// Check that shipq/db/db.go exists
	dbGoPath := filepath.Join(tmpDir, "shipq", "db", "db.go")
	content, err := os.ReadFile(dbGoPath)
	if err != nil {
		t.Fatalf("shipq/db/db.go not found: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "package db") {
		t.Error("db.go missing 'package db'")
	}
	if !strings.Contains(contentStr, "sqlite") {
		t.Error("db.go should reference sqlite driver for sqlite dialect")
	}
}

func TestBootstrapPackages_CreatesQueryStubs(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-queries"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("bootstrapPackages failed: %v", err)
	}

	// Check types.go
	typesPath := filepath.Join(tmpDir, "shipq", "queries", "types.go")
	if _, err := os.Stat(typesPath); os.IsNotExist(err) {
		t.Fatal("shipq/queries/types.go not found after bootstrap")
	}

	// Check runner.go
	runnerPath := filepath.Join(tmpDir, "shipq", "queries", "sqlite", "runner.go")
	if _, err := os.Stat(runnerPath); os.IsNotExist(err) {
		t.Fatal("shipq/queries/sqlite/runner.go not found after bootstrap")
	}
}

func TestBootstrapPackages_SkipsDBIfAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-skip-db"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Pre-create db.go with custom content
	dbDir := filepath.Join(tmpDir, "shipq", "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	customContent := []byte("package db\n// custom content\n")
	if err := os.WriteFile(filepath.Join(dbDir, "db.go"), customContent, 0644); err != nil {
		t.Fatalf("failed to write custom db.go: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("bootstrapPackages failed: %v", err)
	}

	// db.go should NOT have been overwritten
	content, err := os.ReadFile(filepath.Join(dbDir, "db.go"))
	if err != nil {
		t.Fatalf("failed to read db.go: %v", err)
	}
	if string(content) != string(customContent) {
		t.Error("db.go was overwritten — bootstrap should skip if file already exists")
	}
}

func TestBootstrapPackages_SkipsQueryStubsIfAlreadyExist(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-skip-queries"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Pre-create types.go with custom content
	queriesDir := filepath.Join(tmpDir, "shipq", "queries")
	if err := os.MkdirAll(queriesDir, 0755); err != nil {
		t.Fatalf("failed to create queries dir: %v", err)
	}
	customContent := []byte("package queries\n// real queries, not stubs\n")
	if err := os.WriteFile(filepath.Join(queriesDir, "types.go"), customContent, 0644); err != nil {
		t.Fatalf("failed to write custom types.go: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("bootstrapPackages failed: %v", err)
	}

	// types.go should NOT have been overwritten
	content, err := os.ReadFile(filepath.Join(queriesDir, "types.go"))
	if err != nil {
		t.Fatalf("failed to read types.go: %v", err)
	}
	if string(content) != string(customContent) {
		t.Error("types.go was overwritten — bootstrap should skip if file already exists")
	}
}

func TestBootstrapPackages_EmptyDialect(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-no-dialect"

	// Create a minimal shipq.ini with no database_url (empty dialect)
	shipqIniContent := "[db]\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// With empty dialect, query stubs should be skipped (no error)
	// EmbedAllPackages defaults empty dialect to "sqlite" internally, so lib
	// packages will still be created.
	if err := bootstrapPackages(tmpDir, modulePath, "", false, false); err != nil {
		t.Fatalf("bootstrapPackages with empty dialect should not error: %v", err)
	}

	// Query stubs should NOT be created when dialect is empty
	typesPath := filepath.Join(tmpDir, "shipq", "queries", "types.go")
	if _, err := os.Stat(typesPath); !os.IsNotExist(err) {
		t.Error("types.go should NOT be created when dialect is empty")
	}
}

func TestBootstrapPackages_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-idem"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// First call
	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("first bootstrapPackages failed: %v", err)
	}

	// Second call should succeed without errors
	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, false); err != nil {
		t.Fatalf("second bootstrapPackages failed: %v", err)
	}

	// Verify files still exist after both calls
	paths := []string{
		filepath.Join(tmpDir, "shipq", "lib", "handler"),
		filepath.Join(tmpDir, "shipq", "lib", "httpserver"),
		filepath.Join(tmpDir, "shipq", "db", "db.go"),
		filepath.Join(tmpDir, "shipq", "queries", "types.go"),
		filepath.Join(tmpDir, "shipq", "queries", "sqlite", "runner.go"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("%s should exist after two bootstrap calls", p)
		}
	}
}

func TestBootstrapPackages_WithFilesEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-files"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n\n[files]\ns3_bucket = test\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", true, false); err != nil {
		t.Fatalf("bootstrapPackages with files enabled failed: %v", err)
	}

	// filestorage lib should exist
	filestorageDir := filepath.Join(tmpDir, "shipq", "lib", "filestorage")
	if _, err := os.Stat(filestorageDir); os.IsNotExist(err) {
		t.Error("shipq/lib/filestorage/ should exist when FilesEnabled is true")
	}
}

func TestBootstrapPackages_WithWorkersEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-workers"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n\n[workers]\nredis_url = redis://localhost:6379\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := bootstrapPackages(tmpDir, modulePath, "sqlite", false, true); err != nil {
		t.Fatalf("bootstrapPackages with workers enabled failed: %v", err)
	}

	// channel lib should exist
	channelDir := filepath.Join(tmpDir, "shipq", "lib", "channel")
	if _, err := os.Stat(channelDir); os.IsNotExist(err) {
		t.Error("shipq/lib/channel/ should exist when WorkersEnabled is true")
	}
}

// ── Verify embed.EmbedAllPackages creates assets ─────────────────────────────

func TestBootstrapPackages_CreatesAssets(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.test-bootstrap-assets"

	// Create a minimal shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite://" + filepath.Join(tmpDir, ".shipq", "data", "test.db") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Use embed.EmbedAllPackages directly (same as bootstrapPackages calls)
	if err := embed.EmbedAllPackages(tmpDir, modulePath, embed.EmbedOptions{DBDialect: "sqlite"}); err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// Assets directory should exist with embed.go
	assetsDir := filepath.Join(tmpDir, "shipq", "assets")
	embedGoPath := filepath.Join(assetsDir, "embed.go")
	if _, err := os.Stat(embedGoPath); os.IsNotExist(err) {
		t.Error("shipq/assets/embed.go should exist after embedding")
	}
}
