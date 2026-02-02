//go:build integration

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPI_GeneratesFile(t *testing.T) {
	modDir := getModuleDir(t)
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

	// Create handlers.go
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

// Pet represents a pet in the system.
type Pet struct {
	// ID is the unique identifier.
	ID   string ` + "`json:\"id\"`" + `
	// Name is the pet's display name.
	Name string ` + "`json:\"name\"`" + `
}

// GetPetRequest is the request to get a pet.
type GetPetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

// CreatePetRequest is the request to create a pet.
type CreatePetRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Get("/pets/{id}", GetPet)
	app.Post("/pets", CreatePet)
	app.Get("/health", Health)
}

// GetPet retrieves a pet by ID.
func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	return Pet{}, nil
}

// CreatePet creates a new pet.
func CreatePet(ctx context.Context, req CreatePetRequest) (Pet, error) {
	return Pet{}, nil
}

// Health returns the health status.
func Health(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Create config file with OpenAPI enabled
	configContent := `[db]
dialects = mysql

[api]
package = ./api
openapi = true
openapi_output = openapi.json
openapi_title = Test Pet API
openapi_version = 1.0.0
openapi_description = A test API for pets
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// Build the generator
	generatorPath := filepath.Join(tmpDir, "generator")
	buildCmd := exec.Command("go", "build", "-o", generatorPath, "./cmd/portsql-api-httpgen")
	buildCmd.Dir = modDir
	output, err = buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build generator: %s", string(output))
	}

	// Run the generator
	genCmd := exec.Command(generatorPath)
	genCmd.Dir = tmpDir
	genCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err = genCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generator failed: %s", string(output))
	}

	// Check that zz_generated_http.go was created
	generatedHTTP := filepath.Join(apiDir, "zz_generated_http.go")
	if _, err := os.Stat(generatedHTTP); os.IsNotExist(err) {
		t.Fatal("zz_generated_http.go was not created")
	}

	// Check that openapi.json was created
	openapiPath := filepath.Join(apiDir, "openapi.json")
	openapiBytes, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("failed to read openapi.json: %v", err)
	}

	// Parse and validate the OpenAPI document
	var doc map[string]any
	if err := json.Unmarshal(openapiBytes, &doc); err != nil {
		t.Fatalf("failed to parse openapi.json: %v", err)
	}

	// Check basic structure
	if doc["openapi"] != "3.0.3" {
		t.Errorf("expected openapi='3.0.3', got %v", doc["openapi"])
	}

	info := doc["info"].(map[string]any)
	if info["title"] != "Test Pet API" {
		t.Errorf("expected title='Test Pet API', got %v", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("expected version='1.0.0', got %v", info["version"])
	}
	if info["description"] != "A test API for pets" {
		t.Errorf("expected description='A test API for pets', got %v", info["description"])
	}

	// Check paths exist
	paths := doc["paths"].(map[string]any)
	if _, ok := paths["/pets/{id}"]; !ok {
		t.Error("expected /pets/{id} path")
	}
	if _, ok := paths["/pets"]; !ok {
		t.Error("expected /pets path")
	}
	if _, ok := paths["/health"]; !ok {
		t.Error("expected /health path")
	}

	// Check operations have docstrings
	petsPath := paths["/pets/{id}"].(map[string]any)
	getOp := petsPath["get"].(map[string]any)
	if summary, ok := getOp["summary"].(string); !ok || !strings.Contains(summary, "retrieves") {
		t.Errorf("expected GetPet summary to contain 'retrieves', got %v", getOp["summary"])
	}

	// Check components schemas exist
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	if _, ok := schemas["ErrorResponse"]; !ok {
		t.Error("expected ErrorResponse schema")
	}

	// Check Pet schema has description from docstring
	var petSchema map[string]any
	for k, v := range schemas {
		if strings.Contains(k, "Pet") && !strings.Contains(k, "Request") && !strings.Contains(k, "Error") {
			petSchema = v.(map[string]any)
			break
		}
	}
	if petSchema == nil {
		t.Fatal("expected Pet schema")
	}
	if desc, ok := petSchema["description"].(string); !ok || !strings.Contains(desc, "represents a pet") {
		t.Errorf("expected Pet description to contain 'represents a pet', got %v", petSchema["description"])
	}
}

func TestOpenAPI_NotGeneratedWhenDisabled(t *testing.T) {
	modDir := getModuleDir(t)
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

	// Create simple handlers.go
	handlersCode := `package api

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

	// Create config file WITHOUT OpenAPI enabled
	configContent := `[db]
dialects = mysql

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// Build the generator
	generatorPath := filepath.Join(tmpDir, "generator")
	buildCmd := exec.Command("go", "build", "-o", generatorPath, "./cmd/portsql-api-httpgen")
	buildCmd.Dir = modDir
	output, err = buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build generator: %s", string(output))
	}

	// Run the generator
	genCmd := exec.Command(generatorPath)
	genCmd.Dir = tmpDir
	genCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err = genCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generator failed: %s", string(output))
	}

	// Check that zz_generated_http.go was created
	generatedHTTP := filepath.Join(apiDir, "zz_generated_http.go")
	if _, err := os.Stat(generatedHTTP); os.IsNotExist(err) {
		t.Fatal("zz_generated_http.go was not created")
	}

	// Check that openapi.json was NOT created
	openapiPath := filepath.Join(apiDir, "openapi.json")
	if _, err := os.Stat(openapiPath); !os.IsNotExist(err) {
		t.Error("openapi.json should not be created when openapi is disabled")
	}
}

func TestOpenAPI_DeterministicOutput(t *testing.T) {
	// Create a deterministic manifest for testing
	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets",
				HandlerPkg:  "example.com/test/api",
				HandlerName: "ListPets",
				Shape:       "ctx_resp_err",
				RespType:    "[]api.Pet",
			},
			{
				Method:      "POST",
				Path:        "/pets",
				HandlerPkg:  "example.com/test/api",
				HandlerName: "CreatePet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "api.CreatePetRequest",
				RespType:    "api.Pet",
			},
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/test/api",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "api.GetPetRequest",
				RespType:    "api.Pet",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		},
	}

	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	bytes1, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("first BuildOpenAPI failed: %v", err)
	}

	bytes2, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("second BuildOpenAPI failed: %v", err)
	}

	if string(bytes1) != string(bytes2) {
		t.Error("OpenAPI output is not deterministic across runs")
	}
}
