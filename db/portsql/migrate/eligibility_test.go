package migrate

import (
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// Autoincrement Eligibility Tests
// =============================================================================

func TestGetAutoincrementPK_SingleIntegerPK(t *testing.T) {
	table := &ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.IntegerType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}

	info, ok := GetAutoincrementPK(table)

	if !ok {
		t.Error("GetAutoincrementPK() should return true for single integer PK")
	}
	if info.ColumnName != "id" {
		t.Errorf("ColumnName = %q, want %q", info.ColumnName, "id")
	}
	if info.ColumnType != ddl.IntegerType {
		t.Errorf("ColumnType = %q, want %q", info.ColumnType, ddl.IntegerType)
	}
}

func TestGetAutoincrementPK_SingleBigintPK(t *testing.T) {
	table := &ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}

	info, ok := GetAutoincrementPK(table)

	if !ok {
		t.Error("GetAutoincrementPK() should return true for single bigint PK")
	}
	if info.ColumnName != "id" {
		t.Errorf("ColumnName = %q, want %q", info.ColumnName, "id")
	}
	if info.ColumnType != ddl.BigintType {
		t.Errorf("ColumnType = %q, want %q", info.ColumnType, ddl.BigintType)
	}
}

func TestGetAutoincrementPK_CompositePK(t *testing.T) {
	table := &ddl.Table{
		Name: "user_roles",
		Columns: []ddl.ColumnDefinition{
			{Name: "user_id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "role_id", Type: ddl.BigintType, PrimaryKey: true},
		},
	}

	_, ok := GetAutoincrementPK(table)

	if ok {
		t.Error("GetAutoincrementPK() should return false for composite PK")
	}
}

func TestGetAutoincrementPK_StringPK(t *testing.T) {
	table := &ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "key", Type: ddl.StringType, PrimaryKey: true},
			{Name: "value", Type: ddl.StringType},
		},
	}

	_, ok := GetAutoincrementPK(table)

	if ok {
		t.Error("GetAutoincrementPK() should return false for non-integer PK")
	}
}

func TestGetAutoincrementPK_NoPK(t *testing.T) {
	table := &ddl.Table{
		Name: "logs",
		Columns: []ddl.ColumnDefinition{
			{Name: "message", Type: ddl.StringType},
			{Name: "timestamp", Type: ddl.TimestampType},
		},
	}

	_, ok := GetAutoincrementPK(table)

	if ok {
		t.Error("GetAutoincrementPK() should return false for table without PK")
	}
}

func TestGetAutoincrementPK_JunctionTable(t *testing.T) {
	table := &ddl.Table{
		Name:            "user_groups",
		IsJunctionTable: true,
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "user_id", Type: ddl.BigintType},
			{Name: "group_id", Type: ddl.BigintType},
		},
	}

	_, ok := GetAutoincrementPK(table)

	if ok {
		t.Error("GetAutoincrementPK() should return false for junction tables")
	}
}

func TestIsAutoincrementEligible(t *testing.T) {
	tests := []struct {
		name     string
		table    *ddl.Table
		expected bool
	}{
		{
			name: "eligible - single integer PK",
			table: &ddl.Table{
				Name: "users",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.IntegerType, PrimaryKey: true},
				},
			},
			expected: true,
		},
		{
			name: "eligible - single bigint PK",
			table: &ddl.Table{
				Name: "posts",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				},
			},
			expected: true,
		},
		{
			name: "not eligible - composite PK",
			table: &ddl.Table{
				Name: "user_roles",
				Columns: []ddl.ColumnDefinition{
					{Name: "user_id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "role_id", Type: ddl.BigintType, PrimaryKey: true},
				},
			},
			expected: false,
		},
		{
			name: "not eligible - string PK",
			table: &ddl.Table{
				Name: "settings",
				Columns: []ddl.ColumnDefinition{
					{Name: "key", Type: ddl.StringType, PrimaryKey: true},
				},
			},
			expected: false,
		},
		{
			name: "not eligible - no PK",
			table: &ddl.Table{
				Name:    "logs",
				Columns: []ddl.ColumnDefinition{},
			},
			expected: false,
		},
		{
			name: "not eligible - junction table",
			table: &ddl.Table{
				Name:            "user_groups",
				IsJunctionTable: true,
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAutoincrementEligible(tt.table)
			if result != tt.expected {
				t.Errorf("IsAutoincrementEligible() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// AddTable Eligibility Tests
// =============================================================================

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
