package discovery

import (
	"os"
	"path/filepath"
	"strings"
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

	// In standard case, goModRoot and shipqRoot are the same
	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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
	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/myapp")
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

	pkgs, err := DiscoverQuerydefsPackages(tmpDir, tmpDir, "example.com/myapp")
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

func TestDiscoverPackages_MonorepoSetup(t *testing.T) {
	// Create a monorepo structure:
	// tmpDir/ (goModRoot)
	//   go.mod
	//   services/
	//     myservice/ (shipqRoot)
	//       shipq.ini
	//       querydefs/
	//         users.go
	//         orders/
	//           orders.go
	tmpDir, err := os.MkdirTemp("", "shipq-monorepo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	goModRoot := tmpDir
	shipqRoot := filepath.Join(tmpDir, "services", "myservice")

	// Create directories
	if err := os.MkdirAll(filepath.Join(shipqRoot, "querydefs", "orders"), 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}

	// Create go.mod in root
	if err := os.WriteFile(filepath.Join(goModRoot, "go.mod"), []byte("module github.com/company/monorepo\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create shipq.ini in service directory
	if err := os.WriteFile(filepath.Join(shipqRoot, "shipq.ini"), []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to create shipq.ini: %v", err)
	}

	// Create Go files
	if err := os.WriteFile(filepath.Join(shipqRoot, "querydefs", "users.go"), []byte("package querydefs\n"), 0644); err != nil {
		t.Fatalf("failed to create users.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shipqRoot, "querydefs", "orders", "orders.go"), []byte("package orders\n"), 0644); err != nil {
		t.Fatalf("failed to create orders.go: %v", err)
	}

	// Discover packages with different goModRoot and shipqRoot
	modulePath := "github.com/company/monorepo"
	pkgs, err := DiscoverPackages(goModRoot, shipqRoot, "querydefs", modulePath)
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find 2 packages with import paths relative to goModRoot
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %v", len(pkgs), pkgs)
	}

	// Check that import paths are relative to goModRoot, not shipqRoot
	expectedPkgs := map[string]bool{
		"github.com/company/monorepo/services/myservice/querydefs":        false,
		"github.com/company/monorepo/services/myservice/querydefs/orders": false,
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

func TestDiscoverQuerydefsPackages_MonorepoSetup(t *testing.T) {
	// Regression test: in a monorepo, DiscoverQuerydefsPackages must receive the
	// raw module path (from go.mod), NOT the full import prefix that includes the
	// subpath. Passing the full import prefix causes the subpath to be doubled:
	//   "com.company/apps/myservice/apps/myservice/querydefs/users"
	// instead of:
	//   "com.company/apps/myservice/querydefs/users"

	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "apps", "myservice")

	// Create querydefs with nested packages
	for _, dir := range []string{"querydefs", "querydefs/users", "querydefs/orders"} {
		fullPath := filepath.Join(shipqRoot, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		goFile := filepath.Join(fullPath, "queries.go")
		if err := os.WriteFile(goFile, []byte("package "+filepath.Base(dir)+"\n"), 0644); err != nil {
			t.Fatalf("failed to create Go file in %s: %v", dir, err)
		}
	}

	// Correct usage: pass the raw module path from go.mod
	rawModulePath := "com.company"
	pkgs, err := DiscoverQuerydefsPackages(goModRoot, shipqRoot, rawModulePath)
	if err != nil {
		t.Fatalf("DiscoverQuerydefsPackages failed: %v", err)
	}

	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d: %v", len(pkgs), pkgs)
	}

	expectedPkgs := map[string]bool{
		"com.company/apps/myservice/querydefs":        false,
		"com.company/apps/myservice/querydefs/users":  false,
		"com.company/apps/myservice/querydefs/orders": false,
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

	// Verify no package path contains a doubled subpath
	for _, pkg := range pkgs {
		if strings.Contains(pkg, "apps/myservice/apps/myservice") {
			t.Errorf("import path contains doubled subpath: %s", pkg)
		}
	}
}

func TestDiscoverPackages_MonorepoFullImportPrefixDoublesSubpath(t *testing.T) {
	// This test documents the API contract: DiscoverPackages expects the raw
	// module path (from go.mod), NOT a full import prefix that already includes
	// the monorepo subpath. Passing the full import prefix causes the subpath
	// to appear twice in the generated import paths.
	//
	// This was the root cause of a bug in DBCompileCmd where cfg.ModulePath
	// (which is moduleInfo.FullImportPath("")) was passed instead of the raw
	// module path, producing imports like:
	//   "com.company/apps/platform/backend/apps/platform/backend/querydefs/users"

	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "apps", "platform", "backend")

	// Create a querydefs package
	querydefsDir := filepath.Join(shipqRoot, "querydefs", "users")
	if err := os.MkdirAll(querydefsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(querydefsDir, "queries.go"), []byte("package users\n"), 0644); err != nil {
		t.Fatalf("failed to create Go file: %v", err)
	}

	rawModulePath := "com.company"
	subPath := "apps/platform/backend"
	fullImportPrefix := rawModulePath + "/" + subPath

	// CORRECT: raw module path produces correct import paths
	correctPkgs, err := DiscoverPackages(goModRoot, shipqRoot, "querydefs", rawModulePath)
	if err != nil {
		t.Fatalf("DiscoverPackages with raw module path failed: %v", err)
	}

	if len(correctPkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(correctPkgs), correctPkgs)
	}

	expectedCorrect := "com.company/apps/platform/backend/querydefs/users"
	if correctPkgs[0] != expectedCorrect {
		t.Errorf("expected %q, got %q", expectedCorrect, correctPkgs[0])
	}

	// WRONG: full import prefix doubles the subpath — this demonstrates the bug
	wrongPkgs, err := DiscoverPackages(goModRoot, shipqRoot, "querydefs", fullImportPrefix)
	if err != nil {
		t.Fatalf("DiscoverPackages with full import prefix failed: %v", err)
	}

	if len(wrongPkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(wrongPkgs), wrongPkgs)
	}

	// The subpath appears twice — this is the doubled-path bug
	doubled := "com.company/apps/platform/backend/apps/platform/backend/querydefs/users"
	if wrongPkgs[0] != doubled {
		t.Fatalf("expected doubled path %q, got %q", doubled, wrongPkgs[0])
	}

	if !strings.Contains(wrongPkgs[0], subPath+"/"+subPath) {
		t.Error("expected the subpath to appear twice when using full import prefix")
	}
}
