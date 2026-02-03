//go:build integration

package cli

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestE2E_FullResourceWorkflow tests the complete workflow:
// 1. Create a new project with migrations
// 2. Run `shipq db migrate up` to create schema
// 3. Run `shipq api resource users` to generate handlers
// 4. Run `shipq api` to generate HTTP runtime
// 5. Verify the generated code compiles
func TestE2E_FullResourceWorkflow(t *testing.T) {
	// Get the shipq module path BEFORE changing directories
	shipqModulePath := findShipqModulePath(t)
	if shipqModulePath == "" {
		t.Skip("Could not find shipq module path")
	}

	tmpDir := t.TempDir()

	// === Step 1: Set up project structure ===
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a migration file that looks like the demo one
	migrationCode := `package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260203112437_accounts(plan *migrate.MigrationPlan) error {
	plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email").Unique()
		return nil
	})
	return nil
}
`
	migrationPath := filepath.Join(migrationsDir, "20260203112437_accounts.go")
	if err := os.WriteFile(migrationPath, []byte(migrationCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create shipq.ini with SQLite for simplicity
	dbPath := filepath.Join(tmpDir, "test.db")
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://` + dbPath + `
dialects = sqlite
migrations = migrations
queries_out = queries
name = testdb

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod for the temp project
	goMod := `module testworkflow

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + shipqModulePath + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an empty querydef package (required by compile)
	querydefDir := filepath.Join(tmpDir, "querydef")
	if err := os.MkdirAll(querydefDir, 0755); err != nil {
		t.Fatal(err)
	}
	querydefCode := `package querydef

// Empty querydef package - no custom queries defined
`
	if err := os.WriteFile(filepath.Join(querydefDir, "queries.go"), []byte(querydefCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to project directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Build shipq binary first (from the source directory, not the test project)
	t.Log("Building shipq binary...")
	shipqBinary := filepath.Join(tmpDir, "shipq-test")
	cmd := exec.Command("go", "build", "-o", shipqBinary, "./cmd/shipq")
	cmd.Dir = shipqModulePath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build shipq: %v\nOutput: %s", err, output)
	}

	// === Step 2: Run go mod tidy, then shipq db migrate up ===
	t.Log("Running: go mod tidy")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\nOutput: %s", err, output)
	}

	t.Log("Running: shipq db migrate up")
	cmd = exec.Command(shipqBinary, "db", "migrate", "up")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shipq db migrate up failed: %v\nOutput: %s", err, output)
	}
	t.Logf("migrate up output: %s", output)

	// Verify schema.json was created
	schemaPath := filepath.Join(migrationsDir, "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Fatalf("expected schema.json to be created at %s", schemaPath)
	}

	// === Step 2b: Run shipq db compile to generate queries package ===
	t.Log("Running: shipq db compile")
	cmd = exec.Command(shipqBinary, "db", "compile")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shipq db compile failed: %v\nOutput: %s", err, output)
	}
	t.Logf("compile output: %s", output)

	// Verify queries package was generated
	queriesDir := filepath.Join(tmpDir, "queries")
	if _, err := os.Stat(queriesDir); os.IsNotExist(err) {
		t.Fatalf("expected queries directory to be created at %s", queriesDir)
	}

	// === Step 2c: Run shipq db setup to generate db/generated package ===
	t.Log("Running: shipq db setup")
	cmd = exec.Command(shipqBinary, "db", "setup")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shipq db setup failed: %v\nOutput: %s", err, output)
	}
	t.Logf("setup output: %s", output)

	// === Step 3: Run shipq api resource users ===
	// This now automatically generates the HTTP runtime (zz_generated_http.go)
	// in addition to the resource handlers.
	t.Log("Running: shipq api resource users (now auto-generates HTTP runtime)")
	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"users"}, opts)
	if exitCode != ExitSuccess {
		t.Fatalf("runResource() exit code = %d, want %d\nstdout: %s\nstderr: %s",
			exitCode, ExitSuccess, stdout.String(), stderr.String())
	}
	t.Logf("resource output: %s", stdout.String())

	// Verify handlers were created
	handlersPath := filepath.Join(tmpDir, "api", "resources", "users", "handlers.go")
	if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
		t.Fatalf("expected handlers.go to be created at %s", handlersPath)
	}

	// Verify HTTP runtime was automatically generated by resource command
	httpPath := filepath.Join(tmpDir, "api", "zz_generated_http.go")
	if _, err := os.Stat(httpPath); os.IsNotExist(err) {
		t.Fatalf("expected zz_generated_http.go to be auto-generated at %s", httpPath)
	}

	// Verify the output mentions HTTP runtime generation
	outputStr := stdout.String()
	if !strings.Contains(outputStr, "Generating HTTP runtime") {
		t.Errorf("expected output to mention 'Generating HTTP runtime', got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "zz_generated_http.go") {
		t.Errorf("expected output to mention 'zz_generated_http.go', got:\n%s", outputStr)
	}

	// === Step 5: Verify generated code compiles ===
	t.Log("Running: go mod tidy")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("go mod tidy output (may have warnings): %s", output)
		// Don't fail - go mod tidy may have issues with version resolution
	}

	t.Log("Running: go build ./...")
	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Read all generated files for debugging
		handlersContent, _ := os.ReadFile(handlersPath)
		httpContent, _ := os.ReadFile(httpPath)
		registerContent, _ := os.ReadFile(filepath.Join(tmpDir, "api", "zz_generated_register.go"))

		// Read queries files
		queriesTypesContent, _ := os.ReadFile(filepath.Join(queriesDir, "types.go"))

		// List queries subdirectories
		queriesEntries, _ := os.ReadDir(queriesDir)
		var queriesFiles []string
		for _, e := range queriesEntries {
			queriesFiles = append(queriesFiles, e.Name())
		}

		t.Fatalf("go build failed: %v\nOutput: %s\n\n"+
			"=== handlers.go ===\n%s\n\n"+
			"=== zz_generated_http.go ===\n%s\n\n"+
			"=== zz_generated_register.go ===\n%s\n\n"+
			"=== queries/types.go ===\n%s\n\n"+
			"=== queries/ contents ===\n%v",
			err, output, handlersContent, httpContent, registerContent, queriesTypesContent, queriesFiles)
	}

	t.Log("SUCCESS: All generated code compiles!")
}

// TestE2E_FullResourceWorkflow_WithDB tests that the generated code actually works
// against a real database.
func TestE2E_FullResourceWorkflow_WithDB(t *testing.T) {
	// Get the shipq module path BEFORE changing directories
	shipqModulePath := findShipqModulePath(t)
	if shipqModulePath == "" {
		t.Skip("Could not find shipq module path")
	}

	tmpDir := t.TempDir()

	// === Step 1: Set up project structure ===
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a migration file
	migrationCode := `package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260203112437_accounts(plan *migrate.MigrationPlan) error {
	plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email").Unique()
		return nil
	})
	return nil
}
`
	migrationPath := filepath.Join(migrationsDir, "20260203112437_accounts.go")
	if err := os.WriteFile(migrationPath, []byte(migrationCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create shipq.ini with SQLite
	dbPath := filepath.Join(tmpDir, "test.db")
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://` + dbPath + `
dialects = sqlite
migrations = migrations
queries_out = queries
name = testdb

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod
	goMod := `module testworkflow

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + shipqModulePath + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty querydef package
	querydefDir := filepath.Join(tmpDir, "querydef")
	if err := os.MkdirAll(querydefDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(querydefDir, "queries.go"), []byte("package querydef\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Build shipq binary first
	shipqBinary := filepath.Join(tmpDir, "shipq-test")
	cmd := exec.Command("go", "build", "-o", shipqBinary, "./cmd/shipq")
	cmd.Dir = shipqModulePath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build shipq: %v\nOutput: %s", err, output)
	}

	// === Step 2: Run go mod tidy, then migrations ===
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\nOutput: %s", err, output)
	}

	cmd = exec.Command(shipqBinary, "db", "migrate", "up")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shipq db migrate up failed: %v\nOutput: %s", err, output)
	}

	// Run db compile to generate queries package
	cmd = exec.Command(shipqBinary, "db", "compile")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shipq db compile failed: %v\nOutput: %s", err, output)
	}

	// Run db setup to generate db/generated package
	cmd = exec.Command(shipqBinary, "db", "setup")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shipq db setup failed: %v\nOutput: %s", err, output)
	}

	// === Step 3: Verify DB has the users table ===
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Check that users table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	if err != nil {
		t.Fatalf("users table not found in database: %v", err)
	}

	// Check columns
	rows, err := db.Query("PRAGMA table_info(users)")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		columns = append(columns, name)
	}

	t.Logf("Users table columns: %v", columns)

	// Verify expected columns exist
	expectedCols := []string{"id", "public_id", "created_at", "updated_at", "deleted_at", "name", "email"}
	for _, expected := range expectedCols {
		found := false
		for _, col := range columns {
			if col == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected column %q not found in users table", expected)
		}
	}

	// === Step 4: Generate resource and API ===
	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	if exitCode := runResource([]string{"users"}, opts); exitCode != ExitSuccess {
		t.Fatalf("runResource failed: %s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := runGenerator(&stdout, &stderr); err != nil {
		t.Fatalf("runGenerator failed: %v\nstderr: %s", err, stderr.String())
	}

	// === Step 5: Build and verify ===
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	cmd.CombinedOutput() // Ignore errors

	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		handlersContent, _ := os.ReadFile(filepath.Join(tmpDir, "api", "resources", "users", "handlers.go"))
		t.Fatalf("go build failed: %v\nOutput: %s\n\nhandlers.go:\n%s", err, output, handlersContent)
	}

	t.Log("SUCCESS: Generated code compiles and database is properly set up!")
}

// TestE2E_ResourceGeneration_QueriesInterface verifies that the generated handlers
// use the correct queries interface pattern.
func TestE2E_ResourceGeneration_QueriesInterface(t *testing.T) {
	shipqModulePath := findShipqModulePath(t)
	if shipqModulePath == "" {
		t.Skip("Could not find shipq module path")
	}

	tmpDir := t.TempDir()

	// Set up minimal project
	migrationsDir := filepath.Join(tmpDir, "migrations")
	os.MkdirAll(migrationsDir, 0755)

	// Create migration file with proper naming
	migrationCode := `package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260203112437_accounts(plan *migrate.MigrationPlan) error {
	plan.AddTable("accounts", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	return nil
}
`
	os.WriteFile(filepath.Join(migrationsDir, "20260203112437_accounts.go"), []byte(migrationCode), 0644)

	dbPath := filepath.Join(tmpDir, "test.db")
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://` + dbPath + `
dialects = sqlite
migrations = migrations
queries_out = queries
name = testdb

[api]
package = ./api
`
	os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644)

	goMod := `module testproject

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + shipqModulePath + `
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	querydefDir := filepath.Join(tmpDir, "querydef")
	os.MkdirAll(querydefDir, 0755)
	os.WriteFile(filepath.Join(querydefDir, "queries.go"), []byte("package querydef\n"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Build shipq binary first
	shipqBinary := filepath.Join(tmpDir, "shipq-test")
	cmd := exec.Command("go", "build", "-o", shipqBinary, "./cmd/shipq")
	cmd.Dir = shipqModulePath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build shipq: %v\nOutput: %s", err, output)
	}

	// Run go mod tidy
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\nOutput: %s", err, output)
	}

	// Run migrate up
	cmd = exec.Command(shipqBinary, "db", "migrate", "up")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate up failed: %v\nOutput: %s", err, output)
	}

	// Run db compile to generate queries package
	cmd = exec.Command(shipqBinary, "db", "compile")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("db compile failed: %v\nOutput: %s", err, output)
	}

	// Run db setup to generate db/generated package
	cmd = exec.Command(shipqBinary, "db", "setup")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("db setup failed: %v\nOutput: %s", err, output)
	}

	// Generate resource
	var stdout, stderr bytes.Buffer
	opts := Options{Stdout: &stdout, Stderr: &stderr, Version: "test"}
	if exitCode := runResource([]string{"accounts"}, opts); exitCode != ExitSuccess {
		t.Fatalf("runResource failed: %s", stderr.String())
	}

	// Read generated handlers
	handlersContent, err := os.ReadFile(filepath.Join(tmpDir, "api", "resources", "accounts", "handlers.go"))
	if err != nil {
		t.Fatalf("failed to read handlers.go: %v", err)
	}

	contentStr := string(handlersContent)

	// Verify the generated code has DB integration via the global Querier pattern
	if !strings.Contains(contentStr, "generated.Querier") {
		t.Error("expected generated.Querier for DB integration")
	}

	// Verify it imports the queries package
	if !strings.Contains(contentStr, `"testproject/queries"`) {
		t.Errorf("expected import of testproject/queries, got:\n%s", contentStr)
	}

	// Verify it imports the db/generated package
	if !strings.Contains(contentStr, `"testproject/db/generated"`) {
		t.Errorf("expected import of testproject/db/generated, got:\n%s", contentStr)
	}

	// Verify Register function exists
	if !strings.Contains(contentStr, "func Register(app *portapi.App)") {
		t.Error("expected Register function")
	}

	// Verify it uses proper query param types (queries.GetAccountParams, etc.)
	expectedTypes := []string{
		"queries.GetAccountParams",
		"queries.InsertAccountParams",
		"queries.UpdateAccountParams",
		"queries.DeleteAccountParams",
	}
	for _, typ := range expectedTypes {
		if !strings.Contains(contentStr, typ) {
			t.Errorf("expected %s type reference in generated code", typ)
		}
	}

	// Verify handler functions exist
	expectedFuncs := []string{
		"func GetAccount(ctx context.Context",
		"func ListAccounts(ctx context.Context",
		"func CreateAccount(ctx context.Context",
		"func UpdateAccount(ctx context.Context",
		"func DeleteAccount(ctx context.Context",
	}
	for _, fn := range expectedFuncs {
		if !strings.Contains(contentStr, fn) {
			t.Errorf("expected %s handler function in generated code", fn)
		}
	}

	t.Logf("Generated handlers.go:\n%s", contentStr)
}

// TestE2E_FreshProjectResourceWorkflow tests the "fresh project" scenario:
// - Start with just shipq.ini and migrations, NO api/ directory
// - Run `shipq api resource users`
// - Verify that api/api.go, zz_generated_register.go, and zz_generated_http.go are all created
// - Verify the generated code compiles
func TestE2E_FreshProjectResourceWorkflow(t *testing.T) {
	// Get the shipq module path BEFORE changing directories
	shipqModulePath := findShipqModulePath(t)
	if shipqModulePath == "" {
		t.Skip("Could not find shipq module path")
	}

	tmpDir := t.TempDir()

	// === Step 1: Set up minimal project structure (NO api/ directory) ===
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a migration file
	migrationCode := `package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260203112437_accounts(plan *migrate.MigrationPlan) error {
	plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email").Unique()
		return nil
	})
	return nil
}
`
	migrationPath := filepath.Join(migrationsDir, "20260203112437_accounts.go")
	if err := os.WriteFile(migrationPath, []byte(migrationCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create shipq.ini - note: api/ directory does NOT exist yet
	dbPath := filepath.Join(tmpDir, "test.db")
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://` + dbPath + `
dialects = sqlite
migrations = migrations
name = testdb

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod for the temp project
	goMod := `module testfreshproject

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + shipqModulePath + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify api/ directory does NOT exist
	apiDir := filepath.Join(tmpDir, "api")
	if _, err := os.Stat(apiDir); !os.IsNotExist(err) {
		t.Fatalf("expected api/ directory to NOT exist before test, but it does")
	}

	// Change to project directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Build shipq binary first
	t.Log("Building shipq binary...")
	shipqBinary := filepath.Join(tmpDir, "shipq-test")
	cmd := exec.Command("go", "build", "-o", shipqBinary, "./cmd/shipq")
	cmd.Dir = shipqModulePath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build shipq: %v\nOutput: %s", err, output)
	}

	// === Step 2: Run go mod tidy, then shipq db migrate up ===
	t.Log("Running: go mod tidy")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\nOutput: %s", err, output)
	}

	t.Log("Running: shipq db migrate up")
	cmd = exec.Command(shipqBinary, "db", "migrate", "up")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shipq db migrate up failed: %v\nOutput: %s", err, output)
	}
	t.Logf("migrate up output: %s", output)

	// === Step 3: Run shipq api resource users (from fresh project - NO api/ dir) ===
	t.Log("Running: shipq api resource users (from fresh project with no api/ directory)")
	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"users"}, opts)
	if exitCode != ExitSuccess {
		t.Fatalf("runResource() exit code = %d, want %d\nstdout: %s\nstderr: %s",
			exitCode, ExitSuccess, stdout.String(), stderr.String())
	}
	t.Logf("resource output:\n%s", stdout.String())

	// === Step 4: Verify all expected files were created ===

	// 4a: api/api.go (bootstrap file)
	apiGoPath := filepath.Join(apiDir, "api.go")
	if _, err := os.Stat(apiGoPath); os.IsNotExist(err) {
		t.Fatalf("expected api/api.go to be auto-created at %s", apiGoPath)
	}
	apiGoContent, _ := os.ReadFile(apiGoPath)
	if !strings.Contains(string(apiGoContent), "package api") {
		t.Errorf("expected api/api.go to contain 'package api', got:\n%s", apiGoContent)
	}

	// 4b: api/resources/users/handlers.go
	handlersPath := filepath.Join(apiDir, "resources", "users", "handlers.go")
	if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
		t.Fatalf("expected handlers.go to be created at %s", handlersPath)
	}

	// 4c: api/zz_generated_register.go
	registerPath := filepath.Join(apiDir, "zz_generated_register.go")
	if _, err := os.Stat(registerPath); os.IsNotExist(err) {
		t.Fatalf("expected zz_generated_register.go to be created at %s", registerPath)
	}
	registerContent, _ := os.ReadFile(registerPath)
	if !strings.Contains(string(registerContent), "users.Register(app)") {
		t.Errorf("expected zz_generated_register.go to contain 'users.Register(app)', got:\n%s", registerContent)
	}

	// 4d: api/zz_generated_http.go (auto-generated by resource command)
	httpPath := filepath.Join(apiDir, "zz_generated_http.go")
	if _, err := os.Stat(httpPath); os.IsNotExist(err) {
		t.Fatalf("expected zz_generated_http.go to be auto-generated at %s", httpPath)
	}
	httpContent, _ := os.ReadFile(httpPath)
	if !strings.Contains(string(httpContent), "func NewMux()") {
		t.Errorf("expected zz_generated_http.go to contain 'func NewMux()', got:\n%s", httpContent)
	}

	// Verify output mentions all the generated files
	outputStr := stdout.String()
	expectedInOutput := []string{
		"api/api.go",
		"handlers.go",
		"zz_generated_register.go",
		"zz_generated_http.go",
	}
	for _, expected := range expectedInOutput {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("expected output to mention %q, got:\n%s", expected, outputStr)
		}
	}

	// === Step 5: Verify generated code compiles ===
	t.Log("Running: go mod tidy")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("go mod tidy output (may have warnings): %s", output)
	}

	t.Log("Running: go build ./...")
	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\nOutput: %s\n\n"+
			"=== api/api.go ===\n%s\n\n"+
			"=== handlers.go ===\n%s\n\n"+
			"=== zz_generated_register.go ===\n%s\n\n"+
			"=== zz_generated_http.go ===\n%s",
			err, output, apiGoContent, func() string { c, _ := os.ReadFile(handlersPath); return string(c) }(),
			registerContent, httpContent)
	}

	t.Log("SUCCESS: Fresh project resource workflow completed - all files generated and compile!")
}

// TestE2E_MultipleResourcesWorkflow tests generating multiple resources:
// - Generate users, then pets
// - Verify zz_generated_register.go contains both
// - Verify zz_generated_http.go includes routes for both
func TestE2E_MultipleResourcesWorkflow(t *testing.T) {
	// Get the shipq module path BEFORE changing directories
	shipqModulePath := findShipqModulePath(t)
	if shipqModulePath == "" {
		t.Skip("Could not find shipq module path")
	}

	tmpDir := t.TempDir()

	// === Step 1: Set up project structure ===
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a migration file with two tables
	migrationCode := `package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260203112437_tables(plan *migrate.MigrationPlan) error {
	plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email").Unique()
		return nil
	})
	plan.AddTable("pets", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("species")
		return nil
	})
	return nil
}
`
	migrationPath := filepath.Join(migrationsDir, "20260203112437_tables.go")
	if err := os.WriteFile(migrationPath, []byte(migrationCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create shipq.ini
	dbPath := filepath.Join(tmpDir, "test.db")
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://` + dbPath + `
dialects = sqlite
migrations = migrations
name = testdb

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod for the temp project
	goMod := `module testmultiresource

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + shipqModulePath + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to project directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Build shipq binary first
	t.Log("Building shipq binary...")
	shipqBinary := filepath.Join(tmpDir, "shipq-test")
	cmd := exec.Command("go", "build", "-o", shipqBinary, "./cmd/shipq")
	cmd.Dir = shipqModulePath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build shipq: %v\nOutput: %s", err, output)
	}

	// Run migrations
	t.Log("Running: go mod tidy && shipq db migrate up")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	cmd.CombinedOutput()

	cmd = exec.Command(shipqBinary, "db", "migrate", "up")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shipq db migrate up failed: %v\nOutput: %s", err, output)
	}

	// === Step 2: Generate first resource (users) ===
	t.Log("Running: shipq api resource users")
	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"users"}, opts)
	if exitCode != ExitSuccess {
		t.Fatalf("runResource(users) failed: %d\nstdout: %s\nstderr: %s",
			exitCode, stdout.String(), stderr.String())
	}

	// === Step 3: Generate second resource (pets) ===
	t.Log("Running: shipq api resource pets")
	stdout.Reset()
	stderr.Reset()

	exitCode = runResource([]string{"pets"}, opts)
	if exitCode != ExitSuccess {
		t.Fatalf("runResource(pets) failed: %d\nstdout: %s\nstderr: %s",
			exitCode, stdout.String(), stderr.String())
	}

	// === Step 4: Verify both resources are registered ===
	apiDir := filepath.Join(tmpDir, "api")

	// Check register file contains both
	registerPath := filepath.Join(apiDir, "zz_generated_register.go")
	registerContent, err := os.ReadFile(registerPath)
	if err != nil {
		t.Fatalf("failed to read register file: %v", err)
	}
	registerStr := string(registerContent)
	if !strings.Contains(registerStr, "users.Register(app)") {
		t.Errorf("expected register to contain users.Register(app)")
	}
	if !strings.Contains(registerStr, "pets.Register(app)") {
		t.Errorf("expected register to contain pets.Register(app)")
	}

	// Check HTTP runtime contains routes for both
	httpPath := filepath.Join(apiDir, "zz_generated_http.go")
	httpContent, err := os.ReadFile(httpPath)
	if err != nil {
		t.Fatalf("failed to read http file: %v", err)
	}
	httpStr := string(httpContent)
	// Should have routes like "GET /users/{public_id}" and "GET /pets/{public_id}"
	if !strings.Contains(httpStr, "/users") {
		t.Errorf("expected HTTP runtime to contain /users routes")
	}
	if !strings.Contains(httpStr, "/pets") {
		t.Errorf("expected HTTP runtime to contain /pets routes")
	}

	// === Step 5: Verify code compiles ===
	t.Log("Running: go mod tidy && go build ./...")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	cmd.CombinedOutput()

	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\nOutput: %s\n\n"+
			"=== zz_generated_register.go ===\n%s\n\n"+
			"=== zz_generated_http.go ===\n%s",
			err, output, registerContent, httpContent)
	}

	t.Log("SUCCESS: Multiple resources workflow completed!")
}
