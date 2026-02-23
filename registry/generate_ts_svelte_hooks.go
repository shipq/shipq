package registry

import (
	"github.com/shipq/shipq/codegen/httptsgen"
)

// generateTypeScriptSvelteHooks generates the Svelte integration layer
// (svelte/shipq-api.ts) and writes it to <TSHTTPOutput>/svelte/shipq-api.ts.
func generateTypeScriptSvelteHooks(cfg CompileConfig) error {
	return httptsgen.WriteSvelteHooks(cfg.Handlers, cfg.ShipqRoot, cfg.TSHTTPOutput)
}
