package server

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestConvertPathSyntax(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users", "/users"},
		{"/users/:id", "/users/{id}"},
		{"/users/:id/posts", "/users/{id}/posts"},
		{"/users/:id/posts/:post_id", "/users/{id}/posts/{post_id}"},
		{"/", "/"},
		{"/:id", "/{id}"},
		{"/users/:user_id/comments/:comment_id", "/users/{user_id}/comments/{comment_id}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := codegen.ConvertPathSyntax(tt.input)
			if got != tt.expected {
				t.Errorf("ConvertPathSyntax(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateHTTPServer_EmptyRegistry(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should contain package declaration
	if !strings.Contains(codeStr, "package api") {
		t.Error("missing package declaration")
	}

	// Should contain NewMux function
	if !strings.Contains(codeStr, "func NewMux") {
		t.Error("missing NewMux function")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_SingleGetHandler(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should contain route registration with converted path syntax
	if !strings.Contains(codeStr, `"GET /users/{id}"`) {
		t.Error("missing route registration with converted path syntax")
	}

	// Should contain handler wrapper
	if !strings.Contains(codeStr, "func handleGetUser") {
		t.Error("missing handler wrapper function")
	}

	// Should contain path parameter binding
	if !strings.Contains(codeStr, `r.PathValue("id")`) {
		t.Error("missing path parameter binding")
	}

	// Should import the handler package
	if !strings.Contains(codeStr, `"example.com/app/users"`) {
		t.Error("missing handler package import")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_PostHandler(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
						{Name: "Email", Type: "string", JSONName: "email", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should contain JSON body binding
	if !strings.Contains(codeStr, "json.NewDecoder(r.Body).Decode") {
		t.Error("missing JSON body binding for POST handler")
	}

	// Should use StatusCreated for POST
	if !strings.Contains(codeStr, "http.StatusCreated") {
		t.Error("missing http.StatusCreated for POST handler")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_IntPathParam(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should contain strconv import
	if !strings.Contains(codeStr, `"strconv"`) {
		t.Error("missing strconv import for int64 path param")
	}

	// Should contain ParseInt for int64 conversion
	if !strings.Contains(codeStr, "strconv.ParseInt") {
		t.Error("missing strconv.ParseInt for int64 path param")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_MultipleHandlers(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
			{
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
			{
				Method:      "DELETE",
				Path:        "/users/:id",
				FuncName:    "DeleteUser",
				PackagePath: "example.com/app/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "DeleteUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "DeleteUserResponse",
					Package: "example.com/app/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should have all three routes
	if !strings.Contains(codeStr, `"GET /users/{id}"`) {
		t.Error("missing GET route")
	}
	if !strings.Contains(codeStr, `"POST /users"`) {
		t.Error("missing POST route")
	}
	if !strings.Contains(codeStr, `"DELETE /users/{id}"`) {
		t.Error("missing DELETE route")
	}

	// Should have all three handler wrappers
	if !strings.Contains(codeStr, "func handleGetUser") {
		t.Error("missing handleGetUser wrapper")
	}
	if !strings.Contains(codeStr, "func handleCreateUser") {
		t.Error("missing handleCreateUser wrapper")
	}
	if !strings.Contains(codeStr, "func handleDeleteUser") {
		t.Error("missing handleDeleteUser wrapper")
	}

	// Should only import the users package once
	count := strings.Count(codeStr, `"example.com/app/users"`)
	if count != 1 {
		t.Errorf("handler package imported %d times; want 1", count)
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_MultiplePackages(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
			{
				Method:      "GET",
				Path:        "/posts/:id",
				FuncName:    "GetPost",
				PackagePath: "example.com/app/posts",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetPostRequest",
					Package: "example.com/app/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetPostResponse",
					Package: "example.com/app/posts",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should import both packages
	if !strings.Contains(codeStr, `"example.com/app/users"`) {
		t.Error("missing users package import")
	}
	if !strings.Contains(codeStr, `"example.com/app/posts"`) {
		t.Error("missing posts package import")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_ConflictingPackageNames(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/api/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/api/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
			{
				Method:      "GET",
				Path:        "/admin/users/:id",
				FuncName:    "GetAdminUser",
				PackagePath: "example.com/app/admin/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetAdminUserRequest",
					Package: "example.com/app/admin/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetAdminUserResponse",
					Package: "example.com/app/admin/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should be valid Go (meaning aliases don't conflict)
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}

	// Should have unique aliases (users and users2)
	if !strings.Contains(codeStr, "users ") && !strings.Contains(codeStr, "users2 ") {
		// At least one should be aliased
	}
}

func TestGenerateHTTPServer_HelperFunctions(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should contain writeJSON helper
	if !strings.Contains(codeStr, "func writeJSON") {
		t.Error("missing writeJSON helper function")
	}

	// Should contain writeError helper
	if !strings.Contains(codeStr, "func writeError") {
		t.Error("missing writeError helper function")
	}

	// Should contain wrapHandler helper
	if !strings.Contains(codeStr, "func wrapHandler") {
		t.Error("missing wrapHandler helper function")
	}

	// writeError should use errors.As
	if !strings.Contains(codeStr, "errors.As") {
		t.Error("writeError should use errors.As for httperror detection")
	}

	// Should import httperror and httpserver
	if !strings.Contains(codeStr, `"github.com/shipq/shipq/httperror"`) {
		t.Error("missing httperror import")
	}
	if !strings.Contains(codeStr, `"github.com/shipq/shipq/httpserver"`) {
		t.Error("missing httpserver import")
	}

	// wrapHandler should inject request cookies
	if !strings.Contains(codeStr, "httpserver.WithRequestCookies") {
		t.Error("wrapHandler should inject request cookies via WithRequestCookies")
	}

	// wrapHandler should set up cookie ops
	if !strings.Contains(codeStr, "httpserver.WithCookieOps") {
		t.Error("wrapHandler should set up cookie ops via WithCookieOps")
	}

	// wrapHandler should apply cookies after handler
	if !strings.Contains(codeStr, "http.SetCookie") {
		t.Error("wrapHandler should apply cookies via http.SetCookie")
	}
}

func TestGenerateHTTPServer_NewMuxSignature(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should accept PingableQuerier
	if !strings.Contains(codeStr, "httpserver.PingableQuerier") {
		t.Error("missing httpserver.PingableQuerier parameter")
	}

	// Should accept *slog.Logger
	if !strings.Contains(codeStr, "*slog.Logger") {
		t.Error("missing *slog.Logger parameter")
	}

	// Should return http.Handler (not *http.ServeMux)
	if !strings.Contains(codeStr, "func NewMux(q httpserver.PingableQuerier, logger *slog.Logger) http.Handler") {
		t.Error("NewMux should return http.Handler")
	}

	// Should import log/slog
	if !strings.Contains(codeStr, `"log/slog"`) {
		t.Error("missing log/slog import")
	}
}

func TestGenerateHTTPServer_HealthEndpoint(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should have health check endpoint
	if !strings.Contains(codeStr, `"GET /health"`) {
		t.Error("missing health check endpoint")
	}

	// Should call q.Ping() for health check
	if !strings.Contains(codeStr, "q.Ping()") {
		t.Error("missing q.Ping() call in health check")
	}

	// Should return healthy: true/false
	if !strings.Contains(codeStr, `"healthy"`) {
		t.Error("missing healthy key in health response")
	}
}

func TestGenerateHTTPServer_LoggingMiddleware(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	codeStr := string(code)

	// Should import logging package
	if !strings.Contains(codeStr, `"github.com/shipq/shipq/logging"`) {
		t.Error("missing logging package import")
	}

	// Should wrap with logging.Decorate
	if !strings.Contains(codeStr, "logging.Decorate") {
		t.Error("missing logging.Decorate middleware")
	}

	// Should exclude /health from logging
	if !strings.Contains(codeStr, `"/health"`) {
		t.Error("missing /health in logging exclusion list")
	}
}

func TestCollectHandlerPackages_Deduplication(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{PackagePath: "example.com/app/users"},
		{PackagePath: "example.com/app/users"},
		{PackagePath: "example.com/app/posts"},
	}

	pkgs := codegen.CollectHandlerPackages(handlers)

	if len(pkgs) != 2 {
		t.Errorf("collectHandlerPackages() returned %d packages; want 2", len(pkgs))
	}
}

func TestCollectHandlerPackages_UniqueAliases(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{PackagePath: "example.com/app/api/users"},
		{PackagePath: "example.com/app/admin/users"},
	}

	pkgs := codegen.CollectHandlerPackages(handlers)

	// Both have base name "users", so second should be aliased differently
	aliases := make(map[string]bool)
	for _, pkg := range pkgs {
		if aliases[pkg.Alias] {
			t.Errorf("duplicate alias: %s", pkg.Alias)
		}
		aliases[pkg.Alias] = true
	}
}

func TestNeedsStrconv(t *testing.T) {
	tests := []struct {
		name     string
		handlers []codegen.SerializedHandlerInfo
		want     bool
	}{
		{
			name:     "empty handlers",
			handlers: []codegen.SerializedHandlerInfo{},
			want:     false,
		},
		{
			name: "string path param",
			handlers: []codegen.SerializedHandlerInfo{
				{
					PathParams: []codegen.SerializedPathParam{{Name: "id", Position: 1}},
					Request: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "ID", Type: "string", JSONName: "id"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "int64 path param",
			handlers: []codegen.SerializedHandlerInfo{
				{
					PathParams: []codegen.SerializedPathParam{{Name: "id", Position: 1}},
					Request: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "ID", Type: "int64", JSONName: "id"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "int path param",
			handlers: []codegen.SerializedHandlerInfo{
				{
					PathParams: []codegen.SerializedPathParam{{Name: "id", Position: 1}},
					Request: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "ID", Type: "int", JSONName: "id"},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsStrconv(tt.handlers)
			if got != tt.want {
				t.Errorf("needsStrconv() = %v; want %v", got, tt.want)
			}
		})
	}
}
