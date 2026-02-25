package embed

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestCopyEmbeddedPackage_SkipsTestFiles(t *testing.T) {
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

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp"); err != nil {
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

	// foo_test.go should NOT be written
	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "foo_test.go")); !os.IsNotExist(err) {
		t.Error("expected foo_test.go to be skipped, but it was written")
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

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp"); err != nil {
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

	if err := copyEmbeddedPackage(pkg, tmpDir, "example.com/myapp"); err != nil {
		t.Fatalf("copyEmbeddedPackage failed: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(tmpDir, destDir, "foo.go"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	got := string(written)
	if expected := `"example.com/myapp/shipq/lib/handler"`; !containsString(got, expected) {
		t.Errorf("expected rewritten import %s in output, got:\n%s", expected, got)
	}
	if containsString(got, `"github.com/shipq/shipq/handler"`) {
		t.Error("original import path should have been rewritten")
	}
}

func TestEmbedAllPackages_SkipsFilestorageWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-no-files"

	// EmbedAllPackages with FilesEnabled: false
	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   false,
		WorkersEnabled: false,
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

func TestEmbedAllPackages_NoTestFilesInOutput(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "example.com/test-no-test-files"

	err := EmbedAllPackages(tmpDir, modulePath, EmbedOptions{
		FilesEnabled:   true,
		WorkersEnabled: true,
	})
	if err != nil {
		t.Fatalf("EmbedAllPackages failed: %v", err)
	}

	// Walk the entire output and verify no _test.go files were written
	libDir := filepath.Join(tmpDir, "shipq", "lib")
	err = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".go" {
			if len(d.Name()) > len("_test.go") && d.Name()[len(d.Name())-len("_test.go"):] == "_test.go" {
				t.Errorf("found _test.go file in output: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk output directory: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
