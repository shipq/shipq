package resourcegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGenerateResourceTest_FullResource(t *testing.T) {
	cfg := ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	resource := ResourceInfo{
		PackagePath: "example.com/app/api/resources/accounts",
		PackageName: "accounts",
		HasCreate:   true,
		HasGetOne:   true,
		HasList:     true,
		HasUpdate:   true,
		HasDelete:   true,
		CreateHandler: &codegen.SerializedHandlerInfo{
			Method:   "POST",
			Path:     "/accounts",
			FuncName: "CreateAccount",
			Request: &codegen.SerializedStructInfo{
				Name: "CreateAccountRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "AccountResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		GetOneHandler: &codegen.SerializedHandlerInfo{
			Method:   "GET",
			Path:     "/accounts/:public_id",
			FuncName: "GetAccount",
			PathParams: []codegen.SerializedPathParam{
				{Name: "public_id", Position: 1},
			},
			Request: &codegen.SerializedStructInfo{
				Name: "GetAccountRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "AccountResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		ListHandler: &codegen.SerializedHandlerInfo{
			Method:   "GET",
			Path:     "/accounts",
			FuncName: "ListAccounts",
			Request: &codegen.SerializedStructInfo{
				Name:   "ListAccountsRequest",
				Fields: []codegen.SerializedFieldInfo{},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "ListAccountsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]AccountResponse", JSONName: "items"},
				},
			},
		},
		UpdateHandler: &codegen.SerializedHandlerInfo{
			Method:   "PUT",
			Path:     "/accounts/:public_id",
			FuncName: "UpdateAccount",
			PathParams: []codegen.SerializedPathParam{
				{Name: "public_id", Position: 1},
			},
			Request: &codegen.SerializedStructInfo{
				Name: "UpdateAccountRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "AccountResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id"},
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
		},
		DeleteHandler: &codegen.SerializedHandlerInfo{
			Method:   "DELETE",
			Path:     "/accounts/:public_id",
			FuncName: "DeleteAccount",
			PathParams: []codegen.SerializedPathParam{
				{Name: "public_id", Position: 1},
			},
			Request: &codegen.SerializedStructInfo{
				Name: "DeleteAccountRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", Type: "string", JSONName: "public_id", Required: true},
				},
			},
		},
	}

	code, err := GenerateResourceTest(cfg, resource)
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	codeStr := string(code)

	// Should have test package declaration
	if !strings.Contains(codeStr, "package accounts_test") {
		t.Error("missing test package declaration")
	}

	// Should have test function
	if !strings.Contains(codeStr, "func TestResource_Account_CRUD") {
		t.Error("missing CRUD test function")
	}

	// Should import required packages
	if !strings.Contains(codeStr, `"context"`) {
		t.Error("missing context import")
	}
	if !strings.Contains(codeStr, `"testing"`) {
		t.Error("missing testing import")
	}
	if !strings.Contains(codeStr, `"example.com/app/api"`) {
		t.Error("missing api package import")
	}
	if !strings.Contains(codeStr, `"example.com/app/api/resources/accounts"`) {
		t.Error("missing accounts package import")
	}

	// Should have Create section
	if !strings.Contains(codeStr, "client.CreateAccount") {
		t.Error("missing CreateAccount call")
	}

	// Should have GetOne section
	if !strings.Contains(codeStr, "client.GetAccount") {
		t.Error("missing GetAccount call")
	}

	// Should have Update section
	if !strings.Contains(codeStr, "client.UpdateAccount") {
		t.Error("missing UpdateAccount call")
	}

	// Should have List section
	if !strings.Contains(codeStr, "client.ListAccounts") {
		t.Error("missing ListAccounts call")
	}

	// Should have Delete section
	if !strings.Contains(codeStr, "client.DeleteAccount") {
		t.Error("missing DeleteAccount call")
	}

	// Should check for 404 after delete
	if !strings.Contains(codeStr, "Expected 404 after delete") {
		t.Error("missing 404 check after delete")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateResourceTest_NotFullResource(t *testing.T) {
	cfg := ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	resource := ResourceInfo{
		PackagePath: "example.com/app/posts",
		PackageName: "posts",
		HasCreate:   true,
		HasGetOne:   true,
		HasList:     true,
		HasUpdate:   false, // Missing Update
		HasDelete:   true,
	}

	_, err := GenerateResourceTest(cfg, resource)
	if err == nil {
		t.Error("expected error for non-full resource, got nil")
	}
	if !strings.Contains(err.Error(), "not a full resource") {
		t.Errorf("expected 'not a full resource' error, got: %v", err)
	}
}

func TestGenerateResourceTest_TestServerSetup(t *testing.T) {
	cfg := ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	resource := createFullResourceInfo()

	code, err := GenerateResourceTest(cfg, resource)
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	codeStr := string(code)

	// Should create test server
	if !strings.Contains(codeStr, "NewUnauthenticatedTestServer") {
		t.Error("missing NewUnauthenticatedTestServer call")
	}

	// Should create test client
	if !strings.Contains(codeStr, "NewUnauthenticatedTestClient") {
		t.Error("missing NewUnauthenticatedTestClient call")
	}
}

func TestGenerateResourceTest_DatabaseSetup(t *testing.T) {
	cfg := ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	resource := createFullResourceInfo()

	code, err := GenerateResourceTest(cfg, resource)
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	codeStr := string(code)

	// Should get DATABASE_URL from environment
	if !strings.Contains(codeStr, "DATABASE_URL") {
		t.Error("missing DATABASE_URL environment variable")
	}

	// Should skip if DATABASE_URL not set
	if !strings.Contains(codeStr, "t.Skip") {
		t.Error("missing t.Skip for missing DATABASE_URL")
	}

	// Should close database connection
	if !strings.Contains(codeStr, "defer db.Close()") {
		t.Error("missing defer db.Close()")
	}
}

func TestGenerateResourceTest_CRUDFlow(t *testing.T) {
	cfg := ResourceTestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
	}

	resource := createFullResourceInfo()

	code, err := GenerateResourceTest(cfg, resource)
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	codeStr := string(code)

	// Verify the CRUD sections are present with comments
	expectedSections := []string{
		"// Create",
		"// GetOne",
		"// Update",
		"// List (should include our record)",
		"// Delete (soft delete)",
		"// GetOne after delete => 404",
		"// List after delete (should NOT include deleted record)",
	}

	for _, section := range expectedSections {
		if !strings.Contains(codeStr, section) {
			t.Errorf("missing section: %s", section)
		}
	}
}

func TestIsIDField(t *testing.T) {
	tests := []struct {
		jsonName string
		want     bool
	}{
		{"id", true},
		{"ID", true},
		{"public_id", true},
		{"PUBLIC_ID", true},
		{"publicid", true},
		{"user_id", true},
		{"account_id", true},
		{"name", false},
		{"email", false},
		{"created_at", false},
	}

	for _, tt := range tests {
		t.Run(tt.jsonName, func(t *testing.T) {
			if got := isIDField(tt.jsonName); got != tt.want {
				t.Errorf("isIDField(%q) = %v, want %v", tt.jsonName, got, tt.want)
			}
		})
	}
}

func TestGetSampleValue(t *testing.T) {
	tests := []struct {
		goType    string
		fieldName string
		want      string
	}{
		{"string", "Name", `"test_name"`},
		{"string", "Email", `"test_email"`},
		{"int", "Count", "1"},
		{"int32", "Age", "1"},
		{"int64", "ID", "1"},
		{"uint", "Size", "1"},
		{"float64", "Price", "1.0"},
		{"bool", "Active", "true"},
		{"*string", "OptionalName", "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.goType+"_"+tt.fieldName, func(t *testing.T) {
			got := getSampleValue(tt.goType, tt.fieldName)
			if got != tt.want {
				t.Errorf("getSampleValue(%q, %q) = %q, want %q", tt.goType, tt.fieldName, got, tt.want)
			}
		})
	}
}

func TestFindResponseIDField(t *testing.T) {
	tests := []struct {
		name       string
		response   *codegen.SerializedStructInfo
		idJSONName string
		want       string
	}{
		{
			name:       "nil response",
			response:   nil,
			idJSONName: "public_id",
			want:       "",
		},
		{
			name: "finds public_id",
			response: &codegen.SerializedStructInfo{
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicID", JSONName: "public_id"},
					{Name: "Name", JSONName: "name"},
				},
			},
			idJSONName: "public_id",
			want:       "PublicID",
		},
		{
			name: "finds ID",
			response: &codegen.SerializedStructInfo{
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", JSONName: "id"},
					{Name: "Name", JSONName: "name"},
				},
			},
			idJSONName: "id",
			want:       "ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findResponseIDField(tt.response, tt.idJSONName)
			if got != tt.want {
				t.Errorf("findResponseIDField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindListFieldName(t *testing.T) {
	tests := []struct {
		name     string
		response *codegen.SerializedStructInfo
		want     string
	}{
		{
			name:     "nil response",
			response: nil,
			want:     "",
		},
		{
			name: "finds Items",
			response: &codegen.SerializedStructInfo{
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Account", JSONName: "items"},
					{Name: "Total", Type: "int", JSONName: "total"},
				},
			},
			want: "Items",
		},
		{
			name: "finds Data",
			response: &codegen.SerializedStructInfo{
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Data", Type: "[]User", JSONName: "data"},
				},
			},
			want: "Data",
		},
		{
			name: "finds slice field",
			response: &codegen.SerializedStructInfo{
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Accounts", Type: "[]AccountResponse", JSONName: "accounts"},
				},
			},
			want: "Accounts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findListFieldName(tt.response)
			if got != tt.want {
				t.Errorf("findListFieldName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateResourceTest_DifferentPackageNames(t *testing.T) {
	cfg := ResourceTestGenConfig{
		ModulePath: "mycompany.io/myapp",
		OutputPkg:  "server",
	}

	resource := ResourceInfo{
		PackagePath: "mycompany.io/myapp/resources/users",
		PackageName: "users",
		HasCreate:   true,
		HasGetOne:   true,
		HasList:     true,
		HasUpdate:   true,
		HasDelete:   true,
		CreateHandler: &codegen.SerializedHandlerInfo{
			FuncName: "CreateUser",
			Request:  &codegen.SerializedStructInfo{Name: "CreateUserRequest"},
			Response: &codegen.SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}},
			},
		},
		GetOneHandler: &codegen.SerializedHandlerInfo{
			FuncName: "GetUser",
			Request:  &codegen.SerializedStructInfo{Name: "GetUserRequest", Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}}},
			Response: &codegen.SerializedStructInfo{Name: "UserResponse", Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}}},
		},
		ListHandler: &codegen.SerializedHandlerInfo{
			FuncName: "ListUsers",
			Request:  &codegen.SerializedStructInfo{Name: "ListUsersRequest"},
			Response: &codegen.SerializedStructInfo{Name: "ListUsersResponse", Fields: []codegen.SerializedFieldInfo{{Name: "Items", Type: "[]User"}}},
		},
		UpdateHandler: &codegen.SerializedHandlerInfo{
			FuncName: "UpdateUser",
			Request:  &codegen.SerializedStructInfo{Name: "UpdateUserRequest", Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}}},
			Response: &codegen.SerializedStructInfo{Name: "UserResponse"},
		},
		DeleteHandler: &codegen.SerializedHandlerInfo{
			FuncName: "DeleteUser",
			Request:  &codegen.SerializedStructInfo{Name: "DeleteUserRequest", Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}}},
		},
	}

	code, err := GenerateResourceTest(cfg, resource)
	if err != nil {
		t.Fatalf("GenerateResourceTest() error = %v", err)
	}

	codeStr := string(code)

	// Should have correct package declaration
	if !strings.Contains(codeStr, "package users_test") {
		t.Error("missing users_test package declaration")
	}

	// Should import the correct API package
	if !strings.Contains(codeStr, `"mycompany.io/myapp/server"`) {
		t.Error("missing server package import")
	}

	// Should use server as package name for test helpers
	if !strings.Contains(codeStr, "server.NewUnauthenticatedTestServer") {
		t.Error("missing server.NewUnauthenticatedTestServer call")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

// Helper function to create a full resource for tests
func createFullResourceInfo() ResourceInfo {
	return ResourceInfo{
		PackagePath: "example.com/app/accounts",
		PackageName: "accounts",
		HasCreate:   true,
		HasGetOne:   true,
		HasList:     true,
		HasUpdate:   true,
		HasDelete:   true,
		CreateHandler: &codegen.SerializedHandlerInfo{
			FuncName: "CreateAccount",
			Request:  &codegen.SerializedStructInfo{Name: "CreateAccountRequest"},
			Response: &codegen.SerializedStructInfo{
				Name:   "AccountResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "PublicID", JSONName: "public_id"}},
			},
		},
		GetOneHandler: &codegen.SerializedHandlerInfo{
			FuncName: "GetAccount",
			Request:  &codegen.SerializedStructInfo{Name: "GetAccountRequest", Fields: []codegen.SerializedFieldInfo{{Name: "PublicID", JSONName: "public_id"}}},
			Response: &codegen.SerializedStructInfo{Name: "AccountResponse", Fields: []codegen.SerializedFieldInfo{{Name: "PublicID", JSONName: "public_id"}}},
		},
		ListHandler: &codegen.SerializedHandlerInfo{
			FuncName: "ListAccounts",
			Request:  &codegen.SerializedStructInfo{Name: "ListAccountsRequest"},
			Response: &codegen.SerializedStructInfo{Name: "ListAccountsResponse", Fields: []codegen.SerializedFieldInfo{{Name: "Items", Type: "[]Account"}}},
		},
		UpdateHandler: &codegen.SerializedHandlerInfo{
			FuncName: "UpdateAccount",
			Request:  &codegen.SerializedStructInfo{Name: "UpdateAccountRequest", Fields: []codegen.SerializedFieldInfo{{Name: "PublicID", JSONName: "public_id"}}},
			Response: &codegen.SerializedStructInfo{Name: "AccountResponse"},
		},
		DeleteHandler: &codegen.SerializedHandlerInfo{
			FuncName: "DeleteAccount",
			Request:  &codegen.SerializedStructInfo{Name: "DeleteAccountRequest", Fields: []codegen.SerializedFieldInfo{{Name: "PublicID", JSONName: "public_id"}}},
		},
	}
}
