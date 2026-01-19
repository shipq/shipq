package portapi

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
)

// Validation Error Codes
//
// Handler validation errors:
//   - nil_handler: handler is nil
//   - not_a_function: handler must be a function
//   - variadic_not_supported: variadic handlers not supported
//   - missing_context: first arg must be context.Context
//   - first_arg_not_context: first arg must be context.Context
//   - too_many_args: at most 2 args allowed (ctx, req)
//   - missing_error_return: must return error
//   - last_return_not_error: last return must be error
//   - too_many_returns: at most 2 returns allowed (resp, error)
//
// Request type validation errors:
//   - request_not_struct: request type must be struct or *struct
//
// Binding validation errors:
//   - duplicate_path_binding: duplicate path binding for same variable
//   - path_binding_not_in_route: path binding references variable not in route
//   - unsupported_path_type: unsupported type for path variable binding
//   - unsupported_query_type: unsupported type for query parameter binding
//   - unsupported_header_type: unsupported type for header binding
//   - missing_path_binding: route has path variable with no corresponding struct binding

// HandlerShape represents the signature pattern of a handler function.
type HandlerShape int

const (
	// ShapeCtxReqRespErr: func(context.Context, Req) (Resp, error)
	ShapeCtxReqRespErr HandlerShape = iota + 1
	// ShapeCtxReqErr: func(context.Context, Req) error
	ShapeCtxReqErr
	// ShapeCtxRespErr: func(context.Context) (Resp, error)
	ShapeCtxRespErr
	// ShapeCtxErr: func(context.Context) error
	ShapeCtxErr
)

// HandlerInfo contains metadata about a validated handler function.
type HandlerInfo struct {
	Shape    HandlerShape
	ReqType  reflect.Type // nil if no request
	RespType reflect.Type // nil if no response
}

// ValidationError represents a validation failure with a stable error code.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// ValidateHandler validates a handler function and returns its metadata.
// The handler must be one of four supported shapes:
//   - func(context.Context, Req) (Resp, error)
//   - func(context.Context, Req) error
//   - func(context.Context) (Resp, error)
//   - func(context.Context) error
func ValidateHandler(handler any) (*HandlerInfo, error) {
	if handler == nil {
		return nil, &ValidationError{Code: "nil_handler", Message: "handler is nil"}
	}

	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return nil, &ValidationError{Code: "not_a_function", Message: "handler must be a function"}
	}

	if t.IsVariadic() {
		return nil, &ValidationError{Code: "variadic_not_supported", Message: "variadic handlers not supported"}
	}

	// Check args
	if t.NumIn() == 0 {
		return nil, &ValidationError{Code: "missing_context", Message: "first arg must be context.Context"}
	}
	if !t.In(0).Implements(contextType) {
		return nil, &ValidationError{Code: "first_arg_not_context", Message: "first arg must be context.Context"}
	}
	if t.NumIn() > 2 {
		return nil, &ValidationError{Code: "too_many_args", Message: "at most 2 args allowed (ctx, req)"}
	}

	// Check returns
	if t.NumOut() == 0 {
		return nil, &ValidationError{Code: "missing_error_return", Message: "must return error"}
	}
	if !t.Out(t.NumOut() - 1).Implements(errorType) {
		return nil, &ValidationError{Code: "last_return_not_error", Message: "last return must be error"}
	}
	if t.NumOut() > 2 {
		return nil, &ValidationError{Code: "too_many_returns", Message: "at most 2 returns allowed (resp, error)"}
	}

	// Determine shape
	hasReq := t.NumIn() == 2
	hasResp := t.NumOut() == 2

	info := &HandlerInfo{}
	switch {
	case hasReq && hasResp:
		info.Shape = ShapeCtxReqRespErr
		info.ReqType = t.In(1)
		info.RespType = t.Out(0)
	case hasReq && !hasResp:
		info.Shape = ShapeCtxReqErr
		info.ReqType = t.In(1)
	case !hasReq && hasResp:
		info.Shape = ShapeCtxRespErr
		info.RespType = t.Out(0)
	default:
		info.Shape = ShapeCtxErr
	}

	return info, nil
}

// ValidateRequestType validates that a request type is a struct or pointer to struct.
func ValidateRequestType(t reflect.Type) error {
	// Unwrap pointer
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return &ValidationError{
			Code:    "request_not_struct",
			Message: "request type must be struct or *struct",
		}
	}

	return nil
}

// BindingInfo contains analyzed binding metadata for a request type.
type BindingInfo struct {
	HasJSONBody    bool
	PathBindings   []FieldBinding
	QueryBindings  []FieldBinding
	HeaderBindings []FieldBinding
	JSONBindings   []FieldBinding
}

// FieldBinding represents a struct field bound to a request component.
type FieldBinding struct {
	FieldName string
	TagValue  string
	FieldType reflect.Type
}

// bindingPathVarRegex matches {name} or {name...} path variables
var bindingPathVarRegex = regexp.MustCompile(`\{([^}]+?)(?:\.\.\.)?\}`)

// extractPathVariables extracts path variable names from a route pattern.
func extractPathVariables(path string) map[string]bool {
	matches := bindingPathVarRegex.FindAllStringSubmatch(path, -1)
	vars := make(map[string]bool)
	for _, match := range matches {
		vars[match[1]] = true
	}
	return vars
}

// isSupportedQueryType checks if a type can be used for query parameters.
func isSupportedQueryType(t reflect.Type) bool {
	// Handle pointers (optional)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Handle slices (multi-value)
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	// Supported scalar types
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}

	// time.Time is also supported
	if t.String() == "time.Time" {
		return true
	}

	return false
}

// isSupportedHeaderType checks if a type can be used for header binding.
func isSupportedHeaderType(t reflect.Type) bool {
	// Handle pointers (optional)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Supported scalar types for headers
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}

	// time.Time is also supported
	if t.String() == "time.Time" {
		return true
	}

	return false
}

// isSupportedPathType checks if a type can be used for path variable binding.
func isSupportedPathType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

// ValidateBindings validates that struct bindings match the route pattern.
func ValidateBindings(path string, reqType reflect.Type) error {
	_, err := AnalyzeBindings(path, reqType)
	return err
}

// AnalyzeBindings analyzes struct tags and validates bindings against a route pattern.
func AnalyzeBindings(path string, reqType reflect.Type) (*BindingInfo, error) {
	// Unwrap pointer
	if reqType.Kind() == reflect.Ptr {
		reqType = reqType.Elem()
	}

	if reqType.Kind() != reflect.Struct {
		return nil, &ValidationError{
			Code:    "request_not_struct",
			Message: "request type must be struct or *struct",
		}
	}

	pathVars := extractPathVariables(path)
	info := &BindingInfo{}

	// Track which path vars have been bound
	boundPathVars := make(map[string]bool)
	// Track duplicate path tags
	seenPathTags := make(map[string]string) // tag value -> field name

	// Walk struct fields
	for i := 0; i < reqType.NumField(); i++ {
		field := reqType.Field(i)

		// Check path tag
		if pathTag := field.Tag.Get("path"); pathTag != "" {
			// Check for duplicate
			if existingField, exists := seenPathTags[pathTag]; exists {
				return nil, &ValidationError{
					Code:    "duplicate_path_binding",
					Message: fmt.Sprintf("duplicate path binding for %q: fields %s and %s", pathTag, existingField, field.Name),
				}
			}
			seenPathTags[pathTag] = field.Name

			// Check if path var exists in route
			if !pathVars[pathTag] {
				return nil, &ValidationError{
					Code:    "path_binding_not_in_route",
					Message: fmt.Sprintf("path binding %q is not in route pattern", pathTag),
				}
			}

			// Check type is supported
			if !isSupportedPathType(field.Type) {
				return nil, &ValidationError{
					Code:    "unsupported_path_type",
					Message: fmt.Sprintf("unsupported type for path binding %q: %s", pathTag, field.Type),
				}
			}

			info.PathBindings = append(info.PathBindings, FieldBinding{
				FieldName: field.Name,
				TagValue:  pathTag,
				FieldType: field.Type,
			})
			boundPathVars[pathTag] = true
		}

		// Check query tag
		if queryTag := field.Tag.Get("query"); queryTag != "" {
			if !isSupportedQueryType(field.Type) {
				return nil, &ValidationError{
					Code:    "unsupported_query_type",
					Message: fmt.Sprintf("unsupported type for query binding %q: %s", queryTag, field.Type),
				}
			}

			info.QueryBindings = append(info.QueryBindings, FieldBinding{
				FieldName: field.Name,
				TagValue:  queryTag,
				FieldType: field.Type,
			})
		}

		// Check header tag
		if headerTag := field.Tag.Get("header"); headerTag != "" {
			if !isSupportedHeaderType(field.Type) {
				return nil, &ValidationError{
					Code:    "unsupported_header_type",
					Message: fmt.Sprintf("unsupported type for header binding %q: %s", headerTag, field.Type),
				}
			}

			info.HeaderBindings = append(info.HeaderBindings, FieldBinding{
				FieldName: field.Name,
				TagValue:  headerTag,
				FieldType: field.Type,
			})
		}

		// Check json tag
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			info.JSONBindings = append(info.JSONBindings, FieldBinding{
				FieldName: field.Name,
				TagValue:  jsonTag,
				FieldType: field.Type,
			})
		}
	}

	// Check all path vars are bound
	for pathVar := range pathVars {
		if !boundPathVars[pathVar] {
			return nil, &ValidationError{
				Code:    "missing_path_binding",
				Message: fmt.Sprintf("path variable {%s} has no corresponding struct field with path:\"%s\" tag", pathVar, pathVar),
			}
		}
	}

	info.HasJSONBody = len(info.JSONBindings) > 0

	return info, nil
}

// ValidateEndpoint validates an endpoint's handler signature and request bindings.
func ValidateEndpoint(ep Endpoint) error {
	// 1. Validate handler signature
	info, err := ValidateHandler(ep.Handler)
	if err != nil {
		return fmt.Errorf("endpoint %s %s: %w", ep.Method, ep.Path, err)
	}

	// 2. If handler has request type, validate request type and bindings
	if info.ReqType != nil {
		if err := ValidateRequestType(info.ReqType); err != nil {
			return fmt.Errorf("endpoint %s %s: %w", ep.Method, ep.Path, err)
		}
		if err := ValidateBindings(ep.Path, info.ReqType); err != nil {
			return fmt.Errorf("endpoint %s %s: %w", ep.Method, ep.Path, err)
		}
	}

	return nil
}
