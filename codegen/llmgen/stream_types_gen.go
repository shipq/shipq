package llmgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// DetectLLMChannels scans channel package directories and returns the names
// of channels that are LLM-enabled. A channel is considered LLM-enabled if
// any Go source file in its package imports the llm package and calls
// llm.WithClient or llm.WithNamedClient.
//
// Parameters:
//   - goModRoot: filesystem path to the directory containing go.mod
//   - modulePath: the Go module path (e.g., "myapp")
//   - channelPkgs: import paths of all channel packages
//
// Returns a list of import paths for LLM-enabled channel packages.
func DetectLLMChannels(goModRoot, modulePath string, channelPkgs []string) ([]string, error) {
	var llmChannels []string

	for _, importPath := range channelPkgs {
		enabled, err := isLLMEnabledChannel(goModRoot, modulePath, importPath)
		if err != nil {
			return nil, fmt.Errorf("detect llm channel %s: %w", importPath, err)
		}
		if enabled {
			llmChannels = append(llmChannels, importPath)
		}
	}

	return llmChannels, nil
}

// isLLMEnabledChannel checks whether a channel package imports and uses
// llm.WithClient or llm.WithNamedClient. It scans all non-test, non-generated
// Go files in the package directory.
func isLLMEnabledChannel(goModRoot, modulePath, importPath string) (bool, error) {
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
		found, err := fileUsesLLMClient(filePath)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	return false, nil
}

// fileUsesLLMClient parses a single Go file and checks whether it:
//  1. Imports a package whose path ends in "/llm" (or is literally "llm")
//  2. Contains a call to <alias>.WithClient or <alias>.WithNamedClient
//
// Both conditions must be met for the file to be considered LLM-enabled.
func fileUsesLLMClient(filePath string) (bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	// Step 1: Find the llm import alias.
	llmAlias := findLLMImportAlias(node)
	if llmAlias == "" {
		return false, nil
	}

	// Step 2: Walk AST looking for <alias>.WithClient or <alias>.WithNamedClient.
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name != llmAlias {
			return true
		}
		if sel.Sel.Name == "WithClient" || sel.Sel.Name == "WithNamedClient" {
			found = true
			return false
		}
		return true
	})

	return found, nil
}

// findLLMImportAlias returns the local alias for an import whose path ends
// in "/llm" (or is exactly "llm"). Returns "" if no such import exists.
func findLLMImportAlias(file *ast.File) string {
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		isLLM := strings.HasSuffix(importPath, "/llm") || importPath == "llm"
		if !isLLM {
			continue
		}

		// Explicit alias takes precedence.
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			return imp.Name.Name
		}

		// Default alias is the last path segment.
		parts := strings.Split(importPath, "/")
		return parts[len(parts)-1]
	}
	return ""
}

// GenerateLLMStreamTypeScript generates the TypeScript type definitions for
// LLM stream message types. These types are injected into the FromServer
// union type for LLM-enabled channels.
//
// The returned string contains the TypeScript interface definitions and
// a union type combining all LLM stream message types.
func GenerateLLMStreamTypeScript() string {
	var buf bytes.Buffer

	buf.WriteString("// LLM stream message types (auto-injected by shipq llm compile)\n\n")

	buf.WriteString("export interface LLMTextDelta {\n")
	buf.WriteString("  text: string;\n")
	buf.WriteString("}\n\n")

	buf.WriteString("export interface LLMToolCallStart {\n")
	buf.WriteString("  tool_call_id: string;\n")
	buf.WriteString("  tool_name: string;\n")
	buf.WriteString("  input: Record<string, unknown>;\n")
	buf.WriteString("}\n\n")

	buf.WriteString("export interface LLMToolCallResult {\n")
	buf.WriteString("  tool_call_id: string;\n")
	buf.WriteString("  tool_name: string;\n")
	buf.WriteString("  output?: Record<string, unknown>;\n")
	buf.WriteString("  error?: string;\n")
	buf.WriteString("  duration_ms: number;\n")
	buf.WriteString("}\n\n")

	buf.WriteString("export interface LLMDone {\n")
	buf.WriteString("  text: string;\n")
	buf.WriteString("  input_tokens: number;\n")
	buf.WriteString("  output_tokens: number;\n")
	buf.WriteString("  tool_call_count: number;\n")
	buf.WriteString("}\n")

	return buf.String()
}

// LLMFromServerUnionMembers returns the TypeScript union type members for
// LLM stream message types, suitable for injection into a channel's
// FromServer union type.
//
// Each member follows the pattern: { type: "TypeName"; data: TypeName }
func LLMFromServerUnionMembers() []string {
	return []string{
		`{ type: "LLMTextDelta"; data: LLMTextDelta }`,
		`{ type: "LLMToolCallStart"; data: LLMToolCallStart }`,
		`{ type: "LLMToolCallResult"; data: LLMToolCallResult }`,
		`{ type: "LLMDone"; data: LLMDone }`,
	}
}

// WriteLLMChannelsMarker writes a JSON marker file listing which channels
// are LLM-enabled. This marker is read by `shipq channel compile` (or the
// umbrella `shipq compile`) to inject LLM FromServer types when generating
// typed channel code and TypeScript.
//
// The marker file is written to .shipq/llm_channels.json.
func WriteLLMChannelsMarker(shipqRoot string, llmChannelPkgs []string) error {
	markerDir := filepath.Join(shipqRoot, ".shipq")
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .shipq directory: %w", err)
	}

	// Build a simple JSON array of import paths.
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, pkg := range llmChannelPkgs {
		fmt.Fprintf(&buf, "  %q", pkg)
		if i < len(llmChannelPkgs)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("]\n")

	markerPath := filepath.Join(markerDir, "llm_channels.json")
	if err := os.WriteFile(markerPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write llm_channels.json: %w", err)
	}

	return nil
}

// ReadLLMChannelsMarker reads the .shipq/llm_channels.json marker file
// and returns the list of LLM-enabled channel import paths.
// Returns nil (not an error) if the marker file does not exist.
func ReadLLMChannelsMarker(shipqRoot string) ([]string, error) {
	markerPath := filepath.Join(shipqRoot, ".shipq", "llm_channels.json")

	data, err := os.ReadFile(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read llm_channels.json: %w", err)
	}

	// Simple JSON array parsing — avoid importing encoding/json for this
	// trivial format by just extracting quoted strings.
	var result []string
	content := string(data)
	for {
		idx := strings.Index(content, `"`)
		if idx < 0 {
			break
		}
		content = content[idx+1:]
		endIdx := strings.Index(content, `"`)
		if endIdx < 0 {
			break
		}
		result = append(result, content[:endIdx])
		content = content[endIdx+1:]
	}

	return result, nil
}
