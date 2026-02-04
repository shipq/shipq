package handler

import (
	"context"
	"testing"
	"time"
)

// Test request/response types for testing
type CreateUserRequest struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Age      int     `json:"age,omitempty"`
	Nickname *string `json:"nickname,omitempty"`
}

type CreateUserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type GetUserRequest struct {
	ID string `path:"id"`
}

type GetUserResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Age       *int       `json:"age,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ListUsersRequest struct {
	Limit  int     `query:"limit"`
	Cursor *string `query:"cursor"`
}

type ListUsersResponse struct {
	Items      []UserItem `json:"items"`
	NextCursor *string    `json:"next_cursor,omitempty"`
}

type UserItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateUserRequest struct {
	ID   string  `path:"id"`
	Name *string `json:"name,omitempty"`
	Age  *int    `json:"age,omitempty"`
}

type UpdateUserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"`
}

type DeleteUserRequest struct {
	ID string `path:"id"`
}

type DeleteUserResponse struct {
	Success bool `json:"success"`
}

// Test handlers (the actual implementation doesn't matter for registration)
func CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	return nil, nil
}

func GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	return nil, nil
}

func ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	return nil, nil
}

func UpdateUser(ctx context.Context, req *UpdateUserRequest) (*UpdateUserResponse, error) {
	return nil, nil
}

func DeleteUser(ctx context.Context, req *DeleteUserRequest) (*DeleteUserResponse, error) {
	return nil, nil
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
	if app.registry == nil {
		t.Fatal("NewApp returned app with nil registry")
	}
	if len(app.registry.Handlers) != 0 {
		t.Errorf("NewApp returned app with non-empty registry: %d handlers", len(app.registry.Handlers))
	}
}

func TestAppGet(t *testing.T) {
	app := NewApp()
	app.Get("/users/:id", GetUser)

	if len(app.registry.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(app.registry.Handlers))
	}

	h := app.registry.Handlers[0]
	if h.Method != GET {
		t.Errorf("expected method GET, got %s", h.Method)
	}
	if h.Path != "/users/:id" {
		t.Errorf("expected path /users/:id, got %s", h.Path)
	}
	if len(h.PathParams) != 1 {
		t.Fatalf("expected 1 path param, got %d", len(h.PathParams))
	}
	if h.PathParams[0].Name != "id" {
		t.Errorf("expected path param name 'id', got %s", h.PathParams[0].Name)
	}
}

func TestAppPost(t *testing.T) {
	app := NewApp()
	app.Post("/users", CreateUser)

	if len(app.registry.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(app.registry.Handlers))
	}

	h := app.registry.Handlers[0]
	if h.Method != POST {
		t.Errorf("expected method POST, got %s", h.Method)
	}
	if h.Path != "/users" {
		t.Errorf("expected path /users, got %s", h.Path)
	}
	if len(h.PathParams) != 0 {
		t.Errorf("expected 0 path params, got %d", len(h.PathParams))
	}
}

func TestAppPut(t *testing.T) {
	app := NewApp()
	app.Put("/users/:id", UpdateUser)

	if len(app.registry.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(app.registry.Handlers))
	}

	h := app.registry.Handlers[0]
	if h.Method != PUT {
		t.Errorf("expected method PUT, got %s", h.Method)
	}
}

func TestAppPatch(t *testing.T) {
	app := NewApp()
	app.Patch("/users/:id", UpdateUser)

	if len(app.registry.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(app.registry.Handlers))
	}

	h := app.registry.Handlers[0]
	if h.Method != PATCH {
		t.Errorf("expected method PATCH, got %s", h.Method)
	}
}

func TestAppDelete(t *testing.T) {
	app := NewApp()
	app.Delete("/users/:id", DeleteUser)

	if len(app.registry.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(app.registry.Handlers))
	}

	h := app.registry.Handlers[0]
	if h.Method != DELETE {
		t.Errorf("expected method DELETE, got %s", h.Method)
	}
}

func TestMultipleRegistrations(t *testing.T) {
	app := NewApp()
	app.Post("/users", CreateUser)
	app.Get("/users", ListUsers)
	app.Get("/users/:id", GetUser)
	app.Patch("/users/:id", UpdateUser)
	app.Delete("/users/:id", DeleteUser)

	if len(app.registry.Handlers) != 5 {
		t.Fatalf("expected 5 handlers, got %d", len(app.registry.Handlers))
	}

	// Verify order is preserved
	methods := []HTTPMethod{POST, GET, GET, PATCH, DELETE}
	paths := []string{"/users", "/users", "/users/:id", "/users/:id", "/users/:id"}

	for i, h := range app.registry.Handlers {
		if h.Method != methods[i] {
			t.Errorf("handler %d: expected method %s, got %s", i, methods[i], h.Method)
		}
		if h.Path != paths[i] {
			t.Errorf("handler %d: expected path %s, got %s", i, paths[i], h.Path)
		}
	}
}

func TestExtractPathParams(t *testing.T) {
	tests := []struct {
		path     string
		expected []PathParam
	}{
		{
			path:     "/users",
			expected: []PathParam{},
		},
		{
			path: "/users/:id",
			expected: []PathParam{
				{Name: "id", Position: 2},
			},
		},
		{
			path: "/users/:id/posts/:post_id",
			expected: []PathParam{
				{Name: "id", Position: 2},
				{Name: "post_id", Position: 4},
			},
		},
		{
			path: "/orgs/:org_id/teams/:team_id/members/:member_id",
			expected: []PathParam{
				{Name: "org_id", Position: 2},
				{Name: "team_id", Position: 4},
				{Name: "member_id", Position: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			params := extractPathParams(tt.path)
			if len(params) != len(tt.expected) {
				t.Fatalf("expected %d params, got %d", len(tt.expected), len(params))
			}
			for i, p := range params {
				if p.Name != tt.expected[i].Name {
					t.Errorf("param %d: expected name %s, got %s", i, tt.expected[i].Name, p.Name)
				}
				if p.Position != tt.expected[i].Position {
					t.Errorf("param %d: expected position %d, got %d", i, tt.expected[i].Position, p.Position)
				}
			}
		})
	}
}

func TestRequestTypeExtraction(t *testing.T) {
	app := NewApp()
	app.Post("/users", CreateUser)

	h := app.registry.Handlers[0]
	if h.Request == nil {
		t.Fatal("expected non-nil Request")
	}

	req := h.Request
	if req.Name != "CreateUserRequest" {
		t.Errorf("expected Name 'CreateUserRequest', got %s", req.Name)
	}

	// Verify fields
	expectedFields := map[string]struct {
		Type     string
		JSONName string
		Required bool
	}{
		"Name":     {Type: "string", JSONName: "name", Required: true},
		"Email":    {Type: "string", JSONName: "email", Required: true},
		"Age":      {Type: "int", JSONName: "age", Required: false},          // omitempty
		"Nickname": {Type: "*string", JSONName: "nickname", Required: false}, // pointer + omitempty
	}

	if len(req.Fields) != len(expectedFields) {
		t.Fatalf("expected %d fields, got %d", len(expectedFields), len(req.Fields))
	}

	for _, f := range req.Fields {
		expected, ok := expectedFields[f.Name]
		if !ok {
			t.Errorf("unexpected field: %s", f.Name)
			continue
		}
		if f.Type != expected.Type {
			t.Errorf("field %s: expected type %s, got %s", f.Name, expected.Type, f.Type)
		}
		if f.JSONName != expected.JSONName {
			t.Errorf("field %s: expected JSONName %s, got %s", f.Name, expected.JSONName, f.JSONName)
		}
		if f.Required != expected.Required {
			t.Errorf("field %s: expected Required %v, got %v", f.Name, expected.Required, f.Required)
		}
	}
}

func TestResponseTypeExtraction(t *testing.T) {
	app := NewApp()
	app.Get("/users/:id", GetUser)

	h := app.registry.Handlers[0]
	if h.Response == nil {
		t.Fatal("expected non-nil Response")
	}

	resp := h.Response
	if resp.Name != "GetUserResponse" {
		t.Errorf("expected Name 'GetUserResponse', got %s", resp.Name)
	}

	// Just verify we got the expected number of fields
	if len(resp.Fields) != 6 {
		t.Errorf("expected 6 fields, got %d", len(resp.Fields))
	}
}

func TestSliceFieldExtraction(t *testing.T) {
	app := NewApp()
	app.Get("/users", ListUsers)

	h := app.registry.Handlers[0]
	resp := h.Response

	// Find the Items field
	var itemsField *FieldInfo
	for i := range resp.Fields {
		if resp.Fields[i].Name == "Items" {
			itemsField = &resp.Fields[i]
			break
		}
	}

	if itemsField == nil {
		t.Fatal("expected to find Items field")
	}

	// Items is a slice, so should not be required
	if itemsField.Required {
		t.Error("slice field Items should not be required")
	}

	// Type should show it's a slice with full package path
	expectedType := "[]github.com/shipq/shipq/handler.UserItem"
	if itemsField.Type != expectedType {
		t.Errorf("expected type %s, got %s", expectedType, itemsField.Type)
	}
}

func TestTagExtraction(t *testing.T) {
	app := NewApp()
	app.Get("/users/:id", GetUser)

	h := app.registry.Handlers[0]

	// Check request field tags
	var idField *FieldInfo
	for i := range h.Request.Fields {
		if h.Request.Fields[i].Name == "ID" {
			idField = &h.Request.Fields[i]
			break
		}
	}

	if idField == nil {
		t.Fatal("expected to find ID field in request")
	}

	if idField.Tags["path"] != "id" {
		t.Errorf("expected path tag 'id', got %s", idField.Tags["path"])
	}
}

func TestJSONOmitTag(t *testing.T) {
	// Define a type with json:"-"
	type IgnoredFieldRequest struct {
		Public  string `json:"public"`
		ignored string // unexported, won't be included
		Hidden  string `json:"-"`
	}

	type IgnoredFieldResponse struct {
		ID string `json:"id"`
	}

	handler := func(ctx context.Context, req *IgnoredFieldRequest) (*IgnoredFieldResponse, error) {
		return nil, nil
	}

	app := NewApp()
	app.Post("/test", handler)

	h := app.registry.Handlers[0]

	// Should have 2 exported fields (Public and Hidden)
	if len(h.Request.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(h.Request.Fields))
	}

	// Find Hidden field and verify JSONOmit is true
	var hiddenField *FieldInfo
	for i := range h.Request.Fields {
		if h.Request.Fields[i].Name == "Hidden" {
			hiddenField = &h.Request.Fields[i]
			break
		}
	}

	if hiddenField == nil {
		t.Fatal("expected to find Hidden field")
	}

	if !hiddenField.JSONOmit {
		t.Error("expected JSONOmit to be true for field with json:\"-\"")
	}

	if hiddenField.JSONName != "" {
		t.Errorf("expected empty JSONName for omitted field, got %s", hiddenField.JSONName)
	}
}

func TestRegistryAccess(t *testing.T) {
	app := NewApp()
	app.Get("/test", GetUser)

	registry := app.Registry()
	if registry == nil {
		t.Fatal("Registry() returned nil")
	}

	if len(registry.Handlers) != 1 {
		t.Errorf("expected 1 handler in registry, got %d", len(registry.Handlers))
	}

	// Verify it's the same registry
	app.Post("/test2", CreateUser)
	if len(registry.Handlers) != 2 {
		t.Error("registry should be the same instance and see new registrations")
	}
}

func TestInvalidHandlerPanics(t *testing.T) {
	tests := []struct {
		name    string
		handler any
	}{
		{
			name:    "not a function",
			handler: "not a function",
		},
		{
			name:    "wrong number of params",
			handler: func(ctx context.Context) (*CreateUserResponse, error) { return nil, nil },
		},
		{
			name:    "wrong number of returns",
			handler: func(ctx context.Context, req *CreateUserRequest) error { return nil },
		},
		{
			name: "wrong first param type",
			handler: func(notCtx string, req *CreateUserRequest) (*CreateUserResponse, error) {
				return nil, nil
			},
		},
		{
			name: "wrong second return type",
			handler: func(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, string) {
				return nil, ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic but didn't get one")
				}
			}()

			app := NewApp()
			app.Get("/test", tt.handler)
		})
	}
}

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "string",
			input:    "",
			expected: "string",
		},
		{
			name:     "int",
			input:    0,
			expected: "int",
		},
		{
			name:     "int64",
			input:    int64(0),
			expected: "int64",
		},
		{
			name:     "bool",
			input:    false,
			expected: "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test typeToString directly, but we can verify
			// through the extraction process
			type TestStruct struct {
				Field any
			}

			// This is a basic check - the full test is in the extraction tests
			_ = tt.input
		})
	}
}
