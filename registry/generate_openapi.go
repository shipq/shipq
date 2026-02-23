package registry

import (
	"path"

	"github.com/shipq/shipq/codegen/openapigen"
)

// openAPIData holds the generated OpenAPI spec and docs HTML for passing
// to the HTTP server generator.
type openAPIData struct {
	SpecJSON string
	DocsHTML string
}

// generateOpenAPI generates the OpenAPI spec JSON and docs HTML from the
// handler registry. The returned data is embedded into the generated HTTP
// server code so it can be served at /openapi and /docs in development mode.
func generateOpenAPI(cfg CompileConfig) (openAPIData, error) {
	title := path.Base(cfg.ModulePath)

	specCfg := openapigen.OpenAPIGenConfig{
		ModulePath: cfg.ModulePath,
		Handlers:   cfg.Handlers,
		Title:      title,
	}

	specJSON, err := openapigen.GenerateOpenAPISpec(specCfg)
	if err != nil {
		return openAPIData{}, err
	}

	docsHTML := openapigen.GenerateDocsHTML(title + " - API Documentation")

	return openAPIData{
		SpecJSON: string(specJSON),
		DocsHTML: docsHTML,
	}, nil
}
