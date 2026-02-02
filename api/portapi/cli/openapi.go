package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// BuildOpenAPI generates an OpenAPI 3.0.3 JSON document from config and manifest.
func BuildOpenAPI(cfg *Config, manifest *Manifest) ([]byte, error) {
	doc := &OpenAPIDocument{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       cfg.OpenAPITitle,
			Version:     cfg.OpenAPIVersion,
			Description: cfg.OpenAPIDescription,
		},
	}

	// Add servers if configured
	for _, serverURL := range cfg.OpenAPIServers {
		doc.Servers = append(doc.Servers, OpenAPIServer{URL: serverURL})
	}

	// Build type index for schema lookups
	typeIndex := make(map[string]ManifestType)
	for _, mt := range manifest.Types {
		typeIndex[mt.ID] = mt
	}

	// Initialize components with error schema
	doc.Components = OpenAPIComponents{
		Schemas: make(map[string]OpenAPISchema),
	}
	doc.Components.Schemas["ErrorResponse"] = buildErrorResponseSchema()

	// Add schemas from type graph
	for _, mt := range manifest.Types {
		if mt.Kind == "struct" {
			schemaKey := sanitizeSchemaKey(mt.ID)
			doc.Components.Schemas[schemaKey] = buildSchemaFromType(mt, typeIndex)
		}
	}

	// Build paths
	doc.Paths = make(map[string]OpenAPIPathItem)

	// Group endpoints by path
	pathEndpoints := make(map[string][]ManifestEndpoint)
	for _, ep := range manifest.Endpoints {
		pathEndpoints[ep.Path] = append(pathEndpoints[ep.Path], ep)
	}

	// Sort paths for determinism
	paths := make([]string, 0, len(pathEndpoints))
	for p := range pathEndpoints {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		endpoints := pathEndpoints[path]
		pathItem := OpenAPIPathItem{}

		// Sort endpoints by method for determinism
		sort.Slice(endpoints, func(i, j int) bool {
			return methodOrder(endpoints[i].Method) < methodOrder(endpoints[j].Method)
		})

		for _, ep := range endpoints {
			operation := buildOperation(ep, manifest, typeIndex)
			switch strings.ToLower(ep.Method) {
			case "get":
				pathItem.Get = &operation
			case "post":
				pathItem.Post = &operation
			case "put":
				pathItem.Put = &operation
			case "delete":
				pathItem.Delete = &operation
			case "patch":
				pathItem.Patch = &operation
			}
		}

		doc.Paths[path] = pathItem
	}

	// Marshal with indentation for readability
	return json.MarshalIndent(doc, "", "  ")
}

// methodOrder returns a sort order for HTTP methods
func methodOrder(method string) int {
	switch strings.ToLower(method) {
	case "get":
		return 0
	case "post":
		return 1
	case "put":
		return 2
	case "patch":
		return 3
	case "delete":
		return 4
	default:
		return 5
	}
}

// buildOperation creates an OpenAPI operation from a manifest endpoint
func buildOperation(ep ManifestEndpoint, manifest *Manifest, typeIndex map[string]ManifestType) OpenAPIOperation {
	op := OpenAPIOperation{
		OperationID: sanitizeOperationID(ep.HandlerPkg + "." + ep.HandlerName),
		Tags:        []string{extractTag(ep.Path)},
		Responses:   make(map[string]OpenAPIResponse),
	}

	// Add docstrings if available
	handlerKey := ep.HandlerPkg + "." + ep.HandlerName
	if doc, ok := manifest.EndpointDocs[handlerKey]; ok {
		op.Summary = doc.Summary
		op.Description = doc.Description
	}

	// Build parameters
	if ep.Bindings != nil {
		for _, pb := range ep.Bindings.PathBindings {
			op.Parameters = append(op.Parameters, OpenAPIParameter{
				Name:     pb.TagValue,
				In:       "path",
				Required: true, // Path params are always required
				Schema:   schemaForTypeKind(pb.TypeKind),
			})
		}
		for _, qb := range ep.Bindings.QueryBindings {
			op.Parameters = append(op.Parameters, OpenAPIParameter{
				Name:     qb.TagValue,
				In:       "query",
				Required: !qb.IsPointer, // Pointer means optional
				Schema:   schemaForTypeKind(qb.TypeKind),
			})
		}
		for _, hb := range ep.Bindings.HeaderBindings {
			op.Parameters = append(op.Parameters, OpenAPIParameter{
				Name:     hb.TagValue,
				In:       "header",
				Required: !hb.IsPointer,
				Schema:   schemaForTypeKind(hb.TypeKind),
			})
		}

		// Add request body if JSON body exists
		if ep.Bindings.HasJSONBody && ep.ReqType != "" {
			op.RequestBody = &OpenAPIRequestBody{
				Required: true,
				Content: map[string]OpenAPIMediaType{
					"application/json": {
						Schema: schemaRefForType(ep.ReqType, typeIndex),
					},
				},
			}
		}
	}

	// Sort parameters for determinism
	sort.Slice(op.Parameters, func(i, j int) bool {
		if op.Parameters[i].In != op.Parameters[j].In {
			return paramInOrder(op.Parameters[i].In) < paramInOrder(op.Parameters[j].In)
		}
		return op.Parameters[i].Name < op.Parameters[j].Name
	})

	// Build responses
	hasResponse := ep.Shape == "ctx_resp_err" || ep.Shape == "ctx_req_resp_err"

	if hasResponse && ep.RespType != "" {
		op.Responses["200"] = OpenAPIResponse{
			Description: "Successful response",
			Content: map[string]OpenAPIMediaType{
				"application/json": {
					Schema: schemaRefForType(ep.RespType, typeIndex),
				},
			},
		}
	} else {
		op.Responses["204"] = OpenAPIResponse{
			Description: "No Content",
		}
	}

	// Add 400 Bad Request if endpoint has bindings
	if ep.Bindings != nil && (len(ep.Bindings.PathBindings) > 0 ||
		len(ep.Bindings.QueryBindings) > 0 ||
		len(ep.Bindings.HeaderBindings) > 0 ||
		ep.Bindings.HasJSONBody) {
		op.Responses["400"] = OpenAPIResponse{
			Description: "Bad Request",
			Content: map[string]OpenAPIMediaType{
				"application/json": {
					Schema: OpenAPISchema{Ref: "#/components/schemas/ErrorResponse"},
				},
			},
		}
	}

	// Add middleware MayReturn responses
	for _, mw := range ep.Middlewares {
		key := mw.Pkg + "." + mw.Name
		if meta, ok := manifest.MiddlewareMetadata[key]; ok && meta != nil {
			for _, status := range meta.MayReturnStatuses {
				statusCode := fmt.Sprintf("%d", status.Status)
				if _, exists := op.Responses[statusCode]; !exists {
					op.Responses[statusCode] = OpenAPIResponse{
						Description: status.Description,
						Content: map[string]OpenAPIMediaType{
							"application/json": {
								Schema: OpenAPISchema{Ref: "#/components/schemas/ErrorResponse"},
							},
						},
					}
				}
			}
		}
	}

	// Always add 500 Internal Server Error
	op.Responses["500"] = OpenAPIResponse{
		Description: "Internal Server Error",
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: OpenAPISchema{Ref: "#/components/schemas/ErrorResponse"},
			},
		},
	}

	return op
}

// paramInOrder returns sort order for parameter location
func paramInOrder(in string) int {
	switch in {
	case "path":
		return 0
	case "query":
		return 1
	case "header":
		return 2
	case "cookie":
		return 3
	default:
		return 4
	}
}

// schemaForTypeKind returns an OpenAPI schema for a Go type kind
func schemaForTypeKind(typeKind string) OpenAPISchema {
	switch typeKind {
	case "string":
		return OpenAPISchema{Type: "string"}
	case "bool":
		return OpenAPISchema{Type: "boolean"}
	case "int", "int8", "int16", "int32":
		return OpenAPISchema{Type: "integer", Format: "int32"}
	case "int64":
		return OpenAPISchema{Type: "integer", Format: "int64"}
	case "uint", "uint8", "uint16", "uint32":
		return OpenAPISchema{Type: "integer", Format: "int32"}
	case "uint64":
		return OpenAPISchema{Type: "integer", Format: "int64"}
	case "float32":
		return OpenAPISchema{Type: "number", Format: "float"}
	case "float64":
		return OpenAPISchema{Type: "number", Format: "double"}
	case "time.Time":
		return OpenAPISchema{Type: "string", Format: "date-time"}
	default:
		return OpenAPISchema{Type: "string"}
	}
}

// schemaRefForType returns a schema reference or inline schema for a type
func schemaRefForType(typeID string, typeIndex map[string]ManifestType) OpenAPISchema {
	mt, ok := typeIndex[typeID]
	if !ok {
		// Type not in index, return generic object
		return OpenAPISchema{Type: "object"}
	}

	switch mt.Kind {
	case "struct":
		return OpenAPISchema{Ref: "#/components/schemas/" + sanitizeSchemaKey(typeID)}
	case "slice":
		return OpenAPISchema{
			Type:  "array",
			Items: ptrSchema(schemaRefForType(mt.Elem, typeIndex)),
		}
	case "map":
		return OpenAPISchema{
			Type:                 "object",
			AdditionalProperties: ptrSchema(schemaRefForType(mt.Value, typeIndex)),
		}
	case "pointer":
		schema := schemaRefForType(mt.Elem, typeIndex)
		schema.Nullable = true
		return schema
	case "string":
		return OpenAPISchema{Type: "string"}
	case "bool":
		return OpenAPISchema{Type: "boolean"}
	case "int", "int32":
		return OpenAPISchema{Type: "integer", Format: "int32"}
	case "int64":
		return OpenAPISchema{Type: "integer", Format: "int64"}
	case "uint", "uint32":
		return OpenAPISchema{Type: "integer", Format: "int32"}
	case "uint64":
		return OpenAPISchema{Type: "integer", Format: "int64"}
	case "float":
		return OpenAPISchema{Type: "number", Format: "float"}
	case "double":
		return OpenAPISchema{Type: "number", Format: "double"}
	case "time":
		return OpenAPISchema{Type: "string", Format: "date-time"}
	case "bytes":
		return OpenAPISchema{Type: "string", Format: "byte"}
	default:
		return OpenAPISchema{Type: "object", XGoType: mt.GoType}
	}
}

// ptrSchema returns a pointer to a schema
func ptrSchema(s OpenAPISchema) *OpenAPISchema {
	return &s
}

// buildSchemaFromType creates an OpenAPI schema from a manifest type
func buildSchemaFromType(mt ManifestType, typeIndex map[string]ManifestType) OpenAPISchema {
	schema := OpenAPISchema{
		Type:        "object",
		Description: mt.Doc,
		Properties:  make(map[string]OpenAPISchema),
	}

	var required []string

	for _, field := range mt.Fields {
		if field.JSONName == "" {
			continue // Skip fields without JSON serialization
		}

		propSchema := schemaRefForType(field.TypeID, typeIndex)
		propSchema.Description = field.Doc

		schema.Properties[field.JSONName] = propSchema

		if field.Required {
			required = append(required, field.JSONName)
		}
	}

	// Sort required for determinism
	sort.Strings(required)
	schema.Required = required

	return schema
}

// buildErrorResponseSchema creates the shared error response schema
func buildErrorResponseSchema() OpenAPISchema {
	return OpenAPISchema{
		Type: "object",
		Properties: map[string]OpenAPISchema{
			"error": {
				Type: "object",
				Properties: map[string]OpenAPISchema{
					"code": {
						Type:        "string",
						Description: "Error code",
					},
					"message": {
						Type:        "string",
						Description: "Error message",
					},
				},
				Required: []string{"code", "message"},
			},
		},
		Required: []string{"error"},
	}
}

// sanitizeOperationID creates a valid operationId from handler path
func sanitizeOperationID(s string) string {
	// Replace slashes and dots with underscores
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	// Remove any other invalid characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return reg.ReplaceAllString(s, "")
}

// sanitizeSchemaKey creates a valid schema key from a type ID
func sanitizeSchemaKey(typeID string) string {
	// Replace slashes with dots (common in package paths)
	s := strings.ReplaceAll(typeID, "/", ".")
	// Keep only valid characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	return reg.ReplaceAllString(s, "")
}

// extractTag extracts a tag from a path (first segment)
func extractTag(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		// Remove path parameters
		tag := parts[0]
		if strings.HasPrefix(tag, "{") {
			return "default"
		}
		return tag
	}
	return "default"
}

// OpenAPI Document Types

// OpenAPIDocument represents an OpenAPI 3.0 document
type OpenAPIDocument struct {
	OpenAPI    string                     `json:"openapi"`
	Info       OpenAPIInfo                `json:"info"`
	Servers    []OpenAPIServer            `json:"servers,omitempty"`
	Paths      map[string]OpenAPIPathItem `json:"paths"`
	Components OpenAPIComponents          `json:"components,omitempty"`
}

// OpenAPIInfo represents the info section
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// OpenAPIServer represents a server
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// OpenAPIPathItem represents a path item
type OpenAPIPathItem struct {
	Get    *OpenAPIOperation `json:"get,omitempty"`
	Post   *OpenAPIOperation `json:"post,omitempty"`
	Put    *OpenAPIOperation `json:"put,omitempty"`
	Delete *OpenAPIOperation `json:"delete,omitempty"`
	Patch  *OpenAPIOperation `json:"patch,omitempty"`
}

// OpenAPIOperation represents an operation
type OpenAPIOperation struct {
	OperationID string                     `json:"operationId"`
	Tags        []string                   `json:"tags,omitempty"`
	Summary     string                     `json:"summary,omitempty"`
	Description string                     `json:"description,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses"`
}

// OpenAPIParameter represents a parameter
type OpenAPIParameter struct {
	Name        string        `json:"name"`
	In          string        `json:"in"`
	Description string        `json:"description,omitempty"`
	Required    bool          `json:"required"`
	Schema      OpenAPISchema `json:"schema"`
}

// OpenAPIRequestBody represents a request body
type OpenAPIRequestBody struct {
	Description string                      `json:"description,omitempty"`
	Required    bool                        `json:"required"`
	Content     map[string]OpenAPIMediaType `json:"content"`
}

// OpenAPIResponse represents a response
type OpenAPIResponse struct {
	Description string                      `json:"description"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

// OpenAPIMediaType represents a media type
type OpenAPIMediaType struct {
	Schema OpenAPISchema `json:"schema"`
}

// OpenAPIComponents represents the components section
type OpenAPIComponents struct {
	Schemas map[string]OpenAPISchema `json:"schemas,omitempty"`
}

// OpenAPISchema represents a schema
type OpenAPISchema struct {
	Ref                  string                   `json:"$ref,omitempty"`
	Type                 string                   `json:"type,omitempty"`
	Format               string                   `json:"format,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Properties           map[string]OpenAPISchema `json:"properties,omitempty"`
	Required             []string                 `json:"required,omitempty"`
	Items                *OpenAPISchema           `json:"items,omitempty"`
	AdditionalProperties *OpenAPISchema           `json:"additionalProperties,omitempty"`
	Nullable             bool                     `json:"nullable,omitempty"`
	XGoType              string                   `json:"x-go-type,omitempty"`
}
