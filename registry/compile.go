package registry

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/shipq/shipq/codegen"
)

// CompileConfig holds all configuration needed for registry compilation.
type CompileConfig struct {
	ProjectRoot string
	ModulePath  string
	Handlers    []codegen.SerializedHandlerInfo
}

// CompileRegistry is the central hook for all codegen that depends on the
// handler registry. This function will grow to include:
//
//   - generateHTTPServer()
//   - generateHTTPClient()
//   - generateHandlerTests()
//   - generateOpenAPISpec()
//   - generateTypeScriptClient()
//
// For now, it pretty-prints the registry to stdout.
func CompileRegistry(cfg CompileConfig) error {
	// Pretty-print the registry as JSON
	data, err := json.MarshalIndent(cfg.Handlers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	fmt.Fprintln(os.Stdout, string(data))

	return nil
}
