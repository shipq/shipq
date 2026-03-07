package openapigen

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/shipq/shipq/codegen"
)

// OpenAPIGenConfig holds configuration for generating the OpenAPI spec.
type OpenAPIGenConfig struct {
	ModulePath  string                          // e.g., "myapp"
	Handlers    []codegen.SerializedHandlerInfo // handlers from registry
	Title       string                          // defaults to module path base name
	Version     string                          // defaults to "1.0.0"
	StripPrefix string                          // URL prefix for the servers block (e.g., "/api")
}

// GenerateOpenAPISpec generates an OpenAPI 3.1.0 JSON document from the handler registry.
// The spec is built as nested maps and marshalled to indented JSON.
func GenerateOpenAPISpec(cfg OpenAPIGenConfig) ([]byte, error) {
	title := cfg.Title
	if title == "" {
		title = path.Base(cfg.ModulePath)
	}
	version := cfg.Version
	if version == "" {
		version = "1.0.0"
	}

	spec := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   title,
			"version": version,
		},
	}

	// When a strip prefix is configured (e.g., "/api"), add a servers block
	// so that OpenAPI clients know to prepend the prefix to all paths.
	if cfg.StripPrefix != "" {
		spec["servers"] = []map[string]any{
			{"url": cfg.StripPrefix},
		}
	}

	// Build paths
	paths := buildPaths(cfg.Handlers)
	spec["paths"] = paths

	// Build components (schemas + security schemes)
	components := buildComponents(cfg.Handlers)
	spec["components"] = components

	return json.MarshalIndent(spec, "", "  ")
}

// buildPaths converts handler info into the OpenAPI paths object.
func buildPaths(handlers []codegen.SerializedHandlerInfo) map[string]any {
	paths := make(map[string]any)

	// Group by path for deterministic output
	pathOrder := make([]string, 0)
	pathHandlers := make(map[string][]codegen.SerializedHandlerInfo)

	for _, h := range handlers {
		converted := codegen.ConvertPathSyntax(h.Path)
		if _, exists := pathHandlers[converted]; !exists {
			pathOrder = append(pathOrder, converted)
		}
		pathHandlers[converted] = append(pathHandlers[converted], h)
	}

	sort.Strings(pathOrder)

	for _, p := range pathOrder {
		pathItem := make(map[string]any)
		for _, h := range pathHandlers[p] {
			operation := buildOperation(h)
			method := strings.ToLower(h.Method)
			pathItem[method] = operation
		}
		paths[p] = pathItem
	}

	return paths
}

// buildOperation creates an OpenAPI operation object from a handler.
func buildOperation(h codegen.SerializedHandlerInfo) map[string]any {
	op := make(map[string]any)

	// Operation ID from function name
	op["operationId"] = h.FuncName

	// Tags from resource name
	resourceName := path.Base(h.PackagePath)
	op["tags"] = []string{resourceName}

	// Path parameters
	params := buildPathParameters(h)

	// Query parameters
	queryParams := buildQueryParameters(h)
	params = append(params, queryParams...)

	if len(params) > 0 {
		op["parameters"] = params
	}

	// Request body for POST/PUT/PATCH
	if codegen.MethodHasBody(h.Method) && h.Request != nil && len(h.Request.Fields) > 0 {
		// Filter out path param and query param fields from the request body
		bodyFields := filterBodyFields(h)
		if len(bodyFields) > 0 {
			schema := buildSchemaFromFields(bodyFields)
			op["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": schema,
					},
				},
			}
		}
	}

	// Responses
	op["responses"] = buildResponses(h)

	// Security
	if h.RequireAuth {
		op["security"] = []map[string]any{
			{"cookieAuth": []string{}},
		}
	}

	return op
}

// buildPathParameters creates OpenAPI parameter objects from path params.
func buildPathParameters(h codegen.SerializedHandlerInfo) []map[string]any {
	var params []map[string]any

	for _, pp := range h.PathParams {
		param := map[string]any{
			"name":     pp.Name,
			"in":       "path",
			"required": true,
		}

		// Try to find the type from the request struct
		schema := map[string]any{"type": "string"}
		if h.Request != nil {
			for _, f := range h.Request.Fields {
				// Match by path tag first (most explicit), then JSON name, then Go field name
				if f.Tags["path"] == pp.Name ||
					strings.EqualFold(f.JSONName, pp.Name) ||
					strings.EqualFold(f.Name, pp.Name) {
					schema = goTypeToOpenAPISchema(f.Type)
					break
				}
			}
		}
		param["schema"] = schema

		params = append(params, param)
	}

	return params
}

// buildQueryParameters creates OpenAPI parameter objects from query-tagged fields.
func buildQueryParameters(h codegen.SerializedHandlerInfo) []map[string]any {
	queryFields := codegen.FilterQueryFields(h)
	if len(queryFields) == 0 {
		return nil
	}

	var params []map[string]any
	for _, f := range queryFields {
		queryName := f.Tags["query"]
		param := map[string]any{
			"name":     queryName,
			"in":       "query",
			"required": f.Required,
			"schema":   goTypeToOpenAPISchema(f.Type),
		}
		params = append(params, param)
	}

	return params
}

// filterBodyFields returns request fields that are NOT path parameters.
func filterBodyFields(h codegen.SerializedHandlerInfo) []codegen.SerializedFieldInfo {
	if h.Request == nil {
		return nil
	}

	pathParamNames := make(map[string]bool)
	for _, pp := range h.PathParams {
		pathParamNames[strings.ToLower(pp.Name)] = true
	}

	var bodyFields []codegen.SerializedFieldInfo
	for _, f := range h.Request.Fields {
		// Exclude fields that are path parameters (check path tag, JSON name, and Go field name)
		if f.Tags["path"] != "" && pathParamNames[strings.ToLower(f.Tags["path"])] {
			continue
		}
		if pathParamNames[strings.ToLower(f.JSONName)] || pathParamNames[strings.ToLower(f.Name)] {
			continue
		}
		// Exclude fields that are query parameters
		if f.Tags != nil && f.Tags["query"] != "" {
			continue
		}
		bodyFields = append(bodyFields, f)
	}

	return bodyFields
}

// buildResponses creates the OpenAPI responses object for a handler.
func buildResponses(h codegen.SerializedHandlerInfo) map[string]any {
	responses := make(map[string]any)

	successCode := "200"
	if h.Method == "POST" {
		successCode = "201"
	}

	successResp := map[string]any{
		"description": "Successful response",
	}

	if h.Response != nil && len(h.Response.Fields) > 0 {
		schema := buildSchemaFromFields(h.Response.Fields)
		successResp["content"] = map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		}
	}

	responses[successCode] = successResp

	// Add 401 for auth routes
	if h.RequireAuth {
		responses["401"] = map[string]any{
			"description": "Unauthorized",
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"error": map[string]any{"type": "string"},
						},
					},
				},
			},
		}
	}

	return responses
}

// buildSchemaFromFields creates an OpenAPI schema object from struct fields.
func buildSchemaFromFields(fields []codegen.SerializedFieldInfo) map[string]any {
	schema := map[string]any{
		"type": "object",
	}

	properties := make(map[string]any)
	var required []string

	for _, f := range fields {
		// Skip fields with json:"-" (truly hidden). These have JSONOmit=true
		// and JSONName="". Fields with omitempty have JSONOmit=true but a
		// non-empty JSONName — they are optional, not hidden.
		if f.JSONOmit && f.JSONName == "" {
			continue
		}

		jsonName := f.JSONName
		if jsonName == "" {
			jsonName = f.Name
		}

		properties[jsonName] = fieldToOpenAPISchema(f)

		if f.Required {
			required = append(required, jsonName)
		}
	}

	schema["properties"] = properties
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}

	return schema
}

// buildComponents creates the OpenAPI components object.
func buildComponents(handlers []codegen.SerializedHandlerInfo) map[string]any {
	components := make(map[string]any)

	// Add security schemes if any handler requires auth
	hasAuth := false
	for _, h := range handlers {
		if h.RequireAuth {
			hasAuth = true
			break
		}
	}

	if hasAuth {
		components["securitySchemes"] = map[string]any{
			"cookieAuth": map[string]any{
				"type": "apiKey",
				"in":   "cookie",
				"name": "session",
			},
		}
	}

	return components
}

// fieldToOpenAPISchema converts a SerializedFieldInfo to an OpenAPI schema.
// If the field has StructFields (i.e., it's a nested struct), it produces a
// proper object schema (or array of objects) instead of falling back to string.
func fieldToOpenAPISchema(f codegen.SerializedFieldInfo) map[string]any {
	if f.StructFields != nil && len(f.StructFields.Fields) > 0 {
		objSchema := buildSchemaFromFields(f.StructFields.Fields)

		goType := f.Type
		// Peel pointer wrapper
		isNullable := false
		if strings.HasPrefix(goType, "*") {
			isNullable = true
			goType = goType[1:]
		}

		// Slice wrapper → array of objects
		if strings.HasPrefix(goType, "[]") {
			schema := map[string]any{
				"type":  "array",
				"items": objSchema,
			}
			if isNullable {
				schema["nullable"] = true
			}
			return schema
		}

		// Plain struct or *struct
		if isNullable {
			objSchema["nullable"] = true
		}
		return objSchema
	}

	return goTypeToOpenAPISchema(f.Type)
}

// goTypeToOpenAPISchema converts a Go type string to an OpenAPI schema map.
func goTypeToOpenAPISchema(goType string) map[string]any {
	// Handle pointer types by stripping the *
	if strings.HasPrefix(goType, "*") {
		inner := goTypeToOpenAPISchema(goType[1:])
		inner["nullable"] = true
		return inner
	}

	// Handle slice types
	if strings.HasPrefix(goType, "[]") {
		elementType := goType[2:]
		return map[string]any{
			"type":  "array",
			"items": goTypeToOpenAPISchema(elementType),
		}
	}

	switch goType {
	case "string":
		return map[string]any{"type": "string"}
	case "int", "int32":
		return map[string]any{"type": "integer", "format": "int32"}
	case "int64":
		return map[string]any{"type": "integer", "format": "int64"}
	case "uint", "uint32":
		return map[string]any{"type": "integer", "format": "int32", "minimum": 0}
	case "uint64":
		return map[string]any{"type": "integer", "format": "int64", "minimum": 0}
	case "float32":
		return map[string]any{"type": "number", "format": "float"}
	case "float64":
		return map[string]any{"type": "number", "format": "double"}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "time.Time":
		return map[string]any{"type": "string", "format": "date-time"}
	default:
		// Unknown types default to string
		return map[string]any{"type": "string"}
	}
}
