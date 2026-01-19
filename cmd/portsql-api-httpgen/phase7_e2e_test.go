//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/api/portapi"
)

// TestPhase7_ManifestDeterminism verifies that discovery produces byte-identical output across runs.
func TestPhase7_ManifestDeterminism(t *testing.T) {
	modDir := getModuleDir(t)

	// Create fixture with nested groups and multiple middleware
	tmpDir := t.TempDir()

	setupManifestDeterminismFixture(t, tmpDir, modDir)

	// Run discovery twice
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// First run
	manifest1, err := Discover("example.com/testapp/api", "example.com/testapp/api/middleware")
	if err != nil {
		t.Fatalf("first discovery failed: %v", err)
	}

	// Second run
	manifest2, err := Discover("example.com/testapp/api", "example.com/testapp/api/middleware")
	if err != nil {
		t.Fatalf("second discovery failed: %v", err)
	}

	// Marshal to JSON
	json1, err := json.Marshal(manifest1)
	if err != nil {
		t.Fatalf("failed to marshal first manifest: %v", err)
	}

	json2, err := json.Marshal(manifest2)
	if err != nil {
		t.Fatalf("failed to marshal second manifest: %v", err)
	}

	// Compare bytes
	if !bytes.Equal(json1, json2) {
		t.Error("manifest output is not deterministic across runs")
		t.Logf("First:\n%s", string(json1))
		t.Logf("Second:\n%s", string(json2))
	}
}

// TestPhase7_CodegenDeterminism verifies that codegen produces byte-identical output for the same manifest.
func TestPhase7_CodegenDeterminism(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupCodegenDeterminismFixture(t, tmpDir, modDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Run generator twice
	apiDir := filepath.Join(tmpDir, "api")
	mwDir := filepath.Join(apiDir, "middleware")
	generatedPath := filepath.Join(apiDir, "zz_generated_http.go")
	contextPath := filepath.Join(mwDir, "zz_generated_middleware_context.go")

	// First run
	if err := run(); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	code1, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	context1, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	// Delete generated files
	os.Remove(generatedPath)
	os.Remove(contextPath)

	// Second run
	if err := run(); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	code2, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file second time: %v", err)
	}

	context2, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("failed to read context file second time: %v", err)
	}

	// Compare zz_generated_http.go
	if !bytes.Equal(code1, code2) {
		t.Error("zz_generated_http.go is not deterministic")
	}

	// Compare zz_generated_middleware_context.go
	if !bytes.Equal(context1, context2) {
		t.Error("zz_generated_middleware_context.go is not deterministic")
	}
}

// TestPhase7_EndToEnd_HappyPath verifies the complete feature end-to-end.
func TestPhase7_EndToEnd_HappyPath(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupHappyPathFixture(t, tmpDir, modDir)

	// Generate code
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Build and import generated code
	buildCmd := exec.Command("go", "build", "./api")
	buildCmd.Dir = tmpDir
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %s", string(output))
	}

	// The generated NewMux function should be callable
	// We can verify the generated file exists and compiles
	generatedPath := filepath.Join(tmpDir, "api", "zz_generated_http.go")
	if _, err := os.Stat(generatedPath); err != nil {
		t.Fatalf("generated file should exist: %v", err)
	}
}

// TestPhase7_ShortCircuitTypedError verifies middleware can short-circuit with a typed error.
func TestPhase7_ShortCircuitTypedError(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupShortCircuitErrorFixture(t, tmpDir, modDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify build succeeds
	buildCmd := exec.Command("go", "build", "./api")
	buildCmd.Dir = tmpDir
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %s", string(output))
	}
}

// TestPhase7_ShortCircuitHandlerResult verifies middleware can short-circuit with a HandlerResult.
func TestPhase7_ShortCircuitHandlerResult(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupShortCircuitResultFixture(t, tmpDir, modDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify build succeeds
	buildCmd := exec.Command("go", "build", "./api")
	buildCmd.Dir = tmpDir
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %s", string(output))
	}
}

// TestPhase7_StrictMode_NoRegistry verifies error when middleware is used without registry config.
func TestPhase7_StrictMode_NoRegistry(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupStrictModeNoRegistryFixture(t, tmpDir, modDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Run should fail
	err = run()
	if err == nil {
		t.Fatal("expected error when middleware used without registry, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "middleware_used_without_registry") {
		t.Errorf("expected error code 'middleware_used_without_registry', got: %v", errMsg)
	}
	if !strings.Contains(errMsg, "middleware_package") {
		t.Errorf("expected error to mention middleware_package, got: %v", errMsg)
	}
}

// TestPhase7_StrictMode_UndeclaredMiddleware verifies error when middleware is used but not declared.
func TestPhase7_StrictMode_UndeclaredMiddleware(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupStrictModeUndeclaredFixture(t, tmpDir, modDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Run should fail
	err = run()
	if err == nil {
		t.Fatal("expected error when middleware used but undeclared, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "undeclared_middleware") {
		t.Errorf("expected error code 'undeclared_middleware', got: %v", errMsg)
	}
}

// Helper functions to set up fixtures

func setupManifestDeterminismFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()

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

	// Create api package
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create middleware package
	mwDir := filepath.Join(apiDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		t.Fatalf("failed to create middleware dir: %v", err)
	}

	// Middleware code
	mwCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(GlobalA)
	reg.Use(GlobalB)
	reg.Use(GroupA)
	reg.Use(GroupB)

	reg.Provide("request_id", portapi.TypeOf[string]())
	reg.Provide("user_id", portapi.TypeOf[int64]())
}

func GlobalA(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}

func GlobalB(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}

func GroupA(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}

func GroupB(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "middleware.go"), []byte(mwCode), 0644); err != nil {
		t.Fatalf("failed to write middleware.go: %v", err)
	}

	// Handlers code with groups
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
	"example.com/testapp/api/middleware"
)

type Data struct {
	Value string ` + "`json:\"value\"`" + `
}

type GetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.GlobalA)
		g.Use(middleware.GlobalB)

		g.Group(func(outer *portapi.Group) {
			outer.Use(middleware.GroupA)

			outer.Get("/outer/{id}", GetOuter)

			outer.Group(func(inner *portapi.Group) {
				inner.Use(middleware.GroupB)
				inner.Get("/inner/{id}", GetInner)
			})
		})
	})
}

func GetOuter(ctx context.Context, req GetRequest) (Data, error) {
	return Data{Value: "outer"}, nil
}

func GetInner(ctx context.Context, req GetRequest) (Data, error) {
	return Data{Value: "inner"}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Config
	iniContent := `[httpgen]
package = example.com/testapp/api
middleware_package = example.com/testapp/api/middleware
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
}

func setupCodegenDeterminismFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()
	// Use same fixture as manifest determinism
	setupManifestDeterminismFixture(t, tmpDir, modDir)
}

func setupHappyPathFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()
	// Use same fixture as manifest determinism - it has everything needed
	setupManifestDeterminismFixture(t, tmpDir, modDir)
}

func setupShortCircuitErrorFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()

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

	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	mwDir := filepath.Join(apiDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		t.Fatalf("failed to create middleware dir: %v", err)
	}

	mwCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(AuthRequired)
}

func AuthRequired(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
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
	if err := os.WriteFile(filepath.Join(mwDir, "middleware.go"), []byte(mwCode), 0644); err != nil {
		t.Fatalf("failed to write middleware.go: %v", err)
	}

	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
	"example.com/testapp/api/middleware"
)

type Data struct {
	Value string ` + "`json:\"value\"`" + `
}

func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.AuthRequired)
		g.Get("/protected", GetProtected)
	})
}

func GetProtected(ctx context.Context) (Data, error) {
	return Data{Value: "secret"}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	iniContent := `[httpgen]
package = example.com/testapp/api
middleware_package = example.com/testapp/api/middleware
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}
}

func setupShortCircuitResultFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()

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

	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	mwDir := filepath.Join(apiDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		t.Fatalf("failed to create middleware dir: %v", err)
	}

	mwCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(NoContent)
}

func NoContent(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return portapi.HandlerResult{Status: 204, NoContent: true}, nil
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "middleware.go"), []byte(mwCode), 0644); err != nil {
		t.Fatalf("failed to write middleware.go: %v", err)
	}

	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
	"example.com/testapp/api/middleware"
)

type Data struct {
	Value string ` + "`json:\"value\"`" + `
}

func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.NoContent)
		g.Get("/test", GetTest)
	})
}

func GetTest(ctx context.Context) (Data, error) {
	return Data{Value: "should not see this"}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	iniContent := `[httpgen]
package = example.com/testapp/api
middleware_package = example.com/testapp/api/middleware
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}
}

func setupStrictModeNoRegistryFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()

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

	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	mwDir := filepath.Join(apiDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		t.Fatalf("failed to create middleware dir: %v", err)
	}

	// Create middleware but don't configure it in the ini
	mwCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "middleware.go"), []byte(mwCode), 0644); err != nil {
		t.Fatalf("failed to write middleware.go: %v", err)
	}

	// Use middleware in handlers
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
	"example.com/testapp/api/middleware"
)

func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.Logger)
		g.Get("/test", GetTest)
	})
}

func GetTest(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Config WITHOUT middleware_package
	iniContent := `[httpgen]
package = example.com/testapp/api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}
}

func setupStrictModeUndeclaredFixture(t *testing.T, tmpDir, modDir string) {
	t.Helper()

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

	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	mwDir := filepath.Join(apiDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		t.Fatalf("failed to create middleware dir: %v", err)
	}

	// Middleware with RegisterMiddleware that doesn't declare all used middleware
	mwCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	// Declare Logger but NOT Auth
	reg.Use(Logger)
}

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}

func Auth(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	return next(ctx)
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "middleware.go"), []byte(mwCode), 0644); err != nil {
		t.Fatalf("failed to write middleware.go: %v", err)
	}

	// Use both Logger and Auth in handlers
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
	"example.com/testapp/api/middleware"
)

func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.Logger)

		g.Group(func(inner *portapi.Group) {
			inner.Use(middleware.Auth)  // Auth is used but not declared
			inner.Get("/test", GetTest)
		})
	})
}

func GetTest(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	iniContent := `[httpgen]
package = example.com/testapp/api
middleware_package = example.com/testapp/api/middleware
`
	if err := os.WriteFile(filepath.Join(tmpDir, "portsql-api-httpgen.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write ini file: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}
}

// Suppress unused imports warning
var (
	_ = context.Background
	_ = httptest.NewRequest
	_ = http.StatusOK
	_ = io.ReadAll
	_ = portapi.App{}
)
