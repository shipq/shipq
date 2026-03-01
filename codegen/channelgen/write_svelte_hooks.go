package channelgen

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// WriteSvelteChannelHooks generates svelte/shipq-channels.ts and writes it to disk.
// It writes to <shipqRoot>/<tsOutputDir>/svelte/shipq-channels.ts.
// If tsOutputDir is empty, it defaults to "." (project root).
// Uses codegen.WriteFileIfChanged() for idempotency.
func WriteSvelteChannelHooks(channels []codegen.SerializedChannelInfo, shipqRoot, tsOutputDir string, llmCfg *LLMConfig) error {
	code, err := GenerateSvelteChannelHooks(channels, llmCfg)
	if err != nil {
		return fmt.Errorf("generate svelte channel hooks: %w", err)
	}

	if tsOutputDir == "" {
		tsOutputDir = "."
	}

	outputDir := filepath.Join(shipqRoot, tsOutputDir, "svelte")
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create svelte channel output directory %s: %w", outputDir, err)
	}

	outputPath := filepath.Join(outputDir, "shipq-channels.ts")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write svelte/shipq-channels.ts: %w", err)
	}

	return nil
}
