package handler

import (
	"context"
	"reflect"
	"regexp"
	"strings"
)

// App is a registration shim that captures handler metadata.
// It is NOT an actual HTTP router - it exists purely to collect
// information for code generation.
type App struct {
	registry *Registry
}

// NewApp creates a new App for handler registration.
func NewApp() *App {
	return &App{
		registry: NewRegistry(),
	}
}

// Registry returns the captured handler registry.
func (a *App) Registry() *Registry {
	return a.registry
}

// Get registers a GET handler.
func (a *App) Get(path string, handler any) {
	a.register(GET, path, handler)
}

// Post registers a POST handler.
func (a *App) Post(path string, handler any) {
	a.register(POST, path, handler)
}

// Put registers a PUT handler.
func (a *App) Put(path string, handler any) {
	a.register(PUT, path, handler)
}

// Patch registers a PATCH handler.
func (a *App) Patch(path string, handler any) {
	a.register(PATCH, path, handler)
}

// Delete registers a DELETE handler.
func (a *App) Delete(path string, handler any) {
	a.register(DELETE, path, handler)
}

func (a *App) register(method HTTPMethod, path string, handler any) {
	info := HandlerInfo{
		Method:     method,
		Path:       path,
		PathParams: extractPathParams(path),
	}

	// Use reflection to extract handler metadata
	handlerType := reflect.TypeOf(handler)

	// Validate handler signature: func(context.Context, *Request) (*Response, error)
	if handlerType.Kind() != reflect.Func {
		panic("handler must be a function")
	}
	if handlerType.NumIn() != 2 {
		panic("handler must have exactly 2 parameters: (context.Context, *Request)")
	}
	if handlerType.NumOut() != 2 {
		panic("handler must have exactly 2 return values: (*Response, error)")
	}

	// Validate first parameter is context.Context
	ctxType := handlerType.In(0)
	if ctxType.PkgPath() != "context" || ctxType.Name() != "Context" {
		panic("handler's first parameter must be context.Context")
	}

	// Validate second return value is error
	errType := handlerType.Out(1)
	if errType.PkgPath() != "" || errType.Name() != "error" {
		panic("handler's second return value must be error")
	}

	// Extract request type (second parameter)
	reqType := handlerType.In(1)
	if reqType.Kind() == reflect.Ptr {
		reqType = reqType.Elem()
	}
	info.Request = extractStructInfo(reqType)

	// Extract response type (first return value)
	respType := handlerType.Out(0)
	if respType.Kind() == reflect.Ptr {
		respType = respType.Elem()
	}
	info.Response = extractStructInfo(respType)

	// NOTE: Function name is NOT set here. It will be filled in by static
	// analysis of the Register function source code. See handler_static_analysis.go.

	a.registry.Handlers = append(a.registry.Handlers, info)
}

// extractPathParams parses path parameters from a URL pattern.
// Supports :param syntax (e.g., "/users/:id/posts/:post_id")
func extractPathParams(path string) []PathParam {
	var params []PathParam
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			params = append(params, PathParam{
				Name:     strings.TrimPrefix(part, ":"),
				Position: i,
			})
		}
	}
	return params
}

// extractStructInfo builds a StructInfo from a reflect.Type.
func extractStructInfo(t reflect.Type) *StructInfo {
	if t.Kind() != reflect.Struct {
		return nil
	}

	info := &StructInfo{
		Name:        t.Name(),
		Package:     t.PkgPath(),
		Fields:      make([]FieldInfo, 0, t.NumField()),
		ReflectType: t,
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldInfo := FieldInfo{
			Name: field.Name,
			Type: typeToString(field.Type),
			Tags: make(map[string]string),
		}

		// Parse JSON tag
		if jsonTag, ok := field.Tag.Lookup("json"); ok {
			parts := strings.Split(jsonTag, ",")
			if parts[0] == "-" {
				fieldInfo.JSONOmit = true
				fieldInfo.JSONName = ""
			} else {
				fieldInfo.JSONName = parts[0]
				for _, opt := range parts[1:] {
					if opt == "omitempty" {
						fieldInfo.JSONOmit = true
					}
				}
			}
		} else {
			fieldInfo.JSONName = field.Name
		}

		// Determine if required: not omitempty, not a pointer, not a slice/map
		fieldInfo.Required = !fieldInfo.JSONOmit &&
			field.Type.Kind() != reflect.Ptr &&
			field.Type.Kind() != reflect.Slice &&
			field.Type.Kind() != reflect.Map

		// Store all tags for extensibility
		tagStr := string(field.Tag)
		// Simple tag parsing using regex
		tagRegex := regexp.MustCompile(`(\w+):"([^"]*)"`)
		matches := tagRegex.FindAllStringSubmatch(tagStr, -1)
		for _, match := range matches {
			fieldInfo.Tags[match[1]] = match[2]
		}

		info.Fields = append(info.Fields, fieldInfo)
	}

	return info
}

// typeToString converts a reflect.Type to a string representation.
func typeToString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + typeToString(t.Elem())
	case reflect.Slice:
		return "[]" + typeToString(t.Elem())
	case reflect.Map:
		return "map[" + typeToString(t.Key()) + "]" + typeToString(t.Elem())
	default:
		if t.PkgPath() != "" {
			return t.PkgPath() + "." + t.Name()
		}
		return t.Name()
	}
}

// Ensure we reference context to validate the import
var _ context.Context
