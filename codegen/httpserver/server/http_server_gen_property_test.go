package server

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/proptest"
)

// queryParamTypes lists Go types that can appear as query parameters.
var queryParamTypes = []string{"string", "*string", "int", "int64", "int32", "uint64", "bool"}

// goKeywords is the set of Go reserved keywords that cannot be used as identifiers.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// nonKeywordIdentifier generates a random identifier that is not a Go keyword.
func nonKeywordIdentifier(g *proptest.Generator, maxLen int) string {
	for {
		id := g.Identifier(maxLen)
		if !goKeywords[id] {
			return id
		}
	}
}

// nonKeywordIdentifierLower generates a random lowercase identifier that is not a Go keyword.
func nonKeywordIdentifierLower(g *proptest.Generator, maxLen int) string {
	for {
		id := g.IdentifierLower(maxLen)
		if !goKeywords[id] {
			return id
		}
	}
}

// generateRandomHandlers generates random handler definitions for property testing.
// All handlers share the same package to make valid per-resource files.
func generateRandomHandlers(g *proptest.Generator) []codegen.SerializedHandlerInfo {
	numHandlers := g.IntRange(0, 10)
	handlers := make([]codegen.SerializedHandlerInfo, numHandlers)

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

	// Use a single resource package to ensure valid grouped output
	pkgName := nonKeywordIdentifierLower(g, 10)

	for i := 0; i < numHandlers; i++ {
		method := methods[g.Intn(len(methods))]
		path := generateRandomPath(g)
		funcName := nonKeywordIdentifier(g, 20)

		handlers[i] = codegen.SerializedHandlerInfo{
			Method:      method,
			Path:        path,
			FuncName:    funcName,
			PackagePath: "example.com/app/api/" + pkgName,
			PathParams:  extractTestPathParams(path),
			Request:     generateRandomStructInfo(g, funcName+"Request"),
			Response:    generateRandomStructInfo(g, funcName+"Response"),
		}
	}

	return handlers
}

// generateRandomMultiPkgHandlers generates random handlers across multiple packages.
func generateRandomMultiPkgHandlers(g *proptest.Generator) []codegen.SerializedHandlerInfo {
	numPkgs := g.IntRange(1, 4)
	var handlers []codegen.SerializedHandlerInfo

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	usedPkgNames := make(map[string]bool)

	for p := 0; p < numPkgs; p++ {
		pkgName := nonKeywordIdentifierLower(g, 10)
		for usedPkgNames[pkgName] {
			pkgName = nonKeywordIdentifierLower(g, 10)
		}
		usedPkgNames[pkgName] = true

		numInPkg := g.IntRange(1, 5)
		for i := 0; i < numInPkg; i++ {
			method := methods[g.Intn(len(methods))]
			path := "/" + pkgName + generateRandomPath(g)
			funcName := nonKeywordIdentifier(g, 20)

			handlers = append(handlers, codegen.SerializedHandlerInfo{
				Method:      method,
				Path:        path,
				FuncName:    funcName,
				PackagePath: "example.com/app/api/" + pkgName,
				PathParams:  extractTestPathParams(path),
				Request:     generateRandomStructInfo(g, funcName+"Request"),
				Response:    generateRandomStructInfo(g, funcName+"Response"),
			})
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
func extractTestPathParams(path string) []codegen.SerializedPathParam {
	var params []codegen.SerializedPathParam
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			params = append(params, codegen.SerializedPathParam{
				Name:     strings.TrimPrefix(part, ":"),
				Position: i,
			})
		}
	}
	return params
}

// generateRandomStructInfo generates a random struct definition for testing.
func generateRandomStructInfo(g *proptest.Generator, name string) *codegen.SerializedStructInfo {
	if g.BoolWithProb(0.1) {
		return nil // Some handlers have no request/response
	}

	numFields := g.IntRange(1, 5)
	fields := make([]codegen.SerializedFieldInfo, numFields)

	types := []string{"string", "int64", "bool"}

	for i := 0; i < numFields; i++ {
		fieldName := nonKeywordIdentifier(g, 15)
		fieldType := types[g.Intn(len(types))]
		field := codegen.SerializedFieldInfo{
			Name:     fieldName,
			Type:     fieldType,
			JSONName: strings.ToLower(fieldName),
			Required: g.Bool(),
		}
		// 30% chance of adding a query tag
		if g.BoolWithProb(0.3) {
			qType := queryParamTypes[g.Intn(len(queryParamTypes))]
			field.Type = qType
			queryName := g.IdentifierLower(8)
			field.Tags = map[string]string{"query": queryName}
		}
		fields[i] = field
	}

	return &codegen.SerializedStructInfo{
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
		handlers := generateRandomMultiPkgHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			t.Logf("GenerateHTTPServer error: %v", err)
			return false
		}

		for _, f := range files {
			_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
			if err != nil {
				t.Logf("Parse error in %s: %v\nCode:\n%s", f.RelPath, err, string(f.Content))
				return false
			}
		}
		return true
	})
}

// TestProperty_GenerateHTTPServer_AllHandlersRouted tests that every handler appears in a RegisterRoutes.
func TestProperty_GenerateHTTPServer_AllHandlersRouted(t *testing.T) {
	proptest.QuickCheck(t, "all handlers have routes", func(g *proptest.Generator) bool {
		handlers := generateRandomMultiPkgHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			t.Logf("GenerateHTTPServer failed: %v", err)
			return false
		}

		// Concatenate all resource file contents
		var allCode string
		for _, f := range files {
			allCode += string(f.Content)
		}

		for _, h := range handlers {
			expectedPath := codegen.ConvertPathSyntax(h.Path)
			expectedRoute := h.Method + " " + expectedPath
			if !strings.Contains(allCode, expectedRoute) {
				t.Logf("Missing route %q for handler %s", expectedRoute, h.FuncName)
				return false
			}
		}

		return true
	})
}

// TestProperty_ConvertPathSyntax_Roundtrip tests that path conversion preserves all params.
func TestProperty_ConvertPathSyntax_Roundtrip(t *testing.T) {
	proptest.QuickCheck(t, "path conversion preserves params", func(g *proptest.Generator) bool {
		numSegments := g.IntRange(1, 5)
		segments := make([]string, numSegments)
		paramNames := []string{}

		for i := 0; i < numSegments; i++ {
			if g.Bool() {
				name := g.IdentifierLower(10)
				segments[i] = ":" + name
				paramNames = append(paramNames, name)
			} else {
				segments[i] = g.IdentifierLower(10)
			}
		}

		originalPath := "/" + strings.Join(segments, "/")
		convertedPath := codegen.ConvertPathSyntax(originalPath)

		for _, name := range paramNames {
			if !strings.Contains(convertedPath, "{"+name+"}") {
				t.Logf("Missing param %s in converted path %s", name, convertedPath)
				return false
			}
		}

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
		handlers := generateRandomMultiPkgHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		for _, f := range files {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, "", f.Content, parser.ImportsOnly)
			if err != nil {
				return false
			}

			seen := make(map[string]bool)
			for _, imp := range parsed.Imports {
				path := imp.Path.Value
				if seen[path] {
					t.Logf("Duplicate import %s in %s", path, f.RelPath)
					return false
				}
				seen[path] = true
			}
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_TypeConversions tests that type conversions are generated for non-string params.
func TestProperty_GenerateHTTPServer_TypeConversions(t *testing.T) {
	proptest.Check(t, "type conversions generated", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		handler := codegen.SerializedHandlerInfo{
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
					{Name: "ID", Type: "int64", JSONName: "id"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "GetUserResponse",
				Package: "example.com/app/api/users",
				Fields:  []codegen.SerializedFieldInfo{},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []codegen.SerializedHandlerInfo{handler},
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		resFile := findResourceHTTP(files, "users")
		if resFile == nil {
			t.Log("Missing users resource file")
			return false
		}
		codeStr := string(resFile.Content)

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
			Handlers:   []codegen.SerializedHandlerInfo{},
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		if len(files) != 1 {
			t.Logf("Expected 1 file for empty registry, got %d", len(files))
			return false
		}

		topLevel := findTopLevel(files)
		codeStr := string(topLevel.Content)

		if !strings.Contains(codeStr, "func NewMux") {
			t.Log("Missing NewMux function")
			return false
		}

		_, err = parser.ParseFile(token.NewFileSet(), "", topLevel.Content, parser.AllErrors)
		return err == nil
	})
}

// TestProperty_GenerateHTTPServer_HandlerWrapperNames tests that handler wrapper names are generated.
func TestProperty_GenerateHTTPServer_HandlerWrapperNames(t *testing.T) {
	proptest.QuickCheck(t, "handler wrapper names are generated", func(g *proptest.Generator) bool {
		handlers := generateRandomMultiPkgHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		var allCode string
		for _, f := range files {
			allCode += string(f.Content)
		}

		for _, h := range handlers {
			wrapperName := "func handle" + h.FuncName
			if !strings.Contains(allCode, wrapperName) {
				t.Logf("Missing wrapper: %s", wrapperName)
				return false
			}
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_JSONBodyForMutatingMethods tests JSON body binding for POST/PUT/PATCH.
func TestProperty_GenerateHTTPServer_JSONBodyForMutatingMethods(t *testing.T) {
	proptest.Check(t, "JSON body binding for mutating methods", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		methods := []string{"POST", "PUT", "PATCH"}
		method := methods[g.Intn(len(methods))]

		handler := codegen.SerializedHandlerInfo{
			Method:      method,
			Path:        "/users",
			FuncName:    "MutateUser",
			PackagePath: "example.com/app/api/users",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "MutateUserRequest",
				Package: "example.com/app/api/users",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name"},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "MutateUserResponse",
				Package: "example.com/app/api/users",
				Fields:  []codegen.SerializedFieldInfo{},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []codegen.SerializedHandlerInfo{handler},
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		resFile := findResourceHTTP(files, "users")
		if resFile == nil {
			t.Log("Missing users resource file")
			return false
		}
		codeStr := string(resFile.Content)

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
		postHandler := codegen.SerializedHandlerInfo{
			Method:      "POST",
			Path:        "/users",
			FuncName:    "CreateUser",
			PackagePath: "example.com/app/api/users",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "CreateUserRequest",
				Package: "example.com/app/api/users",
				Fields:  []codegen.SerializedFieldInfo{},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "CreateUserResponse",
				Package: "example.com/app/api/users",
				Fields:  []codegen.SerializedFieldInfo{},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []codegen.SerializedHandlerInfo{postHandler},
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		resFile := findResourceHTTP(files, "users")
		if resFile == nil {
			return false
		}
		codeStr := string(resFile.Content)

		if !strings.Contains(codeStr, "http.StatusCreated") {
			t.Log("POST handler should use http.StatusCreated")
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_QueryParamBindingValidGo tests that generated code
// with query-tagged fields is always valid Go.
func TestProperty_GenerateHTTPServer_QueryParamBindingValidGo(t *testing.T) {
	proptest.QuickCheck(t, "query param binding produces valid Go", func(g *proptest.Generator) bool {
		// Generate handlers where some fields have query tags (generateRandomStructInfo now does this)
		handlers := generateRandomMultiPkgHandlers(g)

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			t.Logf("GenerateHTTPServer error: %v", err)
			return false
		}

		for _, f := range files {
			_, err = parser.ParseFile(token.NewFileSet(), "", f.Content, parser.AllErrors)
			if err != nil {
				t.Logf("Parse error in %s: %v\nCode:\n%s", f.RelPath, err, string(f.Content))
				return false
			}
		}
		return true
	})
}

// TestProperty_GenerateHTTPServer_QueryFieldsNeverInJSONDecode tests that for GET handlers
// with only query-tagged request fields, json.NewDecoder(r.Body) must be absent.
func TestProperty_GenerateHTTPServer_QueryFieldsNeverInJSONDecode(t *testing.T) {
	proptest.Check(t, "query-only GET has no JSON decode", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		// Build a GET handler where ALL request fields have query tags
		numFields := g.IntRange(1, 4)
		fields := make([]codegen.SerializedFieldInfo, numFields)
		for i := 0; i < numFields; i++ {
			fieldName := nonKeywordIdentifier(g, 12)
			qType := queryParamTypes[g.Intn(len(queryParamTypes))]
			queryName := g.IdentifierLower(8)
			fields[i] = codegen.SerializedFieldInfo{
				Name:     fieldName,
				Type:     qType,
				JSONName: strings.ToLower(fieldName),
				Tags:     map[string]string{"query": queryName},
			}
		}

		handler := codegen.SerializedHandlerInfo{
			Method:      "GET",
			Path:        "/things",
			FuncName:    "ListThings",
			PackagePath: "example.com/app/api/things",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "ListThingsRequest",
				Package: "example.com/app/api/things",
				Fields:  fields,
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "ListThingsResponse",
				Package: "example.com/app/api/things",
				Fields:  []codegen.SerializedFieldInfo{{Name: "Items", Type: "[]string", JSONName: "items"}},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []codegen.SerializedHandlerInfo{handler},
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			t.Logf("GenerateHTTPServer error: %v", err)
			return false
		}

		resFile := findResourceHTTP(files, "things")
		if resFile == nil {
			t.Log("Missing things resource file")
			return false
		}
		codeStr := string(resFile.Content)

		if strings.Contains(codeStr, "json.NewDecoder(r.Body)") {
			t.Log("GET handler with only query fields should NOT have json.NewDecoder(r.Body)")
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_QueryFieldsAlwaysBound tests that for every handler
// with query-tagged fields, the generated code contains r.URL.Query().
func TestProperty_GenerateHTTPServer_QueryFieldsAlwaysBound(t *testing.T) {
	proptest.Check(t, "query fields always bound", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
		method := methods[g.Intn(len(methods))]

		// Build a handler with at least one query-tagged field
		qType := queryParamTypes[g.Intn(len(queryParamTypes))]
		queryName := g.IdentifierLower(8)
		fieldName := nonKeywordIdentifier(g, 12)

		handler := codegen.SerializedHandlerInfo{
			Method:      method,
			Path:        "/items",
			FuncName:    "DoItems",
			PackagePath: "example.com/app/api/items",
			PathParams:  []codegen.SerializedPathParam{},
			Request: &codegen.SerializedStructInfo{
				Name:    "DoItemsRequest",
				Package: "example.com/app/api/items",
				Fields: []codegen.SerializedFieldInfo{
					{
						Name:     fieldName,
						Type:     qType,
						JSONName: strings.ToLower(fieldName),
						Tags:     map[string]string{"query": queryName},
					},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name:    "DoItemsResponse",
				Package: "example.com/app/api/items",
				Fields:  []codegen.SerializedFieldInfo{{Name: "OK", Type: "bool", JSONName: "ok"}},
			},
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   []codegen.SerializedHandlerInfo{handler},
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			t.Logf("GenerateHTTPServer error: %v", err)
			return false
		}

		resFile := findResourceHTTP(files, "items")
		if resFile == nil {
			t.Log("Missing items resource file")
			return false
		}
		codeStr := string(resFile.Content)

		if !strings.Contains(codeStr, "r.URL.Query()") {
			t.Logf("Handler with query-tagged field must contain r.URL.Query()\nCode:\n%s", codeStr)
			return false
		}

		return true
	})
}

// TestProperty_GenerateHTTPServer_ErrorHandling tests that error handling is present.
func TestProperty_GenerateHTTPServer_ErrorHandling(t *testing.T) {
	proptest.QuickCheck(t, "error handling present", func(g *proptest.Generator) bool {
		handlers := generateRandomMultiPkgHandlers(g)
		if len(handlers) == 0 {
			return true
		}

		cfg := HTTPServerGenConfig{
			ModulePath: "example.com/app",
			Handlers:   handlers,
			OutputPkg:  "api",
		}

		files, err := GenerateHTTPServer(cfg)
		if err != nil {
			return false
		}

		// Check that resource files use httputil.WriteError
		var allResourceCode string
		for _, f := range files {
			if !strings.HasSuffix(f.RelPath, "api/zz_generated_http.go") {
				allResourceCode += string(f.Content)
			}
		}

		if !strings.Contains(allResourceCode, "httputil.WriteError") {
			t.Log("Missing httputil.WriteError function call")
			return false
		}

		return true
	})
}
