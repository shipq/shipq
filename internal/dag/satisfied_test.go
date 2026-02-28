package shipqdag_test

import (
	"os"
	"path/filepath"
	"testing"

	shipqdag "github.com/shipq/shipq/internal/dag"
)

func TestInitSatisfied_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdInit) {
		t.Error("init should not be satisfied in empty dir")
	}
}

func TestInitSatisfied_WithIni(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdInit) {
		t.Error("init should be satisfied when shipq.ini exists")
	}
}

func TestDBSetupSatisfied_NoDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdDBSetup) {
		t.Error("db_setup should not be satisfied without database_url")
	}
}

func TestDBSetupSatisfied_WithDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\ndatabase_url = sqlite://test.db\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdDBSetup) {
		t.Error("db_setup should be satisfied with database_url")
	}
}

func TestDBSetupSatisfied_NoIniFile(t *testing.T) {
	dir := t.TempDir()
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdDBSetup) {
		t.Error("db_setup should not be satisfied without shipq.ini")
	}
}

func TestAuthSatisfied_NoSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdAuth) {
		t.Error("auth should not be satisfied without [auth] section")
	}
}

func TestAuthSatisfied_WithSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[auth]\nprotect_by_default = true\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdAuth) {
		t.Error("auth should be satisfied with [auth] section")
	}
}

func TestWorkersSatisfied_NoSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdWorkers) {
		t.Error("workers should not be satisfied without [workers] section")
	}
}

func TestWorkersSatisfied_WithSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[auth]\n[workers]\nredis_url = redis://localhost:6379\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdWorkers) {
		t.Error("workers should be satisfied with [workers] section")
	}
}

func TestEmailSatisfied_NoSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n[auth]\n[workers]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdEmail) {
		t.Error("email should not be satisfied without [email] section")
	}
}

func TestEmailSatisfied_WithSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[auth]\n[workers]\n[email]\nsmtp_host = localhost\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdEmail) {
		t.Error("email should be satisfied with [email] section")
	}
}

func TestFilesSatisfied_NoSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdFiles) {
		t.Error("files should not be satisfied without [files] section")
	}
}

func TestFilesSatisfied_WithSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[files]\nbucket = my-bucket\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdFiles) {
		t.Error("files should be satisfied with [files] section")
	}
}

func TestLLMSatisfied_NoSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n[workers]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdLLMCompile) {
		t.Error("llm should not be satisfied without [llm] section")
	}
}

func TestLLMSatisfied_WithSection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[workers]\n[llm]\ntool_pkgs = tools\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdLLMCompile) {
		t.Error("llm should be satisfied with [llm] section")
	}
}

func TestMigrateUpSatisfied_NoSchemaJson(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\ndatabase_url = sqlite://test.db\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdMigrateUp) {
		t.Error("migrate_up should not be satisfied without schema.json")
	}
}

func TestMigrateUpSatisfied_WithSchemaJson(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\ndatabase_url = sqlite://test.db\n"), 0644)
	schemaDir := filepath.Join(dir, "shipq", "db", "migrate")
	os.MkdirAll(schemaDir, 0755)
	os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte("{}"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdMigrateUp) {
		t.Error("migrate_up should be satisfied with schema.json")
	}
}

func TestDBCompileSatisfied_NoQueriesDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\ndatabase_url = sqlite://test.db\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdDBCompile) {
		t.Error("db_compile should not be satisfied without queries directory")
	}
}

func TestDBCompileSatisfied_EmptyQueriesDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "shipq", "queries"), 0755)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdDBCompile) {
		t.Error("db_compile should not be satisfied with empty queries directory")
	}
}

func TestDBCompileSatisfied_WithQueriesDir(t *testing.T) {
	dir := t.TempDir()
	queriesDir := filepath.Join(dir, "shipq", "queries")
	os.MkdirAll(queriesDir, 0755)
	os.WriteFile(filepath.Join(queriesDir, "types.go"), []byte("package queries\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdDBCompile) {
		t.Error("db_compile should be satisfied with non-empty queries directory")
	}
}

func TestSignupSatisfied_NoFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n[auth]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdSignup) {
		t.Error("signup should not be satisfied without signup.go")
	}
}

func TestSignupSatisfied_WithFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n[auth]\n"), 0644)
	signupDir := filepath.Join(dir, "api", "auth")
	os.MkdirAll(signupDir, 0755)
	os.WriteFile(filepath.Join(signupDir, "signup.go"), []byte("package auth\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdSignup) {
		t.Error("signup should be satisfied when signup.go exists")
	}
}

func TestOAuthGoogleSatisfied_NoKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n[auth]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdAuthGoogle) {
		t.Error("auth_google should not be satisfied without oauth_google key")
	}
}

func TestOAuthGoogleSatisfied_WithKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[auth]\noauth_google = true\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdAuthGoogle) {
		t.Error("auth_google should be satisfied with oauth_google = true")
	}
}

func TestOAuthGitHubSatisfied_NoKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[db]\n[auth]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CmdAuthGitHub) {
		t.Error("auth_github should not be satisfied without oauth_github key")
	}
}

func TestOAuthGitHubSatisfied_WithKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[auth]\noauth_github = true\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdAuthGitHub) {
		t.Error("auth_github should be satisfied with oauth_github = true")
	}
}

func TestUnknownCommand_NeverSatisfied(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[db]\n[auth]\n[workers]\n[email]\n[files]\n[llm]\n"), 0644)
	satisfied := shipqdag.SatisfiedFunc(dir)
	if satisfied(shipqdag.CommandID("nonexistent")) {
		t.Error("unknown command should never be satisfied")
	}
}

// TestSatisfiedFunc_Integration builds up a project state step-by-step and
// verifies that Available returns the correct commands at each stage.
func TestSatisfiedFunc_Integration(t *testing.T) {
	dir := t.TempDir()
	g := shipqdag.Graph()

	toSet := func(ids []shipqdag.CommandID) map[shipqdag.CommandID]bool {
		m := make(map[shipqdag.CommandID]bool)
		for _, id := range ids {
			m[id] = true
		}
		return m
	}

	// Step 0: empty dir — only standalone commands available
	satisfied := shipqdag.SatisfiedFunc(dir)
	avail := g.Available(satisfied)
	availSet := toSet(avail)
	if !availSet[shipqdag.CmdNix] {
		t.Error("nix should be available in empty dir")
	}
	if !availSet[shipqdag.CmdInit] {
		t.Error("init should be available in empty dir")
	}
	if availSet[shipqdag.CmdDBSetup] {
		t.Error("db_setup should NOT be available in empty dir")
	}
	if availSet[shipqdag.CmdAuth] {
		t.Error("auth should NOT be available in empty dir")
	}
	if availSet[shipqdag.CmdDocker] {
		t.Error("docker should NOT be available in empty dir (needs init)")
	}

	// Step 1: create shipq.ini → init satisfied
	os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[project]\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	avail = g.Available(satisfied)
	availSet = toSet(avail)
	if !availSet[shipqdag.CmdDBSetup] {
		t.Error("db_setup should be available after init")
	}
	if !availSet[shipqdag.CmdDocker] {
		t.Error("docker should be available after init")
	}
	if !availSet[shipqdag.CmdMigrateNew] {
		t.Error("migrate_new should be available after init")
	}
	if !availSet[shipqdag.CmdHandlerCompile] {
		t.Error("handler_compile should be available after init")
	}
	if availSet[shipqdag.CmdAuth] {
		t.Error("auth should NOT be available (needs db_setup)")
	}
	if availSet[shipqdag.CmdMigrateUp] {
		t.Error("migrate_up should NOT be available (needs db_setup)")
	}

	// Step 2: add [db] database_url → db_setup satisfied
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	avail = g.Available(satisfied)
	availSet = toSet(avail)
	if !availSet[shipqdag.CmdAuth] {
		t.Error("auth should be available after db_setup")
	}
	if !availSet[shipqdag.CmdWorkers] {
		t.Error("workers should be available after db_setup (auth is only soft dep)")
	}
	if !availSet[shipqdag.CmdMigrateUp] {
		t.Error("migrate_up should be available after db_setup")
	}
	if !availSet[shipqdag.CmdMigrateReset] {
		t.Error("migrate_reset should be available after db_setup")
	}
	if !availSet[shipqdag.CmdFiles] {
		t.Error("files should be available after db_setup")
	}
	if !availSet[shipqdag.CmdSeed] {
		t.Error("seed should be available after db_setup")
	}
	if availSet[shipqdag.CmdDBCompile] {
		t.Error("db_compile should NOT be available (needs migrate_up)")
	}
	if availSet[shipqdag.CmdEmail] {
		t.Error("email should NOT be available (needs auth + workers)")
	}

	// Step 3: add [auth] → auth satisfied
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	avail = g.Available(satisfied)
	availSet = toSet(avail)
	if !availSet[shipqdag.CmdSignup] {
		t.Error("signup should be available after auth")
	}
	if !availSet[shipqdag.CmdAuthGoogle] {
		t.Error("auth_google should be available after auth")
	}
	if !availSet[shipqdag.CmdAuthGitHub] {
		t.Error("auth_github should be available after auth")
	}
	if availSet[shipqdag.CmdEmail] {
		t.Error("email should NOT be available (still needs workers)")
	}

	// Step 4: add [workers] → workers satisfied
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n[workers]\nredis_url = redis://localhost\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	avail = g.Available(satisfied)
	availSet = toSet(avail)
	if !availSet[shipqdag.CmdEmail] {
		t.Error("email should be available after auth + workers")
	}
	if !availSet[shipqdag.CmdWorkersCompile] {
		t.Error("workers_compile should be available after workers")
	}
	if !availSet[shipqdag.CmdLLMCompile] {
		t.Error("llm_compile should be available after workers")
	}

	// Step 5: add schema.json → migrate_up satisfied
	schemaDir := filepath.Join(dir, "shipq", "db", "migrate")
	os.MkdirAll(schemaDir, 0755)
	os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte("{}"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	avail = g.Available(satisfied)
	availSet = toSet(avail)
	if !availSet[shipqdag.CmdDBCompile] {
		t.Error("db_compile should be available after migrate_up")
	}
	if !availSet[shipqdag.CmdResource] {
		t.Error("resource should be available after migrate_up")
	}
	if !availSet[shipqdag.CmdHandlerGen] {
		t.Error("handler_generate should be available after migrate_up")
	}

	// Step 6: add queries → db_compile satisfied
	queriesDir := filepath.Join(dir, "shipq", "queries")
	os.MkdirAll(queriesDir, 0755)
	os.WriteFile(filepath.Join(queriesDir, "types.go"), []byte("package queries\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdDBCompile) {
		t.Error("db_compile should be satisfied with queries")
	}

	// Step 7: add [email] → email satisfied
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n[workers]\nredis_url = redis://localhost\n[email]\nsmtp_host = localhost\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdEmail) {
		t.Error("email should be satisfied with [email] section")
	}

	// Step 8: add signup.go → signup satisfied
	signupDir := filepath.Join(dir, "api", "auth")
	os.MkdirAll(signupDir, 0755)
	os.WriteFile(filepath.Join(signupDir, "signup.go"), []byte("package auth\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdSignup) {
		t.Error("signup should be satisfied with signup.go")
	}

	// Step 9: add oauth → oauth satisfied
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\noauth_google = true\noauth_github = true\n[workers]\nredis_url = redis://localhost\n[email]\nsmtp_host = localhost\n[files]\nbucket = test\n[llm]\ntool_pkgs = tools\n"), 0644)
	satisfied = shipqdag.SatisfiedFunc(dir)
	if !satisfied(shipqdag.CmdAuthGoogle) {
		t.Error("auth_google should be satisfied")
	}
	if !satisfied(shipqdag.CmdAuthGitHub) {
		t.Error("auth_github should be satisfied")
	}
	if !satisfied(shipqdag.CmdFiles) {
		t.Error("files should be satisfied")
	}
	if !satisfied(shipqdag.CmdLLMCompile) {
		t.Error("llm should be satisfied")
	}
}

// TestRunOnlyCommands_NeverSatisfied verifies that "run-only" commands
// (those that don't produce persistent state) are never satisfied.
func TestRunOnlyCommands_NeverSatisfied(t *testing.T) {
	dir := t.TempDir()
	// Create a fully-loaded project
	os.WriteFile(filepath.Join(dir, "shipq.ini"),
		[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\noauth_google = true\noauth_github = true\n[workers]\n[email]\n[files]\n[llm]\n"), 0644)

	schemaDir := filepath.Join(dir, "shipq", "db", "migrate")
	os.MkdirAll(schemaDir, 0755)
	os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte("{}"), 0644)

	queriesDir := filepath.Join(dir, "shipq", "queries")
	os.MkdirAll(queriesDir, 0755)
	os.WriteFile(filepath.Join(queriesDir, "types.go"), []byte("package queries\n"), 0644)

	satisfied := shipqdag.SatisfiedFunc(dir)

	// These are run-only commands with no persistent state to check.
	runOnlyCmds := []shipqdag.CommandID{
		shipqdag.CmdMigrateNew,
		shipqdag.CmdMigrateReset,
		shipqdag.CmdWorkersCompile,
		shipqdag.CmdResource,
		shipqdag.CmdHandlerGen,
		shipqdag.CmdHandlerCompile,
		shipqdag.CmdSeed,
		shipqdag.CmdDocker,
		shipqdag.CmdNix,
	}

	for _, cmd := range runOnlyCmds {
		if satisfied(cmd) {
			t.Errorf("%q should never be satisfied (run-only command)", cmd)
		}
	}
}
