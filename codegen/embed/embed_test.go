package embed

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestCopyEmbeddedPackage_CopiesUnitTestFiles(t *testing.T) {
	// Create an in-memory FS with both a regular .go file and a _test.go file
	memFS := fstest.MapFS{
		"src/foo.go":      {Data: []byte("package foo\n")},
		"src/foo_test.go": {Data: []byte("package foo\n\nfunc TestFoo(t *testing.T) {}\n")},
		"src/bar.go":      {Data: []byte("package foo\n\nfunc Bar() {}\n")},
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join("out", "pkg")

	pkg := embeddedPackage{
		fs:      memFS,
		srcDir:  "src",
		destDir: destDir,
	}

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp", "sqlite"); err != nil {
		t.Fatalf("copyEmbeddedPackage failed: %v", err)
	}

	// foo.go should be written
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "foo.go")); err != nil {
		t.Error("expected foo.go to be written, but it was not")
	}

	// bar.go should be written
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "bar.go")); err != nil {
		t.Error("expected bar.go to be written, but it was not")
	}

	// foo_test.go SHOULD be written (unit test files are now included)
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "foo_test.go")); err != nil {
		t.Error("expected foo_test.go to be written, but it was not")
	}
}

func TestCopyEmbeddedPackage_SkipsIntegrationTestFiles(t *testing.T) {
	memFS := fstest.MapFS{
		"src/foo.go":                  {Data: []byte("package foo\n")},
		"src/foo_test.go":             {Data: []byte("package foo\n\nfunc TestFoo(t *testing.T) {}\n")},
		"src/foo_integration_test.go": {Data: []byte("package foo\n\nfunc TestFooIntegration(t *testing.T) {}\n")},
		"src/foo_e2e_test.go":         {Data: []byte("package foo\n\nfunc TestFooE2E(t *testing.T) {}\n")},
		"src/foo_crossdb_test.go":     {Data: []byte("package foo\n\nfunc TestFooCrossDB(t *testing.T) {}\n")},
		"src/foo_fuzz_test.go":        {Data: []byte("package foo\n\nfunc TestFooFuzz(t *testing.T) {}\n")},
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join("out", "pkg")

	pkg := embeddedPackage{
		fs:      memFS,
		srcDir:  "src",
		destDir: destDir,
	}

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp", "sqlite"); err != nil {
		t.Fatalf("copyEmbeddedPackage failed: %v", err)
	}

	// foo.go should be written
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "foo.go")); err != nil {
		t.Error("expected foo.go to be written, but it was not")
	}

	// foo_test.go should be written (unit test)
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "foo_test.go")); err != nil {
		t.Error("expected foo_test.go to be written, but it was not")
	}

	// All infra-heavy test categories should be skipped
	skippedFiles := []string{
		"foo_integration_test.go",
		"foo_e2e_test.go",
		"foo_crossdb_test.go",
		"foo_fuzz_test.go",
	}
	for _, name := range skippedFiles {
		if _, err := os.Stat(filepath.Join(tmpDir, destDir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be skipped, but it was written", name)
		}
	}
}

func TestCopyEmbeddedPackage_SkipsWrongDriverTests(t *testing.T) {
	sqliteTest := []byte("package foo\n\nimport _ \"modernc.org/sqlite\"\n\nfunc TestWithSQLite(t *testing.T) {}\n")
	pgTest := []byte("package foo\n\nimport _ \"github.com/jackc/pgx/v5/stdlib\"\n\nfunc TestWithPG(t *testing.T) {}\n")
	mysqlTest := []byte("package foo\n\nimport _ \"github.com/go-sql-driver/mysql\"\n\nfunc TestWithMySQL(t *testing.T) {}\n")
	pureTest := []byte("package foo\n\nimport \"testing\"\n\nfunc TestPure(t *testing.T) {}\n")

	memFS := fstest.MapFS{
		"src/foo.go":         {Data: []byte("package foo\n")},
		"src/sqlite_test.go": {Data: sqliteTest},
		"src/pg_test.go":     {Data: pgTest},
		"src/mysql_test.go":  {Data: mysqlTest},
		"src/pure_test.go":   {Data: pureTest},
	}

	tests := []struct {
		dialect       string
		expectPresent []string
		expectSkipped []string
	}{
		{
			dialect:       "sqlite",
			expectPresent: []string{"sqlite_test.go", "pure_test.go"},
			expectSkipped: []string{"pg_test.go", "mysql_test.go"},
		},
		{
			dialect:       "postgres",
			expectPresent: []string{"pg_test.go", "pure_test.go"},
			expectSkipped: []string{"sqlite_test.go", "mysql_test.go"},
		},
		{
			dialect:       "mysql",
			expectPresent: []string{"mysql_test.go", "pure_test.go"},
			expectSkipped: []string{"sqlite_test.go", "pg_test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			tmpDir := t.TempDir()
			destDir := filepath.Join("out", "pkg")

			pkg := embeddedPackage{
				fs:      memFS,
				srcDir:  "src",
				destDir: destDir,
			}

			if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp", tt.dialect); err != nil {
				t.Fatalf("copyEmbeddedPackage failed: %v", err)
			}

			for _, name := range tt.expectPresent {
				if _, err := os.Stat(filepath.Join(tmpDir, destDir, name)); err != nil {
					t.Errorf("expected %s to be present for dialect %s, but it was not", name, tt.dialect)
				}
			}

			for _, name := range tt.expectSkipped {
				if _, err := os.Stat(filepath.Join(tmpDir, destDir, name)); !os.IsNotExist(err) {
					t.Errorf("expected %s to be skipped for dialect %s, but it was written", name, tt.dialect)
				}
			}
		})
	}
}

func TestCopyEmbeddedPackage_SkipsNonGoFiles(t *testing.T) {
	memFS := fstest.MapFS{
		"src/foo.go":   {Data: []byte("package foo\n")},
		"src/data.txt": {Data: []byte("some data\n")},
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join("out", "pkg")

	pkg := embeddedPackage{
		fs:      memFS,
		srcDir:  "src",
		destDir: destDir,
	}

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp", "sqlite"); err != nil {
		t.Fatalf("copyEmbeddedPackage failed: %v", err)
	}

	// foo.go should be written
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "foo.go")); err != nil {
		t.Error("expected foo.go to be written, but it was not")
	}

	// data.txt should NOT be written
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "data.txt")); !os.IsNotExist(err) {
		t.Error("expected data.txt to be skipped, but it was written")
	}
}

func TestCopyEmbeddedPackage_RewritesImports(t *testing.T) {
	content := []byte(`package foo

import "github.com/shipq/shipq/handler"

func Foo() {}
`)
	memFS := fstest.MapFS{
		"src/foo.go": {Data: content},
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join("out", "pkg")

	pkg := embeddedPackage{
		fs:      memFS,
		srcDir:  "src",
		destDir: destDir,
	}

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp", "sqlite"); err != nil {
		t.Fatalf("copyEmbeddedPackage failed: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(tmpDir, destDir, "foo.go"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	got := string(written)
	if expected := `"example.com/myapp/shipq/lib/handler"`; !strings.Contains(got, expected) {
		t.Errorf("expected rewritten import %s in output, got:\n%s", expected, got)
	}
	if strings.Contains(got, `"github.com/shipq/shipq/handler"`) {
		t.Error("original import path should have been rewritten")
	}
}

func TestCopyEmbeddedPackage_RewritesImportsInTestFiles(t *testing.T) {
	testContent := []byte(`package foo_test

import (
	"testing"

	"github.com/shipq/shipq/proptest"
	"github.com/shipq/shipq/handler"
)

func TestFoo(t *testing.T) {}
`)
	memFS := fstest.MapFS{
		"src/foo.go":      {Data: []byte("package foo\n")},
		"src/foo_test.go": {Data: testContent},
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join("out", "pkg")

	pkg := embeddedPackage{
		fs:      memFS,
		srcDir:  "src",
		destDir: destDir,
	}

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp", "sqlite"); err != nil {
		t.Fatalf("copyEmbeddedPackage failed: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(tmpDir, destDir, "foo_test.go"))
	if err != nil {
		t.Fatalf("failed to read written test file: %v", err)
	}

	got := string(written)

	// Check that imports were rewritten
	if !strings.Contains(got, `"example.com/myapp/shipq/lib/proptest"`) {
		t.Errorf("expected rewritten proptest import in test file, got:\n%s", got)
	}
	if !strings.Contains(got, `"example.com/myapp/shipq/lib/handler"`) {
		t.Errorf("expected rewritten handler import in test file, got:\n%s", got)
	}

	// Check that original imports were replaced
	if strings.Contains(got, `"github.com/shipq/shipq/proptest"`) {
		t.Error("original proptest import should have been rewritten in test file")
	}
	if strings.Contains(got, `"github.com/shipq/shipq/handler"`) {
		t.Error("original handler import should have been rewritten in test file")
	}
}

func TestEmbedAllPackages_SkipsFilestorageWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-no-files"

	// EmbedAllPackages with FilesEnabled: false
	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   false,
		WorkersEnabled: false,
		DBDialect:      "sqlite",
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// filestorage directory should NOT exist
	filestorageDir := filepath.Join(tmpDir, "shipq", "lib", "filestorage")
	if _, err := os.Stat(filestorageDir); !os.IsNotExist(err) {
		t.Error("expected shipq/lib/filestorage/ to NOT be created when FilesEnabled is false")
	}

	// channel directory should NOT exist
	channelDir := filepath.Join(tmpDir, "shipq", "lib", "channel")
	if _, err := os.Stat(channelDir); !os.IsNotExist(err) {
		t.Error("expected shipq/lib/channel/ to NOT be created when WorkersEnabled is false")
	}

	// Always-required packages should exist
	handlerDir := filepath.Join(tmpDir, "shipq", "lib", "handler")
	if _, err := os.Stat(handlerDir); os.IsNotExist(err) {
		t.Error("expected shipq/lib/handler/ to be created (always required)")
	}
}

func TestEmbedAllPackages_IncludesFilestorageWhenEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-with-files"

	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   true,
		WorkersEnabled: false,
		DBDialect:      "sqlite",
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// filestorage directory should exist
	filestorageDir := filepath.Join(tmpDir, "shipq", "lib", "filestorage")
	if _, err := os.Stat(filestorageDir); os.IsNotExist(err) {
		t.Error("expected shipq/lib/filestorage/ to be created when FilesEnabled is true")
	}

	// Verify at least one .go file was written
	hasGoFile := false
	entries, _ := os.ReadDir(filestorageDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".go" {
			hasGoFile = true
			break
		}
	}
	if !hasGoFile {
		t.Error("expected at least one .go file in shipq/lib/filestorage/")
	}

	// channel should NOT exist (WorkersEnabled is false)
	channelDir := filepath.Join(tmpDir, "shipq", "lib", "channel")
	if _, err := os.Stat(channelDir); !os.IsNotExist(err) {
		t.Error("expected shipq/lib/channel/ to NOT be created when WorkersEnabled is false")
	}
}

func TestEmbedAllPackages_IncludesChannelWhenEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-with-workers"

	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   false,
		WorkersEnabled: true,
		DBDialect:      "sqlite",
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// channel directory should exist
	channelDir := filepath.Join(tmpDir, "shipq", "lib", "channel")
	if _, err := os.Stat(channelDir); os.IsNotExist(err) {
		t.Error("expected shipq/lib/channel/ to be created when WorkersEnabled is true")
	}

	// Verify at least one .go file was written
	hasGoFile := false
	entries, _ := os.ReadDir(channelDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".go" {
			hasGoFile = true
			break
		}
	}
	if !hasGoFile {
		t.Error("expected at least one .go file in shipq/lib/channel/")
	}

	// filestorage should NOT exist (FilesEnabled is false)
	filestorageDir := filepath.Join(tmpDir, "shipq", "lib", "filestorage")
	if _, err := os.Stat(filestorageDir); !os.IsNotExist(err) {
		t.Error("expected shipq/lib/filestorage/ to NOT be created when FilesEnabled is false")
	}
}

func TestEmbedAllPackages_IncludesUnitTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-with-tests"

	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   true,
		WorkersEnabled: true,
		DBDialect:      "sqlite",
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// Walk the output and verify that at least one _test.go file was written
	libDir := filepath.Join(tmpDir, "shipq", "lib")
	foundTestFile := false
	err = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), "_test.go") {
			foundTestFile = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk output directory: %v", err)
	}

	if !foundTestFile {
		t.Error("expected at least one _test.go file in output, but found none")
	}
}

func TestEmbedAllPackages_ExcludesIntegrationTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-no-integration"

	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   true,
		WorkersEnabled: true,
		DBDialect:      "sqlite",
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// Walk the output and verify no integration/e2e/crossdb/fuzz test files were written
	libDir := filepath.Join(tmpDir, "shipq", "lib")
	err = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, "_test.go") {
			return nil
		}
		base := strings.TrimSuffix(name, "_test.go")
		if strings.HasSuffix(base, "_integration") {
			t.Errorf("found integration test file in output: %s", path)
		}
		if strings.HasSuffix(base, "_e2e") {
			t.Errorf("found e2e test file in output: %s", path)
		}
		if strings.HasSuffix(base, "_crossdb") {
			t.Errorf("found crossdb test file in output: %s", path)
		}
		if strings.HasSuffix(base, "_fuzz") {
			t.Errorf("found fuzz test file in output: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk output directory: %v", err)
	}
}

func TestEmbedAllPackages_DefaultsToSQLiteWhenDialectEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-default-dialect"

	// DBDialect left empty — should default to sqlite
	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   false,
		WorkersEnabled: false,
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// Should still work and create packages
	handlerDir := filepath.Join(tmpDir, "shipq", "lib", "handler")
	if _, err := os.Stat(handlerDir); os.IsNotExist(err) {
		t.Error("expected shipq/lib/handler/ to be created")
	}
}

func TestImportsWrongDriver(t *testing.T) {
	sqliteContent := []byte(`package foo

import _ "modernc.org/sqlite"
`)
	pgContent := []byte(`package foo

import _ "github.com/jackc/pgx/v5/stdlib"
`)
	mysqlContent := []byte(`package foo

import _ "github.com/go-sql-driver/mysql"
`)
	pureContent := []byte(`package foo

import "testing"
`)

	tests := []struct {
		name    string
		content []byte
		dialect string
		want    bool
	}{
		{"sqlite content, sqlite dialect", sqliteContent, "sqlite", false},
		{"sqlite content, postgres dialect", sqliteContent, "postgres", true},
		{"sqlite content, mysql dialect", sqliteContent, "mysql", true},
		{"pg content, sqlite dialect", pgContent, "sqlite", true},
		{"pg content, postgres dialect", pgContent, "postgres", false},
		{"pg content, mysql dialect", pgContent, "mysql", true},
		{"mysql content, sqlite dialect", mysqlContent, "sqlite", true},
		{"mysql content, postgres dialect", mysqlContent, "postgres", true},
		{"mysql content, mysql dialect", mysqlContent, "mysql", false},
		{"pure content, sqlite dialect", pureContent, "sqlite", false},
		{"pure content, postgres dialect", pureContent, "postgres", false},
		{"pure content, mysql dialect", pureContent, "mysql", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importsWrongDriver(tt.content, tt.dialect)
			if got != tt.want {
				t.Errorf("importsWrongDriver() = %v, want %v", got, tt.want)
			}
		})
	}
}
