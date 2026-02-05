package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// generateHTTPTestHarness generates the HTTP test harness code.
func generateHTTPTestHarness(cfg CompileConfig) error {
	testHarnessCfg := codegen.HTTPTestHarnessGenConfig{
		ModulePath: cfg.ModulePath,
		OutputPkg:  cfg.OutputPkg,
		DBDialect:  cfg.DBDialect,
	}

	testHarnessCode, err := codegen.GenerateHTTPTestHarness(testHarnessCfg)
	if err != nil {
		return fmt.Errorf("failed to generate HTTP test harness: %w", err)
	}

	// Ensure output directory exists (in shipq root)
	outputDir := filepath.Join(cfg.ShipqRoot, cfg.OutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write test harness to user's project
	testHarnessOutputPath := filepath.Join(outputDir, "zz_generated_testharness.go")
	written, err := codegen.WriteFileIfChanged(testHarnessOutputPath, testHarnessCode)
	if err != nil {
		return fmt.Errorf("failed to write HTTP test harness: %w", err)
	}

	if cfg.Verbose && written {
		fmt.Printf("Generated %s\n", testHarnessOutputPath)
	}

	return nil
}
