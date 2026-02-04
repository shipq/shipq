package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/handler"
)

// RegisterCall represents a parsed handler registration call.
type RegisterCall struct {
	Method   string // "Get", "Post", "Put", "Patch", "Delete"
	Path     string // "/posts/:id"
	FuncName string // "GetPost"
	Line     int    // Source line number for error reporting
}

// ParseRegisterFile parses a register.go file and extracts handler registrations.
// Returns an error with context if the file doesn't follow the expected pattern.
func ParseRegisterFile(filePath string) ([]RegisterCall, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	var calls []RegisterCall
	var parseErrors []string

	// Find the Register function
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Register" {
			continue
		}

		// Walk the function body looking for method calls
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if it's a method call on 'app'
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "app" {
				return true
			}

			method := sel.Sel.Name
			if !isHTTPMethod(method) {
				return true
			}

			// Must have exactly 2 arguments: path and handler
			if len(call.Args) != 2 {
				pos := fset.Position(call.Pos())
				parseErrors = append(parseErrors, fmt.Sprintf(
					"%s:%d: app.%s must have exactly 2 arguments (path, handler)",
					filepath.Base(filePath), pos.Line, method,
				))
				return true
			}

			// First argument must be a string literal (path)
			pathLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || pathLit.Kind != token.STRING {
				pos := fset.Position(call.Args[0].Pos())
				parseErrors = append(parseErrors, fmt.Sprintf(
					"%s:%d: first argument to app.%s must be a string literal path",
					filepath.Base(filePath), pos.Line, method,
				))
				return true
			}

			// Second argument must be a simple identifier (function name)
			funcIdent, ok := call.Args[1].(*ast.Ident)
			if !ok {
				pos := fset.Position(call.Args[1].Pos())
				parseErrors = append(parseErrors, fmt.Sprintf(
					"%s:%d: second argument to app.%s must be a function name (got %T)\n"+
						"    Handlers must be registered as direct function references, e.g.:\n"+
						"        app.%s(\"/path\", MyHandler)\n"+
						"    Anonymous functions, method expressions, and computed values are not supported.",
					filepath.Base(filePath), pos.Line, method,
					call.Args[1], method,
				))
				return true
			}

			// Remove quotes from path string
			path := strings.Trim(pathLit.Value, `"`)

			calls = append(calls, RegisterCall{
				Method:   method,
				Path:     path,
				FuncName: funcIdent.Name,
				Line:     fset.Position(call.Pos()).Line,
			})

			return true
		})
	}

	if len(parseErrors) > 0 {
		return nil, fmt.Errorf("handler registration errors:\n%s", strings.Join(parseErrors, "\n"))
	}

	return calls, nil
}

func isHTTPMethod(name string) bool {
	switch name {
	case "Get", "Post", "Put", "Patch", "Delete":
		return true
	default:
		return false
	}
}

// MergeStaticAndRuntime combines static analysis (function names) with
// runtime reflection (request/response types) to produce complete HandlerInfo.
func MergeStaticAndRuntime(static []RegisterCall, runtime []handler.HandlerInfo) ([]handler.HandlerInfo, error) {
	if len(static) != len(runtime) {
		return nil, fmt.Errorf(
			"mismatch between static analysis (%d handlers) and runtime (%d handlers)",
			len(static), len(runtime),
		)
	}

	result := make([]handler.HandlerInfo, len(static))
	for i := range static {
		// Verify the method and path match
		if string(runtime[i].Method) != strings.ToUpper(static[i].Method) {
			return nil, fmt.Errorf(
				"handler %d: method mismatch (static: %s, runtime: %s)",
				i, static[i].Method, runtime[i].Method,
			)
		}
		if runtime[i].Path != static[i].Path {
			return nil, fmt.Errorf(
				"handler %d: path mismatch (static: %s, runtime: %s)",
				i, static[i].Path, runtime[i].Path,
			)
		}

		// Merge: take function name from static, types from runtime
		result[i] = runtime[i]
		result[i].FuncName = static[i].FuncName
	}

	return result, nil
}

// HTTPMethodFromString converts a method name like "Get" to handler.HTTPMethod.
func HTTPMethodFromString(method string) handler.HTTPMethod {
	switch method {
	case "Get":
		return handler.GET
	case "Post":
		return handler.POST
	case "Put":
		return handler.PUT
	case "Patch":
		return handler.PATCH
	case "Delete":
		return handler.DELETE
	default:
		return handler.HTTPMethod(strings.ToUpper(method))
	}
}
