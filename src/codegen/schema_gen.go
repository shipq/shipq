package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

// TableAnalysis contains the analyzed structure of a table,
// detecting standard columns and classifying columns for CRUD generation.
type TableAnalysis struct {
	Table         ddl.Table
	HasPublicID   bool
	HasCreatedAt  bool
	HasUpdatedAt  bool
	HasDeletedAt  bool
	PrimaryKey    *ddl.ColumnDefinition
	UserColumns   []ddl.ColumnDefinition // Columns for params (not auto-filled)
	ResultColumns []ddl.ColumnDefinition // Columns for results (not internal id, deleted_at)
}

// AnalyzeTable inspects a table and classifies its columns.
func AnalyzeTable(table ddl.Table) TableAnalysis {
	analysis := TableAnalysis{Table: table}

	for _, col := range table.Columns {
		switch col.Name {
		case "public_id":
			analysis.HasPublicID = true
		case "created_at":
			analysis.HasCreatedAt = true
		case "updated_at":
			analysis.HasUpdatedAt = true
		case "deleted_at":
			analysis.HasDeletedAt = true
		}

		if col.PrimaryKey {
			colCopy := col
			analysis.PrimaryKey = &colCopy
		}
	}

	// UserColumns: exclude auto-filled columns
	// These are columns that users provide values for in Insert/Update params
	for _, col := range table.Columns {
		if col.Name == "id" || col.Name == "public_id" ||
			col.Name == "created_at" || col.Name == "updated_at" ||
			col.Name == "deleted_at" {
			continue
		}
		analysis.UserColumns = append(analysis.UserColumns, col)
	}

	// ResultColumns: exclude internal id and deleted_at
	// These are columns returned in query results
	for _, col := range table.Columns {
		if col.Name == "id" || col.Name == "deleted_at" {
			continue
		}
		analysis.ResultColumns = append(analysis.ResultColumns, col)
	}

	return analysis
}

// TypeMapping contains the mapping from DDL types to Go types and column types.
type TypeMapping struct {
	GoType      string // e.g., "int64", "string", "*string"
	ColumnType  string // e.g., "Int64Column", "NullStringColumn"
	NeedsImport string // e.g., "time" for time.Time
}

// MapColumnType returns the Go type and column type for a DDL column.
func MapColumnType(col ddl.ColumnDefinition) TypeMapping {
	switch col.Type {
	case ddl.IntegerType:
		if col.Nullable {
			return TypeMapping{GoType: "*int32", ColumnType: "NullInt32Column"}
		}
		return TypeMapping{GoType: "int32", ColumnType: "Int32Column"}

	case ddl.BigintType:
		if col.Nullable {
			return TypeMapping{GoType: "*int64", ColumnType: "NullInt64Column"}
		}
		return TypeMapping{GoType: "int64", ColumnType: "Int64Column"}

	case ddl.DecimalType:
		if col.Nullable {
			return TypeMapping{GoType: "*string", ColumnType: "NullDecimalColumn"}
		}
		return TypeMapping{GoType: "string", ColumnType: "DecimalColumn"}

	case ddl.FloatType:
		if col.Nullable {
			return TypeMapping{GoType: "*float64", ColumnType: "NullFloat64Column"}
		}
		return TypeMapping{GoType: "float64", ColumnType: "Float64Column"}

	case ddl.BooleanType:
		if col.Nullable {
			return TypeMapping{GoType: "*bool", ColumnType: "NullBoolColumn"}
		}
		return TypeMapping{GoType: "bool", ColumnType: "BoolColumn"}

	case ddl.StringType, ddl.TextType:
		if col.Nullable {
			return TypeMapping{GoType: "*string", ColumnType: "NullStringColumn"}
		}
		return TypeMapping{GoType: "string", ColumnType: "StringColumn"}

	case ddl.DatetimeType, ddl.TimestampType:
		if col.Nullable {
			return TypeMapping{GoType: "*time.Time", ColumnType: "NullTimeColumn", NeedsImport: "time"}
		}
		return TypeMapping{GoType: "time.Time", ColumnType: "TimeColumn", NeedsImport: "time"}

	case ddl.BinaryType:
		return TypeMapping{GoType: "[]byte", ColumnType: "BytesColumn"}

	case ddl.JSONType:
		if col.Nullable {
			return TypeMapping{GoType: "json.RawMessage", ColumnType: "NullJSONColumn", NeedsImport: "encoding/json"}
		}
		return TypeMapping{GoType: "json.RawMessage", ColumnType: "JSONColumn", NeedsImport: "encoding/json"}

	default:
		// Default to string for unknown types
		if col.Nullable {
			return TypeMapping{GoType: "*string", ColumnType: "NullStringColumn"}
		}
		return TypeMapping{GoType: "string", ColumnType: "StringColumn"}
	}
}

// toPascalCase converts a snake_case string to PascalCase.
// It ensures the result is a valid Go identifier.
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")

	// Filter out empty parts and capitalize
	var result []string
	for _, part := range parts {
		if len(part) > 0 {
			result = append(result, strings.ToUpper(part[:1])+part[1:])
		}
	}

	joined := strings.Join(result, "")

	// Ensure the result starts with a letter (valid Go identifier)
	// If it starts with a digit, prefix with X
	if len(joined) > 0 && joined[0] >= '0' && joined[0] <= '9' {
		joined = "X" + joined
	}

	// If still empty, return a placeholder
	if joined == "" {
		return "X"
	}

	return joined
}

// toSingular attempts to convert a plural table name to singular.
// This is a simple implementation that handles common cases.
func toSingular(s string) string {
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "es") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") {
		return s[:len(s)-1]
	}
	return s
}

// GenerateTableStruct generates Go code for a single table struct.
// It returns the generated code as formatted bytes.
func GenerateTableStruct(table ddl.Table, queryPkgPath string) ([]byte, error) {
	var buf bytes.Buffer

	tableName := table.Name
	structName := toPascalCase(tableName) + "Table"
	varName := toPascalCase(tableName)

	// Collect imports
	imports := make(map[string]bool)
	imports[queryPkgPath] = true

	// Check columns for additional imports (currently not needed for column types)
	// Column types are in the query package

	// Write package and imports
	buf.WriteString("package schema\n\n")
	buf.WriteString("import (\n")
	for imp := range imports {
		buf.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	buf.WriteString(")\n\n")

	// Write struct type
	buf.WriteString(fmt.Sprintf("// %s provides type-safe column references for the %s table.\n", structName, tableName))
	buf.WriteString(fmt.Sprintf("type %s struct{}\n\n", structName))

	// Write global instance
	buf.WriteString(fmt.Sprintf("// %s is the global instance for building queries against the %s table.\n", varName, tableName))
	buf.WriteString(fmt.Sprintf("var %s = %s{}\n\n", varName, structName))

	// Write TableName method
	buf.WriteString(fmt.Sprintf("// TableName returns the SQL table name.\n"))
	buf.WriteString(fmt.Sprintf("func (%s) TableName() string { return %q }\n\n", structName, tableName))

	// Write column accessor methods
	for _, col := range table.Columns {
		mapping := MapColumnType(col)
		methodName := toPascalCase(col.Name)

		buf.WriteString(fmt.Sprintf("// %s returns the %s column.\n", methodName, col.Name))
		buf.WriteString(fmt.Sprintf("func (%s) %s() query.%s {\n", structName, methodName, mapping.ColumnType))
		buf.WriteString(fmt.Sprintf("\treturn query.%s{Table: %q, Name: %q}\n", mapping.ColumnType, tableName, col.Name))
		buf.WriteString("}\n\n")
	}

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("failed to format generated code: %w", err)
	}

	return formatted, nil
}

// GenerateSchemaPackage generates Go code for all tables in a migration plan.
// It produces a single file with all table structs.
func GenerateSchemaPackage(plan *migrate.MigrationPlan, queryPkgPath string) ([]byte, error) {
	var buf bytes.Buffer

	// Collect all imports
	imports := make(map[string]bool)
	imports[queryPkgPath] = true

	// Sort tables for deterministic output
	tableNames := make([]string, 0, len(plan.Schema.Tables))
	for name := range plan.Schema.Tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)

	// Write package and imports
	buf.WriteString("// Code generated by portsql. DO NOT EDIT.\n")
	buf.WriteString("package schema\n\n")
	buf.WriteString("import (\n")
	for imp := range imports {
		buf.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	buf.WriteString(")\n\n")

	// Generate each table
	for _, tableName := range tableNames {
		table := plan.Schema.Tables[tableName]
		structName := toPascalCase(tableName) + "Table"
		varName := toPascalCase(tableName)

		// Write section comment
		buf.WriteString(fmt.Sprintf("// ========== %s ==========\n\n", toPascalCase(tableName)))

		// Write struct type
		buf.WriteString(fmt.Sprintf("// %s provides type-safe column references for the %s table.\n", structName, tableName))
		buf.WriteString(fmt.Sprintf("type %s struct{}\n\n", structName))

		// Write global instance
		buf.WriteString(fmt.Sprintf("// %s is the global instance for building queries against the %s table.\n", varName, tableName))
		buf.WriteString(fmt.Sprintf("var %s = %s{}\n\n", varName, structName))

		// Write TableName method
		buf.WriteString(fmt.Sprintf("// TableName returns the SQL table name.\n"))
		buf.WriteString(fmt.Sprintf("func (%s) TableName() string { return %q }\n\n", structName, tableName))

		// Write column accessor methods
		for _, col := range table.Columns {
			mapping := MapColumnType(col)
			methodName := toPascalCase(col.Name)

			buf.WriteString(fmt.Sprintf("func (%s) %s() query.%s {\n", structName, methodName, mapping.ColumnType))
			buf.WriteString(fmt.Sprintf("\treturn query.%s{Table: %q, Name: %q}\n", mapping.ColumnType, tableName, col.Name))
			buf.WriteString("}\n\n")
		}
	}

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("failed to format generated code: %w", err)
	}

	return formatted, nil
}
