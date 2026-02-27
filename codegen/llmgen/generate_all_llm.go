package llmgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/llmcompile"
)

// GenerateAllLLMConfig holds configuration for the full LLM code generation pass.
type GenerateAllLLMConfig struct {
	// ToolPackages is the merged tool metadata from the compile pipeline.
	ToolPackages []llmcompile.SerializedToolPackage

	// ModulePath is the full import prefix for the user's project.
	ModulePath string

	// GoModRoot is the filesystem path to the directory containing go.mod.
	GoModRoot string

	// ShipqRoot is the filesystem path to the directory containing shipq.ini.
	ShipqRoot string

	// DBDialect is the SQL dialect ("postgres", "mysql", "sqlite").
	DBDialect string

	// HasTenancy is true when the project has organisation-scoped tenancy.
	HasTenancy bool

	// HasAuth is true when the project has auth configured.
	HasAuth bool

	// ChannelPkgs is the list of all channel package import paths (used for
	// LLM stream type detection).
	ChannelPkgs []string
}

// GenerateAllLLM runs all LLM code generators in sequence:
//
//  1. Tool registries (per-package zz_generated_registry.go)
//  2. Persister adapter (shipq/lib/llmpersist/zz_generated_persister.go)
//  3. Database migration (migrations/Migrate_<ts>_llm_tables.go)
//  4. Querydefs (querydefs/llm/queries.go)
//  5. Stream types detection + marker file
//
// Each generator is independent — if one fails, the error is returned
// immediately and subsequent generators are not run.
func GenerateAllLLM(cfg GenerateAllLLMConfig) error {
	// ── Step 1: Generate tool registries ──────────────────────────────
	if err := generateToolRegistries(cfg); err != nil {
		return fmt.Errorf("generate tool registries: %w", err)
	}

	// ── Step 1b: Write tool metadata marker file ─────────────────────
	if err := generateToolsMarker(cfg); err != nil {
		return fmt.Errorf("generate tools marker: %w", err)
	}

	// ── Step 2: Generate persister adapter ────────────────────────────
	if err := generatePersisterAdapter(cfg); err != nil {
		return fmt.Errorf("generate persister adapter: %w", err)
	}

	// ── Step 3: Generate migration ───────────────────────────────────
	if err := generateMigration(cfg); err != nil {
		return fmt.Errorf("generate migration: %w", err)
	}

	// ── Step 4: Generate querydefs ───────────────────────────────────
	if err := generateQuerydefs(cfg); err != nil {
		return fmt.Errorf("generate querydefs: %w", err)
	}

	// ── Step 5: Detect LLM channels and write marker ─────────────────
	if err := generateStreamTypesMarker(cfg); err != nil {
		return fmt.Errorf("generate stream types marker: %w", err)
	}

	return nil
}

// generateToolRegistries generates zz_generated_registry.go for each tool package.
func generateToolRegistries(cfg GenerateAllLLMConfig) error {
	for _, pkg := range cfg.ToolPackages {
		content, err := GenerateToolRegistry(pkg, cfg.ModulePath)
		if err != nil {
			return fmt.Errorf("package %s: %w", pkg.PackagePath, err)
		}

		// Convert import path to filesystem path.
		// ModulePath is the full import prefix (including any monorepo subpath),
		// so after stripping it the remainder is relative to ShipqRoot, not GoModRoot.
		relImport := strings.TrimPrefix(pkg.PackagePath, cfg.ModulePath+"/")
		outputDir := filepath.Join(cfg.ShipqRoot, relImport)
		outputPath := filepath.Join(outputDir, "zz_generated_registry.go")

		if err := codegen.EnsureDir(outputDir); err != nil {
			return fmt.Errorf("create directory for %s: %w", pkg.PackagePath, err)
		}

		if _, err := codegen.WriteFileIfChanged(outputPath, content); err != nil {
			return fmt.Errorf("write registry for %s: %w", pkg.PackagePath, err)
		}
	}

	return nil
}

// generateToolsMarker collects all tool metadata from the compiled tool
// packages and writes the .shipq/llm_tools.json marker file. This marker
// is consumed by `shipq workers compile` to generate typed TypeScript
// interfaces for tool call inputs and outputs.
func generateToolsMarker(cfg GenerateAllLLMConfig) error {
	var allTools []llmcompile.SerializedToolInfo
	for _, pkg := range cfg.ToolPackages {
		allTools = append(allTools, pkg.Tools...)
	}
	return WriteLLMToolsMarker(cfg.ShipqRoot, allTools)
}

// generatePersisterAdapter generates the llmpersist adapter.
func generatePersisterAdapter(cfg GenerateAllLLMConfig) error {
	content, err := GeneratePersisterAdapter(cfg.ModulePath, cfg.HasAuth)
	if err != nil {
		return err
	}

	outputDir := filepath.Join(cfg.ShipqRoot, "shipq", "lib", "llmpersist")
	outputPath := filepath.Join(outputDir, "zz_generated_persister.go")

	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("create llmpersist directory: %w", err)
	}

	if _, err := codegen.WriteFileIfChanged(outputPath, content); err != nil {
		return fmt.Errorf("write persister adapter: %w", err)
	}

	return nil
}

// llmMigrationSuffix is the suffix used to identify the LLM migration file.
const llmMigrationSuffix = "_llm_tables.go"

// generateMigration generates the LLM tables migration if it doesn't already exist.
func generateMigration(cfg GenerateAllLLMConfig) error {
	migrationsDir := filepath.Join(cfg.ShipqRoot, "migrations")
	if err := codegen.EnsureDir(migrationsDir); err != nil {
		return fmt.Errorf("create migrations directory: %w", err)
	}

	// Check if the migration already exists (idempotent).
	if migrationExists(migrationsDir, llmMigrationSuffix) {
		return nil
	}

	// Use a timestamp that is guaranteed to come after all existing migrations.
	// Auth pre-generates timestamps into the future (baseTime + 0..6s), so
	// time.Now() alone can race with those future timestamps on fast machines.
	// The LLM migration creates two plan entries (llm_conversations at this
	// timestamp and llm_messages at timestamp+1s), so we need 2 consecutive
	// seconds of headroom.
	timestamp := nextMigrationTimestamp(migrationsDir)
	content := GenerateLLMMigration(timestamp, cfg.ModulePath, cfg.HasTenancy, cfg.HasAuth)

	filename := fmt.Sprintf("%s_llm_tables.go", timestamp)
	outputPath := filepath.Join(migrationsDir, filename)

	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return fmt.Errorf("write migration: %w", err)
	}

	return nil
}

// nextMigrationTimestamp returns a timestamp that is strictly after both
// time.Now() and the latest existing migration file timestamp. This prevents
// ordering conflicts when prior commands (e.g. shipq auth) pre-generate
// migration timestamps into the future.
//
// Comparisons use the formatted 14-digit string representation (not time.Time)
// to avoid nanosecond precision issues: time.Now() includes sub-second
// precision while time.Parse produces zero-nanosecond values, so a direct
// time.Before comparison can incorrectly conclude that a parsed timestamp
// with the same second is "before" now.
func nextMigrationTimestamp(migrationsDir string) string {
	candidateStr := time.Now().UTC().Format("20060102150405")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return candidateStr
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		name := entry.Name()
		if len(name) < 14 {
			continue
		}
		ts := name[:14]
		// Validate that it's a proper timestamp.
		parsed, err := time.Parse("20060102150405", ts)
		if err != nil {
			continue
		}
		// We need our timestamp to be strictly after this one.
		// Compare strings to avoid nanosecond precision issues.
		if ts >= candidateStr {
			candidateStr = parsed.Add(time.Second).Format("20060102150405")
		}
	}

	return candidateStr
}

// migrationExists checks whether a migration file with the given suffix
// already exists in the migrations directory.
func migrationExists(migrationsDir, suffix string) bool {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			return true
		}
	}

	return false
}

// generateQuerydefs generates the LLM querydefs.
func generateQuerydefs(cfg GenerateAllLLMConfig) error {
	querydefsDir := filepath.Join(cfg.ShipqRoot, "querydefs", "llm")
	if err := codegen.EnsureDir(querydefsDir); err != nil {
		return fmt.Errorf("create querydefs/llm directory: %w", err)
	}

	content := GenerateLLMQuerydefs(cfg.ModulePath, cfg.HasTenancy, cfg.HasAuth)
	outputPath := filepath.Join(querydefsDir, "queries.go")

	if _, err := codegen.WriteFileIfChanged(outputPath, content); err != nil {
		return fmt.Errorf("write querydefs: %w", err)
	}

	return nil
}

// generateStreamTypesMarker detects LLM-enabled channels and writes the
// marker file used by channel compile to inject stream types.
func generateStreamTypesMarker(cfg GenerateAllLLMConfig) error {
	if len(cfg.ChannelPkgs) == 0 {
		// No channels to scan — write an empty marker.
		return WriteLLMChannelsMarker(cfg.ShipqRoot, nil)
	}

	// ModulePath is the full import prefix (including any monorepo subpath),
	// so after stripping it the remainder is relative to ShipqRoot, not GoModRoot.
	llmChannels, err := DetectLLMChannels(cfg.ShipqRoot, cfg.ModulePath, cfg.ChannelPkgs)
	if err != nil {
		return fmt.Errorf("detect llm channels: %w", err)
	}

	return WriteLLMChannelsMarker(cfg.ShipqRoot, llmChannels)
}
