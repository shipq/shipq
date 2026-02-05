package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// generateHTTPTestClient generates the HTTP test client code.
func generateHTTPTestClient(cfg CompileConfig) error {
	testClientCfg := codegen.HTTPTestClientGenConfig{
		ModulePath: cfg.ModulePath,
		Handlers:   cfg.Handlers,
		OutputPkg:  cfg.OutputPkg,
	}

	testClientCode, err := codegen.GenerateHTTPTestClient(testClientCfg)
	if err != nil {
		return fmt.Errorf("failed to generate HTTP test client: %w", err)
	}

	// Ensure output directory exists (in shipq root)
	outputDir := filepath.Join(cfg.ShipqRoot, cfg.OutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write test client to user's project
	testClientOutputPath := filepath.Join(outputDir, "zz_generated_testclient.go")
	written, err := codegen.WriteFileIfChanged(testClientOutputPath, testClientCode)
	if err != nil {
		return fmt.Errorf("failed to write HTTP test client: %w", err)
	}

	if cfg.Verbose && written {
		fmt.Printf("Generated %s\n", testClientOutputPath)
	}

	return nil
}
