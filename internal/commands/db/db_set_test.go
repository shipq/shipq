package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/inifile"
)

// ── DefaultDatabaseURL tests ─────────────────────────────────────────────────

func TestDefaultDatabaseURL_SQLite_Structure(t *testing.T) {
	projectRoot := "/tmp/test-project"
	url := DefaultDatabaseURL("sqlite", "myapp", projectRoot)

	if !strings.HasPrefix(url, "sqlite://") {
		t.Errorf("expected sqlite:// prefix, got %q", url)
	}
	if !strings.Contains(url, ".shipq/data/myapp.db") {
		t.Errorf("expected path to contain .shipq/data/myapp.db, got %q", url)
	}
	if !strings.Contains(url, projectRoot) {
		t.Errorf("expected path to contain project root %q, got %q", projectRoot, url)
	}
}

func TestDefaultDatabaseURL_Postgres_Structure(t *testing.T) {
	url := DefaultDatabaseURL("postgres", "myapp", "/tmp/whatever")

	expected := "postgres://postgres@localhost:5432/myapp?sslmode=disable"
	if url != expected {
		t.Errorf("expected %q, got %q", expected, url)
	}
}

func TestDefaultDatabaseURL_MySQL_Structure(t *testing.T) {
	url := DefaultDatabaseURL("mysql", "myapp", "/tmp/whatever")

	expected := "mysql://root@localhost:3306/myapp"
	if url != expected {
		t.Errorf("expected %q, got %q", expected, url)
	}
}

func TestDefaultDatabaseURL_UnknownDialectDefaultsToSQLite(t *testing.T) {
	url := DefaultDatabaseURL("unknown", "myapp", "/tmp/test")

	if !strings.HasPrefix(url, "sqlite://") {
		t.Errorf("unknown dialect should fall through to sqlite, got %q", url)
	}
}

func TestDefaultDatabaseURL_ProjectNameInURL(t *testing.T) {
	tests := []struct {
		dialect     string
		projectName string
		wantSubstr  string
	}{
		{"postgres", "cool-app", "cool-app"},
		{"mysql", "my-project", "my-project"},
		{"sqlite", "demo", "demo.db"},
	}
	for _, tt := range tests {
		t.Run(tt.dialect+"/"+tt.projectName, func(t *testing.T) {
			url := DefaultDatabaseURL(tt.dialect, tt.projectName, "/tmp/root")
			if !strings.Contains(url, tt.wantSubstr) {
				t.Errorf("expected URL to contain %q, got %q", tt.wantSubstr, url)
			}
		})
	}
}

// ── IsValidDialect tests ─────────────────────────────────────────────────────

func TestIsValidDialect_AcceptsValid(t *testing.T) {
	for _, d := range []string{"sqlite", "postgres", "mysql"} {
		if !IsValidDialect(d) {
			t.Errorf("IsValidDialect(%q) should be true", d)
		}
	}
}

func TestIsValidDialect_RejectsInvalid(t *testing.T) {
	for _, d := range []string{"", "postgresql", "mssql", "SQLite", "POSTGRES"} {
		if IsValidDialect(d) {
			t.Errorf("IsValidDialect(%q) should be false", d)
		}
	}
}

// ── DBSetCmd integration tests ───────────────────────────────────────────────

// setupDBSetProject creates a minimal project directory with go.mod and
// shipq.ini so that DBSetCmd's project.FindProjectRoots can locate the project.
// It changes the working directory to the project and returns a cleanup
// function that restores the original working directory.
func setupDBSetProject(t *testing.T, dialect string) (projectDir string, cleanup func()) {
	t.Helper()

	projectDir = t.TempDir()
	projectName := filepath.Base(projectDir)

	// Create go.mod
	goMod := "module com." + projectName + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create shipq.ini with initial dialect
	initialURL := DefaultDatabaseURL(dialect, projectName, projectDir)
	ini := &inifile.File{}
	ini.Sections = append(ini.Sections, inifile.Section{
		Name: "db",
		Values: []inifile.KeyValue{
			{Key: "database_url", Value: initialURL},
		},
	})
	iniPath := filepath.Join(projectDir, "shipq.ini")
	if err := ini.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Change to project directory so FindProjectRoots works
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cleanup = func() {
		os.Chdir(origDir)
	}

	return projectDir, cleanup
}

func TestDBSetCmd_SwitchesToPostgres(t *testing.T) {
	projectDir, cleanup := setupDBSetProject(t, "sqlite")
	defer cleanup()

	DBSetCmd("postgres")

	ini, err := inifile.ParseFile(filepath.Join(projectDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	url := ini.Get("db", "database_url")
	if !strings.HasPrefix(url, "postgres://") {
		t.Errorf("expected postgres:// URL, got %q", url)
	}
	if !strings.Contains(url, "localhost:5432") {
		t.Errorf("expected localhost:5432 in URL, got %q", url)
	}
}

func TestDBSetCmd_SwitchesToMySQL(t *testing.T) {
	projectDir, cleanup := setupDBSetProject(t, "sqlite")
	defer cleanup()

	DBSetCmd("mysql")

	ini, err := inifile.ParseFile(filepath.Join(projectDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	url := ini.Get("db", "database_url")
	if !strings.HasPrefix(url, "mysql://") {
		t.Errorf("expected mysql:// URL, got %q", url)
	}
	if !strings.Contains(url, "localhost:3306") {
		t.Errorf("expected localhost:3306 in URL, got %q", url)
	}
}

func TestDBSetCmd_SwitchesToSQLite(t *testing.T) {
	projectDir, cleanup := setupDBSetProject(t, "postgres")
	defer cleanup()

	DBSetCmd("sqlite")

	ini, err := inifile.ParseFile(filepath.Join(projectDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	url := ini.Get("db", "database_url")
	if !strings.HasPrefix(url, "sqlite://") {
		t.Errorf("expected sqlite:// URL, got %q", url)
	}
	if !strings.Contains(url, ".db") {
		t.Errorf("expected .db extension in URL, got %q", url)
	}
}

func TestDBSetCmd_PreservesOtherIniSections(t *testing.T) {
	projectDir, cleanup := setupDBSetProject(t, "sqlite")
	defer cleanup()

	// Add extra sections to shipq.ini before calling DBSetCmd
	iniPath := filepath.Join(projectDir, "shipq.ini")
	ini, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}
	ini.Sections = append(ini.Sections, inifile.Section{
		Name: "typescript",
		Values: []inifile.KeyValue{
			{Key: "framework", Value: "react"},
		},
	})
	if err := ini.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	DBSetCmd("postgres")

	// Re-read and verify [typescript] section is still there
	updatedIni, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse updated shipq.ini: %v", err)
	}

	if updatedIni.Get("typescript", "framework") != "react" {
		t.Error("expected [typescript] framework = react to be preserved")
	}

	// And the URL was still updated
	url := updatedIni.Get("db", "database_url")
	if !strings.HasPrefix(url, "postgres://") {
		t.Errorf("expected postgres:// URL, got %q", url)
	}
}

func TestDBSetCmd_Idempotent(t *testing.T) {
	projectDir, cleanup := setupDBSetProject(t, "sqlite")
	defer cleanup()

	DBSetCmd("postgres")
	DBSetCmd("postgres") // second call should not error

	ini, err := inifile.ParseFile(filepath.Join(projectDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	url := ini.Get("db", "database_url")
	if !strings.HasPrefix(url, "postgres://") {
		t.Errorf("expected postgres:// URL after idempotent calls, got %q", url)
	}
}

func TestDBSetCmd_SQLiteURLContainsProjectName(t *testing.T) {
	projectDir, cleanup := setupDBSetProject(t, "postgres")
	defer cleanup()

	DBSetCmd("sqlite")

	ini, err := inifile.ParseFile(filepath.Join(projectDir, "shipq.ini"))
	if err != nil {
		t.Fatalf("failed to parse shipq.ini: %v", err)
	}

	url := ini.Get("db", "database_url")
	projectName := filepath.Base(projectDir)
	if !strings.Contains(url, projectName+".db") {
		t.Errorf("expected SQLite URL to contain %s.db, got %q", projectName, url)
	}
}
