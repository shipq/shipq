package channelgen

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// WriteWorkerMain generates cmd/worker/main.go and writes it to disk.
// It creates the cmd/worker/ directory if it doesn't already exist.
func WriteWorkerMain(cfg WorkerGenConfig, shipqRoot string) error {
	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		return fmt.Errorf("generate worker main: %w", err)
	}

	// Ensure cmd/worker directory exists
	workerDir := filepath.Join(shipqRoot, "cmd", "worker")
	if err := codegen.EnsureDir(workerDir); err != nil {
		return fmt.Errorf("create cmd/worker directory: %w", err)
	}

	// Write main.go (only if changed, for idempotency)
	outputPath := filepath.Join(workerDir, "main.go")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write cmd/worker/main.go: %w", err)
	}

	return nil
}
