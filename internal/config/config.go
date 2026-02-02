// Package config provides unified configuration loading from shipq.ini.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/inifile"
)

// ConfigFilename is the name of the unified config file.
const ConfigFilename = "shipq.ini"

// ValidDialects is the list of supported database dialects.
var ValidDialects = []string{"postgres", "mysql", "sqlite"}

// ShipqConfig holds the complete configuration from shipq.ini.
type ShipqConfig struct {
	// ConfigDir is the directory containing shipq.ini (the project root).
	ConfigDir string

	Project ProjectConfig
	DB      DBConfig
	API     APIConfig
}

// ProjectConfig holds project-level settings from the [project] section.
type ProjectConfig struct {
	IncludeLogging bool
}

// DBConfig holds database and PortSQL settings from the [db] section.
type DBConfig struct {
	URL         string
	Dialects    []string
	Migrations  string
	Schematypes string
	QueriesIn   string
	QueriesOut  string

	// Database setup settings
	Name      string // Base name for derived dev/test databases
	DevName   string // Explicit dev database name (overrides derivation)
	TestName  string // Explicit test database name (overrides derivation)
	DataDir   string // Directory for local DB data (e.g., .shipq/db/postgres)
	LocalPort string // Port for local DB servers

	// CRUD settings
	GlobalScope string
	TableScopes map[string]string
	GlobalOrder string
	TableOrders map[string]string
}

// APIConfig holds API generator settings from the [api] section.
type APIConfig struct {
	Package           string
	MiddlewarePackage string

	// OpenAPI
	OpenAPIEnabled     bool
	OpenAPIOutput      string
	OpenAPITitle       string
	OpenAPIVersion     string
	OpenAPIDescription string
	OpenAPIServers     []string

	// Docs UI
	DocsUIEnabled   bool
	DocsPath        string
	OpenAPIJSONPath string

	// Test Client
	TestClientEnabled  bool
	TestClientFilename string
}

// Load reads shipq.ini from the given directory (or CWD if empty).
// Returns an error if shipq.ini is not found.
func Load(dir string) (*ShipqConfig, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	iniPath := filepath.Join(dir, ConfigFilename)
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s not found in %s\n"+
			"  Hint: Run 'shipq init' to create a new project, or ensure you're in the project root directory",
			ConfigFilename, dir)
	}

	f, err := inifile.ParseFile(iniPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ConfigFilename, err)
	}

	cfg := &ShipqConfig{
		ConfigDir: dir,
		Project:   defaultProjectConfig(),
		DB:        defaultDBConfig(),
		API:       defaultAPIConfig(),
	}

	// Parse [project] section
	if err := parseProjectSection(f, &cfg.Project); err != nil {
		return nil, err
	}

	// Parse [db] section
	if err := parseDBSection(f, &cfg.DB); err != nil {
		return nil, err
	}

	// Parse [api] section
	if err := parseAPISection(f, &cfg.API); err != nil {
		return nil, err
	}

	// Parse [crud.*] sections
	if err := parseCRUDSections(f, &cfg.DB); err != nil {
		return nil, err
	}

	// Apply DATABASE_URL fallback if db.url is empty
	if cfg.DB.URL == "" {
		cfg.DB.URL = os.Getenv("DATABASE_URL")
	}

	return cfg, nil
}

// LoadDBOnly loads only the DB configuration from shipq.ini.
// This is a convenience function for PortSQL.
func LoadDBOnly(dir string) (*ShipqConfig, error) {
	return Load(dir)
}

// LoadAPIOnly loads only the API configuration from shipq.ini.
// This is a convenience function for the API generator.
func LoadAPIOnly(dir string) (*ShipqConfig, error) {
	return Load(dir)
}

// defaultDBConfig returns DBConfig with default values.
func defaultDBConfig() DBConfig {
	return DBConfig{
		URL:         "",
		Dialects:    nil, // Will be inferred from URL if not set
		Migrations:  "migrations",
		Schematypes: "schematypes",
		QueriesIn:   "querydef",
		QueriesOut:  "queries",
		Name:        "",
		DevName:     "",
		TestName:    "",
		DataDir:     "",
		LocalPort:   "",
		GlobalScope: "",
		TableScopes: make(map[string]string),
		GlobalOrder: "",
		TableOrders: make(map[string]string),
	}
}

// defaultProjectConfig returns ProjectConfig with default values.
func defaultProjectConfig() ProjectConfig {
	return ProjectConfig{
		IncludeLogging: true,
	}
}

// defaultAPIConfig returns APIConfig with default values.
func defaultAPIConfig() APIConfig {
	return APIConfig{
		Package:            "",
		MiddlewarePackage:  "",
		OpenAPIEnabled:     false,
		OpenAPIOutput:      "openapi.json",
		OpenAPITitle:       "",
		OpenAPIVersion:     "0.0.0",
		OpenAPIDescription: "",
		OpenAPIServers:     nil,
		DocsUIEnabled:      false,
		DocsPath:           "",
		OpenAPIJSONPath:    "/openapi.json",
		TestClientEnabled:  false,
		TestClientFilename: "zz_generated_testclient_test.go",
	}
}

// parseProjectSection parses the [project] section from the INI file.
func parseProjectSection(f *inifile.File, cfg *ProjectConfig) error {
	if v := f.Get("project", "include_logging"); v != "" {
		b, err := parseBool(v, "project.include_logging")
		if err != nil {
			return err
		}
		cfg.IncludeLogging = b
	}
	return nil
}

// parseDBSection parses the [db] section from the INI file.
func parseDBSection(f *inifile.File, cfg *DBConfig) error {
	// url
	if v := f.Get("db", "url"); v != "" {
		cfg.URL = v
	}

	// dialects
	if v := f.Get("db", "dialects"); v != "" {
		dialects, err := parseDialects(v)
		if err != nil {
			return err
		}
		cfg.Dialects = dialects
	}

	// paths
	if v := f.Get("db", "migrations"); v != "" {
		cfg.Migrations = v
	}
	if v := f.Get("db", "schematypes"); v != "" {
		cfg.Schematypes = v
	}
	if v := f.Get("db", "queries_in"); v != "" {
		cfg.QueriesIn = v
	}
	if v := f.Get("db", "queries_out"); v != "" {
		cfg.QueriesOut = v
	}

	// Database setup settings
	if v := f.Get("db", "name"); v != "" {
		cfg.Name = v
	}
	if v := f.Get("db", "dev_name"); v != "" {
		cfg.DevName = v
	}
	if v := f.Get("db", "test_name"); v != "" {
		cfg.TestName = v
	}
	if v := f.Get("db", "data_dir"); v != "" {
		cfg.DataDir = v
	}
	if v := f.Get("db", "local_port"); v != "" {
		cfg.LocalPort = v
	}

	// CRUD global settings
	if v := f.Get("db", "scope"); v != "" {
		cfg.GlobalScope = v
	}
	if v := f.Get("db", "order"); v != "" {
		cfg.GlobalOrder = v
	}

	return nil
}

// parseAPISection parses the [api] section from the INI file.
func parseAPISection(f *inifile.File, cfg *APIConfig) error {
	// package (required if [api] section exists)
	cfg.Package = f.Get("api", "package")

	// middleware_package (optional)
	cfg.MiddlewarePackage = f.Get("api", "middleware_package")

	// OpenAPI settings
	if v := f.Get("api", "openapi"); v != "" {
		b, err := parseBool(v, "api.openapi")
		if err != nil {
			return err
		}
		cfg.OpenAPIEnabled = b
	}
	if v := f.Get("api", "openapi_output"); v != "" {
		cfg.OpenAPIOutput = v
	}
	if v := f.Get("api", "openapi_title"); v != "" {
		cfg.OpenAPITitle = v
	}
	if v := f.Get("api", "openapi_version"); v != "" {
		cfg.OpenAPIVersion = v
	}
	if v := f.Get("api", "openapi_description"); v != "" {
		cfg.OpenAPIDescription = v
	}
	if v := f.Get("api", "openapi_servers"); v != "" {
		cfg.OpenAPIServers = parseServersList(v)
	}

	// Docs UI settings
	if v := f.Get("api", "docs_ui"); v != "" {
		b, err := parseBool(v, "api.docs_ui")
		if err != nil {
			return err
		}
		cfg.DocsUIEnabled = b
	}
	if v := f.Get("api", "docs_path"); v != "" {
		cfg.DocsPath = v
	}
	if v := f.Get("api", "openapi_json_path"); v != "" {
		cfg.OpenAPIJSONPath = v
	}

	// Test client settings
	if v := f.Get("api", "test_client"); v != "" {
		b, err := parseBool(v, "api.test_client")
		if err != nil {
			return err
		}
		cfg.TestClientEnabled = b
	}
	if v := f.Get("api", "test_client_filename"); v != "" {
		cfg.TestClientFilename = v
	}

	// Normalize and validate API config
	if err := normalizeAPIConfig(cfg); err != nil {
		return err
	}

	return nil
}

// parseCRUDSections parses [crud.*] sections for per-table overrides.
func parseCRUDSections(f *inifile.File, cfg *DBConfig) error {
	for _, section := range f.SectionsWithPrefix("crud.") {
		tableName := strings.TrimPrefix(section.Name, "crud.")

		// Check if scope key exists (even if empty value)
		if section.HasKey("scope") {
			if cfg.TableScopes == nil {
				cfg.TableScopes = make(map[string]string)
			}
			cfg.TableScopes[tableName] = section.Get("scope")
		}

		if order := section.Get("order"); order != "" {
			if cfg.TableOrders == nil {
				cfg.TableOrders = make(map[string]string)
			}
			cfg.TableOrders[tableName] = order
		}
	}

	return nil
}

// normalizeAPIConfig applies normalization rules and validates the API config.
func normalizeAPIConfig(cfg *APIConfig) error {
	// Validate openapi_output
	if cfg.OpenAPIOutput != "" {
		if filepath.IsAbs(cfg.OpenAPIOutput) {
			return fmt.Errorf("%s: api.openapi_output must be a relative path, got %q", ConfigFilename, cfg.OpenAPIOutput)
		}
	}

	// Derive openapi_title from package if empty
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
			return fmt.Errorf("%s: api.docs_path is required when api.docs_ui is enabled", ConfigFilename)
		}

		// Normalize: remove trailing slash (except for root)
		cfg.DocsPath = strings.TrimSuffix(cfg.DocsPath, "/")

		// After trimming, if empty it means it was just "/"
		if cfg.DocsPath == "" {
			return fmt.Errorf("%s: api.docs_path cannot be '/' (root path is too invasive)", ConfigFilename)
		}

		// Must start with /
		if !strings.HasPrefix(cfg.DocsPath, "/") {
			return fmt.Errorf("%s: api.docs_path must start with /, got %q", ConfigFilename, cfg.DocsPath)
		}
	}

	// Validate openapi_json_path
	if cfg.DocsUIEnabled && cfg.OpenAPIJSONPath != "" {
		// Must start with /
		if !strings.HasPrefix(cfg.OpenAPIJSONPath, "/") {
			return fmt.Errorf("%s: api.openapi_json_path must start with /, got %q", ConfigFilename, cfg.OpenAPIJSONPath)
		}
		// Must not end with /
		if strings.HasSuffix(cfg.OpenAPIJSONPath, "/") {
			return fmt.Errorf("%s: api.openapi_json_path must not have trailing slash, got %q", ConfigFilename, cfg.OpenAPIJSONPath)
		}
	}

	// Validate openapi_output if openapi is enabled
	if cfg.OpenAPIEnabled {
		trimmed := strings.TrimSpace(cfg.OpenAPIOutput)
		if trimmed == "" {
			return fmt.Errorf("%s: api.openapi_output cannot be empty when api.openapi is enabled", ConfigFilename)
		}
		if filepath.IsAbs(trimmed) {
			return fmt.Errorf("%s: api.openapi_output must be a relative path, got %q", ConfigFilename, cfg.OpenAPIOutput)
		}
	}

	// Validate test_client_filename
	if cfg.TestClientFilename != "" {
		if !strings.HasSuffix(cfg.TestClientFilename, "_test.go") {
			return fmt.Errorf("%s: api.test_client_filename must end with _test.go, got %q", ConfigFilename, cfg.TestClientFilename)
		}
	}

	return nil
}

// parseDialects parses a comma-separated list of dialects.
func parseDialects(s string) ([]string, error) {
	parts := strings.Split(s, ",")
	var dialects []string

	for _, part := range parts {
		d, err := normalizeDialect(part)
		if err != nil {
			return nil, err
		}
		if d != "" { // Filter out empty entries
			dialects = append(dialects, d)
		}
	}

	return dialects, nil
}

// normalizeDialect normalizes and validates a dialect string.
func normalizeDialect(s string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(s))
	if d == "" {
		return "", nil // Will be filtered out
	}

	switch d {
	case "postgres", "mysql", "sqlite":
		return d, nil
	default:
		return "", fmt.Errorf("%s: invalid db.dialects value %q\n"+
			"  Supported dialects: postgres, mysql, sqlite\n"+
			"  Example: dialects = postgres, sqlite\n"+
			"  Hint: Check for typos or extra spaces",
			ConfigFilename, s)
	}
}

// parseBool parses a boolean value from a string.
func parseBool(s, key string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s: invalid boolean value for %s: %q (expected true/false/1/0)", ConfigFilename, key, s)
	}
}

// parseServersList parses a comma-separated list of server URLs.
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
func deriveTitle(pkg string) string {
	pkg = strings.TrimPrefix(pkg, "./")
	pkg = strings.TrimSuffix(pkg, "/")

	parts := strings.Split(pkg, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return pkg
}

// Exists checks if shipq.ini exists in the given directory.
func Exists(dir string) (bool, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return false, err
		}
	}

	iniPath := filepath.Join(dir, ConfigFilename)
	_, err := os.Stat(iniPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ErrConfigNotFound is returned when shipq.ini is not found.
var ErrConfigNotFound = errors.New("shipq.ini not found")
