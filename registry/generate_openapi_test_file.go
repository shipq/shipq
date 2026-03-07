package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/openapigen"
	"github.com/shipq/shipq/dburl"
)

// generateOpenAPITest generates the OpenAPI endpoint test file and writes it
// to the user's output directory.
func generateOpenAPITest(cfg CompileConfig) error {
	// Derive test database URL from the dev URL
	testDatabaseURL := ""
	if cfg.DatabaseURL != "" {
		if u, err := dburl.TestDatabaseURL(cfg.DatabaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	testCfg := openapigen.OpenAPITestGenConfig{
		ModulePath:      cfg.ModulePath,
		OutputPkg:       cfg.OutputPkg,
		DBDialect:       cfg.DBDialect,
		TestDatabaseURL: testDatabaseURL,
		StripPrefix:     cfg.StripPrefix,
	}

	testCode, err := openapigen.GenerateOpenAPITest(testCfg)
	if err != nil {
		return fmt.Errorf("failed to generate OpenAPI test: %w", err)
	}

	// Ensure output directory exists
	outputDir := filepath.Join(cfg.ShipqRoot, cfg.OutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write test file
	testOutputPath := filepath.Join(outputDir, "zz_generated_openapi_test.go")
	written, err := codegen.WriteFileIfChanged(testOutputPath, testCode)
	if err != nil {
		return fmt.Errorf("failed to write OpenAPI test: %w", err)
	}

	if cfg.Verbose && written {
		fmt.Printf("Generated %s\n", testOutputPath)
	}

	return nil
}
