package channelcompile

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
)

// ChannelHandlerFunc represents a handler function declaration found via static analysis.
type ChannelHandlerFunc struct {
	// FuncName is the name of the handler function, e.g. "HandleUserMessage".
	FuncName string
	// TypeName is the request type name derived from the function name by
	// stripping the "Handle" prefix, e.g. "UserMessage".
	TypeName string
	// PackagePath is the import path of the package containing the handler.
	PackagePath string
	// Line is the source line number for error reporting.
	Line int
}

// MergeChannelStaticAnalysis parses register.go files for each channel package
// and fills in HandlerName on the corresponding SerializedMessageInfo entries.
//
// Handler naming convention:
//
// Each client_to_server message type expects a corresponding exported handler
// function in the channel package named Handle<TypeName>. For example, a
// channel with a dispatch type named "ChatRequest" should have a handler:
//
//	func HandleChatRequest(ctx context.Context, req *ChatRequest) error
//
// This convention is used consistently throughout ShipQ:
//   - channel/app.go sets the default HandlerName to "Handle" + TypeName during
//     channel registration (e.g., ChatRequest → HandleChatRequest).
//   - This function (static analysis) scans the channel package's Go source
//     files and overrides HandlerName with the actual function name if found.
//   - generate_worker_main.go emits handler references as <pkg>.<HandlerName>
//     (e.g., chatbot.HandleChatRequest) in the generated cmd/worker/main.go.
//
// Because the generated worker code lives in package main and must reference
// the handler cross-package, the handler function MUST be exported (uppercase).
//
// The channels slice is modified in place.
func MergeChannelStaticAnalysis(goModRoot, modulePath string, channelPkgs []string, channels []codegen.SerializedChannelInfo) error {
	for idx := range channels {
		ch := &channels[idx]

		// Find the register.go for this channel's package
		registerPath := importPathToChannelRegisterFilePath(goModRoot, modulePath, ch.PackagePath)
		if _, err := os.Stat(registerPath); os.IsNotExist(err) {
			// No register.go — skip static analysis for this channel
			continue
		}

		// Find all Handle* functions in the package directory
		handlers, err := findChannelHandlerFuncs(goModRoot, modulePath, ch.PackagePath)
		if err != nil {
			return fmt.Errorf("static analysis for channel %q: %w", ch.Name, err)
		}

		// Build a lookup: TypeName -> FuncName
		handlerMap := make(map[string]string, len(handlers))
		for _, h := range handlers {
			handlerMap[h.TypeName] = h.FuncName
		}

		// Match handler functions to FromClient messages
		for i := range ch.Messages {
			msg := &ch.Messages[i]
			if msg.Direction != "client_to_server" {
				continue
			}
			if funcName, ok := handlerMap[msg.TypeName]; ok {
				msg.HandlerName = funcName
			}
		}

		// Detect convention-based Setup function:
		//   func Setup(ctx context.Context) context.Context
		hasSetup, err := channelPackageHasSetup(goModRoot, modulePath, ch.PackagePath)
		if err != nil {
			return fmt.Errorf("static analysis (Setup) for channel %q: %w", ch.Name, err)
		}
		ch.HasSetup = hasSetup
	}

	return nil
}

// findChannelHandlerFuncs scans all Go files in the channel package directory
// for functions matching the pattern:
//
//	func Handle<TypeName>(ctx context.Context, req *<Type>) error
//
// It returns one ChannelHandlerFunc per matching function.
func findChannelHandlerFuncs(goModRoot, modulePath, importPath string) ([]ChannelHandlerFunc, error) {
	// Convert import path to filesystem directory
	relImport := strings.TrimPrefix(importPath, modulePath+"/")
	dirPath := filepath.Join(goModRoot, relImport)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dirPath, err)
	}

	var handlers []ChannelHandlerFunc

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip non-Go files, test files, and generated files
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
		found, err := parseFileForHandlerFuncs(filePath)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filePath, err)
		}

		for i := range found {
			found[i].PackagePath = importPath
		}
		handlers = append(handlers, found...)
	}

	return handlers, nil
}

// parseFileForHandlerFuncs parses a single Go file and extracts handler function
// declarations matching the Handle<TypeName> convention.
//
// A valid handler function must:
//   - Be named Handle<Something> (exported, with "Handle" prefix)
//   - Accept exactly two parameters: (context.Context, *SomeType)
//   - Return exactly one result: error
func parseFileForHandlerFuncs(filePath string) ([]ChannelHandlerFunc, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	var handlers []ChannelHandlerFunc

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Must be a top-level function (not a method)
		if fn.Recv != nil {
			continue
		}

		// Must start with "Handle"
		if !strings.HasPrefix(fn.Name.Name, "Handle") {
			continue
		}

		// Extract the type name by stripping "Handle" prefix
		typeName := strings.TrimPrefix(fn.Name.Name, "Handle")
		if typeName == "" {
			continue
		}

		// Validate signature: func(context.Context, *Type) error
		if !isValidChannelHandlerSignature(fn) {
			continue
		}

		handlers = append(handlers, ChannelHandlerFunc{
			FuncName: fn.Name.Name,
			TypeName: typeName,
			Line:     fset.Position(fn.Pos()).Line,
		})
	}

	return handlers, nil
}

// isValidChannelHandlerSignature checks that a function declaration has the
// signature: func(context.Context, *SomeType) error
func isValidChannelHandlerSignature(fn *ast.FuncDecl) bool {
	if fn.Type == nil {
		return false
	}

	// Must have exactly 2 parameters
	params := fn.Type.Params
	if params == nil || len(params.List) == 0 {
		return false
	}

	// Count actual parameters (a single field can declare multiple names)
	paramCount := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			paramCount++
		} else {
			paramCount += len(field.Names)
		}
	}
	if paramCount != 2 {
		return false
	}

	// First param should be context.Context (we check for selector "context.Context"
	// or just identifier "Context" if the import is dot-imported, though that's rare)
	firstParam := params.List[0]
	if !isContextType(firstParam.Type) {
		return false
	}

	// Second param should be a pointer type
	var secondParamType ast.Expr
	if len(params.List) == 1 {
		// Both params declared in one field (unusual but valid):
		// func Handle(ctx, req context.Context, *Type) — not really, skip this
		return false
	}
	secondParamType = params.List[1].Type
	if _, ok := secondParamType.(*ast.StarExpr); !ok {
		return false
	}

	// Must return exactly 1 result: error
	results := fn.Type.Results
	if results == nil || len(results.List) != 1 {
		return false
	}
	resultIdent, ok := results.List[0].Type.(*ast.Ident)
	if !ok || resultIdent.Name != "error" {
		return false
	}

	return true
}

// isContextType checks if an AST expression represents context.Context.
func isContextType(expr ast.Expr) bool {
	// Check for context.Context (selector expression)
	sel, ok := expr.(*ast.SelectorExpr)
	if ok {
		ident, ok := sel.X.(*ast.Ident)
		return ok && ident.Name == "context" && sel.Sel.Name == "Context"
	}
	return false
}

// channelPackageHasSetup scans all Go files in a channel package directory for
// an exported function with the exact signature:
//
//	func Setup(ctx context.Context) context.Context
//
// This convention allows channel packages to inject dependencies into the
// context before each handler invocation, without requiring mutable package-
// level state.
func channelPackageHasSetup(goModRoot, modulePath, importPath string) (bool, error) {
	relImport := strings.TrimPrefix(importPath, modulePath+"/")
	dirPath := filepath.Join(goModRoot, relImport)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, fmt.Errorf("read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "zz_generated_") {
			continue
		}

		filePath := filepath.Join(dirPath, name)
		found, err := parseFileForSetupFunc(filePath)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	return false, nil
}

// parseFileForSetupFunc parses a single Go file and checks whether it contains
// a top-level function declaration matching:
//
//	func Setup(ctx context.Context) context.Context
func parseFileForSetupFunc(filePath string) (bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if fn.Name.Name != "Setup" {
			continue
		}
		if isSetupSignature(fn) {
			return true, nil
		}
	}

	return false, nil
}

// isSetupSignature checks that a function declaration has the signature:
//
//	func Setup(context.Context) context.Context
//
// i.e., exactly one context.Context parameter and exactly one context.Context result.
func isSetupSignature(fn *ast.FuncDecl) bool {
	if fn.Type == nil {
		return false
	}

	// Exactly 1 parameter
	params := fn.Type.Params
	if params == nil {
		return false
	}
	paramCount := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			paramCount++
		} else {
			paramCount += len(field.Names)
		}
	}
	if paramCount != 1 {
		return false
	}
	if !isContextType(params.List[0].Type) {
		return false
	}

	// Exactly 1 result
	results := fn.Type.Results
	if results == nil || len(results.List) != 1 {
		return false
	}
	if !isContextType(results.List[0].Type) {
		return false
	}

	return true
}
