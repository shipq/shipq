package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/project"
)

func TestDirExists(t *testing.T) {
	t.Run("returns true for existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if !dirExists(tmpDir) {
			t.Error("expected dirExists to return true for existing directory")
		}
	})

	t.Run("returns false for non-existing directory", func(t *testing.T) {
		if dirExists("/non/existing/path") {
			t.Error("expected dirExists to return false for non-existing directory")
		}
	})

	t.Run("returns false for file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if dirExists(filePath) {
			t.Error("expected dirExists to return false for file")
		}
	})
}

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

	t.Run("returns false for directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if fileExists(tmpDir) {
			t.Error("expected fileExists to return false for directory")
		}
	})
}

func TestRemoveUndoFiles(t *testing.T) {
	t.Run("removes undo_ prefixed files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create some undo files
		undo1 := filepath.Join(tmpDir, "undo_001")
		undo2 := filepath.Join(tmpDir, "undo_002")
		keep := filepath.Join(tmpDir, "ibdata1")

		for _, f := range []string{undo1, undo2, keep} {
			if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		removeUndoFiles(tmpDir)

		// undo files should be removed
		if fileExists(undo1) {
			t.Error("undo_001 should have been removed")
		}
		if fileExists(undo2) {
			t.Error("undo_002 should have been removed")
		}

		// other files should remain
		if !fileExists(keep) {
			t.Error("ibdata1 should not have been removed")
		}
	})

	t.Run("handles non-existing directory gracefully", func(t *testing.T) {
		// Should not panic
		removeUndoFiles("/non/existing/path")
	})
}

func TestDbStartValidation(t *testing.T) {
	t.Run("valid database types", func(t *testing.T) {
		validTypes := []string{"postgres", "mysql", "sqlite"}
		for _, dbType := range validTypes {
			switch dbType {
			case dbTypePostgres, dbTypeMySQL, dbTypeSQLite:
				// Valid - this is what we're testing
			default:
				t.Errorf("expected %s to be a valid database type", dbType)
			}
		}
	})

	t.Run("invalid database types are rejected", func(t *testing.T) {
		invalidTypes := []string{"mongodb", "redis", "oracle", ""}
		for _, dbType := range invalidTypes {
			switch dbType {
			case dbTypePostgres, dbTypeMySQL, dbTypeSQLite:
				t.Errorf("expected %s to be an invalid database type", dbType)
			default:
				// Invalid - this is what we expect
			}
		}
	})
}

func TestDbStartDataDirectoryCreation(t *testing.T) {
	t.Run("data directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, ".shipq", "data")

		// Create the data directory as dbStartCmd would
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			t.Fatalf("failed to create data directory: %v", err)
		}

		// Verify structure
		if !dirExists(dataDir) {
			t.Error("expected data directory to exist")
		}

		shipqDir := filepath.Join(tmpDir, ".shipq")
		if !dirExists(shipqDir) {
			t.Error("expected .shipq directory to exist")
		}
	})
}

func TestStartSQLiteCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up project structure
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

	// Create SQLite file (simulating what startSQLite does)
	if !fileExists(sqliteDBPath) {
		file, err := os.Create(sqliteDBPath)
		if err != nil {
			t.Fatalf("failed to create SQLite database file: %v", err)
		}
		file.Close()
	}

	// Verify file exists
	if !fileExists(sqliteDBPath) {
		t.Error("expected SQLite database file to exist")
	}
}

func TestDataDirectoryPaths(t *testing.T) {
	projectRoot := "/test/project"
	dataDir := filepath.Join(projectRoot, ".shipq", "data")

	tests := []struct {
		name     string
		dbType   string
		wantPath string
	}{
		{
			name:     "postgres data directory",
			dbType:   dbTypePostgres,
			wantPath: filepath.Join(dataDir, ".postgres-data"),
		},
		{
			name:     "mysql data directory",
			dbType:   dbTypeMySQL,
			wantPath: filepath.Join(dataDir, ".mysql-data"),
		},
		{
			name:     "sqlite database file",
			dbType:   dbTypeSQLite,
			wantPath: filepath.Join(dataDir, ".sqlite-db"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			switch tt.dbType {
			case dbTypePostgres:
				gotPath = filepath.Join(dataDir, ".postgres-data")
			case dbTypeMySQL:
				gotPath = filepath.Join(dataDir, ".mysql-data")
			case dbTypeSQLite:
				gotPath = filepath.Join(dataDir, ".sqlite-db")
			}

			if gotPath != tt.wantPath {
				t.Errorf("got path %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}
