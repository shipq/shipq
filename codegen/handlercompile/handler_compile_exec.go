package handlercompile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// HandlerCompileProgramDir is the directory where the handler compile program is generated.
const HandlerCompileProgramDir = ".shipq/handler_compile"

// SerializedHandlerInfo and related types are defined in the codegen package.
// Import codegen to use them: codegen.SerializedHandlerInfo

// WriteHandlerCompileProgram writes the handler compile program to .shipq/handler_compile/main.go.
// It creates the directory structure if needed.
func WriteHandlerCompileProgram(projectRoot string, cfg HandlerCompileProgramConfig) error {
	// Generate the program
	programCode, err := GenerateHandlerCompileProgram(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate handler compile program: %w", err)
	}

	// Create the directory
	compileDir := filepath.Join(projectRoot, HandlerCompileProgramDir)
	if err := codegen.EnsureDir(compileDir); err != nil {
		return fmt.Errorf("failed to create handler compile directory: %w", err)
	}

	// Write the program
	programPath := filepath.Join(compileDir, "main.go")
	if _, err := codegen.WriteFileIfChanged(programPath, programCode); err != nil {
		return fmt.Errorf("failed to write handler compile program: %w", err)
	}

	return nil
}

// RunHandlerCompileProgram builds and executes the handler compile program.
// Returns the parsed handler definitions.
func RunHandlerCompileProgram(projectRoot string) ([]codegen.SerializedHandlerInfo, error) {
	programDir := filepath.Join(projectRoot, HandlerCompileProgramDir)
	binaryPath := filepath.Join(programDir, "handler_compile")

	// Build the program
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = programDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build handler compile program: %w\nstderr: %s", err, buildStderr.String())
	}

	// Run the program
	runCmd := exec.Command(binaryPath)
	runCmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	if err := runCmd.Run(); err != nil {
		return nil, fmt.Errorf("handler compile program failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse the output
	var handlers []codegen.SerializedHandlerInfo
	if err := json.Unmarshal(stdout.Bytes(), &handlers); err != nil {
		return nil, fmt.Errorf("failed to parse handler compile output: %w\noutput: %s", err, stdout.String())
	}

	return handlers, nil
}

// BuildAndRunHandlerCompileProgram writes the handler compile program, builds it,
// runs it, and merges the results with static analysis to populate function names.
//
// The compile program uses runtime reflection to capture handler metadata (method,
// path, request/response types), but Go reflection cannot get function names.
// Static analysis parses register.go files via AST to extract function names.
// This function merges both sources to produce complete handler info.
func BuildAndRunHandlerCompileProgram(projectRoot string, cfg HandlerCompileProgramConfig) ([]codegen.SerializedHandlerInfo, error) {
	// Write the program
	if err := WriteHandlerCompileProgram(projectRoot, cfg); err != nil {
		return nil, err
	}

	// Build and run to get runtime data (types, but no function names)
	runtimeHandlers, err := RunHandlerCompileProgram(projectRoot)
	if err != nil {
		return nil, err
	}

	// Parse register.go files to get function names via static analysis.
	// Uses raw module path (GoModModule) for import-to-filesystem conversion.
	// Falls back to ModulePath if GoModModule is not set (non-monorepo case).
	rawModule := cfg.GoModModule
	if rawModule == "" {
		rawModule = cfg.ModulePath
	}
	staticCalls, err := parseAllRegisterFiles(projectRoot, rawModule, cfg.APIPkgs)
	if err != nil {
		return nil, fmt.Errorf("static analysis failed: %w", err)
	}

	// Merge static analysis (function names) with runtime data (types)
	merged, err := mergeStaticAndRuntimeSerialized(staticCalls, runtimeHandlers)
	if err != nil {
		return nil, fmt.Errorf("failed to merge static and runtime data: %w", err)
	}

	return merged, nil
}

// importPathToRegisterFilePath converts an import path to a register.go file path.
func importPathToRegisterFilePath(projectRoot, modulePath, importPath string) string {
	var relPath string
	if importPath == modulePath {
		relPath = ""
	} else if len(importPath) > len(modulePath)+1 {
		relPath = importPath[len(modulePath)+1:]
	} else {
		relPath = ""
	}

	if relPath == "" {
		return filepath.Join(projectRoot, "register.go")
	}
	return filepath.Join(projectRoot, relPath, "register.go")
}

// parseAllRegisterFiles parses register.go files for all API packages.
// It sets the PackagePath field on each RegisterCall to the corresponding import path.
func parseAllRegisterFiles(projectRoot, modulePath string, apiPkgs []string) ([]RegisterCall, error) {
	var allCalls []RegisterCall

	for _, importPath := range apiPkgs {
		registerPath := importPathToRegisterFilePath(projectRoot, modulePath, importPath)

		// Check if register.go exists
		if _, err := os.Stat(registerPath); os.IsNotExist(err) {
			continue
		}

		// Parse the register.go file
		calls, err := ParseRegisterFile(registerPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", registerPath, err)
		}

		// Set PackagePath for each call to the import path
		for i := range calls {
			calls[i].PackagePath = importPath
		}

		allCalls = append(allCalls, calls...)
	}

	return allCalls, nil
}

// mergeStaticAndRuntimeSerialized combines static analysis with runtime data.
func mergeStaticAndRuntimeSerialized(static []RegisterCall, runtime []codegen.SerializedHandlerInfo) ([]codegen.SerializedHandlerInfo, error) {
	if len(static) != len(runtime) {
		return nil, fmt.Errorf(
			"mismatch between static analysis (%d handlers) and runtime (%d handlers)",
			len(static), len(runtime),
		)
	}

	result := make([]codegen.SerializedHandlerInfo, len(static))
	for i := range static {
		// Verify the method and path match
		staticMethod := HTTPMethodFromString(static[i].Method)
		if runtime[i].Method != string(staticMethod) {
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

		// Merge: take function name and package path from static, everything else from runtime
		result[i] = runtime[i]
		result[i].FuncName = static[i].FuncName
		result[i].PackagePath = static[i].PackagePath
	}

	return result, nil
}

// CleanHandlerCompileArtifacts removes the compiled binary but keeps the source.
// The source is kept for debugging purposes.
func CleanHandlerCompileArtifacts(projectRoot string) error {
	binaryPath := filepath.Join(projectRoot, HandlerCompileProgramDir, "handler_compile")

	// Remove binary if it exists
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove handler compile binary: %w", err)
	}

	return nil
}
