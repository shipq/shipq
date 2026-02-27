package channelgen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
)

// GenerateAllTypedChannels generates zz_generated_channel.go for each channel package.
// It calls GenerateTypedChannel for each channel and writes the output to the
// appropriate directory under channels/<pkg>/.
func GenerateAllTypedChannels(channels []codegen.SerializedChannelInfo, goModRoot, shipqRoot, modulePath string) error {
	for _, ch := range channels {
		content, err := GenerateTypedChannel(ch, modulePath)
		if err != nil {
			return fmt.Errorf("generate typed channel %q: %w", ch.Name, err)
		}

		// Convert import path back to filesystem path relative to shipqRoot.
		// modulePath is the full import prefix (including any monorepo subpath),
		// so after stripping it the remainder is relative to shipqRoot, not goModRoot.
		relImport := strings.TrimPrefix(ch.PackagePath, modulePath+"/")
		outputDir := filepath.Join(shipqRoot, relImport)
		outputPath := filepath.Join(outputDir, "zz_generated_channel.go")

		// Ensure the directory exists
		if err := codegen.EnsureDir(outputDir); err != nil {
			return fmt.Errorf("create directory for channel %q: %w", ch.Name, err)
		}

		// Write only if changed (idempotent)
		if _, err := codegen.WriteFileIfChanged(outputPath, content); err != nil {
			return fmt.Errorf("write generated channel file for %q: %w", ch.Name, err)
		}
	}

	return nil
}
