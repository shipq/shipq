package registry

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/shipq/shipq/codegen"
)

// printDebugRegistry pretty-prints the handler registry as JSON for debugging.
func printDebugRegistry(handlers []codegen.SerializedHandlerInfo) error {
	data, err := json.MarshalIndent(handlers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}
