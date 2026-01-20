package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/inifile"
)

// Config holds the parsed configuration for portsql-api-httpgen.
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
}

// FindConfig locates the configuration file.
// It first checks the PORTSQL_API_HTTPGEN_CONFIG environment variable,
// then falls back to ./portsql-api-httpgen.ini in the current directory.
func FindConfig() (string, error) {
	if p := os.Getenv("PORTSQL_API_HTTPGEN_CONFIG"); p != "" {
		return p, nil
	}
	if _, err := os.Stat("./portsql-api-httpgen.ini"); err == nil {
		return "./portsql-api-httpgen.ini", nil
	}
	return "", errors.New("config not found: set PORTSQL_API_HTTPGEN_CONFIG or create ./portsql-api-httpgen.ini")
}

// LoadConfig reads and parses a config file from the given path.
func LoadConfig(path string) (*Config, error) {
	f, err := inifile.ParseFile(path)
	if err != nil {
		return nil, err
	}

	pkg := f.Get("httpgen", "package")
	if pkg == "" {
		return nil, errors.New("missing [httpgen] section with 'package' key")
	}

	// Optional middleware package
	mwPkg := f.Get("httpgen", "middleware_package")

	cfg := &Config{
		Package:           pkg,
		MiddlewarePackage: mwPkg,
	}

	// Parse OpenAPI settings
	if err := parseOpenAPIConfig(f, cfg); err != nil {
		return nil, err
	}

	// Parse Docs UI settings
	if err := parseDocsUIConfig(f, cfg); err != nil {
		return nil, err
	}

	// Normalize the config
	if err := normalizeConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseOpenAPIConfig parses OpenAPI-related configuration keys.
func parseOpenAPIConfig(f *inifile.File, cfg *Config) error {
	// openapi (bool)
	openAPIStr := f.Get("httpgen", "openapi")
	if openAPIStr != "" {
		b, err := parseBool(openAPIStr, "openapi")
		if err != nil {
			return err
		}
		cfg.OpenAPIEnabled = b
	}

	// openapi_output (string)
	cfg.OpenAPIOutput = f.Get("httpgen", "openapi_output")

	// openapi_title (string)
	cfg.OpenAPITitle = f.Get("httpgen", "openapi_title")

	// openapi_version (string)
	cfg.OpenAPIVersion = f.Get("httpgen", "openapi_version")

	// openapi_description (string)
	cfg.OpenAPIDescription = f.Get("httpgen", "openapi_description")

	// openapi_servers (comma-separated string)
	serversStr := f.Get("httpgen", "openapi_servers")
	if serversStr != "" {
		cfg.OpenAPIServers = parseServersList(serversStr)
	}

	return nil
}

// parseDocsUIConfig parses Docs UI-related configuration keys.
func parseDocsUIConfig(f *inifile.File, cfg *Config) error {
	// docs_ui (bool)
	docsUIStr := f.Get("httpgen", "docs_ui")
	if docsUIStr != "" {
		b, err := parseBool(docsUIStr, "docs_ui")
		if err != nil {
			return err
		}
		cfg.DocsUIEnabled = b
	}

	// docs_path (string)
	cfg.DocsPath = f.Get("httpgen", "docs_path")

	// openapi_json_path (string)
	cfg.OpenAPIJSONPath = f.Get("httpgen", "openapi_json_path")

	return nil
}

// normalizeConfig applies normalization rules and validates the config.
func normalizeConfig(cfg *Config) error {
	// Normalize openapi_output
	if cfg.OpenAPIOutput == "" {
		cfg.OpenAPIOutput = "openapi.json"
	} else {
		// Validate: must be relative path
		if filepath.IsAbs(cfg.OpenAPIOutput) {
			return fmt.Errorf("openapi_output must be relative path, got %q", cfg.OpenAPIOutput)
		}
	}

	// Normalize openapi_version
	if cfg.OpenAPIVersion == "" {
		cfg.OpenAPIVersion = "0.0.0"
	}

	// Normalize openapi_title (derive from package if empty)
	if cfg.OpenAPITitle == "" {
		cfg.OpenAPITitle = deriveTitle(cfg.Package)
	}

	// docs_ui=true implies openapi=true
	if cfg.DocsUIEnabled {
		cfg.OpenAPIEnabled = true
	}

	// Validate docs_path if docs_ui is enabled
	if cfg.DocsUIEnabled {
		if cfg.DocsPath == "" {
			return errors.New("docs_path is required when docs_ui is enabled")
		}

		// Normalize: remove trailing slash (except for root)
		cfg.DocsPath = strings.TrimSuffix(cfg.DocsPath, "/")

		// After trimming, if empty it means it was just "/"
		if cfg.DocsPath == "" {
			return errors.New("docs_path cannot be '/' (root path is too invasive)")
		}

		// Must start with /
		if !strings.HasPrefix(cfg.DocsPath, "/") {
			return fmt.Errorf("docs_path must start with /, got %q", cfg.DocsPath)
		}
	}

	// Normalize openapi_json_path
	if cfg.DocsUIEnabled {
		if cfg.OpenAPIJSONPath == "" {
			cfg.OpenAPIJSONPath = "/openapi.json"
		} else {
			// Must start with /
			if !strings.HasPrefix(cfg.OpenAPIJSONPath, "/") {
				return fmt.Errorf("openapi_json_path must start with /, got %q", cfg.OpenAPIJSONPath)
			}
			// Must not end with /
			if strings.HasSuffix(cfg.OpenAPIJSONPath, "/") {
				return fmt.Errorf("openapi_json_path must not have trailing slash, got %q", cfg.OpenAPIJSONPath)
			}
		}
	}

	// Validate openapi_output if openapi is enabled
	if cfg.OpenAPIEnabled {
		trimmed := strings.TrimSpace(cfg.OpenAPIOutput)
		if trimmed == "" {
			return errors.New("openapi_output cannot be empty when openapi is enabled")
		}
		if filepath.IsAbs(trimmed) {
			return fmt.Errorf("openapi_output must be relative path, got %q", cfg.OpenAPIOutput)
		}
	}

	return nil
}

// parseBool parses a boolean value from a string.
// Accepts: true, false, 1, 0 (case-insensitive for true/false).
func parseBool(s, key string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value for %s: %q (expected true/false/1/0)", key, s)
	}
}

// parseServersList parses a comma-separated list of server URLs.
// Empty entries are dropped.
func parseServersList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
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
