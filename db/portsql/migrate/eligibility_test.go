package migrate

import (
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

func TestIsAddTableTable(t *testing.T) {
	tests := []struct {
		name     string
		table    ddl.Table
		expected bool
	}{
		{
			name: "AddTable table with public_id and deleted_at",
			table: ddl.Table{
				Name: "users",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "created_at", Type: ddl.TimestampType},
					{Name: "updated_at", Type: ddl.TimestampType},
					{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
					{Name: "email", Type: ddl.StringType},
				},
			},
			expected: true,
		},
		{
			name: "AddEmptyTable without public_id",
			table: ddl.Table{
				Name: "settings",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "key", Type: ddl.StringType},
					{Name: "value", Type: ddl.StringType},
				},
			},
			expected: false,
		},
		{
			name: "Table with only public_id but no deleted_at",
			table: ddl.Table{
				Name: "partial",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "name", Type: ddl.StringType},
				},
			},
			expected: false,
		},
		{
			name: "Table with only deleted_at but no public_id",
			table: ddl.Table{
				Name: "partial",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
					{Name: "name", Type: ddl.StringType},
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
			result := IsAddTableTable(tt.table)
			if result != tt.expected {
				t.Errorf("IsAddTableTable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetCRUDTables(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Tables: map[string]ddl.Table{
				"users": {
					Name: "users",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "created_at", Type: ddl.TimestampType},
						{Name: "updated_at", Type: ddl.TimestampType},
						{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
						{Name: "email", Type: ddl.StringType},
					},
				},
				"settings": {
					Name: "settings",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "key", Type: ddl.StringType},
						{Name: "value", Type: ddl.StringType},
					},
				},
				"posts": {
					Name: "posts",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "created_at", Type: ddl.TimestampType},
						{Name: "updated_at", Type: ddl.TimestampType},
						{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
						{Name: "title", Type: ddl.StringType},
					},
				},
			},
		},
	}

	tables := GetCRUDTables(plan)

	// Should return only users and posts (the AddTable tables)
	if len(tables) != 2 {
		t.Errorf("GetCRUDTables() returned %d tables, want 2", len(tables))
	}

	// Check that the returned tables are the right ones
	tableNames := make(map[string]bool)
	for _, table := range tables {
		tableNames[table.Name] = true
	}

	if !tableNames["users"] {
		t.Error("GetCRUDTables() should include 'users' table")
	}
	if !tableNames["posts"] {
		t.Error("GetCRUDTables() should include 'posts' table")
	}
	if tableNames["settings"] {
		t.Error("GetCRUDTables() should not include 'settings' table")
	}
}

func TestGetCRUDTables_EmptyPlan(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Tables: map[string]ddl.Table{},
		},
	}

	tables := GetCRUDTables(plan)

	if len(tables) != 0 {
		t.Errorf("GetCRUDTables() returned %d tables for empty plan, want 0", len(tables))
	}
}

func TestIsEligibleForResource(t *testing.T) {
	// IsEligibleForResource should be the same as IsAddTableTable
	addTable := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}

	emptyTable := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
		},
	}

	if !IsEligibleForResource(addTable) {
		t.Error("IsEligibleForResource() should return true for AddTable tables")
	}

	if IsEligibleForResource(emptyTable) {
		t.Error("IsEligibleForResource() should return false for AddEmptyTable tables")
	}
}
