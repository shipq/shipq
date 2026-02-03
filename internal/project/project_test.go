package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/config"
)

// resolveSymlinks resolves any symlinks in a path for consistent comparison.
// This is needed on macOS where /tmp is a symlink to /private/tmp.
func resolveSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", path, err)
	}
	return resolved
}

func TestFindRoot_FromProjectDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.ConfigFilename, "[db]\n")

	root, found, err := FindRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find root")
	}
	if root.Dir != dir {
		t.Errorf("expected Dir=%q, got %q", dir, root.Dir)
	}
	expectedConfig := filepath.Join(dir, config.ConfigFilename)
	if root.ConfigPath != expectedConfig {
		t.Errorf("expected ConfigPath=%q, got %q", expectedConfig, root.ConfigPath)
	}
}

func TestFindRoot_FromSubdirectory(t *testing.T) {
	// Create project structure:
	// /tmp/project/shipq.ini
	// /tmp/project/sub/dir/
	projectDir := t.TempDir()
	writeFile(t, projectDir, config.ConfigFilename, "[db]\n")

	subDir := filepath.Join(projectDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	root, found, err := FindRoot(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find root from subdirectory")
	}
	if root.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, root.Dir)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	// Use a temp dir that definitely doesn't have shipq.ini
	dir := t.TempDir()

	root, found, err := FindRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Errorf("expected not found, but got root: %+v", root)
	}
	if root != nil {
		t.Errorf("expected nil root when not found, got %+v", root)
	}
}

func TestFindRoot_EmptyStartDir_UsesCwd(t *testing.T) {
	// Create a project in temp dir and cd into it
	projectDir := resolveSymlinks(t, t.TempDir())
	writeFile(t, projectDir, config.ConfigFilename, "[db]\n")

	oldDir := changeDir(t, projectDir)
	defer os.Chdir(oldDir)

	root, found, err := FindRoot("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find root")
	}
	if root.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, root.Dir)
	}
}

func TestFindRoot_IgnoresDirectory(t *testing.T) {
	// If shipq.ini is a directory (not a file), it should be ignored
	dir := t.TempDir()
	configDir := filepath.Join(dir, config.ConfigFilename)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	root, found, err := FindRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Errorf("expected not found when shipq.ini is a directory, got: %+v", root)
	}
}

func TestResolve_Found(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.ConfigFilename, "[db]\n")

	root, err := Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root.Dir != dir {
		t.Errorf("expected Dir=%q, got %q", dir, root.Dir)
	}
}

func TestResolve_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := Resolve(dir)
	if err == nil {
		t.Fatal("expected error when project not found")
	}

	// Error should mention helpful information
	errStr := err.Error()
	if !contains(errStr, "shipq.ini") {
		t.Errorf("error should mention shipq.ini: %v", err)
	}
	if !contains(errStr, "--project") {
		t.Errorf("error should mention --project flag: %v", err)
	}
}

func TestResolveWithOverride_ValidOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.ConfigFilename, "[db]\n")

	root, err := ResolveWithOverride(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root.Dir != dir {
		t.Errorf("expected Dir=%q, got %q", dir, root.Dir)
	}
}

func TestResolveWithOverride_InvalidOverride_NotExist(t *testing.T) {
	_, err := ResolveWithOverride("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestResolveWithOverride_InvalidOverride_NotDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not_a_dir")
	writeFile(t, dir, "not_a_dir", "content")

	_, err := ResolveWithOverride(filePath)
	if err == nil {
		t.Fatal("expected error for file path")
	}
	if !contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' error, got: %v", err)
	}
}

func TestResolveWithOverride_InvalidOverride_NoConfig(t *testing.T) {
	dir := t.TempDir()
	// No shipq.ini in the directory

	_, err := ResolveWithOverride(dir)
	if err == nil {
		t.Fatal("expected error when config not found in override dir")
	}
	if !contains(err.Error(), config.ConfigFilename) {
		t.Errorf("expected error to mention %s, got: %v", config.ConfigFilename, err)
	}
}

func TestResolveWithOverride_EmptyOverride_SearchesUpward(t *testing.T) {
	projectDir := resolveSymlinks(t, t.TempDir())
	writeFile(t, projectDir, config.ConfigFilename, "[db]\n")

	subDir := filepath.Join(projectDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldDir := changeDir(t, subDir)
	defer os.Chdir(oldDir)

	root, err := ResolveWithOverride("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, root.Dir)
	}
}

func TestResolveWithOverride_RelativePath(t *testing.T) {
	projectDir := resolveSymlinks(t, t.TempDir())
	writeFile(t, projectDir, config.ConfigFilename, "[db]\n")

	subDir := filepath.Join(projectDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldDir := changeDir(t, subDir)
	defer os.Chdir(oldDir)

	// Use relative path ".."
	root, err := ResolveWithOverride("..")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, root.Dir)
	}
}

func TestMustResolve_Found(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.ConfigFilename, "[db]\n")

	rootDir, err := MustResolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rootDir != dir {
		t.Errorf("expected %q, got %q", dir, rootDir)
	}
}

func TestMustResolve_NotFound_FallsBackToCwd(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	// No shipq.ini

	oldDir := changeDir(t, dir)
	defer os.Chdir(oldDir)

	rootDir, err := MustResolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rootDir != dir {
		t.Errorf("expected fallback to cwd %q, got %q", dir, rootDir)
	}
}

func TestMustResolve_NotFound_FallsBackToStartDir(t *testing.T) {
	dir := t.TempDir()
	// No shipq.ini

	rootDir, err := MustResolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rootDir != dir {
		t.Errorf("expected fallback to %q, got %q", dir, rootDir)
	}
}

func TestFindRoot_DeepNesting(t *testing.T) {
	// Test finding root from deeply nested directory
	projectDir := t.TempDir()
	writeFile(t, projectDir, config.ConfigFilename, "[db]\n")

	deepDir := filepath.Join(projectDir, "a", "b", "c", "d", "e", "f")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	root, found, err := FindRoot(deepDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find root from deep nesting")
	}
	if root.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, root.Dir)
	}
}

func TestFindRoot_StopsAtClosestConfig(t *testing.T) {
	// If there are nested projects, stop at the closest one
	outerDir := t.TempDir()
	writeFile(t, outerDir, config.ConfigFilename, "[db]\nurl = outer")

	innerDir := filepath.Join(outerDir, "inner")
	if err := os.MkdirAll(innerDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, innerDir, config.ConfigFilename, "[db]\nurl = inner")

	subDir := filepath.Join(innerDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	root, found, err := FindRoot(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find root")
	}
	// Should find innerDir, not outerDir
	if root.Dir != innerDir {
		t.Errorf("expected Dir=%q (inner), got %q", innerDir, root.Dir)
	}
}

// Helper functions

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func changeDir(t *testing.T, dir string) string {
	t.Helper()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return oldDir
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
