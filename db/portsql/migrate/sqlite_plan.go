package migrate

// sqlite_plan.go - SQLite SQL Generation
//
// This file contains all SQLite-specific SQL generation functions.
// See plans/SQLITE_PLAN.adoc for the full implementation plan.

import (
	"fmt"
	"strings"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// sqliteType maps DDL types to SQLite types
func sqliteType(col *ddl.ColumnDefinition) string {
	switch col.Type {
	case ddl.IntegerType, ddl.BigintType:
		// SQLite INTEGER is always 64-bit
		return "INTEGER"
	case ddl.StringType:
		// SQLite doesn't have VARCHAR, use TEXT
		return "TEXT"
	case ddl.TextType:
		return "TEXT"
	case ddl.BooleanType:
		// SQLite uses INTEGER for booleans (0=false, 1=true)
		return "INTEGER"
	case ddl.DecimalType:
		// SQLite uses REAL for decimals
		return "REAL"
	case ddl.FloatType:
		return "REAL"
	case ddl.DatetimeType, ddl.TimestampType:
		// SQLite stores datetime as TEXT (ISO8601 format)
		return "TEXT"
	case ddl.BinaryType:
		return "BLOB"
	case ddl.JSONType:
		// SQLite stores JSON as TEXT
		return "TEXT"
	default:
		return "TEXT"
	}
}

// escapeSQLiteString escapes single quotes in a string for SQLite
func escapeSQLiteString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// formatSQLiteDefault formats a default value for SQLite
func formatSQLiteDefault(col *ddl.ColumnDefinition) string {
	if col.Default == nil {
		return ""
	}

	defaultVal := *col.Default
	switch col.Type {
	case ddl.BooleanType:
		// SQLite booleans default to 1 or 0
		if defaultVal == "true" {
			return "1"
		}
		return "0"
	case ddl.IntegerType, ddl.BigintType, ddl.FloatType, ddl.DecimalType:
		// Numeric defaults are unquoted
		return defaultVal
	default:
		// String defaults are single-quoted
		return fmt.Sprintf("'%s'", escapeSQLiteString(defaultVal))
	}
}

// generateSQLiteColumnDef generates a column definition for CREATE TABLE.
// isAutoincrementPK should be true if this column is the autoincrement-eligible primary key.
func generateSQLiteColumnDef(col *ddl.ColumnDefinition, isAutoincrementPK bool) string {
	var parts []string

	// Column name (double-quoted like PostgreSQL)
	parts = append(parts, fmt.Sprintf(`"%s"`, col.Name))

	// Type
	// For autoincrement PK, SQLite requires exactly "INTEGER" (not BIGINT) for rowid aliasing
	if isAutoincrementPK {
		parts = append(parts, "INTEGER")
	} else {
		parts = append(parts, sqliteType(col))
	}

	// NOT NULL (only if not nullable and not primary key - PK implies NOT NULL)
	// For autoincrement PK, skip NOT NULL as it's implied and can interfere with rowid semantics
	if !isAutoincrementPK && !col.Nullable && !col.PrimaryKey {
		parts = append(parts, "NOT NULL")
	}

	// PRIMARY KEY
	if col.PrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}

	// DEFAULT (skip for autoincrement PK - rowid is the source of truth)
	if col.Default != nil && !isAutoincrementPK {
		parts = append(parts, "DEFAULT", formatSQLiteDefault(col))
	}

	return strings.Join(parts, " ")
}

// generateSQLiteCreateTable generates a CREATE TABLE statement for SQLite.
func generateSQLiteCreateTable(table *ddl.Table) string {
	var sb strings.Builder

	// Check for autoincrement-eligible PK
	pkInfo, hasAutoincrementPK := GetAutoincrementPK(table)

	// CREATE TABLE statement
	sb.WriteString(fmt.Sprintf(`CREATE TABLE "%s" (`, table.Name))

	// Columns
	for i, col := range table.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		// Determine if this column is the autoincrement PK
		isAutoincrementPK := hasAutoincrementPK && col.Name == pkInfo.ColumnName
		sb.WriteString(generateSQLiteColumnDef(&col, isAutoincrementPK))
	}

	sb.WriteString(")")

	// Generate index statements separately
	var indexStatements []string
	for _, idx := range table.Indexes {
		indexStatements = append(indexStatements, generateSQLiteIndexStatement(table.Name, &idx))
	}

	// Combine CREATE TABLE with index statements
	result := sb.String()
	if len(indexStatements) > 0 {
		result += ";\n" + strings.Join(indexStatements, ";\n")
	}

	return result
}

// generateSQLiteIndexStatement generates a CREATE INDEX statement for SQLite
func generateSQLiteIndexStatement(tableName string, idx *ddl.IndexDefinition) string {
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

// generateSQLiteAlterTable generates ALTER TABLE statements for SQLite.
// Note: currentTable is needed for table rebuild operations (change type, set not null, etc.)
// For now, we implement supported operations; unsupported ops will generate comments.
func generateSQLiteAlterTable(tableName string, ops []ddl.TableOperation, currentTable *ddl.Table) string {
	var statements []string

	for _, op := range ops {
		stmt := generateSQLiteOperation(tableName, &op)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return strings.Join(statements, ";\n")
}

// generateSQLiteOperation generates a single ALTER TABLE operation for SQLite
func generateSQLiteOperation(tableName string, op *ddl.TableOperation) string {
	switch op.Type {
	case ddl.OpAddColumn:
		if op.ColumnDef == nil {
			return ""
		}
		// ALTER TABLE ADD COLUMN does not support autoincrement
		// (that would require table rebuild)
		return fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s`,
			tableName, generateSQLiteColumnDef(op.ColumnDef, false))

	case ddl.OpDropColumn:
		// SQLite 3.35.0+ supports DROP COLUMN
		return fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN "%s"`,
			tableName, op.Column)

	case ddl.OpRenameColumn:
		// SQLite 3.25.0+ syntax
		return fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`,
			tableName, op.Column, op.NewName)

	case ddl.OpChangeType:
		// SQLite doesn't support changing column type - requires table rebuild
		return fmt.Sprintf(`-- SQLite does not support ALTER COLUMN TYPE; table rebuild required for "%s"`,
			op.Column)

	case ddl.OpChangeNullable:
		// SQLite doesn't support changing nullability - requires table rebuild
		return fmt.Sprintf(`-- SQLite does not support changing NULL constraint; table rebuild required for "%s"`,
			op.Column)

	case ddl.OpChangeDefault:
		// SQLite doesn't support changing default on existing columns - requires table rebuild
		return fmt.Sprintf(`-- SQLite does not support changing DEFAULT; table rebuild required for "%s"`,
			op.Column)

	case ddl.OpAddIndex:
		if op.IndexDef == nil {
			return ""
		}
		return generateSQLiteIndexStatement(tableName, op.IndexDef)

	case ddl.OpDropIndex:
		// SQLite DROP INDEX doesn't need ON clause (like PostgreSQL)
		return fmt.Sprintf(`DROP INDEX "%s"`, op.IndexName)

	case ddl.OpRenameIndex:
		// SQLite doesn't support renaming indexes directly
		// Would need to drop and recreate
		return fmt.Sprintf(`-- SQLite does not support RENAME INDEX; drop and recreate required for "%s"`,
			op.IndexName)

	default:
		return ""
	}
}

// sqliteTypeFromString converts a DDL type string to SQLite type
func sqliteTypeFromString(ddlType string) string {
	switch ddlType {
	case ddl.IntegerType, ddl.BigintType:
		return "INTEGER"
	case ddl.StringType, ddl.TextType:
		return "TEXT"
	case ddl.BooleanType:
		return "INTEGER"
	case ddl.DecimalType, ddl.FloatType:
		return "REAL"
	case ddl.DatetimeType, ddl.TimestampType:
		return "TEXT"
	case ddl.BinaryType:
		return "BLOB"
	case ddl.JSONType:
		return "TEXT"
	default:
		return "TEXT"
	}
}

// generateSQLiteDropTable generates a DROP TABLE statement for SQLite.
func generateSQLiteDropTable(tableName string) string {
	return fmt.Sprintf(`DROP TABLE "%s"`, tableName)
}

// requiresTableRebuild checks if any operation requires a SQLite table rebuild.
// Returns true for: OpChangeType, OpChangeNullable, OpChangeDefault (on existing columns)
func requiresTableRebuild(ops []ddl.TableOperation) bool {
	for _, op := range ops {
		switch op.Type {
		case ddl.OpChangeType, ddl.OpChangeNullable, ddl.OpChangeDefault:
			return true
		}
	}
	return false
}

// generateSQLiteTableRebuild generates the multi-statement SQL for a table rebuild.
// This is needed for operations SQLite doesn't support natively.
// Returns a multi-statement SQL string that:
// 1. Creates a new table with the desired schema
// 2. Copies data from the old table
// 3. Drops the old table
// 4. Renames the new table
// 5. Recreates indexes
func generateSQLiteTableRebuild(tableName string, currentTable *ddl.Table, ops []ddl.TableOperation) string {
	if currentTable == nil {
		return "-- Cannot rebuild table: current table definition required"
	}

	// Apply operations to get the new table definition
	newTable := applyOperationsToTable(currentTable, ops)
	newTableName := tableName + "_new"

	var sb strings.Builder

	// Begin transaction for safety
	sb.WriteString("PRAGMA foreign_keys=OFF;\n")
	sb.WriteString("BEGIN TRANSACTION;\n")

	// Check for autoincrement-eligible PK in the new table
	pkInfo, hasAutoincrementPK := GetAutoincrementPK(newTable)

	// 1. Create new table with desired schema
	sb.WriteString(fmt.Sprintf(`CREATE TABLE "%s" (`, newTableName))
	for i, col := range newTable.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		// Determine if this column is the autoincrement PK
		isAutoincrementPK := hasAutoincrementPK && col.Name == pkInfo.ColumnName
		sb.WriteString(generateSQLiteColumnDef(&col, isAutoincrementPK))
	}
	sb.WriteString(");\n")

	// 2. Copy data from old table
	// Build column list for both tables
	var oldCols, newCols []string
	for _, newCol := range newTable.Columns {
		// Check if column exists in old table
		for _, oldCol := range currentTable.Columns {
			if oldCol.Name == newCol.Name {
				oldCols = append(oldCols, fmt.Sprintf(`"%s"`, oldCol.Name))
				newCols = append(newCols, fmt.Sprintf(`"%s"`, newCol.Name))
				break
			}
		}
	}

	if len(oldCols) > 0 {
		sb.WriteString(fmt.Sprintf(`INSERT INTO "%s" (%s) SELECT %s FROM "%s"`,
			newTableName,
			strings.Join(newCols, ", "),
			strings.Join(oldCols, ", "),
			tableName))
		sb.WriteString(";\n")
	}

	// 3. Drop old table
	sb.WriteString(fmt.Sprintf(`DROP TABLE "%s"`, tableName))
	sb.WriteString(";\n")

	// 4. Rename new table
	sb.WriteString(fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, newTableName, tableName))
	sb.WriteString(";\n")

	// 5. Recreate indexes
	for _, idx := range newTable.Indexes {
		sb.WriteString(generateSQLiteIndexStatement(tableName, &idx))
		sb.WriteString(";\n")
	}

	// Commit transaction
	sb.WriteString("COMMIT;\n")
	sb.WriteString("PRAGMA foreign_keys=ON;")

	return sb.String()
}

// applyOperationsToTable creates a copy of the table with operations applied
func applyOperationsToTable(table *ddl.Table, ops []ddl.TableOperation) *ddl.Table {
	// Create a deep copy of the table
	newTable := &ddl.Table{
		Name:    table.Name,
		Columns: make([]ddl.ColumnDefinition, len(table.Columns)),
		Indexes: make([]ddl.IndexDefinition, len(table.Indexes)),
	}
	copy(newTable.Columns, table.Columns)
	copy(newTable.Indexes, table.Indexes)

	// Apply each operation
	for _, op := range ops {
		switch op.Type {
		case ddl.OpAddColumn:
			if op.ColumnDef != nil {
				newTable.Columns = append(newTable.Columns, *op.ColumnDef)
			}
		case ddl.OpDropColumn:
			newCols := make([]ddl.ColumnDefinition, 0, len(newTable.Columns))
			for _, col := range newTable.Columns {
				if col.Name != op.Column {
					newCols = append(newCols, col)
				}
			}
			newTable.Columns = newCols
		case ddl.OpRenameColumn:
			for i, col := range newTable.Columns {
				if col.Name == op.Column {
					newTable.Columns[i].Name = op.NewName
					break
				}
			}
		case ddl.OpChangeType:
			for i, col := range newTable.Columns {
				if col.Name == op.Column {
					newTable.Columns[i].Type = op.NewType
					break
				}
			}
		case ddl.OpChangeNullable:
			if op.Nullable != nil {
				for i, col := range newTable.Columns {
					if col.Name == op.Column {
						newTable.Columns[i].Nullable = *op.Nullable
						break
					}
				}
			}
		case ddl.OpChangeDefault:
			for i, col := range newTable.Columns {
				if col.Name == op.Column {
					newTable.Columns[i].Default = op.Default
					break
				}
			}
		case ddl.OpAddIndex:
			if op.IndexDef != nil {
				newTable.Indexes = append(newTable.Indexes, *op.IndexDef)
			}
		case ddl.OpDropIndex:
			newIdxs := make([]ddl.IndexDefinition, 0, len(newTable.Indexes))
			for _, idx := range newTable.Indexes {
				if idx.Name != op.IndexName {
					newIdxs = append(newIdxs, idx)
				}
			}
			newTable.Indexes = newIdxs
		case ddl.OpRenameIndex:
			for i, idx := range newTable.Indexes {
				if idx.Name == op.IndexName {
					newTable.Indexes[i].Name = op.NewName
					break
				}
			}
		}
	}

	return newTable
}
