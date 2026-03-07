package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/httpserver/server"
)

// generateHTTPMain generates the main.go entrypoint file for the HTTP server.
func generateHTTPMain(cfg CompileConfig) error {
	// Determine if any channel requires auth (i.e., is not public)
	channelsNeedAuth := false
	for _, ch := range cfg.Channels {
		if !ch.IsPublic {
			channelsNeedAuth = true
			break
		}
	}

	mainCfg := server.HTTPMainGenConfig{
		ModulePath:  cfg.ModulePath,
		OutputPkg:   cfg.OutputPkg,
		DBDialect:   cfg.DBDialect,
		HasChannels: cfg.WorkersEnabled && len(cfg.Channels) > 0,
		HasAuth:     cfg.HasAuth && channelsNeedAuth,
		AutoMigrate: cfg.AutoMigrate,
		StripPrefix: cfg.StripPrefix,
	}

	mainCode, err := server.GenerateHTTPMain(mainCfg)
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
