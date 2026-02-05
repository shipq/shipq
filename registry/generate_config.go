package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/httpserver/config"
)

// generateConfig generates the config package files (config.go and config_test.go).
func generateConfig(cfg CompileConfig) error {
	configCfg := config.ConfigGenConfig{
		ModulePath: cfg.ModulePath,
	}

	// Generate config.go
	configCode, err := config.GenerateConfig(configCfg)
	if err != nil {
		return fmt.Errorf("failed to generate config.go: %w", err)
	}

	// Generate config_test.go
	configTestCode, err := config.GenerateConfigTest(configCfg)
	if err != nil {
		return fmt.Errorf("failed to generate config_test.go: %w", err)
	}

	// Ensure config directory exists (in shipq root)
	configDir := filepath.Join(cfg.ShipqRoot, "config")
	if err := codegen.EnsureDir(configDir); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config.go
	configOutputPath := filepath.Join(configDir, "config.go")
	configWritten, err := codegen.WriteFileIfChanged(configOutputPath, configCode)
	if err != nil {
		return fmt.Errorf("failed to write config.go: %w", err)
	}

	if cfg.Verbose && configWritten {
		fmt.Printf("Generated %s\n", configOutputPath)
	}

	// Write config_test.go
	configTestOutputPath := filepath.Join(configDir, "config_test.go")
	configTestWritten, err := codegen.WriteFileIfChanged(configTestOutputPath, configTestCode)
	if err != nil {
		return fmt.Errorf("failed to write config_test.go: %w", err)
	}

	if cfg.Verbose && configTestWritten {
		fmt.Printf("Generated %s\n", configTestOutputPath)
	}

	return nil
}
