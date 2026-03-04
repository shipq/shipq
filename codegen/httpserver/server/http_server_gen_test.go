package server

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// findFile finds a generated file by relative path suffix.
func findFile(files []GeneratedHTTPFile, pathSuffix string) *GeneratedHTTPFile {
	for i := range files {
		if strings.HasSuffix(files[i].RelPath, pathSuffix) {
			return &files[i]
		}
	}
	return nil
}

// findTopLevel finds the top-level api/zz_generated_http.go file.
func findTopLevel(files []GeneratedHTTPFile) *GeneratedHTTPFile {
	return findFile(files, "api/zz_generated_http.go")
}

// findResourceHTTP finds the resource's http sub-package file.
func findResourceHTTP(files []GeneratedHTTPFile, resource string) *GeneratedHTTPFile {
	return findFile(files, resource+"/http/zz_generated_http.go")
}

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

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	// Should produce exactly one file: the top-level
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	topLevel := findTopLevel(files)
	if topLevel == nil {
		t.Fatal("missing top-level api/zz_generated_http.go")
	}

	codeStr := string(topLevel.Content)

	if !strings.Contains(codeStr, "package api") {
		t.Error("missing package declaration")
	}

	if !strings.Contains(codeStr, "func NewMux") {
		t.Error("missing NewMux function")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
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
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	// Should produce 2 files: resource http + top-level
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Check resource file
	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users/http/zz_generated_http.go")
	}
	resCode := string(resFile.Content)

	if !strings.Contains(resCode, `"GET /users/{id}"`) {
		t.Error("missing route registration with converted path syntax")
	}
	if !strings.Contains(resCode, "func handleGetUser") {
		t.Error("missing handler wrapper function")
	}
	if !strings.Contains(resCode, `r.PathValue("id")`) {
		t.Error("missing path parameter binding")
	}
	if !strings.Contains(resCode, `"example.com/app/api/users"`) {
		t.Error("missing handler package import")
	}
	if !strings.Contains(resCode, "func RegisterRoutes") {
		t.Error("missing RegisterRoutes function")
	}
	if !strings.Contains(resCode, "httputil.WrapHandler") {
		t.Error("missing httputil.WrapHandler call")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("resource code is not valid Go: %v\n%s", err, resCode)
	}

	// Check top-level file
	topLevel := findTopLevel(files)
	if topLevel == nil {
		t.Fatal("missing top-level file")
	}
	topCode := string(topLevel.Content)

	if !strings.Contains(topCode, "usershttp.RegisterRoutes") {
		t.Error("missing usershttp.RegisterRoutes call")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("top-level code is not valid Go: %v\n%s", err, topCode)
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
				PackagePath: "example.com/app/api/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
						{Name: "Email", Type: "string", JSONName: "email", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, "json.NewDecoder(r.Body).Decode") {
		t.Error("missing JSON body binding for POST handler")
	}
	if !strings.Contains(codeStr, "http.StatusCreated") {
		t.Error("missing http.StatusCreated for POST handler")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
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
				PackagePath: "example.com/app/api/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, `"strconv"`) {
		t.Error("missing strconv import for int64 path param")
	}
	if !strings.Contains(codeStr, "strconv.ParseInt") {
		t.Error("missing strconv.ParseInt for int64 path param")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
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
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/api/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
			{
				Method:      "DELETE",
				Path:        "/users/:id",
				FuncName:    "DeleteUser",
				PackagePath: "example.com/app/api/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "DeleteUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "DeleteUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, `"GET /users/{id}"`) {
		t.Error("missing GET route")
	}
	if !strings.Contains(codeStr, `"POST /users"`) {
		t.Error("missing POST route")
	}
	if !strings.Contains(codeStr, `"DELETE /users/{id}"`) {
		t.Error("missing DELETE route")
	}

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
	count := strings.Count(codeStr, `"example.com/app/api/users"`)
	if count != 1 {
		t.Errorf("handler package imported %d times; want 1", count)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
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
				Path:        "/posts/:id",
				FuncName:    "GetPost",
				PackagePath: "example.com/app/api/posts",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "GetPostRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetPostResponse",
					Package: "example.com/app/api/posts",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	// Should produce 3 files: users/http, posts/http, and top-level
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	usersFile := findResourceHTTP(files, "users")
	if usersFile == nil {
		t.Fatal("missing users resource file")
	}
	postsFile := findResourceHTTP(files, "posts")
	if postsFile == nil {
		t.Fatal("missing posts resource file")
	}

	// All files should be valid Go
	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}

	// Top-level should import both sub-packages
	topLevel := findTopLevel(files)
	topCode := string(topLevel.Content)
	if !strings.Contains(topCode, "usershttp.RegisterRoutes") {
		t.Error("missing usershttp.RegisterRoutes call")
	}
	if !strings.Contains(topCode, "postshttp.RegisterRoutes") {
		t.Error("missing postshttp.RegisterRoutes call")
	}
}

func TestGenerateHTTPServer_HelperFunctions(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users",
				FuncName:    "ListUsers",
				PackagePath: "example.com/app/api/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListUsersRequest",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListUsersResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource file")
	}
	codeStr := string(resFile.Content)

	// Resource files should use httputil helpers
	if !strings.Contains(codeStr, "httputil.WriteError") || !strings.Contains(codeStr, "httputil.WriteJSON") {
		t.Error("resource file should use httputil.WriteError and httputil.WriteJSON")
	}
	if !strings.Contains(codeStr, "httputil.WrapHandler") {
		t.Error("resource file should use httputil.WrapHandler")
	}

	// Should import httputil, httpserver (httperror is only needed for typed
	// path params or JSON body binding, which this GET handler doesn't have)
	if !strings.Contains(codeStr, `/shipq/lib/httputil"`) {
		t.Error("missing httputil import")
	}
	if !strings.Contains(codeStr, `/shipq/lib/httpserver"`) {
		t.Error("missing httpserver import")
	}

	// Top-level should NOT import httputil — the health endpoint is now
	// registered through the normal handler pipeline (api/health package),
	// not inline in the top-level generated file.
	topLevel := findTopLevel(files)
	topCode := string(topLevel.Content)
	if strings.Contains(topCode, "httputil.WriteJSON") {
		t.Error("top-level should not reference httputil.WriteJSON (health endpoint moved to api/health)")
	}
}

func TestGenerateHTTPServer_NewMuxSignature(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	if !strings.Contains(codeStr, "httpserver.PingableQuerier") {
		t.Error("missing httpserver.PingableQuerier parameter")
	}
	if !strings.Contains(codeStr, "*slog.Logger") {
		t.Error("missing *slog.Logger parameter")
	}
	if !strings.Contains(codeStr, "func NewMux(q httpserver.PingableQuerier, runner queries.Runner, logger *slog.Logger) http.Handler") {
		t.Error("NewMux should return http.Handler")
	}
	if !strings.Contains(codeStr, `"log/slog"`) {
		t.Error("missing log/slog import")
	}
}

func TestGenerateHTTPServer_HealthEndpointNotInline(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	// The inline health endpoint has been removed from the top-level generated
	// file. The health route is now registered through the normal handler
	// pipeline via the api/health package scaffolded by `shipq init`.
	if strings.Contains(codeStr, `"GET /health"`) {
		t.Error("top-level should not contain inline GET /health route (moved to api/health)")
	}
	if strings.Contains(codeStr, "q.Ping()") {
		t.Error("top-level should not contain q.Ping() (moved to api/health handler)")
	}
}

func TestGenerateHTTPServer_LoggingMiddleware(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	if !strings.Contains(codeStr, `/shipq/lib/logging"`) {
		t.Error("missing logging package import")
	}
	if !strings.Contains(codeStr, "logging.Decorate") {
		t.Error("missing logging.Decorate middleware")
	}
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

func TestGenerateHTTPServer_HandlerCallPassesPointer(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/login",
				FuncName:    "Login",
				PackagePath: "example.com/app/api/auth",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "LoginRequest",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Email", Type: "string", JSONName: "email", Required: true},
						{Name: "Password", Type: "string", JSONName: "password", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "LoginResponse",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Token", Type: "string", JSONName: "token", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "auth")
	if resFile == nil {
		t.Fatal("missing auth resource file")
	}
	codeStr := string(resFile.Content)

	// In per-resource file, the handler package is aliased as the resource name
	if !strings.Contains(codeStr, "auth.Login(r.Context(), &req)") {
		t.Errorf("POINTER BUG: handler call should pass &req (pointer)\nGenerated code:\n%s", codeStr)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_EmptyRequestPassesPointer(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/me",
				FuncName:    "Me",
				PackagePath: "example.com/app/api/auth",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "MeRequest",
					Package: "example.com/app/api/auth",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "MeResponse",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "UserID", Type: "string", JSONName: "user_id", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "auth")
	if resFile == nil {
		t.Fatal("missing auth resource file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, "&auth.MeRequest{}") {
		t.Errorf("POINTER BUG: empty request should pass &Type{} (pointer)\nGenerated code:\n%s", codeStr)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPServer_RegisterRoutesFunction(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users",
				FuncName:    "ListUsers",
				PackagePath: "example.com/app/api/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListUsersRequest",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListUsersResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource file")
	}
	codeStr := string(resFile.Content)

	// Should have RegisterRoutes with correct signature
	if !strings.Contains(codeStr, "func RegisterRoutes(mux *http.ServeMux, q httpserver.PingableQuerier, runner queries.Runner)") {
		t.Error("missing RegisterRoutes with correct signature")
	}

	// Should have context injection closure
	if !strings.Contains(codeStr, "queries.NewContextWithRunner") {
		t.Error("missing queries.NewContextWithRunner in RegisterRoutes")
	}
}

func TestGenerateHTTPServer_ErrorLogging(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/api/users",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource file")
	}
	codeStr := string(resFile.Content)

	// Should import config package
	if !strings.Contains(codeStr, `"example.com/app/config"`) {
		t.Error("missing config package import")
	}

	// Should log errors with config.Logger.Error before writing the error response
	if !strings.Contains(codeStr, `config.Logger.Error("CreateUser failed"`) {
		t.Errorf("missing config.Logger.Error call in handler wrapper\nGenerated code:\n%s", codeStr)
	}

	// Should still have httputil.WriteError after the log
	if !strings.Contains(codeStr, "httputil.WriteError(w, err)") {
		t.Error("missing httputil.WriteError after logging")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

// ── HasChannels tests ────────────────────────────────────────────────────────

func TestGenerateHTTPServer_HasChannels_GeneratesSetupMux(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath:  "example.com/app",
		Handlers:    []codegen.SerializedHandlerInfo{},
		OutputPkg:   "api",
		HasChannels: true,
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	if !strings.Contains(codeStr, "func SetupMux(q httpserver.PingableQuerier, runner queries.Runner) *http.ServeMux") {
		t.Error("expected SetupMux function returning *http.ServeMux")
	}
}

func TestGenerateHTTPServer_HasChannels_NewMuxDelegatesToSetupMux(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath:  "example.com/app",
		Handlers:    []codegen.SerializedHandlerInfo{},
		OutputPkg:   "api",
		HasChannels: true,
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	// NewMux should delegate to SetupMux
	if !strings.Contains(codeStr, "SetupMux(q, runner)") {
		t.Error("NewMux should call SetupMux internally when HasChannels is true")
	}

	// NewMux should still exist with the same signature
	if !strings.Contains(codeStr, "func NewMux(q httpserver.PingableQuerier, runner queries.Runner, logger *slog.Logger) http.Handler") {
		t.Error("NewMux should keep the same signature for backward compatibility")
	}
}

func TestGenerateHTTPServer_HasChannels_SetupMuxNoInlineHealth(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath:  "example.com/app",
		Handlers:    []codegen.SerializedHandlerInfo{},
		OutputPkg:   "api",
		HasChannels: true,
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	// SetupMux should NOT contain an inline health endpoint — the health
	// route is registered through the normal handler pipeline (api/health).
	if strings.Contains(codeStr, `"GET /health"`) {
		t.Error("SetupMux should not contain inline GET /health (moved to api/health)")
	}
}

func TestGenerateHTTPServer_HasChannels_ValidGo(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath:  "example.com/app",
		Handlers:    []codegen.SerializedHandlerInfo{},
		OutputPkg:   "api",
		HasChannels: true,
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "server.go", topLevel.Content, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated code with HasChannels is not valid Go: %v\n%s", parseErr, string(topLevel.Content))
	}
}

func TestGenerateHTTPServer_NoChannels_NoSetupMux(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath:  "example.com/app",
		Handlers:    []codegen.SerializedHandlerInfo{},
		OutputPkg:   "api",
		HasChannels: false,
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	codeStr := string(topLevel.Content)

	if strings.Contains(codeStr, "func SetupMux(") {
		t.Error("SetupMux should NOT be generated when HasChannels is false")
	}
}

func TestGenerateHTTPServer_PerResourcePackageName(t *testing.T) {
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/accounts",
				FuncName:    "ListAccounts",
				PackagePath: "example.com/app/api/accounts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListAccountsRequest",
					Package: "example.com/app/api/accounts",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListAccountsResponse",
					Package: "example.com/app/api/accounts",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	resFile := findResourceHTTP(files, "accounts")
	if resFile == nil {
		t.Fatal("missing accounts resource file")
	}
	codeStr := string(resFile.Content)

	// Package name should be "accountshttp" to avoid conflict with stdlib "http"
	if !strings.Contains(codeStr, "package accountshttp") {
		t.Error("expected package name 'accountshttp'")
	}
}

func TestGenerateHTTPServer_OAuthNoConfigImport(t *testing.T) {
	// Regression test for Bug 1: when HasOAuth is true the top-level
	// generated file must NOT import the config package (it is unused there).
	cfg := HTTPServerGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/auth/login",
				FuncName:    "Login",
				PackagePath: "example.com/app/api/auth",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "LoginRequest",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Email", Type: "string", JSONName: "email", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "LoginResponse",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Token", Type: "string", JSONName: "token", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
		HasOAuth:  true,
	}

	files, err := GenerateHTTPServer(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPServer() error = %v", err)
	}

	topLevel := findTopLevel(files)
	if topLevel == nil {
		t.Fatal("missing top-level api/zz_generated_http.go")
	}
	topCode := string(topLevel.Content)

	// The top-level file must NOT import the config package — nothing in
	// the top-level file references config.*.
	if strings.Contains(topCode, `"example.com/app/config"`) {
		t.Error("top-level file must NOT import config package (unused)")
	}

	// The generated code must be valid Go (unused imports cause a parse/compile error).
	_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("top-level code is not valid Go: %v\n%s", err, topCode)
	}
}
