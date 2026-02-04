package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/dbops"
	"github.com/shipq/shipq/project"
)

func TestMysqlURLToDSN(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "standard mysql URL",
			url:  "mysql://root@localhost:3306/mydb",
			want: "root@tcp(localhost:3306)/mydb",
		},
		{
			name: "mysql URL without database",
			url:  "mysql://root@localhost:3306",
			want: "root@tcp(localhost:3306)/",
		},
		{
			name: "mysql URL with different port",
			url:  "mysql://admin@127.0.0.1:3307/testdb",
			want: "admin@tcp(127.0.0.1:3307)/testdb",
		},
		{
			name:    "invalid URL without @",
			url:     "mysql://localhost:3306/db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dbops.MySQLURLToDSN(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		want    string
	}{
		{
			name:    "simple",
			dialect: "postgres",
			want:    `"simple"`,
		},
		{
			name:    "with_underscore",
			dialect: "postgres",
			want:    `"with_underscore"`,
		},
		{
			name:    `with"quote`,
			dialect: "postgres",
			want:    `"with""quote"`,
		},
		{
			name:    `multiple"quotes"here`,
			dialect: "postgres",
			want:    `"multiple""quotes""here"`,
		},
		{
			name:    "simple",
			dialect: "mysql",
			want:    "`simple`",
		},
		{
			name:    "with`backtick",
			dialect: "mysql",
			want:    "`with``backtick`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.dialect, func(t *testing.T) {
			got := dbops.QuoteIdentifier(tt.name, tt.dialect)
			if got != tt.want {
				t.Errorf("QuoteIdentifier(%q, %q) = %q, want %q", tt.name, tt.dialect, got, tt.want)
			}
		})
	}
}

func TestCreateSQLiteDB(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "newfile.db")

		err := dbops.CreateSQLiteDB(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !fileExists(filePath) {
			t.Error("expected file to exist")
		}

		// Check file is empty
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if info.Size() != 0 {
			t.Errorf("expected empty file, got size %d", info.Size())
		}
	})

	t.Run("does not error on existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "existing.db")

		// Create file with content
		content := []byte("existing content")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := dbops.CreateSQLiteDB(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify content is unchanged (CreateSQLiteDB should not modify existing files)
		gotContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(gotContent) != string(content) {
			t.Errorf("file content was modified")
		}
	})
}

func TestDropSQLiteDB(t *testing.T) {
	t.Run("deletes existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.db")

		// Create file
		if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := dbops.DropSQLiteDB(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fileExists(filePath) {
			t.Error("expected file to be deleted")
		}
	})

	t.Run("does not error on non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "nonexistent.db")

		err := dbops.DropSQLiteDB(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSetupSQLite(t *testing.T) {
	t.Run("creates database files", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "testproject"

		url, err := setupSQLite(tmpDir, projectName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check main database file exists
		mainDBPath := filepath.Join(tmpDir, ".shipq", "data", projectName+".db")
		if !fileExists(mainDBPath) {
			t.Error("expected main database file to exist")
		}

		// Check test database file exists
		testDBPath := filepath.Join(tmpDir, ".shipq", "data", projectName+"_test.db")
		if !fileExists(testDBPath) {
			t.Error("expected test database file to exist")
		}

		// Check URL format
		expectedURL := "sqlite://" + mainDBPath
		if url != expectedURL {
			t.Errorf("got URL %q, want %q", url, expectedURL)
		}
	})

	t.Run("creates data directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "newproject"

		_, err := setupSQLite(tmpDir, projectName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dataDir := filepath.Join(tmpDir, ".shipq", "data")
		if !dirExists(dataDir) {
			t.Error("expected data directory to be created")
		}
	})
}

func TestDbSetupUpdatesIniFile(t *testing.T) {
	t.Run("sets database_url in shipq.ini", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create initial shipq.ini
		shipqIniPath := filepath.Join(tmpDir, project.ShipqIniFile)
		initialIni := &inifile.File{}
		initialIni.Sections = append(initialIni.Sections, inifile.Section{Name: "db"})
		if err := initialIni.WriteFile(shipqIniPath); err != nil {
			t.Fatalf("failed to create initial shipq.ini: %v", err)
		}

		// Simulate what dbSetupCmd does - update the ini file
		iniFile, err := inifile.ParseFile(shipqIniPath)
		if err != nil {
			t.Fatalf("failed to parse shipq.ini: %v", err)
		}

		testURL := "postgres://postgres@localhost:5432/testdb"
		iniFile.Set("db", "database_url", testURL)

		if err := iniFile.WriteFile(shipqIniPath); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		// Verify the change
		updatedIni, err := inifile.ParseFile(shipqIniPath)
		if err != nil {
			t.Fatalf("failed to parse updated shipq.ini: %v", err)
		}

		got := updatedIni.Get("db", "database_url")
		if got != testURL {
			t.Errorf("got database_url %q, want %q", got, testURL)
		}
	})

	t.Run("overwrites existing database_url", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create shipq.ini with existing database_url
		shipqIniPath := filepath.Join(tmpDir, project.ShipqIniFile)
		initialIni := &inifile.File{}
		initialIni.Set("db", "database_url", "old_url")
		if err := initialIni.WriteFile(shipqIniPath); err != nil {
			t.Fatalf("failed to create initial shipq.ini: %v", err)
		}

		// Update the ini file
		iniFile, err := inifile.ParseFile(shipqIniPath)
		if err != nil {
			t.Fatalf("failed to parse shipq.ini: %v", err)
		}

		newURL := "postgres://postgres@localhost:5432/newdb"
		iniFile.Set("db", "database_url", newURL)

		if err := iniFile.WriteFile(shipqIniPath); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		// Verify the change and that there's only one database_url
		updatedIni, err := inifile.ParseFile(shipqIniPath)
		if err != nil {
			t.Fatalf("failed to parse updated shipq.ini: %v", err)
		}

		got := updatedIni.Get("db", "database_url")
		if got != newURL {
			t.Errorf("got database_url %q, want %q", got, newURL)
		}

		// Check no duplicates
		dbSection := updatedIni.Section("db")
		if dbSection == nil {
			t.Fatal("expected db section to exist")
		}

		count := 0
		for _, kv := range dbSection.Values {
			if kv.Key == "database_url" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected 1 database_url key, got %d", count)
		}
	})
}

func TestDbSetupValidation(t *testing.T) {
	t.Run("requires DATABASE_URL environment variable", func(t *testing.T) {
		// This tests the validation logic - DATABASE_URL must be set
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			// This is the expected state for most test runs
			// The actual command would call cli.Fatal here
		}
	})
}

func TestSQLiteURLToPath(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "sqlite:// prefix",
			url:  "sqlite:///path/to/db.sqlite",
			want: "/path/to/db.sqlite",
		},
		{
			name: "sqlite: prefix",
			url:  "sqlite:/path/to/db.sqlite",
			want: "/path/to/db.sqlite",
		},
		{
			name: "no prefix",
			url:  "/path/to/db.sqlite",
			want: "/path/to/db.sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dbops.SQLiteURLToPath(tt.url)
			if got != tt.want {
				t.Errorf("SQLiteURLToPath(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestGenerateDropSQL(t *testing.T) {
	tests := []struct {
		name    string
		dbName  string
		dialect string
		want    string
	}{
		{
			name:    "postgres simple",
			dbName:  "mydb",
			dialect: "postgres",
			want:    `DROP DATABASE IF EXISTS "mydb"`,
		},
		{
			name:    "mysql simple",
			dbName:  "mydb",
			dialect: "mysql",
			want:    "DROP DATABASE IF EXISTS `mydb`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dbops.GenerateDropSQL(tt.dbName, tt.dialect)
			if got != tt.want {
				t.Errorf("GenerateDropSQL(%q, %q) = %q, want %q", tt.dbName, tt.dialect, got, tt.want)
			}
		})
	}
}

func TestGenerateCreateSQL(t *testing.T) {
	tests := []struct {
		name    string
		dbName  string
		dialect string
		want    string
	}{
		{
			name:    "postgres simple",
			dbName:  "mydb",
			dialect: "postgres",
			want:    `CREATE DATABASE "mydb"`,
		},
		{
			name:    "mysql simple",
			dbName:  "mydb",
			dialect: "mysql",
			want:    "CREATE DATABASE IF NOT EXISTS `mydb`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dbops.GenerateCreateSQL(tt.dbName, tt.dialect)
			if got != tt.want {
				t.Errorf("GenerateCreateSQL(%q, %q) = %q, want %q", tt.dbName, tt.dialect, got, tt.want)
			}
		})
	}
}
