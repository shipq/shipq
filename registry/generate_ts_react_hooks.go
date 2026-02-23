package registry

import (
	"github.com/shipq/shipq/codegen/httptsgen"
)

// generateTypeScriptReactHooks generates the React integration layer
// (react/shipq-api.ts) and writes it to <TSHTTPOutput>/react/shipq-api.ts.
func generateTypeScriptReactHooks(cfg CompileConfig) error {
	return httptsgen.WriteReactHooks(cfg.Handlers, cfg.ShipqRoot, cfg.TSHTTPOutput)
}
