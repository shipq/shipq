package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/httpserver/testclient"
)

// generateHTTPTestClient generates the HTTP test client code.
func generateHTTPTestClient(cfg CompileConfig) error {
	testClientCfg := testclient.HTTPTestClientGenConfig{
		ModulePath: cfg.ModulePath,
		Handlers:   cfg.Handlers,
		OutputPkg:  cfg.OutputPkg,
	}

	files, err := testclient.GenerateHTTPTestClient(testClientCfg)
	if err != nil {
		return fmt.Errorf("failed to generate HTTP test client: %w", err)
	}

	// Write each generated file
	for _, f := range files {
		outputPath := filepath.Join(cfg.ShipqRoot, f.RelPath)
		outputDir := filepath.Dir(outputPath)

		if err := codegen.EnsureDir(outputDir); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", f.RelPath, err)
		}

		written, err := codegen.WriteFileIfChanged(outputPath, f.Content)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", f.RelPath, err)
		}

		if cfg.Verbose && written {
			fmt.Printf("Generated %s\n", outputPath)
		}
	}

	return nil
}
