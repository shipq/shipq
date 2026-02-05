package crud

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/inifile"
)

// CRUDConfig holds the global and per-table CRUD configuration.
type CRUDConfig struct {
	// GlobalScope is the default scope column from [db] scope
	GlobalScope string

	// GlobalScopeTable is the table that scope references (from [db] scope_table or inferred)
	GlobalScopeTable string

	// GlobalOrderAsc is true if the default order is ASC (oldest first)
	// Default is false (DESC, newest first)
	GlobalOrderAsc bool

	// TableOpts holds per-table CRUD options, keyed by table name
	TableOpts map[string]codegen.CRUDOptions
}

// LoadCRUDConfig reads scope and order configuration from shipq.ini.
// It merges global defaults from [db] with per-table overrides from [crud.<table>] sections.
// The tables parameter is used to determine which tables to generate options for.
func LoadCRUDConfig(ini *inifile.File, tables []string) (*CRUDConfig, error) {
	cfg := &CRUDConfig{
		TableOpts: make(map[string]codegen.CRUDOptions),
	}

	// Read global defaults from [db]
	cfg.GlobalScope = ini.Get("db", "scope")
	cfg.GlobalScopeTable = ini.Get("db", "scope_table")

	// Infer scope table from column name if not explicitly set
	if cfg.GlobalScope != "" && cfg.GlobalScopeTable == "" {
		cfg.GlobalScopeTable = InferScopeTable(cfg.GlobalScope)
	}

	// Read global order
	globalOrder := strings.ToLower(ini.Get("db", "order"))
	cfg.GlobalOrderAsc = (globalOrder == "asc")

	// Build options for each table
	for _, tableName := range tables {
		opts := codegen.CRUDOptions{
			ScopeColumn: cfg.GlobalScope,
			OrderAsc:    cfg.GlobalOrderAsc,
		}

		// Check for per-table override in [crud.<table>] section
		sectionName := "crud." + tableName
		section := ini.Section(sectionName)
		if section != nil {
			// Override scope if specified
			if section.HasKey("scope") {
				opts.ScopeColumn = section.Get("scope")
			}

			// Override order if specified
			if section.HasKey("order") {
				tableOrder := strings.ToLower(section.Get("order"))
				opts.OrderAsc = (tableOrder == "asc")
			}
		}

		cfg.TableOpts[tableName] = opts
	}

	return cfg, nil
}

// InferScopeTable infers the referenced table name from a scope column name.
// For example:
//   - organization_id -> organizations
//   - tenant_id -> tenants
//   - user_id -> users
func InferScopeTable(column string) string {
	if strings.HasSuffix(column, "_id") {
		singular := strings.TrimSuffix(column, "_id")
		return ToPlural(singular)
	}
	return column + "s"
}

// ToPlural converts a singular noun to plural using simple English rules.
// This is a basic implementation that handles common cases.
func ToPlural(singular string) string {
	if singular == "" {
		return ""
	}

	// Handle common irregular plurals
	irregulars := map[string]string{
		"person":   "people",
		"child":    "children",
		"man":      "men",
		"woman":    "women",
		"company":  "companies",
		"category": "categories",
	}
	if plural, ok := irregulars[singular]; ok {
		return plural
	}

	// Handle words ending in 'y' preceded by a consonant
	if strings.HasSuffix(singular, "y") && len(singular) > 1 {
		prev := singular[len(singular)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return singular[:len(singular)-1] + "ies"
		}
	}

	// Handle words ending in 's', 'x', 'z', 'ch', 'sh'
	if strings.HasSuffix(singular, "s") ||
		strings.HasSuffix(singular, "x") ||
		strings.HasSuffix(singular, "z") ||
		strings.HasSuffix(singular, "ch") ||
		strings.HasSuffix(singular, "sh") {
		return singular + "es"
	}

	// Default: just add 's'
	return singular + "s"
}

// ValidateScopeColumn verifies that a configured scope column exists in the table.
// Returns nil if the column exists or if no scope is configured.
func ValidateScopeColumn(table ddl.Table, scopeColumn string) error {
	if scopeColumn == "" {
		return nil // No scope configured, nothing to validate
	}

	for _, col := range table.Columns {
		if col.Name == scopeColumn {
			return nil
		}
	}

	return fmt.Errorf("scope column %q not found in table %q", scopeColumn, table.Name)
}

// FilterScopeForTable returns the scope column to use for a table,
// but only if the table actually has that column.
// This implements the "column presence = opt-in" rule.
func FilterScopeForTable(table ddl.Table, scopeColumn string) string {
	if scopeColumn == "" {
		return ""
	}

	for _, col := range table.Columns {
		if col.Name == scopeColumn {
			return scopeColumn
		}
	}

	// Table doesn't have the scope column, so it's a global/unscoped table
	return ""
}

// ApplyScopeFiltering adjusts TableOpts to only include scope for tables that have the scope column.
// This should be called after loading the config and knowing the actual table schemas.
func ApplyScopeFiltering(cfg *CRUDConfig, tables map[string]ddl.Table) {
	for tableName, opts := range cfg.TableOpts {
		table, exists := tables[tableName]
		if !exists {
			continue
		}

		// Only apply scope if the table has the scope column
		opts.ScopeColumn = FilterScopeForTable(table, opts.ScopeColumn)
		cfg.TableOpts[tableName] = opts
	}
}

// LoadCRUDConfigWithTables loads CRUD configuration from shipq.ini with table filtering applied.
// This is a convenience function that loads the config, applies it to the given tables,
// and filters scope based on column presence.
func LoadCRUDConfigWithTables(projectRoot string, tableNames []string, tables map[string]ddl.Table) (*CRUDConfig, error) {
	shipqIniPath := filepath.Join(projectRoot, "shipq.ini")
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse shipq.ini: %w", err)
	}

	cfg, err := LoadCRUDConfig(ini, tableNames)
	if err != nil {
		return nil, err
	}

	// Apply scope filtering based on actual table schemas
	ApplyScopeFiltering(cfg, tables)

	return cfg, nil
}
