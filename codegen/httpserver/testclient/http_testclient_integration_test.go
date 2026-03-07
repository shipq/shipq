package testclient

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/resourcegen"
)

// TestHTTPTestClient_Integration_GeneratesCompilableCode tests that the complete
// test client generation produces valid, compilable Go code.
func TestHTTPTestClient_Integration_GeneratesCompilableCode(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/accounts",
			FuncName:    "CreateAccount",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "CreateAccountRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
					{Name: "Email", Type: "string", JSONName: "email", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
					{Name: "Email", Type: "string", JSONName: "email"},
					{Name: "CreatedAt", Type: "time.Time", JSONName: "created_at"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/accounts/:public_id",
			FuncName:    "GetAccount",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{{Name: "public_id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:    "GetAccountRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/accounts",
			FuncName:    "ListAccounts",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "ListAccountsRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Limit", Type: "int", JSONName: "limit"},
					{Name: "Offset", Type: "int", JSONName: "offset"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "ListAccountsResponse",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]AccountResponse", JSONName: "items"},
					{Name: "Total", Type: "int", JSONName: "total"},
				},
			},
		},
		{
			Method:      "PUT",
			Path:        "/accounts/:public_id",
			FuncName:    "UpdateAccount",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{{Name: "public_id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:    "UpdateAccountRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/accounts/:public_id",
			FuncName:    "DeleteAccount",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{{Name: "public_id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:    "DeleteAccountRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
				},
			},
			Response: nil,
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Verify all files are valid Go
	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}

	// Verify all CRUD methods are present in resource test client
	all := allContent(files)
	expectedMethods := []string{
		"func (c *AccountsTestClient) CreateAccount",
		"func (c *AccountsTestClient) GetAccount",
		"func (c *AccountsTestClient) ListAccounts",
		"func (c *AccountsTestClient) UpdateAccount",
		"func (c *AccountsTestClient) DeleteAccount",
	}

	for _, method := range expectedMethods {
		if !strings.Contains(all, method) {
			t.Errorf("missing method: %s", method)
		}
	}

	// Verify httputil import
	if !strings.Contains(all, "httputil") {
		t.Error("missing httputil import")
	}
}

// TestHTTPTestClient_Integration_WithHarness tests that test client and harness
// work together correctly.
func TestHTTPTestClient_Integration_WithHarness(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
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
					{Name: "Status", Type: "string", JSONName: "status"},
				},
			},
		},
	}

	clientCfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	clientFiles, err := GenerateHTTPTestClient(clientCfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	harnessCfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	harnessCode, err := GenerateHTTPTestHarness(harnessCfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	fset := token.NewFileSet()

	// All client files should be valid Go
	for _, f := range clientFiles {
		_, err = parser.ParseFile(fset, "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v", f.RelPath, err)
		}
	}

	_, err = parser.ParseFile(fset, "testharness.go", harnessCode, parser.AllErrors)
	if err != nil {
		t.Errorf("test harness code is not valid Go: %v", err)
	}

	// Top-level client and harness should use same package
	topLevel := findTopLevelTestClient(clientFiles)
	if topLevel == nil {
		t.Fatal("missing top-level test client")
	}
	clientStr := string(topLevel.Content)
	harnessStr := string(harnessCode)

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
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/users",
			FuncName:    "CreateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:   "CreateUserRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "Name", Type: "string", JSONName: "name", Required: true}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id"}, {Name: "Name", Type: "string", JSONName: "name"}},
			},
		},
		{
			Method:      "GET",
			Path:        "/users/:id",
			FuncName:    "GetUser",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:   "GetUserRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id", Required: true}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id"}},
			},
		},
		{
			Method:      "GET",
			Path:        "/users",
			FuncName:    "ListUsers",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:   "ListUsersRequest",
				Fields: []codegen.SerializedFieldInfo{},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "ListUsersResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "Items", Type: "[]UserResponse", JSONName: "items"}},
			},
		},
		{
			Method:      "PUT",
			Path:        "/users/:id",
			FuncName:    "UpdateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:   "UpdateUserRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id", Required: true}, {Name: "Name", Type: "string", JSONName: "name"}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []codegen.SerializedFieldInfo{},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/users/:id",
			FuncName:    "DeleteUser",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:   "DeleteUserRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id", Required: true}},
			},
			Response: nil,
		},
		{
			Method:      "GET",
			Path:        "/health",
			FuncName:    "HealthCheck",
			PackagePath: "example.com/app/health",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:   "HealthCheckRequest",
				Fields: []codegen.SerializedFieldInfo{},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "HealthCheckResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "Status", Type: "string", JSONName: "status"}},
			},
		},
	}

	resources := resourcegen.DetectFullResources(handlers)
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	fullResources := resourcegen.FilterFullResources(resources)
	if len(fullResources) != 1 {
		t.Fatalf("expected 1 full resource, got %d", len(fullResources))
	}

	if fullResources[0].PackageName != "users" {
		t.Errorf("expected users resource, got %s", fullResources[0].PackageName)
	}

	testCfg := resourcegen.ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	testCode, err := resourcegen.GenerateResourceTest(testCfg, fullResources[0])
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "users_test.go", testCode, parser.AllErrors)
	if err != nil {
		t.Errorf("generated test code is not valid Go: %v\n%s", err, string(testCode))
	}

	testStr := string(testCode)
	if !strings.Contains(testStr, "package spec") {
		t.Error("test should be in spec package")
	}
	if !strings.Contains(testStr, "func TestResource_User_CRUD") {
		t.Error("test should have CRUD test function")
	}
}

// TestHTTPTestClient_Integration_MultiplePackages tests handling of handlers
// from multiple packages.
func TestHTTPTestClient_Integration_MultiplePackages(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/users/:id",
			FuncName:    "GetUser",
			PackagePath: "example.com/app/api/users",
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:   "GetUserRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id"}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []codegen.SerializedFieldInfo{},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts/:id",
			FuncName:    "GetPost",
			PackagePath: "example.com/app/api/posts",
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:   "GetPostRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id"}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "PostResponse",
				Fields: []codegen.SerializedFieldInfo{},
			},
		},
		{
			Method:      "GET",
			Path:        "/comments/:id",
			FuncName:    "GetComment",
			PackagePath: "example.com/app/api/comments",
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
			Request: &codegen.SerializedStructInfo{
				Name:   "GetCommentRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", Type: "string", JSONName: "id"}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "CommentResponse",
				Fields: []codegen.SerializedFieldInfo{},
			},
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	// Verify all packages are imported across files
	expectedImports := []string{
		`"example.com/app/api/users"`,
		`"example.com/app/api/posts"`,
		`"example.com/app/api/comments"`,
	}
	for _, imp := range expectedImports {
		if !strings.Contains(all, imp) {
			t.Errorf("missing import: %s", imp)
		}
	}

	// Verify methods exist
	expectedCalls := []string{
		"users.GetUserRequest",
		"posts.GetPostRequest",
		"comments.GetCommentRequest",
	}
	for _, call := range expectedCalls {
		if !strings.Contains(all, call) {
			t.Errorf("missing package-qualified type: %s", call)
		}
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

// TestHTTPTestClient_Integration_PathParamTypes tests handling of different
// path parameter types.
func TestHTTPTestClient_Integration_PathParamTypes(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/users/:user_id/posts/:post_id",
			FuncName:    "GetUserPost",
			PackagePath: "example.com/app/api/posts",
			PathParams: []codegen.SerializedPathParam{
				{Name: "user_id", Position: 1},
				{Name: "post_id", Position: 3},
			},
			Request: &codegen.SerializedStructInfo{
				Name: "GetUserPostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "UserID", Type: "string", JSONName: "user_id"},
					{Name: "PostID", Type: "string", JSONName: "post_id"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "PostResponse",
				Fields: []codegen.SerializedFieldInfo{},
			},
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	all := allContent(files)

	if !strings.Contains(all, "strings.NewReplacer") {
		t.Error("should use strings.NewReplacer for multiple path params")
	}
	if !strings.Contains(all, `"{user_id}"`) {
		t.Error("should substitute user_id parameter")
	}
	if !strings.Contains(all, `"{post_id}"`) {
		t.Error("should substitute post_id parameter")
	}

	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}
}

// TestHTTPTestClient_Integration_EmptyHandlers tests that empty handler list
// produces valid code.
func TestHTTPTestClient_Integration_QueryParamCompilableCode(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/accounts",
			FuncName:    "CreateAccount",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "CreateAccountRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
					{Name: "Email", Type: "string", JSONName: "email", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "AccountResponse",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/accounts",
			FuncName:    "ListAccounts",
			PackagePath: "example.com/app/api/accounts",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "ListAccountsRequest",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Limit", Type: "int", JSONName: "limit", Required: false, Tags: map[string]string{"query": "limit"}},
					{Name: "Cursor", Type: "*string", JSONName: "cursor", Required: false, Tags: map[string]string{"query": "cursor"}},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "ListAccountsResponse",
				Package: "example.com/app/api/accounts",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]AccountResponse", JSONName: "items"},
					{Name: "NextCursor", Type: "*string", JSONName: "next_cursor"},
				},
			},
		},
	}

	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   handlers,
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	// Verify all files are valid Go
	for _, f := range files {
		_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
		if err != nil {
			t.Errorf("%s is not valid Go: %v\n%s", f.RelPath, err, string(f.Content))
		}
	}

	// Verify ListAccounts method exists
	all := allContent(files)
	if !strings.Contains(all, "func (c *AccountsTestClient) ListAccounts") {
		t.Error("missing ListAccounts method")
	}

	// Verify net/url import is present (needed for query params)
	resFile := findResourceTestClient(files, "accounts")
	if resFile == nil {
		t.Fatal("missing accounts resource test client file")
	}
	codeStr := string(resFile.Content)

	if !strings.Contains(codeStr, `"net/url"`) {
		t.Error("missing \"net/url\" import for query param support")
	}

	// Verify url.Values appears in the generated code
	if !strings.Contains(codeStr, "url.Values") {
		t.Error("missing url.Values in generated code for query params")
	}
}

func TestHTTPTestClient_Integration_EmptyHandlers(t *testing.T) {
	cfg := HTTPTestClientGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		OutputPkg:  "api",
	}

	files, err := GenerateHTTPTestClient(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestClient() error = %v", err)
	}

	topLevel := findTopLevelTestClient(files)
	if topLevel == nil {
		t.Fatal("missing top-level test client")
	}

	_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, string(topLevel.Content))
	}

	codeStr := string(topLevel.Content)
	if !strings.Contains(codeStr, "type Client struct") {
		t.Error("should have Client struct")
	}
	if !strings.Contains(codeStr, "func NewUnauthenticatedTestClient") {
		t.Error("should have constructor")
	}
}
