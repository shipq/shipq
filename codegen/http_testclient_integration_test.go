package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestHTTPTestClient_Integration_GeneratesCompilableCode tests that the complete
// test client generation produces valid, compilable Go code.
func TestHTTPTestClient_Integration_GeneratesCompilableCode(t *testing.T) {
	// Create a realistic handler registry with all CRUD operations
	handlers := []SerializedHandlerInfo{
		// Create Account
		{
			Method:      "POST",
			Path:        "/accounts",
			FuncName:    "CreateAccount",
			PackagePath: "example.com/app/api/resources/accounts",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name:    "CreateAccountRequest",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
					{Name: "Email", Type: "string", JSONName: "email", Required: true},
				},
			},
			Response: &SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
					{Name: "Email", Type: "string", JSONName: "email"},
					{Name: "CreatedAt", Type: "time.Time", JSONName: "created_at"},
				},
			},
		},
		// Get Account
		{
			Method:      "GET",
			Path:        "/accounts/:public_id",
			FuncName:    "GetAccount",
			PackagePath: "example.com/app/api/resources/accounts",
			PathParams: []SerializedPathParam{
				{Name: "public_id", Position: 1},
			},
			Request: &SerializedStructInfo{
				Name:    "GetAccountRequest",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
				},
			},
			Response: &SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		// List Accounts
		{
			Method:      "GET",
			Path:        "/accounts",
			FuncName:    "ListAccounts",
			PackagePath: "example.com/app/api/resources/accounts",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name:    "ListAccountsRequest",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "Limit", Type: "int", JSONName: "limit"},
					{Name: "Offset", Type: "int", JSONName: "offset"},
				},
			},
			Response: &SerializedStructInfo{
				Name:    "ListAccountsResponse",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "Items", Type: "[]AccountResponse", JSONName: "items"},
					{Name: "Total", Type: "int", JSONName: "total"},
				},
			},
		},
		// Update Account
		{
			Method:      "PUT",
			Path:        "/accounts/:public_id",
			FuncName:    "UpdateAccount",
			PackagePath: "example.com/app/api/resources/accounts",
			PathParams: []SerializedPathParam{
				{Name: "public_id", Position: 1},
			},
			Request: &SerializedStructInfo{
				Name:    "UpdateAccountRequest",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
			Response: &SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		// Delete Account
		{
			Method:      "DELETE",
			Path:        "/accounts/:public_id",
			FuncName:    "DeleteAccount",
			PackagePath: "example.com/app/api/resources/accounts",
			PathParams: []SerializedPathParam{
				{Name: "public_id", Position: 1},
			},
			Request: &SerializedStructInfo{
				Name:    "DeleteAccountRequest",
				Package: "example.com/app/api/resources/accounts",
				Fields: []SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
				},
			},
			Response: nil, // DELETE typically returns no body
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Verify the code is valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "testclient.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}

	// Verify all CRUD methods are present
	expectedMethods := []string{
		"func (c *Client) CreateAccount",
		"func (c *Client) GetAccount",
		"func (c *Client) ListAccounts",
		"func (c *Client) UpdateAccount",
		"func (c *Client) DeleteAccount",
	}

	for _, method := range expectedMethods {
		if !strings.Contains(codeStr, method) {
			t.Errorf("missing method: %s", method)
		}
	}

	// Verify package imports
	expectedImports := []string{
		`"bytes"`,
		`"context"`,
		`"encoding/json"`,
		`"fmt"`,
		`"io"`,
		`"net/http"`,
		`"net/http/httptest"`,
		`"strings"`,
		`"example.com/app/api/resources/accounts"`,
	}

	for _, imp := range expectedImports {
		if !strings.Contains(codeStr, imp) {
			t.Errorf("missing import: %s", imp)
		}
	}
}

// TestHTTPTestClient_Integration_WithHarness tests that test client and harness
// work together correctly.
func TestHTTPTestClient_Integration_WithHarness(t *testing.T) {
	handlers := []SerializedHandlerInfo{
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
					{Name: "Status", Type: "string", JSONName: "status"},
				},
			},
		},
	}

	// Generate test client
	clientCfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	clientCode, err := GenerateHTTPTestClient(clientCfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Generate test harness
	harnessCfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	harnessCode, err := GenerateHTTPTestHarness(harnessCfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	// Verify both are valid Go
	fset := token.NewFileSet()

	_, err = parser.ParseFile(fset, "testclient.go", clientCode, parser.AllErrors)
	if err != nil {
		t.Errorf("test client code is not valid Go: %v", err)
	}

	_, err = parser.ParseFile(fset, "testharness.go", harnessCode, parser.AllErrors)
	if err != nil {
		t.Errorf("test harness code is not valid Go: %v", err)
	}

	// Verify they use compatible types
	clientStr := string(clientCode)
	harnessStr := string(harnessCode)

	// Client should accept *httptest.Server
	if !strings.Contains(clientStr, "*httptest.Server") {
		t.Error("test client should accept *httptest.Server")
	}

	// Harness should embed *httptest.Server
	if !strings.Contains(harnessStr, "*httptest.Server") {
		t.Error("test harness should embed *httptest.Server")
	}

	// Both should use the same package
	if !strings.Contains(clientStr, "package api") {
		t.Error("test client should be in api package")
	}
	if !strings.Contains(harnessStr, "package api") {
		t.Error("test harness should be in api package")
	}
}

// TestHTTPTestClient_Integration_ResourceDetection tests the full resource
// detection and test generation pipeline.
func TestHTTPTestClient_Integration_ResourceDetection(t *testing.T) {
	// Create handlers for a full resource
	handlers := []SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/users",
			FuncName:    "CreateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name: "CreateUserRequest",
				Fields: []SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
				},
			},
			Response: &SerializedStructInfo{
				Name: "UserResponse",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/users/:id",
			FuncName:    "GetUser",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{{Name: "id", Position: 1}},
			Request: &SerializedStructInfo{
				Name: "GetUserRequest",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
			Response: &SerializedStructInfo{
				Name: "UserResponse",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/users",
			FuncName:    "ListUsers",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name:   "ListUsersRequest",
				Fields: []SerializedFieldInfo{},
			},
			Response: &SerializedStructInfo{
				Name: "ListUsersResponse",
				Fields: []SerializedFieldInfo{
					{Name: "Items", Type: "[]UserResponse", JSONName: "items"},
				},
			},
		},
		{
			Method:      "PUT",
			Path:        "/users/:id",
			FuncName:    "UpdateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{{Name: "id", Position: 1}},
			Request: &SerializedStructInfo{
				Name: "UpdateUserRequest",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
			Response: &SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []SerializedFieldInfo{},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/users/:id",
			FuncName:    "DeleteUser",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{{Name: "id", Position: 1}},
			Request: &SerializedStructInfo{
				Name: "DeleteUserRequest",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
			Response: nil,
		},
		// Add a partial resource (only GET)
		{
			Method:      "GET",
			Path:        "/health",
			FuncName:    "HealthCheck",
			PackagePath: "example.com/app/health",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name:   "HealthCheckRequest",
				Fields: []SerializedFieldInfo{},
			},
			Response: &SerializedStructInfo{
				Name: "HealthCheckResponse",
				Fields: []SerializedFieldInfo{
					{Name: "Status", Type: "string", JSONName: "status"},
				},
			},
		},
	}

	// Detect resources
	resources := DetectFullResources(handlers)

	// Should detect 2 packages
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Filter to full resources
	fullResources := FilterFullResources(resources)

	// Should have exactly 1 full resource (users)
	if len(fullResources) != 1 {
		t.Fatalf("expected 1 full resource, got %d", len(fullResources))
	}

	// Verify it's the users resource
	if fullResources[0].PackageName != "users" {
		t.Errorf("expected users resource, got %s", fullResources[0].PackageName)
	}

	// Generate test for the full resource
	testCfg := ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	testCode, err := GenerateResourceTest(testCfg, fullResources[0])
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	// Verify test code is valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "users_test.go", testCode, parser.AllErrors)
	if err != nil {
		t.Errorf("generated test code is not valid Go: %v\n%s", err, string(testCode))
	}

	testStr := string(testCode)

	// Verify test package name
	if !strings.Contains(testStr, "package users_test") {
		t.Error("test should be in users_test package")
	}

	// Verify test function
	if !strings.Contains(testStr, "func TestResource_User_CRUD") {
		t.Error("test should have CRUD test function")
	}
}

// TestHTTPTestClient_Integration_MultiplePackages tests handling of handlers
// from multiple packages.
func TestHTTPTestClient_Integration_MultiplePackages(t *testing.T) {
	handlers := []SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/users/:id",
			FuncName:    "GetUser",
			PackagePath: "example.com/app/api/users",
			PathParams:  []SerializedPathParam{{Name: "id", Position: 1}},
			Request: &SerializedStructInfo{
				Name: "GetUserRequest",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id"},
				},
			},
			Response: &SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []SerializedFieldInfo{},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts/:id",
			FuncName:    "GetPost",
			PackagePath: "example.com/app/api/posts",
			PathParams:  []SerializedPathParam{{Name: "id", Position: 1}},
			Request: &SerializedStructInfo{
				Name: "GetPostRequest",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id"},
				},
			},
			Response: &SerializedStructInfo{
				Name:   "PostResponse",
				Fields: []SerializedFieldInfo{},
			},
		},
		{
			Method:      "GET",
			Path:        "/comments/:id",
			FuncName:    "GetComment",
			PackagePath: "example.com/app/api/comments",
			PathParams:  []SerializedPathParam{{Name: "id", Position: 1}},
			Request: &SerializedStructInfo{
				Name: "GetCommentRequest",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id"},
				},
			},
			Response: &SerializedStructInfo{
				Name:   "CommentResponse",
				Fields: []SerializedFieldInfo{},
			},
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Verify all packages are imported
	expectedImports := []string{
		`"example.com/app/api/users"`,
		`"example.com/app/api/posts"`,
		`"example.com/app/api/comments"`,
	}

	for _, imp := range expectedImports {
		if !strings.Contains(codeStr, imp) {
			t.Errorf("missing import: %s", imp)
		}
	}

	// Verify all methods are present with correct package prefixes
	expectedCalls := []string{
		"users.GetUserRequest",
		"posts.GetPostRequest",
		"comments.GetCommentRequest",
	}

	for _, call := range expectedCalls {
		if !strings.Contains(codeStr, call) {
			t.Errorf("missing package-qualified type: %s", call)
		}
	}

	// Verify code is valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "testclient.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestHTTPTestClient_Integration_PathParamTypes tests handling of different
// path parameter types.
func TestHTTPTestClient_Integration_PathParamTypes(t *testing.T) {
	handlers := []SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/users/:user_id/posts/:post_id",
			FuncName:    "GetUserPost",
			PackagePath: "example.com/app/api/posts",
			PathParams: []SerializedPathParam{
				{Name: "user_id", Position: 1},
				{Name: "post_id", Position: 3},
			},
			Request: &SerializedStructInfo{
				Name: "GetUserPostRequest",
				Fields: []SerializedFieldInfo{
					{Name: "UserID", Type: "string", JSONName: "user_id"},
					{Name: "PostID", Type: "string", JSONName: "post_id"},
				},
			},
			Response: &SerializedStructInfo{
				Name:   "PostResponse",
				Fields: []SerializedFieldInfo{},
			},
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	codeStr := string(code)

	// Verify path parameter substitution uses strings.NewReplacer
	if !strings.Contains(codeStr, "strings.NewReplacer") {
		t.Error("should use strings.NewReplacer for multiple path params")
	}

	// Verify both parameters are substituted
	if !strings.Contains(codeStr, `"{user_id}"`) {
		t.Error("should substitute user_id parameter")
	}
	if !strings.Contains(codeStr, `"{post_id}"`) {
		t.Error("should substitute post_id parameter")
	}

	// Verify code is valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "testclient.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestHTTPTestClient_Integration_EmptyHandlers tests that empty handler list
// produces valid code.
func TestHTTPTestClient_Integration_EmptyHandlers(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	code, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Verify code is valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "testclient.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, string(code))
	}

	// Should still have Client struct and constructor
	codeStr := string(code)
	if !strings.Contains(codeStr, "type Client struct") {
		t.Error("should have Client struct")
	}
	if !strings.Contains(codeStr, "func NewUnauthenticatedTestClient") {
		t.Error("should have constructor")
	}
}
