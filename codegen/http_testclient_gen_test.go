package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateHTTPTestClient_EmptyRegistry(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should have package declaration
	if !strings.Contains(codeStr, "package api") {
		t.Error("missing package declaration")
	}

	// Should have Client struct
	if !strings.Contains(codeStr, "type Client struct") {
		t.Error("missing Client struct")
	}

	// Should have constructor
	if !strings.Contains(codeStr, "func NewUnauthenticatedTestClient") {
		t.Error("missing constructor")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_SingleGetHandler(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should have method for GetUser
	if !strings.Contains(codeStr, "func (c *Client) GetUser") {
		t.Error("missing GetUser method")
	}

	// Should have correct parameter and return types
	if !strings.Contains(codeStr, "users.GetUserRequest") {
		t.Error("missing request type reference")
	}
	if !strings.Contains(codeStr, "users.GetUserResponse") {
		t.Error("missing response type reference")
	}

	// Should import the handler package
	if !strings.Contains(codeStr, `"example.com/app/users"`) {
		t.Error("missing handler package import")
	}

	// Should have path parameter substitution
	if !strings.Contains(codeStr, "strings.NewReplacer") {
		t.Error("missing path parameter substitution")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_PostHandler(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/users",
				PathParams:  []SerializedPathParam{},
				Request: &SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
						{Name: "Email", Type: "string", JSONName: "email", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should have method for CreateUser
	if !strings.Contains(codeStr, "func (c *Client) CreateUser") {
		t.Error("missing CreateUser method")
	}

	// Should marshal request body
	if !strings.Contains(codeStr, "json.Marshal(req)") {
		t.Error("missing JSON marshal for request body")
	}

	// Should set Content-Type header
	if !strings.Contains(codeStr, `Header.Set("Content-Type", "application/json")`) {
		t.Error("missing Content-Type header")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_DeleteHandler_NoBody(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "DELETE",
				Path:        "/users/:id",
				FuncName:    "DeleteUser",
				PackagePath: "example.com/app/users",
				PathParams: []SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &SerializedStructInfo{
					Name:    "DeleteUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: nil, // No response body for DELETE
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should have method that returns only error
	if !strings.Contains(codeStr, "func (c *Client) DeleteUser(ctx context.Context, req users.DeleteUserRequest) error") {
		t.Error("DELETE method should return only error")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_MultipleHandlers(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "POST",
				Path:        "/users",
				FuncName:    "CreateUser",
				PackagePath: "example.com/app/users",
				Request: &SerializedStructInfo{
					Name:    "CreateUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "UserResponse",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "UserResponse",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
			},
			{
				Method:      "GET",
				Path:        "/users",
				FuncName:    "ListUsers",
				PackagePath: "example.com/app/users",
				Request: &SerializedStructInfo{
					Name:    "ListUsersRequest",
					Package: "example.com/app/users",
					Fields:  []SerializedFieldInfo{},
				},
				Response: &SerializedStructInfo{
					Name:    "ListUsersResponse",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "Users", Type: "[]UserResponse", JSONName: "users", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should have all methods
	if !strings.Contains(codeStr, "func (c *Client) CreateUser") {
		t.Error("missing CreateUser method")
	}
	if !strings.Contains(codeStr, "func (c *Client) GetUser") {
		t.Error("missing GetUser method")
	}
	if !strings.Contains(codeStr, "func (c *Client) ListUsers") {
		t.Error("missing ListUsers method")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_MultiplePackages(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "UserResponse",
					Package: "example.com/app/users",
					Fields:  []SerializedFieldInfo{},
				},
			},
			{
				Method:      "GET",
				Path:        "/posts/:id",
				FuncName:    "GetPost",
				PackagePath: "example.com/app/posts",
				PathParams: []SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &SerializedStructInfo{
					Name:    "GetPostRequest",
					Package: "example.com/app/posts",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "PostResponse",
					Package: "example.com/app/posts",
					Fields:  []SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should import both packages
	if !strings.Contains(codeStr, `"example.com/app/users"`) {
		t.Error("missing users package import")
	}
	if !strings.Contains(codeStr, `"example.com/app/posts"`) {
		t.Error("missing posts package import")
	}

	// Should use correct package aliases
	if !strings.Contains(codeStr, "users.GetUserRequest") {
		t.Error("missing users package alias usage")
	}
	if !strings.Contains(codeStr, "posts.GetPostRequest") {
		t.Error("missing posts package alias usage")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestFindRequestFieldForParam(t *testing.T) {
	h := SerializedHandlerInfo{
		Request: &SerializedStructInfo{
			Fields: []SerializedFieldInfo{
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
		want int // number of parts
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
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/users",
				PathParams: []SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/users",
					Fields: []SerializedFieldInfo{
						{Name: "ID", Type: "string", JSONName: "id", Required: true},
					},
				},
				Response: &SerializedStructInfo{
					Name:    "UserResponse",
					Package: "example.com/app/users",
					Fields:  []SerializedFieldInfo{},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should handle HTTP error responses
	if !strings.Contains(codeStr, "httpResp.StatusCode >= 400") {
		t.Error("missing HTTP error status check")
	}

	// Should read error response body
	if !strings.Contains(codeStr, "io.ReadAll(httpResp.Body)") {
		t.Error("missing error body reading")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestClient_SimplePathWithNoParams(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers: []SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/health",
				FuncName:    "HealthCheck",
				PackagePath: "example.com/app/health",
				PathParams:  []SerializedPathParam{},
				Request: &SerializedStructInfo{
					Name:    "HealthCheckRequest",
					Package: "example.com/app/health",
					Fields:  []SerializedFieldInfo{},
				},
				Response: &SerializedStructInfo{
					Name:    "HealthCheckResponse",
					Package: "example.com/app/health",
					Fields: []SerializedFieldInfo{
						{Name: "Status", Type: "string", JSONName: "status", Required: true},
					},
				},
			},
		},
		OutputPkg: "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Should have simple URL construction without strings.NewReplacer
	if strings.Contains(codeStr, "strings.NewReplacer") {
		t.Error("should not use strings.NewReplacer for paths without params")
	}

	// Should have direct URL concatenation
	if !strings.Contains(codeStr, `c.server.URL + "/health"`) {
		t.Error("missing direct URL concatenation")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}
