package testclient

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// allContent concatenates all generated file contents for searching.
func allContent(files []GeneratedTestClientFile) string {
	var sb strings.Builder
	for _, f := range files {
		sb.Write(f.Content)
	}
	return sb.String()
}

// findTestClientFile finds a generated file by path suffix.
func findTestClientFile(files []GeneratedTestClientFile, pathSuffix string) *GeneratedTestClientFile {
	for i := range files {
		if strings.HasSuffix(files[i].RelPath, pathSuffix) {
			return &files[i]
		}
	}
	return nil
}

// findTopLevelTestClient finds the top-level test client file.
func findTopLevelTestClient(files []GeneratedTestClientFile) *GeneratedTestClientFile {
	return findTestClientFile(files, "api/zz_generated_testclient.go")
}

// findResourceTestClient finds the per-resource test client file.
func findResourceTestClient(files []GeneratedTestClientFile, resource string) *GeneratedTestClientFile {
	return findTestClientFile(files, resource+"/http/zz_generated_testclient.go")
}

func TestGenerateHTTPTestClient_EmptyRegistry(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Should produce just the top-level file
	topLevel := findTopLevelTestClient(files)
	if topLevel == nil {
		t.Fatal("missing top-level test client")
	}
	codeStr := string(topLevel.Content)

	if !strings.Contains(codeStr, "package api") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(codeStr, "type Client struct") {
		t.Error("missing Client struct")
	}
	if !strings.Contains(codeStr, "func NewUnauthenticatedTestClient") {
		t.Error("missing constructor")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_SingleGetHandler(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
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

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Check per-resource test client
	resFile := findResourceTestClient(files, "users")
	if resFile == nil {
		t.Fatal("missing users resource test client")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, "func (c *UsersTestClient) GetUser") {
		t.Error("missing GetUser method on UsersTestClient")
	}
	if !strings.Contains(codeStr, "users.GetUserRequest") {
		t.Error("missing request type reference")
	}
	if !strings.Contains(codeStr, "users.GetUserResponse") {
		t.Error("missing response type reference")
	}
	if !strings.Contains(codeStr, `"example.com/app/api/users"`) {
		t.Error("missing handler package import")
	}
	if !strings.Contains(codeStr, "strings.NewReplacer") {
		t.Error("missing path parameter substitution")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("resource test client is not valid Go: %v\n%s", err, codeStr)
	}

	// Check top-level embeds the resource TestClient
	topLevel := findTopLevelTestClient(files)
	if topLevel == nil {
		t.Fatal("missing top-level test client")
	}
	topCode := string(topLevel.Content)

	if !strings.Contains(topCode, "usershttp.UsersTestClient") {
		t.Error("top-level Client should embed usershttp.UsersTestClient")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("top-level test client is not valid Go: %v\n%s", err, topCode)
	}
}

func TestGenerateHTTPTestClient_PostHandler(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
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

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if !strings.Contains(all, "func (c *UsersTestClient) CreateUser") {
		t.Error("missing CreateUser method")
	}
	if !strings.Contains(all, "json.Marshal(req)") {
		t.Error("missing JSON marshal for request body")
	}
	if !strings.Contains(all, `Header.Set("Content-Type", "application/json")`) {
		t.Error("missing Content-Type header")
	}

	// All files should be valid Go
	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

func TestGenerateHTTPTestClient_DeleteHandler_NoBody(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
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
				Response: nil,
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if !strings.Contains(all, "func (c *UsersTestClient) DeleteUser(ctx context.Context, req users.DeleteUserRequest) error") {
		t.Error("DELETE method should return only error")
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

func TestGenerateHTTPTestClient_MultipleHandlers(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/api/users",
				Request: &codegen.SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "UserResponse",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
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
					Name:    "UserResponse",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
			{
				Method:      "GET",
				Path:        "/users",
				FuncName:    "ListUsers",
				PackagePath: "example.com/app/api/users",
				Request: &codegen.SerializedStructInfo{
					Name:    "ListUsersRequest",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListUsersResponse",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Users", Type: "[]UserResponse", JSONName: "users", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if !strings.Contains(all, "func (c *UsersTestClient) CreateUser") {
		t.Error("missing CreateUser method")
	}
	if !strings.Contains(all, "func (c *UsersTestClient) GetUser") {
		t.Error("missing GetUser method")
	}
	if !strings.Contains(all, "func (c *UsersTestClient) ListUsers") {
		t.Error("missing ListUsers method")
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

func TestGenerateHTTPTestClient_MultiplePackages(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
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
					Name:    "UserResponse",
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
					Name:    "PostResponse",
					Package: "example.com/app/api/posts",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Should have per-resource files + top-level
	usersFile := findResourceTestClient(files, "users")
	postsFile := findResourceTestClient(files, "posts")
	topLevel := findTopLevelTestClient(files)

	if usersFile == nil {
		t.Fatal("missing users test client")
	}
	if postsFile == nil {
		t.Fatal("missing posts test client")
	}
	if topLevel == nil {
		t.Fatal("missing top-level test client")
	}

	// Top-level should embed both with unique type names
	topCode := string(topLevel.Content)
	if !strings.Contains(topCode, "usershttp.UsersTestClient") {
		t.Error("missing usershttp.UsersTestClient embed")
	}
	if !strings.Contains(topCode, "postshttp.PostsTestClient") {
		t.Error("missing postshttp.PostsTestClient embed")
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

func TestFindRequestFieldForParam(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Fields: []codegen.SerializedFieldInfo{
				{Name: "PublicID", Type: "string", JSONName: "public_id"},
				{Name: "Name", Type: "string", JSONName: "name"},
			},
		},
	}

	tests := []struct {
		paramName string
		want      string
	}{
		{"public_id", "PublicID"},
		{"name", "Name"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.paramName, func(t *testing.T) {
			got := findRequestFieldForParam(h, tt.paramName)
			if got != tt.want {
				t.Errorf("findRequestFieldForParam() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePathForReplacement(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/users", 1},
		{"/users/{id}", 2},
		{"/users/{id}/posts", 3},
		{"/users/{user_id}/posts/{post_id}", 4},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			parts := parsePathForReplacement(tt.path)
			if len(parts) != tt.want {
				t.Errorf("parsePathForReplacement(%q) = %d parts, want %d", tt.path, len(parts), tt.want)
			}
		})
	}
}

func TestGenerateHTTPTestClient_ErrorHandling(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
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
					Name:    "UserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if !strings.Contains(all, "httpResp.StatusCode >= 400") {
		t.Error("missing HTTP error status check")
	}
	if !strings.Contains(all, "io.ReadAll(httpResp.Body)") {
		t.Error("missing error body reading")
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

func TestGenerateHTTPTestClient_SimplePathWithNoParams(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/health",
				FuncName:    "HealthCheck",
				PackagePath: "example.com/app/api/health",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "HealthCheckRequest",
					Package: "example.com/app/api/health",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "HealthCheckResponse",
					Package: "example.com/app/api/health",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Status", Type: "string", JSONName: "status", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if strings.Contains(all, "strings.NewReplacer") {
		t.Error("should not use strings.NewReplacer for paths without params")
	}
	if !strings.Contains(all, `c.server.URL + "/health"`) {
		t.Error("missing direct URL concatenation")
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

func TestGenerateHTTPTestClient_StringsImportConditional(t *testing.T) {
	t.Run("no path params - strings should NOT be imported", func(t *testing.T) {
		cfg := HTTPTestClientGenConfig{
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

		files, err := GenerateHTTPTestClient(cfg)
		if err != nil {
			t.Fatalf("GenerateHTTPTestClient() error = %v", err)
		}

		all := allContent(files)

		if strings.Contains(all, `"strings"`) {
			t.Error("strings should not be imported when no handlers have path params")
		}

		for _, f := range files {
			_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
			if err != nil {
				t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
			}
		}
	})

	t.Run("with path params - strings should be imported", func(t *testing.T) {
		cfg := HTTPTestClientGenConfig{
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
							{Name: "Name", Type: "string", JSONName: "name", Required: true},
						},
					},
				},
			},
			OutputPkg: "api",
		}

		files, err := GenerateHTTPTestClient(cfg)
		if err != nil {
			t.Fatalf("GenerateHTTPTestClient() error = %v", err)
		}

		all := allContent(files)

		if !strings.Contains(all, `"strings"`) {
			t.Error("missing strings import when handlers have path params")
		}

		for _, f := range files {
			_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
			if err != nil {
				t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
			}
		}
	})
}

func TestGenerateHTTPTestClient_UsesHttputilAddAuth(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
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

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if !strings.Contains(all, "httputil.AddAuth") {
		t.Error("should use httputil.AddAuth instead of inline addAuth method")
	}
}

// ─── Query param tests (Step 7f) ───

func TestGenerateHTTPTestClient_QueryParams_AppendedToURL(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/posts",
				FuncName:    "ListPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Limit", Type: "int", JSONName: "limit", Required: false, Tags: map[string]string{"query": "limit"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, "url.Values") {
		t.Error("missing url.Values for query param construction")
	}
	if !strings.Contains(codeStr, `"limit"`) {
		t.Error("missing query param name \"limit\" in URL construction")
	}
	if !strings.Contains(codeStr, `qv.Encode()`) {
		t.Error("missing qv.Encode() for query string")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_PointerString(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/posts",
				FuncName:    "ListPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Cursor", Type: "*string", JSONName: "cursor", Required: false, Tags: map[string]string{"query": "cursor"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	// Pointer string should check != nil before adding to url.Values
	if !strings.Contains(codeStr, "req.Cursor != nil") {
		t.Error("missing nil check for *string query param")
	}
	if !strings.Contains(codeStr, "*req.Cursor") {
		t.Error("missing dereference *req.Cursor for *string query param")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_NoJSONBodyForGET(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/posts",
				FuncName:    "ListPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Limit", Type: "int", JSONName: "limit", Required: false, Tags: map[string]string{"query": "limit"}},
						{Name: "Cursor", Type: "*string", JSONName: "cursor", Required: false, Tags: map[string]string{"query": "cursor"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	// GET handler with only query-tagged fields should NOT send JSON body
	if strings.Contains(codeStr, "json.Marshal(req)") {
		t.Error("GET handler with only query fields should NOT call json.Marshal(req)")
	}
	if strings.Contains(codeStr, "bytes.NewReader(body)") {
		t.Error("GET handler with only query fields should NOT use bytes.NewReader(body)")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_MixedPathAndQuery(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id/posts",
				FuncName:    "ListUserPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListUserPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
						{Name: "Cursor", Type: "*string", JSONName: "cursor", Required: false, Tags: map[string]string{"query": "cursor"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListUserPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	// URL should have path param substitution
	if !strings.Contains(codeStr, "strings.NewReplacer") {
		t.Error("missing strings.NewReplacer for path param substitution")
	}
	if !strings.Contains(codeStr, `"{id}"`) {
		t.Error("missing {id} replacement for path param")
	}

	// AND query param appending
	if !strings.Contains(codeStr, "url.Values") {
		t.Error("missing url.Values for query param")
	}
	if !strings.Contains(codeStr, `"cursor"`) {
		t.Error("missing query param name \"cursor\"")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_IntFieldConversion(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/posts",
				FuncName:    "ListPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Limit", Type: "int", JSONName: "limit", Required: false, Tags: map[string]string{"query": "limit"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, "strconv.Itoa") {
		t.Error("missing strconv.Itoa for int query param conversion")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_BoolFieldConversion(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/posts",
				FuncName:    "ListPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "IncludeDeleted", Type: "bool", JSONName: "include_deleted", Required: false, Tags: map[string]string{"query": "include_deleted"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, "strconv.FormatBool") {
		t.Error("missing strconv.FormatBool for bool query param conversion")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_WithCookiesVariant(t *testing.T) {
	// The WithCookies variant of a POST method with query params should also
	// correctly construct the URL with query params.
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/posts",
				FuncName:    "CreatePost",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreatePostRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Tag", Type: "string", JSONName: "tag", Required: false, Tags: map[string]string{"query": "tag"}},
						{Name: "Title", Type: "string", JSONName: "title", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreatePostResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	// Should have both the normal method and WithCookies variant
	if !strings.Contains(codeStr, "func (c *PostsTestClient) CreatePost(") {
		t.Error("missing CreatePost method")
	}
	if !strings.Contains(codeStr, "func (c *PostsTestClient) CreatePostWithCookies(") {
		t.Error("missing CreatePostWithCookies method")
	}

	// Both should have url.Values for query param
	// Count occurrences of url.Values{} — should appear at least twice
	// (once for normal method, once for WithCookies)
	count := strings.Count(codeStr, "url.Values{}")
	if count < 2 {
		t.Errorf("expected url.Values{} at least twice (normal + WithCookies), got %d", count)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_ImportsPresent(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/posts",
				FuncName:    "ListPosts",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "ListPostsRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Limit", Type: "int", JSONName: "limit", Required: false, Tags: map[string]string{"query": "limit"}},
						{Name: "Cursor", Type: "*string", JSONName: "cursor", Required: false, Tags: map[string]string{"query": "cursor"}},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "ListPostsResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Items", Type: "[]string", JSONName: "items", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, `"net/url"`) {
		t.Error("missing \"net/url\" import when query-tagged fields exist")
	}
	if !strings.Contains(codeStr, `"strconv"`) {
		t.Error("missing \"strconv\" import when int query-tagged fields exist")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_QueryParams_NoImportsWhenNotNeeded(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/posts",
				FuncName:    "CreatePost",
				PackagePath: "example.com/app/api/posts",
				PathParams:  []codegen.SerializedPathParam{},
				Request: &codegen.SerializedStructInfo{
					Name:    "CreatePostRequest",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Title", Type: "string", JSONName: "title", Required: true},
						{Name: "Body", Type: "string", JSONName: "body", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreatePostResponse",
					Package: "example.com/app/api/posts",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	resFile := findResourceTestClient(files, "posts")
	if resFile == nil {
		t.Fatal("missing posts resource test client file")
	}
	codeStr := string(resFile.Content)

	if strings.Contains(codeStr, `"net/url"`) {
		t.Error("\"net/url\" should NOT be imported when no query-tagged fields exist")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", resFile.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestTestClientTypeName(t *testing.T) {
	tests := []struct {
		resource string
		want     string
	}{
		{"accounts", "AccountsTestClient"},
		{"users", "UsersTestClient"},
		{"organization_users", "OrganizationUsersTestClient"},
		{"pets", "PetsTestClient"},
		{"sessions", "SessionsTestClient"},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			got := testClientTypeName(tt.resource)
			if got != tt.want {
				t.Errorf("testClientTypeName(%q) = %q, want %q", tt.resource, got, tt.want)
			}
		})
	}
}
