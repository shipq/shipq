package channelgen

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// WriteTypeScriptChannelClient generates shipq-channels.ts and writes it to disk.
// It filters to frontend channels only and writes to <shipqRoot>/<tsOutputDir>/shipq-channels.ts.
// If tsOutputDir is empty, it defaults to "." (project root).
// Uses codegen.WriteFileIfChanged() for idempotency.
func WriteTypeScriptChannelClient(channels []codegen.SerializedChannelInfo, shipqRoot, tsOutputDir string) error {
	code, err := GenerateTypeScriptChannelClient(channels)
	if err != nil {
		return fmt.Errorf("generate typescript channel client: %w", err)
	}

	if tsOutputDir == "" {
		tsOutputDir = "."
	}

	outputDir := filepath.Join(shipqRoot, tsOutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create typescript output directory %s: %w", outputDir, err)
	}

	outputPath := filepath.Join(outputDir, "shipq-channels.ts")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write shipq-channels.ts: %w", err)
	}

	return nil
}
