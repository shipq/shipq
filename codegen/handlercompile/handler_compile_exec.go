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

// BuildAndRunHandlerCompileProgram is a convenience function that writes the handler
// compile program, builds it, and runs it in one step.
func BuildAndRunHandlerCompileProgram(projectRoot string, cfg HandlerCompileProgramConfig) ([]codegen.SerializedHandlerInfo, error) {
	// Write the program
	if err := WriteHandlerCompileProgram(projectRoot, cfg); err != nil {
		return nil, err
	}

	// Build and run
	return RunHandlerCompileProgram(projectRoot)
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
