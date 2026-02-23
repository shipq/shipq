package registry

import (
	"github.com/shipq/shipq/codegen/httptsgen"
)

// generateTypeScriptHTTPClient generates the base TypeScript HTTP client
// (shipq-api.ts) and writes it to <TSHTTPOutput>/shipq-api.ts.
func generateTypeScriptHTTPClient(cfg CompileConfig) error {
	return httptsgen.WriteHTTPTypeScriptClient(cfg.Handlers, cfg.ShipqRoot, cfg.TSHTTPOutput)
}
