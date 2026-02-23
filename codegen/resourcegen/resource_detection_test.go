package resourcegen

import (
	"github.com/shipq/shipq/codegen"
)

import (
	"testing"
)

func TestDetectFullResources_EmptyHandlers(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{}
	resources := DetectFullResources(handlers)

	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestDetectFullResources_SinglePackageFullCRUD(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/users",
			FuncName:    "CreateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name: "CreateUserRequest",
			},
			Response: &codegen.SerializedStructInfo{
				Name: "UserResponse",
			},
		},
		{
			Method:      "GET",
			Path:        "/users/:id",
			FuncName:    "GetUser",
			PackagePath: "example.com/app/users",
			PathParams: []codegen.SerializedPathParam{
				{Name: "id", Position: 1},
			},
			Request: &codegen.SerializedStructInfo{
				Name:   "GetUserRequest",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}},
			},
			Response: &codegen.SerializedStructInfo{
				Name:   "UserResponse",
				Fields: []codegen.SerializedFieldInfo{{Name: "ID", JSONName: "id"}},
			},
		},
		{
			Method:      "GET",
			Path:        "/users",
			FuncName:    "ListUsers",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name: "ListUsersRequest",
			},
			Response: &codegen.SerializedStructInfo{
				Name: "ListUsersResponse",
			},
		},
		{
			Method:      "PUT",
			Path:        "/users/:id",
			FuncName:    "UpdateUser",
			PackagePath: "example.com/app/users",
			PathParams: []codegen.SerializedPathParam{
				{Name: "id", Position: 1},
			},
			Request: &codegen.SerializedStructInfo{
				Name: "UpdateUserRequest",
			},
			Response: &codegen.SerializedStructInfo{
				Name: "UserResponse",
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
				Name: "DeleteUserRequest",
			},
		},
	}

	resources := DetectFullResources(handlers)

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.PackagePath != "example.com/app/users" {
		t.Errorf("expected package path 'example.com/app/users', got %q", r.PackagePath)
	}
	if r.PackageName != "users" {
		t.Errorf("expected package name 'users', got %q", r.PackageName)
	}
	if !r.IsFullResource() {
		t.Error("expected IsFullResource() to be true")
	}
	if !r.HasCreate {
		t.Error("expected HasCreate to be true")
	}
	if !r.HasGetOne {
		t.Error("expected HasGetOne to be true")
	}
	if !r.HasList {
		t.Error("expected HasList to be true")
	}
	if !r.HasUpdate {
		t.Error("expected HasUpdate to be true")
	}
	if !r.HasDelete {
		t.Error("expected HasDelete to be true")
	}
}

func TestDetectFullResources_PartialCRUD(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/posts/:id",
			FuncName:    "GetPost",
			PackagePath: "example.com/app/posts",
			PathParams: []codegen.SerializedPathParam{
				{Name: "id", Position: 1},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts",
			FuncName:    "ListPosts",
			PackagePath: "example.com/app/posts",
			PathParams:  []codegen.SerializedPathParam{},
		},
	}

	resources := DetectFullResources(handlers)

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.IsFullResource() {
		t.Error("expected IsFullResource() to be false for partial CRUD")
	}
	if !r.HasGetOne {
		t.Error("expected HasGetOne to be true")
	}
	if !r.HasList {
		t.Error("expected HasList to be true")
	}
	if r.HasCreate {
		t.Error("expected HasCreate to be false")
	}
	if r.HasUpdate {
		t.Error("expected HasUpdate to be false")
	}
	if r.HasDelete {
		t.Error("expected HasDelete to be false")
	}
}

func TestDetectFullResources_MultiplePackages(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/users",
			FuncName:    "ListUsers",
			PackagePath: "example.com/app/users",
			PathParams:  []codegen.SerializedPathParam{},
		},
		{
			Method:      "GET",
			Path:        "/posts",
			FuncName:    "ListPosts",
			PackagePath: "example.com/app/posts",
			PathParams:  []codegen.SerializedPathParam{},
		},
	}

	resources := DetectFullResources(handlers)

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Results should be sorted by package path
	if resources[0].PackagePath != "example.com/app/posts" {
		t.Errorf("expected first resource to be 'posts', got %q", resources[0].PackagePath)
	}
	if resources[1].PackagePath != "example.com/app/users" {
		t.Errorf("expected second resource to be 'users', got %q", resources[1].PackagePath)
	}
}

func TestFilterFullResources(t *testing.T) {
	resources := []ResourceInfo{
		{
			PackagePath: "example.com/app/users",
			HasCreate:   true,
			HasGetOne:   true,
			HasList:     true,
			HasUpdate:   true,
			HasDelete:   true,
		},
		{
			PackagePath: "example.com/app/posts",
			HasCreate:   true,
			HasGetOne:   true,
			HasList:     true,
			HasUpdate:   false, // Missing update
			HasDelete:   true,
		},
		{
			PackagePath: "example.com/app/accounts",
			HasCreate:   true,
			HasGetOne:   true,
			HasList:     true,
			HasUpdate:   true,
			HasDelete:   true,
		},
	}

	full := FilterFullResources(resources)

	if len(full) != 2 {
		t.Fatalf("expected 2 full resources, got %d", len(full))
	}

	// Check that the correct resources are included
	paths := make(map[string]bool)
	for _, r := range full {
		paths[r.PackagePath] = true
	}

	if !paths["example.com/app/users"] {
		t.Error("expected users to be in full resources")
	}
	if !paths["example.com/app/accounts"] {
		t.Error("expected accounts to be in full resources")
	}
	if paths["example.com/app/posts"] {
		t.Error("posts should not be in full resources (missing update)")
	}
}

func TestResourceInfo_IsFullResource(t *testing.T) {
	tests := []struct {
		name     string
		info     ResourceInfo
		wantFull bool
	}{
		{
			name: "all operations",
			info: ResourceInfo{
				HasCreate: true,
				HasGetOne: true,
				HasList:   true,
				HasUpdate: true,
				HasDelete: true,
			},
			wantFull: true,
		},
		{
			name: "missing create",
			info: ResourceInfo{
				HasCreate: false,
				HasGetOne: true,
				HasList:   true,
				HasUpdate: true,
				HasDelete: true,
			},
			wantFull: false,
		},
		{
			name: "missing get one",
			info: ResourceInfo{
				HasCreate: true,
				HasGetOne: false,
				HasList:   true,
				HasUpdate: true,
				HasDelete: true,
			},
			wantFull: false,
		},
		{
			name: "missing list",
			info: ResourceInfo{
				HasCreate: true,
				HasGetOne: true,
				HasList:   false,
				HasUpdate: true,
				HasDelete: true,
			},
			wantFull: false,
		},
		{
			name: "missing update",
			info: ResourceInfo{
				HasCreate: true,
				HasGetOne: true,
				HasList:   true,
				HasUpdate: false,
				HasDelete: true,
			},
			wantFull: false,
		},
		{
			name: "missing delete",
			info: ResourceInfo{
				HasCreate: true,
				HasGetOne: true,
				HasList:   true,
				HasUpdate: true,
				HasDelete: false,
			},
			wantFull: false,
		},
		{
			name:     "all missing",
			info:     ResourceInfo{},
			wantFull: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.IsFullResource(); got != tt.wantFull {
				t.Errorf("IsFullResource() = %v, want %v", got, tt.wantFull)
			}
		})
	}
}

func TestClassifyCRUDOperation_POST(t *testing.T) {
	// POST without path params = Create
	h := &codegen.SerializedHandlerInfo{
		Method:     "POST",
		Path:       "/users",
			PathParams: []codegen.SerializedPathParam{},
	}
	if got := classifyCRUDOperation(h); got != crudCreate {
		t.Errorf("expected crudCreate, got %v", got)
	}
}

func TestClassifyCRUDOperation_GET(t *testing.T) {
	// GET without path params = List
	listHandler := &codegen.SerializedHandlerInfo{
		Method:     "GET",
		Path:       "/users",
			PathParams: []codegen.SerializedPathParam{},
	}
	if got := classifyCRUDOperation(listHandler); got != crudList {
		t.Errorf("expected crudList, got %v", got)
	}

	// GET with path params = GetOne
	getOneHandler := &codegen.SerializedHandlerInfo{
		Method: "GET",
		Path:   "/users/:id",
			PathParams: []codegen.SerializedPathParam{
			{Name: "id", Position: 1},
		},
	}
	if got := classifyCRUDOperation(getOneHandler); got != crudGetOne {
		t.Errorf("expected crudGetOne, got %v", got)
	}
}

func TestClassifyCRUDOperation_PUT_PATCH(t *testing.T) {
	// PUT with path params = Update
	putHandler := &codegen.SerializedHandlerInfo{
		Method: "PUT",
		Path:   "/users/:id",
			PathParams: []codegen.SerializedPathParam{
			{Name: "id", Position: 1},
		},
	}
	if got := classifyCRUDOperation(putHandler); got != crudUpdate {
		t.Errorf("expected crudUpdate for PUT, got %v", got)
	}

	// PATCH with path params = Update
	patchHandler := &codegen.SerializedHandlerInfo{
		Method: "PATCH",
		Path:   "/users/:id",
			PathParams: []codegen.SerializedPathParam{
			{Name: "id", Position: 1},
		},
	}
	if got := classifyCRUDOperation(patchHandler); got != crudUpdate {
		t.Errorf("expected crudUpdate for PATCH, got %v", got)
	}
}

func TestClassifyCRUDOperation_DELETE(t *testing.T) {
	// DELETE with path params = Delete
	h := &codegen.SerializedHandlerInfo{
		Method: "DELETE",
		Path:   "/users/:id",
			PathParams: []codegen.SerializedPathParam{
			{Name: "id", Position: 1},
		},
	}
	if got := classifyCRUDOperation(h); got != crudDelete {
		t.Errorf("expected crudDelete, got %v", got)
	}
}

func TestHasResourceIDParam(t *testing.T) {
	tests := []struct {
		name string
		h    *codegen.SerializedHandlerInfo
		want bool
	}{
		{
			name: "id param",
			h: &codegen.SerializedHandlerInfo{
				PathParams: []codegen.SerializedPathParam{{Name: "id"}},
			},
			want: true,
		},
		{
			name: "public_id param",
			h: &codegen.SerializedHandlerInfo{
				PathParams: []codegen.SerializedPathParam{{Name: "public_id"}},
			},
			want: true,
		},
		{
			name: "user_id param",
			h: &codegen.SerializedHandlerInfo{
				PathParams: []codegen.SerializedPathParam{{Name: "user_id"}},
			},
			want: true,
		},
		{
			name: "no params",
			h: &codegen.SerializedHandlerInfo{
				PathParams: []codegen.SerializedPathParam{},
			},
			want: false,
		},
		{
			name: "slug param (still considered ID-like)",
			h: &codegen.SerializedHandlerInfo{
				PathParams: []codegen.SerializedPathParam{{Name: "slug"}},
			},
			want: true, // Any path param is considered resource identifier
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasResourceIDParam(tt.h); got != tt.want {
				t.Errorf("hasResourceIDParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		pkgPath string
		want    string
	}{
		{"example.com/app/users", "users"},
		{"example.com/app/api/resources/accounts", "accounts"},
		{"users", "users"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.pkgPath, func(t *testing.T) {
			if got := extractPackageName(tt.pkgPath); got != tt.want {
				t.Errorf("extractPackageName(%q) = %q, want %q", tt.pkgPath, got, tt.want)
			}
		})
	}
}

func TestGetResourceBasePath(t *testing.T) {
	tests := []struct {
		name string
		info ResourceInfo
		want string
	}{
		{
			name: "from list handler",
			info: ResourceInfo{
				ListHandler: &codegen.SerializedHandlerInfo{Path: "/users"},
			},
			want: "/users",
		},
		{
			name: "from create handler when no list",
			info: ResourceInfo{
				CreateHandler: &codegen.SerializedHandlerInfo{Path: "/users"},
			},
			want: "/users",
		},
		{
			name: "from get one handler",
			info: ResourceInfo{
				GetOneHandler: &codegen.SerializedHandlerInfo{Path: "/users/:id"},
			},
			want: "/users",
		},
		{
			name: "empty when no handlers",
			info: ResourceInfo{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetResourceBasePath(tt.info); got != tt.want {
				t.Errorf("GetResourceBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetResourceIDField(t *testing.T) {
	tests := []struct {
		name string
		info ResourceInfo
		want string
	}{
		{
			name: "id field",
			info: ResourceInfo{
				GetOneHandler: &codegen.SerializedHandlerInfo{
					Request: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "ID", JSONName: "id"},
						},
					},
				},
			},
			want: "ID",
		},
		{
			name: "public_id field",
			info: ResourceInfo{
				GetOneHandler: &codegen.SerializedHandlerInfo{
					Request: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "PublicID", JSONName: "public_id"},
						},
					},
				},
			},
			want: "PublicID",
		},
		{
			name: "default when no handler",
			info: ResourceInfo{},
			want: "PublicID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetResourceIDField(tt.info); got != tt.want {
				t.Errorf("GetResourceIDField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetResourceIDJSONName(t *testing.T) {
	tests := []struct {
		name string
		info ResourceInfo
		want string
	}{
		{
			name: "id json name",
			info: ResourceInfo{
				GetOneHandler: &codegen.SerializedHandlerInfo{
					Response: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "ID", JSONName: "id"},
						},
					},
				},
			},
			want: "id",
		},
		{
			name: "public_id json name",
			info: ResourceInfo{
				GetOneHandler: &codegen.SerializedHandlerInfo{
					Response: &codegen.SerializedStructInfo{
						Fields: []codegen.SerializedFieldInfo{
							{Name: "PublicID", JSONName: "public_id"},
						},
					},
				},
			},
			want: "public_id",
		},
		{
			name: "default when no handler",
			info: ResourceInfo{},
			want: "public_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetResourceIDJSONName(tt.info); got != tt.want {
				t.Errorf("GetResourceIDJSONName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectFullResources_HandlerReferences(t *testing.T) {
	createHandler := codegen.SerializedHandlerInfo{
		Method:      "POST",
		Path:        "/accounts",
		FuncName:    "CreateAccount",
		PackagePath: "example.com/app/accounts",
		PathParams:  []codegen.SerializedPathParam{},
	}
	getOneHandler := codegen.SerializedHandlerInfo{
		Method:      "GET",
		Path:        "/accounts/:public_id",
		FuncName:    "GetAccount",
		PackagePath: "example.com/app/accounts",
		PathParams:  []codegen.SerializedPathParam{{Name: "public_id", Position: 1}},
	}
	listHandler := codegen.SerializedHandlerInfo{
		Method:      "GET",
		Path:        "/accounts",
		FuncName:    "ListAccounts",
		PackagePath: "example.com/app/accounts",
		PathParams:  []codegen.SerializedPathParam{},
	}
	updateHandler := codegen.SerializedHandlerInfo{
		Method:      "PUT",
		Path:        "/accounts/:public_id",
		FuncName:    "UpdateAccount",
		PackagePath: "example.com/app/accounts",
		PathParams:  []codegen.SerializedPathParam{{Name: "public_id", Position: 1}},
	}
	deleteHandler := codegen.SerializedHandlerInfo{
		Method:      "DELETE",
		Path:        "/accounts/:public_id",
		FuncName:    "DeleteAccount",
		PackagePath: "example.com/app/accounts",
		PathParams:  []codegen.SerializedPathParam{{Name: "public_id", Position: 1}},
	}

	handlers := []codegen.SerializedHandlerInfo{
		createHandler,
		getOneHandler,
		listHandler,
		updateHandler,
		deleteHandler,
	}

	resources := DetectFullResources(handlers)

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify handler references are set correctly
	if r.CreateHandler == nil || r.CreateHandler.FuncName != "CreateAccount" {
		t.Error("CreateHandler not set correctly")
	}
	if r.GetOneHandler == nil || r.GetOneHandler.FuncName != "GetAccount" {
		t.Error("GetOneHandler not set correctly")
	}
	if r.ListHandler == nil || r.ListHandler.FuncName != "ListAccounts" {
		t.Error("ListHandler not set correctly")
	}
	if r.UpdateHandler == nil || r.UpdateHandler.FuncName != "UpdateAccount" {
		t.Error("UpdateHandler not set correctly")
	}
	if r.DeleteHandler == nil || r.DeleteHandler.FuncName != "DeleteAccount" {
		t.Error("DeleteHandler not set correctly")
	}
}
