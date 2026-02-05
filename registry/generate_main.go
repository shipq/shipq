package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// generateHTTPMain generates the main.go entrypoint file for the HTTP server.
func generateHTTPMain(cfg CompileConfig) error {
	mainCfg := codegen.HTTPMainGenConfig{
		ModulePath: cfg.ModulePath,
		OutputPkg:  cfg.OutputPkg,
		DBDialect:  cfg.DBDialect,
		Port:       cfg.Port,
	}

	mainCode, err := codegen.GenerateHTTPMain(mainCfg)
	if err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Ensure cmd/server directory exists (in shipq root)
	cmdDir := filepath.Join(cfg.ShipqRoot, "cmd", "server")
	if err := codegen.EnsureDir(cmdDir); err != nil {
		return fmt.Errorf("failed to create cmd/server directory: %w", err)
	}

	// Write main.go
	mainOutputPath := filepath.Join(cmdDir, "main.go")
	mainWritten, err := codegen.WriteFileIfChanged(mainOutputPath, mainCode)
	if err != nil {
		return fmt.Errorf("failed to write main.go: %w", err)
	}

	if cfg.Verbose && mainWritten {
		fmt.Printf("Generated %s\n", mainOutputPath)
	}

	return nil
}
