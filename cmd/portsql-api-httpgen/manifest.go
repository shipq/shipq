package main

// Manifest represents the discovered endpoints from a package.
type Manifest struct {
	Endpoints          []ManifestEndpoint                     `json:"endpoints"`
	Middlewares        []ManifestMiddleware                   `json:"middlewares,omitempty"`
	ContextKeys        []ManifestContextKey                   `json:"context_keys,omitempty"`
	MiddlewareMetadata map[string]*ManifestMiddlewareMetadata `json:"middleware_metadata,omitempty"`

	// OpenAPI extensions (Step 2)
	Types        []ManifestType         `json:"types,omitempty"`
	EndpointDocs map[string]ManifestDoc `json:"endpoint_docs,omitempty"`
}

// ManifestType represents a type in the type graph for OpenAPI schema generation.
type ManifestType struct {
	ID       string          `json:"id"`                 // Stable unique ID (e.g., "github.com/example/pkg.TypeName")
	GoType   string          `json:"go_type"`            // Original Go type string for debugging
	Kind     string          `json:"kind"`               // "struct", "slice", "map", "pointer", "string", "int", "bool", "time", "bytes", "unknown", etc.
	Nullable bool            `json:"nullable,omitempty"` // True for pointer types
	Fields   []ManifestField `json:"fields,omitempty"`   // For struct types
	Elem     string          `json:"elem,omitempty"`     // Element type ID for slices/arrays/pointers
	Key      string          `json:"key,omitempty"`      // Key type ID for maps
	Value    string          `json:"value,omitempty"`    // Value type ID for maps
	Doc      string          `json:"doc,omitempty"`      // Doc comment for named types
	Warnings []string        `json:"warnings,omitempty"` // Warnings for unsupported features
}

// ManifestField represents a struct field in the type graph.
type ManifestField struct {
	GoName   string `json:"go_name"`             // Go struct field name
	JSONName string `json:"json_name,omitempty"` // JSON field name (empty if not serialized)
	TypeID   string `json:"type_id"`             // Reference to type in the type graph
	Required bool   `json:"required"`            // True if field is required (no omitempty, not pointer)
	Doc      string `json:"doc,omitempty"`       // Doc comment for field
}

// ManifestDoc represents documentation for a handler or type.
type ManifestDoc struct {
	Summary     string `json:"summary,omitempty"`     // First line/sentence of doc comment
	Description string `json:"description,omitempty"` // Full doc comment text
}

// ManifestMiddleware represents a declared middleware function.
type ManifestMiddleware struct {
	Pkg  string `json:"pkg"`
	Name string `json:"name"`
}

// ManifestContextKey represents a context key provided by middleware.
type ManifestContextKey struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}

// ManifestMiddlewareMetadata represents metadata about a middleware function.
type ManifestMiddlewareMetadata struct {
	RequiredHeaders   []string                  `json:"required_headers,omitempty"`
	RequiredCookies   []string                  `json:"required_cookies,omitempty"`
	SecuritySchemes   []string                  `json:"security_schemes,omitempty"`
	MayReturnStatuses []ManifestMayReturnStatus `json:"may_return_statuses,omitempty"`
}

// ManifestMayReturnStatus represents a status code that middleware may return.
type ManifestMayReturnStatus struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
}

// ManifestEndpoint represents a single discovered endpoint.
type ManifestEndpoint struct {
	Method      string               `json:"method"`
	Path        string               `json:"path"`
	HandlerPkg  string               `json:"handler_pkg"`
	HandlerName string               `json:"handler_name"`
	Shape       string               `json:"shape"` // ctx_req_resp_err, ctx_req_err, ctx_resp_err, ctx_err
	ReqType     string               `json:"req_type,omitempty"`
	RespType    string               `json:"resp_type,omitempty"`
	Middlewares []ManifestMiddleware `json:"middlewares,omitempty"`

	// Binding metadata for codegen (populated during discovery)
	Bindings *BindingInfo `json:"bindings,omitempty"`
}

// BindingInfo contains analyzed binding metadata for a request type.
type BindingInfo struct {
	HasJSONBody    bool           `json:"has_json_body"`
	PathBindings   []FieldBinding `json:"path_bindings,omitempty"`
	QueryBindings  []FieldBinding `json:"query_bindings,omitempty"`
	HeaderBindings []FieldBinding `json:"header_bindings,omitempty"`
}

// FieldBinding represents a struct field bound to a request component.
type FieldBinding struct {
	FieldName string `json:"field_name"` // Go struct field name
	TagValue  string `json:"tag_value"`  // tag value (e.g., "id" from path:"id")
	TypeKind  string `json:"type_kind"`  // "string", "int", "int64", "bool", "time.Time", etc.
	IsPointer bool   `json:"is_pointer"` // true if field type is *T (optional)
	IsSlice   bool   `json:"is_slice"`   // true if field type is []T (multi-value)
	ElemKind  string `json:"elem_kind"`  // element type for slices (e.g., "string" for []string)
}
