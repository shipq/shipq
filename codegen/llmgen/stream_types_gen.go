package llmgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen/llmcompile"
)

// DetectLLMChannels scans channel package directories and returns the names
// of channels that are LLM-enabled. A channel is considered LLM-enabled if
// any Go source file in its package imports the llm package and calls
// llm.WithClient or llm.WithNamedClient.
//
// Parameters:
//   - rootDir: the filesystem directory that modulePath maps to. When modulePath
//     is the full import prefix (including any monorepo subpath), this must be
//     shipqRoot (the directory containing shipq.ini), NOT goModRoot.
//   - modulePath: the Go import prefix used to strip package paths down to
//     filesystem-relative paths. This should be the full import prefix
//     (e.g., "github.com/company/monorepo/services/myservice").
//   - channelPkgs: import paths of all channel packages
//
// Returns a list of import paths for LLM-enabled channel packages.
func DetectLLMChannels(rootDir, modulePath string, channelPkgs []string) ([]string, error) {
	var llmChannels []string

	for _, importPath := range channelPkgs {
		enabled, err := isLLMEnabledChannel(rootDir, modulePath, importPath)
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
//
// rootDir is the filesystem directory that modulePath maps to. When modulePath
// is the full import prefix (including any monorepo subpath), this must be
// shipqRoot, not goModRoot.
func isLLMEnabledChannel(rootDir, modulePath, importPath string) (bool, error) {
	// modulePath is the full import prefix, so stripping it yields a path
	// relative to rootDir (which should be shipqRoot in a monorepo setup).
	relImport := strings.TrimPrefix(importPath, modulePath+"/")
	dirPath := filepath.Join(rootDir, relImport)

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
// When tool metadata is provided, the function generates:
//   - Per-tool input/output interfaces derived from JSON Schema
//   - A LLMToolName union type of all tool name literals
//   - Discriminated-union variants of LLMToolCallStart and LLMToolCallResult
//     where tool_name narrows the input/output types
//
// When no tool metadata is provided, the function falls back to generic
// Record<string, unknown> for input/output types.
//
// The returned string contains the TypeScript interface definitions and
// a union type combining all LLM stream message types.
func GenerateLLMStreamTypeScript(tools []llmcompile.SerializedToolInfo) string {
	var buf bytes.Buffer

	buf.WriteString("// LLM stream message types (auto-injected by shipq llm compile)\n\n")

	// If we have tool metadata, generate per-tool interfaces and discriminated unions.
	hasTools := len(tools) > 0

	if hasTools {
		// Generate per-tool input/output interfaces from JSON Schemas
		for _, tool := range tools {
			inputName := toPascalCaseToolType(tool.InputType)
			outputName := toPascalCaseToolType(tool.OutputType)

			buf.WriteString(fmt.Sprintf("export interface %s ", inputName))
			writeJSONSchemaAsTS(&buf, tool.InputSchema)
			buf.WriteString("\n\n")

			buf.WriteString(fmt.Sprintf("export interface %s ", outputName))
			writeOutputInterfaceStub(&buf, outputName)
			buf.WriteString("\n\n")
		}

		// LLMToolName union
		buf.WriteString("export type LLMToolName =\n")
		for i, tool := range tools {
			buf.WriteString(fmt.Sprintf("  | %q", tool.Name))
			if i < len(tools)-1 {
				buf.WriteString("\n")
			} else {
				buf.WriteString(";\n\n")
			}
		}
	}

	buf.WriteString("export interface LLMTextDelta {\n")
	buf.WriteString("  text: string;\n")
	buf.WriteString("}\n\n")

	if hasTools {
		// Discriminated union for LLMToolCallStart
		buf.WriteString("export type LLMToolCallStart =\n")
		for i, tool := range tools {
			inputName := toPascalCaseToolType(tool.InputType)
			buf.WriteString(fmt.Sprintf("  | { tool_call_id: string; tool_name: %q; input: %s }", tool.Name, inputName))
			if i < len(tools)-1 {
				buf.WriteString("\n")
			} else {
				buf.WriteString(";\n\n")
			}
		}

		// Discriminated union for LLMToolCallResult
		buf.WriteString("export type LLMToolCallResult =\n")
		for i, tool := range tools {
			outputName := toPascalCaseToolType(tool.OutputType)
			buf.WriteString(fmt.Sprintf("  | { tool_call_id: string; tool_name: %q; output?: %s; error?: string; duration_ms: number }", tool.Name, outputName))
			if i < len(tools)-1 {
				buf.WriteString("\n")
			} else {
				buf.WriteString(";\n\n")
			}
		}
	} else {
		// Fallback: generic tool call types
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
	}

	buf.WriteString("export interface LLMDone {\n")
	buf.WriteString("  text: string;\n")
	buf.WriteString("  input_tokens: number;\n")
	buf.WriteString("  output_tokens: number;\n")
	buf.WriteString("  tool_call_count: number;\n")
	buf.WriteString("}\n")

	return buf.String()
}

// toPascalCaseToolType converts a Go type name (e.g., "WeatherInput") to a
// TypeScript-friendly PascalCase name. If the name is already PascalCase,
// it is returned unchanged.
func toPascalCaseToolType(name string) string {
	if name == "" {
		return "Record<string, unknown>"
	}
	return name
}

// writeJSONSchemaAsTS writes a TypeScript interface body from a JSON Schema
// object. It handles simple "object" schemas with "properties" and "required".
// For complex or unsupported schemas, it falls back to Record<string, unknown>.
func writeJSONSchemaAsTS(buf *bytes.Buffer, schema json.RawMessage) {
	if len(schema) == 0 {
		buf.WriteString("{ [key: string]: unknown }")
		return
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &obj); err != nil {
		buf.WriteString("{ [key: string]: unknown }")
		return
	}

	// Extract properties
	var props map[string]json.RawMessage
	if propsRaw, ok := obj["properties"]; ok {
		if err := json.Unmarshal(propsRaw, &props); err != nil {
			buf.WriteString("{ [key: string]: unknown }")
			return
		}
	}

	if len(props) == 0 {
		buf.WriteString("{ [key: string]: unknown }")
		return
	}

	// Extract required fields
	requiredSet := make(map[string]bool)
	if reqRaw, ok := obj["required"]; ok {
		var required []string
		if err := json.Unmarshal(reqRaw, &required); err == nil {
			for _, r := range required {
				requiredSet[r] = true
			}
		}
	}

	buf.WriteString("{\n")
	for propName, propSchema := range props {
		tsType := jsonSchemaTypeToTS(propSchema)
		optional := ""
		if !requiredSet[propName] {
			optional = "?"
		}
		fmt.Fprintf(buf, "  %s%s: %s;\n", propName, optional, tsType)
	}
	buf.WriteString("}")
}

// jsonSchemaTypeToTS converts a JSON Schema type definition to a TypeScript type string.
func jsonSchemaTypeToTS(schema json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &obj); err != nil {
		return "unknown"
	}

	typeRaw, ok := obj["type"]
	if !ok {
		return "unknown"
	}

	var typeStr string
	if err := json.Unmarshal(typeRaw, &typeStr); err != nil {
		return "unknown"
	}

	switch typeStr {
	case "string":
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		itemType := "unknown"
		if items, ok := obj["items"]; ok {
			itemType = jsonSchemaTypeToTS(items)
		}
		return itemType + "[]"
	case "object":
		return "Record<string, unknown>"
	default:
		return "unknown"
	}
}

// writeOutputInterfaceStub writes a generic output interface body.
// Output types don't have JSON Schema metadata from the compile step,
// so we generate a permissive Record-based type.
func writeOutputInterfaceStub(buf *bytes.Buffer, _ string) {
	buf.WriteString("{ [key: string]: unknown }")
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

// WriteLLMToolsMarker writes a JSON marker file containing serialized tool
// metadata (names, descriptions, JSON Schemas, input/output types). This
// marker is read by `shipq workers compile` to generate typed TypeScript
// interfaces for tool call inputs and outputs.
//
// The marker file is written to .shipq/llm_tools.json.
func WriteLLMToolsMarker(shipqRoot string, tools []llmcompile.SerializedToolInfo) error {
	markerDir := filepath.Join(shipqRoot, ".shipq")
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .shipq directory: %w", err)
	}

	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tool metadata: %w", err)
	}
	data = append(data, '\n')

	markerPath := filepath.Join(markerDir, "llm_tools.json")
	if err := os.WriteFile(markerPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write llm_tools.json: %w", err)
	}

	return nil
}

// ReadLLMToolsMarker reads the .shipq/llm_tools.json marker file and returns
// the list of serialized tool metadata.
// Returns nil (not an error) if the marker file does not exist.
func ReadLLMToolsMarker(shipqRoot string) ([]llmcompile.SerializedToolInfo, error) {
	markerPath := filepath.Join(shipqRoot, ".shipq", "llm_tools.json")

	data, err := os.ReadFile(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read llm_tools.json: %w", err)
	}

	var tools []llmcompile.SerializedToolInfo
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("failed to parse llm_tools.json: %w", err)
	}

	return tools, nil
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
