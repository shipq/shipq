package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/httpserver/server"
)

// generateHTTPServer generates the HTTP server code and writes it to the output directory.
// openAPISpec and openAPIDocsHTML are optional; when non-empty they enable dev-mode
// OpenAPI documentation routes in the generated server.
// adminHTML is optional; when non-empty it enables admin panel routes.
func generateHTTPServer(cfg CompileConfig, openAPISpec, openAPIDocsHTML, adminHTML string) error {
	// Generate HTTP server
	httpCfg := server.HTTPServerGenConfig{
		ModulePath:      cfg.ModulePath,
		Handlers:        cfg.Handlers,
		OutputPkg:       cfg.OutputPkg,
		OpenAPISpec:     openAPISpec,
		OpenAPIDocsHTML: openAPIDocsHTML,
		AdminHTML:       adminHTML,
		ScopeColumn:     cfg.ScopeColumn,
		HasChannels:     cfg.WorkersEnabled && len(cfg.Channels) > 0,
		HasOAuth:        cfg.OAuthGoogle || cfg.OAuthGitHub,
		StripPrefix:     cfg.StripPrefix,
	}

	files, err := server.GenerateHTTPServer(httpCfg)
	if err != nil {
		return fmt.Errorf("failed to generate HTTP server: %w", err)
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
