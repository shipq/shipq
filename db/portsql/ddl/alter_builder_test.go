package ddl

import (
	"encoding/json"
	"testing"
)

func TestAlterTableRenameColumn(t *testing.T) {
	alt := AlterTable("users")
	alt.RenameColumn("name", "full_name")
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != OpRenameColumn {
		t.Errorf("operation type = %q, want %q", op.Type, OpRenameColumn)
	}
	if op.Column != "name" {
		t.Errorf("column = %q, want %q", op.Column, "name")
	}
	if op.NewName != "full_name" {
		t.Errorf("new_name = %q, want %q", op.NewName, "full_name")
	}
}

func TestAlterTableDropColumn(t *testing.T) {
	alt := AlterTable("users")
	alt.DropColumn("legacy_field")
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != OpDropColumn {
		t.Errorf("operation type = %q, want %q", op.Type, OpDropColumn)
	}
	if op.Column != "legacy_field" {
		t.Errorf("column = %q, want %q", op.Column, "legacy_field")
	}
}

func TestAlterTableDropIndex(t *testing.T) {
	alt := AlterTable("users")
	alt.DropIndex("idx_users_old")
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != OpDropIndex {
		t.Errorf("operation type = %q, want %q", op.Type, OpDropIndex)
	}
	if op.IndexName != "idx_users_old" {
		t.Errorf("index_name = %q, want %q", op.IndexName, "idx_users_old")
	}
}

func TestAlterTableRenameIndex(t *testing.T) {
	alt := AlterTable("users")
	alt.RenameIndex("idx_old", "idx_new")
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != OpRenameIndex {
		t.Errorf("operation type = %q, want %q", op.Type, OpRenameIndex)
	}
	if op.IndexName != "idx_old" {
		t.Errorf("index_name = %q, want %q", op.IndexName, "idx_old")
	}
	if op.NewName != "idx_new" {
		t.Errorf("new_name = %q, want %q", op.NewName, "idx_new")
	}
}

func TestAlterTableChangeType(t *testing.T) {
	alt := AlterTable("users")
	alt.ChangeType("count", BigintType)
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != OpChangeType {
		t.Errorf("operation type = %q, want %q", op.Type, OpChangeType)
	}
	if op.Column != "count" {
		t.Errorf("column = %q, want %q", op.Column, "count")
	}
	if op.NewType != BigintType {
		t.Errorf("new_type = %q, want %q", op.NewType, BigintType)
	}
}

func TestAlterTableNullability(t *testing.T) {
	t.Run("SetNullable", func(t *testing.T) {
		alt := AlterTable("users")
		alt.SetNullable("email")
		ops := alt.Build()

		if len(ops) != 1 {
			t.Fatalf("expected 1 operation, got %d", len(ops))
		}

		op := ops[0]
		if op.Type != OpChangeNullable {
			t.Errorf("operation type = %q, want %q", op.Type, OpChangeNullable)
		}
		if op.Column != "email" {
			t.Errorf("column = %q, want %q", op.Column, "email")
		}
		if op.Nullable == nil || !*op.Nullable {
			t.Error("nullable should be true")
		}
	})

	t.Run("SetNotNull", func(t *testing.T) {
		alt := AlterTable("users")
		alt.SetNotNull("email")
		ops := alt.Build()

		if len(ops) != 1 {
			t.Fatalf("expected 1 operation, got %d", len(ops))
		}

		op := ops[0]
		if op.Type != OpChangeNullable {
			t.Errorf("operation type = %q, want %q", op.Type, OpChangeNullable)
		}
		if op.Column != "email" {
			t.Errorf("column = %q, want %q", op.Column, "email")
		}
		if op.Nullable == nil || *op.Nullable {
			t.Error("nullable should be false")
		}
	})
}

func TestAlterTableDefault(t *testing.T) {
	t.Run("SetDefault", func(t *testing.T) {
		alt := AlterTable("users")
		alt.SetDefault("status", "active")
		ops := alt.Build()

		if len(ops) != 1 {
			t.Fatalf("expected 1 operation, got %d", len(ops))
		}

		op := ops[0]
		if op.Type != OpChangeDefault {
			t.Errorf("operation type = %q, want %q", op.Type, OpChangeDefault)
		}
		if op.Column != "status" {
			t.Errorf("column = %q, want %q", op.Column, "status")
		}
		if op.Default == nil || *op.Default != "active" {
			t.Errorf("default = %v, want %q", op.Default, "active")
		}
	})

	t.Run("DropDefault", func(t *testing.T) {
		alt := AlterTable("users")
		alt.DropDefault("status")
		ops := alt.Build()

		if len(ops) != 1 {
			t.Fatalf("expected 1 operation, got %d", len(ops))
		}

		op := ops[0]
		if op.Type != OpChangeDefault {
			t.Errorf("operation type = %q, want %q", op.Type, OpChangeDefault)
		}
		if op.Column != "status" {
			t.Errorf("column = %q, want %q", op.Column, "status")
		}
		if op.Default != nil {
			t.Errorf("default should be nil, got %v", op.Default)
		}
	})
}

func TestAlterTableAddColumn(t *testing.T) {
	alt := AlterTable("users")
	alt.String("bio").Nullable()
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Type != OpAddColumn {
		t.Errorf("operation type = %q, want %q", op.Type, OpAddColumn)
	}
	if op.ColumnDef == nil {
		t.Fatal("column_def should not be nil")
	}
	if op.ColumnDef.Name != "bio" {
		t.Errorf("column name = %q, want %q", op.ColumnDef.Name, "bio")
	}
	if op.ColumnDef.Type != StringType {
		t.Errorf("column type = %q, want %q", op.ColumnDef.Type, StringType)
	}
	if !op.ColumnDef.Nullable {
		t.Error("column should be nullable")
	}
}

func TestAlterTableAddColumnWithDefault(t *testing.T) {
	alt := AlterTable("users")
	alt.Bool("active").Default(true)
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.ColumnDef.Default == nil || *op.ColumnDef.Default != "true" {
		var got string
		if op.ColumnDef.Default != nil {
			got = *op.ColumnDef.Default
		}
		t.Errorf("default = %q, want %q", got, "true")
	}
}

func TestAlterTableAddColumnWithIndex(t *testing.T) {
	alt := AlterTable("users")
	alt.String("email").Indexed()
	ops := alt.Build()

	// Should have 2 operations: add_column and add_index
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}

	if ops[0].Type != OpAddColumn {
		t.Errorf("first operation type = %q, want %q", ops[0].Type, OpAddColumn)
	}
	if ops[1].Type != OpAddIndex {
		t.Errorf("second operation type = %q, want %q", ops[1].Type, OpAddIndex)
	}
	if ops[1].IndexDef == nil {
		t.Fatal("index_def should not be nil")
	}
	if ops[1].IndexDef.Unique {
		t.Error("index should not be unique")
	}
}

func TestAlterTableAddColumnWithUniqueIndex(t *testing.T) {
	alt := AlterTable("users")
	alt.String("email").Unique()
	ops := alt.Build()

	// Should have 2 operations: add_column and add_index (unique)
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}

	if ops[0].Type != OpAddColumn {
		t.Errorf("first operation type = %q, want %q", ops[0].Type, OpAddColumn)
	}
	if ops[1].Type != OpAddIndex {
		t.Errorf("second operation type = %q, want %q", ops[1].Type, OpAddIndex)
	}
	if !ops[1].IndexDef.Unique {
		t.Error("index should be unique")
	}
}

func TestAlterTableAddIndex(t *testing.T) {
	alt := AlterTable("orders")
	userId := alt.Bigint("user_id")
	status := alt.String("status")
	alt.AddIndex(userId.Col(), status.Col())
	ops := alt.Build()

	// Should have 3 operations: 2 add_column and 1 add_index
	if len(ops) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(ops))
	}

	indexOp := ops[2]
	if indexOp.Type != OpAddIndex {
		t.Errorf("operation type = %q, want %q", indexOp.Type, OpAddIndex)
	}
	if indexOp.IndexDef == nil {
		t.Fatal("index_def should not be nil")
	}
	if indexOp.IndexDef.Name != "idx_orders_user_id_status" {
		t.Errorf("index name = %q, want %q", indexOp.IndexDef.Name, "idx_orders_user_id_status")
	}
	if len(indexOp.IndexDef.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(indexOp.IndexDef.Columns))
	}
	if indexOp.IndexDef.Unique {
		t.Error("index should not be unique")
	}
}

func TestAlterTableAddUniqueIndex(t *testing.T) {
	alt := AlterTable("user_roles")
	userId := alt.Bigint("user_id")
	roleId := alt.Bigint("role_id")
	alt.AddUniqueIndex(userId.Col(), roleId.Col())
	ops := alt.Build()

	indexOp := ops[2]
	if !indexOp.IndexDef.Unique {
		t.Error("index should be unique")
	}
}

func TestAlterTableMultipleOperations(t *testing.T) {
	alt := AlterTable("users")
	alt.RenameColumn("name", "full_name")
	alt.DropColumn("legacy_field")
	bio := alt.String("bio").Nullable()
	alt.DropIndex("idx_users_old")
	alt.SetNullable("email")
	alt.SetDefault("status", "active")
	orgId := alt.Bigint("org_id")
	alt.AddIndex(bio.Col(), orgId.Col())
	ops := alt.Build()

	// Expected operations in order:
	// 1. rename_column (name -> full_name)
	// 2. drop_column (legacy_field)
	// 3. add_column (bio)
	// 4. drop_index (idx_users_old)
	// 5. change_nullable (email)
	// 6. change_default (status)
	// 7. add_column (org_id)
	// 8. add_index (bio, org_id)
	if len(ops) != 8 {
		t.Fatalf("expected 8 operations, got %d", len(ops))
	}

	expectedTypes := []OperationType{
		OpRenameColumn,
		OpDropColumn,
		OpAddColumn,
		OpDropIndex,
		OpChangeNullable,
		OpChangeDefault,
		OpAddColumn,
		OpAddIndex,
	}

	for i, wantType := range expectedTypes {
		if ops[i].Type != wantType {
			t.Errorf("operation %d: type = %q, want %q", i, ops[i].Type, wantType)
		}
	}
}

func TestAlterTableSerialize(t *testing.T) {
	alt := AlterTable("users")
	alt.RenameColumn("name", "full_name")
	alt.DropColumn("old_field")
	alt.String("bio").Nullable()

	jsonStr, err := alt.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	// Verify it's valid JSON by unmarshaling
	var parsed []TableOperation
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Serialize() produced invalid JSON: %v", err)
	}

	if len(parsed) != 3 {
		t.Errorf("parsed operations count = %d, want 3", len(parsed))
	}
}

func TestAlterTableName(t *testing.T) {
	alt := AlterTable("users")
	if alt.TableName() != "users" {
		t.Errorf("TableName() = %q, want %q", alt.TableName(), "users")
	}
}

func TestAlterTableAllColumnTypes(t *testing.T) {
	// Test that all column type methods work on AlterTableBuilder
	alt := AlterTable("test")
	alt.Integer("int_col")
	alt.Bigint("bigint_col")
	alt.Decimal("decimal_col", 10, 2)
	alt.Float("float_col")
	alt.Bool("bool_col")
	alt.String("string_col")
	alt.VarChar("varchar_col", 100)
	alt.Text("text_col")
	alt.Datetime("datetime_col")
	alt.Timestamp("timestamp_col")
	alt.Binary("binary_col")
	alt.JSON("json_col")
	ops := alt.Build()

	if len(ops) != 12 {
		t.Fatalf("expected 12 operations, got %d", len(ops))
	}

	expectedTypes := []string{
		IntegerType,
		BigintType,
		DecimalType,
		FloatType,
		BooleanType,
		StringType,
		StringType,
		TextType,
		DatetimeType,
		TimestampType,
		BinaryType,
		JSONType,
	}

	for i, wantType := range expectedTypes {
		if ops[i].ColumnDef.Type != wantType {
			t.Errorf("operation %d: column type = %q, want %q", i, ops[i].ColumnDef.Type, wantType)
		}
	}
}

// --- ExistingColumn and *Ref Method Tests ---

func TestAlterTableFromConstructor(t *testing.T) {
	// Create a table with some columns
	tb := MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("email").Unique()
	tb.String("name")
	table := tb.Build()

	// Create AlterTableBuilder from existing table
	alt := AlterTableFrom(table)

	if alt.TableName() != "users" {
		t.Errorf("TableName() = %q, want %q", alt.TableName(), "users")
	}
}

func TestExistingColumnFound(t *testing.T) {
	tb := MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("email").Unique()
	tb.String("name")
	table := tb.Build()

	alt := AlterTableFrom(table)

	// Should find existing columns
	emailRef, err := alt.ExistingColumn("email")
	if err != nil {
		t.Fatalf("ExistingColumn(\"email\") error = %v", err)
	}

	// Use the ref in an operation
	alt.SetNullableRef(emailRef)
	ops := alt.Build()

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Column != "email" {
		t.Errorf("column = %q, want %q", ops[0].Column, "email")
	}
}

func TestExistingColumnNotFound(t *testing.T) {
	tb := MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("email")
	table := tb.Build()

	alt := AlterTableFrom(table)

	// Should return error for non-existent column
	_, err := alt.ExistingColumn("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent column, got nil")
	}
}

func TestExistingColumnNoTable(t *testing.T) {
	// AlterTable (not AlterTableFrom) doesn't have existing table
	alt := AlterTable("users")

	// Should return error when no existing table provided
	_, err := alt.ExistingColumn("email")
	if err == nil {
		t.Error("expected error when no existing table, got nil")
	}
}

func TestRefMethods(t *testing.T) {
	tb := MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("email")
	tb.String("name")
	tb.String("status")
	table := tb.Build()

	alt := AlterTableFrom(table)

	email, _ := alt.ExistingColumn("email")
	name, _ := alt.ExistingColumn("name")
	status, _ := alt.ExistingColumn("status")

	// Test all *Ref methods
	alt.RenameColumnRef(name, "full_name")
	alt.SetNullableRef(email)
	alt.SetNotNullRef(status)
	alt.SetDefaultRef(status, "active")
	alt.ChangeTypeRef(email, TextType)
	alt.DropDefaultRef(status)
	alt.DropColumnRef(email)

	ops := alt.Build()

	expectedOps := []struct {
		opType OperationType
		column string
	}{
		{OpRenameColumn, "name"},
		{OpChangeNullable, "email"},
		{OpChangeNullable, "status"},
		{OpChangeDefault, "status"},
		{OpChangeType, "email"},
		{OpChangeDefault, "status"},
		{OpDropColumn, "email"},
	}

	if len(ops) != len(expectedOps) {
		t.Fatalf("expected %d operations, got %d", len(expectedOps), len(ops))
	}

	for i, want := range expectedOps {
		if ops[i].Type != want.opType {
			t.Errorf("operation %d: type = %q, want %q", i, ops[i].Type, want.opType)
		}
		if ops[i].Column != want.column {
			t.Errorf("operation %d: column = %q, want %q", i, ops[i].Column, want.column)
		}
	}
}

func TestRefMethodsWithCompositeIndex(t *testing.T) {
	tb := MakeEmptyTable("orders")
	tb.Bigint("id").PrimaryKey()
	tb.Bigint("user_id")
	tb.String("status")
	table := tb.Build()

	alt := AlterTableFrom(table)

	// Get existing column refs
	userId, _ := alt.ExistingColumn("user_id")
	status, _ := alt.ExistingColumn("status")

	// Add a new column and use it with existing columns in an index
	createdAt := alt.Datetime("created_at")
	alt.AddIndex(userId, status, createdAt.Col())

	ops := alt.Build()

	// Should have 2 operations: add_column and add_index
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}

	if ops[0].Type != OpAddColumn {
		t.Errorf("operation 0: type = %q, want %q", ops[0].Type, OpAddColumn)
	}
	if ops[1].Type != OpAddIndex {
		t.Errorf("operation 1: type = %q, want %q", ops[1].Type, OpAddIndex)
	}
	if len(ops[1].IndexDef.Columns) != 3 {
		t.Fatalf("expected 3 columns in index, got %d", len(ops[1].IndexDef.Columns))
	}
	if ops[1].IndexDef.Columns[0] != "user_id" || ops[1].IndexDef.Columns[1] != "status" || ops[1].IndexDef.Columns[2] != "created_at" {
		t.Errorf("unexpected index columns: %v", ops[1].IndexDef.Columns)
	}
}
