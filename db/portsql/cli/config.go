package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/inifile"
)

// ValidDialects is the list of supported database dialects.
var ValidDialects = []string{"postgres", "mysql", "sqlite"}

// normalizeDialect normalizes and validates a dialect string.
// Returns the normalized dialect (lowercased, trimmed) or an error if invalid.
// Returns empty string for empty input (to allow filtering).
func normalizeDialect(s string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(s))
	if d == "" {
		return "", nil // Will be filtered out by caller
	}
	switch d {
	case "postgres", "mysql", "sqlite":
		return d, nil
	default:
		return "", fmt.Errorf(
			"invalid dialect %q in portsql.ini\n"+
				"  Supported dialects: postgres, mysql, sqlite\n"+
				"  Example: dialects = postgres, sqlite\n"+
				"  Hint: Check for typos or extra spaces",
			s,
		)
	}
}

// Config holds the portsql configuration.
type Config struct {
	Database DatabaseConfig
	Paths    PathsConfig
	CRUD     CRUDConfig
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL      string
	Dialects []string // Explicit list of dialects to generate (e.g., ["sqlite", "postgres"])
}

// GetDialects returns the list of dialects to generate code for.
// If Dialects is explicitly set, returns that.
// Otherwise, infers from the database URL.
func (c *DatabaseConfig) GetDialects() []string {
	if len(c.Dialects) > 0 {
		return c.Dialects
	}
	// Fall back to URL-inferred dialect
	dialect := ParseDialect(c.URL)
	if dialect != "" {
		return []string{dialect}
	}
	return nil
}

// PathsConfig holds file path settings.
type PathsConfig struct {
	Migrations  string
	Schematypes string
	QueriesIn   string
	QueriesOut  string
}

// CRUDConfig holds CRUD generation settings.
type CRUDConfig struct {
	// GlobalScope is the default scope column for all tables.
	// Example: "org_id", "tenant_id"
	GlobalScope string
	// TableScopes holds per-table scope overrides.
	// Key is table name, value is scope column (empty string = no scope).
	TableScopes map[string]string

	// GlobalOrder is the default sort order for list queries.
	// Valid values: "asc" (oldest first), "desc" (newest first, default).
	GlobalOrder string
	// TableOrders holds per-table order overrides.
	// Key is table name, value is "asc" or "desc".
	TableOrders map[string]string
}

// GetScopeForTable returns the scope column for a table.
// It checks table-specific overrides first, then falls back to global scope.
// Returns empty string if no scope is configured.
func (c *CRUDConfig) GetScopeForTable(tableName string) string {
	// Check if table has a specific scope override
	if c.TableScopes != nil {
		if scope, exists := c.TableScopes[tableName]; exists {
			return scope // Could be empty string (explicit no-scope)
		}
	}
	// Fall back to global scope
	return c.GlobalScope
}

// GetOrderForTable returns the sort order for a table.
// It checks table-specific overrides first, then falls back to global order.
// Returns "desc" (newest first) if no order is configured.
func (c *CRUDConfig) GetOrderForTable(tableName string) string {
	// Check if table has a specific order override
	if c.TableOrders != nil {
		if order, exists := c.TableOrders[tableName]; exists {
			return order
		}
	}
	// Fall back to global order
	if c.GlobalOrder != "" {
		return c.GlobalOrder
	}
	// Default to desc (newest first)
	return "desc"
}

// HasTableOverride returns true if the table has a specific scope override.
func (c *CRUDConfig) HasTableOverride(tableName string) bool {
	if c.TableScopes == nil {
		return false
	}
	_, exists := c.TableScopes[tableName]
	return exists
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		Paths: PathsConfig{
			Migrations:  "migrations",
			Schematypes: "schematypes",
			QueriesIn:   "querydef",
			QueriesOut:  "queries",
		},
		CRUD: CRUDConfig{
			GlobalScope: "",
			TableScopes: make(map[string]string),
			GlobalOrder: "",
			TableOrders: make(map[string]string),
		},
	}
}

// LoadConfig reads portsql.ini if present, falls back to defaults + DATABASE_URL.
// The configPath parameter specifies the directory to look for portsql.ini.
// If empty, it defaults to the current working directory.
func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath == "" {
		var err error
		configPath, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	iniPath := filepath.Join(configPath, "portsql.ini")
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		// No config file, use defaults
		return cfg, nil
	}

	f, err := inifile.ParseFile(iniPath)
	if err != nil {
		return nil, err
	}

	// [database] section
	if url := f.Get("database", "url"); url != "" {
		cfg.Database.URL = url
	}
	if dialectsStr := f.Get("database", "dialects"); dialectsStr != "" {
		parts := strings.Split(dialectsStr, ",")
		var dialects []string
		for _, part := range parts {
			d, err := normalizeDialect(part)
			if err != nil {
				return nil, err
			}
			if d != "" { // Filter out empty entries (e.g., trailing comma)
				dialects = append(dialects, d)
			}
		}
		cfg.Database.Dialects = dialects
	}

	// [paths] section
	if v := f.Get("paths", "migrations"); v != "" {
		cfg.Paths.Migrations = v
	}
	if v := f.Get("paths", "schematypes"); v != "" {
		cfg.Paths.Schematypes = v
	}
	if v := f.Get("paths", "queries_in"); v != "" {
		cfg.Paths.QueriesIn = v
	}
	if v := f.Get("paths", "queries_out"); v != "" {
		cfg.Paths.QueriesOut = v
	}

	// [crud] section
	if v := f.Get("crud", "scope"); v != "" {
		cfg.CRUD.GlobalScope = v
	}
	if v := f.Get("crud", "order"); v != "" {
		cfg.CRUD.GlobalOrder = v
	}

	// [crud.tablename] sections
	for _, section := range f.SectionsWithPrefix("crud.") {
		tableName := strings.TrimPrefix(section.Name, "crud.")
		// Check if scope key exists (even if empty value)
		if section.HasKey("scope") {
			if cfg.CRUD.TableScopes == nil {
				cfg.CRUD.TableScopes = make(map[string]string)
			}
			cfg.CRUD.TableScopes[tableName] = section.Get("scope")
		}
		if order := section.Get("order"); order != "" {
			if cfg.CRUD.TableOrders == nil {
				cfg.CRUD.TableOrders = make(map[string]string)
			}
			cfg.CRUD.TableOrders[tableName] = order
		}
	}

	// If database URL is still empty, try DATABASE_URL env var
	if cfg.Database.URL == "" {
		cfg.Database.URL = os.Getenv("DATABASE_URL")
	}

	return cfg, nil
}
