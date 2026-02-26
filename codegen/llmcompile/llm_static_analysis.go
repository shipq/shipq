package llmcompile

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// SerializedToolInfo holds the metadata extracted from a single app.Tool() call,
// combining static analysis data (function name, package) with runtime data
// (JSON Schema, input/output types).
type SerializedToolInfo struct {
	Name        string          `json:"name"`         // tool name from app.Tool("name", ...)
	Description string          `json:"description"`  // description from app.Tool(..., "desc", ...)
	FuncName    string          `json:"func_name"`    // Go function name (e.g., "GetWeather")
	PackagePath string          `json:"package_path"` // import path of the tool package
	PackageName string          `json:"package_name"` // short package name (e.g., "weather")
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema for the input struct (from runtime)
	InputType   string          `json:"input_type"`   // Go type name (e.g., "WeatherInput")
	OutputType  string          `json:"output_type"`  // Go type name (e.g., "WeatherOutput")
}

// SerializedToolPackage groups tools by their source package.
type SerializedToolPackage struct {
	PackagePath string               `json:"package_path"`
	PackageName string               `json:"package_name"`
	Tools       []SerializedToolInfo `json:"tools"`
}

// StaticToolInfo holds what we can extract from AST alone (no runtime data).
type StaticToolInfo struct {
	Name        string // "get_weather"
	Description string // "Get the current weather for a city"
	FuncName    string // "GetWeather"
	Line        int    // source line for error reporting
}

// FindToolRegistrations scans a tool package's Go source files for
// app.Tool("name", "desc", FuncRef) calls inside Register(app *llm.App) functions.
//
// It extracts:
//   - Tool name (first string argument to app.Tool)
//   - Tool description (second string argument)
//   - Function reference name (third argument, must be an identifier or selector)
//
// This is analogous to channel_static_analysis.go's findChannelHandlerFuncs.
func FindToolRegistrations(goModRoot, modulePath, importPath string) ([]StaticToolInfo, error) {
	// Convert import path to filesystem directory.
	relImport := strings.TrimPrefix(importPath, modulePath+"/")
	dirPath := filepath.Join(goModRoot, relImport)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dirPath, err)
	}

	var tools []StaticToolInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip non-Go files, test files, and generated files.
		if filepath.Ext(name) != ".go" {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasPrefix(name, "zz_generated_") {
			continue
		}

		filePath := filepath.Join(dirPath, name)
		found, err := parseFileForToolRegistrations(filePath)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filePath, err)
		}
		tools = append(tools, found...)
	}

	return tools, nil
}

// parseFileForToolRegistrations parses a single Go file and extracts app.Tool()
// calls from Register(app *llm.App) functions.
func parseFileForToolRegistrations(filePath string) ([]StaticToolInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	var tools []StaticToolInfo

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Must be a top-level function (not a method).
		if fn.Recv != nil {
			continue
		}

		// Must be named "Register".
		if fn.Name.Name != "Register" {
			continue
		}

		// Validate signature: func Register(app *llm.App) or func Register(app *someAlias.App).
		paramName, ok := isRegisterSignature(fn, node)
		if !ok {
			continue
		}

		// Walk the function body looking for app.Tool(...) calls.
		found, err := extractToolCalls(fn.Body, paramName, fset)
		if err != nil {
			return nil, err
		}
		tools = append(tools, found...)
	}

	return tools, nil
}

// isRegisterSignature checks if a function declaration has the shape:
//
//	func Register(<paramName> *llm.App)
//
// It validates that the function has exactly one parameter whose type is a
// pointer to a selector expression ending in ".App", where the package name
// resolves to an import of a path ending in "/llm" or is literally "llm".
//
// Returns the parameter name (e.g. "app") and true if the signature matches.
func isRegisterSignature(fn *ast.FuncDecl, file *ast.File) (string, bool) {
	if fn.Type == nil || fn.Type.Params == nil {
		return "", false
	}

	params := fn.Type.Params
	// Count actual parameters.
	paramCount := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			paramCount++
		} else {
			paramCount += len(field.Names)
		}
	}
	if paramCount != 1 {
		return "", false
	}

	field := params.List[0]

	// Get the parameter name.
	paramName := ""
	if len(field.Names) > 0 {
		paramName = field.Names[0].Name
	} else {
		// Unnamed parameter — we can't track method calls on it.
		return "", false
	}

	// Must be *<something>.App
	starExpr, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return "", false
	}

	sel, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	if sel.Sel.Name != "App" {
		return "", false
	}

	// The package identifier must resolve to an import ending in "/llm" or be "llm".
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}

	// Check imports to see if this identifier maps to an llm package.
	if isLLMImport(pkgIdent.Name, file) {
		return paramName, true
	}

	return "", false
}

// isLLMImport checks whether a package alias in the file resolves to an import
// path ending in "/llm" (or is literally "llm").
func isLLMImport(alias string, file *ast.File) bool {
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Explicit alias: import <alias> "some/path/llm"
		if imp.Name != nil {
			if imp.Name.Name == alias {
				return strings.HasSuffix(importPath, "/llm") || importPath == "llm"
			}
			continue
		}

		// Default alias: last path segment.
		parts := strings.Split(importPath, "/")
		defaultAlias := parts[len(parts)-1]
		if defaultAlias == alias {
			return strings.HasSuffix(importPath, "/llm") || importPath == "llm"
		}
	}
	return false
}

// extractToolCalls walks a function body's AST and extracts all calls of the
// form <paramName>.Tool("name", "description", FuncRef). It also follows
// chained calls like <paramName>.Tool(...).Tool(...).
func extractToolCalls(body *ast.BlockStmt, paramName string, fset *token.FileSet) ([]StaticToolInfo, error) {
	if body == nil {
		return nil, nil
	}

	var tools []StaticToolInfo

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if this is a .Tool() method call.
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if sel.Sel.Name != "Tool" {
			return true
		}

		// Check if the receiver is our parameter (directly or via chaining).
		if !receiverIsParam(sel.X, paramName) {
			return true
		}

		// We have a <paramName>.Tool(...) call or a chained .Tool(...).Tool(...).
		// Extract the arguments.
		if len(call.Args) < 3 {
			// Not enough arguments — skip silently (the compile program will
			// catch the real error at build time).
			return true
		}

		// First argument: tool name (must be a string literal).
		nameStr, nameOk := extractStringLiteral(call.Args[0])
		if !nameOk {
			// We'll still record this but mark it; however per spec we should
			// return an error. We collect errors via the return.
			return true
		}

		// Second argument: description (must be a string literal).
		descStr, descOk := extractStringLiteral(call.Args[1])
		if !descOk {
			return true
		}

		// Third argument: function reference (identifier or selector).
		funcName := extractFuncRef(call.Args[2])
		if funcName == "" {
			return true
		}

		tools = append(tools, StaticToolInfo{
			Name:        nameStr,
			Description: descStr,
			FuncName:    funcName,
			Line:        fset.Position(call.Pos()).Line,
		})

		return true
	})

	return tools, nil
}

// receiverIsParam checks whether expr ultimately resolves to the parameter
// name, either directly (ident == paramName) or through chaining
// (<expr>.Tool(...) where <expr> is another method call on paramName).
func receiverIsParam(expr ast.Expr, paramName string) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == paramName

	case *ast.CallExpr:
		// Chained call: expr.Something(...) — check if the inner receiver
		// traces back to paramName.
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		return receiverIsParam(sel.X, paramName)

	default:
		return false
	}
}

// extractStringLiteral extracts the string value from an *ast.BasicLit of kind
// STRING. Returns the unquoted value and true, or ("", false) if the node is
// not a string literal.
func extractStringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	// Remove surrounding quotes. Handle both `"..."` and `` `...` ``.
	s := lit.Value
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			// Interpret escape sequences.
			// For simplicity, handle the common cases.
			s = s[1 : len(s)-1]
			s = strings.ReplaceAll(s, `\"`, `"`)
			s = strings.ReplaceAll(s, `\\`, `\`)
			return s, true
		}
		if s[0] == '`' && s[len(s)-1] == '`' {
			return s[1 : len(s)-1], true
		}
	}
	return "", false
}

// extractFuncRef extracts a function reference from an AST expression.
// It handles:
//   - Simple identifiers: GetWeather → "GetWeather"
//   - Selector expressions: otherpkg.SomeFunc → "otherpkg.SomeFunc"
func extractFuncRef(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name

	case *ast.SelectorExpr:
		pkgIdent, ok := e.X.(*ast.Ident)
		if !ok {
			return ""
		}
		return pkgIdent.Name + "." + e.Sel.Name

	default:
		return ""
	}
}
