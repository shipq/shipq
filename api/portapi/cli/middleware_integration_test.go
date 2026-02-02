//go:build integration

package cli

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMiddleware_GeneratesMiddlewareAwarePipeline verifies that the generator
// produces middleware-aware code that compiles successfully.
func TestMiddleware_GeneratesMiddlewareAwarePipeline(t *testing.T) {
	modDir := getModuleDir(t)
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

	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// Create api package directory
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create middleware directory
	mwDir := filepath.Join(apiDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		t.Fatalf("failed to create middleware dir: %v", err)
	}

	// Create middleware package
	middlewareCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(Logger)
	reg.Use(Auth)
}

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}

func Auth(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	authHeader, ok := req.HeaderValue("Authorization")
	if !ok || authHeader == "" {
		return portapi.HandlerResult{}, portapi.HTTPError{
			Status: 401,
			Code:   "unauthorized",
			Msg:    "missing authorization header",
		}
	}
	return next(ctx)
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "middleware.go"), []byte(middlewareCode), 0644); err != nil {
		t.Fatalf("failed to write middleware.go: %v", err)
	}

	// Create handlers
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
	"example.com/testapp/api/middleware"
)

type Pet struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type GetPetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.Logger)

		g.Group(func(protected *portapi.Group) {
			protected.Use(middleware.Auth)
			protected.Get("/pets/{id}", GetPet)
		})
	})
}

func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	return Pet{ID: req.ID, Name: "Fluffy"}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Create config
	iniContent := `[db]
dialects = mysql

[api]
package = example.com/testapp/api
middleware_package = example.com/testapp/api/middleware
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini file: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}

	// Run generator
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

	// Read generated code
	generatedCode, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	codeStr := string(generatedCode)

	// Verify it contains middleware-related code
	expectedStrings := []string{
		"portapi.Request",
		"portapi.Next",
		"portapi.Middleware",
		"portapi.HandlerResult",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("expected generated code to contain %q", expected)
		}
	}

	// Verify it imports the middleware package
	if !strings.Contains(codeStr, "example.com/testapp/api/middleware") {
		t.Error("expected generated code to import middleware package")
	}

	// Verify the code parses as valid Go
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, generatedPath, generatedCode, 0); err != nil {
		t.Fatalf("generated code must be valid Go: %v", err)
	}

	// Verify the code compiles
	buildCmd := exec.Command("go", "build", "./api")
	buildCmd.Dir = tmpDir
	output, err = buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %s\nGenerated code:\n%s", output, codeStr)
	}
}

// TestMiddleware_GeneratedCodeStructure verifies the structure of middleware-aware generated code.
func TestMiddleware_GeneratedCodeStructure(t *testing.T) {
	// Create a minimal manifest with middleware
	manifest := Manifest{
		Middlewares: []ManifestMiddleware{
			{Pkg: "example.com/test/middleware", Name: "Logger"},
			{Pkg: "example.com/test/middleware", Name: "Auth"},
		},
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/test/handlers",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "handlers.GetPetRequest",
				RespType:    "handlers.Pet",
				Middlewares: []ManifestMiddleware{
					{Pkg: "example.com/test/middleware", Name: "Logger"},
					{Pkg: "example.com/test/middleware", Name: "Auth"},
				},
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{
							FieldName: "ID",
							TagValue:  "id",
							TypeKind:  "string",
						},
					},
				},
			},
		},
	}

	// Generate code
	code, err := Generate(manifest, "api", "example.com/test/handlers")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify structure
	expectedPatterns := []string{
		// Should have Request type usage
		"*portapi.Request",

		// Should have middleware chain construction
		"[]portapi.Middleware",

		// Should have decoded request storage
		"var zzDecoded",
		"var zzDecodedOK",

		// Should have DecodedReq closure
		"DecodedReq:",

		// Should handle errors properly (checks for coded error interface)
		"StatusCode()",
		"ErrorCode()",

		// Should handle HandlerResult
		"portapi.HandlerResult",

		// Should have chain builder
		"zzChain",

		// Should have result writer
		"writeResult",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(code, pattern) {
			t.Errorf("expected generated code to contain %q", pattern)
		}
	}

	// Verify the code parses
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "test.go", code, 0); err != nil {
		t.Fatalf("generated code must be valid Go: %v\nCode:\n%s", err, code)
	}
}

// TestMiddleware_DeterministicOutput verifies that middleware-aware generation is deterministic.
func TestMiddleware_DeterministicOutput(t *testing.T) {
	manifest := Manifest{
		Middlewares: []ManifestMiddleware{
			{Pkg: "example.com/test/mw", Name: "B"},
			{Pkg: "example.com/test/mw", Name: "A"},
		},
		Endpoints: []ManifestEndpoint{
			{
				Method:      "POST",
				Path:        "/items",
				HandlerPkg:  "example.com/test/h",
				HandlerName: "Create",
				Shape:       "ctx_req_resp_err",
				ReqType:     "h.CreateRequest",
				RespType:    "h.Item",
				Middlewares: []ManifestMiddleware{
					{Pkg: "example.com/test/mw", Name: "B"},
					{Pkg: "example.com/test/mw", Name: "A"},
				},
			},
			{
				Method:      "GET",
				Path:        "/items/{id}",
				HandlerPkg:  "example.com/test/h",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "h.GetRequest",
				RespType:    "h.Item",
				Middlewares: []ManifestMiddleware{
					{Pkg: "example.com/test/mw", Name: "A"},
				},
			},
		},
	}

	// Generate twice
	code1, err := Generate(manifest, "api", "example.com/test/h")
	if err != nil {
		t.Fatalf("first Generate failed: %v", err)
	}

	code2, err := Generate(manifest, "api", "example.com/test/h")
	if err != nil {
		t.Fatalf("second Generate failed: %v", err)
	}

	// Should be identical
	if code1 != code2 {
		t.Error("generated code should be deterministic")
		t.Logf("First:\n%s\n\nSecond:\n%s", code1, code2)
	}
}
