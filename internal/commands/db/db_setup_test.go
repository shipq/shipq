package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/dbops"
	"github.com/shipq/shipq/project"
)

// mockCommandExists creates a mock for commandExists that returns true for specified commands.
func mockCommandExists(available map[string]bool) func(string) bool {
	return func(name string) bool {
		return available[name]
	}
}

func TestDetectDatabaseDialect(t *testing.T) {
	// Save original and restore after test
	originalCommandExists := commandExists
	defer func() { commandExists = originalCommandExists }()

	tests := []struct {
		name      string
		available map[string]bool
		want      string
	}{
		{
			name:      "mysql available - should pick MySQL first",
			available: map[string]bool{"mysqld": true, "postgres": true},
			want:      dburl.DialectMySQL,
		},
		{
			name:      "only mysql available",
			available: map[string]bool{"mysqld": true},
			want:      dburl.DialectMySQL,
		},
		{
			name:      "only postgres available",
			available: map[string]bool{"postgres": true},
			want:      dburl.DialectPostgres,
		},
		{
			name:      "postgres available but not mysqld",
			available: map[string]bool{"postgres": true, "mysqld": false},
			want:      dburl.DialectPostgres,
		},
		{
			name:      "nothing available - fallback to SQLite",
			available: map[string]bool{},
			want:      dburl.DialectSQLite,
		},
		{
			name:      "neither mysqld nor postgres",
			available: map[string]bool{"mysqld": false, "postgres": false},
			want:      dburl.DialectSQLite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandExists = mockCommandExists(tt.available)

			got := detectDatabaseDialect()
			if got != tt.want {
				t.Errorf("detectDatabaseDialect() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferDatabaseURL(t *testing.T) {
	// Save original and restore after test
	originalCommandExists := commandExists
	defer func() { commandExists = originalCommandExists }()

	tests := []struct {
		name        string
		available   map[string]bool
		projectName string
		wantDialect string
		wantURLHas  string // substring that should be in the URL
	}{
		{
			name:        "mysql detected",
			available:   map[string]bool{"mysqld": true},
			projectName: "myapp",
			wantDialect: dburl.DialectMySQL,
			wantURLHas:  "mysql://",
		},
		{
			name:        "postgres detected",
			available:   map[string]bool{"postgres": true},
			projectName: "myapp",
			wantDialect: dburl.DialectPostgres,
			wantURLHas:  "postgres://",
		},
		{
			name:        "sqlite fallback",
			available:   map[string]bool{},
			projectName: "myapp",
			wantDialect: dburl.DialectSQLite,
			wantURLHas:  "sqlite:",
		},
		{
			name:        "sqlite URL contains project name",
			available:   map[string]bool{},
			projectName: "testproject",
			wantDialect: dburl.DialectSQLite,
			wantURLHas:  "testproject.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandExists = mockCommandExists(tt.available)
			tmpDir := t.TempDir()

			gotURL, gotDialect := inferDatabaseURL(tmpDir, tt.projectName)

			if gotDialect != tt.wantDialect {
				t.Errorf("inferDatabaseURL() dialect = %q, want %q", gotDialect, tt.wantDialect)
			}

			if !strings.Contains(gotURL, tt.wantURLHas) {
				t.Errorf("inferDatabaseURL() URL = %q, want it to contain %q", gotURL, tt.wantURLHas)
			}
		})
	}
}

func TestInferDatabaseURL_SQLitePathStructure(t *testing.T) {
	// Save original and restore after test
	originalCommandExists := commandExists
	defer func() { commandExists = originalCommandExists }()

	// Force SQLite by making no other DB available
	commandExists = mockCommandExists(map[string]bool{})

	tmpDir := t.TempDir()
	projectName := "myproject"

	gotURL, gotDialect := inferDatabaseURL(tmpDir, projectName)

	if gotDialect != dburl.DialectSQLite {
		t.Fatalf("expected SQLite dialect, got %q", gotDialect)
	}

	// SQLite URL should point to .shipq/data/<project>.db
	expectedPath := filepath.Join(tmpDir, ".shipq", "data", projectName+".db")
	if !strings.Contains(gotURL, expectedPath) {
		t.Errorf("SQLite URL = %q, expected to contain path %q", gotURL, expectedPath)
	}
}

func TestDbSetupWithoutDATABASE_URL(t *testing.T) {
	// This test verifies that db setup works without DATABASE_URL set
	// by falling back to detection logic

	// Save original and restore after test
	originalCommandExists := commandExists
	defer func() { commandExists = originalCommandExists }()

	// Force SQLite (no external DB dependencies needed for test)
	commandExists = mockCommandExists(map[string]bool{})

	tmpDir := t.TempDir()
	projectName := "testdbsetup"

	// Ensure DATABASE_URL is not set for this specific test logic
	// (The actual command would read from env, but we're testing inferDatabaseURL)
	url, dialect := inferDatabaseURL(tmpDir, projectName)

	if dialect != dburl.DialectSQLite {
		t.Errorf("expected SQLite dialect when nothing available, got %q", dialect)
	}

	if url == "" {
		t.Error("expected non-empty URL to be inferred")
	}

	if !strings.HasPrefix(url, "sqlite:") {
		t.Errorf("expected SQLite URL, got %q", url)
	}
}

func TestDbSetupDialectPriority(t *testing.T) {
	// Verify the priority order: MySQL > Postgres > SQLite
	originalCommandExists := commandExists
	defer func() { commandExists = originalCommandExists }()

	t.Run("MySQL takes priority over Postgres", func(t *testing.T) {
		commandExists = mockCommandExists(map[string]bool{
			"mysqld":   true,
			"postgres": true,
		})

		dialect := detectDatabaseDialect()
		if dialect != dburl.DialectMySQL {
			t.Errorf("expected MySQL when both available, got %q", dialect)
		}
	})

	t.Run("Postgres takes priority over SQLite", func(t *testing.T) {
		commandExists = mockCommandExists(map[string]bool{
			"mysqld":   false,
			"postgres": true,
		})

		dialect := detectDatabaseDialect()
		if dialect != dburl.DialectPostgres {
			t.Errorf("expected Postgres when MySQL unavailable, got %q", dialect)
		}
	})

	t.Run("SQLite is the fallback", func(t *testing.T) {
		commandExists = mockCommandExists(map[string]bool{
			"mysqld":   false,
			"postgres": false,
		})

		dialect := detectDatabaseDialect()
		if dialect != dburl.DialectSQLite {
			t.Errorf("expected SQLite fallback, got %q", dialect)
		}
	})
}

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
	t.Run("DATABASE_URL is optional - falls back to detection", func(t *testing.T) {
		// Save original and restore after test
		originalCommandExists := commandExists
		defer func() { commandExists = originalCommandExists }()

		// When DATABASE_URL is not set, we should fall back to detection
		commandExists = mockCommandExists(map[string]bool{})

		tmpDir := t.TempDir()
		projectName := "validationtest"

		// This should work without DATABASE_URL by falling back to SQLite
		url, dialect := inferDatabaseURL(tmpDir, projectName)

		if dialect != dburl.DialectSQLite {
			t.Errorf("expected SQLite fallback, got %q", dialect)
		}
		if url == "" {
			t.Error("expected URL to be inferred even without DATABASE_URL")
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
