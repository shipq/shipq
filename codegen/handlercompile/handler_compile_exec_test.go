package handlercompile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/embed"
)

// TestBuildAndRunHandlerCompileProgram_FuncNamePopulated tests that the
// handler compile pipeline correctly populates FuncName via static analysis.
//
// This test creates a minimal project with ONE handler to verify the fix.
// Before the fix, FuncName would be empty because runtime reflection cannot
// capture function names - static analysis of register.go is required.
func TestBuildAndRunHandlerCompileProgram_FuncNamePopulated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp project directory
	tmpDir := t.TempDir()

	modulePath := "com.test-funcname"

	// Create go.mod (no external dependency on shipq — handler is embedded)
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Embed the handler package into shipq/lib/handler (mirrors real project setup)
	if err := embed.EmbedAllPackages(tmpDir, modulePath, embed.EmbedOptions{}); err != nil {
		t.Fatalf("failed to embed packages: %v", err)
	}

	// Create api/simple directory
	apiDir := filepath.Join(tmpDir, "api", "simple")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api/simple directory: %v", err)
	}

	handlerImport := modulePath + "/shipq/lib/handler"

	// Create handler.go with a simple handler
	handlerCode := `package simple

import "context"

// SimpleRequest is the request for the simple handler.
type SimpleRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

// SimpleResponse is the response from the simple handler.
type SimpleResponse struct {
	Message string ` + "`json:\"message\"`" + `
}

// SimpleHandler handles GET /simple
func SimpleHandler(ctx context.Context, req *SimpleRequest) (*SimpleResponse, error) {
	return &SimpleResponse{Message: "Hello, " + req.Name}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handler.go"), []byte(handlerCode), 0644); err != nil {
		t.Fatalf("failed to create handler.go: %v", err)
	}

	// Create register.go that registers the handler
	registerCode := `package simple

import "` + handlerImport + `"

// Register registers the simple handler.
func Register(app *handler.App) {
	app.Get("/simple", SimpleHandler)
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "register.go"), []byte(registerCode), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	// Run go mod tidy to resolve dependencies
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
	}

	// Run the handler compile pipeline
	cfg := HandlerCompileProgramConfig{
		ModulePath: modulePath,
		APIPkgs:    []string{modulePath + "/api/simple"},
	}

	handlers, err := BuildAndRunHandlerCompileProgram(tmpDir, cfg)
	if err != nil {
		t.Fatalf("BuildAndRunHandlerCompileProgram failed: %v", err)
	}

	// Verify we got exactly one handler
	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(handlers))
	}

	h := handlers[0]

	// Verify basic handler info
	if h.Method != "GET" {
		t.Errorf("expected method GET, got %s", h.Method)
	}
	if h.Path != "/simple" {
		t.Errorf("expected path /simple, got %s", h.Path)
	}

	// THE KEY ASSERTION: FuncName must be populated
	// Before the fix, this would be empty because runtime reflection
	// cannot capture function names.
	if h.FuncName != "SimpleHandler" {
		t.Errorf("FUNCNAME BUG: expected FuncName 'SimpleHandler', got %q", h.FuncName)
	}
}

// TestImportPathToRegisterFilePath tests the conversion from import paths to register.go file paths.
// This conversion is critical for static analysis to find and parse register.go files.
func TestImportPathToRegisterFilePath(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		modulePath  string
		importPath  string
		wantPath    string
	}{
		{
			name:        "simple api package",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp/api/users",
			wantPath:    "/project/api/users/register.go",
		},
		{
			name:        "nested api package",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp/api/admin/users",
			wantPath:    "/project/api/admin/users/register.go",
		},
		{
			name:        "github module path",
			projectRoot: "/home/user/myproject",
			modulePath:  "github.com/user/myproject",
			importPath:  "github.com/user/myproject/api/posts",
			wantPath:    "/home/user/myproject/api/posts/register.go",
		},
		{
			name:        "monorepo layout",
			projectRoot: "/monorepo",
			modulePath:  "github.com/company/monorepo",
			importPath:  "github.com/company/monorepo/services/api/handlers/auth",
			wantPath:    "/monorepo/services/api/handlers/auth/register.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importPathToRegisterFilePath(tt.projectRoot, tt.modulePath, tt.importPath)
			if got != tt.wantPath {
				t.Errorf("importPathToRegisterFilePath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

// TestImportPathToRegisterFilePath_EdgeCases tests edge cases for path conversion.
func TestImportPathToRegisterFilePath_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		modulePath  string
		importPath  string
		wantPath    string
	}{
		{
			name:        "import path equals module path (root package)",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp",
			wantPath:    "/project/register.go",
		},
		{
			name:        "single level below module",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp/api",
			wantPath:    "/project/api/register.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importPathToRegisterFilePath(tt.projectRoot, tt.modulePath, tt.importPath)
			if got != tt.wantPath {
				t.Errorf("importPathToRegisterFilePath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

// Note: importPathToRegisterFilePath is now in handler_compile_exec.go

// TestParseAllRegisterFiles_SetsPackagePath tests that parseAllRegisterFiles
// correctly associates each RegisterCall with its import path (PackagePath).
func TestParseAllRegisterFiles_SetsPackagePath(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "com.test"

	// Create api/users directory and register.go
	usersDir := filepath.Join(tmpDir, "api", "users")
	if err := os.MkdirAll(usersDir, 0755); err != nil {
		t.Fatalf("failed to create api/users directory: %v", err)
	}
	usersRegister := `package users

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Get("/users", ListUsers)
}
`
	if err := os.WriteFile(filepath.Join(usersDir, "register.go"), []byte(usersRegister), 0644); err != nil {
		t.Fatalf("failed to create users/register.go: %v", err)
	}

	// Create api/posts directory and register.go
	postsDir := filepath.Join(tmpDir, "api", "posts")
	if err := os.MkdirAll(postsDir, 0755); err != nil {
		t.Fatalf("failed to create api/posts directory: %v", err)
	}
	postsRegister := `package posts

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Post("/posts", CreatePost)
}
`
	if err := os.WriteFile(filepath.Join(postsDir, "register.go"), []byte(postsRegister), 0644); err != nil {
		t.Fatalf("failed to create posts/register.go: %v", err)
	}

	// Call parseAllRegisterFiles
	calls, err := parseAllRegisterFiles(tmpDir, modulePath, []string{
		"com.test/api/users",
		"com.test/api/posts",
	})
	if err != nil {
		t.Fatalf("parseAllRegisterFiles failed: %v", err)
	}

	// Verify we got 2 calls
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	// Verify first call (users)
	if calls[0].PackagePath != "com.test/api/users" {
		t.Errorf("calls[0].PackagePath = %q, want %q", calls[0].PackagePath, "com.test/api/users")
	}
	if calls[0].FuncName != "ListUsers" {
		t.Errorf("calls[0].FuncName = %q, want %q", calls[0].FuncName, "ListUsers")
	}

	// Verify second call (posts)
	if calls[1].PackagePath != "com.test/api/posts" {
		t.Errorf("calls[1].PackagePath = %q, want %q", calls[1].PackagePath, "com.test/api/posts")
	}
	if calls[1].FuncName != "CreatePost" {
		t.Errorf("calls[1].FuncName = %q, want %q", calls[1].FuncName, "CreatePost")
	}
}

// TestMergeStaticAndRuntimeSerialized_CopiesPackagePath tests that the merge
// function correctly copies PackagePath from static analysis to the result.
func TestMergeStaticAndRuntimeSerialized_CopiesPackagePath(t *testing.T) {
	static := []RegisterCall{
		{
			Method:      "Get",
			Path:        "/users",
			FuncName:    "ListUsers",
			PackagePath: "com.test/api/users",
		},
		{
			Method:      "Post",
			Path:        "/posts",
			FuncName:    "CreatePost",
			PackagePath: "com.test/api/posts",
		},
	}

	runtime := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/users",
			PackagePath: "", // Empty from runtime
		},
		{
			Method:      "POST",
			Path:        "/posts",
			PackagePath: "", // Empty from runtime
		},
	}

	merged, err := mergeStaticAndRuntimeSerialized(static, runtime)
	if err != nil {
		t.Fatalf("mergeStaticAndRuntimeSerialized failed: %v", err)
	}

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged handlers, got %d", len(merged))
	}

	// Verify PackagePath is copied from static to result
	if merged[0].PackagePath != "com.test/api/users" {
		t.Errorf("merged[0].PackagePath = %q, want %q", merged[0].PackagePath, "com.test/api/users")
	}
	if merged[0].FuncName != "ListUsers" {
		t.Errorf("merged[0].FuncName = %q, want %q", merged[0].FuncName, "ListUsers")
	}

	if merged[1].PackagePath != "com.test/api/posts" {
		t.Errorf("merged[1].PackagePath = %q, want %q", merged[1].PackagePath, "com.test/api/posts")
	}
	if merged[1].FuncName != "CreatePost" {
		t.Errorf("merged[1].FuncName = %q, want %q", merged[1].FuncName, "CreatePost")
	}
}

// TestBuildAndRunHandlerCompileProgram_PackagePathPopulated tests that the
// handler compile pipeline correctly populates PackagePath.
//
// This is essential for the test client to correctly qualify types with
// package prefixes (e.g., "auth.LoginRequest" instead of "LoginRequest").
func TestBuildAndRunHandlerCompileProgram_PackagePathPopulated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp project directory
	tmpDir := t.TempDir()

	modulePath := "com.test-pkgpath"
	expectedPkgPath := modulePath + "/api/simple"

	// Create go.mod (no external dependency on shipq — handler is embedded)
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Embed the handler package into shipq/lib/handler (mirrors real project setup)
	if err := embed.EmbedAllPackages(tmpDir, modulePath, embed.EmbedOptions{}); err != nil {
		t.Fatalf("failed to embed packages: %v", err)
	}

	// Create api/simple directory
	apiDir := filepath.Join(tmpDir, "api", "simple")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api/simple directory: %v", err)
	}

	handlerImport := modulePath + "/shipq/lib/handler"

	// Create handler.go with a simple handler
	handlerCode := `package simple

import "context"

type SimpleRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

type SimpleResponse struct {
	Message string ` + "`json:\"message\"`" + `
}

func SimpleHandler(ctx context.Context, req *SimpleRequest) (*SimpleResponse, error) {
	return &SimpleResponse{Message: "Hello, " + req.Name}, nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handler.go"), []byte(handlerCode), 0644); err != nil {
		t.Fatalf("failed to create handler.go: %v", err)
	}

	// Create register.go that registers the handler
	registerCode := `package simple

import "` + handlerImport + `"

func Register(app *handler.App) {
	app.Get("/simple", SimpleHandler)
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "register.go"), []byte(registerCode), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	// Run go mod tidy to resolve dependencies
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
	}

	// Run the handler compile pipeline
	cfg := HandlerCompileProgramConfig{
		ModulePath: modulePath,
		APIPkgs:    []string{expectedPkgPath},
	}

	handlers, err := BuildAndRunHandlerCompileProgram(tmpDir, cfg)
	if err != nil {
		t.Fatalf("BuildAndRunHandlerCompileProgram failed: %v", err)
	}

	// Verify we got exactly one handler
	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(handlers))
	}

	h := handlers[0]

	// THE KEY ASSERTION: PackagePath must be populated
	// Before the fix, this would be empty because:
	// 1. Runtime reflection doesn't set it
	// 2. Static analysis didn't capture it
	// 3. The merge function didn't copy it
	if h.PackagePath != expectedPkgPath {
		t.Errorf("PACKAGEPATH BUG: expected PackagePath %q, got %q", expectedPkgPath, h.PackagePath)
	}
}
