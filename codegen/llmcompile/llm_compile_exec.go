package llmcompile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
)

// LLMCompileProgramDir is the directory where the LLM compile program is generated.
const LLMCompileProgramDir = ".shipq/llm_compile"

// CompileLLMConfig holds all configuration needed to run the LLM compile pipeline.
type CompileLLMConfig struct {
	// ToolPkgs is the list of Go import paths for tool packages.
	ToolPkgs []string

	// ModulePath is the full import prefix for the user's project
	// (e.g., "myapp" or "github.com/company/monorepo/services/myservice").
	ModulePath string

	// GoModRoot is the filesystem path to the directory containing go.mod.
	GoModRoot string

	// ShipqRoot is the filesystem path to the directory containing shipq.ini.
	ShipqRoot string

	// DBDialect is the SQL dialect ("postgres", "mysql", "sqlite").
	DBDialect string

	// HasTenancy is true when the project has organisation-scoped tenancy.
	HasTenancy bool

	// HasAuth is true when the project has auth configured.
	HasAuth bool

	// ModuleInfo is the resolved module info (module path + subpath).
	ModuleInfo *codegen.ModuleInfo
}

// CompileLLM is the main entry point for `shipq llm compile`.
// It runs the full compilation pipeline:
//
//  1. Static analysis: find Register() functions and app.Tool() calls
//  2. Generate + build + run the temporary compile program
//  3. Parse the compile program's JSON output
//  4. Merge static analysis data with runtime metadata
//  5. Return the merged tool packages for code generators
func CompileLLM(cfg CompileLLMConfig) ([]SerializedToolPackage, error) {
	// ── Step 1: Static analysis ──────────────────────────────────────
	staticByPkg := make(map[string][]StaticToolInfo)

	for _, importPath := range cfg.ToolPkgs {
		tools, err := FindToolRegistrations(cfg.GoModRoot, cfg.ModulePath, importPath)
		if err != nil {
			return nil, fmt.Errorf("static analysis for %s: %w", importPath, err)
		}
		staticByPkg[importPath] = tools
	}

	// ── Step 2: Generate the temporary compile program ───────────────
	if err := WriteLLMCompileProgram(cfg.GoModRoot, cfg); err != nil {
		return nil, err
	}

	// ── Step 3: Build and run the compile program ────────────────────
	runtimeTools, err := RunLLMCompileProgram(cfg.GoModRoot)
	if err != nil {
		return nil, err
	}

	// ── Step 4: Merge static + runtime data ──────────────────────────
	merged := mergeStaticAndRuntime(cfg.ToolPkgs, staticByPkg, runtimeTools)

	// ── Step 5: Clean up binary ──────────────────────────────────────
	_ = CleanLLMCompileArtifacts(cfg.GoModRoot)

	return merged, nil
}

// runtimeToolMeta is the JSON-serializable output of the compile program.
type runtimeToolMeta struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	InputType   string          `json:"input_type"`
	OutputType  string          `json:"output_type"`
	Package     string          `json:"package"`
}

// WriteLLMCompileProgram writes the LLM compile program to .shipq/llm_compile/main.go.
func WriteLLMCompileProgram(projectRoot string, cfg CompileLLMConfig) error {
	programCode, err := GenerateLLMCompileProgram(LLMCompileProgramConfig{
		ModulePath: cfg.ModulePath,
		ToolPkgs:   cfg.ToolPkgs,
	})
	if err != nil {
		return fmt.Errorf("failed to generate llm compile program: %w", err)
	}

	compileDir := filepath.Join(projectRoot, LLMCompileProgramDir)
	if err := codegen.EnsureDir(compileDir); err != nil {
		return fmt.Errorf("failed to create llm compile directory: %w", err)
	}

	programPath := filepath.Join(compileDir, "main.go")
	if _, err := codegen.WriteFileIfChanged(programPath, programCode); err != nil {
		return fmt.Errorf("failed to write llm compile program: %w", err)
	}

	return nil
}

// RunLLMCompileProgram builds and executes the LLM compile program.
// Returns the parsed tool definitions from the program's JSON output.
func RunLLMCompileProgram(projectRoot string) ([]runtimeToolMeta, error) {
	programDir := filepath.Join(projectRoot, LLMCompileProgramDir)
	binaryPath := filepath.Join(programDir, "llm_compile")

	// Build the program.
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = programDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build llm compile program: %w\nstderr: %s\n\nThis usually means there is a type error in your tool package code.", err, buildStderr.String())
	}

	// Run the program.
	runCmd := exec.Command(binaryPath)
	runCmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	if err := runCmd.Run(); err != nil {
		return nil, fmt.Errorf("llm compile program failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse the output.
	var tools []runtimeToolMeta
	if err := json.Unmarshal(stdout.Bytes(), &tools); err != nil {
		return nil, fmt.Errorf("failed to parse llm compile output: %w\noutput: %s", err, stdout.String())
	}

	return tools, nil
}

// mergeStaticAndRuntime combines AST-extracted metadata (function names) with
// runtime-extracted metadata (JSON Schemas, type names, package paths) to
// produce the final SerializedToolPackage list.
//
// The runtime data is the ground truth for tool names, schemas, and types.
// The static data adds the Go function name (needed for code generation of
// typed dispatchers).
func mergeStaticAndRuntime(toolPkgs []string, staticByPkg map[string][]StaticToolInfo, runtime []runtimeToolMeta) []SerializedToolPackage {
	// Build a lookup from runtime: tool name → runtimeToolMeta.
	runtimeByName := make(map[string]runtimeToolMeta, len(runtime))
	for _, rt := range runtime {
		runtimeByName[rt.Name] = rt
	}

	// Build a lookup from static: tool name → StaticToolInfo (per package).
	type staticKey struct {
		pkg  string
		name string
	}
	staticByKey := make(map[staticKey]StaticToolInfo)
	for pkg, tools := range staticByPkg {
		for _, st := range tools {
			staticByKey[staticKey{pkg: pkg, name: st.Name}] = st
		}
	}

	// Group runtime tools by their package path.
	pkgTools := make(map[string][]SerializedToolInfo)
	for _, rt := range runtime {
		// Determine the import path for this tool's package.
		// The runtime data has rt.Package which is the reflect.Type.PkgPath()
		// of the input struct — this is the full Go import path.
		importPath := rt.Package

		st, hasStatic := staticByKey[staticKey{pkg: importPath, name: rt.Name}]

		// If we can't find the static info by the runtime package, try
		// matching by tool name across all packages (fallback).
		funcName := ""
		if hasStatic {
			funcName = st.FuncName
		} else {
			// Try to find in any package.
			for pkg, tools := range staticByPkg {
				_ = pkg
				for _, s := range tools {
					if s.Name == rt.Name {
						funcName = s.FuncName
						break
					}
				}
				if funcName != "" {
					break
				}
			}
		}

		// Extract the short package name from the import path.
		pkgName := ""
		if parts := strings.Split(importPath, "/"); len(parts) > 0 {
			pkgName = parts[len(parts)-1]
		}

		info := SerializedToolInfo{
			Name:        rt.Name,
			Description: rt.Description,
			FuncName:    funcName,
			PackagePath: importPath,
			PackageName: pkgName,
			InputSchema: rt.InputSchema,
			InputType:   rt.InputType,
			OutputType:  rt.OutputType,
		}

		pkgTools[importPath] = append(pkgTools[importPath], info)
	}

	// Also include packages from the config that had no tools registered
	// (they may still need a Registry() function returning an empty registry).
	for _, pkg := range toolPkgs {
		if _, exists := pkgTools[pkg]; !exists {
			pkgTools[pkg] = []SerializedToolInfo{}
		}
	}

	// Build the result, ordered by the original toolPkgs order for packages
	// listed in config, then any extra packages found via runtime.
	seen := make(map[string]bool)
	var result []SerializedToolPackage

	// First, add packages in the config order.
	for _, pkg := range toolPkgs {
		seen[pkg] = true
		pkgName := ""
		if parts := strings.Split(pkg, "/"); len(parts) > 0 {
			pkgName = parts[len(parts)-1]
		}
		result = append(result, SerializedToolPackage{
			PackagePath: pkg,
			PackageName: pkgName,
			Tools:       pkgTools[pkg],
		})
	}

	// Then add any packages found via runtime that weren't in config.
	for pkg, tools := range pkgTools {
		if seen[pkg] {
			continue
		}
		pkgName := ""
		if parts := strings.Split(pkg, "/"); len(parts) > 0 {
			pkgName = parts[len(parts)-1]
		}
		result = append(result, SerializedToolPackage{
			PackagePath: pkg,
			PackageName: pkgName,
			Tools:       tools,
		})
	}

	return result
}

// CleanLLMCompileArtifacts removes the compiled binary but keeps the source.
// The source is kept for debugging purposes.
func CleanLLMCompileArtifacts(projectRoot string) error {
	binaryPath := filepath.Join(projectRoot, LLMCompileProgramDir, "llm_compile")

	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove llm compile binary: %w", err)
	}

	return nil
}
