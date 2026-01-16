package migrate

import (
	"testing"

	"github.com/shipq/shipq/db/proptest"
)

// =============================================================================
// PostgreSQL Type Normalization Tests
// =============================================================================

func TestNormalizePostgresType_Integer(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"integer", BaseTypeInteger},
		{"int", BaseTypeInteger},
		{"int4", BaseTypeInteger},
		{"serial", BaseTypeInteger},
		{"INTEGER", BaseTypeInteger},
		{"smallint", BaseTypeInteger},
		{"int2", BaseTypeInteger},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizePostgresType(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestNormalizePostgresType_Bigint(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"bigint", BaseTypeBigint},
		{"int8", BaseTypeBigint},
		{"bigserial", BaseTypeBigint},
		{"BIGINT", BaseTypeBigint},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizePostgresType(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestNormalizePostgresType_String(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"character varying", BaseTypeString},
		{"varchar", BaseTypeString},
		{"character", BaseTypeString},
		{"char", BaseTypeString},
		{"CHARACTER VARYING", BaseTypeString},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizePostgresType(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestNormalizePostgresType_Text(t *testing.T) {
	result := NormalizePostgresType("text")
	if result != BaseTypeText {
		t.Errorf("NormalizePostgresType(\"text\") = %q, want %q", result, BaseTypeText)
	}
}

func TestNormalizePostgresType_Boolean(t *testing.T) {
	testCases := []string{"boolean", "bool", "BOOLEAN"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizePostgresType(tc)
			if result != BaseTypeBoolean {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc, result, BaseTypeBoolean)
			}
		})
	}
}

func TestNormalizePostgresType_Float(t *testing.T) {
	testCases := []string{"double precision", "float8", "real", "float4"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizePostgresType(tc)
			if result != BaseTypeFloat {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc, result, BaseTypeFloat)
			}
		})
	}
}

func TestNormalizePostgresType_Decimal(t *testing.T) {
	testCases := []string{"numeric", "decimal"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizePostgresType(tc)
			if result != BaseTypeDecimal {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc, result, BaseTypeDecimal)
			}
		})
	}
}

func TestNormalizePostgresType_Datetime(t *testing.T) {
	testCases := []string{
		"timestamp",
		"timestamp without time zone",
		"timestamp with time zone",
		"timestamptz",
		"date",
		"time",
	}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizePostgresType(tc)
			if result != BaseTypeDatetime {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc, result, BaseTypeDatetime)
			}
		})
	}
}

func TestNormalizePostgresType_Binary(t *testing.T) {
	result := NormalizePostgresType("bytea")
	if result != BaseTypeBinary {
		t.Errorf("NormalizePostgresType(\"bytea\") = %q, want %q", result, BaseTypeBinary)
	}
}

func TestNormalizePostgresType_JSON(t *testing.T) {
	testCases := []string{"json", "jsonb"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizePostgresType(tc)
			if result != BaseTypeJSON {
				t.Errorf("NormalizePostgresType(%q) = %q, want %q", tc, result, BaseTypeJSON)
			}
		})
	}
}

// =============================================================================
// MySQL Type Normalization Tests
// =============================================================================

func TestNormalizeMySQLType_Integer(t *testing.T) {
	testCases := []string{"int", "integer", "mediumint", "smallint", "tinyint"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeInteger {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeInteger)
			}
		})
	}
}

func TestNormalizeMySQLType_Bigint(t *testing.T) {
	result := NormalizeMySQLType("bigint")
	if result != BaseTypeBigint {
		t.Errorf("NormalizeMySQLType(\"bigint\") = %q, want %q", result, BaseTypeBigint)
	}
}

func TestNormalizeMySQLType_String(t *testing.T) {
	testCases := []string{"varchar", "char"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeString {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeString)
			}
		})
	}
}

func TestNormalizeMySQLType_Text(t *testing.T) {
	testCases := []string{"text", "mediumtext", "longtext", "tinytext"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeText {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeText)
			}
		})
	}
}

func TestNormalizeMySQLType_Float(t *testing.T) {
	testCases := []string{"double", "float", "real"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeFloat {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeFloat)
			}
		})
	}
}

func TestNormalizeMySQLType_Decimal(t *testing.T) {
	testCases := []string{"decimal", "numeric"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeDecimal {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeDecimal)
			}
		})
	}
}

func TestNormalizeMySQLType_Datetime(t *testing.T) {
	testCases := []string{"datetime", "timestamp", "date", "time"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeDatetime {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeDatetime)
			}
		})
	}
}

func TestNormalizeMySQLType_Binary(t *testing.T) {
	testCases := []string{"blob", "mediumblob", "longblob", "tinyblob", "binary", "varbinary"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeMySQLType(tc)
			if result != BaseTypeBinary {
				t.Errorf("NormalizeMySQLType(%q) = %q, want %q", tc, result, BaseTypeBinary)
			}
		})
	}
}

func TestNormalizeMySQLType_JSON(t *testing.T) {
	result := NormalizeMySQLType("json")
	if result != BaseTypeJSON {
		t.Errorf("NormalizeMySQLType(\"json\") = %q, want %q", result, BaseTypeJSON)
	}
}

// =============================================================================
// SQLite Type Normalization Tests
// =============================================================================

func TestNormalizeSQLiteType_Integer(t *testing.T) {
	testCases := []string{"INTEGER", "INT", "integer", "int"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeSQLiteType(tc)
			if result != BaseTypeInteger {
				t.Errorf("NormalizeSQLiteType(%q) = %q, want %q", tc, result, BaseTypeInteger)
			}
		})
	}
}

func TestNormalizeSQLiteType_Text(t *testing.T) {
	testCases := []string{"TEXT", "text"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeSQLiteType(tc)
			if result != BaseTypeText {
				t.Errorf("NormalizeSQLiteType(%q) = %q, want %q", tc, result, BaseTypeText)
			}
		})
	}
}

func TestNormalizeSQLiteType_Real(t *testing.T) {
	testCases := []string{"REAL", "real"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeSQLiteType(tc)
			if result != BaseTypeFloat {
				t.Errorf("NormalizeSQLiteType(%q) = %q, want %q", tc, result, BaseTypeFloat)
			}
		})
	}
}

func TestNormalizeSQLiteType_Blob(t *testing.T) {
	testCases := []string{"BLOB", "blob"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := NormalizeSQLiteType(tc)
			if result != BaseTypeBinary {
				t.Errorf("NormalizeSQLiteType(%q) = %q, want %q", tc, result, BaseTypeBinary)
			}
		})
	}
}

// =============================================================================
// Type Equivalence Tests
// =============================================================================

func TestTypesEquivalent_SameTypes(t *testing.T) {
	types := []string{
		BaseTypeInteger, BaseTypeBigint, BaseTypeString, BaseTypeText,
		BaseTypeBoolean, BaseTypeFloat, BaseTypeDecimal, BaseTypeDatetime,
		BaseTypeBinary, BaseTypeJSON,
	}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			if !TypesEquivalent(typ, typ) {
				t.Errorf("TypesEquivalent(%q, %q) = false, want true", typ, typ)
			}
		})
	}
}

func TestTypesEquivalent_IntegerBigint(t *testing.T) {
	// SQLite doesn't distinguish between integer and bigint
	if !TypesEquivalent(BaseTypeInteger, BaseTypeBigint) {
		t.Error("TypesEquivalent(integer, bigint) = false, want true")
	}
	if !TypesEquivalent(BaseTypeBigint, BaseTypeInteger) {
		t.Error("TypesEquivalent(bigint, integer) = false, want true")
	}
}

func TestTypesEquivalent_StringText(t *testing.T) {
	if !TypesEquivalent(BaseTypeString, BaseTypeText) {
		t.Error("TypesEquivalent(string, text) = false, want true")
	}
	if !TypesEquivalent(BaseTypeText, BaseTypeString) {
		t.Error("TypesEquivalent(text, string) = false, want true")
	}
}

func TestTypesEquivalent_FloatDecimal(t *testing.T) {
	// SQLite maps both to REAL
	if !TypesEquivalent(BaseTypeFloat, BaseTypeDecimal) {
		t.Error("TypesEquivalent(float, decimal) = false, want true")
	}
	if !TypesEquivalent(BaseTypeDecimal, BaseTypeFloat) {
		t.Error("TypesEquivalent(decimal, float) = false, want true")
	}
}

func TestTypesEquivalent_BooleanInteger(t *testing.T) {
	// SQLite and MySQL store booleans as integers
	if !TypesEquivalent(BaseTypeBoolean, BaseTypeInteger) {
		t.Error("TypesEquivalent(boolean, integer) = false, want true")
	}
	if !TypesEquivalent(BaseTypeInteger, BaseTypeBoolean) {
		t.Error("TypesEquivalent(integer, boolean) = false, want true")
	}
}

func TestTypesEquivalent_DatetimeText(t *testing.T) {
	// SQLite stores datetime as TEXT
	if !TypesEquivalent(BaseTypeDatetime, BaseTypeText) {
		t.Error("TypesEquivalent(datetime, text) = false, want true")
	}
}

func TestTypesEquivalent_JSONText(t *testing.T) {
	// SQLite stores JSON as TEXT
	if !TypesEquivalent(BaseTypeJSON, BaseTypeText) {
		t.Error("TypesEquivalent(json, text) = false, want true")
	}
}

func TestTypesEquivalent_NonEquivalent(t *testing.T) {
	// These should NOT be equivalent
	testCases := []struct {
		type1 string
		type2 string
	}{
		{BaseTypeInteger, BaseTypeBinary},
		{BaseTypeString, BaseTypeBinary},
		{BaseTypeBoolean, BaseTypeBinary},
		{BaseTypeFloat, BaseTypeBinary},
	}

	for _, tc := range testCases {
		t.Run(tc.type1+"_"+tc.type2, func(t *testing.T) {
			if TypesEquivalent(tc.type1, tc.type2) {
				t.Errorf("TypesEquivalent(%q, %q) = true, want false", tc.type1, tc.type2)
			}
		})
	}
}

// =============================================================================
// Column Comparison Tests
// =============================================================================

func TestCompareNormalizedColumns_Identical(t *testing.T) {
	col1 := NormalizedColumn{
		Name:       "test_col",
		BaseType:   BaseTypeInteger,
		Nullable:   false,
		IsPrimary:  true,
		HasDefault: false,
	}
	col2 := col1

	diffs := CompareNormalizedColumns(col1, col2)
	if len(diffs) > 0 {
		t.Errorf("CompareNormalizedColumns returned diffs for identical columns: %v", diffs)
	}
}

func TestCompareNormalizedColumns_TypeMismatch(t *testing.T) {
	col1 := NormalizedColumn{Name: "test_col", BaseType: BaseTypeInteger}
	col2 := NormalizedColumn{Name: "test_col", BaseType: BaseTypeBinary}

	diffs := CompareNormalizedColumns(col1, col2)
	if len(diffs) == 0 {
		t.Error("CompareNormalizedColumns should have reported type mismatch")
	}
}

func TestCompareNormalizedColumns_NullableMismatch(t *testing.T) {
	col1 := NormalizedColumn{Name: "test_col", BaseType: BaseTypeInteger, Nullable: true}
	col2 := NormalizedColumn{Name: "test_col", BaseType: BaseTypeInteger, Nullable: false}

	diffs := CompareNormalizedColumns(col1, col2)
	if len(diffs) == 0 {
		t.Error("CompareNormalizedColumns should have reported nullable mismatch")
	}
}

func TestCompareNormalizedColumns_EquivalentTypes(t *testing.T) {
	// integer and bigint should be considered equivalent
	col1 := NormalizedColumn{Name: "test_col", BaseType: BaseTypeInteger}
	col2 := NormalizedColumn{Name: "test_col", BaseType: BaseTypeBigint}

	diffs := CompareNormalizedColumns(col1, col2)
	if len(diffs) > 0 {
		t.Errorf("CompareNormalizedColumns should not report diff for equivalent types: %v", diffs)
	}
}

// =============================================================================
// Table Comparison Tests
// =============================================================================

func TestCompareNormalizedTables_Identical(t *testing.T) {
	table1 := NormalizedTable{
		Name: "test_table",
		Columns: []NormalizedColumn{
			{Name: "id", BaseType: BaseTypeInteger, IsPrimary: true},
			{Name: "name", BaseType: BaseTypeString},
		},
		Indexes: []NormalizedIndex{
			{Name: "idx_name", Columns: []string{"name"}, Unique: false},
		},
	}
	table2 := table1

	diffs := CompareNormalizedTables(table1, table2)
	if len(diffs) > 0 {
		t.Errorf("CompareNormalizedTables returned diffs for identical tables: %v", diffs)
	}
}

func TestCompareNormalizedTables_MissingColumn(t *testing.T) {
	table1 := NormalizedTable{
		Name: "test_table",
		Columns: []NormalizedColumn{
			{Name: "id", BaseType: BaseTypeInteger},
			{Name: "name", BaseType: BaseTypeString},
		},
	}
	table2 := NormalizedTable{
		Name: "test_table",
		Columns: []NormalizedColumn{
			{Name: "id", BaseType: BaseTypeInteger},
		},
	}

	diffs := CompareNormalizedTables(table1, table2)
	if len(diffs) == 0 {
		t.Error("CompareNormalizedTables should have reported missing column")
	}

	found := false
	for _, d := range diffs {
		if d == "missing column: name" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'missing column: name' in diffs, got: %v", diffs)
	}
}

func TestCompareNormalizedTables_ExtraColumn(t *testing.T) {
	table1 := NormalizedTable{
		Name: "test_table",
		Columns: []NormalizedColumn{
			{Name: "id", BaseType: BaseTypeInteger},
		},
	}
	table2 := NormalizedTable{
		Name: "test_table",
		Columns: []NormalizedColumn{
			{Name: "id", BaseType: BaseTypeInteger},
			{Name: "extra", BaseType: BaseTypeString},
		},
	}

	diffs := CompareNormalizedTables(table1, table2)
	if len(diffs) == 0 {
		t.Error("CompareNormalizedTables should have reported extra column")
	}
}

// =============================================================================
// Property Tests for Normalization
// =============================================================================

func TestProperty_NormalizationIdempotent(t *testing.T) {
	// Property: normalizing a base type should return the same base type
	proptest.QuickCheck(t, "normalization is idempotent", func(g *proptest.Generator) bool {
		baseTypes := []string{
			BaseTypeInteger, BaseTypeBigint, BaseTypeString, BaseTypeText,
			BaseTypeBoolean, BaseTypeFloat, BaseTypeDecimal, BaseTypeDatetime,
			BaseTypeBinary, BaseTypeJSON,
		}
		baseType := proptest.Pick(g, baseTypes)

		// Normalizing a base type via DDL should return the same base type
		ddlType := ""
		switch baseType {
		case BaseTypeInteger:
			ddlType = "integer"
		case BaseTypeBigint:
			ddlType = "bigint"
		case BaseTypeString:
			ddlType = "string"
		case BaseTypeText:
			ddlType = "text"
		case BaseTypeBoolean:
			ddlType = "boolean"
		case BaseTypeFloat:
			ddlType = "float"
		case BaseTypeDecimal:
			ddlType = "decimal"
		case BaseTypeDatetime:
			ddlType = "datetime"
		case BaseTypeBinary:
			ddlType = "binary"
		case BaseTypeJSON:
			ddlType = "json"
		}

		result := NormalizeDDLType(ddlType)
		return result == baseType
	})
}

func TestProperty_TypeEquivalenceSymmetric(t *testing.T) {
	// Property: type equivalence should be symmetric
	proptest.QuickCheck(t, "type equivalence is symmetric", func(g *proptest.Generator) bool {
		baseTypes := []string{
			BaseTypeInteger, BaseTypeBigint, BaseTypeString, BaseTypeText,
			BaseTypeBoolean, BaseTypeFloat, BaseTypeDecimal, BaseTypeDatetime,
			BaseTypeBinary, BaseTypeJSON,
		}

		type1 := proptest.Pick(g, baseTypes)
		type2 := proptest.Pick(g, baseTypes)

		// If type1 == type2 then both directions should be true
		// If type1 != type2 then both directions should give same result
		forward := TypesEquivalent(type1, type2)
		backward := TypesEquivalent(type2, type1)

		return forward == backward
	})
}

func TestProperty_TypeEquivalenceReflexive(t *testing.T) {
	// Property: every type should be equivalent to itself
	proptest.QuickCheck(t, "type equivalence is reflexive", func(g *proptest.Generator) bool {
		baseTypes := []string{
			BaseTypeInteger, BaseTypeBigint, BaseTypeString, BaseTypeText,
			BaseTypeBoolean, BaseTypeFloat, BaseTypeDecimal, BaseTypeDatetime,
			BaseTypeBinary, BaseTypeJSON,
		}

		typ := proptest.Pick(g, baseTypes)
		return TypesEquivalent(typ, typ)
	})
}
