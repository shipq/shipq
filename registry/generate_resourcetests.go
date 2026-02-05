package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// generateResourceTests generates tests for all full resources.
func generateResourceTests(cfg CompileConfig) error {
	// Detect full resources
	resources := codegen.DetectFullResources(cfg.Handlers)
	fullResources := codegen.FilterFullResources(resources)

	if len(fullResources) == 0 {
		if cfg.Verbose {
			fmt.Println("No full resources detected, skipping resource test generation")
		}
		return nil
	}

	testCfg := codegen.ResourceTestGenConfig{
		ModulePath: cfg.ModulePath,
		OutputPkg:  cfg.OutputPkg,
	}

	for _, resource := range fullResources {
		testCode, err := codegen.GenerateResourceTest(testCfg, resource)
		if err != nil {
			return fmt.Errorf("failed to generate resource test for %s: %w", resource.PackageName, err)
		}

		// Create test directory: {resource_pkg}_test
		// Extract the resource package directory from the full path
		resourceDir := extractResourceDir(cfg.ProjectRoot, cfg.ModulePath, resource.PackagePath)
		testDir := resourceDir + "_test"

		if err := codegen.EnsureDir(testDir); err != nil {
			return fmt.Errorf("failed to create test directory %s: %w", testDir, err)
		}

		// Write test file
		testOutputPath := filepath.Join(testDir, "handlers_http_test.go")
		written, err := codegen.WriteFileIfChanged(testOutputPath, testCode)
		if err != nil {
			return fmt.Errorf("failed to write resource test: %w", err)
		}

		if cfg.Verbose && written {
			fmt.Printf("Generated %s\n", testOutputPath)
		}
	}

	return nil
}

// extractResourceDir converts a package path to a directory path.
// e.g., "myapp/api/resources/accounts" -> "/path/to/project/api/resources/accounts"
func extractResourceDir(projectRoot, modulePath, packagePath string) string {
	// Remove module path prefix to get relative path
	relativePath := packagePath
	if len(modulePath) > 0 && len(packagePath) > len(modulePath) {
		relativePath = packagePath[len(modulePath)+1:] // +1 for the "/"
	}
	return filepath.Join(projectRoot, relativePath)
}
