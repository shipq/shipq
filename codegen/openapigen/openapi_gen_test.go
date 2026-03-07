package openapigen

import (
	"encoding/json"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// parseSpec is a test helper that generates the spec and unmarshals it.
func parseSpec(t *testing.T, cfg OpenAPIGenConfig) map[string]any {
	t.Helper()
	raw, err := GenerateOpenAPISpec(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPISpec() error = %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("failed to unmarshal spec: %v", err)
	}
	return spec
}

func TestGenerateOpenAPISpec_BasicStructure(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
	}

	spec := parseSpec(t, cfg)

	if spec["openapi"] != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %v", spec["openapi"])
	}

	info, ok := spec["info"].(map[string]any)
	if !ok {
		t.Fatal("missing info object")
	}
	if info["title"] != "app" {
		t.Errorf("expected title 'app' (base of module path), got %v", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", info["version"])
	}
}

func TestGenerateOpenAPISpec_CustomTitleVersion(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers:   []codegen.SerializedHandlerInfo{},
		Title:      "My API",
		Version:    "2.0.0",
	}

	spec := parseSpec(t, cfg)

	info := spec["info"].(map[string]any)
	if info["title"] != "My API" {
		t.Errorf("expected title 'My API', got %v", info["title"])
	}
	if info["version"] != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %v", info["version"])
	}
}

func TestGenerateOpenAPISpec_SingleGetHandler(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths object")
	}

	// Path should be converted from :id to {id}
	pathItem, ok := paths["/users/{id}"].(map[string]any)
	if !ok {
		t.Fatal("missing /users/{id} path")
	}

	get, ok := pathItem["get"].(map[string]any)
	if !ok {
		t.Fatal("missing GET operation")
	}

	if get["operationId"] != "GetUser" {
		t.Errorf("expected operationId GetUser, got %v", get["operationId"])
	}

	// Check path parameters
	params, ok := get["parameters"].([]any)
	if !ok || len(params) != 1 {
		t.Fatalf("expected 1 path parameter, got %v", params)
	}

	param := params[0].(map[string]any)
	if param["name"] != "id" {
		t.Errorf("expected param name 'id', got %v", param["name"])
	}
	if param["in"] != "path" {
		t.Errorf("expected param in 'path', got %v", param["in"])
	}

	paramSchema := param["schema"].(map[string]any)
	if paramSchema["type"] != "integer" {
		t.Errorf("expected param type integer, got %v", paramSchema["type"])
	}
	if paramSchema["format"] != "int64" {
		t.Errorf("expected param format int64, got %v", paramSchema["format"])
	}

	// Check response
	responses := get["responses"].(map[string]any)
	resp200, ok := responses["200"].(map[string]any)
	if !ok {
		t.Fatal("missing 200 response")
	}
	content := resp200["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	if _, ok := props["id"]; !ok {
		t.Error("missing id property in response schema")
	}
	if _, ok := props["name"]; !ok {
		t.Error("missing name property in response schema")
	}
}

func TestGenerateOpenAPISpec_PostHandler_RequestBody(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
					},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths := spec["paths"].(map[string]any)
	pathItem := paths["/users"].(map[string]any)
	post := pathItem["post"].(map[string]any)

	// Should have request body
	reqBody, ok := post["requestBody"].(map[string]any)
	if !ok {
		t.Fatal("missing requestBody for POST handler")
	}

	if reqBody["required"] != true {
		t.Error("expected requestBody to be required")
	}

	content := reqBody["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	if _, ok := props["name"]; !ok {
		t.Error("missing name property in request body schema")
	}
	if _, ok := props["email"]; !ok {
		t.Error("missing email property in request body schema")
	}

	// Check required fields
	required := schema["required"].([]any)
	if len(required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(required))
	}

	// Should have 201 response for POST
	responses := post["responses"].(map[string]any)
	if _, ok := responses["201"]; !ok {
		t.Error("expected 201 response for POST handler")
	}
}

func TestGenerateOpenAPISpec_AuthHandlers(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/me",
				FuncName:    "Me",
				PackagePath: "example.com/app/api/auth",
				RequireAuth: true,
				Request: &codegen.SerializedStructInfo{
					Name:    "MeRequest",
					Package: "example.com/app/api/auth",
					Fields:  []codegen.SerializedFieldInfo{},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "MeResponse",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "UserID", Type: "int64", JSONName: "user_id", Required: true},
					},
				},
			},
			{
				Method:      "POST",
				Path:        "/login",
				FuncName:    "Login",
				PackagePath: "example.com/app/api/auth",
				RequireAuth: false,
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
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	// Auth route should have security requirement
	paths := spec["paths"].(map[string]any)
	mePath := paths["/me"].(map[string]any)
	meGet := mePath["get"].(map[string]any)

	security, ok := meGet["security"].([]any)
	if !ok || len(security) != 1 {
		t.Fatal("expected security requirement on auth route")
	}

	secItem := security[0].(map[string]any)
	if _, ok := secItem["cookieAuth"]; !ok {
		t.Error("expected cookieAuth security scheme")
	}

	// Auth route should have 401 response
	responses := meGet["responses"].(map[string]any)
	if _, ok := responses["401"]; !ok {
		t.Error("expected 401 response on auth route")
	}

	// Non-auth route should NOT have security
	loginPath := paths["/login"].(map[string]any)
	loginPost := loginPath["post"].(map[string]any)
	if _, ok := loginPost["security"]; ok {
		t.Error("non-auth route should not have security requirement")
	}

	// Components should have cookieAuth security scheme
	components := spec["components"].(map[string]any)
	schemes := components["securitySchemes"].(map[string]any)
	cookieAuth := schemes["cookieAuth"].(map[string]any)
	if cookieAuth["type"] != "apiKey" {
		t.Errorf("expected type apiKey, got %v", cookieAuth["type"])
	}
	if cookieAuth["in"] != "cookie" {
		t.Errorf("expected in cookie, got %v", cookieAuth["in"])
	}
	if cookieAuth["name"] != "session" {
		t.Errorf("expected name session, got %v", cookieAuth["name"])
	}
}

func TestGenerateOpenAPISpec_NoAuthNoCookieScheme(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/health",
				FuncName:    "Health",
				PackagePath: "example.com/app/api/health",
				RequireAuth: false,
			},
		},
	}

	spec := parseSpec(t, cfg)

	components := spec["components"].(map[string]any)
	if _, ok := components["securitySchemes"]; ok {
		t.Error("should not have securitySchemes when no auth handlers exist")
	}
}

func TestGoTypeToOpenAPISchema(t *testing.T) {
	tests := []struct {
		goType       string
		expectedType string
		format       string
		nullable     bool
		isArray      bool
	}{
		{"string", "string", "", false, false},
		{"int", "integer", "int32", false, false},
		{"int32", "integer", "int32", false, false},
		{"int64", "integer", "int64", false, false},
		{"float32", "number", "float", false, false},
		{"float64", "number", "double", false, false},
		{"bool", "boolean", "", false, false},
		{"time.Time", "string", "date-time", false, false},
		{"*string", "string", "", true, false},
		{"*int64", "integer", "int64", true, false},
		{"[]string", "string", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			schema := goTypeToOpenAPISchema(tt.goType)

			if tt.isArray {
				if schema["type"] != "array" {
					t.Errorf("expected array type, got %v", schema["type"])
				}
				items, ok := schema["items"].(map[string]any)
				if !ok {
					t.Fatal("missing items for array type")
				}
				if items["type"] != tt.expectedType {
					t.Errorf("expected items type %s, got %v", tt.expectedType, items["type"])
				}
				return
			}

			if schema["type"] != tt.expectedType {
				t.Errorf("expected type %s, got %v", tt.expectedType, schema["type"])
			}

			if tt.format != "" {
				if schema["format"] != tt.format {
					t.Errorf("expected format %s, got %v", tt.format, schema["format"])
				}
			}

			if tt.nullable {
				if schema["nullable"] != true {
					t.Error("expected nullable=true")
				}
			}
		})
	}
}

func TestGenerateOpenAPISpec_Tags(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users",
				FuncName:    "ListUsers",
				PackagePath: "example.com/app/api/users",
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths := spec["paths"].(map[string]any)
	pathItem := paths["/users"].(map[string]any)
	get := pathItem["get"].(map[string]any)

	tags := get["tags"].([]any)
	if len(tags) != 1 || tags[0] != "users" {
		t.Errorf("expected tags [users], got %v", tags)
	}
}

func TestGenerateOpenAPISpec_PathParamsExcludedFromBody(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "PUT",
				Path:        "/users/:id",
				FuncName:    "UpdateUser",
				PackagePath: "example.com/app/api/users",
				PathParams: []codegen.SerializedPathParam{
					{Name: "id", Position: 1},
				},
				Request: &codegen.SerializedStructInfo{
					Name:    "UpdateUserRequest",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "UpdateUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths := spec["paths"].(map[string]any)
	pathItem := paths["/users/{id}"].(map[string]any)
	put := pathItem["put"].(map[string]any)

	// Request body should only contain "name", not "id" (which is a path param)
	reqBody := put["requestBody"].(map[string]any)
	content := reqBody["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	if _, ok := props["id"]; ok {
		t.Error("path param 'id' should be excluded from request body")
	}
	if _, ok := props["name"]; !ok {
		t.Error("body field 'name' should be in request body")
	}
}

func TestGenerateOpenAPISpec_OmittedFields(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users",
				FuncName:    "ListUsers",
				PackagePath: "example.com/app/api/users",
				Response: &codegen.SerializedStructInfo{
					Name:    "ListUsersResponse",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
						{Name: "Internal", Type: "string", JSONName: "", JSONOmit: true},
					},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths := spec["paths"].(map[string]any)
	pathItem := paths["/users"].(map[string]any)
	get := pathItem["get"].(map[string]any)
	responses := get["responses"].(map[string]any)
	resp200 := responses["200"].(map[string]any)
	content := resp200["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	if _, ok := props["Internal"]; ok {
		t.Error("json:\"-\" field should not appear in schema")
	}
}

func TestGenerateOpenAPISpec_NestedStructSlice(t *testing.T) {
	// Simulates ListFilesResponse.Items []FileListItem — the field should produce
	// {type: "array", items: {type: "object", properties: {id: ..., name: ..., size: ...}}}
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/files",
				FuncName:    "ListFiles",
				PackagePath: "example.com/app/api/files",
				Response: &codegen.SerializedStructInfo{
					Name:    "ListFilesResponse",
					Package: "example.com/app/api/files",
					Fields: []codegen.SerializedFieldInfo{
						{
							Name:     "Items",
							Type:     "[]example.com/app/api/files.FileListItem",
							JSONName: "items",
							StructFields: &codegen.SerializedStructInfo{
								Name:    "FileListItem",
								Package: "example.com/app/api/files",
								Fields: []codegen.SerializedFieldInfo{
									{Name: "ID", Type: "string", JSONName: "id", Required: true},
									{Name: "Name", Type: "string", JSONName: "name", Required: true},
									{Name: "Size", Type: "int64", JSONName: "size", Required: true},
								},
							},
						},
					},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths := spec["paths"].(map[string]any)
	pathItem := paths["/files"].(map[string]any)
	get := pathItem["get"].(map[string]any)
	responses := get["responses"].(map[string]any)
	resp200 := responses["200"].(map[string]any)
	content := resp200["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	itemsSchema, ok := props["items"].(map[string]any)
	if !ok {
		t.Fatal("missing items property in response schema")
	}

	if itemsSchema["type"] != "array" {
		t.Errorf("expected items type 'array', got %v", itemsSchema["type"])
	}

	itemObj, ok := itemsSchema["items"].(map[string]any)
	if !ok {
		t.Fatal("missing items.items (inner object schema)")
	}

	if itemObj["type"] != "object" {
		t.Errorf("expected inner type 'object', got %v", itemObj["type"])
	}

	innerProps, ok := itemObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties in inner object schema")
	}

	for _, fieldName := range []string{"id", "name", "size"} {
		if _, ok := innerProps[fieldName]; !ok {
			t.Errorf("missing property %q in nested object schema", fieldName)
		}
	}
}

func TestGenerateOpenAPISpec_NestedStructPointer(t *testing.T) {
	// Simulates MeResponse.Organization *OrgInfo — should produce
	// {type: "object", nullable: true, properties: {id: ..., name: ...}}
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/me",
				FuncName:    "Me",
				PackagePath: "example.com/app/api/auth",
				RequireAuth: true,
				Response: &codegen.SerializedStructInfo{
					Name:    "MeResponse",
					Package: "example.com/app/api/auth",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "UserID", Type: "int64", JSONName: "user_id", Required: true},
						{
							Name:     "Organization",
							Type:     "*example.com/app/api/auth.OrgInfo",
							JSONName: "organization",
							StructFields: &codegen.SerializedStructInfo{
								Name:    "OrgInfo",
								Package: "example.com/app/api/auth",
								Fields: []codegen.SerializedFieldInfo{
									{Name: "ID", Type: "int64", JSONName: "id", Required: true},
									{Name: "Name", Type: "string", JSONName: "name", Required: true},
								},
							},
						},
						{
							Name:     "Roles",
							Type:     "[]example.com/app/api/auth.RoleInfo",
							JSONName: "roles",
							StructFields: &codegen.SerializedStructInfo{
								Name:    "RoleInfo",
								Package: "example.com/app/api/auth",
								Fields: []codegen.SerializedFieldInfo{
									{Name: "ID", Type: "int64", JSONName: "id", Required: true},
									{Name: "RoleName", Type: "string", JSONName: "role_name", Required: true},
								},
							},
						},
					},
				},
			},
		},
	}

	spec := parseSpec(t, cfg)

	paths := spec["paths"].(map[string]any)
	pathItem := paths["/me"].(map[string]any)
	get := pathItem["get"].(map[string]any)
	responses := get["responses"].(map[string]any)
	resp200 := responses["200"].(map[string]any)
	content := resp200["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	// Organization should be an object with nullable: true
	orgSchema, ok := props["organization"].(map[string]any)
	if !ok {
		t.Fatal("missing organization property")
	}

	if orgSchema["type"] != "object" {
		t.Errorf("expected organization type 'object', got %v", orgSchema["type"])
	}

	if orgSchema["nullable"] != true {
		t.Error("expected organization to be nullable (pointer type)")
	}

	orgProps, ok := orgSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing organization.properties")
	}

	for _, fieldName := range []string{"id", "name"} {
		if _, ok := orgProps[fieldName]; !ok {
			t.Errorf("missing property %q in organization schema", fieldName)
		}
	}

	// Roles should be an array of objects
	rolesSchema, ok := props["roles"].(map[string]any)
	if !ok {
		t.Fatal("missing roles property")
	}

	if rolesSchema["type"] != "array" {
		t.Errorf("expected roles type 'array', got %v", rolesSchema["type"])
	}

	roleItems, ok := rolesSchema["items"].(map[string]any)
	if !ok {
		t.Fatal("missing roles.items")
	}

	if roleItems["type"] != "object" {
		t.Errorf("expected role items type 'object', got %v", roleItems["type"])
	}

	roleProps, ok := roleItems["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing roles.items.properties")
	}

	for _, fieldName := range []string{"id", "role_name"} {
		if _, ok := roleProps[fieldName]; !ok {
			t.Errorf("missing property %q in role item schema", fieldName)
		}
	}
}

func TestGenerateOpenAPISpec_ValidJSON(t *testing.T) {
	cfg := OpenAPIGenConfig{
		ModulePath: "example.com/app",
		Handlers: []codegen.SerializedHandlerInfo{
			{
				Method:      "GET",
				Path:        "/users/:id",
				FuncName:    "GetUser",
				PackagePath: "example.com/app/api/users",
				PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 1}},
				RequireAuth: true,
				Request: &codegen.SerializedStructInfo{
					Name:    "GetUserRequest",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{{Name: "ID", Type: "int64", JSONName: "id", Required: true}},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "GetUserResponse",
					Package: "example.com/app/api/users",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "ID", Type: "int64", JSONName: "id", Required: true},
						{Name: "Name", Type: "string", JSONName: "name", Required: true},
						{Name: "Email", Type: "*string", JSONName: "email"},
						{Name: "Tags", Type: "[]string", JSONName: "tags"},
						{Name: "CreatedAt", Type: "time.Time", JSONName: "created_at", Required: true},
					},
				},
			},
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
						{Name: "Email", Type: "string", JSONName: "email", Required: true},
					},
				},
				Response: &codegen.SerializedStructInfo{
					Name:    "CreateUserResponse",
					Package: "example.com/app/api/users",
					Fields:  []codegen.SerializedFieldInfo{{Name: "ID", Type: "int64", JSONName: "id", Required: true}},
				},
			},
		},
	}

	raw, err := GenerateOpenAPISpec(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPISpec() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("generated spec is not valid JSON: %v", err)
	}
}

// ─── Query param tests (Step 7h) ───

func TestGenerateOpenAPISpec_QueryParams(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
	}

	spec := parseSpec(t, cfg)

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths in spec")
	}
	postsPath, ok := paths["/posts"].(map[string]any)
	if !ok {
		t.Fatal("missing /posts path")
	}
	getOp, ok := postsPath["get"].(map[string]any)
	if !ok {
		t.Fatal("missing get operation on /posts")
	}
	params, ok := getOp["parameters"].([]any)
	if !ok {
		t.Fatal("missing parameters on GET /posts")
	}

	// Should have 2 query parameters
	if len(params) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(params))
	}

	// Check each parameter
	foundLimit := false
	foundCursor := false
	for _, p := range params {
		param, ok := p.(map[string]any)
		if !ok {
			continue
		}
		name, _ := param["name"].(string)
		in, _ := param["in"].(string)
		if in != "query" {
			t.Errorf("expected parameter %q to be 'in: query', got %q", name, in)
		}

		switch name {
		case "limit":
			foundLimit = true
			schema, _ := param["schema"].(map[string]any)
			if schema["type"] != "integer" {
				t.Errorf("expected limit schema type 'integer', got %v", schema["type"])
			}
		case "cursor":
			foundCursor = true
			schema, _ := param["schema"].(map[string]any)
			if schema["type"] != "string" {
				t.Errorf("expected cursor schema type 'string', got %v", schema["type"])
			}
			if schema["nullable"] != true {
				t.Error("expected cursor schema to be nullable (*string)")
			}
		}
	}

	if !foundLimit {
		t.Error("missing 'limit' query parameter")
	}
	if !foundCursor {
		t.Error("missing 'cursor' query parameter")
	}
}

func TestGenerateOpenAPISpec_QueryParamsNotInRequestBody(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
	}

	spec := parseSpec(t, cfg)

	paths, _ := spec["paths"].(map[string]any)
	postsPath, _ := paths["/posts"].(map[string]any)
	postOp, _ := postsPath["post"].(map[string]any)

	// The query-tagged field should appear in parameters, not requestBody
	params, hasParams := postOp["parameters"].([]any)
	if !hasParams || len(params) == 0 {
		t.Fatal("expected parameters with query param 'tag'")
	}

	foundTag := false
	for _, p := range params {
		param, _ := p.(map[string]any)
		if param["name"] == "tag" && param["in"] == "query" {
			foundTag = true
		}
	}
	if !foundTag {
		t.Error("query-tagged field 'tag' should appear in parameters with in=query")
	}

	// The requestBody should contain title and body, but NOT tag
	reqBody, _ := postOp["requestBody"].(map[string]any)
	if reqBody == nil {
		t.Fatal("expected requestBody for POST handler with body fields")
	}
	content, _ := reqBody["content"].(map[string]any)
	jsonContent, _ := content["application/json"].(map[string]any)
	schema, _ := jsonContent["schema"].(map[string]any)
	properties, _ := schema["properties"].(map[string]any)

	if _, hasTitle := properties["title"]; !hasTitle {
		t.Error("requestBody should contain 'title' field")
	}
	if _, hasBody := properties["body"]; !hasBody {
		t.Error("requestBody should contain 'body' field")
	}
	if _, hasTag := properties["tag"]; hasTag {
		t.Error("requestBody should NOT contain query-tagged 'tag' field")
	}
}

func TestGenerateOpenAPISpec_QueryParamsBoolType(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
	}

	spec := parseSpec(t, cfg)

	paths, _ := spec["paths"].(map[string]any)
	postsPath, _ := paths["/posts"].(map[string]any)
	getOp, _ := postsPath["get"].(map[string]any)
	params, _ := getOp["parameters"].([]any)

	if len(params) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(params))
	}

	param, _ := params[0].(map[string]any)
	if param["name"] != "include_deleted" {
		t.Errorf("expected parameter name 'include_deleted', got %v", param["name"])
	}
	schema, _ := param["schema"].(map[string]any)
	if schema["type"] != "boolean" {
		t.Errorf("expected bool query param schema type 'boolean', got %v", schema["type"])
	}
}

func TestGenerateOpenAPISpec_NoQueryParamsNoParameters(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
	}

	spec := parseSpec(t, cfg)

	paths, _ := spec["paths"].(map[string]any)
	postsPath, _ := paths["/posts"].(map[string]any)
	postOp, _ := postsPath["post"].(map[string]any)

	// No path params, no query params — parameters should be absent
	if _, hasParams := postOp["parameters"]; hasParams {
		t.Error("POST handler with only body fields should NOT have parameters key")
	}
}

func TestGenerateOpenAPISpec_MixedPathAndQueryParams(t *testing.T) {
	cfg := OpenAPIGenConfig{
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
	}

	spec := parseSpec(t, cfg)

	paths, _ := spec["paths"].(map[string]any)
	postsPath, _ := paths["/users/{id}/posts"].(map[string]any)
	if postsPath == nil {
		t.Fatal("missing /users/{id}/posts path")
	}
	getOp, _ := postsPath["get"].(map[string]any)
	params, _ := getOp["parameters"].([]any)

	if len(params) != 2 {
		t.Fatalf("expected 2 parameters (1 path + 1 query), got %d", len(params))
	}

	foundPathParam := false
	foundQueryParam := false
	for _, p := range params {
		param, _ := p.(map[string]any)
		in, _ := param["in"].(string)
		name, _ := param["name"].(string)
		if in == "path" && name == "id" {
			foundPathParam = true
		}
		if in == "query" && name == "cursor" {
			foundQueryParam = true
		}
	}

	if !foundPathParam {
		t.Error("missing path parameter 'id' with in=path")
	}
	if !foundQueryParam {
		t.Error("missing query parameter 'cursor' with in=query")
	}
}
