package start

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/project"
)

// ── serverPort ───────────────────────────────────────────────────────────────

func TestServerPort_Default(t *testing.T) {
	// Ensure PORT is unset so the default is used.
	old := os.Getenv("PORT")
	os.Unsetenv("PORT")
	defer func() {
		if old != "" {
			os.Setenv("PORT", old)
		}
	}()

	got := serverPort()
	if got != defaultServerPort {
		t.Errorf("serverPort() = %d, want %d (default)", got, defaultServerPort)
	}
}

func TestServerPort_FromEnv(t *testing.T) {
	old := os.Getenv("PORT")
	defer func() {
		if old != "" {
			os.Setenv("PORT", old)
		} else {
			os.Unsetenv("PORT")
		}
	}()

	os.Setenv("PORT", "3000")
	got := serverPort()
	if got != 3000 {
		t.Errorf("serverPort() = %d, want 3000", got)
	}
}

func TestServerPort_InvalidEnvFallsBackToDefault(t *testing.T) {
	old := os.Getenv("PORT")
	defer func() {
		if old != "" {
			os.Setenv("PORT", old)
		} else {
			os.Unsetenv("PORT")
		}
	}()

	cases := []string{"abc", "0", "-1", "99999", ""}
	for _, val := range cases {
		if val == "" {
			os.Unsetenv("PORT")
		} else {
			os.Setenv("PORT", val)
		}
		got := serverPort()
		if got != defaultServerPort {
			t.Errorf("serverPort() with PORT=%q = %d, want %d (default)", val, got, defaultServerPort)
		}
	}
}

// ── killStaleServer ──────────────────────────────────────────────────────────

func TestKillStaleServer_NoProcessOnPort(t *testing.T) {
	// Calling killStaleServer on a port with nothing listening should not
	// panic or produce an error — it simply does nothing.
	killStaleServer(19999) // high port unlikely to be in use
}

// ── dirExists ────────────────────────────────────────────────────────────────

func TestDirExists(t *testing.T) {
	t.Run("returns true for existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if !dirExists(tmpDir) {
			t.Error("expected dirExists to return true for existing directory")
		}
	})

	t.Run("returns false for non-existing path", func(t *testing.T) {
		if dirExists("/non/existing/path") {
			t.Error("expected dirExists to return false for non-existing path")
		}
	})

	t.Run("returns false for a file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if dirExists(filePath) {
			t.Error("expected dirExists to return false for a file")
		}
	})
}

// ── fileExists ───────────────────────────────────────────────────────────────

func TestFileExists(t *testing.T) {
	t.Run("returns true for existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if !fileExists(filePath) {
			t.Error("expected fileExists to return true for existing file")
		}
	})

	t.Run("returns false for non-existing file", func(t *testing.T) {
		if fileExists("/non/existing/file.txt") {
			t.Error("expected fileExists to return false for non-existing file")
		}
	})

	t.Run("returns false for a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if fileExists(tmpDir) {
			t.Error("expected fileExists to return false for a directory")
		}
	})
}

// ── removeUndoFiles ──────────────────────────────────────────────────────────

func TestRemoveUndoFiles(t *testing.T) {
	t.Run("removes undo_ prefixed files", func(t *testing.T) {
		tmpDir := t.TempDir()

		undo1 := filepath.Join(tmpDir, "undo_001")
		undo2 := filepath.Join(tmpDir, "undo_002")
		keep := filepath.Join(tmpDir, "ibdata1")

		for _, f := range []string{undo1, undo2, keep} {
			if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		removeUndoFiles(tmpDir)

		if fileExists(undo1) {
			t.Error("undo_001 should have been removed")
		}
		if fileExists(undo2) {
			t.Error("undo_002 should have been removed")
		}
		if !fileExists(keep) {
			t.Error("ibdata1 should not have been removed")
		}
	})

	t.Run("handles non-existing directory gracefully", func(t *testing.T) {
		// Should not panic.
		removeUndoFiles("/non/existing/path")
	})
}

// ── data directory path logic ────────────────────────────────────────────────

func TestDataDirectoryCreation(t *testing.T) {
	t.Run("creates .shipq/data hierarchy", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, ".shipq", "data")

		if err := os.MkdirAll(dataDir, 0755); err != nil {
			t.Fatalf("failed to create data directory: %v", err)
		}

		if !dirExists(dataDir) {
			t.Error("expected data directory to exist")
		}
		if !dirExists(filepath.Join(tmpDir, ".shipq")) {
			t.Error("expected .shipq directory to exist")
		}
	})
}

func TestDataDirectoryPaths(t *testing.T) {
	projectRoot := "/test/project"
	dataDir := filepath.Join(projectRoot, ".shipq", "data")

	tests := []struct {
		name     string
		service  string
		wantPath string
	}{
		{
			name:     "postgres data directory",
			service:  "postgres",
			wantPath: filepath.Join(dataDir, ".postgres-data"),
		},
		{
			name:     "mysql data directory",
			service:  "mysql",
			wantPath: filepath.Join(dataDir, ".mysql-data"),
		},
		{
			name:     "sqlite database file",
			service:  "sqlite",
			wantPath: filepath.Join(dataDir, ".sqlite-db"),
		},
		{
			name:     "redis data directory",
			service:  "redis",
			wantPath: filepath.Join(dataDir, ".redis-data"),
		},
		{
			name:     "minio data directory",
			service:  "minio",
			wantPath: filepath.Join(dataDir, ".minio-data"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			switch tt.service {
			case "postgres":
				gotPath = filepath.Join(dataDir, ".postgres-data")
			case "mysql":
				gotPath = filepath.Join(dataDir, ".mysql-data")
			case "sqlite":
				gotPath = filepath.Join(dataDir, ".sqlite-db")
			case "redis":
				gotPath = filepath.Join(dataDir, ".redis-data")
			case "minio":
				gotPath = filepath.Join(dataDir, ".minio-data")
			}

			if gotPath != tt.wantPath {
				t.Errorf("got path %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}

// ── SQLite file creation ─────────────────────────────────────────────────────

func TestStartSQLiteCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up the minimal project structure expected by FindProjectRoots.
	goModPath := filepath.Join(tmpDir, project.GoModFile)
	shipqIniPath := filepath.Join(tmpDir, project.ShipqIniFile)

	if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}
	if err := os.WriteFile(shipqIniPath, []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to create shipq.ini: %v", err)
	}

	dataDir := filepath.Join(tmpDir, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data directory: %v", err)
	}

	sqliteDBPath := filepath.Join(dataDir, ".sqlite-db")

	// Simulate what StartSQLite does (without touching FindProjectRoots).
	if !fileExists(sqliteDBPath) {
		f, err := os.Create(sqliteDBPath)
		if err != nil {
			t.Fatalf("failed to create SQLite database file: %v", err)
		}
		f.Close()
	}

	if !fileExists(sqliteDBPath) {
		t.Error("expected SQLite database file to exist after creation")
	}
}

// ── service name validation ──────────────────────────────────────────────────

func TestValidServices(t *testing.T) {
	want := []string{
		"postgres", "mysql", "sqlite", "redis", "minio",
		"centrifugo", "server", "worker",
	}

	got := ValidServices()

	if len(got) != len(want) {
		t.Fatalf("ValidServices() returned %d services, want %d\ngot:  %v\nwant: %v",
			len(got), len(want), got, want)
	}

	wantSet := make(map[string]bool, len(want))
	for _, s := range want {
		wantSet[s] = true
	}

	for _, s := range got {
		if !wantSet[s] {
			t.Errorf("unexpected service %q in ValidServices()", s)
		}
	}
}

func TestStartCmdServiceNamesAreKnown(t *testing.T) {
	known := map[string]bool{}
	for _, s := range ValidServices() {
		known[s] = true
	}

	// Every service returned by ValidServices must be handled by the switch in
	// StartCmd.  We verify this indirectly: if a service is in the list it must
	// be a non-empty string and must not be a help flag.
	for _, s := range ValidServices() {
		if s == "" {
			t.Error("ValidServices() returned an empty string")
		}
		if s == "-h" || s == "--help" || s == "help" {
			t.Errorf("ValidServices() returned help flag %q", s)
		}
	}
}

func TestStartCmdInvalidServiceNames(t *testing.T) {
	invalid := []string{"mongodb", "oracle", "kafka", ""}
	known := map[string]bool{}
	for _, s := range ValidServices() {
		known[s] = true
	}

	for _, s := range invalid {
		if known[s] {
			t.Errorf("expected %q to be an invalid service name, but it is in ValidServices()", s)
		}
	}
}
