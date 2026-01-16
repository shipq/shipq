//go:build integration

package migrate

// =============================================================================
// Database-Specific Column Normalization
//
// These functions convert database-specific column info to normalized columns.
// They're in an integration-only file because they depend on the column info
// types that are only available in integration tests.
// =============================================================================

// NormalizePostgresColumn converts a PostgreSQL ColumnInfo to a NormalizedColumn.
func NormalizePostgresColumn(col ColumnInfo, isPrimary bool) NormalizedColumn {
	return NormalizedColumn{
		Name:       col.Name,
		BaseType:   NormalizePostgresType(col.DataType),
		Nullable:   col.IsNullable,
		IsPrimary:  isPrimary,
		HasDefault: col.Default != nil && *col.Default != "",
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

	return NormalizedColumn{
		Name:       col.Name,
		BaseType:   baseType,
		Nullable:   col.IsNullable,
		IsPrimary:  isPrimary,
		HasDefault: col.Default != nil,
	}
}

// NormalizeSQLiteColumn converts a SQLiteColumnInfo to a NormalizedColumn.
func NormalizeSQLiteColumn(col SQLiteColumnInfo) NormalizedColumn {
	// SQLite quirk: PRAGMA table_info reports notnull=false for PRIMARY KEY columns,
	// but they're effectively NOT NULL. Treat PK columns as not nullable.
	isPrimary := col.PK > 0
	isNullable := !col.NotNull && !isPrimary

	return NormalizedColumn{
		Name:       col.Name,
		BaseType:   NormalizeSQLiteType(col.Type),
		Nullable:   isNullable,
		IsPrimary:  isPrimary,
		HasDefault: col.DefaultVal != nil,
	}
}
