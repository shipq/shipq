package querycompile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/db/portsql/query"
)

// CompileProgramDir is the directory where the compile program is generated.
const CompileProgramDir = ".shipq/compile"

// CompileProgramFile is the filename of the generated compile program.
const CompileProgramFile = "main.go"

// WriteCompileProgram writes the compile program to .shipq/compile/main.go.
// It creates the directory structure if needed.
func WriteCompileProgram(projectRoot string, cfg CompileProgramConfig) error {
	// Generate the program
	programCode, err := GenerateCompileProgram(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate compile program: %w", err)
	}

	// Create the directory
	compileDir := filepath.Join(projectRoot, CompileProgramDir)
	if err := codegen.EnsureDir(compileDir); err != nil {
		return fmt.Errorf("failed to create compile directory: %w", err)
	}

	// Write the program
	programPath := filepath.Join(compileDir, CompileProgramFile)
	if _, err := codegen.WriteFileIfChanged(programPath, programCode); err != nil {
		return fmt.Errorf("failed to write compile program: %w", err)
	}

	return nil
}

// RunCompileProgram builds and executes the temporary program.
// Returns the parsed query definitions.
func RunCompileProgram(projectRoot string) ([]query.SerializedQuery, error) {
	programDir := filepath.Join(projectRoot, CompileProgramDir)
	binaryPath := filepath.Join(programDir, "compile")

	// Build the program
	// Using -o to specify output path, building from the directory containing main.go
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = programDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0") // Disable CGO for simpler builds

	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build compile program: %w\nstderr: %s", err, buildStderr.String())
	}

	// Run the program
	runCmd := exec.Command(binaryPath)
	runCmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	if err := runCmd.Run(); err != nil {
		return nil, fmt.Errorf("compile program failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse the output
	var queries []query.SerializedQuery
	if err := json.Unmarshal(stdout.Bytes(), &queries); err != nil {
		return nil, fmt.Errorf("failed to parse compile output: %w\noutput: %s", err, stdout.String())
	}

	return queries, nil
}

// BuildAndRunCompileProgram is a convenience function that writes the compile
// program, builds it, and runs it in one step.
func BuildAndRunCompileProgram(projectRoot string, cfg CompileProgramConfig) ([]query.SerializedQuery, error) {
	// Write the program
	if err := WriteCompileProgram(projectRoot, cfg); err != nil {
		return nil, err
	}

	// Build and run
	return RunCompileProgram(projectRoot)
}

// CleanCompileArtifacts removes the compiled binary but keeps the source.
// The source is kept for debugging purposes.
func CleanCompileArtifacts(projectRoot string) error {
	binaryPath := filepath.Join(projectRoot, CompileProgramDir, "compile")

	// Remove binary if it exists
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove compile binary: %w", err)
	}

	return nil
}
