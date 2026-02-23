package channelgen

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// WriteReactChannelHooks generates react/shipq-channels.ts and writes it to disk.
// It writes to <shipqRoot>/<tsOutputDir>/react/shipq-channels.ts.
// If tsOutputDir is empty, it defaults to "." (project root).
// Uses codegen.WriteFileIfChanged() for idempotency.
func WriteReactChannelHooks(channels []codegen.SerializedChannelInfo, shipqRoot, tsOutputDir string) error {
	code, err := GenerateReactChannelHooks(channels)
	if err != nil {
		return fmt.Errorf("generate react channel hooks: %w", err)
	}

	if tsOutputDir == "" {
		tsOutputDir = "."
	}

	outputDir := filepath.Join(shipqRoot, tsOutputDir, "react")
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create react channel output directory %s: %w", outputDir, err)
	}

	outputPath := filepath.Join(outputDir, "shipq-channels.ts")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write react/shipq-channels.ts: %w", err)
	}

	return nil
}
