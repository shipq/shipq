package codegen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestLoadDBPackageConfig(t *testing.T) {
	t.Run("loads config from valid project", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create go.mod
		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		// Create shipq.ini
		shipqIni := `[db]
database_url = postgres://user@localhost:5432/mydb
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		cfg, err := codegen.LoadDBPackageConfig(tmpDir)
		if err != nil {
			t.Fatalf("LoadDBPackageConfig() error = %v", err)
		}

		if cfg.ModulePath != "example.com/myapp" {
			t.Errorf("ModulePath = %q, want %q", cfg.ModulePath, "example.com/myapp")
		}
		if cfg.DatabaseURL != "postgres://user@localhost:5432/mydb" {
			t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://user@localhost:5432/mydb")
		}
		if cfg.Dialect != "postgres" {
			t.Errorf("Dialect = %q, want %q", cfg.Dialect, "postgres")
		}
		if cfg.ProjectRoot != tmpDir {
			t.Errorf("ProjectRoot = %q, want %q", cfg.ProjectRoot, tmpDir)
		}
	})

	t.Run("detects mysql dialect", func(t *testing.T) {
		tmpDir := t.TempDir()

		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		shipqIni := `[db]
database_url = mysql://user@localhost:3306/mydb
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		cfg, err := codegen.LoadDBPackageConfig(tmpDir)
		if err != nil {
			t.Fatalf("LoadDBPackageConfig() error = %v", err)
		}

		if cfg.Dialect != "mysql" {
			t.Errorf("Dialect = %q, want %q", cfg.Dialect, "mysql")
		}
	})

	t.Run("detects sqlite dialect", func(t *testing.T) {
		tmpDir := t.TempDir()

		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		shipqIni := `[db]
database_url = sqlite:///path/to/db.sqlite
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		cfg, err := codegen.LoadDBPackageConfig(tmpDir)
		if err != nil {
			t.Fatalf("LoadDBPackageConfig() error = %v", err)
		}

		if cfg.Dialect != "sqlite" {
			t.Errorf("Dialect = %q, want %q", cfg.Dialect, "sqlite")
		}
	})

	t.Run("error when database_url not configured", func(t *testing.T) {
		tmpDir := t.TempDir()

		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		shipqIni := `[db]
migrations = migrations
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		_, err := codegen.LoadDBPackageConfig(tmpDir)
		if err == nil {
			t.Error("LoadDBPackageConfig() expected error when database_url missing")
		}
	})

	t.Run("error when go.mod missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		shipqIni := `[db]
database_url = postgres://user@localhost:5432/mydb
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		_, err := codegen.LoadDBPackageConfig(tmpDir)
		if err == nil {
			t.Error("LoadDBPackageConfig() expected error when go.mod missing")
		}
	})

	t.Run("error when shipq.ini missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		_, err := codegen.LoadDBPackageConfig(tmpDir)
		if err == nil {
			t.Error("LoadDBPackageConfig() expected error when shipq.ini missing")
		}
	})
}

func TestGenerateDBFile(t *testing.T) {
	t.Run("generates valid go code for postgres", func(t *testing.T) {
		cfg := &codegen.DBPackageConfig{
			ProjectRoot: "/fake/root",
			ModulePath:  "example.com/myapp",
			DatabaseURL: "postgres://user@localhost:5432/mydb",
			Dialect:     "postgres",
		}

		content, err := codegen.GenerateDBFile(cfg)
		if err != nil {
			t.Fatalf("GenerateDBFile() error = %v", err)
		}

		contentStr := string(content)

		// Check package declaration
		if !strings.Contains(contentStr, "package db") {
			t.Error("generated code missing 'package db'")
		}

		// Check dialect constant
		if !strings.Contains(contentStr, `const Dialect = "postgres"`) {
			t.Error("generated code missing Dialect constant")
		}

		// Check localhost URL
		if !strings.Contains(contentStr, `const localhostURL = "postgres://user@localhost:5432/mydb"`) {
			t.Error("generated code missing localhostURL constant")
		}

		// Check driver import
		if !strings.Contains(contentStr, `_ "github.com/jackc/pgx/v5/stdlib"`) {
			t.Error("generated code missing pgx import")
		}

		// Check DB function
		if !strings.Contains(contentStr, "func DB() (*sql.DB, error)") {
			t.Error("generated code missing DB() function")
		}

		// Check MustDB function
		if !strings.Contains(contentStr, "func MustDB() *sql.DB") {
			t.Error("generated code missing MustDB() function")
		}

		// Check for generated code header
		if !strings.Contains(contentStr, "Code generated by shipq") {
			t.Error("generated code missing generation header")
		}
	})

	t.Run("generates valid go code for mysql", func(t *testing.T) {
		cfg := &codegen.DBPackageConfig{
			ProjectRoot: "/fake/root",
			ModulePath:  "example.com/myapp",
			DatabaseURL: "mysql://user@localhost:3306/mydb",
			Dialect:     "mysql",
		}

		content, err := codegen.GenerateDBFile(cfg)
		if err != nil {
			t.Fatalf("GenerateDBFile() error = %v", err)
		}

		contentStr := string(content)

		// Check dialect constant
		if !strings.Contains(contentStr, `const Dialect = "mysql"`) {
			t.Error("generated code missing mysql Dialect constant")
		}

		// Check driver import
		if !strings.Contains(contentStr, `_ "github.com/go-sql-driver/mysql"`) {
			t.Error("generated code missing mysql driver import")
		}

		// Check MySQL-specific DSN conversion function
		if !strings.Contains(contentStr, "urlToDSN") {
			t.Error("generated code missing urlToDSN function")
		}

		// Check for tcp format in DSN conversion
		if !strings.Contains(contentStr, "@tcp(") {
			t.Error("generated code missing MySQL tcp format in urlToDSN")
		}
	})

	t.Run("generates valid go code for sqlite", func(t *testing.T) {
		cfg := &codegen.DBPackageConfig{
			ProjectRoot: "/fake/root",
			ModulePath:  "example.com/myapp",
			DatabaseURL: "sqlite:///path/to/db.sqlite",
			Dialect:     "sqlite",
		}

		content, err := codegen.GenerateDBFile(cfg)
		if err != nil {
			t.Fatalf("GenerateDBFile() error = %v", err)
		}

		contentStr := string(content)

		// Check dialect constant
		if !strings.Contains(contentStr, `const Dialect = "sqlite"`) {
			t.Error("generated code missing sqlite Dialect constant")
		}

		// Check driver import
		if !strings.Contains(contentStr, `_ "modernc.org/sqlite"`) {
			t.Error("generated code missing sqlite driver import")
		}
	})

	t.Run("error for unsupported dialect", func(t *testing.T) {
		cfg := &codegen.DBPackageConfig{
			ProjectRoot: "/fake/root",
			ModulePath:  "example.com/myapp",
			DatabaseURL: "oracle://user@localhost:1521/mydb",
			Dialect:     "oracle",
		}

		_, err := codegen.GenerateDBFile(cfg)
		if err == nil {
			t.Error("GenerateDBFile() expected error for unsupported dialect")
		}
	})
}

func TestEnsureDBPackage(t *testing.T) {
	t.Run("creates shipq/db directory and db.go file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create go.mod
		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		// Create shipq.ini
		shipqIni := `[db]
database_url = postgres://user@localhost:5432/mydb
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		err := codegen.EnsureDBPackage(tmpDir)
		if err != nil {
			t.Fatalf("EnsureDBPackage() error = %v", err)
		}

		// Verify directory was created
		dbPkgPath := filepath.Join(tmpDir, "shipq", "db")
		if _, err := os.Stat(dbPkgPath); os.IsNotExist(err) {
			t.Error("EnsureDBPackage() did not create shipq/db directory")
		}

		// Verify db.go was created
		dbFilePath := filepath.Join(dbPkgPath, "db.go")
		if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
			t.Error("EnsureDBPackage() did not create db.go file")
		}

		// Verify file content
		content, err := os.ReadFile(dbFilePath)
		if err != nil {
			t.Fatalf("failed to read db.go: %v", err)
		}

		if !strings.Contains(string(content), "package db") {
			t.Error("db.go missing package declaration")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create go.mod
		goMod := "module example.com/myapp\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		// Create shipq.ini
		shipqIni := `[db]
database_url = postgres://user@localhost:5432/mydb
`
		if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
			t.Fatalf("failed to write shipq.ini: %v", err)
		}

		// Run twice
		err := codegen.EnsureDBPackage(tmpDir)
		if err != nil {
			t.Fatalf("first EnsureDBPackage() error = %v", err)
		}

		err = codegen.EnsureDBPackage(tmpDir)
		if err != nil {
			t.Fatalf("second EnsureDBPackage() error = %v", err)
		}

		// Verify file still exists and is valid
		dbFilePath := filepath.Join(tmpDir, "shipq", "db", "db.go")
		content, err := os.ReadFile(dbFilePath)
		if err != nil {
			t.Fatalf("failed to read db.go: %v", err)
		}

		if !strings.Contains(string(content), "package db") {
			t.Error("db.go missing package declaration after second run")
		}
	})
}
