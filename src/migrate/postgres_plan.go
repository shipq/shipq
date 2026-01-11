package migrate

// postgres_plan.go - PostgreSQL SQL Generation
//
// This file contains all PostgreSQL-specific SQL generation functions.
// See plans/POSTGRES_PLAN.adoc for the full implementation plan.

import (
	"fmt"
	"strings"

	"github.com/portsql/portsql/src/ddl"
)

// postgresTypeMap maps DDL types to PostgreSQL types
func postgresType(col *ddl.ColumnDefinition) string {
	switch col.Type {
	case ddl.IntegerType:
		return "INTEGER"
	case ddl.BigintType:
		return "BIGINT"
	case ddl.StringType:
		length := 255
		if col.Length != nil {
			length = *col.Length
		}
		return fmt.Sprintf("VARCHAR(%d)", length)
	case ddl.TextType:
		return "TEXT"
	case ddl.BooleanType:
		return "BOOLEAN"
	case ddl.DecimalType:
		precision := 10
		scale := 0
		if col.Precision != nil {
			precision = *col.Precision
		}
		if col.Scale != nil {
			scale = *col.Scale
		}
		return fmt.Sprintf("DECIMAL(%d, %d)", precision, scale)
	case ddl.FloatType:
		return "DOUBLE PRECISION"
	case ddl.DatetimeType, ddl.TimestampType:
		return "TIMESTAMP WITH TIME ZONE"
	case ddl.BinaryType:
		return "BYTEA"
	case ddl.JSONType:
		return "JSONB"
	default:
		return "TEXT"
	}
}

// escapePostgresString escapes single quotes in a string for PostgreSQL
func escapePostgresString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// formatPostgresDefault formats a default value for PostgreSQL
func formatPostgresDefault(col *ddl.ColumnDefinition) string {
	if col.Default == nil {
		return ""
	}

	defaultVal := *col.Default
	switch col.Type {
	case ddl.BooleanType:
		// Boolean defaults: TRUE or FALSE
		if defaultVal == "true" {
			return "TRUE"
		}
		return "FALSE"
	case ddl.IntegerType, ddl.BigintType, ddl.FloatType, ddl.DecimalType:
		// Numeric defaults are unquoted
		return defaultVal
	default:
		// String defaults are single-quoted
		return fmt.Sprintf("'%s'", escapePostgresString(defaultVal))
	}
}

// generatePostgresColumnDef generates a column definition for CREATE TABLE
func generatePostgresColumnDef(col *ddl.ColumnDefinition) string {
	var parts []string

	// Column name (double-quoted)
	parts = append(parts, fmt.Sprintf(`"%s"`, col.Name))

	// Type
	parts = append(parts, postgresType(col))

	// NOT NULL (only if not nullable and not primary key - PK implies NOT NULL)
	if !col.Nullable && !col.PrimaryKey {
		parts = append(parts, "NOT NULL")
	}

	// PRIMARY KEY
	if col.PrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}

	// DEFAULT
	if col.Default != nil {
		parts = append(parts, "DEFAULT", formatPostgresDefault(col))
	}

	return strings.Join(parts, " ")
}

// generatePostgresCreateTable generates a CREATE TABLE statement for PostgreSQL.
func generatePostgresCreateTable(table *ddl.Table) string {
	var sb strings.Builder

	// CREATE TABLE statement
	sb.WriteString(fmt.Sprintf(`CREATE TABLE "%s" (`, table.Name))

	// Columns
	for i, col := range table.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(generatePostgresColumnDef(&col))
	}

	sb.WriteString(")")

	// Generate index statements separately
	var indexStatements []string
	for _, idx := range table.Indexes {
		indexStatements = append(indexStatements, generatePostgresIndexStatement(table.Name, &idx))
	}

	// Combine CREATE TABLE with index statements
	result := sb.String()
	if len(indexStatements) > 0 {
		result += ";\n" + strings.Join(indexStatements, ";\n")
	}

	return result
}

// generatePostgresIndexStatement generates a CREATE INDEX statement
func generatePostgresIndexStatement(tableName string, idx *ddl.IndexDefinition) string {
	var sb strings.Builder

	if idx.Unique {
		sb.WriteString("CREATE UNIQUE INDEX ")
	} else {
		sb.WriteString("CREATE INDEX ")
	}

	// Index name (double-quoted)
	sb.WriteString(fmt.Sprintf(`"%s" ON "%s" (`, idx.Name, tableName))

	// Columns
	for i, col := range idx.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf(`"%s"`, col))
	}

	sb.WriteString(")")

	return sb.String()
}

// generatePostgresAlterTable generates ALTER TABLE statements for PostgreSQL.
func generatePostgresAlterTable(tableName string, ops []ddl.TableOperation) string {
	var statements []string

	for _, op := range ops {
		stmt := generatePostgresOperation(tableName, &op)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return strings.Join(statements, ";\n")
}

// generatePostgresOperation generates a single ALTER TABLE operation
func generatePostgresOperation(tableName string, op *ddl.TableOperation) string {
	switch op.Type {
	case ddl.OpAddColumn:
		if op.ColumnDef == nil {
			return ""
		}
		return fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s`,
			tableName, generatePostgresColumnDef(op.ColumnDef))

	case ddl.OpDropColumn:
		return fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN "%s"`,
			tableName, op.Column)

	case ddl.OpRenameColumn:
		return fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`,
			tableName, op.Column, op.NewName)

	case ddl.OpChangeType:
		return fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" TYPE %s`,
			tableName, op.Column, postgresTypeFromString(op.NewType))

	case ddl.OpChangeNullable:
		if op.Nullable == nil {
			return ""
		}
		if *op.Nullable {
			return fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" DROP NOT NULL`,
				tableName, op.Column)
		}
		return fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" SET NOT NULL`,
			tableName, op.Column)

	case ddl.OpChangeDefault:
		if op.Default == nil {
			return fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" DROP DEFAULT`,
				tableName, op.Column)
		}
		// For simplicity, quote all defaults as strings (works for most cases)
		// A more sophisticated implementation would check the column type
		return fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" SET DEFAULT '%s'`,
			tableName, op.Column, escapePostgresString(*op.Default))

	case ddl.OpAddIndex:
		if op.IndexDef == nil {
			return ""
		}
		return generatePostgresIndexStatement(tableName, op.IndexDef)

	case ddl.OpDropIndex:
		return fmt.Sprintf(`DROP INDEX "%s"`, op.IndexName)

	case ddl.OpRenameIndex:
		return fmt.Sprintf(`ALTER INDEX "%s" RENAME TO "%s"`,
			op.IndexName, op.NewName)

	default:
		return ""
	}
}

// postgresTypeFromString converts a DDL type string to PostgreSQL type
func postgresTypeFromString(ddlType string) string {
	switch ddlType {
	case ddl.IntegerType:
		return "INTEGER"
	case ddl.BigintType:
		return "BIGINT"
	case ddl.StringType:
		return "VARCHAR(255)"
	case ddl.TextType:
		return "TEXT"
	case ddl.BooleanType:
		return "BOOLEAN"
	case ddl.DecimalType:
		return "DECIMAL"
	case ddl.FloatType:
		return "DOUBLE PRECISION"
	case ddl.DatetimeType, ddl.TimestampType:
		return "TIMESTAMP WITH TIME ZONE"
	case ddl.BinaryType:
		return "BYTEA"
	case ddl.JSONType:
		return "JSONB"
	default:
		return "TEXT"
	}
}

// generatePostgresDropTable generates a DROP TABLE statement for PostgreSQL.
func generatePostgresDropTable(tableName string) string {
	return fmt.Sprintf(`DROP TABLE "%s"`, tableName)
}
