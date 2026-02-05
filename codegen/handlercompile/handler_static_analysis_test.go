package handlercompile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/handler"
)

func TestParseRegisterFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "handler_static_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name          string
		content       string
		expectedCalls []RegisterCall
		expectError   bool
	}{
		{
			name: "basic CRUD handlers",
			content: `package posts

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Post("/posts", CreatePost)
	app.Get("/posts", ListPosts)
	app.Get("/posts/:id", GetPost)
	app.Patch("/posts/:id", UpdatePost)
	app.Delete("/posts/:id", SoftDeletePost)
}
`,
			expectedCalls: []RegisterCall{
				{Method: "Post", Path: "/posts", FuncName: "CreatePost"},
				{Method: "Get", Path: "/posts", FuncName: "ListPosts"},
				{Method: "Get", Path: "/posts/:id", FuncName: "GetPost"},
				{Method: "Patch", Path: "/posts/:id", FuncName: "UpdatePost"},
				{Method: "Delete", Path: "/posts/:id", FuncName: "SoftDeletePost"},
			},
			expectError: false,
		},
		{
			name: "PUT handler",
			content: `package users

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Put("/users/:id", ReplaceUser)
}
`,
			expectedCalls: []RegisterCall{
				{Method: "Put", Path: "/users/:id", FuncName: "ReplaceUser"},
			},
			expectError: false,
		},
		{
			name: "nested path params",
			content: `package comments

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Get("/posts/:post_id/comments/:id", GetComment)
}
`,
			expectedCalls: []RegisterCall{
				{Method: "Get", Path: "/posts/:post_id/comments/:id", FuncName: "GetComment"},
			},
			expectError: false,
		},
		{
			name: "empty register function",
			content: `package empty

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	// No handlers registered
}
`,
			expectedCalls: []RegisterCall{},
			expectError:   false,
		},
		{
			name: "anonymous function - should error",
			content: `package bad

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Get("/test", func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return nil, nil
	})
}
`,
			expectedCalls: nil,
			expectError:   true,
		},
		{
			name: "variable path - should error",
			content: `package bad

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	path := "/test"
	app.Get(path, TestHandler)
}
`,
			expectedCalls: nil,
			expectError:   true,
		},
		{
			name: "function call as handler - should error",
			content: `package bad

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Get("/test", getHandler())
}
`,
			expectedCalls: nil,
			expectError:   true,
		},
		{
			name: "no Register function",
			content: `package other

import "github.com/shipq/shipq/handler"

func Setup(app *handler.App) {
	app.Get("/test", TestHandler)
}
`,
			expectedCalls: []RegisterCall{},
			expectError:   false,
		},
		{
			name: "other app methods ignored",
			content: `package mixed

import "github.com/shipq/shipq/handler"

func Register(app *handler.App) {
	app.Get("/test", TestHandler)
	app.SomeOtherMethod("ignored")
	app.Configure()
}
`,
			expectedCalls: []RegisterCall{
				{Method: "Get", Path: "/test", FuncName: "TestHandler"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			testFile := filepath.Join(tmpDir, "register.go")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			calls, err := ParseRegisterFile(testFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(calls) != len(tt.expectedCalls) {
				t.Fatalf("expected %d calls, got %d", len(tt.expectedCalls), len(calls))
			}

			for i, expected := range tt.expectedCalls {
				actual := calls[i]
				if actual.Method != expected.Method {
					t.Errorf("call %d: expected method %s, got %s", i, expected.Method, actual.Method)
				}
				if actual.Path != expected.Path {
					t.Errorf("call %d: expected path %s, got %s", i, expected.Path, actual.Path)
				}
				if actual.FuncName != expected.FuncName {
					t.Errorf("call %d: expected funcName %s, got %s", i, expected.FuncName, actual.FuncName)
				}
				if actual.Line == 0 {
					t.Errorf("call %d: line number should not be 0", i)
				}
			}
		})
	}
}

func TestParseRegisterFile_InvalidFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "handler_static_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write invalid Go file
	testFile := filepath.Join(tmpDir, "invalid.go")
	if err := os.WriteFile(testFile, []byte("this is not valid go code"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = ParseRegisterFile(testFile)
	if err == nil {
		t.Error("expected error for invalid Go file")
	}
}

func TestParseRegisterFile_NonexistentFile(t *testing.T) {
	_, err := ParseRegisterFile("/nonexistent/path/register.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestIsHTTPMethod(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"Get", true},
		{"Post", true},
		{"Put", true},
		{"Patch", true},
		{"Delete", true},
		{"get", false},
		{"GET", false},
		{"Options", false},
		{"Head", false},
		{"Connect", false},
		{"SomeOther", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := isHTTPMethod(tt.method)
			if result != tt.expected {
				t.Errorf("isHTTPMethod(%q) = %v, want %v", tt.method, result, tt.expected)
			}
		})
	}
}

func TestMergeStaticAndRuntime(t *testing.T) {
	static := []RegisterCall{
		{Method: "Post", Path: "/posts", FuncName: "CreatePost", Line: 10},
		{Method: "Get", Path: "/posts/:id", FuncName: "GetPost", Line: 11},
	}

	runtime := []handler.HandlerInfo{
		{
			Method: handler.POST,
			Path:   "/posts",
			Request: &handler.StructInfo{
				Name: "CreatePostRequest",
			},
			Response: &handler.StructInfo{
				Name: "CreatePostResponse",
			},
		},
		{
			Method: handler.GET,
			Path:   "/posts/:id",
			PathParams: []handler.PathParam{
				{Name: "id", Position: 2},
			},
			Request: &handler.StructInfo{
				Name: "GetPostRequest",
			},
			Response: &handler.StructInfo{
				Name: "GetPostResponse",
			},
		},
	}

	result, err := MergeStaticAndRuntime(static, runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	// Verify first handler
	if result[0].FuncName != "CreatePost" {
		t.Errorf("expected FuncName 'CreatePost', got %s", result[0].FuncName)
	}
	if result[0].Request.Name != "CreatePostRequest" {
		t.Errorf("expected request name 'CreatePostRequest', got %s", result[0].Request.Name)
	}

	// Verify second handler
	if result[1].FuncName != "GetPost" {
		t.Errorf("expected FuncName 'GetPost', got %s", result[1].FuncName)
	}
	if len(result[1].PathParams) != 1 {
		t.Errorf("expected 1 path param, got %d", len(result[1].PathParams))
	}
}

func TestMergeStaticAndRuntime_LengthMismatch(t *testing.T) {
	static := []RegisterCall{
		{Method: "Get", Path: "/test", FuncName: "Test"},
	}

	runtime := []handler.HandlerInfo{
		{Method: handler.GET, Path: "/test"},
		{Method: handler.POST, Path: "/test2"},
	}

	_, err := MergeStaticAndRuntime(static, runtime)
	if err == nil {
		t.Error("expected error for length mismatch")
	}
}

func TestMergeStaticAndRuntime_MethodMismatch(t *testing.T) {
	static := []RegisterCall{
		{Method: "Post", Path: "/test", FuncName: "Test"},
	}

	runtime := []handler.HandlerInfo{
		{Method: handler.GET, Path: "/test"},
	}

	_, err := MergeStaticAndRuntime(static, runtime)
	if err == nil {
		t.Error("expected error for method mismatch")
	}
}

func TestMergeStaticAndRuntime_PathMismatch(t *testing.T) {
	static := []RegisterCall{
		{Method: "Get", Path: "/test", FuncName: "Test"},
	}

	runtime := []handler.HandlerInfo{
		{Method: handler.GET, Path: "/different"},
	}

	_, err := MergeStaticAndRuntime(static, runtime)
	if err == nil {
		t.Error("expected error for path mismatch")
	}
}

func TestHTTPMethodFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected handler.HTTPMethod
	}{
		{"Get", handler.GET},
		{"Post", handler.POST},
		{"Put", handler.PUT},
		{"Patch", handler.PATCH},
		{"Delete", handler.DELETE},
		{"Unknown", handler.HTTPMethod("UNKNOWN")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := HTTPMethodFromString(tt.input)
			if result != tt.expected {
				t.Errorf("HTTPMethodFromString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
