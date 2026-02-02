package migrate

import (
	"sort"
	"strings"
)

// =============================================================================
// Normalized Types
// =============================================================================

// NormalizedColumn represents a database-agnostic column for comparison.
type NormalizedColumn struct {
	Name       string
	BaseType   string // "integer", "bigint", "string", "text", "boolean", "float", "decimal", "datetime", "binary", "json"
	Nullable   bool
	IsPrimary  bool
	HasDefault bool
}

// NormalizedIndex represents a database-agnostic index for comparison.
type NormalizedIndex struct {
	Name    string
	Columns []string
	Unique  bool
}

// NormalizedTable represents a database-agnostic table schema for comparison.
type NormalizedTable struct {
	Name    string
	Columns []NormalizedColumn
	Indexes []NormalizedIndex
}

// =============================================================================
// Base Type Constants
// =============================================================================

const (
	BaseTypeInteger  = "integer"
	BaseTypeBigint   = "bigint"
	BaseTypeString   = "string"
	BaseTypeText     = "text"
	BaseTypeBoolean  = "boolean"
	BaseTypeFloat    = "float"
	BaseTypeDecimal  = "decimal"
	BaseTypeDatetime = "datetime"
	BaseTypeBinary   = "binary"
	BaseTypeJSON     = "json"
	BaseTypeUnknown  = "unknown"
)

// =============================================================================
// PostgreSQL Normalization
// =============================================================================

// NormalizePostgresType converts a PostgreSQL type to a base type.
func NormalizePostgresType(pgType string) string {
	pgType = strings.ToLower(strings.TrimSpace(pgType))

	switch pgType {
	case "integer", "int", "int4", "serial":
		return BaseTypeInteger
	case "bigint", "int8", "bigserial":
		return BaseTypeBigint
	case "smallint", "int2":
		return BaseTypeInteger
	case "character varying", "varchar", "character", "char":
		return BaseTypeString
	case "text":
		return BaseTypeText
	case "boolean", "bool":
		return BaseTypeBoolean
	case "double precision", "float8", "real", "float4":
		return BaseTypeFloat
	case "numeric", "decimal":
		return BaseTypeDecimal
	case "timestamp", "timestamp without time zone", "timestamp with time zone", "timestamptz":
		return BaseTypeDatetime
	case "date", "time", "time without time zone", "time with time zone":
		return BaseTypeDatetime
	case "bytea":
		return BaseTypeBinary
	case "jsonb", "json":
		return BaseTypeJSON
	default:
		// Handle types with precision like "numeric(10,2)"
		if strings.HasPrefix(pgType, "numeric") || strings.HasPrefix(pgType, "decimal") {
			return BaseTypeDecimal
		}
		if strings.HasPrefix(pgType, "character varying") || strings.HasPrefix(pgType, "varchar") {
			return BaseTypeString
		}
		if strings.HasPrefix(pgType, "timestamp") {
			return BaseTypeDatetime
		}
		return BaseTypeUnknown
	}
}

// =============================================================================
// MySQL Normalization
// =============================================================================

// NormalizeMySQLType converts a MySQL type to a base type.
func NormalizeMySQLType(mysqlType string) string {
	mysqlType = strings.ToLower(strings.TrimSpace(mysqlType))

	switch mysqlType {
	case "int", "integer", "mediumint":
		return BaseTypeInteger
	case "bigint":
		return BaseTypeBigint
	case "smallint", "tinyint":
		// tinyint(1) is typically boolean in MySQL
		return BaseTypeInteger
	case "varchar", "char":
		return BaseTypeString
	case "text", "mediumtext", "longtext", "tinytext":
		return BaseTypeText
	case "double", "float", "real":
		return BaseTypeFloat
	case "decimal", "numeric":
		return BaseTypeDecimal
	case "datetime", "timestamp", "date", "time":
		return BaseTypeDatetime
	case "blob", "mediumblob", "longblob", "tinyblob", "binary", "varbinary":
		return BaseTypeBinary
	case "json":
		return BaseTypeJSON
	default:
		return BaseTypeUnknown
	}
}

// =============================================================================
// SQLite Normalization
// =============================================================================

// NormalizeSQLiteType converts a SQLite type to a base type.
func NormalizeSQLiteType(sqliteType string) string {
	sqliteType = strings.ToUpper(strings.TrimSpace(sqliteType))

	switch sqliteType {
	case "INTEGER", "INT":
		return BaseTypeInteger // Note: SQLite uses INTEGER for both int and bigint
	case "TEXT":
		return BaseTypeText // Note: SQLite uses TEXT for strings, text, datetime, and json
	case "REAL":
		return BaseTypeFloat // Note: SQLite uses REAL for both float and decimal
	case "BLOB":
		return BaseTypeBinary
	case "NUMERIC":
		return BaseTypeDecimal
	default:
		// SQLite is flexible with types, try to infer
		if strings.Contains(sqliteType, "INT") {
			return BaseTypeInteger
		}
		if strings.Contains(sqliteType, "CHAR") || strings.Contains(sqliteType, "TEXT") {
			return BaseTypeText
		}
		if strings.Contains(sqliteType, "REAL") || strings.Contains(sqliteType, "FLOAT") || strings.Contains(sqliteType, "DOUBLE") {
			return BaseTypeFloat
		}
		if strings.Contains(sqliteType, "BLOB") {
			return BaseTypeBinary
		}
		return BaseTypeUnknown
	}
}

// =============================================================================
// Comparison Functions
// =============================================================================

// CompareNormalizedColumns compares two columns for equivalence.
// Returns a list of differences (empty if equivalent).
func CompareNormalizedColumns(expected, actual NormalizedColumn) []string {
	var diffs []string

	if expected.Name != actual.Name {
		diffs = append(diffs, "name mismatch: expected "+expected.Name+", got "+actual.Name)
	}

	if !TypesEquivalent(expected.BaseType, actual.BaseType) {
		diffs = append(diffs, "type mismatch for "+expected.Name+": expected "+expected.BaseType+", got "+actual.BaseType)
	}

	if expected.Nullable != actual.Nullable {
		diffs = append(diffs, "nullable mismatch for "+expected.Name+": expected "+boolStr(expected.Nullable)+", got "+boolStr(actual.Nullable))
	}

	if expected.IsPrimary != actual.IsPrimary {
		diffs = append(diffs, "primary key mismatch for "+expected.Name+": expected "+boolStr(expected.IsPrimary)+", got "+boolStr(actual.IsPrimary))
	}

	return diffs
}

// TypesEquivalent checks if two base types are equivalent.
// This handles cases where types map differently across databases.
func TypesEquivalent(type1, type2 string) bool {
	if type1 == type2 {
		return true
	}

	// SQLite maps many types to TEXT or INTEGER
	// integer and bigint both map to INTEGER in SQLite
	if (type1 == BaseTypeInteger && type2 == BaseTypeBigint) ||
		(type1 == BaseTypeBigint && type2 == BaseTypeInteger) {
		return true
	}

	// string and text are often interchangeable
	if (type1 == BaseTypeString && type2 == BaseTypeText) ||
		(type1 == BaseTypeText && type2 == BaseTypeString) {
		return true
	}

	// float and decimal can be equivalent in SQLite (both map to REAL)
	if (type1 == BaseTypeFloat && type2 == BaseTypeDecimal) ||
		(type1 == BaseTypeDecimal && type2 == BaseTypeFloat) {
		return true
	}

	// boolean maps to integer in SQLite and MySQL
	if (type1 == BaseTypeBoolean && type2 == BaseTypeInteger) ||
		(type1 == BaseTypeInteger && type2 == BaseTypeBoolean) {
		return true
	}

	// datetime maps to text in SQLite
	if (type1 == BaseTypeDatetime && type2 == BaseTypeText) ||
		(type1 == BaseTypeText && type2 == BaseTypeDatetime) {
		return true
	}

	// json maps to text in SQLite
	if (type1 == BaseTypeJSON && type2 == BaseTypeText) ||
		(type1 == BaseTypeText && type2 == BaseTypeJSON) {
		return true
	}

	return false
}

// CompareNormalizedTables compares two normalized tables.
// Returns a list of differences (empty if equivalent).
func CompareNormalizedTables(expected, actual NormalizedTable) []string {
	var diffs []string

	if expected.Name != actual.Name {
		diffs = append(diffs, "table name mismatch: expected "+expected.Name+", got "+actual.Name)
	}

	// Compare columns
	expectedCols := make(map[string]NormalizedColumn)
	for _, col := range expected.Columns {
		expectedCols[col.Name] = col
	}

	actualCols := make(map[string]NormalizedColumn)
	for _, col := range actual.Columns {
		actualCols[col.Name] = col
	}

	// Check for missing columns
	for name := range expectedCols {
		if _, ok := actualCols[name]; !ok {
			diffs = append(diffs, "missing column: "+name)
		}
	}

	// Check for extra columns
	for name := range actualCols {
		if _, ok := expectedCols[name]; !ok {
			diffs = append(diffs, "extra column: "+name)
		}
	}

	// Compare matching columns
	for name, expCol := range expectedCols {
		if actCol, ok := actualCols[name]; ok {
			colDiffs := CompareNormalizedColumns(expCol, actCol)
			diffs = append(diffs, colDiffs...)
		}
	}

	// Compare indexes (more lenient - just check for existence and uniqueness)
	expectedIdxs := make(map[string]NormalizedIndex)
	for _, idx := range expected.Indexes {
		key := normalizeIndexKey(idx.Columns, idx.Unique)
		expectedIdxs[key] = idx
	}

	actualIdxs := make(map[string]NormalizedIndex)
	for _, idx := range actual.Indexes {
		key := normalizeIndexKey(idx.Columns, idx.Unique)
		actualIdxs[key] = idx
	}

	for key := range expectedIdxs {
		if _, ok := actualIdxs[key]; !ok {
			diffs = append(diffs, "missing index: "+key)
		}
	}

	return diffs
}

// normalizeIndexKey creates a canonical key for an index (for comparison).
func normalizeIndexKey(columns []string, unique bool) string {
	sorted := make([]string, len(columns))
	copy(sorted, columns)
	sort.Strings(sorted)

	prefix := "idx"
	if unique {
		prefix = "uniq"
	}
	return prefix + ":" + strings.Join(sorted, ",")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// =============================================================================
// DDL to Normalized Conversion
// =============================================================================

// NormalizeDDLType converts a DDL type constant to a base type.
func NormalizeDDLType(ddlType string) string {
	switch ddlType {
	case "integer":
		return BaseTypeInteger
	case "bigint":
		return BaseTypeBigint
	case "string":
		return BaseTypeString
	case "text":
		return BaseTypeText
	case "boolean":
		return BaseTypeBoolean
	case "float":
		return BaseTypeFloat
	case "decimal":
		return BaseTypeDecimal
	case "datetime", "timestamp":
		return BaseTypeDatetime
	case "binary":
		return BaseTypeBinary
	case "json":
		return BaseTypeJSON
	default:
		return BaseTypeUnknown
	}
}

// NormalizeDDLColumn converts a DDL ColumnDefinition to a NormalizedColumn.
func NormalizeDDLColumn(col interface {
	GetName() string
	GetType() string
	IsNullable() bool
	IsPrimaryKey() bool
	GetDefault() string
}) NormalizedColumn {
	return NormalizedColumn{
		Name:       col.GetName(),
		BaseType:   NormalizeDDLType(col.GetType()),
		Nullable:   col.IsNullable(),
		IsPrimary:  col.IsPrimaryKey(),
		HasDefault: col.GetDefault() != "",
	}
}
