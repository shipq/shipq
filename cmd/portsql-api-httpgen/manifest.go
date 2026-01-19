package main

// Manifest represents the discovered endpoints from a package.
type Manifest struct {
	Endpoints          []ManifestEndpoint                     `json:"endpoints"`
	Middlewares        []ManifestMiddleware                   `json:"middlewares,omitempty"`
	ContextKeys        []ManifestContextKey                   `json:"context_keys,omitempty"`
	MiddlewareMetadata map[string]*ManifestMiddlewareMetadata `json:"middleware_metadata,omitempty"`
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
