package channelcompile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/discovery"
)

// ChannelCompileProgramDir is the directory where the channel compile program is generated.
const ChannelCompileProgramDir = ".shipq/channel_compile"

// WriteChannelCompileProgram writes the channel compile program to .shipq/channel_compile/main.go.
// It creates the directory structure if needed.
func WriteChannelCompileProgram(projectRoot string, cfg ChannelCompileProgramConfig) error {
	// Generate the program
	programCode, err := GenerateChannelCompileProgram(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate channel compile program: %w", err)
	}

	// Create the directory
	compileDir := filepath.Join(projectRoot, ChannelCompileProgramDir)
	if err := codegen.EnsureDir(compileDir); err != nil {
		return fmt.Errorf("failed to create channel compile directory: %w", err)
	}

	// Write the program
	programPath := filepath.Join(compileDir, "main.go")
	if _, err := codegen.WriteFileIfChanged(programPath, programCode); err != nil {
		return fmt.Errorf("failed to write channel compile program: %w", err)
	}

	return nil
}

// RunChannelCompileProgram builds and executes the channel compile program.
// Returns the parsed channel definitions.
func RunChannelCompileProgram(projectRoot string) ([]codegen.SerializedChannelInfo, error) {
	programDir := filepath.Join(projectRoot, ChannelCompileProgramDir)
	binaryPath := filepath.Join(programDir, "channel_compile")

	// Build the program
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = programDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build channel compile program: %w\nstderr: %s", err, buildStderr.String())
	}

	// Run the program
	runCmd := exec.Command(binaryPath)
	runCmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	if err := runCmd.Run(); err != nil {
		return nil, fmt.Errorf("channel compile program failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse the output
	var channels []codegen.SerializedChannelInfo
	if err := json.Unmarshal(stdout.Bytes(), &channels); err != nil {
		return nil, fmt.Errorf("failed to parse channel compile output: %w\noutput: %s", err, stdout.String())
	}

	return channels, nil
}

// BuildAndRunChannelCompileProgram discovers channel packages, writes the channel
// compile program, builds it, runs it, and merges the results with static analysis
// to populate handler names.
//
// The compile program uses runtime reflection to capture channel metadata (name,
// visibility, message types), but Go reflection cannot get function names.
// Static analysis parses register.go files via AST to extract handler function names.
// This function merges both sources to produce complete channel info.
func BuildAndRunChannelCompileProgram(goModRoot, shipqRoot, modulePath string) ([]codegen.SerializedChannelInfo, error) {
	// Discover channel packages
	channelPkgs, err := discovery.DiscoverChannelPackages(goModRoot, shipqRoot, modulePath)
	if err != nil {
		return nil, fmt.Errorf("channel discovery failed: %w", err)
	}

	if len(channelPkgs) == 0 {
		return nil, nil
	}

	cfg := ChannelCompileProgramConfig{
		ModulePath:  modulePath,
		ChannelPkgs: channelPkgs,
	}

	// Write the program
	if err := WriteChannelCompileProgram(goModRoot, cfg); err != nil {
		return nil, err
	}

	// Build and run to get runtime data
	channels, err := RunChannelCompileProgram(goModRoot)
	if err != nil {
		return nil, err
	}

	// Parse register.go files to get handler function names via static analysis
	if err := MergeChannelStaticAnalysis(goModRoot, modulePath, channelPkgs, channels); err != nil {
		return nil, fmt.Errorf("static analysis failed: %w", err)
	}

	return channels, nil
}

// importPathToChannelRegisterFilePath converts an import path to a register.go file path.
func importPathToChannelRegisterFilePath(projectRoot, modulePath, importPath string) string {
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

// CleanChannelCompileArtifacts removes the compiled binary but keeps the source.
// The source is kept for debugging purposes.
func CleanChannelCompileArtifacts(projectRoot string) error {
	binaryPath := filepath.Join(projectRoot, ChannelCompileProgramDir, "channel_compile")

	// Remove binary if it exists
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove channel compile binary: %w", err)
	}

	return nil
}
