package migrate

// mysql_plan.go - MySQL SQL Generation
//
// This file contains all MySQL-specific SQL generation functions.
// See plans/MYSQL_PLAN.adoc for the full implementation plan.

import (
	"fmt"
	"strings"

	"github.com/portsql/portsql/src/ddl"
)

// mysqlType maps DDL types to MySQL types
func mysqlType(col *ddl.ColumnDefinition) string {
	switch col.Type {
	case ddl.IntegerType:
		return "INT"
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
		// MySQL uses TINYINT(1) for booleans
		return "TINYINT(1)"
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
		return "DOUBLE"
	case ddl.DatetimeType:
		return "DATETIME"
	case ddl.TimestampType:
		return "TIMESTAMP"
	case ddl.BinaryType:
		return "BLOB"
	case ddl.JSONType:
		return "JSON"
	default:
		return "TEXT"
	}
}

// escapeMySQLString escapes single quotes in a string for MySQL
func escapeMySQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// formatMySQLDefault formats a default value for MySQL
func formatMySQLDefault(col *ddl.ColumnDefinition) string {
	if col.Default == nil {
		return ""
	}

	defaultVal := *col.Default
	switch col.Type {
	case ddl.BooleanType:
		// MySQL booleans default to 1 or 0
		if defaultVal == "true" {
			return "1"
		}
		return "0"
	case ddl.IntegerType, ddl.BigintType, ddl.FloatType, ddl.DecimalType:
		// Numeric defaults are unquoted
		return defaultVal
	default:
		// String defaults are single-quoted
		return fmt.Sprintf("'%s'", escapeMySQLString(defaultVal))
	}
}

// generateMySQLColumnDef generates a column definition for CREATE TABLE
func generateMySQLColumnDef(col *ddl.ColumnDefinition) string {
	var parts []string

	// Column name (backtick-quoted)
	parts = append(parts, fmt.Sprintf("`%s`", col.Name))

	// Type
	parts = append(parts, mysqlType(col))

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
		parts = append(parts, "DEFAULT", formatMySQLDefault(col))
	}

	return strings.Join(parts, " ")
}

// generateMySQLCreateTable generates a CREATE TABLE statement for MySQL.
func generateMySQLCreateTable(table *ddl.Table) string {
	var sb strings.Builder

	// CREATE TABLE statement
	sb.WriteString(fmt.Sprintf("CREATE TABLE `%s` (", table.Name))

	// Columns
	for i, col := range table.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(generateMySQLColumnDef(&col))
	}

	sb.WriteString(") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4")

	// Generate index statements separately
	var indexStatements []string
	for _, idx := range table.Indexes {
		indexStatements = append(indexStatements, generateMySQLIndexStatement(table.Name, &idx))
	}

	// Combine CREATE TABLE with index statements
	result := sb.String()
	if len(indexStatements) > 0 {
		result += ";\n" + strings.Join(indexStatements, ";\n")
	}

	return result
}

// generateMySQLIndexStatement generates a CREATE INDEX statement for MySQL
func generateMySQLIndexStatement(tableName string, idx *ddl.IndexDefinition) string {
	var sb strings.Builder

	if idx.Unique {
		sb.WriteString("CREATE UNIQUE INDEX ")
	} else {
		sb.WriteString("CREATE INDEX ")
	}

	// Index name (backtick-quoted)
	sb.WriteString(fmt.Sprintf("`%s` ON `%s` (", idx.Name, tableName))

	// Columns
	for i, col := range idx.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("`%s`", col))
	}

	sb.WriteString(")")

	return sb.String()
}

// generateMySQLAlterTable generates ALTER TABLE statements for MySQL.
func generateMySQLAlterTable(tableName string, ops []ddl.TableOperation) string {
	var statements []string

	for _, op := range ops {
		stmt := generateMySQLOperation(tableName, &op)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return strings.Join(statements, ";\n")
}

// generateMySQLOperation generates a single ALTER TABLE operation for MySQL
func generateMySQLOperation(tableName string, op *ddl.TableOperation) string {
	switch op.Type {
	case ddl.OpAddColumn:
		if op.ColumnDef == nil {
			return ""
		}
		return fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s",
			tableName, generateMySQLColumnDef(op.ColumnDef))

	case ddl.OpDropColumn:
		return fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`",
			tableName, op.Column)

	case ddl.OpRenameColumn:
		// MySQL 8.0+ syntax
		return fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`",
			tableName, op.Column, op.NewName)

	case ddl.OpChangeType:
		// MySQL uses MODIFY COLUMN for type changes
		return fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s",
			tableName, op.Column, mysqlTypeFromString(op.NewType))

	case ddl.OpChangeNullable:
		if op.Nullable == nil {
			return ""
		}
		// MySQL uses MODIFY COLUMN for nullability changes
		// Note: This is a simplified version - in production you'd need to know the current type
		if *op.Nullable {
			return fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` TEXT NULL",
				tableName, op.Column)
		}
		return fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` TEXT NOT NULL",
			tableName, op.Column)

	case ddl.OpChangeDefault:
		if op.Default == nil {
			return fmt.Sprintf("ALTER TABLE `%s` ALTER COLUMN `%s` DROP DEFAULT",
				tableName, op.Column)
		}
		// For simplicity, quote all defaults as strings
		return fmt.Sprintf("ALTER TABLE `%s` ALTER COLUMN `%s` SET DEFAULT '%s'",
			tableName, op.Column, escapeMySQLString(*op.Default))

	case ddl.OpAddIndex:
		if op.IndexDef == nil {
			return ""
		}
		return generateMySQLIndexStatement(tableName, op.IndexDef)

	case ddl.OpDropIndex:
		// MySQL DROP INDEX requires ON table_name
		return fmt.Sprintf("DROP INDEX `%s` ON `%s`",
			op.IndexName, tableName)

	case ddl.OpRenameIndex:
		// MySQL 5.7+ supports ALTER TABLE ... RENAME INDEX
		return fmt.Sprintf("ALTER TABLE `%s` RENAME INDEX `%s` TO `%s`",
			tableName, op.IndexName, op.NewName)

	default:
		return ""
	}
}

// mysqlTypeFromString converts a DDL type string to MySQL type
func mysqlTypeFromString(ddlType string) string {
	switch ddlType {
	case ddl.IntegerType:
		return "INT"
	case ddl.BigintType:
		return "BIGINT"
	case ddl.StringType:
		return "VARCHAR(255)"
	case ddl.TextType:
		return "TEXT"
	case ddl.BooleanType:
		return "TINYINT(1)"
	case ddl.DecimalType:
		return "DECIMAL"
	case ddl.FloatType:
		return "DOUBLE"
	case ddl.DatetimeType:
		return "DATETIME"
	case ddl.TimestampType:
		return "TIMESTAMP"
	case ddl.BinaryType:
		return "BLOB"
	case ddl.JSONType:
		return "JSON"
	default:
		return "TEXT"
	}
}

// generateMySQLDropTable generates a DROP TABLE statement for MySQL.
func generateMySQLDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE `%s`", tableName)
}
