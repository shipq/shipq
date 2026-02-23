package query

import (
	"testing"
)

func TestInt32Column(t *testing.T) {
	col := Int32Column{Table: "users", Name: "age"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "age" {
		t.Errorf("expected ColumnName() = %q, got %q", "age", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "int32" {
		t.Errorf("expected GoType() = %q, got %q", "int32", col.GoType())
	}
}

func TestNullInt32Column(t *testing.T) {
	col := NullInt32Column{Table: "users", Name: "score"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "score" {
		t.Errorf("expected ColumnName() = %q, got %q", "score", col.ColumnName())
	}
	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*int32" {
		t.Errorf("expected GoType() = %q, got %q", "*int32", col.GoType())
	}
}

func TestInt64Column(t *testing.T) {
	col := Int64Column{Table: "users", Name: "id"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "id" {
		t.Errorf("expected ColumnName() = %q, got %q", "id", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "int64" {
		t.Errorf("expected GoType() = %q, got %q", "int64", col.GoType())
	}
}

func TestNullInt64Column(t *testing.T) {
	col := NullInt64Column{Table: "users", Name: "parent_id"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "parent_id" {
		t.Errorf("expected ColumnName() = %q, got %q", "parent_id", col.ColumnName())
	}
	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*int64" {
		t.Errorf("expected GoType() = %q, got %q", "*int64", col.GoType())
	}
}

func TestFloat64Column(t *testing.T) {
	col := Float64Column{Table: "products", Name: "weight"}

	if col.TableName() != "products" {
		t.Errorf("expected TableName() = %q, got %q", "products", col.TableName())
	}
	if col.ColumnName() != "weight" {
		t.Errorf("expected ColumnName() = %q, got %q", "weight", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "float64" {
		t.Errorf("expected GoType() = %q, got %q", "float64", col.GoType())
	}
}

func TestNullFloat64Column(t *testing.T) {
	col := NullFloat64Column{Table: "products", Name: "discount"}

	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*float64" {
		t.Errorf("expected GoType() = %q, got %q", "*float64", col.GoType())
	}
}

func TestDecimalColumn(t *testing.T) {
	col := DecimalColumn{Table: "orders", Name: "total"}

	if col.TableName() != "orders" {
		t.Errorf("expected TableName() = %q, got %q", "orders", col.TableName())
	}
	if col.ColumnName() != "total" {
		t.Errorf("expected ColumnName() = %q, got %q", "total", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	// Decimal is stored as string for precision
	if col.GoType() != "string" {
		t.Errorf("expected GoType() = %q, got %q", "string", col.GoType())
	}
}

func TestNullDecimalColumn(t *testing.T) {
	col := NullDecimalColumn{Table: "orders", Name: "discount"}

	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*string" {
		t.Errorf("expected GoType() = %q, got %q", "*string", col.GoType())
	}
}

func TestBoolColumn(t *testing.T) {
	col := BoolColumn{Table: "users", Name: "active"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "active" {
		t.Errorf("expected ColumnName() = %q, got %q", "active", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "bool" {
		t.Errorf("expected GoType() = %q, got %q", "bool", col.GoType())
	}
}

func TestNullBoolColumn(t *testing.T) {
	col := NullBoolColumn{Table: "users", Name: "verified"}

	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*bool" {
		t.Errorf("expected GoType() = %q, got %q", "*bool", col.GoType())
	}
}

func TestStringColumn(t *testing.T) {
	col := StringColumn{Table: "users", Name: "name"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "name" {
		t.Errorf("expected ColumnName() = %q, got %q", "name", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "string" {
		t.Errorf("expected GoType() = %q, got %q", "string", col.GoType())
	}
}

func TestNullStringColumn(t *testing.T) {
	col := NullStringColumn{Table: "users", Name: "bio"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "bio" {
		t.Errorf("expected ColumnName() = %q, got %q", "bio", col.ColumnName())
	}
	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*string" {
		t.Errorf("expected GoType() = %q, got %q", "*string", col.GoType())
	}
}

func TestTimeColumn(t *testing.T) {
	col := TimeColumn{Table: "users", Name: "created_at"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "created_at" {
		t.Errorf("expected ColumnName() = %q, got %q", "created_at", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "time.Time" {
		t.Errorf("expected GoType() = %q, got %q", "time.Time", col.GoType())
	}
}

func TestNullTimeColumn(t *testing.T) {
	col := NullTimeColumn{Table: "users", Name: "deleted_at"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "deleted_at" {
		t.Errorf("expected ColumnName() = %q, got %q", "deleted_at", col.ColumnName())
	}
	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "*time.Time" {
		t.Errorf("expected GoType() = %q, got %q", "*time.Time", col.GoType())
	}
}

func TestBytesColumn(t *testing.T) {
	col := BytesColumn{Table: "files", Name: "data"}

	if col.TableName() != "files" {
		t.Errorf("expected TableName() = %q, got %q", "files", col.TableName())
	}
	if col.ColumnName() != "data" {
		t.Errorf("expected ColumnName() = %q, got %q", "data", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "[]byte" {
		t.Errorf("expected GoType() = %q, got %q", "[]byte", col.GoType())
	}
}

func TestJSONColumn(t *testing.T) {
	col := JSONColumn{Table: "users", Name: "metadata"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "metadata" {
		t.Errorf("expected ColumnName() = %q, got %q", "metadata", col.ColumnName())
	}
	if col.IsNullable() {
		t.Error("expected IsNullable() = false")
	}
	if col.GoType() != "json.RawMessage" {
		t.Errorf("expected GoType() = %q, got %q", "json.RawMessage", col.GoType())
	}
}

func TestNullJSONColumn(t *testing.T) {
	col := NullJSONColumn{Table: "users", Name: "settings"}

	if col.TableName() != "users" {
		t.Errorf("expected TableName() = %q, got %q", "users", col.TableName())
	}
	if col.ColumnName() != "settings" {
		t.Errorf("expected ColumnName() = %q, got %q", "settings", col.ColumnName())
	}
	if !col.IsNullable() {
		t.Error("expected IsNullable() = true")
	}
	if col.GoType() != "json.RawMessage" {
		t.Errorf("expected GoType() = %q, got %q", "json.RawMessage", col.GoType())
	}
}

// TestColumnInterface verifies that all column types implement the Column interface.
func TestColumnInterface(t *testing.T) {
	columns := []Column{
		Int32Column{Table: "t", Name: "c"},
		NullInt32Column{Table: "t", Name: "c"},
		Int64Column{Table: "t", Name: "c"},
		NullInt64Column{Table: "t", Name: "c"},
		Float64Column{Table: "t", Name: "c"},
		NullFloat64Column{Table: "t", Name: "c"},
		DecimalColumn{Table: "t", Name: "c"},
		NullDecimalColumn{Table: "t", Name: "c"},
		BoolColumn{Table: "t", Name: "c"},
		NullBoolColumn{Table: "t", Name: "c"},
		StringColumn{Table: "t", Name: "c"},
		NullStringColumn{Table: "t", Name: "c"},
		TimeColumn{Table: "t", Name: "c"},
		NullTimeColumn{Table: "t", Name: "c"},
		BytesColumn{Table: "t", Name: "c"},
		JSONColumn{Table: "t", Name: "c"},
		NullJSONColumn{Table: "t", Name: "c"},
	}

	for _, col := range columns {
		// Just verify the interface methods work
		_ = col.TableName()
		_ = col.ColumnName()
		_ = col.IsNullable()
		_ = col.GoType()
	}
}
