package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// shipqRepoRoot returns the absolute path to the shipq repository root.
// We are in internal/commands/e2e, so go up 3 levels.
func shipqRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	root, err := filepath.Abs(filepath.Join(wd, "..", "..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	// Sanity check: go.mod should exist
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s doesn't contain go.mod", root)
	}
	return root
}

// buildShipq compiles the shipq binary and returns its path.
func buildShipq(t *testing.T, repoRoot string) string {
	t.Helper()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "shipq")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/shipq")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build shipq binary: %v\n%s", err, out)
	}
	return bin
}

// cleanEnv returns os.Environ() with DATABASE_URL and TEST_DATABASE_URL removed
// so that the shipq CLI reads its config from shipq.ini rather than the shell env.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "DATABASE_URL=") || strings.HasPrefix(e, "TEST_DATABASE_URL=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// run executes a command in the given directory and fails the test on error.
// It strips DATABASE_URL from the environment so shipq uses its ini config.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = cleanEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// runWithEnv executes a command with extra environment variables.
// Starts from cleanEnv() so shell-level DATABASE_URL does not leak in.
func runWithEnv(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(cleanEnv(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// -------------------------------------------------------------------------
// Database configuration
// -------------------------------------------------------------------------

// dbConfig describes how to set up and connect to a database for E2E tests.
// Note: `shipq db setup` always names databases after the project directory,
// so BaseURL just needs to point at the right server; the db name in the URL
// is overwritten by setup.
type dbConfig struct {
	Name     string   // e.g., "sqlite", "postgres", "mysql"
	BaseURL  string   // DATABASE_URL for `shipq db setup`
	ExtraEnv []string // extra env vars for test runs (e.g., COOKIE_SECRET)
}

// testEnvForProject returns the env vars slice for running `go test` in a
// generated project. It reads the DATABASE_URL from shipq.ini (set by db setup)
// and derives the test URL from the project name.
//
// The generated config package now uses compile-time dev defaults when
// GO_ENV != "production", so we only need to set vars that the test must
// override (database URLs, GO_ENV, and an explicit COOKIE_SECRET for tests).
func testEnvForProject(t *testing.T, cleanDir string, db dbConfig) []string {
	t.Helper()

	projectName := filepath.Base(cleanDir)
	var testURL string

	switch db.Name {
	case "sqlite":
		testDBPath := filepath.Join(cleanDir, ".shipq", "data", projectName+"_test.db")
		testURL = "sqlite://" + testDBPath
	case "postgres":
		testURL = fmt.Sprintf("postgres://postgres@localhost:5432/%s_test?sslmode=disable", projectName)
	case "mysql":
		testURL = fmt.Sprintf("mysql://root@localhost:3306/%s_test", projectName)
	}

	env := []string{
		"DATABASE_URL=" + testURL,
		"TEST_DATABASE_URL=" + testURL,
		"CGO_ENABLED=1",
		// GO_ENV=test triggers dev-mode defaults (anything != "production").
		"GO_ENV=test",
		// Tests should set explicit cookie secret rather than relying on dev defaults.
		"COOKIE_SECRET=test-secret-for-e2e",
	}
	env = append(env, db.ExtraEnv...)
	return env
}

// isServerReachable checks if a TCP connection can be established.
func isServerReachable(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// allDBConfigs returns dbConfigs for all available databases.
// SQLite is always available. Postgres and MySQL are skipped with a message
// if their server is not reachable.
func allDBConfigs(t *testing.T) []dbConfig {
	t.Helper()
	var configs []dbConfig

	// SQLite: always available
	configs = append(configs, dbConfig{
		Name:    "sqlite",
		BaseURL: "", // will be set per-project in setupProject
	})

	// PostgreSQL: skip if server unreachable
	if isServerReachable("localhost", 5432) {
		configs = append(configs, dbConfig{
			Name:    "postgres",
			BaseURL: "postgres://postgres@localhost:5432/postgres",
		})
	} else {
		t.Log("PostgreSQL server not reachable on localhost:5432, skipping")
	}

	// MySQL: skip if server unreachable
	if isServerReachable("localhost", 3306) {
		configs = append(configs, dbConfig{
			Name:    "mysql",
			BaseURL: "mysql://root@localhost:3306/",
		})
	} else {
		t.Log("MySQL server not reachable on localhost:3306, skipping")
	}

	return configs
}

// -------------------------------------------------------------------------
// Scenario helpers
// -------------------------------------------------------------------------

// dropDatabases drops the main and test databases for postgres/mysql to ensure
// a clean slate. For SQLite this is a no-op (the file system dir is removed).
// projectName is the basename of the project directory, which shipq db setup
// uses as the database name.
func dropDatabases(t *testing.T, db dbConfig, projectName string) {
	t.Helper()

	switch db.Name {
	case "postgres":
		testDBName := projectName + "_test"
		t.Logf("Dropping postgres databases %q and %q...", projectName, testDBName)
		for _, name := range []string{projectName, testDBName} {
			// Must quote identifiers with hyphens
			cmd := exec.Command("psql", "-U", "postgres", "-h", "localhost", "-c",
				fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, name))
			cmd.Env = cleanEnv()
			out, _ := cmd.CombinedOutput()
			t.Logf("  psql drop %s: %s", name, strings.TrimSpace(string(out)))
		}

	case "mysql":
		testDBName := projectName + "_test"
		t.Logf("Dropping mysql databases %q and %q...", projectName, testDBName)
		for _, name := range []string{projectName, testDBName} {
			cmd := exec.Command("mysql", "-u", "root", "-h", "127.0.0.1", "-P", "3306", "-e",
				fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
			cmd.Env = cleanEnv()
			out, _ := cmd.CombinedOutput()
			t.Logf("  mysql drop %s: %s", name, strings.TrimSpace(string(out)))
		}
	}
}

// projectEnv holds the environment context for a set up E2E project.
type projectEnv struct {
	CleanDir    string // project directory
	DatabaseURL string // DATABASE_URL from shipq.ini (set by db setup)
}

// setupProject initializes a clean project directory and runs db setup.
// Returns a projectEnv with the directory and the DATABASE_URL that
// shipq db setup wrote into shipq.ini.
func setupProject(t *testing.T, shipq, baseDirName string, db dbConfig) projectEnv {
	t.Helper()
	cleanDir := "/tmp/" + baseDirName + "-" + db.Name
	projectName := filepath.Base(cleanDir)

	os.RemoveAll(cleanDir)
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Drop any leftover databases from previous runs
	dropDatabases(t, db, projectName)

	t.Log("Initializing project...")
	run(t, cleanDir, shipq, "init")

	// For SQLite, construct the URL from the project directory
	dbURL := db.BaseURL
	if db.Name == "sqlite" {
		dbPath := filepath.Join(cleanDir, ".shipq", "data", projectName+".db")
		dbURL = "sqlite://" + dbPath
	}

	t.Logf("Setting up database (%s)...", db.Name)
	runWithEnv(t, cleanDir,
		[]string{"DATABASE_URL=" + dbURL},
		shipq, "db", "setup",
	)

	// Read the DATABASE_URL that db setup wrote to shipq.ini
	iniBytes, err := os.ReadFile(filepath.Join(cleanDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to read shipq.ini: %v", err)
	}
	var finalURL string
	for _, line := range strings.Split(string(iniBytes), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "database_url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				finalURL = strings.TrimSpace(parts[1])
			}
		}
	}
	if finalURL == "" {
		t.Fatalf("could not find database_url in shipq.ini:\n%s", iniBytes)
	}

	return projectEnv{CleanDir: cleanDir, DatabaseURL: finalURL}
}

// -------------------------------------------------------------------------
// Scenario: Auth + public pets resource
// -------------------------------------------------------------------------

func scenarioAuthAndPublicPets(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-public", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Auth
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Auth tests
	t.Log("Running auth tests...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./api/auth/spec/...", "-v", "-count=1")

	// Pets resource (public)
	t.Log("Creating public pets resource...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "pets", "name:string", "species:string", "age:int")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "pets", "all", "--public")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// All tests
	t.Log("Running all tests...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")
	t.Log("All tests passed!")
}

// -------------------------------------------------------------------------
// Scenario: Auth-protected pets resource
// -------------------------------------------------------------------------

func scenarioAuthProtectedPets(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-protected", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Auth
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Verify config
	iniContent, err := os.ReadFile(filepath.Join(proj.CleanDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to read shipq.ini: %v", err)
	}
	if !strings.Contains(string(iniContent), "protect_by_default = true") {
		t.Fatalf("shipq.ini missing protect_by_default = true:\n%s", iniContent)
	}

	// Pets resource (auth-protected, no --public)
	t.Log("Creating auth-protected pets resource...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "pets", "name:string", "species:string", "age:int")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "pets", "all")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// All tests (includes 401 + authenticated CRUD)
	t.Log("Running all tests (401 + authenticated CRUD)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")
	t.Log("All tests passed!")
}

// -------------------------------------------------------------------------
// Scenario: Auth-protected nested resources (authors + books)
// -------------------------------------------------------------------------

func scenarioAuthProtectedNested(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-nested", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Auth
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Authors resource (parent)
	t.Log("Creating authors resource...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "authors", "name:string", "bio:text")

	// Books resource (child, references authors)
	t.Log("Creating books resource (references authors)...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "books", "title:string", "author_id:references:authors")

	// Generate handlers for both tables (auth-protected)
	t.Log("Generating auth-protected resources...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "authors", "all")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "books", "all")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// All tests
	t.Log("Running all tests (nested resources, auth-protected)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")
	t.Log("All tests passed!")
}

// -------------------------------------------------------------------------
// Scenario: Tenancy-scoped pets resource (organization_id)
// -------------------------------------------------------------------------

func scenarioTenancyScopedPets(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-tenancy", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Auth (creates organizations, accounts, sessions, etc.)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Add scope configuration to shipq.ini BEFORE creating the migration,
	// so the migration generator auto-injects organization_id as a references column.
	t.Log("Configuring scope = organization_id in shipq.ini...")
	iniPath := filepath.Join(proj.CleanDir, "shipq.ini")
	addScopeToIni(t, iniPath, "organization_id")

	// Pets migration — organization_id is auto-injected by the scope config
	t.Log("Creating pets migration (organization_id auto-injected by scope)...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "pets",
		"name:string", "species:string", "age:int")

	// Generate scoped, auth-protected pets resource
	t.Log("Generating scoped pets resource...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "pets", "all")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Verify that tenancy test was generated
	tenancyTestPath := filepath.Join(proj.CleanDir, "api", "pets", "spec", "zz_generated_tenancy_test.go")
	if _, err := os.Stat(tenancyTestPath); os.IsNotExist(err) {
		t.Fatalf("expected tenancy test at %s, but file does not exist", tenancyTestPath)
	}
	t.Log("Tenancy test file generated successfully")

	// All tests (includes CRUD + tenancy isolation)
	t.Log("Running all tests (CRUD + tenancy isolation)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")
	t.Log("All tests passed including tenancy isolation!")
}

// addScopeToIni appends a scope directive to the [db] section of shipq.ini.
// It inserts `scope = <column>` right after the [db] section header.
func addScopeToIni(t *testing.T, iniPath, scopeColumn string) {
	t.Helper()
	data, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", iniPath, err)
	}

	content := string(data)
	// Insert scope = <column> after the [db] section header
	dbHeader := "[db]"
	idx := strings.Index(content, dbHeader)
	if idx == -1 {
		t.Fatalf("shipq.ini missing [db] section:\n%s", content)
	}

	insertAt := idx + len(dbHeader)
	// Find the end of the [db] header line
	nlIdx := strings.Index(content[insertAt:], "\n")
	if nlIdx != -1 {
		insertAt += nlIdx + 1
	} else {
		content += "\n"
		insertAt = len(content)
	}

	newContent := content[:insertAt] + "scope = " + scopeColumn + "\n" + content[insertAt:]

	if err := os.WriteFile(iniPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", iniPath, err)
	}
}

// -------------------------------------------------------------------------
// Top-level tests: each scenario x each available database
// -------------------------------------------------------------------------

func TestEndToEnd_AuthAndPublicPets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioAuthAndPublicPets(t, shipq, db)
		})
	}
}

func TestEndToEnd_AuthProtectedPets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioAuthProtectedPets(t, shipq, db)
		})
	}
}

func TestEndToEnd_AuthProtectedNestedResources(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioAuthProtectedNested(t, shipq, db)
		})
	}
}

func TestEndToEnd_TenancyScopedPets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioTenancyScopedPets(t, shipq, db)
		})
	}
}

// -------------------------------------------------------------------------
// Scenario: RBAC Auth Global (no tenancy)
// -------------------------------------------------------------------------

func scenarioRBACAuthGlobal(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-rbac-global", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Auth + signup (generates roles, account_roles, role_actions migrations)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	t.Log("Generating signup...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "signup")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Verify RBAC migration files exist
	migrationsDir := filepath.Join(proj.CleanDir, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	migrationNames := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "_roles.go") {
			migrationNames["roles"] = true
		}
		if strings.HasSuffix(name, "_account_roles.go") {
			migrationNames["account_roles"] = true
		}
		if strings.HasSuffix(name, "_role_actions.go") {
			migrationNames["role_actions"] = true
		}
	}
	for _, expected := range []string{"roles", "account_roles", "role_actions"} {
		if !migrationNames[expected] {
			t.Fatalf("missing RBAC migration: %s", expected)
		}
	}
	t.Log("RBAC migration files verified")

	// Verify the roles migration does NOT have organization_id (no tenancy)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_roles.go") {
			content, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
			if err != nil {
				t.Fatalf("failed to read roles migration: %v", err)
			}
			if strings.Contains(string(content), "organization_id") {
				t.Error("roles migration should NOT have organization_id in global (unscoped) mode")
			}
			break
		}
	}

	// Verify seed file exists
	seedPath := filepath.Join(proj.CleanDir, "seeds", "dev_auth_seed.go")
	if _, err := os.Stat(seedPath); os.IsNotExist(err) {
		t.Fatalf("expected dev auth seed file at %s", seedPath)
	}
	t.Log("Dev auth seed file verified")

	// Verify RBAC test file was generated
	rbacTestPath := filepath.Join(proj.CleanDir, "api", "zz_generated_rbac_test.go")
	if _, err := os.Stat(rbacTestPath); os.IsNotExist(err) {
		t.Fatalf("expected RBAC test at %s, but file does not exist", rbacTestPath)
	}

	// Verify it has 5 test functions (unscoped)
	rbacTestContent, err := os.ReadFile(rbacTestPath)
	if err != nil {
		t.Fatalf("failed to read RBAC test file: %v", err)
	}
	rbacStr := string(rbacTestContent)
	testCount := strings.Count(rbacStr, "func TestRBAC_")
	if testCount != 5 {
		t.Errorf("expected 5 RBAC test functions, got %d", testCount)
	}
	if strings.Contains(rbacStr, "TestRBAC_OrgScopedRolesDoNotCrossOrgs") {
		t.Error("global RBAC should NOT have org-isolation test")
	}
	t.Logf("RBAC test file verified with %d tests", testCount)

	// Run all tests
	t.Log("Running all tests (auth + RBAC global)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")
	t.Log("All tests passed!")
}

// -------------------------------------------------------------------------
// Scenario: RBAC Auth Scoped (with tenancy)
// -------------------------------------------------------------------------

func scenarioRBACAuthScoped(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-rbac-scoped", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Configure scope BEFORE running auth, so the migration generators
	// produce the scoped roles migration with organization_id.
	t.Log("Configuring scope = organization_id in shipq.ini...")
	iniPath := filepath.Join(proj.CleanDir, "shipq.ini")
	addScopeToIni(t, iniPath, "organization_id")

	// Auth + signup
	t.Log("Generating auth (scoped)...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	t.Log("Generating signup...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "signup")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Verify the roles migration HAS nullable organization_id
	migrationsDir := filepath.Join(proj.CleanDir, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_roles.go") {
			content, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
			if err != nil {
				t.Fatalf("failed to read roles migration: %v", err)
			}
			contentStr := string(content)
			if !strings.Contains(contentStr, "organization_id") {
				t.Error("scoped roles migration should have organization_id column")
			}
			if !strings.Contains(contentStr, "Nullable()") {
				t.Error("scoped roles migration organization_id should be nullable")
			}
			break
		}
	}
	t.Log("Scoped roles migration verified")

	// Verify RBAC test file has 6 tests (scoped includes org-isolation test)
	rbacTestPath := filepath.Join(proj.CleanDir, "api", "zz_generated_rbac_test.go")
	if _, err := os.Stat(rbacTestPath); os.IsNotExist(err) {
		t.Fatalf("expected RBAC test at %s, but file does not exist", rbacTestPath)
	}

	rbacTestContent, err := os.ReadFile(rbacTestPath)
	if err != nil {
		t.Fatalf("failed to read RBAC test file: %v", err)
	}
	rbacStr := string(rbacTestContent)
	testCount := strings.Count(rbacStr, "func TestRBAC_")
	if testCount != 6 {
		t.Errorf("expected 6 RBAC test functions, got %d", testCount)
	}
	if !strings.Contains(rbacStr, "TestRBAC_OrgScopedRolesDoNotCrossOrgs") {
		t.Error("scoped RBAC should have org-isolation test")
	}
	t.Logf("RBAC test file verified with %d tests", testCount)

	// Run all tests
	t.Log("Running all tests (auth + RBAC scoped)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")
	t.Log("All tests passed!")
}

func TestEndToEnd_RBACAuthGlobal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioRBACAuthGlobal(t, shipq, db)
		})
	}
}

func TestEndToEnd_RBACAuthScoped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioRBACAuthScoped(t, shipq, db)
		})
	}
}

// -------------------------------------------------------------------------
// Scenario: Auth + Files (S3 file upload system)
// -------------------------------------------------------------------------

func scenarioAuthAndFiles(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-files", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Auth (required before files -- managed_files references accounts)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Files (needs S3 env vars because handler compilation runs the generated
	// binary, which calls config.init() and panics without them)
	t.Log("Generating files system...")
	runWithEnv(t, proj.CleanDir, tEnv, shipq, "files")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Verify shipq.ini has [files] section
	iniContent, err := os.ReadFile(filepath.Join(proj.CleanDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to read shipq.ini: %v", err)
	}
	iniStr := string(iniContent)
	if !strings.Contains(iniStr, "[files]") {
		t.Fatalf("shipq.ini missing [files] section:\n%s", iniStr)
	}
	if !strings.Contains(iniStr, "s3_bucket") {
		t.Fatalf("shipq.ini missing s3_bucket key:\n%s", iniStr)
	}
	t.Log("[files] section verified in shipq.ini")

	// Verify migrations were generated
	migrationsDir := filepath.Join(proj.CleanDir, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	migrationNames := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "_managed_files.go") {
			migrationNames["managed_files"] = true
		}
		if strings.HasSuffix(name, "_file_access.go") {
			migrationNames["file_access"] = true
		}
	}
	for _, expected := range []string{"managed_files", "file_access"} {
		if !migrationNames[expected] {
			t.Fatalf("missing files migration: %s", expected)
		}
	}
	t.Log("Files migration files verified")

	// Verify handler files were generated
	handlersDir := filepath.Join(proj.CleanDir, "api", "managed_files")
	expectedHandlers := []string{
		"upload_url.go",
		"complete.go",
		"download.go",
		"list.go",
		"soft_delete.go",
		"visibility.go",
		"access.go",
		"helpers.go",
		"register.go",
	}
	for _, name := range expectedHandlers {
		path := filepath.Join(handlersDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected handler file %s does not exist", path)
		}
	}
	t.Log("Handler files verified")

	// Verify .shipq-no-regen marker exists
	markerPath := filepath.Join(handlersDir, ".shipq-no-regen")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Fatalf("expected .shipq-no-regen marker at %s", markerPath)
	}

	// Verify query definitions were generated
	queryDefsPath := filepath.Join(proj.CleanDir, "querydefs", "managed_files", "queries.go")
	if _, err := os.Stat(queryDefsPath); os.IsNotExist(err) {
		t.Fatalf("expected query defs at %s", queryDefsPath)
	}
	t.Log("Query definitions verified")

	// Verify filestorage library was embedded
	filestorageDir := filepath.Join(proj.CleanDir, "shipq", "lib", "filestorage")
	s3GoPath := filepath.Join(filestorageDir, "s3.go")
	if _, err := os.Stat(s3GoPath); os.IsNotExist(err) {
		t.Fatalf("expected embedded filestorage/s3.go at %s", s3GoPath)
	}
	t.Log("Embedded filestorage library verified")

	// Verify TypeScript helpers were generated
	tsPath := filepath.Join(proj.CleanDir, "shipq-files.ts")
	if _, err := os.Stat(tsPath); os.IsNotExist(err) {
		t.Fatalf("expected TypeScript helpers at %s", tsPath)
	}
	tsContent, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatalf("failed to read shipq-files.ts: %v", err)
	}
	tsStr := string(tsContent)
	if !strings.Contains(tsStr, "export async function uploadFile") {
		t.Error("shipq-files.ts missing uploadFile function")
	}
	if !strings.Contains(tsStr, "export function getDownloadUrl") {
		t.Error("shipq-files.ts missing getDownloadUrl function")
	}
	if !strings.Contains(tsStr, "export function configure") {
		t.Error("shipq-files.ts missing configure function")
	}
	t.Log("TypeScript helpers verified")

	// Verify config has S3 fields
	configPath := filepath.Join(proj.CleanDir, "config", "config.go")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config.go: %v", err)
	}
	configStr := string(configContent)
	for _, field := range []string{"S3_BUCKET", "S3_REGION", "S3_ENDPOINT", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		if !strings.Contains(configStr, field) {
			t.Errorf("config.go missing field %s", field)
		}
	}
	t.Log("Config S3 fields verified")

	// Build the generated project to verify everything compiles
	t.Log("Building generated project...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "build", "./...")
	t.Log("Project compiles successfully!")
}

func TestEndToEnd_AuthAndFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioAuthAndFiles(t, shipq, db)
		})
	}
}

// -------------------------------------------------------------------------
// Scenario: Workers basic pipeline
// -------------------------------------------------------------------------

func scenarioWorkersBasic(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-workers", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Step 1: Auth (prerequisite for workers)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Step 2: Workers
	t.Log("Running shipq workers...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "workers")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Step 3: Verify generated files exist
	t.Log("Verifying generated files...")
	expectedFiles := []string{
		"channels/example/register.go",
		"channels/example/zz_generated_channel.go",
		"cmd/worker/main.go",
		"centrifugo.json",
		"channels/spec/zz_generated_integration_test.go",
		"channels/spec/zz_generated_e2e_test.go",
		"config/config.go",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(proj.CleanDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify shipq.ini has [workers] section
	iniBytes, err := os.ReadFile(filepath.Join(proj.CleanDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to read shipq.ini: %v", err)
	}
	if !strings.Contains(string(iniBytes), "[workers]") {
		t.Error("expected [workers] section in shipq.ini")
	}

	// Step 4: Verify the generated project compiles
	t.Log("Compiling cmd/server...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "build", "./cmd/server")

	t.Log("Compiling cmd/worker...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "build", "./cmd/worker")

	// Step 5: Run generated tests (integration tests with mocked infra, no e2e tag)
	t.Log("Running go test ./... (integration tests with mocked infra)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")

	t.Log("Workers scenario passed!")
}

func TestEndToEnd_Workers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioWorkersBasic(t, shipq, db)
		})
	}
}

// -------------------------------------------------------------------------
// Scenario: Auth with OAuth (Google + GitHub)
// -------------------------------------------------------------------------

func scenarioAuthWithOAuth(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-oauth", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// 1. shipq auth (base email/password system)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 2. shipq auth google
	t.Log("Adding Google OAuth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "google")

	// 3. shipq auth github
	t.Log("Adding GitHub OAuth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "github")

	// 4. go mod tidy
	t.Log("Running go mod tidy...")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 5. Verify the project compiles
	t.Log("Verifying project compiles...")
	run(t, proj.CleanDir, "go", "build", "./...")

	// 6. Verify go vet passes
	t.Log("Running go vet...")
	run(t, proj.CleanDir, "go", "vet", "./...")

	// 7. Verify shipq.ini flags are set
	t.Log("Verifying shipq.ini OAuth flags...")
	iniContent, err := os.ReadFile(filepath.Join(proj.CleanDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to read shipq.ini: %v", err)
	}
	iniStr := string(iniContent)
	if !strings.Contains(iniStr, "oauth_google = true") {
		t.Fatalf("shipq.ini missing oauth_google = true:\n%s", iniStr)
	}
	if !strings.Contains(iniStr, "oauth_github = true") {
		t.Fatalf("shipq.ini missing oauth_github = true:\n%s", iniStr)
	}

	// 8. Verify generated files exist
	t.Log("Verifying generated OAuth files exist...")
	for _, f := range []string{
		filepath.Join("api", "auth", "oauth_shared.go"),
		filepath.Join("api", "auth", "oauth_google.go"),
		filepath.Join("api", "auth", "oauth_github.go"),
	} {
		fp := filepath.Join(proj.CleanDir, f)
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			t.Fatalf("expected generated file %s, but it does not exist", f)
		}
	}

	// 9. Verify oauth_accounts migration exists
	t.Log("Verifying OAuth migration files exist...")
	migrationsDir := filepath.Join(proj.CleanDir, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations dir: %v", err)
	}
	foundOAuthAccounts := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_oauth_accounts.go") {
			foundOAuthAccounts = true
		}
	}
	if !foundOAuthAccounts {
		t.Fatal("missing _oauth_accounts.go migration file")
	}

	// 10. Run all tests in the generated project
	t.Log("Running all tests in generated project...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")

	// 11. Verify idempotency: run both commands again
	t.Log("Verifying idempotency (running auth google + auth github again)...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "google")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "github")

	// Count oauth_accounts migration files — should still be exactly 1
	entries2, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations dir after idempotency check: %v", err)
	}
	oauthMigrationCount := 0
	for _, entry := range entries2 {
		if strings.HasSuffix(entry.Name(), "_oauth_accounts.go") {
			oauthMigrationCount++
		}
	}
	if oauthMigrationCount != 1 {
		t.Fatalf("expected exactly 1 _oauth_accounts.go migration, got %d", oauthMigrationCount)
	}

	// Project still compiles after idempotent re-run
	t.Log("Verifying project still compiles after idempotent re-run...")
	run(t, proj.CleanDir, "go", "build", "./...")

	t.Log("Auth with OAuth scenario passed!")
}

func TestEndToEnd_AuthWithOAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioAuthWithOAuth(t, shipq, db)
		})
	}
}

// -------------------------------------------------------------------------
// Scenario: Signup then OAuth (regression test)
//
// Running `shipq signup` followed by `shipq auth google` used to produce
// duplicate Register / RegisterOAuthRoutes declarations because the OAuth
// command wrote register.go (without signup) AND signup_register.go (with
// signup), both declaring the same symbols.
// -------------------------------------------------------------------------

func scenarioSignupThenOAuth(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-signup-oauth", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// 1. shipq auth (base email/password system)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 2. shipq signup (adds /signup route — writes register.go with signup)
	t.Log("Generating signup handler...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "signup")

	// 3. Verify the project compiles after signup
	t.Log("Verifying project compiles after signup...")
	run(t, proj.CleanDir, "go", "build", "./...")

	// 4. shipq auth google (adds OAuth — must not conflict with signup's register.go)
	t.Log("Adding Google OAuth after signup...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "google")

	// 5. go mod tidy
	t.Log("Running go mod tidy...")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 6. Verify the project compiles (this was the failing step before the fix)
	t.Log("Verifying project compiles after signup + auth google...")
	run(t, proj.CleanDir, "go", "build", "./...")

	// 7. Verify go vet passes
	t.Log("Running go vet...")
	run(t, proj.CleanDir, "go", "vet", "./...")

	// 8. Verify register.go contains the /signup route (not clobbered)
	t.Log("Verifying register.go still contains /signup route...")
	registerContent, err := os.ReadFile(filepath.Join(proj.CleanDir, "api", "auth", "register.go"))
	if err != nil {
		t.Fatalf("failed to read register.go: %v", err)
	}
	if !strings.Contains(string(registerContent), "/signup") {
		t.Fatalf("register.go missing /signup route after auth google:\n%s", registerContent)
	}

	// 9. Verify OAuth files exist
	t.Log("Verifying generated OAuth files exist...")
	for _, f := range []string{
		filepath.Join("api", "auth", "oauth_shared.go"),
		filepath.Join("api", "auth", "oauth_google.go"),
		filepath.Join("api", "auth", "signup.go"),
	} {
		fp := filepath.Join(proj.CleanDir, f)
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			t.Fatalf("expected generated file %s, but it does not exist", f)
		}
	}

	// 10. Run all tests in the generated project
	t.Log("Running all tests in generated project...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")

	// 11. Verify idempotency: run signup + auth google again
	t.Log("Verifying idempotency (running signup + auth google again)...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "signup")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "google")
	run(t, proj.CleanDir, "go", "build", "./...")

	t.Log("Signup then OAuth scenario passed!")
}

func TestEndToEnd_SignupThenOAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioSignupThenOAuth(t, shipq, db)
		})
	}
}

// scenarioAuthGoogleThenSignup verifies that running "shipq auth google" followed
// by "shipq signup" does NOT strip RegisterOAuthRoutes from register.go.
// This is the exact sequence that triggered Bug 1 before the fix.
func scenarioAuthGoogleThenSignup(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-auth-google-then-signup", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// 1. shipq auth (base email/password system)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 2. shipq auth google (adds OAuth — generates register.go WITH RegisterOAuthRoutes)
	t.Log("Adding Google OAuth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth", "google")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 3. Verify register.go contains RegisterOAuthRoutes after auth google
	t.Log("Verifying register.go contains RegisterOAuthRoutes after auth google...")
	registerContent, err := os.ReadFile(filepath.Join(proj.CleanDir, "api", "auth", "register.go"))
	if err != nil {
		t.Fatalf("failed to read register.go: %v", err)
	}
	if !strings.Contains(string(registerContent), "func RegisterOAuthRoutes(") {
		t.Fatalf("register.go missing RegisterOAuthRoutes after auth google:\n%s", registerContent)
	}

	// 4. shipq signup (must NOT strip RegisterOAuthRoutes from register.go)
	t.Log("Generating signup handler...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "signup")

	// 5. Verify register.go STILL contains RegisterOAuthRoutes after signup
	t.Log("Verifying register.go still contains RegisterOAuthRoutes after signup...")
	registerContent, err = os.ReadFile(filepath.Join(proj.CleanDir, "api", "auth", "register.go"))
	if err != nil {
		t.Fatalf("failed to read register.go after signup: %v", err)
	}
	if !strings.Contains(string(registerContent), "func RegisterOAuthRoutes(") {
		t.Fatalf("register.go missing RegisterOAuthRoutes after signup — signup clobbered OAuth routes:\n%s", registerContent)
	}

	// 6. Verify the project compiles (this was the failing step before the fix)
	t.Log("Verifying project compiles after auth google + signup...")
	run(t, proj.CleanDir, "go", "build", "./...")

	// 7. Verify go vet passes
	t.Log("Running go vet...")
	run(t, proj.CleanDir, "go", "vet", "./...")

	// 8. Run all tests in the generated project
	t.Log("Running all tests in generated project...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")

	t.Log("Auth Google then Signup scenario passed!")
}

func TestEndToEnd_AuthGoogleThenSignup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	for _, db := range allDBConfigs(t) {
		t.Run(db.Name, func(t *testing.T) {
			scenarioAuthGoogleThenSignup(t, shipq, db)
		})
	}
}

// -------------------------------------------------------------------------
// Scenario: FK Resolution — OpenAPI schema assertions
// -------------------------------------------------------------------------

func scenarioFKResolutionOpenAPI(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-fk-openapi", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}

	// 1. Create a parent table with no FK columns
	t.Log("Creating categories migration...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "categories", "name:string")

	// 2. Create a child table that references the parent
	t.Log("Creating posts migration (references categories)...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "migrate", "new", "posts",
		"title:string", "body:text", "category_id:references:categories")

	// 3. Generate public resources (no auth)
	t.Log("Generating categories resource...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "categories", "all", "--public")

	t.Log("Generating posts resource...")
	runWithEnv(t, proj.CleanDir, dbEnv,
		shipq, "resource", "posts", "all", "--public")

	run(t, proj.CleanDir, "go", "mod", "tidy")

	// 4. Extract the OpenAPI spec from the generated server file.
	//    The registry embeds it as: var openAPISpec = `{...}`
	serverFile := filepath.Join(proj.CleanDir, "api", "zz_generated_http.go")
	serverCode, err := os.ReadFile(serverFile)
	if err != nil {
		t.Fatalf("failed to read generated server file: %v", err)
	}

	specJSON := extractOpenAPISpec(t, string(serverCode))

	// 5. Parse and assert
	var spec map[string]any
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		t.Fatalf("OpenAPI spec is not valid JSON: %v", err)
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("OpenAPI spec missing 'paths'")
	}

	// --- Assert: POST /posts request body accepts category_id as string ---
	assertRequestBodyFieldType(t, paths, "/posts", "post",
		"category_id", "string",
		"POST /posts request body category_id should be string (public ID)")

	// --- Assert: GET /posts list response has category_id as string ---
	assertResponseFieldType(t, paths, "/posts", "get",
		"category_id", "string",
		"GET /posts response category_id should be string (public ID)")

	// --- Assert: GET /posts/{id} response has category_id as string ---
	assertResponseFieldType(t, paths, "/posts/{id}", "get",
		"category_id", "string",
		"GET /posts/{id} response category_id should be string (public ID)")

	// --- Assert: no response field anywhere has format: int64 ---
	//     (which would indicate an internal autoincrement ID leak)
	assertNoInt64InResponses(t, paths)

	t.Log("OpenAPI schema assertions passed!")
}

// extractOpenAPISpec extracts the JSON from the `var openAPISpec = `...“
// raw string literal in the generated server Go file.
func extractOpenAPISpec(t *testing.T, serverCode string) string {
	t.Helper()
	const marker = "var openAPISpec = `"
	start := strings.Index(serverCode, marker)
	if start == -1 {
		t.Fatal("could not find openAPISpec in generated server file")
	}
	start += len(marker)
	end := strings.Index(serverCode[start:], "`")
	if end == -1 {
		t.Fatal("could not find closing backtick for openAPISpec")
	}
	return serverCode[start : start+end]
}

// digSchema extracts the JSON schema from an OpenAPI response object,
// navigating through content -> application/json -> schema.
func digSchema(resp map[string]any) map[string]any {
	content, ok := resp["content"].(map[string]any)
	if !ok {
		return nil
	}
	appJSON, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil
	}
	schema, ok := appJSON["schema"].(map[string]any)
	if !ok {
		return nil
	}
	return schema
}

// findFieldAnywhere looks for a named property in a schema's top-level
// properties, and also inside any "items" array property's sub-properties
// (to handle list responses like { items: [{ category_id: ... }] }).
func findFieldAnywhere(schema map[string]any, fieldName string) map[string]any {
	// Direct property
	if props, ok := schema["properties"].(map[string]any); ok {
		if field, ok := props[fieldName].(map[string]any); ok {
			return field
		}
		// Check inside "items" property (list responses)
		if itemsProp, ok := props["items"].(map[string]any); ok {
			if itemsSchema, ok := itemsProp["items"].(map[string]any); ok {
				if innerProps, ok := itemsSchema["properties"].(map[string]any); ok {
					if field, ok := innerProps[fieldName].(map[string]any); ok {
						return field
					}
				}
			}
		}
	}
	// Array schema (items at top level)
	if items, ok := schema["items"].(map[string]any); ok {
		return findFieldAnywhere(items, fieldName)
	}
	return nil
}

// assertRequestBodyFieldType navigates the OpenAPI paths tree to check a
// request body schema property type.
func assertRequestBodyFieldType(t *testing.T, paths map[string]any,
	pathKey, method, fieldName, expectedType, msg string) {
	t.Helper()

	pathItem, ok := paths[pathKey].(map[string]any)
	if !ok {
		t.Fatalf("path %q not found in OpenAPI spec", pathKey)
	}
	op, ok := pathItem[method].(map[string]any)
	if !ok {
		t.Fatalf("method %q not found for path %q", method, pathKey)
	}
	reqBody, ok := op["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("no requestBody for %s %s", method, pathKey)
	}

	schema := digSchema(reqBody)
	if schema == nil {
		t.Fatalf("no schema found in requestBody for %s %s", method, pathKey)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("no properties in requestBody schema for %s %s", method, pathKey)
	}
	field, ok := props[fieldName].(map[string]any)
	if !ok {
		t.Fatalf("field %q not found in requestBody schema for %s %s",
			fieldName, method, pathKey)
	}

	actualType, _ := field["type"].(string)
	if actualType != expectedType {
		t.Errorf("%s: expected type %q, got %q (full schema: %v)",
			msg, expectedType, actualType, field)
	}
}

// assertResponseFieldType navigates the OpenAPI paths tree to check a
// response schema property type.
func assertResponseFieldType(t *testing.T, paths map[string]any,
	pathKey, method, fieldName, expectedType, msg string) {
	t.Helper()

	pathItem, ok := paths[pathKey].(map[string]any)
	if !ok {
		t.Fatalf("path %q not found in OpenAPI spec", pathKey)
	}
	op, ok := pathItem[method].(map[string]any)
	if !ok {
		t.Fatalf("method %q not found for path %q", method, pathKey)
	}
	responses, ok := op["responses"].(map[string]any)
	if !ok {
		t.Fatalf("no responses for %s %s", method, pathKey)
	}

	// Check 200 or 201
	var respObj map[string]any
	for _, code := range []string{"200", "201"} {
		if r, ok := responses[code].(map[string]any); ok {
			respObj = r
			break
		}
	}
	if respObj == nil {
		t.Fatalf("no 200/201 response for %s %s", method, pathKey)
	}

	schema := digSchema(respObj)
	if schema == nil {
		t.Fatalf("no schema found in response for %s %s", method, pathKey)
	}

	// The field may be nested inside "items" (list responses wrap in an object
	// with an "items" array property)
	props := findFieldAnywhere(schema, fieldName)
	if props == nil {
		t.Fatalf("field %q not found in response schema for %s %s", fieldName, method, pathKey)
	}

	actualType, _ := props["type"].(string)
	if actualType != expectedType {
		t.Errorf("%s: expected type %q, got %q (full schema: %v)",
			msg, expectedType, actualType, props)
	}
}

// assertNoInt64InResponses recursively walks every response schema in every
// path and fails if any property has "format": "int64".
func assertNoInt64InResponses(t *testing.T, paths map[string]any) {
	t.Helper()
	for pathKey, pathItem := range paths {
		methods, ok := pathItem.(map[string]any)
		if !ok {
			continue
		}
		for method, opRaw := range methods {
			op, ok := opRaw.(map[string]any)
			if !ok {
				continue
			}
			responses, ok := op["responses"].(map[string]any)
			if !ok {
				continue
			}
			for code, respRaw := range responses {
				resp, ok := respRaw.(map[string]any)
				if !ok {
					continue
				}
				schema := digSchema(resp)
				if schema == nil {
					continue
				}
				walkSchemaForInt64(t, schema,
					fmt.Sprintf("%s %s (response %s)", method, pathKey, code))
			}
		}
	}
}

// walkSchemaForInt64 recursively checks that no property in a schema
// has format: int64, which would indicate a leaked internal ID.
func walkSchemaForInt64(t *testing.T, schema map[string]any, context string) {
	t.Helper()
	if f, ok := schema["format"].(string); ok && f == "int64" {
		t.Errorf("%s: found format int64 — internal ID leak: %v", context, schema)
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for name, propRaw := range props {
			if prop, ok := propRaw.(map[string]any); ok {
				walkSchemaForInt64(t, prop, context+"."+name)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		walkSchemaForInt64(t, items, context+"[]")
	}
}

func TestEndToEnd_FKResolutionOpenAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	db := dbConfig{Name: "sqlite"}
	t.Run("sqlite", func(t *testing.T) {
		scenarioFKResolutionOpenAPI(t, shipq, db)
	})
}
