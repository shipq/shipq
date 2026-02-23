package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/channelgen"
)

// generateChannelRoutes generates the channel HTTP route files (dispatch, token, job status)
// and writes them to the output directory alongside the existing HTTP server code.
func generateChannelRoutes(cfg CompileConfig) error {
	httpCfg := channelgen.ChannelHTTPGenConfig{
		Channels:    cfg.Channels,
		ModulePath:  cfg.ModulePath,
		OutputPkg:   cfg.OutputPkg,
		ScopeColumn: cfg.ScopeColumn,
		HasAuth:     cfg.HasAuth,
	}

	files, err := channelgen.GenerateChannelHTTPRoutes(httpCfg)
	if err != nil {
		return fmt.Errorf("failed to generate channel HTTP routes: %w", err)
	}

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
