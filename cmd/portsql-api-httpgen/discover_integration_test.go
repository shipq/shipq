//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover_ValidPackage(t *testing.T) {
	// 1. Create temp module with valid handlers
	tmpDir := t.TempDir()
	modDir := getModuleDir(t)

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

func Register(app *portapi.App) {
	app.Get("/pets", ListPets)
	app.Post("/pets", CreatePet)
	app.Get("/health", Health)
}

func ListPets(ctx context.Context) ([]Pet, error) {
	return nil, nil
}

func CreatePet(ctx context.Context, req CreatePetRequest) (Pet, error) {
	return Pet{}, nil
}

func Health(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// 2. Run Discover(pkgPath) from the temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	manifest, err := Discover("example.com/testapp/api")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// 3. Assert manifest contains expected endpoints
	if len(manifest.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(manifest.Endpoints))
	}

	// Find endpoints by path
	endpointMap := make(map[string]ManifestEndpoint)
	for _, ep := range manifest.Endpoints {
		key := ep.Method + " " + ep.Path
		endpointMap[key] = ep
	}

	// Check GET /pets
	getPets, ok := endpointMap["GET /pets"]
	if !ok {
		t.Fatal("GET /pets should exist")
	}
	if getPets.Shape != "ctx_resp_err" {
		t.Errorf("GET /pets shape: got %q, want %q", getPets.Shape, "ctx_resp_err")
	}
	if getPets.HandlerName != "ListPets" {
		t.Errorf("GET /pets handler: got %q, want %q", getPets.HandlerName, "ListPets")
	}
	if !strings.Contains(getPets.RespType, "Pet") {
		t.Errorf("GET /pets resp type should contain Pet, got %q", getPets.RespType)
	}

	// Check POST /pets
	postPets, ok := endpointMap["POST /pets"]
	if !ok {
		t.Fatal("POST /pets should exist")
	}
	if postPets.Shape != "ctx_req_resp_err" {
		t.Errorf("POST /pets shape: got %q, want %q", postPets.Shape, "ctx_req_resp_err")
	}
	if postPets.HandlerName != "CreatePet" {
		t.Errorf("POST /pets handler: got %q, want %q", postPets.HandlerName, "CreatePet")
	}
	if !strings.Contains(postPets.ReqType, "CreatePetRequest") {
		t.Errorf("POST /pets req type should contain CreatePetRequest, got %q", postPets.ReqType)
	}

	// Check GET /health
	health, ok := endpointMap["GET /health"]
	if !ok {
		t.Fatal("GET /health should exist")
	}
	if health.Shape != "ctx_err" {
		t.Errorf("GET /health shape: got %q, want %q", health.Shape, "ctx_err")
	}
	if health.HandlerName != "Health" {
		t.Errorf("GET /health handler: got %q, want %q", health.HandlerName, "Health")
	}
}

func TestDiscover_InvalidHandler(t *testing.T) {
	// 1. Create temp module with invalid handler
	tmpDir := t.TempDir()
	modDir := getModuleDir(t)

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

	// Create handlers.go with invalid handler (missing error return)
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func Register(app *portapi.App) {
	app.Get("/bad", BadHandler)
}

// BadHandler is invalid: returns string instead of error
func BadHandler(ctx context.Context) string {
	return "bad"
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// 2. Run Discover(pkgPath) from the temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// 3. Assert error with clear message
	_, err = Discover("example.com/testapp/api")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The error should indicate that the handler validation failed
	if !strings.Contains(err.Error(), "runner failed") {
		t.Errorf("expected error to contain 'runner failed', got %q", err.Error())
	}
}

func TestDiscover_PathBindings(t *testing.T) {
	// Test that path bindings are validated correctly
	tmpDir := t.TempDir()
	modDir := getModuleDir(t)

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

	// Create handlers.go with path bindings
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

type Pet struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type GetPetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

func Register(app *portapi.App) {
	app.Get("/pets/{id}", GetPet)
}

func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	return Pet{}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// Run Discover from the temp dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	manifest, err := Discover("example.com/testapp/api")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Verify endpoint was discovered
	if len(manifest.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(manifest.Endpoints))
	}
	ep := manifest.Endpoints[0]
	if ep.Method != "GET" {
		t.Errorf("method: got %q, want %q", ep.Method, "GET")
	}
	if ep.Path != "/pets/{id}" {
		t.Errorf("path: got %q, want %q", ep.Path, "/pets/{id}")
	}
	if ep.HandlerName != "GetPet" {
		t.Errorf("handler: got %q, want %q", ep.HandlerName, "GetPet")
	}
	if ep.Shape != "ctx_req_resp_err" {
		t.Errorf("shape: got %q, want %q", ep.Shape, "ctx_req_resp_err")
	}
}
