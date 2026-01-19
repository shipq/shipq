//go:build integration

package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_EndToEnd(t *testing.T) {
	// Get the module dir BEFORE changing directories
	modDir := getModuleDir(t)

	// 1. Create temp module with api/ package
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module example.com/testapp

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + modDir + `
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create empty go.sum
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// Create api package directory
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create handlers.go with valid handlers
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

type Pet struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type CreatePetRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

type GetPetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

type DeletePetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

func Register(app *portapi.App) {
	app.Get("/pets", ListPets)
	app.Post("/pets", CreatePet)
	app.Get("/pets/{id}", GetPet)
	app.Delete("/pets/{id}", DeletePet)
	app.Get("/health", Health)
}

func ListPets(ctx context.Context) ([]Pet, error) {
	return nil, nil
}

func CreatePet(ctx context.Context, req CreatePetRequest) (Pet, error) {
	return Pet{}, nil
}

func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	return Pet{}, nil
}

func DeletePet(ctx context.Context, req DeletePetRequest) error {
	return nil
}

func Health(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// 2. Create portsql-api-httpgen.ini pointing to full import path
	iniContent := `[httpgen]
package = example.com/testapp/api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	// 3. Run go mod tidy in the temp directory to set up dependencies
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// 4. Run the generator from the temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// 5. Assert zz_generated_http.go exists
	generatedPath := filepath.Join(apiDir, "zz_generated_http.go")
	if _, err := os.Stat(generatedPath); err != nil {
		t.Fatalf("generated file should exist: %v", err)
	}

	// Read the generated file
	generatedCode, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	codeStr := string(generatedCode)

	// Verify it contains the DO NOT EDIT header
	if !strings.Contains(codeStr, "DO NOT EDIT") {
		t.Error("expected generated code to contain DO NOT EDIT header")
	}

	// Verify it contains all endpoint patterns
	expectedPatterns := []string{
		`"GET /pets"`,
		`"POST /pets"`,
		`"GET /pets/{id}"`,
		`"DELETE /pets/{id}"`,
		`"GET /health"`,
	}
	for _, pattern := range expectedPatterns {
		if !strings.Contains(codeStr, pattern) {
			t.Errorf("expected generated code to contain %s", pattern)
		}
	}

	// Verify it declares NewMux function
	if !strings.Contains(codeStr, "func NewMux()") {
		t.Error("expected generated code to contain func NewMux()")
	}

	// 6. Verify the generated code parses as valid Go
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, generatedPath, generatedCode, 0); err != nil {
		t.Fatalf("generated code must be valid Go: %v", err)
	}
}

func TestMain_EndToEnd_ConfigFromEnv(t *testing.T) {
	// Get the module dir BEFORE changing directories
	modDir := getModuleDir(t)

	// Test that PORTSQL_API_HTTPGEN_CONFIG env var works
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module example.com/testapp

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + modDir + `
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create empty go.sum
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// Create api package directory
	apiDir := filepath.Join(tmpDir, "myapi")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create minimal handlers
	handlersCode := `package myapi

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func Register(app *portapi.App) {
	app.Get("/health", Health)
}

func Health(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Create config in a different location
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	iniContent := `[httpgen]
package = example.com/testapp/myapi
`
	configPath := filepath.Join(configDir, "custom-config.ini")
	if err := os.WriteFile(configPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set env var
	t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", configPath)

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// Run from tmpDir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// Verify generated file exists
	generatedPath := filepath.Join(apiDir, "zz_generated_http.go")
	if _, err := os.Stat(generatedPath); err != nil {
		t.Fatalf("generated file should exist: %v", err)
	}

	// Verify it has the correct package name
	generatedCode, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	if !strings.Contains(string(generatedCode), "package myapi") {
		t.Error("expected generated code to contain 'package myapi'")
	}
}

func TestMain_EndToEnd_NoConfig(t *testing.T) {
	// Test that missing config produces a clear error
	tmpDir := t.TempDir()

	// Clear env var
	t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", "")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	err = run()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "config not found") {
		t.Errorf("expected error to contain 'config not found', got %q", err.Error())
	}
}

func TestMain_EndToEnd_InvalidPackage(t *testing.T) {
	// Get the module dir BEFORE changing directories
	modDir := getModuleDir(t)

	// Test that invalid package produces a clear error
	tmpDir := t.TempDir()

	// Create config pointing to non-existent package
	iniContent := `[httpgen]
package = ./nonexistent
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	// Create go.mod
	goMod := `module example.com/testapp

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + modDir + `
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	err = run()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "discovering endpoints") {
		t.Errorf("expected error to contain 'discovering endpoints', got %q", err.Error())
	}
}

func TestMain_EndToEnd_Deterministic(t *testing.T) {
	// Get the module dir BEFORE changing directories
	modDir := getModuleDir(t)

	// Test that running the generator twice produces identical output
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module example.com/testapp

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + modDir

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create empty go.sum
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// Create api package directory
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create handlers with multiple endpoints
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

type Data struct {
	Value string ` + "`json:\"value\"`" + `
}

func Register(app *portapi.App) {
	app.Get("/z", Z)
	app.Get("/a", A)
	app.Post("/m", M)
	app.Delete("/b", B)
}

func Z(ctx context.Context) error { return nil }
func A(ctx context.Context) error { return nil }
func M(ctx context.Context, req Data) (Data, error) { return Data{}, nil }
func B(ctx context.Context) error { return nil }
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Create config
	iniContent := `[httpgen]
package = example.com/testapp/api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Run first time
	if err := run(); err != nil {
		t.Fatalf("first run() failed: %v", err)
	}

	generatedPath := filepath.Join(apiDir, "zz_generated_http.go")
	code1, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	// Delete generated file
	if err := os.Remove(generatedPath); err != nil {
		t.Fatalf("failed to remove generated file: %v", err)
	}

	// Run second time
	if err := run(); err != nil {
		t.Fatalf("second run() failed: %v", err)
	}

	code2, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file second time: %v", err)
	}

	// Compare - should be byte-identical
	if !bytes.Equal(code1, code2) {
		t.Error("generated code should be deterministic")
	}
}

// getModuleDir returns the directory of the current module
func getModuleDir(t *testing.T) string {
	t.Helper()

	// Walk up from current directory to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod in parent directories")
		}
		dir = parent
	}
}
