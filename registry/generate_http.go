package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// generateHTTPServer generates the HTTP server code and writes it to the output directory.
func generateHTTPServer(cfg CompileConfig) error {
	// Generate HTTP server
	httpCfg := codegen.HTTPServerGenConfig{
		ModulePath: cfg.ModulePath,
		Handlers:   cfg.Handlers,
		OutputPkg:  cfg.OutputPkg,
	}

	httpCode, err := codegen.GenerateHTTPServer(httpCfg)
	if err != nil {
		return fmt.Errorf("failed to generate HTTP server: %w", err)
	}

	// Ensure output directory exists (in shipq root)
	outputDir := filepath.Join(cfg.ShipqRoot, cfg.OutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write HTTP server to user's project
	httpOutputPath := filepath.Join(outputDir, "zz_generated_http.go")
	written, err := codegen.WriteFileIfChanged(httpOutputPath, httpCode)
	if err != nil {
		return fmt.Errorf("failed to write HTTP server: %w", err)
	}

	if cfg.Verbose && written {
		fmt.Printf("Generated %s\n", httpOutputPath)
	}

	return nil
}
