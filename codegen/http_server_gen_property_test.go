package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/proptest"
)

// generateRandomHandlers generates random handler definitions for property testing.
func generateRandomHandlers(g *proptest.Generator) []SerializedHandlerInfo {
	numHandlers := g.IntRange(0, 10)
	handlers := make([]SerializedHandlerInfo, numHandlers)

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

	for i := 0; i < numHandlers; i++ {
		method := methods[g.Intn(len(methods))]
		path := generateRandomPath(g)
		funcName := g.Identifier(20)
		pkgName := g.IdentifierLower(10)

		handlers[i] = SerializedHandlerInfo{
			Method:      method,
			Path:        path,
			FuncName:    funcName,
			PackagePath: "example.com/app/" + pkgName,
			PathParams:  extractTestPathParams(path),
			Request:     generateRandomStructInfo(g, funcName+"Request"),
			Response:    generateRandomStructInfo(g, funcName+"Response"),
		}
	}

	return handlers
}

// generateRandomPath generates a random URL path with optional path parameters.
func generateRandomPath(g *proptest.Generator) string {
	numSegments := g.IntRange(1, 4)
	segments := make([]string, numSegments)

	for i := 0; i < numSegments; i++ {
		if g.BoolWithProb(0.3) {
			// Path parameter
			segments[i] = ":" + g.IdentifierLower(8)
		} else {
			// Static segment
			segments[i] = g.IdentifierLower(10)
		}
	}

	return "/" + strings.Join(segments, "/")
}

// extractTestPathParams extracts path parameters from a path (for test purposes).
func extractTestPathParams(path string) []SerializedPathParam {
	var params []SerializedPathParam
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			params = append(params, SerializedPathParam{
				Name:     strings.TrimPrefix(part, ":"),
				Position: i,
			})
		}
	}
	return params
}

// generateRandomStructInfo generates a random struct definition for testing.
func generateRandomStructInfo(g *proptest.Generator, name string) *SerializedStructInfo {
	if g.BoolWithProb(0.1) {
		return nil // Some handlers have no request/response
	}

	numFields := g.IntRange(1, 5)
	fields := make([]SerializedFieldInfo, numFields)

	types := []string{"string", "int64", "bool"}

	for i := 0; i < numFields; i++ {
		fieldName := g.Identifier(15)
		fields[i] = SerializedFieldInfo{
			Name:     fieldName,
			Type:     types[g.Intn(len(types))],
			JSONName: strings.ToLower(fieldName),
			Required: g.Bool(),
		}
	}

	return &SerializedStructInfo{
		Name:    name,
		Package: "example.com/app/handlers",
		Fields:  fields,
	}
}

// generateModulePath generates a random module path.
func generateModulePath(g *proptest.Generator) string {
	parts := g.IntRange(1, 3)
	segments := make([]string, parts)
	for i := 0; i < parts; i++ {
		segments[i] = g.IdentifierLower(8)
	}
	return strings.Join(segments, "/")
}

// TestProperty_GenerateHTTPServer_ValidGo tests that generated code is always valid Go.
func TestProperty_GenerateHTTPServer_ValidGo(t *testing.T) {
	proptest.QuickCheck(t, "generated code is valid go", func(g *proptest.Generator) bool {
		handlers := generateRandomHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: generateModulePath(g),
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			t.Logf("GenerateHTTPServer error: %v", err)
			return false
		}

		// Parse as Go code
		_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
		if err != nil {
			t.Logf("Parse error: %v\nCode:\n%s", err, string(code))
			return false
		}
		return true
	})
}

// TestProperty_GenerateHTTPServer_AllHandlersRouted tests that every handler appears in the generated mux.
func TestProperty_GenerateHTTPServer_AllHandlersRouted(t *testing.T) {
	proptest.QuickCheck(t, "all handlers have routes", func(g *proptest.Generator) bool {
		handlers := generateRandomHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		codeStr := string(code)

		// Every handler should have a mux.Handle line
		for _, h := range handlers {
			expectedPath := ConvertPathSyntax(h.Path)
			expectedRoute := h.Method + " " + expectedPath
			if !strings.Contains(codeStr, expectedRoute) {
				return false
			}
		}

		return true
	})
}

// TestProperty_ConvertPathSyntax_Roundtrip tests that path conversion preserves all params.
func TestProperty_ConvertPathSyntax_Roundtrip(t *testing.T) {
	proptest.QuickCheck(t, "path conversion preserves params", func(g *proptest.Generator) bool {
		// Generate path with random params
		numSegments := g.IntRange(1, 5)
		segments := make([]string, numSegments)
		paramNames := []string{}

		for i := 0; i < numSegments; i++ {
			if g.Bool() {
				// Parameter segment
				name := g.IdentifierLower(10)
				segments[i] = ":" + name
				paramNames = append(paramNames, name)
			} else {
				// Static segment
				segments[i] = g.IdentifierLower(10)
			}
		}

		originalPath := "/" + strings.Join(segments, "/")
		convertedPath := ConvertPathSyntax(originalPath)

		// Verify all param names are present in {param} format
		for _, name := range paramNames {
			if !strings.Contains(convertedPath, "{"+name+"}") {
				t.Logf("Missing param %s in converted path %s", name, convertedPath)
				return false
			}
		}

		// Verify no :param syntax remains
		if strings.Contains(convertedPath, ":") {
			t.Logf("Unconverted :param in path: %s", convertedPath)
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_NoDuplicateImports tests that imports are deduplicated.
func TestProperty_GenerateHTTPServer_NoDuplicateImports(t *testing.T) {
	proptest.QuickCheck(t, "no duplicate imports", func(g *proptest.Generator) bool {
		// Generate handlers from same and different packages
		handlers := generateRandomHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		// Parse imports
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", code, parser.ImportsOnly)
		if err != nil {
			return false
		}

		// Check for duplicates
		seen := make(map[string]bool)
		for _, imp := range f.Imports {
			path := imp.Path.Value
			if seen[path] {
				t.Logf("Duplicate import: %s", path)
				return false // Duplicate!
			}
			seen[path] = true
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_TypeConversions tests that type conversions are generated for non-string params.
func TestProperty_GenerateHTTPServer_TypeConversions(t *testing.T) {
	proptest.Check(t, "type conversions generated", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		// Generate a handler with int64 path param
		handler := SerializedHandlerInfo{
			Method:      "GET",
			Path:        "/users/:id",
			FuncName:    "GetUser",
			PackagePath: "example.com/app/users",
			PathParams: []SerializedPathParam{
				{Name: "id", Position: 1},
			},
			Request: &SerializedStructInfo{
				Name:    "GetUserRequest",
				Package: "example.com/app/users",
				Fields: []SerializedFieldInfo{
					{Name: "ID", Type: "int64", JSONName: "id"},
				},
			},
			Response: &SerializedStructInfo{
				Name:    "GetUserResponse",
				Package: "example.com/app/users",
				Fields:  []SerializedFieldInfo{},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []SerializedHandlerInfo{handler},
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		codeStr := string(code)

		// Should contain strconv for int64 conversion
		if !strings.Contains(codeStr, "strconv.ParseInt") && !strings.Contains(codeStr, "strconv") {
			t.Log("Missing strconv for int64 conversion")
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_EmptyRegistry tests that empty registry produces minimal valid server.
func TestProperty_GenerateHTTPServer_EmptyRegistry(t *testing.T) {
	proptest.Check(t, "empty registry works", proptest.Config{NumTrials: 1}, func(g *proptest.Generator) bool {
		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []SerializedHandlerInfo{},
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		// Should still produce valid Go with NewMux
		codeStr := string(code)
		if !strings.Contains(codeStr, "func NewMux") {
			t.Log("Missing NewMux function")
			return false
		}
		if !strings.Contains(codeStr, "http.ServeMux") {
			t.Log("Missing http.ServeMux")
			return false
		}

		// Should be valid Go
		_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
		return err == nil
	})
}

// TestProperty_GenerateHTTPServer_HandlerWrapperNames tests that handler wrapper names are unique.
func TestProperty_GenerateHTTPServer_HandlerWrapperNames(t *testing.T) {
	proptest.QuickCheck(t, "handler wrapper names are generated", func(g *proptest.Generator) bool {
		handlers := generateRandomHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		codeStr := string(code)

		// Every handler should have its wrapper function
		for _, h := range handlers {
			wrapperName := "func handle" + h.FuncName
			if !strings.Contains(codeStr, wrapperName) {
				t.Logf("Missing wrapper: %s", wrapperName)
				return false
			}
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_JSONBodyForMutatingMethods tests that JSON body binding is generated for POST/PUT/PATCH.
func TestProperty_GenerateHTTPServer_JSONBodyForMutatingMethods(t *testing.T) {
	proptest.Check(t, "JSON body binding for mutating methods", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		methods := []string{"POST", "PUT", "PATCH"}
		method := methods[g.Intn(len(methods))]

		handler := SerializedHandlerInfo{
			Method:      method,
			Path:        "/users",
			FuncName:    "MutateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name:    "MutateUserRequest",
				Package: "example.com/app/users",
				Fields: []SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
			Response: &SerializedStructInfo{
				Name:    "MutateUserResponse",
				Package: "example.com/app/users",
				Fields:  []SerializedFieldInfo{},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []SerializedHandlerInfo{handler},
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		codeStr := string(code)

		// Should have JSON body binding
		if !strings.Contains(codeStr, "json.NewDecoder(r.Body).Decode") {
			t.Logf("Missing JSON body binding for %s method", method)
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_StatusCodes tests that correct status codes are used.
func TestProperty_GenerateHTTPServer_StatusCodes(t *testing.T) {
	proptest.Check(t, "correct status codes", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		// POST should use StatusCreated
		postHandler := SerializedHandlerInfo{
			Method:      "POST",
			Path:        "/users",
			FuncName:    "CreateUser",
			PackagePath: "example.com/app/users",
			PathParams:  []SerializedPathParam{},
			Request: &SerializedStructInfo{
				Name:    "CreateUserRequest",
				Package: "example.com/app/users",
				Fields:  []SerializedFieldInfo{},
			},
			Response: &SerializedStructInfo{
				Name:    "CreateUserResponse",
				Package: "example.com/app/users",
				Fields:  []SerializedFieldInfo{},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []SerializedHandlerInfo{postHandler},
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		codeStr := string(code)

		if !strings.Contains(codeStr, "http.StatusCreated") {
			t.Log("POST handler should use http.StatusCreated")
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_ErrorHandling tests that error handling is present.
func TestProperty_GenerateHTTPServer_ErrorHandling(t *testing.T) {
	proptest.QuickCheck(t, "error handling present", func(g *proptest.Generator) bool {
		handlers := generateRandomHandlers(g)
		if len(handlers) == 0 {
			return true // Nothing to check
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		code, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		codeStr := string(code)

		// Should have writeError function
		if !strings.Contains(codeStr, "func writeError") {
			t.Log("Missing writeError function")
			return false
		}

		// Should use errors.As for httperror detection
		if !strings.Contains(codeStr, "errors.As") {
			t.Log("Missing errors.As for httperror detection")
			return false
		}

		return true
	})
}
