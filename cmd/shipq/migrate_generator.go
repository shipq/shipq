package main

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"time"
)

// MigrationConfig holds the configuration for generating a migration.
type MigrationConfig struct {
	PackageName   string       // "migrations"
	MigrationName string       // e.g., "users"
	Timestamp     string       // e.g., "20260111170656"
	Columns       []ColumnSpec // columns to generate
	ScopeColumn   string       // e.g., "organization_id" (empty = no scope)
	ScopeTable    string       // e.g., "organizations"
	IsGlobal      bool         // --global flag skips scope injection
}

// GenerateTimestamp generates a 14-digit UTC timestamp string.
func GenerateTimestamp() string {
	return time.Now().UTC().Format("20060102150405")
}

// GenerateMigration generates the migration file content.
func GenerateMigration(cfg MigrationConfig) ([]byte, error) {
	var buf bytes.Buffer

	// Build the effective column list, potentially with scope column injected
	columns := cfg.Columns
	if cfg.ScopeColumn != "" && !cfg.IsGlobal {
		// Inject scope column at the beginning
		scopeCol := ColumnSpec{
			Name:       cfg.ScopeColumn,
			Type:       "references",
			References: cfg.ScopeTable,
		}
		columns = append([]ColumnSpec{scopeCol}, columns...)
	}

	// Collect unique referenced tables for imports
	referencedTables := collectReferencedTables(columns)

	// Write package declaration
	buf.WriteString(fmt.Sprintf("package %s\n\n", cfg.PackageName))

	// Write imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"github.com/shipq/shipq/db/portsql/ddl\"\n")
	buf.WriteString("\t\"github.com/shipq/shipq/db/portsql/migrate\"\n")
	buf.WriteString(")\n\n")

	// Write function signature
	funcName := fmt.Sprintf("Migrate_%s_%s", cfg.Timestamp, cfg.MigrationName)
	buf.WriteString(fmt.Sprintf("func %s(plan *migrate.MigrationPlan) error {\n", funcName))

	// Write table reference lookups if there are references
	if len(referencedTables) > 0 {
		for _, tableName := range referencedTables {
			refVarName := tableName + "Ref"
			buf.WriteString(fmt.Sprintf("\t%s, err := plan.Table(%q)\n", refVarName, tableName))
			buf.WriteString("\tif err != nil {\n")
			buf.WriteString("\t\treturn err\n")
			buf.WriteString("\t}\n\n")
		}
	}

	// Write AddTable call
	buf.WriteString(fmt.Sprintf("\t_, err := plan.AddTable(%q, func(tb *ddl.TableBuilder) error {\n", cfg.MigrationName))

	// Write column definitions
	for i, col := range columns {
		// Add comment for auto-injected scope column
		if i == 0 && cfg.ScopeColumn != "" && !cfg.IsGlobal && col.Name == cfg.ScopeColumn {
			buf.WriteString(fmt.Sprintf("\t\t// Auto-added: global scope from [db] scope = %s\n", cfg.ScopeColumn))
		}
		colCode := generateColumnCode(col)
		buf.WriteString(fmt.Sprintf("\t\t%s\n", colCode))
	}

	buf.WriteString("\t\treturn nil\n")
	buf.WriteString("\t})\n")
	buf.WriteString("\treturn err\n")
	buf.WriteString("}\n")

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code with error info for debugging
		return buf.Bytes(), fmt.Errorf("failed to format generated code: %w", err)
	}

	return formatted, nil
}

// collectReferencedTables returns a list of unique table names from reference columns.
// The order is preserved (first occurrence order).
func collectReferencedTables(columns []ColumnSpec) []string {
	seen := make(map[string]bool)
	var tables []string

	for _, col := range columns {
		if col.Type == "references" && col.References != "" {
			if !seen[col.References] {
				seen[col.References] = true
				tables = append(tables, col.References)
			}
		}
	}

	return tables
}

// generateColumnCode generates the Go code for a single column definition.
func generateColumnCode(col ColumnSpec) string {
	if col.Type == "references" {
		refVarName := col.References + "Ref"
		return fmt.Sprintf("tb.Bigint(%q).References(%s)", col.Name, refVarName)
	}

	// Map column types to TableBuilder methods
	methodName := columnTypeToMethod(col.Type)
	return fmt.Sprintf("tb.%s(%q)", methodName, col.Name)
}

// columnTypeToMethod maps a column type string to the TableBuilder method name.
func columnTypeToMethod(colType string) string {
	switch colType {
	case "string":
		return "String"
	case "text":
		return "Text"
	case "int":
		return "Integer"
	case "bigint":
		return "Bigint"
	case "bool":
		return "Bool"
	case "float":
		return "Float"
	case "decimal":
		return "Decimal"
	case "datetime":
		return "Datetime"
	case "timestamp":
		return "Timestamp"
	case "binary":
		return "Binary"
	case "json":
		return "JSON"
	default:
		// Capitalize first letter as fallback
		return strings.Title(colType)
	}
}

// GenerateMigrationFileName generates the migration file name.
func GenerateMigrationFileName(timestamp, name string) string {
	return fmt.Sprintf("%s_%s.go", timestamp, name)
}
