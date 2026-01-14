package query

import "testing"

func TestWithTable_Int64Column(t *testing.T) {
	col := Int64Column{Table: "users", Name: "id"}
	aliased := col.WithTable("u")

	if aliased.TableName() != "u" {
		t.Errorf("Expected table name 'u', got %q", aliased.TableName())
	}
	if aliased.ColumnName() != "id" {
		t.Errorf("Expected column name 'id', got %q", aliased.ColumnName())
	}
	// Original should be unchanged
	if col.TableName() != "users" {
		t.Errorf("Original column should be unchanged, got %q", col.TableName())
	}
}

func TestWithTable_StringColumn(t *testing.T) {
	col := StringColumn{Table: "users", Name: "name"}
	aliased := col.WithTable("u")

	if aliased.TableName() != "u" {
		t.Errorf("Expected table name 'u', got %q", aliased.TableName())
	}
	if aliased.ColumnName() != "name" {
		t.Errorf("Expected column name 'name', got %q", aliased.ColumnName())
	}
}

func TestWithTable_NullableColumn(t *testing.T) {
	col := NullStringColumn{Table: "users", Name: "bio"}
	aliased := col.WithTable("u")

	if aliased.TableName() != "u" {
		t.Errorf("Expected table name 'u', got %q", aliased.TableName())
	}
	if !aliased.IsNullable() {
		t.Error("Aliased column should still be nullable")
	}
	if aliased.GoType() != "*string" {
		t.Errorf("Aliased column should have same GoType, got %q", aliased.GoType())
	}
}

func TestWithTable_AllColumnTypes(t *testing.T) {
	// Test that WithTable preserves all properties except Table name
	tests := []struct {
		name       string
		col        Column
		aliasedCol Column
	}{
		{"Int32Column", Int32Column{Table: "t", Name: "c"}, Int32Column{Table: "t", Name: "c"}.WithTable("a")},
		{"NullInt32Column", NullInt32Column{Table: "t", Name: "c"}, NullInt32Column{Table: "t", Name: "c"}.WithTable("a")},
		{"Int64Column", Int64Column{Table: "t", Name: "c"}, Int64Column{Table: "t", Name: "c"}.WithTable("a")},
		{"NullInt64Column", NullInt64Column{Table: "t", Name: "c"}, NullInt64Column{Table: "t", Name: "c"}.WithTable("a")},
		{"Float64Column", Float64Column{Table: "t", Name: "c"}, Float64Column{Table: "t", Name: "c"}.WithTable("a")},
		{"NullFloat64Column", NullFloat64Column{Table: "t", Name: "c"}, NullFloat64Column{Table: "t", Name: "c"}.WithTable("a")},
		{"DecimalColumn", DecimalColumn{Table: "t", Name: "c"}, DecimalColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"NullDecimalColumn", NullDecimalColumn{Table: "t", Name: "c"}, NullDecimalColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"BoolColumn", BoolColumn{Table: "t", Name: "c"}, BoolColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"NullBoolColumn", NullBoolColumn{Table: "t", Name: "c"}, NullBoolColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"StringColumn", StringColumn{Table: "t", Name: "c"}, StringColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"NullStringColumn", NullStringColumn{Table: "t", Name: "c"}, NullStringColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"TimeColumn", TimeColumn{Table: "t", Name: "c"}, TimeColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"NullTimeColumn", NullTimeColumn{Table: "t", Name: "c"}, NullTimeColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"BytesColumn", BytesColumn{Table: "t", Name: "c"}, BytesColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"JSONColumn", JSONColumn{Table: "t", Name: "c"}, JSONColumn{Table: "t", Name: "c"}.WithTable("a")},
		{"NullJSONColumn", NullJSONColumn{Table: "t", Name: "c"}, NullJSONColumn{Table: "t", Name: "c"}.WithTable("a")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.aliasedCol.TableName() != "a" {
				t.Errorf("TableName() = %q, want 'a'", tt.aliasedCol.TableName())
			}
			if tt.aliasedCol.ColumnName() != tt.col.ColumnName() {
				t.Errorf("ColumnName() = %q, want %q", tt.aliasedCol.ColumnName(), tt.col.ColumnName())
			}
			if tt.aliasedCol.IsNullable() != tt.col.IsNullable() {
				t.Errorf("IsNullable() = %v, want %v", tt.aliasedCol.IsNullable(), tt.col.IsNullable())
			}
			if tt.aliasedCol.GoType() != tt.col.GoType() {
				t.Errorf("GoType() = %q, want %q", tt.aliasedCol.GoType(), tt.col.GoType())
			}
		})
	}
}
