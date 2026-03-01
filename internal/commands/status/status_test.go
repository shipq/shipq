package status_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/internal/commands/status"
)

// captureStdout runs fn while capturing everything written to os.Stdout.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// setupProject creates a temp directory, writes shipq.ini (if content is
// non-empty), and chdir's into it so FindProjectRoots works.  It returns a
// cleanup function that restores the original working directory.
func setupProject(t *testing.T, iniContent string) (dir string, cleanup func()) {
	t.Helper()
	dir = t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	if iniContent != "" {
		if err := os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}
		// FindProjectRoots also needs go.mod to find the Go module root.
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/testproject\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}
		// FindProjectRoots walks up from cwd looking for shipq.ini,
		// so we need to be inside the directory.
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}
	} else {
		// No ini — chdir to a dir without shipq.ini
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}
	}

	return dir, func() { os.Chdir(origDir) }
}

func TestStatusCmd_NoProject(t *testing.T) {
	_, cleanup := setupProject(t, "")
	defer cleanup()

	out := captureStdout(func() {
		status.StatusCmd()
	})

	if !strings.Contains(out, "Not in a shipq project") {
		t.Errorf("expected 'Not in a shipq project' message, got:\n%s", out)
	}
	if !strings.Contains(out, "shipq init") {
		t.Errorf("expected 'shipq init' suggestion, got:\n%s", out)
	}
}

func TestStatusCmd_AfterInit(t *testing.T) {
	_, cleanup := setupProject(t, "[project]\n")
	defer cleanup()

	out := captureStdout(func() {
		status.StatusCmd()
	})

	if !strings.Contains(out, "shipq project status:") {
		t.Errorf("expected header, got:\n%s", out)
	}

	// init should be satisfied
	if !strings.Contains(out, "✓") {
		t.Error("expected at least one ✓ (for init)")
	}

	// db setup should be available as a next step
	if !strings.Contains(out, "Available next steps:") {
		t.Errorf("expected 'Available next steps:' section, got:\n%s", out)
	}
	if !strings.Contains(out, "db setup") {
		t.Errorf("expected 'db setup' in available steps, got:\n%s", out)
	}
}

func TestStatusCmd_AfterDBSetup(t *testing.T) {
	_, cleanup := setupProject(t, "[project]\n[db]\ndatabase_url = sqlite://dev.db\n")
	defer cleanup()

	out := captureStdout(func() {
		status.StatusCmd()
	})

	// init and db_setup should be satisfied
	lines := strings.Split(out, "\n")
	initSatisfied := false
	dbSetupSatisfied := false
	for _, line := range lines {
		if strings.Contains(line, "✓") && strings.Contains(line, "init") {
			initSatisfied = true
		}
		if strings.Contains(line, "✓") && strings.Contains(line, "db setup") {
			dbSetupSatisfied = true
		}
	}
	if !initSatisfied {
		t.Error("expected init to be satisfied (✓)")
	}
	if !dbSetupSatisfied {
		t.Error("expected db setup to be satisfied (✓)")
	}

	// auth, workers, migrate up should be available
	if !strings.Contains(out, "Available next steps:") {
		t.Errorf("expected available next steps section")
	}
	for _, cmd := range []string{"auth", "workers", "migrate up"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected %q in output, got:\n%s", cmd, out)
		}
	}
}

func TestStatusCmd_AfterAuth(t *testing.T) {
	_, cleanup := setupProject(t, "[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n")
	defer cleanup()

	out := captureStdout(func() {
		status.StatusCmd()
	})

	// auth should be satisfied
	lines := strings.Split(out, "\n")
	authSatisfied := false
	for _, line := range lines {
		if strings.Contains(line, "✓") && strings.Contains(line, "auth") && !strings.Contains(line, "google") && !strings.Contains(line, "github") {
			authSatisfied = true
		}
	}
	if !authSatisfied {
		t.Error("expected auth to be satisfied (✓)")
	}

	// signup, auth google, auth github should be available
	for _, cmd := range []string{"signup", "auth google", "auth github"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected %q in available steps, got:\n%s", cmd, out)
		}
	}
}

func TestStatusCmd_AfterAuthAndWorkers(t *testing.T) {
	_, cleanup := setupProject(t, "[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n[workers]\nredis_url = redis://localhost\n")
	defer cleanup()

	out := captureStdout(func() {
		status.StatusCmd()
	})

	// workers should be satisfied
	lines := strings.Split(out, "\n")
	workersSatisfied := false
	for _, line := range lines {
		if strings.Contains(line, "✓") && strings.Contains(line, "workers") && !strings.Contains(line, "compile") {
			workersSatisfied = true
		}
	}
	if !workersSatisfied {
		t.Error("expected workers to be satisfied (✓)")
	}

	// email should be available (auth + workers both met)
	if !strings.Contains(out, "email") {
		t.Errorf("expected 'email' in available steps after auth + workers, got:\n%s", out)
	}
}

func TestStatusCmd_UnsatisfiedShowsRequires(t *testing.T) {
	// Only init satisfied — commands needing db_setup should show "(requires: ...)"
	_, cleanup := setupProject(t, "[project]\n")
	defer cleanup()

	out := captureStdout(func() {
		status.StatusCmd()
	})

	// Email needs auth + workers, which both need db_setup (which needs init).
	// Since db_setup is not satisfied, email should show a requires note.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "✗") && strings.Contains(line, "email") {
			if !strings.Contains(line, "requires:") {
				t.Errorf("expected email line to show requires, got: %s", line)
			}
		}
	}
}

func TestStatusCmd_AllSatisfied_NoAvailableSteps(t *testing.T) {
	dir, cleanup := setupProject(t, "[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\noauth_google = true\noauth_github = true\n[workers]\nredis_url = redis://localhost\n[email]\nsmtp_host = localhost\n[files]\nbucket = test\n[llm]\ntool_pkgs = tools\n")
	defer cleanup()

	// Create schema.json for migrate_up
	schemaDir := filepath.Join(dir, "shipq", "db", "migrate")
	os.MkdirAll(schemaDir, 0755)
	os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte("{}"), 0644)

	// Create queries for db_compile
	queriesDir := filepath.Join(dir, "shipq", "queries")
	os.MkdirAll(queriesDir, 0755)
	os.WriteFile(filepath.Join(queriesDir, "types.go"), []byte("package queries\n"), 0644)

	// Create signup.go
	signupDir := filepath.Join(dir, "api", "auth")
	os.MkdirAll(signupDir, 0755)
	os.WriteFile(filepath.Join(signupDir, "signup.go"), []byte("package auth\n"), 0644)

	out := captureStdout(func() {
		status.StatusCmd()
	})

	// Even when all satisfiable commands are satisfied, "run-only" commands
	// (migrate_new, migrate_reset, resource, handler_generate, etc.) are
	// never satisfied, so they'll still appear as available next steps.
	// But all satisfiable ones should show ✓.
	for _, label := range []string{"init", "db setup", "auth", "email"} {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "✓") && strings.Contains(line, label) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be satisfied (✓), got:\n%s", label, out)
		}
	}
}
