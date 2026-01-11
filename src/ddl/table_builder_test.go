package ddl

import (
	"testing"
)

func TestColumnTypes(t *testing.T) {
	tests := []struct {
		name        string
		buildTable  func() *Table
		wantType    string
		wantColName string
		wantLength  *int
		wantPrec    *int
		wantScale   *int
	}{
		{
			name: "Integer sets integer type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Integer("count")
				return tb.Build()
			},
			wantType:    IntegerType,
			wantColName: "count",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Bigint sets bigint type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Bigint("id")
				return tb.Build()
			},
			wantType:    BigintType,
			wantColName: "id",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Float sets float type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Float("price")
				return tb.Build()
			},
			wantType:    FloatType,
			wantColName: "price",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Bool sets boolean type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Bool("active")
				return tb.Build()
			},
			wantType:    BooleanType,
			wantColName: "active",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Text sets text type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Text("bio")
				return tb.Build()
			},
			wantType:    TextType,
			wantColName: "bio",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Datetime sets datetime type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Datetime("created_at")
				return tb.Build()
			},
			wantType:    DatetimeType,
			wantColName: "created_at",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Timestamp sets timestamp type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Timestamp("updated_at")
				return tb.Build()
			},
			wantType:    TimestampType,
			wantColName: "updated_at",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "Binary sets binary type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Binary("data")
				return tb.Build()
			},
			wantType:    BinaryType,
			wantColName: "data",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
		{
			name: "JSON sets json type",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.JSON("metadata")
				return tb.Build()
			},
			wantType:    JSONType,
			wantColName: "metadata",
			wantLength:  nil,
			wantPrec:    nil,
			wantScale:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := tt.buildTable()
			if len(table.Columns) != 1 {
				t.Fatalf("expected 1 column, got %d", len(table.Columns))
			}
			col := table.Columns[0]
			if col.Name != tt.wantColName {
				t.Errorf("column name = %q, want %q", col.Name, tt.wantColName)
			}
			if col.Type != tt.wantType {
				t.Errorf("column type = %q, want %q", col.Type, tt.wantType)
			}
			if !ptrEqual(col.Length, tt.wantLength) {
				t.Errorf("column length = %v, want %v", ptrVal(col.Length), ptrVal(tt.wantLength))
			}
			if !ptrEqual(col.Precision, tt.wantPrec) {
				t.Errorf("column precision = %v, want %v", ptrVal(col.Precision), ptrVal(tt.wantPrec))
			}
			if !ptrEqual(col.Scale, tt.wantScale) {
				t.Errorf("column scale = %v, want %v", ptrVal(col.Scale), ptrVal(tt.wantScale))
			}
		})
	}
}

func TestString(t *testing.T) {
	tb := MakeEmptyTable("test")
	tb.String("email")
	table := tb.Build()

	if len(table.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(table.Columns))
	}
	col := table.Columns[0]
	if col.Type != StringType {
		t.Errorf("column type = %q, want %q", col.Type, StringType)
	}
	if col.Length == nil || *col.Length != 255 {
		t.Errorf("column length = %v, want 255", ptrVal(col.Length))
	}
}

func TestVarchar(t *testing.T) {
	tests := []struct {
		name       string
		length     int
		wantLength int
	}{
		{
			name:       "Varchar sets length to 100",
			length:     100,
			wantLength: 100,
		},
		{
			name:       "Varchar sets length to 21 (nanoid)",
			length:     21,
			wantLength: 21,
		},
		{
			name:       "Varchar sets length to 500",
			length:     500,
			wantLength: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := MakeEmptyTable("test")
			tb.Varchar("field", tt.length)
			table := tb.Build()

			if len(table.Columns) != 1 {
				t.Fatalf("expected 1 column, got %d", len(table.Columns))
			}
			col := table.Columns[0]
			if col.Type != StringType {
				t.Errorf("column type = %q, want %q", col.Type, StringType)
			}
			if col.Length == nil || *col.Length != tt.wantLength {
				t.Errorf("column length = %v, want %d", ptrVal(col.Length), tt.wantLength)
			}
		})
	}
}

func TestDecimal(t *testing.T) {
	tests := []struct {
		name          string
		precision     int
		scale         int
		wantPrecision int
		wantScale     int
	}{
		{
			name:          "decimal with precision 10, scale 2",
			precision:     10,
			scale:         2,
			wantPrecision: 10,
			wantScale:     2,
		},
		{
			name:          "decimal with precision 18, scale 4",
			precision:     18,
			scale:         4,
			wantPrecision: 18,
			wantScale:     4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := MakeEmptyTable("test")
			tb.Decimal("amount", tt.precision, tt.scale)
			table := tb.Build()

			if len(table.Columns) != 1 {
				t.Fatalf("expected 1 column, got %d", len(table.Columns))
			}
			col := table.Columns[0]
			if col.Type != DecimalType {
				t.Errorf("column type = %q, want %q", col.Type, DecimalType)
			}
			if col.Precision == nil || *col.Precision != tt.wantPrecision {
				t.Errorf("column precision = %v, want %d", ptrVal(col.Precision), tt.wantPrecision)
			}
			if col.Scale == nil || *col.Scale != tt.wantScale {
				t.Errorf("column scale = %v, want %d", ptrVal(col.Scale), tt.wantScale)
			}
		})
	}
}

func TestModifiers(t *testing.T) {
	tests := []struct {
		name           string
		buildTable     func() *Table
		wantPrimaryKey bool
		wantNullable   bool
		wantUnique     bool
	}{
		{
			name: "PrimaryKey sets primary key",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Bigint("id").PrimaryKey()
				return tb.Build()
			},
			wantPrimaryKey: true,
			wantNullable:   false,
			wantUnique:     false,
		},
		{
			name: "Nullable sets nullable",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.String("bio").Nullable()
				return tb.Build()
			},
			wantPrimaryKey: false,
			wantNullable:   true,
			wantUnique:     false,
		},
		{
			name: "Unique sets unique",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.String("email").Unique()
				return tb.Build()
			},
			wantPrimaryKey: false,
			wantNullable:   false,
			wantUnique:     true,
		},
		{
			name: "multiple modifiers can be chained",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.String("code").Unique().Nullable()
				return tb.Build()
			},
			wantPrimaryKey: false,
			wantNullable:   true,
			wantUnique:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := tt.buildTable()
			if len(table.Columns) != 1 {
				t.Fatalf("expected 1 column, got %d", len(table.Columns))
			}
			col := table.Columns[0]
			if col.PrimaryKey != tt.wantPrimaryKey {
				t.Errorf("PrimaryKey = %v, want %v", col.PrimaryKey, tt.wantPrimaryKey)
			}
			if col.Nullable != tt.wantNullable {
				t.Errorf("Nullable = %v, want %v", col.Nullable, tt.wantNullable)
			}
			if col.Unique != tt.wantUnique {
				t.Errorf("Unique = %v, want %v", col.Unique, tt.wantUnique)
			}
		})
	}
}

func TestDefaultValues(t *testing.T) {
	tests := []struct {
		name        string
		buildTable  func() *Table
		wantDefault *string
	}{
		{
			name: "Default(bool) serializes true correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Bool("active").Default(true)
				return tb.Build()
			},
			wantDefault: strPtr("true"),
		},
		{
			name: "Default(bool) serializes false correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Bool("active").Default(false)
				return tb.Build()
			},
			wantDefault: strPtr("false"),
		},
		{
			name: "Default(int64) serializes correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Integer("count").Default(42)
				return tb.Build()
			},
			wantDefault: strPtr("42"),
		},
		{
			name: "Default(int64) serializes zero correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Bigint("count").Default(0)
				return tb.Build()
			},
			wantDefault: strPtr("0"),
		},
		{
			name: "Default(int64) serializes negative correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Integer("offset").Default(-10)
				return tb.Build()
			},
			wantDefault: strPtr("-10"),
		},
		{
			name: "Default(string) serializes correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.String("status").Default("pending")
				return tb.Build()
			},
			wantDefault: strPtr("pending"),
		},
		// Note: TEXT columns cannot have DEFAULT values in MySQL.
		// The test for Text().Default() was removed for cross-database compatibility.
		{
			name: "Default(float64) serializes correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Float("rate").Default(3.14)
				return tb.Build()
			},
			wantDefault: strPtr("3.14"),
		},
		{
			name: "Default(float64) serializes integer-like float correctly",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Float("amount").Default(100.0)
				return tb.Build()
			},
			wantDefault: strPtr("100"),
		},
		{
			name: "Default on decimal column",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Decimal("price", 10, 2).Default("99.99")
				return tb.Build()
			},
			wantDefault: strPtr("99.99"),
		},
		{
			name: "Default on time column",
			buildTable: func() *Table {
				tb := MakeEmptyTable("test")
				tb.Datetime("created_at").Default("CURRENT_TIMESTAMP")
				return tb.Build()
			},
			wantDefault: strPtr("CURRENT_TIMESTAMP"),
		},
		// Note: JSON columns cannot have defaults (MySQL limitation)
		// so there's no test case for JSON defaults
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := tt.buildTable()
			if len(table.Columns) != 1 {
				t.Fatalf("expected 1 column, got %d", len(table.Columns))
			}
			col := table.Columns[0]
			if !strPtrEqual(col.Default, tt.wantDefault) {
				t.Errorf("Default = %q, want %q", strPtrVal(col.Default), strPtrVal(tt.wantDefault))
			}
		})
	}
}

func TestMultipleColumns(t *testing.T) {
	tb := MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.Varchar("public_id", 21).Unique()
	tb.String("email").Unique()
	tb.Varchar("name", 100)
	tb.Text("bio").Nullable()
	tb.Bool("active").Default(true)
	tb.Integer("login_count").Default(0)
	tb.Datetime("created_at")
	table := tb.Build()

	expectedColumns := []struct {
		name       string
		colType    string
		primaryKey bool
		unique     bool
		nullable   bool
		defaultVal *string
		length     *int
	}{
		{name: "id", colType: BigintType, primaryKey: true, unique: false, nullable: false, defaultVal: nil, length: nil},
		{name: "public_id", colType: StringType, primaryKey: false, unique: true, nullable: false, defaultVal: nil, length: intPtr(21)},
		{name: "email", colType: StringType, primaryKey: false, unique: true, nullable: false, defaultVal: nil, length: intPtr(255)},
		{name: "name", colType: StringType, primaryKey: false, unique: false, nullable: false, defaultVal: nil, length: intPtr(100)},
		{name: "bio", colType: TextType, primaryKey: false, unique: false, nullable: true, defaultVal: nil, length: nil},
		{name: "active", colType: BooleanType, primaryKey: false, unique: false, nullable: false, defaultVal: strPtr("true"), length: nil},
		{name: "login_count", colType: IntegerType, primaryKey: false, unique: false, nullable: false, defaultVal: strPtr("0"), length: nil},
		{name: "created_at", colType: DatetimeType, primaryKey: false, unique: false, nullable: false, defaultVal: nil, length: nil},
	}

	if len(table.Columns) != len(expectedColumns) {
		t.Fatalf("expected %d columns, got %d", len(expectedColumns), len(table.Columns))
	}

	for i, expected := range expectedColumns {
		col := table.Columns[i]
		if col.Name != expected.name {
			t.Errorf("column %d: name = %q, want %q", i, col.Name, expected.name)
		}
		if col.Type != expected.colType {
			t.Errorf("column %d: type = %q, want %q", i, col.Type, expected.colType)
		}
		if col.PrimaryKey != expected.primaryKey {
			t.Errorf("column %d: primaryKey = %v, want %v", i, col.PrimaryKey, expected.primaryKey)
		}
		if col.Unique != expected.unique {
			t.Errorf("column %d: unique = %v, want %v", i, col.Unique, expected.unique)
		}
		if col.Nullable != expected.nullable {
			t.Errorf("column %d: nullable = %v, want %v", i, col.Nullable, expected.nullable)
		}
		if !strPtrEqual(col.Default, expected.defaultVal) {
			t.Errorf("column %d: default = %q, want %q", i, strPtrVal(col.Default), strPtrVal(expected.defaultVal))
		}
		if !ptrEqual(col.Length, expected.length) {
			t.Errorf("column %d: length = %v, want %v", i, ptrVal(col.Length), ptrVal(expected.length))
		}
	}
}

func TestMakeTable(t *testing.T) {
	type colWant struct {
		name       string
		colType    string
		primaryKey bool
		unique     bool
		nullable   bool
		defaultVal *string
		length     *int
	}

	defaultCols := []colWant{
		{
			name:       "id",
			colType:    BigintType,
			primaryKey: true,
			unique:     true,
			nullable:   false,
			defaultVal: nil,
			length:     nil,
		},
		{
			name:       "public_id",
			colType:    StringType,
			primaryKey: false,
			unique:     true,
			nullable:   false,
			defaultVal: nil,
			length:     nil,
		},
		{
			name:       "created_at",
			colType:    DatetimeType,
			primaryKey: false,
			unique:     false,
			nullable:   false,
			defaultVal: nil,
			length:     nil,
		},
		{
			name:       "deleted_at",
			colType:    DatetimeType,
			primaryKey: false,
			unique:     false,
			nullable:   false,
			defaultVal: nil,
			length:     nil,
		},
		{
			name:       "updated_at",
			colType:    DatetimeType,
			primaryKey: false,
			unique:     false,
			nullable:   false,
			defaultVal: nil,
			length:     nil,
		},
	}

	tbl := MakeTable("users").Build()
	if tbl.Name != "users" {
		t.Errorf("expected table name %q, got %q", "users", tbl.Name)
	}
	if len(tbl.Columns) != len(defaultCols) {
		t.Fatalf("expected %d columns, got %d", len(defaultCols), len(tbl.Columns))
	}
	for i, want := range defaultCols {
		got := tbl.Columns[i]
		if got.Name != want.name {
			t.Errorf("col %d: name = %q, want %q", i, got.Name, want.name)
		}
		if got.Type != want.colType {
			t.Errorf("col %d: type = %q, want %q", i, got.Type, want.colType)
		}
		if got.PrimaryKey != want.primaryKey {
			t.Errorf("col %d: primaryKey = %v, want %v", i, got.PrimaryKey, want.primaryKey)
		}
		if got.Unique != want.unique {
			t.Errorf("col %d: unique = %v, want %v", i, got.Unique, want.unique)
		}
		if got.Nullable != want.nullable {
			t.Errorf("col %d: nullable = %v, want %v", i, got.Nullable, want.nullable)
		}
		if !strPtrEqual(got.Default, want.defaultVal) {
			t.Errorf("col %d: default = %q, want %q", i, strPtrVal(got.Default), strPtrVal(want.defaultVal))
		}
		if !ptrEqual(got.Length, want.length) {
			t.Errorf("col %d: length = %v, want %v", i, ptrVal(got.Length), ptrVal(want.length))
		}
	}

	// MakeTable should also create indexes for id and public_id
	if len(tbl.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(tbl.Indexes))
	}
	if tbl.Indexes[0].Name != "idx_users_id" {
		t.Errorf("index 0: name = %q, want %q", tbl.Indexes[0].Name, "idx_users_id")
	}
	if tbl.Indexes[1].Name != "idx_users_public_id" {
		t.Errorf("index 1: name = %q, want %q", tbl.Indexes[1].Name, "idx_users_public_id")
	}
}

func TestMakeEmptyTable(t *testing.T) {
	table := MakeEmptyTable("test").Build()
	if table.Name != "test" {
		t.Errorf("table name = %q, want %q", table.Name, "test")
	}
	if len(table.Columns) != 0 {
		t.Errorf("expected 0 columns, got %d", len(table.Columns))
	}
	if table.Columns == nil {
		t.Error("Columns should be empty slice, not nil")
	}
	if table.Indexes == nil {
		t.Error("Indexes should be empty slice, not nil")
	}
}

// --- Index Tests ---

func TestSingleColumnIndex(t *testing.T) {
	tb := MakeEmptyTable("orders")
	tb.String("status").Indexed()
	table := tb.Build()

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_orders_status" {
		t.Errorf("index name = %q, want %q", idx.Name, "idx_orders_status")
	}
	if len(idx.Columns) != 1 || idx.Columns[0] != "status" {
		t.Errorf("index columns = %v, want [status]", idx.Columns)
	}
	if idx.Unique {
		t.Error("index should not be unique")
	}

	// Column should also have Index flag set
	col := table.Columns[0]
	if !col.Index {
		t.Error("column Index flag should be true")
	}
}

func TestUniqueAutoIndex(t *testing.T) {
	tb := MakeEmptyTable("users")
	tb.String("email").Unique()
	table := tb.Build()

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_users_email" {
		t.Errorf("index name = %q, want %q", idx.Name, "idx_users_email")
	}
	if len(idx.Columns) != 1 || idx.Columns[0] != "email" {
		t.Errorf("index columns = %v, want [email]", idx.Columns)
	}
	if !idx.Unique {
		t.Error("index should be unique")
	}
}

func TestCompositeIndex(t *testing.T) {
	tb := MakeEmptyTable("orders")
	userId := tb.Bigint("user_id")
	status := tb.String("status")
	tb.AddIndex(userId.Col(), status.Col())
	table := tb.Build()

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_orders_user_id_status" {
		t.Errorf("index name = %q, want %q", idx.Name, "idx_orders_user_id_status")
	}
	if len(idx.Columns) != 2 {
		t.Fatalf("expected 2 columns in index, got %d", len(idx.Columns))
	}
	if idx.Columns[0] != "user_id" || idx.Columns[1] != "status" {
		t.Errorf("index columns = %v, want [user_id status]", idx.Columns)
	}
	if idx.Unique {
		t.Error("index should not be unique")
	}
}

func TestUniqueCompositeIndex(t *testing.T) {
	tb := MakeEmptyTable("user_roles")
	userId := tb.Bigint("user_id")
	roleId := tb.Bigint("role_id")
	tb.AddUniqueIndex(userId.Col(), roleId.Col())
	table := tb.Build()

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_user_roles_user_id_role_id" {
		t.Errorf("index name = %q, want %q", idx.Name, "idx_user_roles_user_id_role_id")
	}
	if len(idx.Columns) != 2 {
		t.Fatalf("expected 2 columns in index, got %d", len(idx.Columns))
	}
	if idx.Columns[0] != "user_id" || idx.Columns[1] != "role_id" {
		t.Errorf("index columns = %v, want [user_id role_id]", idx.Columns)
	}
	if !idx.Unique {
		t.Error("index should be unique")
	}
}

func TestMultipleIndexes(t *testing.T) {
	tb := MakeEmptyTable("orders")
	tb.Bigint("id").PrimaryKey()
	tb.String("public_id").Unique()
	userId := tb.Bigint("user_id").Indexed()
	status := tb.String("status")
	tb.Datetime("created_at").Indexed()
	tb.AddIndex(userId.Col(), status.Col())
	table := tb.Build()

	// Should have:
	// 1. idx_orders_public_id (unique, from .Unique())
	// 2. idx_orders_user_id (from .Indexed())
	// 3. idx_orders_created_at (from .Indexed())
	// 4. idx_orders_user_id_status (composite)
	if len(table.Indexes) != 4 {
		t.Fatalf("expected 4 indexes, got %d", len(table.Indexes))
	}

	expectedIndexes := []struct {
		name    string
		columns []string
		unique  bool
	}{
		{name: "idx_orders_public_id", columns: []string{"public_id"}, unique: true},
		{name: "idx_orders_user_id", columns: []string{"user_id"}, unique: false},
		{name: "idx_orders_created_at", columns: []string{"created_at"}, unique: false},
		{name: "idx_orders_user_id_status", columns: []string{"user_id", "status"}, unique: false},
	}

	for i, want := range expectedIndexes {
		got := table.Indexes[i]
		if got.Name != want.name {
			t.Errorf("index %d: name = %q, want %q", i, got.Name, want.name)
		}
		if len(got.Columns) != len(want.columns) {
			t.Errorf("index %d: columns count = %d, want %d", i, len(got.Columns), len(want.columns))
		} else {
			for j, col := range want.columns {
				if got.Columns[j] != col {
					t.Errorf("index %d: column %d = %q, want %q", i, j, got.Columns[j], col)
				}
			}
		}
		if got.Unique != want.unique {
			t.Errorf("index %d: unique = %v, want %v", i, got.Unique, want.unique)
		}
	}
}

func TestColRefTypeSafety(t *testing.T) {
	// This test verifies that column refs can only be created from actual columns.
	// The type system ensures you can't pass arbitrary strings to AddIndex.
	tb := MakeEmptyTable("test")
	col1 := tb.String("foo")
	col2 := tb.Integer("bar")

	// These should compile and work
	tb.AddIndex(col1.Col(), col2.Col())

	table := tb.Build()
	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}
	if table.Indexes[0].Columns[0] != "foo" || table.Indexes[0].Columns[1] != "bar" {
		t.Errorf("unexpected columns: %v", table.Indexes[0].Columns)
	}
}
