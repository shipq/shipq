// Package handler provides types and utilities for registering HTTP handlers
// and capturing their metadata for code generation.
//
// This package enables automatic generation of HTTP handlers, OpenAPI specs,
// and TypeScript clients from a simple registration DSL.
package handler

import (
	"reflect"
)

// HTTPMethod represents an HTTP method.
type HTTPMethod string

const (
	GET    HTTPMethod = "GET"
	POST   HTTPMethod = "POST"
	PUT    HTTPMethod = "PUT"
	PATCH  HTTPMethod = "PATCH"
	DELETE HTTPMethod = "DELETE"
)

// PathParam represents a path parameter extracted from the URL pattern.
type PathParam struct {
	Name     string // e.g., "id" from "/users/:id"
	Position int    // position in the path segments
}

// FieldInfo represents a single field in a request or response struct.
type FieldInfo struct {
	Name     string            // Go field name
	Type     string            // Go type (e.g., "string", "int64", "*time.Time")
	JSONName string            // JSON field name from `json` tag
	JSONOmit bool              // true if `json:"-"` or `json:",omitempty"` omits when empty
	Required bool              // true if field is required (no omitempty, not a pointer)
	Tags     map[string]string // all struct tags for extensibility
}

// StructInfo represents a request or response struct's full definition.
type StructInfo struct {
	Name        string       // Type name (e.g., "CreateUserRequest")
	Package     string       // Package path (e.g., "myapp/api/users")
	Fields      []FieldInfo  // All fields in the struct
	ReflectType reflect.Type // For runtime introspection if needed
}

// HandlerInfo holds all metadata about a registered handler.
type HandlerInfo struct {
	// HTTP routing
	Method     HTTPMethod  // GET, POST, PUT, PATCH, DELETE
	Path       string      // e.g., "/users/:id"
	PathParams []PathParam // extracted from Path

	// Handler identity
	FuncName    string // e.g., "GetUser"
	PackagePath string // e.g., "myapp/api/users"

	// Request/Response types - full struct definitions
	Request  *StructInfo // nil for handlers with no request body (some GETs)
	Response *StructInfo // nil for handlers that return no body
}

// Registry holds all registered handlers.
type Registry struct {
	Handlers []HandlerInfo
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		Handlers: make([]HandlerInfo, 0),
	}
}
