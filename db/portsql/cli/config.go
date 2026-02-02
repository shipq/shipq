package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/shipq/shipq/internal/config"
)

// ValidDialects is the list of supported database dialects.
var ValidDialects = []string{"postgres", "mysql", "sqlite"}

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

	// Database setup settings
	Name      string // Base name for derived dev/test databases
	DevName   string // Explicit dev database name (overrides derivation)
	TestName  string // Explicit test database name (overrides derivation)
	DataDir   string // Directory for local DB data (e.g., .shipq/db/postgres)
	LocalPort string // Port for local DB servers
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

// LoadConfig reads shipq.ini and returns a Config.
// The configPath parameter specifies the directory to look for shipq.ini.
// If empty, it defaults to the current working directory.
func LoadConfig(configPath string) (*Config, error) {
	shipqCfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	return ConfigFromShipq(shipqCfg), nil
}

// ConfigFromShipq converts a unified ShipqConfig to a PortSQL Config.
func ConfigFromShipq(shipqCfg *config.ShipqConfig) *Config {
	cfg := DefaultConfig()

	// Map DB settings
	cfg.Database.URL = shipqCfg.DB.URL
	cfg.Database.Dialects = shipqCfg.DB.Dialects

	// Map database setup settings
	cfg.Database.Name = shipqCfg.DB.Name
	cfg.Database.DevName = shipqCfg.DB.DevName
	cfg.Database.TestName = shipqCfg.DB.TestName
	cfg.Database.DataDir = shipqCfg.DB.DataDir
	cfg.Database.LocalPort = shipqCfg.DB.LocalPort

	// Map paths
	cfg.Paths.Migrations = shipqCfg.DB.Migrations
	cfg.Paths.Schematypes = shipqCfg.DB.Schematypes
	cfg.Paths.QueriesIn = shipqCfg.DB.QueriesIn
	cfg.Paths.QueriesOut = shipqCfg.DB.QueriesOut

	// Map CRUD settings
	cfg.CRUD.GlobalScope = shipqCfg.DB.GlobalScope
	cfg.CRUD.GlobalOrder = shipqCfg.DB.GlobalOrder
	cfg.CRUD.TableScopes = shipqCfg.DB.TableScopes
	cfg.CRUD.TableOrders = shipqCfg.DB.TableOrders

	// Ensure maps are initialized
	if cfg.CRUD.TableScopes == nil {
		cfg.CRUD.TableScopes = make(map[string]string)
	}
	if cfg.CRUD.TableOrders == nil {
		cfg.CRUD.TableOrders = make(map[string]string)
	}

	return cfg
}

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
			"invalid dialect %q in shipq.ini\n"+
				"  Supported dialects: postgres, mysql, sqlite\n"+
				"  Example: dialects = postgres, sqlite\n"+
				"  Hint: Check for typos or extra spaces",
			s,
		)
	}
}
