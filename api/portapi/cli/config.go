package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/internal/config"
)

// Config holds the parsed configuration for the PortAPI generator.
type Config struct {
	Package           string // e.g. "./api"
	MiddlewarePackage string // e.g. "./middleware" (optional)

	// OpenAPI settings
	OpenAPIEnabled     bool
	OpenAPIOutput      string   // e.g. "openapi.json"
	OpenAPITitle       string   // e.g. "My API"
	OpenAPIVersion     string   // e.g. "1.0.0"
	OpenAPIDescription string   // e.g. "API description"
	OpenAPIServers     []string // e.g. ["http://localhost:8080", "https://api.example.com"]

	// Docs UI settings (Stoplight Elements)
	DocsUIEnabled   bool
	DocsPath        string // e.g. "/docs"
	OpenAPIJSONPath string // e.g. "/openapi.json"

	// Test client settings
	TestClientEnabled  bool   // Enable test client generation
	TestClientFilename string // Output filename (default: "zz_generated_testclient_test.go")
}

// FindConfig locates the shipq.ini configuration file.
// Returns the directory containing shipq.ini (the project root).
func FindConfig() (string, error) {
	exists, err := config.Exists("")
	if err != nil {
		return "", err
	}
	if !exists {
		return "", errors.New("shipq.ini not found in current directory\n  Hint: Run 'shipq init' to create a new project, or ensure you're in the project root directory")
	}
	return "", nil // Empty string means current directory
}

// LoadConfig reads shipq.ini from the given directory and returns a Config.
// If dir is empty, uses the current working directory.
func LoadConfig(dir string) (*Config, error) {
	shipqCfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	return ConfigFromShipq(shipqCfg), nil
}

// ConfigFromShipq converts a unified ShipqConfig to a PortAPI generator Config.
func ConfigFromShipq(shipqCfg *config.ShipqConfig) *Config {
	apiCfg := &shipqCfg.API

	cfg := &Config{
		Package:            apiCfg.Package,
		MiddlewarePackage:  apiCfg.MiddlewarePackage,
		OpenAPIEnabled:     apiCfg.OpenAPIEnabled,
		OpenAPIOutput:      apiCfg.OpenAPIOutput,
		OpenAPITitle:       apiCfg.OpenAPITitle,
		OpenAPIVersion:     apiCfg.OpenAPIVersion,
		OpenAPIDescription: apiCfg.OpenAPIDescription,
		OpenAPIServers:     apiCfg.OpenAPIServers,
		DocsUIEnabled:      apiCfg.DocsUIEnabled,
		DocsPath:           apiCfg.DocsPath,
		OpenAPIJSONPath:    apiCfg.OpenAPIJSONPath,
		TestClientEnabled:  apiCfg.TestClientEnabled,
		TestClientFilename: apiCfg.TestClientFilename,
	}

	// Apply local normalization if needed
	if err := normalizeConfig(cfg); err != nil {
		// Normalization errors should have been caught by config.Load,
		// but we still apply it here for consistency
		return cfg
	}

	return cfg
}

// normalizeConfig applies normalization rules and validates the config.
// Note: Most validation is done in the unified config package, but we keep
// some local normalization for backwards compatibility.
func normalizeConfig(cfg *Config) error {
	// Normalize openapi_output
	if cfg.OpenAPIOutput == "" {
		cfg.OpenAPIOutput = "openapi.json"
	} else {
		// Validate: must be relative path
		if filepath.IsAbs(cfg.OpenAPIOutput) {
			return fmt.Errorf("shipq.ini: api.openapi_output must be relative path, got %q", cfg.OpenAPIOutput)
		}
	}

	// Normalize openapi_version
	if cfg.OpenAPIVersion == "" {
		cfg.OpenAPIVersion = "0.0.0"
	}

	// Normalize openapi_title (derive from package if empty)
	if cfg.OpenAPITitle == "" && cfg.Package != "" {
		cfg.OpenAPITitle = deriveTitle(cfg.Package)
	}

	// docs_ui=true implies openapi=true
	if cfg.DocsUIEnabled {
		cfg.OpenAPIEnabled = true
	}

	// Validate docs_path if docs_ui is enabled
	if cfg.DocsUIEnabled {
		if cfg.DocsPath == "" {
			return errors.New("shipq.ini: api.docs_path is required when api.docs_ui is enabled")
		}

		// Normalize: remove trailing slash (except for root)
		cfg.DocsPath = strings.TrimSuffix(cfg.DocsPath, "/")

		// After trimming, if empty it means it was just "/"
		if cfg.DocsPath == "" {
			return errors.New("shipq.ini: api.docs_path cannot be '/' (root path is too invasive)")
		}

		// Must start with /
		if !strings.HasPrefix(cfg.DocsPath, "/") {
			return fmt.Errorf("shipq.ini: api.docs_path must start with /, got %q", cfg.DocsPath)
		}
	}

	// Normalize openapi_json_path
	if cfg.DocsUIEnabled {
		if cfg.OpenAPIJSONPath == "" {
			cfg.OpenAPIJSONPath = "/openapi.json"
		} else {
			// Must start with /
			if !strings.HasPrefix(cfg.OpenAPIJSONPath, "/") {
				return fmt.Errorf("shipq.ini: api.openapi_json_path must start with /, got %q", cfg.OpenAPIJSONPath)
			}
			// Must not end with /
			if strings.HasSuffix(cfg.OpenAPIJSONPath, "/") {
				return fmt.Errorf("shipq.ini: api.openapi_json_path must not have trailing slash, got %q", cfg.OpenAPIJSONPath)
			}
		}
	}

	// Validate openapi_output if openapi is enabled
	if cfg.OpenAPIEnabled {
		trimmed := strings.TrimSpace(cfg.OpenAPIOutput)
		if trimmed == "" {
			return errors.New("shipq.ini: api.openapi_output cannot be empty when api.openapi is enabled")
		}
		if filepath.IsAbs(trimmed) {
			return fmt.Errorf("shipq.ini: api.openapi_output must be relative path, got %q", cfg.OpenAPIOutput)
		}
	}

	// Normalize test client settings
	if cfg.TestClientFilename == "" {
		cfg.TestClientFilename = "zz_generated_testclient_test.go"
	} else {
		// Validate: must end with _test.go for test-only compilation
		if !strings.HasSuffix(cfg.TestClientFilename, "_test.go") {
			return fmt.Errorf("shipq.ini: api.test_client_filename must end with _test.go, got %q", cfg.TestClientFilename)
		}
	}

	return nil
}

// deriveTitle derives an API title from a package path.
// It uses the last segment of the path.
func deriveTitle(pkg string) string {
	// Clean up the path
	pkg = strings.TrimPrefix(pkg, "./")
	pkg = strings.TrimSuffix(pkg, "/")

	// Get the last segment
	parts := strings.Split(pkg, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return pkg
}
