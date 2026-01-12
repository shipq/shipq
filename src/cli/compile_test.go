package cli

import (
	"testing"

	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

// =============================================================================
// Table Detection Tests
// =============================================================================

func TestIsAddTableTable(t *testing.T) {
	tests := []struct {
		name     string
		table    ddl.Table
		expected bool
	}{
		{
			name: "AddTable table with all standard columns",
			table: ddl.Table{
				Name: "users",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType, Unique: true},
					{Name: "created_at", Type: ddl.DatetimeType},
					{Name: "updated_at", Type: ddl.DatetimeType},
					{Name: "deleted_at", Type: ddl.DatetimeType},
					{Name: "name", Type: ddl.StringType},
				},
			},
			expected: true,
		},
		{
			name: "AddEmptyTable missing public_id",
			table: ddl.Table{
				Name: "categories",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "name", Type: ddl.StringType},
				},
			},
			expected: false,
		},
		{
			name: "Table with public_id but no deleted_at",
			table: ddl.Table{
				Name: "logs",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "message", Type: ddl.TextType},
				},
			},
			expected: false,
		},
		{
			name: "Table with deleted_at but no public_id",
			table: ddl.Table{
				Name: "internal_data",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "deleted_at", Type: ddl.DatetimeType},
					{Name: "data", Type: ddl.JSONType},
				},
			},
			expected: false,
		},
		{
			name: "Empty table",
			table: ddl.Table{
				Name:    "empty",
				Columns: []ddl.ColumnDefinition{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddTableTable(tt.table)
			if got != tt.expected {
				t.Errorf("isAddTableTable(%s) = %v, want %v", tt.table.Name, got, tt.expected)
			}
		})
	}
}

func TestGetCRUDTables(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: map[string]ddl.Table{
				// AddTable table - should be included
				"users": {
					Name: "users",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "deleted_at", Type: ddl.DatetimeType},
						{Name: "name", Type: ddl.StringType},
					},
				},
				// AddEmptyTable - should NOT be included
				"categories": {
					Name: "categories",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "name", Type: ddl.StringType},
					},
				},
				// Another AddTable table - should be included
				"orders": {
					Name: "orders",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "deleted_at", Type: ddl.DatetimeType},
						{Name: "total", Type: ddl.DecimalType},
					},
				},
			},
		},
	}

	tables := getCRUDTables(plan)

	// Should get 2 tables (users and orders)
	if len(tables) != 2 {
		t.Errorf("expected 2 CRUD tables, got %d", len(tables))
	}

	// Check that categories is not included
	for _, table := range tables {
		if table.Name == "categories" {
			t.Error("categories should not be included in CRUD tables")
		}
	}

	// Check that users and orders are included
	foundUsers := false
	foundOrders := false
	for _, table := range tables {
		if table.Name == "users" {
			foundUsers = true
		}
		if table.Name == "orders" {
			foundOrders = true
		}
	}
	if !foundUsers {
		t.Error("users should be included in CRUD tables")
	}
	if !foundOrders {
		t.Error("orders should be included in CRUD tables")
	}
}

// =============================================================================
// Scope Resolution Tests
// =============================================================================

func TestResolveScopeForTable(t *testing.T) {
	config := &Config{
		CRUD: CRUDConfig{
			GlobalScope: "org_id",
			TableScopes: map[string]string{
				"orders":      "user_id", // Override with different scope
				"public_logs": "",        // Override with no scope
			},
		},
	}

	tests := []struct {
		table    string
		expected string
	}{
		{"users", "org_id"},      // Uses global scope
		{"orders", "user_id"},    // Uses table-specific scope
		{"public_logs", ""},      // Empty override
		{"products", "org_id"},   // Uses global scope
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			got := config.CRUD.GetScopeForTable(tt.table)
			if got != tt.expected {
				t.Errorf("GetScopeForTable(%q) = %q, want %q", tt.table, got, tt.expected)
			}
		})
	}
}
