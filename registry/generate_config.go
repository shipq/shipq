package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
)

// GenerateConfigEarly generates the config package before handler compilation.
// This is needed when handlers import the config package (e.g., auth handlers
// that need COOKIE_SECRET). Call this before registry.Run() in such cases.
//
// Parameters:
//   - shipqRoot: directory containing shipq.ini (where config/ will be created)
//   - goModRoot: directory containing go.mod
//   - dialect: the database dialect ("mysql", "postgres", "sqlite")
func GenerateConfigEarly(shipqRoot, goModRoot, dialect string) error {
	return GenerateConfigEarlyWithOptions(shipqRoot, goModRoot, dialect, false)
}

// GenerateConfigEarlyWithOptions is like GenerateConfigEarly but allows
// specifying whether the files feature is enabled.
func GenerateConfigEarlyWithOptions(shipqRoot, goModRoot, dialect string, filesEnabled bool) error {
	return GenerateConfigEarlyWithFullOptions(ConfigEarlyOptions{
		ShipqRoot:    shipqRoot,
		GoModRoot:    goModRoot,
		Dialect:      dialect,
		FilesEnabled: filesEnabled,
	})
}

// ConfigEarlyOptions holds all options for early config generation.
type ConfigEarlyOptions struct {
	ShipqRoot      string
	GoModRoot      string
	Dialect        string
	FilesEnabled   bool
	WorkersEnabled bool
	OAuthGoogle    bool
	OAuthGitHub    bool
	EmailEnabled   bool
	DevDefaults    configpkg.DevDefaults
	CustomEnvVars  []configpkg.CustomEnvVar
}

// GenerateConfigEarlyWithFullOptions generates the config package with full
// control over which features are enabled.
func GenerateConfigEarlyWithFullOptions(opts ConfigEarlyOptions) error {
	moduleInfo, err := codegen.GetModuleInfo(opts.GoModRoot, opts.ShipqRoot)
	if err != nil {
		return fmt.Errorf("failed to get module info: %w", err)
	}

	cfg := CompileConfig{
		ShipqRoot:      opts.ShipqRoot,
		GoModRoot:      opts.GoModRoot,
		ModulePath:     moduleInfo.FullImportPath(""),
		DBDialect:      opts.Dialect,
		FilesEnabled:   opts.FilesEnabled,
		WorkersEnabled: opts.WorkersEnabled,
		OAuthGoogle:    opts.OAuthGoogle,
		OAuthGitHub:    opts.OAuthGitHub,
		EmailEnabled:   opts.EmailEnabled,
		DevDefaults:    opts.DevDefaults,
		CustomEnvVars:  opts.CustomEnvVars,
	}

	return generateConfig(cfg)
}

// generateConfig generates the config package files (config.go and config_test.go).
func generateConfig(cfg CompileConfig) error {
	configCfg := configpkg.ConfigGenConfig{
		ModulePath:     cfg.ModulePath,
		Dialect:        cfg.DBDialect,
		FilesEnabled:   cfg.FilesEnabled,
		WorkersEnabled: cfg.WorkersEnabled,
		OAuthGoogle:    cfg.OAuthGoogle,
		OAuthGitHub:    cfg.OAuthGitHub,
		EmailEnabled:   cfg.EmailEnabled,
		DevDefaults:    cfg.DevDefaults,
		CustomEnvVars:  cfg.CustomEnvVars,
	}

	// Generate config.go
	configCode, err := configpkg.GenerateConfig(configCfg)
	if err != nil {
		return fmt.Errorf("failed to generate config.go: %w", err)
	}

	// Generate config_test.go
	configTestCode, err := configpkg.GenerateConfigTest(configCfg)
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
