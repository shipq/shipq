//go:build integration

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMiddlewareContext_GeneratesHelpers verifies that the generator produces
// typed context helpers in the middleware package that compile and work correctly.
func TestMiddlewareContext_GeneratesHelpers(t *testing.T) {
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

	// Create middleware package with Provide() calls
	middlewareCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

type User struct {
	ID   string
	Name string
}

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(Logger)

	// Provide context keys
	if err := reg.Provide("current_user", portapi.TypeOf[*User]()); err != nil {
		panic(err)
	}
	if err := reg.Provide("request_id", portapi.TypeOf[string]()); err != nil {
		panic(err)
	}
	if err := reg.Provide("retry_count", portapi.TypeOf[int]()); err != nil {
		panic(err)
	}
}

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
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

	// Verify generated context file exists
	contextPath := filepath.Join(mwDir, "zz_generated_middleware_context.go")
	if _, err := os.Stat(contextPath); err != nil {
		t.Fatalf("generated context file should exist: %v", err)
	}

	// Read generated code
	generatedCode, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	codeStr := string(generatedCode)

	// Verify it contains expected helper functions
	expectedFunctions := []string{
		"func WithCurrentUser(ctx context.Context, v *User) context.Context",
		"func CurrentUser(ctx context.Context) (*User, bool)",
		"func MustCurrentUser(ctx context.Context) *User",
		"func WithRequestID(ctx context.Context, v string) context.Context",
		"func RequestID(ctx context.Context) (string, bool)",
		"func MustRequestID(ctx context.Context) string",
		"func WithRetryCount(ctx context.Context, v int) context.Context",
		"func RetryCount(ctx context.Context) (int, bool)",
		"func MustRetryCount(ctx context.Context) int",
	}

	for _, expected := range expectedFunctions {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("expected generated code to contain %q", expected)
		}
	}

	// Verify it uses the new portapi store pattern (not the old zzCtxKey types)
	if !strings.Contains(codeStr, "portapi.WithTyped") {
		t.Error("expected generated code to use portapi.WithTyped")
	}
	if !strings.Contains(codeStr, "portapi.GetTyped") {
		t.Error("expected generated code to use portapi.GetTyped")
	}

	// Verify it does NOT contain old-style context key types
	if strings.Contains(codeStr, "type zzCtxKey") {
		t.Error("should not generate per-key context types anymore")
	}

	// Verify it uses the exact key strings for portapi calls
	expectedKeys := []string{
		`"current_user"`,
		`"request_id"`,
		`"retry_count"`,
	}

	for _, expected := range expectedKeys {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("expected generated code to contain key string %s", expected)
		}
	}

	// Verify the code compiles
	buildCmd := exec.Command("go", "build", "./api/middleware")
	buildCmd.Dir = tmpDir
	output, err = buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %s\nGenerated code:\n%s", output, codeStr)
	}
}

// TestMiddlewareContext_RuntimeBehavior tests that the generated helpers work correctly at runtime.
func TestMiddlewareContext_RuntimeBehavior(t *testing.T) {
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

	// Create middleware package with Provide() calls
	middlewareCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

type User struct {
	ID   string
	Name string
}

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(Logger)

	if err := reg.Provide("current_user", portapi.TypeOf[*User]()); err != nil {
		panic(err)
	}
	if err := reg.Provide("request_id", portapi.TypeOf[string]()); err != nil {
		panic(err)
	}
}

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
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
)

type Pet struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func Register(app *portapi.App) {
	app.Get("/pets", ListPets)
}

func ListPets(ctx context.Context) ([]Pet, error) {
	return []Pet{{ID: "1", Name: "Fluffy"}}, nil
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

	// Create a test file that exercises the generated helpers
	testCode := `package middleware

import (
	"context"
	"testing"
)

func TestContextHelpers_RoundTrip(t *testing.T) {
	ctx := context.Background()

	// Test CurrentUser (pointer type)
	user := &User{ID: "123", Name: "Alice"}
	ctx = WithCurrentUser(ctx, user)

	retrieved, ok := CurrentUser(ctx)
	if !ok {
		t.Fatal("expected CurrentUser to be present")
	}
	if retrieved != user {
		t.Errorf("expected %v, got %v", user, retrieved)
	}

	mustUser := MustCurrentUser(ctx)
	if mustUser != user {
		t.Errorf("MustCurrentUser: expected %v, got %v", user, mustUser)
	}

	// Test RequestID (string type)
	ctx = WithRequestID(ctx, "req-456")

	reqID, ok := RequestID(ctx)
	if !ok {
		t.Fatal("expected RequestID to be present")
	}
	if reqID != "req-456" {
		t.Errorf("expected req-456, got %s", reqID)
	}

	mustReqID := MustRequestID(ctx)
	if mustReqID != "req-456" {
		t.Errorf("MustRequestID: expected req-456, got %s", mustReqID)
	}
}

func TestContextHelpers_Missing(t *testing.T) {
	ctx := context.Background()

	// Test missing value returns false
	_, ok := CurrentUser(ctx)
	if ok {
		t.Error("expected CurrentUser to be absent")
	}

	_, ok = RequestID(ctx)
	if ok {
		t.Error("expected RequestID to be absent")
	}
}

func TestContextHelpers_MustPanics(t *testing.T) {
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustCurrentUser to panic when value is missing")
		}
	}()

	MustCurrentUser(ctx)
}

func TestContextHelpers_Independence(t *testing.T) {
	ctx := context.Background()

	// Set both values
	user := &User{ID: "123", Name: "Alice"}
	ctx = WithCurrentUser(ctx, user)
	ctx = WithRequestID(ctx, "req-456")

	// Verify both can be retrieved independently
	retrievedUser, ok := CurrentUser(ctx)
	if !ok || retrievedUser != user {
		t.Error("CurrentUser retrieval failed")
	}

	retrievedID, ok := RequestID(ctx)
	if !ok || retrievedID != "req-456" {
		t.Error("RequestID retrieval failed")
	}
}
`
	if err := os.WriteFile(filepath.Join(mwDir, "context_test.go"), []byte(testCode), 0644); err != nil {
		t.Fatalf("failed to write context_test.go: %v", err)
	}

	// Run the tests
	testCmd := exec.Command("go", "test", "./api/middleware", "-v")
	testCmd.Dir = tmpDir
	output, err = testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %s", string(output))
	}

	// Verify all tests passed
	outputStr := string(output)
	if !strings.Contains(outputStr, "PASS") {
		t.Errorf("expected tests to pass, output:\n%s", outputStr)
	}
}

// TestMiddlewareContext_NoContextKeys verifies that no file is generated when there are no Provide() calls.
func TestMiddlewareContext_NoContextKeys(t *testing.T) {
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

	// Create middleware package WITHOUT Provide() calls
	middlewareCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(Logger)
}

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
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
)

type Pet struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func Register(app *portapi.App) {
	app.Get("/pets", ListPets)
}

func ListPets(ctx context.Context) ([]Pet, error) {
	return []Pet{{ID: "1", Name: "Fluffy"}}, nil
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

	// Verify generated context file does NOT exist
	contextPath := filepath.Join(mwDir, "zz_generated_middleware_context.go")
	if _, err := os.Stat(contextPath); err == nil {
		t.Error("context file should not be generated when there are no Provide() calls")
	}
}

// TestMiddlewareContext_DeterministicOutput verifies that generation is deterministic.
func TestMiddlewareContext_DeterministicOutput(t *testing.T) {
	modDir := getModuleDir(t)

	var outputs []string

	// Generate twice
	for i := 0; i < 2; i++ {
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

		// Create middleware package with Provide() calls in non-alphabetical order
		middlewareCode := `package middleware

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

type User struct {
	ID string
}

func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(Logger)

	// Declare in non-alphabetical order
	if err := reg.Provide("zebra", portapi.TypeOf[string]()); err != nil {
		panic(err)
	}
	if err := reg.Provide("apple", portapi.TypeOf[int]()); err != nil {
		panic(err)
	}
	if err := reg.Provide("mango", portapi.TypeOf[*User]()); err != nil {
		panic(err)
	}
}

func Logger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
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
)

type Pet struct {
	ID string ` + "`json:\"id\"`" + `
}

func Register(app *portapi.App) {
	app.Get("/pets", ListPets)
}

func ListPets(ctx context.Context) ([]Pet, error) {
	return []Pet{{ID: "1"}}, nil
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

		// Read generated code
		contextPath := filepath.Join(mwDir, "zz_generated_middleware_context.go")
		generatedCode, err := os.ReadFile(contextPath)
		if err != nil {
			t.Fatalf("failed to read generated file: %v", err)
		}

		outputs = append(outputs, string(generatedCode))
	}

	// Verify both outputs are identical
	if outputs[0] != outputs[1] {
		t.Error("generated code should be deterministic across runs")
		t.Logf("First output:\n%s\n\nSecond output:\n%s", outputs[0], outputs[1])
	}
}
