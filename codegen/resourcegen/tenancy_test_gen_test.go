package resourcegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGenerateTenancyTest_ValidGo(t *testing.T) {
	cfg := TenancyTestGenConfig{
		ModulePath:      "myapp",
		OutputPkg:       "api",
		Dialect:         "postgres",
		TestDatabaseURL: "postgres://localhost:5432/myapp_test?sslmode=disable",
		ScopeColumn:     "organization_id",
	}

	resource := ResourceInfo{
		PackagePath: "myapp/api/posts",
		PackageName: "posts",
		HasCreate:   true,
		HasGetOne:   true,
		HasList:     true,
		HasUpdate:   true,
		HasDelete:   true,
		RequireAuth: true,
		CreateHandler: &codegen.SerializedHandlerInfo{
			Method:      "POST",
			Path:        "/posts",
			FuncName:    "CreatePost",
			PackagePath: "myapp/api/posts",
			Request: &codegen.SerializedStructInfo{
				Name: "CreatePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Title", JSONName: "title", Type: "string", Required: true},
					{Name: "Body", JSONName: "body", Type: "string", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreatePostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicId", JSONName: "public_id", Type: "string"},
					{Name: "Title", JSONName: "title", Type: "string"},
				},
			},
		},
		GetOneHandler: &codegen.SerializedHandlerInfo{
			Method:      "GET",
			Path:        "/posts/:id",
			FuncName:    "GetPost",
			PackagePath: "myapp/api/posts",
			Request: &codegen.SerializedStructInfo{
				Name: "GetPostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", JSONName: "id", Type: "string"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "GetPostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PublicId", JSONName: "public_id", Type: "string"},
				},
			},
		},
		ListHandler: &codegen.SerializedHandlerInfo{
			Method:      "GET",
			Path:        "/posts",
			FuncName:    "ListPosts",
			PackagePath: "myapp/api/posts",
			Request: &codegen.SerializedStructInfo{
				Name: "ListPostsRequest",
			},
			Response: &codegen.SerializedStructInfo{
				Name: "ListPostsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Post"},
				},
			},
		},
		UpdateHandler: &codegen.SerializedHandlerInfo{
			Method:      "PATCH",
			Path:        "/posts/:id",
			FuncName:    "UpdatePost",
			PackagePath: "myapp/api/posts",
			Request: &codegen.SerializedStructInfo{
				Name: "UpdatePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", JSONName: "id", Type: "string"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "UpdatePostResponse",
			},
		},
		DeleteHandler: &codegen.SerializedHandlerInfo{
			Method:      "DELETE",
			Path:        "/posts/:id",
			FuncName:    "SoftDeletePost",
			PackagePath: "myapp/api/posts",
			Request: &codegen.SerializedStructInfo{
				Name: "SoftDeletePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", JSONName: "id", Type: "string"},
				},
			},
		},
	}

	result, err := GenerateTenancyTest(cfg, resource)
	if err != nil {
		t.Fatalf("GenerateTenancyTest failed: %v", err)
	}

	code := string(result)

	// Verify it's valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Verify it's in the spec package
	if !strings.Contains(code, "package spec") {
		t.Error("expected package spec")
	}

	// Verify the test function name
	if !strings.Contains(code, "func TestTenancyIsolation_Posts(t *testing.T)") {
		t.Error("expected TestTenancyIsolation_Posts function")
	}

	// Verify it imports the resource package
	if !strings.Contains(code, `"myapp/api/posts"`) {
		t.Error("expected resource package import")
	}

	// Verify it uses createTestUser for both users (same server)
	if !strings.Contains(code, "createTestUser(t, ts, \"user_a@test.com\"") {
		t.Error("expected createTestUser for user A")
	}
	if !strings.Contains(code, "createTestUser(t, ts, \"user_b@test.com\"") {
		t.Error("expected createTestUser for user B")
	}

	// Verify it uses real client method names
	if !strings.Contains(code, "clientA.CreatePost") {
		t.Error("expected clientA.CreatePost call")
	}
	if !strings.Contains(code, "clientA.ListPosts") {
		t.Error("expected clientA.ListPosts call")
	}
	if !strings.Contains(code, "clientB.ListPosts") {
		t.Error("expected clientB.ListPosts call")
	}
	if !strings.Contains(code, "clientB.GetPost") {
		t.Error("expected clientB.GetPost cross-org call")
	}
	if !strings.Contains(code, "clientB.SoftDeletePost") {
		t.Error("expected clientB.SoftDeletePost cross-org call")
	}

	// Verify cross-org isolation logic
	if !strings.Contains(code, "different org") {
		t.Error("expected different org verification")
	}
	if !strings.Contains(code, "cross-org") {
		t.Error("expected cross-org check")
	}
}

func TestGenerateTenancyTest_NotFullResource(t *testing.T) {
	cfg := TenancyTestGenConfig{
		ModulePath:  "myapp",
		OutputPkg:   "api",
		Dialect:     "postgres",
		ScopeColumn: "organization_id",
	}

	resource := ResourceInfo{
		PackagePath: "myapp/api/posts",
		PackageName: "posts",
		HasCreate:   true,
		HasGetOne:   true,
		// Missing List, Update, Delete
	}

	_, err := GenerateTenancyTest(cfg, resource)
	if err == nil {
		t.Error("expected error for non-full resource")
	}
}
