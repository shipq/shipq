package httptsgen

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// WriteHTTPTypeScriptClient generates shipq-api.ts and writes it to disk.
// It writes to <shipqRoot>/<tsOutputDir>/shipq-api.ts.
// If tsOutputDir is empty, it defaults to "." (project root).
// Uses codegen.WriteFileIfChanged() for idempotency.
func WriteHTTPTypeScriptClient(handlers []codegen.SerializedHandlerInfo, shipqRoot, tsOutputDir string) error {
	code, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		return fmt.Errorf("generate typescript http client: %w", err)
	}

	if tsOutputDir == "" {
		tsOutputDir = "."
	}

	outputDir := filepath.Join(shipqRoot, tsOutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create typescript output directory %s: %w", outputDir, err)
	}

	outputPath := filepath.Join(outputDir, "shipq-api.ts")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write shipq-api.ts: %w", err)
	}

	return nil
}

// WriteReactHooks generates react/shipq-api.ts and writes it to disk.
// It writes to <shipqRoot>/<tsOutputDir>/react/shipq-api.ts.
// If tsOutputDir is empty, it defaults to "." (project root).
func WriteReactHooks(handlers []codegen.SerializedHandlerInfo, shipqRoot, tsOutputDir string) error {
	code, err := GenerateReactHooks(handlers)
	if err != nil {
		return fmt.Errorf("generate react hooks: %w", err)
	}

	if tsOutputDir == "" {
		tsOutputDir = "."
	}

	outputDir := filepath.Join(shipqRoot, tsOutputDir, "react")
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create react output directory %s: %w", outputDir, err)
	}

	outputPath := filepath.Join(outputDir, "shipq-api.ts")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write react/shipq-api.ts: %w", err)
	}

	return nil
}

// WriteSvelteHooks generates svelte/shipq-api.ts and writes it to disk.
// It writes to <shipqRoot>/<tsOutputDir>/svelte/shipq-api.ts.
// If tsOutputDir is empty, it defaults to "." (project root).
func WriteSvelteHooks(handlers []codegen.SerializedHandlerInfo, shipqRoot, tsOutputDir string) error {
	code, err := GenerateSvelteHooks(handlers)
	if err != nil {
		return fmt.Errorf("generate svelte hooks: %w", err)
	}

	if tsOutputDir == "" {
		tsOutputDir = "."
	}

	outputDir := filepath.Join(shipqRoot, tsOutputDir, "svelte")
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create svelte output directory %s: %w", outputDir, err)
	}

	outputPath := filepath.Join(outputDir, "shipq-api.ts")
	if _, err := codegen.WriteFileIfChanged(outputPath, code); err != nil {
		return fmt.Errorf("write svelte/shipq-api.ts: %w", err)
	}

	return nil
}
