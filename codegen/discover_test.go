package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPackages_EmptyDir(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the querydefs directory but leave it empty
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	if err := os.MkdirAll(querydefsDir, 0755); err != nil {
		t.Fatalf("failed to create querydefs dir: %v", err)
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverPackages_MissingDir(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Don't create querydefs - it should return empty, not error
	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverPackages_SinglePackage(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create querydefs with a Go file
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	if err := os.MkdirAll(querydefsDir, 0755); err != nil {
		t.Fatalf("failed to create querydefs dir: %v", err)
	}

	goFile := filepath.Join(querydefsDir, "users.go")
	if err := os.WriteFile(goFile, []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}

	expected := "example.com/myapp/querydefs"
	if pkgs[0] != expected {
		t.Errorf("expected package %q, got %q", expected, pkgs[0])
	}
}

func TestDiscoverPackages_NestedPackages(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested package structure
	dirs := []string{
		"querydefs",
		"querydefs/users",
		"querydefs/orders",
		"querydefs/products/inventory",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		// Create a Go file in each
		goFile := filepath.Join(fullPath, "queries.go")
		if err := os.WriteFile(goFile, []byte("package "+filepath.Base(dir)+"\n"), 0644); err != nil {
			t.Fatalf("failed to create Go file in %s: %v", dir, err)
		}
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find 4 packages
	if len(pkgs) != 4 {
		t.Fatalf("expected 4 packages, got %d: %v", len(pkgs), pkgs)
	}

	// Check that all expected packages are present
	expectedPkgs := map[string]bool{
		"example.com/myapp/querydefs":                    false,
		"example.com/myapp/querydefs/users":              false,
		"example.com/myapp/querydefs/orders":             false,
		"example.com/myapp/querydefs/products/inventory": false,
	}

	for _, pkg := range pkgs {
		if _, ok := expectedPkgs[pkg]; !ok {
			t.Errorf("unexpected package: %s", pkg)
		}
		expectedPkgs[pkg] = true
	}

	for pkg, found := range expectedPkgs {
		if !found {
			t.Errorf("missing expected package: %s", pkg)
		}
	}
}

func TestDiscoverPackages_SkipsTestFiles(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create querydefs with only test files
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	if err := os.MkdirAll(querydefsDir, 0755); err != nil {
		t.Fatalf("failed to create querydefs dir: %v", err)
	}

	testFile := filepath.Join(querydefsDir, "users_test.go")
	if err := os.WriteFile(testFile, []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find 0 packages since only test files exist
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages (only test files), got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverPackages_SkipsHiddenDirs(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create querydefs with a hidden subdirectory
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	hiddenDir := filepath.Join(querydefsDir, ".hidden")

	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("failed to create hidden dir: %v", err)
	}

	// Create Go files in both
	if err := os.WriteFile(filepath.Join(querydefsDir, "queries.go"), []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "hidden.go"), []byte("package hidden\n"), 0644); err != nil {
		t.Fatalf("failed to create hidden Go file: %v", err)
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find only 1 package (hidden dir should be skipped)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}

	expected := "example.com/myapp/querydefs"
	if pkgs[0] != expected {
		t.Errorf("expected package %q, got %q", expected, pkgs[0])
	}
}

func TestDiscoverPackages_SkipsVendor(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create querydefs with a vendor subdirectory
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	vendorDir := filepath.Join(querydefsDir, "vendor")

	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}

	// Create Go files in both
	if err := os.WriteFile(filepath.Join(querydefsDir, "queries.go"), []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "vendored.go"), []byte("package vendor\n"), 0644); err != nil {
		t.Fatalf("failed to create vendor Go file: %v", err)
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find only 1 package (vendor dir should be skipped)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverPackages_SkipsTestdata(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create querydefs with a testdata subdirectory
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	testdataDir := filepath.Join(querydefsDir, "testdata")

	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		t.Fatalf("failed to create testdata dir: %v", err)
	}

	// Create Go files in both
	if err := os.WriteFile(filepath.Join(querydefsDir, "queries.go"), []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testdataDir, "fixtures.go"), []byte("package testdata\n"), 0644); err != nil {
		t.Fatalf("failed to create testdata Go file: %v", err)
	}

	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find only 1 package (testdata dir should be skipped)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverQuerydefsPackages(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create querydefs with a Go file
	querydefsDir := filepath.Join(tmpDir, "querydefs")
	if err := os.MkdirAll(querydefsDir, 0755); err != nil {
		t.Fatalf("failed to create querydefs dir: %v", err)
	}

	goFile := filepath.Join(querydefsDir, "users.go")
	if err := os.WriteFile(goFile, []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}

	pkgs, err := DiscoverQuerydefsPackages(tmpDir, "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverQuerydefsPackages failed: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}

	expected := "example.com/myapp/querydefs"
	if pkgs[0] != expected {
		t.Errorf("expected package %q, got %q", expected, pkgs[0])
	}
}

func TestContainsGoFiles(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test empty directory
	has, err := containsGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("containsGoFiles failed: %v", err)
	}
	if has {
		t.Error("expected false for empty directory")
	}

	// Add a non-Go file
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# Readme\n"), 0644); err != nil {
		t.Fatalf("failed to create readme: %v", err)
	}

	has, err = containsGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("containsGoFiles failed: %v", err)
	}
	if has {
		t.Error("expected false for directory with only non-Go files")
	}

	// Add a Go file
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}

	has, err = containsGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("containsGoFiles failed: %v", err)
	}
	if !has {
		t.Error("expected true for directory with Go file")
	}
}

func TestContainsGoFiles_OnlyTestFiles(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "shipq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Add only test files
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	has, err := containsGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("containsGoFiles failed: %v", err)
	}
	if has {
		t.Error("expected false for directory with only test files")
	}
}
