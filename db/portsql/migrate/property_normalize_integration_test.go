//go:build integration

package migrate

import "strings"

// =============================================================================
// Database-Specific Column Normalization
//
// These functions convert database-specific column info to normalized columns.
// They're in an integration-only file because they depend on the column info
// types that are only available in integration tests.
// =============================================================================

// NormalizePostgresColumn converts a PostgreSQL ColumnInfo to a NormalizedColumn.
func NormalizePostgresColumn(col ColumnInfo, isPrimary bool) NormalizedColumn {
	baseType := NormalizePostgresType(col.DataType)

	// Detect autoincrement: Postgres identity columns have "generated" in identity_generation
	// or the default contains nextval() for serial types
	isAutoIncrement := false
	if isPrimary && (baseType == BaseTypeInteger || baseType == BaseTypeBigint) {
		// Check for identity column or serial sequence
		if col.Default != nil && (strings.Contains(*col.Default, "nextval") || strings.Contains(*col.Default, "identity")) {
			isAutoIncrement = true
		}
		// For identity columns, the column might not have a default but be marked as identity
		// We'll treat any integer/bigint PK as potentially autoincrement for comparison purposes
		isAutoIncrement = true
	}

	return NormalizedColumn{
		Name:              col.Name,
		BaseType:          baseType,
		Nullable:          col.IsNullable,
		IsPrimary:         isPrimary,
		HasDefault:        col.Default != nil && *col.Default != "",
		IsAutoIncrementPK: isAutoIncrement,
	}
}

// NormalizeMySQLColumn converts a MySQLColumnInfo to a NormalizedColumn.
func NormalizeMySQLColumn(col MySQLColumnInfo, isPrimary bool) NormalizedColumn {
	baseType := NormalizeMySQLType(col.DataType)

	// Special handling for tinyint(1) as boolean
	if col.DataType == "tinyint" {
		// Check if this looks like a boolean based on defaults
		if col.Default != nil && (*col.Default == "0" || *col.Default == "1") {
			baseType = BaseTypeBoolean
		}
	}

	// Detect autoincrement: MySQL has Extra field with "auto_increment"
	isAutoIncrement := false
	if isPrimary && (baseType == BaseTypeInteger || baseType == BaseTypeBigint) {
		if col.Extra != nil && strings.Contains(strings.ToLower(*col.Extra), "auto_increment") {
			isAutoIncrement = true
		}
	}

	return NormalizedColumn{
		Name:              col.Name,
		BaseType:          baseType,
		Nullable:          col.IsNullable,
		IsPrimary:         isPrimary,
		HasDefault:        col.Default != nil,
		IsAutoIncrementPK: isAutoIncrement,
	}
}

// NormalizeSQLiteColumn converts a SQLiteColumnInfo to a NormalizedColumn.
func NormalizeSQLiteColumn(col SQLiteColumnInfo) NormalizedColumn {
	// SQLite quirk: PRAGMA table_info reports notnull=false for PRIMARY KEY columns,
	// but they're effectively NOT NULL. Treat PK columns as not nullable.
	isPrimary := col.PK > 0
	isNullable := !col.NotNull && !isPrimary
	baseType := NormalizeSQLiteType(col.Type)

	// Detect autoincrement: SQLite INTEGER PRIMARY KEY is the rowid alias
	// which provides autoincrement semantics
	isAutoIncrement := false
	if isPrimary && baseType == BaseTypeInteger {
		// SQLite INTEGER PRIMARY KEY is always autoincrement (rowid alias)
		isAutoIncrement = true
	}

	return NormalizedColumn{
		Name:              col.Name,
		BaseType:          baseType,
		Nullable:          isNullable,
		IsPrimary:         isPrimary,
		HasDefault:        col.DefaultVal != nil,
		IsAutoIncrementPK: isAutoIncrement,
	}
}
