//go:build integration

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
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

	manifest, err := Discover("example.com/testapp/api", "")
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
	_, err = Discover("example.com/testapp/api", "")
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

	manifest, err := Discover("example.com/testapp/api", "")
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

// =============================================================================
// OpenAPI Step 2 Tests: Type Graph and Docstrings
// =============================================================================

// Test Group A: Manifest schema extensions exist and parse

func TestDiscover_ManifestHasTypesAndEndpointDocs(t *testing.T) {
	// Test A1: Manifest JSON includes Types and EndpointDocs (empty when none)
	// Use existing testdata that has endpoints without special doc comments
	modDir := getModuleDir(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(modDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	manifest, err := Discover("github.com/shipq/shipq/api/portapi/cli/testdata/apiroot_mw_happy_path", "github.com/shipq/shipq/api/portapi/cli/testdata/apiroot_mw_happy_path/middleware")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Types should be non-nil (may be empty slice or populated)
	if manifest.Types == nil {
		t.Error("manifest.Types should be non-nil")
	}

	// EndpointDocs should be non-nil map
	if manifest.EndpointDocs == nil {
		t.Error("manifest.EndpointDocs should be non-nil")
	}

	// Should have endpoints
	if len(manifest.Endpoints) == 0 {
		t.Error("expected at least one endpoint")
	}
}

// Test Group B: Type graph generation (happy path)

func TestDiscover_TypeGraphHappyPath(t *testing.T) {
	modDir := getModuleDir(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(modDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	manifest, err := Discover("github.com/shipq/shipq/api/portapi/cli/testdata/openapi_types_happy", "")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// B1: Type graph includes named types referenced by req/resp
	t.Run("includes_named_types", func(t *testing.T) {
		if len(manifest.Types) == 0 {
			t.Fatal("expected types in manifest")
		}

		// Build a map for easier lookup
		typesByID := make(map[string]ManifestType)
		for _, mt := range manifest.Types {
			typesByID[mt.ID] = mt
		}

		// Should have GetThingResp
		var foundResp bool
		for id := range typesByID {
			if strings.HasSuffix(id, "GetThingResp") {
				foundResp = true
				break
			}
		}
		if !foundResp {
			t.Error("expected type graph to include GetThingResp")
		}

		// Should have nested Address type
		var foundAddress bool
		for id := range typesByID {
			if strings.HasSuffix(id, "Address") {
				foundAddress = true
				break
			}
		}
		if !foundAddress {
			t.Error("expected type graph to include nested Address type")
		}

		// Should have Tag type (used in []Tag)
		var foundTag bool
		for id := range typesByID {
			if strings.HasSuffix(id, "Tag") {
				foundTag = true
				break
			}
		}
		if !foundTag {
			t.Error("expected type graph to include Tag type")
		}
	})

	t.Run("struct_fields_have_correct_json_names", func(t *testing.T) {
		// Find GetThingResp type (must be a struct, not a slice of it)
		var respType *ManifestType
		for i := range manifest.Types {
			if strings.HasSuffix(manifest.Types[i].ID, "GetThingResp") && manifest.Types[i].Kind == "struct" {
				respType = &manifest.Types[i]
				break
			}
		}
		if respType == nil {
			t.Fatal("GetThingResp type not found")
		}

		if respType.Kind != "struct" {
			t.Errorf("expected kind 'struct', got %q", respType.Kind)
		}

		// Check fields
		fieldsByJSON := make(map[string]ManifestField)
		for _, f := range respType.Fields {
			if f.JSONName != "" {
				fieldsByJSON[f.JSONName] = f
			}
		}

		// Check required field "id"
		if f, ok := fieldsByJSON["id"]; !ok {
			t.Error("expected 'id' field")
		} else if !f.Required {
			t.Error("'id' field should be required (no omitempty, not pointer)")
		}

		// Check optional field "description" (pointer with omitempty)
		if f, ok := fieldsByJSON["description"]; !ok {
			t.Error("expected 'description' field")
		} else if f.Required {
			t.Error("'description' field should not be required (pointer with omitempty)")
		}

		// Check "created_at" field exists
		if _, ok := fieldsByJSON["created_at"]; !ok {
			t.Error("expected 'created_at' field")
		}
	})

	// B2: time.Time maps to a known type
	t.Run("time_type_recognized", func(t *testing.T) {
		var foundTimeType bool
		for _, mt := range manifest.Types {
			if mt.Kind == "time" || strings.Contains(mt.ID, "time.Time") {
				foundTimeType = true
				break
			}
		}
		if !foundTimeType {
			t.Error("expected time.Time to be recognized as a special type")
		}
	})

	// B3: Slices and maps are represented correctly
	t.Run("slice_type_representation", func(t *testing.T) {
		var foundSlice bool
		for _, mt := range manifest.Types {
			if mt.Kind == "slice" && mt.Elem != "" {
				foundSlice = true
				break
			}
		}
		if !foundSlice {
			t.Error("expected at least one slice type with elem reference")
		}
	})

	t.Run("map_type_representation", func(t *testing.T) {
		var foundMap bool
		for _, mt := range manifest.Types {
			if mt.Kind == "map" && mt.Key != "" && mt.Value != "" {
				foundMap = true
				break
			}
		}
		if !foundMap {
			t.Error("expected at least one map type with key/value references")
		}
	})
}

// Test Group C: Docstring extraction (happy path)

func TestDiscover_DocstringExtraction(t *testing.T) {
	modDir := getModuleDir(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(modDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	manifest, err := Discover("github.com/shipq/shipq/api/portapi/cli/testdata/openapi_docs_happy", "")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// C1: Endpoint docstrings exist
	t.Run("endpoint_docstrings", func(t *testing.T) {
		if len(manifest.EndpointDocs) == 0 {
			t.Fatal("expected endpoint docs")
		}

		// Find GetPet handler docs
		var foundGetPet bool
		for key, doc := range manifest.EndpointDocs {
			if strings.HasSuffix(key, "GetPet") {
				foundGetPet = true
				if doc.Summary == "" {
					t.Error("GetPet should have a summary")
				}
				if doc.Description == "" {
					t.Error("GetPet should have a description")
				}
				// Summary should be first line
				if !strings.Contains(doc.Summary, "retrieves a pet") {
					t.Errorf("GetPet summary should mention 'retrieves a pet', got %q", doc.Summary)
				}
				break
			}
		}
		if !foundGetPet {
			t.Error("expected GetPet handler docs")
		}
	})

	// C2: Type docstrings attached to type nodes
	t.Run("type_docstrings", func(t *testing.T) {
		var foundPetDoc bool
		for _, mt := range manifest.Types {
			if strings.HasSuffix(mt.ID, "Pet") && mt.Kind == "struct" {
				if mt.Doc != "" {
					foundPetDoc = true
					if !strings.Contains(mt.Doc, "represents a pet") {
						t.Errorf("Pet type doc should mention 'represents a pet', got %q", mt.Doc)
					}
				}
				break
			}
		}
		if !foundPetDoc {
			t.Error("expected Pet type to have doc comment")
		}
	})

	// C3: Field docstrings attached to field entries
	t.Run("field_docstrings", func(t *testing.T) {
		var foundFieldDoc bool
		for _, mt := range manifest.Types {
			if strings.HasSuffix(mt.ID, "Pet") && mt.Kind == "struct" {
				for _, f := range mt.Fields {
					if f.JSONName == "id" && f.Doc != "" {
						foundFieldDoc = true
						if !strings.Contains(f.Doc, "unique identifier") {
							t.Errorf("Pet.ID field doc should mention 'unique identifier', got %q", f.Doc)
						}
						break
					}
				}
				break
			}
		}
		if !foundFieldDoc {
			t.Error("expected Pet.ID field to have doc comment")
		}
	})
}

// Test Group D: Validations for unsupported declarations

func TestDiscover_InvalidBindings(t *testing.T) {
	modDir := getModuleDir(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(modDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// D1: Discovery fails with clear error for conflicting bindings
	_, err = Discover("github.com/shipq/shipq/api/portapi/cli/testdata/openapi_invalid_bindings", "")
	if err == nil {
		t.Fatal("expected error for conflicting bindings, got nil")
	}

	// Error should mention the field or binding conflict
	errStr := err.Error()
	if !strings.Contains(errStr, "multiple binding") && !strings.Contains(errStr, "conflicting") && !strings.Contains(errStr, "path") {
		t.Errorf("error should mention binding conflict, got %q", errStr)
	}
}

func TestDiscover_NonStringMapKeys(t *testing.T) {
	modDir := getModuleDir(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(modDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// D2: Non-string map keys are flagged with warning (not failure)
	manifest, err := Discover("github.com/shipq/shipq/api/portapi/cli/testdata/openapi_map_key_invalid", "")
	if err != nil {
		t.Fatalf("Discover should not fail for non-string map keys, got: %v", err)
	}

	// Check that there's a type with warnings about non-string keys
	var foundWarning bool
	for _, mt := range manifest.Types {
		if mt.Kind == "map" && len(mt.Warnings) > 0 {
			for _, w := range mt.Warnings {
				if strings.Contains(w, "non-string") || strings.Contains(w, "int") {
					foundWarning = true
					break
				}
			}
		}
		// Also check if it's marked as unknown
		if mt.Kind == "unknown" && len(mt.Warnings) > 0 {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for non-string map keys")
	}
}

// Test Group E: Determinism

func TestDiscover_TypeGraphDeterminism(t *testing.T) {
	modDir := getModuleDir(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(modDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Run discovery twice
	manifest1, err := Discover("github.com/shipq/shipq/api/portapi/cli/testdata/openapi_types_happy", "")
	if err != nil {
		t.Fatalf("first Discover failed: %v", err)
	}

	manifest2, err := Discover("github.com/shipq/shipq/api/portapi/cli/testdata/openapi_types_happy", "")
	if err != nil {
		t.Fatalf("second Discover failed: %v", err)
	}

	// E1: Types should be in the same order
	if len(manifest1.Types) != len(manifest2.Types) {
		t.Fatalf("type counts differ: %d vs %d", len(manifest1.Types), len(manifest2.Types))
	}

	// Extract type IDs and compare order
	ids1 := make([]string, len(manifest1.Types))
	ids2 := make([]string, len(manifest2.Types))
	for i := range manifest1.Types {
		ids1[i] = manifest1.Types[i].ID
		ids2[i] = manifest2.Types[i].ID
	}

	// Check that IDs are sorted (our determinism guarantee)
	if !sort.StringsAreSorted(ids1) {
		t.Error("types in first manifest should be sorted by ID")
	}
	if !sort.StringsAreSorted(ids2) {
		t.Error("types in second manifest should be sorted by ID")
	}

	// Check equality
	if !reflect.DeepEqual(ids1, ids2) {
		t.Errorf("type IDs differ between runs:\nfirst:  %v\nsecond: %v", ids1, ids2)
	}

	// Check field ordering within types is deterministic
	for i := range manifest1.Types {
		t1 := manifest1.Types[i]
		t2 := manifest2.Types[i]
		if len(t1.Fields) != len(t2.Fields) {
			t.Errorf("field counts differ for type %s: %d vs %d", t1.ID, len(t1.Fields), len(t2.Fields))
			continue
		}
		for j := range t1.Fields {
			if t1.Fields[j].GoName != t2.Fields[j].GoName {
				t.Errorf("field order differs for type %s at index %d: %s vs %s",
					t1.ID, j, t1.Fields[j].GoName, t2.Fields[j].GoName)
			}
		}
	}
}
