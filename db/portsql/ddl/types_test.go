package ddl

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTableSerialize(t *testing.T) {
	tb := MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("email").Unique()
	tb.Bool("active").Default(true)
	table := tb.Build()

	jsonStr, err := table.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	// Verify it's valid JSON by unmarshaling
	var parsed Table
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Serialize() produced invalid JSON: %v", err)
	}

	// Verify key fields
	if parsed.Name != "users" {
		t.Errorf("parsed table name = %q, want %q", parsed.Name, "users")
	}
	if len(parsed.Columns) != 3 {
		t.Errorf("parsed columns count = %d, want 3", len(parsed.Columns))
	}
}

func TestIndexNameGeneration(t *testing.T) {
	tests := []struct {
		tableName string
		columns   []string
		wantName  string
	}{
		{
			tableName: "users",
			columns:   []string{"email"},
			wantName:  "idx_users_email",
		},
		{
			tableName: "orders",
			columns:   []string{"user_id", "status"},
			wantName:  "idx_orders_user_id_status",
		},
		{
			tableName: "product_variants",
			columns:   []string{"product_id", "color", "size"},
			wantName:  "idx_product_variants_product_id_color_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			got := GenerateIndexName(tt.tableName, tt.columns)
			if got != tt.wantName {
				t.Errorf("GenerateIndexName(%q, %v) = %q, want %q", tt.tableName, tt.columns, got, tt.wantName)
			}
		})
	}
}

// Helper functions used across test files

func ptrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrVal(p *int) string {
	if p == nil {
		return "nil"
	}
	return string(rune(*p))
}

func intPtr(v int) *int {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func strPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func strPtrVal(p *string) string {
	if p == nil {
		return "nil"
	}
	return *p
}

// =============================================================================
// ColumnDefinition.References Field Tests
// =============================================================================

func TestColumnDefinition_References_JSON(t *testing.T) {
	col := ColumnDefinition{
		Name:       "category_id",
		Type:       BigintType,
		References: "categories",
	}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if !strings.Contains(string(data), `"references":"categories"`) {
		t.Errorf("expected references in JSON, got: %s", data)
	}
}

func TestColumnDefinition_References_OmitEmpty(t *testing.T) {
	col := ColumnDefinition{Name: "id", Type: BigintType}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if strings.Contains(string(data), "references") {
		t.Errorf("expected references to be omitted, got: %s", data)
	}
}

func TestColumnDefinition_References_Roundtrip(t *testing.T) {
	original := ColumnDefinition{
		Name:       "pet_id",
		Type:       BigintType,
		References: "pets",
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal
	var restored ColumnDefinition
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.References != "pets" {
		t.Errorf("expected References='pets', got %q", restored.References)
	}
}
