package migrate

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// Table() Method Tests - Get validated table reference
// =============================================================================

func TestMigrationPlan_Table_Success(t *testing.T) {
	plan := NewPlan()
	plan.Schema.Tables["users"] = ddl.Table{Name: "users"}

	tableRef, err := plan.Table("users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tableRef.TableName() != "users" {
		t.Errorf("expected 'users', got %q", tableRef.TableName())
	}
}

func TestMigrationPlan_Table_NotFound(t *testing.T) {
	plan := NewPlan()

	_, err := plan.Table("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent table")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestMigrationPlan_Table_MultipleTablesExist(t *testing.T) {
	plan := NewPlan()
	plan.Schema.Tables["users"] = ddl.Table{Name: "users"}
	plan.Schema.Tables["categories"] = ddl.Table{Name: "categories"}
	plan.Schema.Tables["pets"] = ddl.Table{Name: "pets"}

	// Should find each table
	for _, name := range []string{"users", "categories", "pets"} {
		tableRef, err := plan.Table(name)
		if err != nil {
			t.Errorf("Table(%q) failed: %v", name, err)
			continue
		}
		if tableRef.TableName() != name {
			t.Errorf("expected %q, got %q", name, tableRef.TableName())
		}
	}
}

func TestMigrationPlan_Table_EmptySchema(t *testing.T) {
	plan := NewPlan()

	_, err := plan.Table("any")
	if err == nil {
		t.Fatal("expected error for empty schema")
	}
}

// =============================================================================
// Junction Table Validation Tests
// =============================================================================

func TestAddEmptyTable_JunctionTable_Valid(t *testing.T) {
	plan := NewPlan()

	// First create the referenced tables
	_, err := plan.AddEmptyTable("pets", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable pets failed: %v", err)
	}

	_, err = plan.AddEmptyTable("tags", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable tags failed: %v", err)
	}

	pets, _ := plan.Table("pets")
	tags, _ := plan.Table("tags")

	// Valid junction table with exactly 2 references
	_, err = plan.AddEmptyTable("pet_tags", func(tb *ddl.TableBuilder) error {
		tb.JunctionTable()
		tb.Bigint("pet_id").References(pets)
		tb.Bigint("tag_id").References(tags)
		return nil
	})
	if err != nil {
		t.Fatalf("valid junction table failed: %v", err)
	}

	table := plan.Schema.Tables["pet_tags"]
	if !table.IsJunctionTable {
		t.Error("expected IsJunctionTable to be true")
	}
}

func TestAddEmptyTable_JunctionTable_TooFewReferences(t *testing.T) {
	plan := NewPlan()

	// Create a referenced table
	_, err := plan.AddEmptyTable("pets", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable pets failed: %v", err)
	}

	pets, _ := plan.Table("pets")

	// Invalid: junction table with only 1 reference
	_, err = plan.AddEmptyTable("bad_junction", func(tb *ddl.TableBuilder) error {
		tb.JunctionTable()
		tb.Bigint("pet_id").References(pets)
		tb.Bigint("other_id") // No reference!
		return nil
	})
	if err == nil {
		t.Error("expected error for junction table with <2 references")
	}
	if err != nil && !strings.Contains(err.Error(), "2") {
		t.Errorf("expected error to mention '2', got: %v", err)
	}
}

func TestAddEmptyTable_JunctionTable_TooManyReferences(t *testing.T) {
	plan := NewPlan()

	// Create referenced tables
	plan.AddEmptyTable("pets", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		return nil
	})
	plan.AddEmptyTable("tags", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		return nil
	})
	plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		return nil
	})

	pets, _ := plan.Table("pets")
	tags, _ := plan.Table("tags")
	users, _ := plan.Table("users")

	// Invalid: junction table with 3 references
	_, err := plan.AddEmptyTable("bad_junction", func(tb *ddl.TableBuilder) error {
		tb.JunctionTable()
		tb.Bigint("pet_id").References(pets)
		tb.Bigint("tag_id").References(tags)
		tb.Bigint("user_id").References(users)
		return nil
	})
	if err == nil {
		t.Error("expected error for junction table with >2 references")
	}
}

func TestAddEmptyTable_JunctionTable_NoReferences(t *testing.T) {
	plan := NewPlan()

	// Invalid: junction table with no references
	_, err := plan.AddEmptyTable("bad_junction", func(tb *ddl.TableBuilder) error {
		tb.JunctionTable()
		tb.Bigint("col1")
		tb.Bigint("col2")
		return nil
	})
	if err == nil {
		t.Error("expected error for junction table with 0 references")
	}
}

// =============================================================================
// ValidateMigrationName Tests (TDD - these should fail initially)
// =============================================================================

func TestValidateMigrationName_ValidNames(t *testing.T) {
	validNames := []string{
		"20260111170656_create_users",
		"20260111170656_x", // minimum valid (14 digits + underscore + 1 char)
		"99991231235959_some_migration_name",
		"00000000000000_a",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateMigrationName(name)
			if err != nil {
				t.Errorf("ValidateMigrationName(%q) should pass, got error: %v", name, err)
			}
		})
	}
}

func TestValidateMigrationName_InvalidNames(t *testing.T) {
	testCases := []struct {
		name        string
		errContains string
	}{
		{"create_users", "timestamp"},          // no timestamp
		{"2026011117065_create", "timestamp"},  // 13 digits (too short)
		{"20260111170656create", "underscore"}, // missing underscore after timestamp
		{"abcd0111170656_create", "timestamp"}, // non-digits in timestamp
		{"20260111170656_", "empty"},           // empty name after underscore
		{"", "short"},                          // empty string
		{"12345678901234", "short"},            // just timestamp, no underscore or name
		{"1234567890123_x", "timestamp"},       // 13 digits
		{"123456789012345_x", "timestamp"},     // position 14 is digit, not underscore
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMigrationName(tc.name)
			if err == nil {
				t.Errorf("ValidateMigrationName(%q) should fail", tc.name)
				return
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.errContains) {
				t.Errorf("ValidateMigrationName(%q) error should contain %q, got: %v", tc.name, tc.errContains, err)
			}
		})
	}
}

// =============================================================================
// AddTable Tests
// =============================================================================

func TestAddTable_UpdatesSchema(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	if _, ok := plan.Schema.Tables["users"]; !ok {
		t.Error("expected users table to exist in Schema.Tables")
	}
}

func TestAddTable_WithColumns(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.Integer("age").Nullable()
		tb.Bool("active").Default(true)
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	table, ok := plan.Schema.Tables["users"]
	if !ok {
		t.Fatal("expected users table to exist")
	}

	// Find each column and verify its properties
	columnsByName := make(map[string]ddl.ColumnDefinition)
	for _, col := range table.Columns {
		columnsByName[col.Name] = col
	}

	// Check "name" column
	nameCol, ok := columnsByName["name"]
	if !ok {
		t.Error("expected 'name' column to exist")
	} else {
		if nameCol.Type != ddl.StringType {
			t.Errorf("expected name column type to be %q, got %q", ddl.StringType, nameCol.Type)
		}
		if nameCol.Nullable {
			t.Error("expected name column to not be nullable")
		}
	}

	// Check "age" column
	ageCol, ok := columnsByName["age"]
	if !ok {
		t.Error("expected 'age' column to exist")
	} else {
		if ageCol.Type != ddl.IntegerType {
			t.Errorf("expected age column type to be %q, got %q", ddl.IntegerType, ageCol.Type)
		}
		if !ageCol.Nullable {
			t.Error("expected age column to be nullable")
		}
	}

	// Check "active" column
	activeCol, ok := columnsByName["active"]
	if !ok {
		t.Error("expected 'active' column to exist")
	} else {
		if activeCol.Type != ddl.BooleanType {
			t.Errorf("expected active column type to be %q, got %q", ddl.BooleanType, activeCol.Type)
		}
		if activeCol.Default == nil || *activeCol.Default != "true" {
			var got string
			if activeCol.Default != nil {
				got = *activeCol.Default
			}
			t.Errorf("expected active column default to be %q, got %q", "true", got)
		}
	}
}

func TestAddTable_WithIndexes(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		emailCol := tb.String("email").Unique()
		statusCol := tb.String("status").Indexed()
		// Composite index
		tb.AddIndex(emailCol.Col(), statusCol.Col())
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	table, ok := plan.Schema.Tables["users"]
	if !ok {
		t.Fatal("expected users table to exist")
	}

	// Should have at least 3 indexes: unique on email, regular on status, composite
	if len(table.Indexes) < 3 {
		t.Errorf("expected at least 3 indexes, got %d", len(table.Indexes))
	}

	// Verify unique index on email exists
	foundEmailUnique := false
	for _, idx := range table.Indexes {
		if len(idx.Columns) == 1 && idx.Columns[0] == "email" && idx.Unique {
			foundEmailUnique = true
			break
		}
	}
	if !foundEmailUnique {
		t.Error("expected unique index on email column")
	}

	// Verify composite index exists
	foundComposite := false
	for _, idx := range table.Indexes {
		if len(idx.Columns) == 2 && idx.Columns[0] == "email" && idx.Columns[1] == "status" {
			foundComposite = true
			break
		}
	}
	if !foundComposite {
		t.Error("expected composite index on (email, status)")
	}
}

func TestAddTable_AppendsMigration(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	if len(plan.Migrations) != 1 {
		t.Errorf("expected 1 migration, got %d", len(plan.Migrations))
	}
}

func TestAddTable_MigrationName(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	if len(plan.Migrations) == 0 {
		t.Fatal("expected at least 1 migration")
	}

	// Migration name should be timestamped: YYYYMMDDHHMMSS_create_users
	migrationName := plan.Migrations[0].Name
	if !strings.Contains(migrationName, "_create_users") {
		t.Errorf("expected migration name to contain '_create_users', got %q", migrationName)
	}
	if len(migrationName) < 16 {
		t.Errorf("expected migration name to have timestamp prefix, got %q", migrationName)
	}
}

func TestAddTable_DuplicateError(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	// First add should succeed
	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("first AddTable failed: %v", err)
	}

	// Second add of same table should fail
	_, err = plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err == nil {
		t.Error("expected error when adding duplicate table, got nil")
	}
}

// =============================================================================
// AddTable (with default columns) Tests
// =============================================================================

func TestAddTableWithDefaults_IncludesDefaultColumns(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		// Add one custom column
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	table, ok := plan.Schema.Tables["users"]
	if !ok {
		t.Fatal("expected users table to exist")
	}

	// Build map for easier lookup
	columnsByName := make(map[string]ddl.ColumnDefinition)
	for _, col := range table.Columns {
		columnsByName[col.Name] = col
	}

	// Check default columns exist
	defaultCols := []string{"id", "public_id", "created_at", "deleted_at", "updated_at"}
	for _, colName := range defaultCols {
		if _, ok := columnsByName[colName]; !ok {
			t.Errorf("expected default column %q to exist", colName)
		}
	}

	// Check id column properties
	idCol := columnsByName["id"]
	if idCol.Type != ddl.BigintType {
		t.Errorf("expected id type %q, got %q", ddl.BigintType, idCol.Type)
	}
	if !idCol.PrimaryKey {
		t.Error("expected id to be primary key")
	}
	if idCol.Nullable {
		t.Error("expected id to not be nullable")
	}

	// Check public_id column properties
	publicIdCol := columnsByName["public_id"]
	if publicIdCol.Type != ddl.StringType {
		t.Errorf("expected public_id type %q, got %q", ddl.StringType, publicIdCol.Type)
	}
	if !publicIdCol.Unique {
		t.Error("expected public_id to be unique")
	}
	if publicIdCol.Nullable {
		t.Error("expected public_id to not be nullable")
	}

	// Check datetime columns
	for _, colName := range []string{"created_at", "deleted_at", "updated_at"} {
		col := columnsByName[colName]
		if col.Type != ddl.DatetimeType {
			t.Errorf("expected %s type %q, got %q", colName, ddl.DatetimeType, col.Type)
		}
		if col.Nullable {
			t.Errorf("expected %s to not be nullable", colName)
		}
	}

	// Check custom column was also added
	if _, ok := columnsByName["email"]; !ok {
		t.Error("expected custom 'email' column to exist")
	}
}

func TestAddTableWithDefaults_IncludesDefaultIndexes(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]

	// Should have at least 2 default indexes: unique on id, unique on public_id
	if len(table.Indexes) < 2 {
		t.Errorf("expected at least 2 default indexes, got %d", len(table.Indexes))
	}

	// Check for unique index on id
	foundIdIndex := false
	for _, idx := range table.Indexes {
		if len(idx.Columns) == 1 && idx.Columns[0] == "id" && idx.Unique {
			foundIdIndex = true
			break
		}
	}
	if !foundIdIndex {
		t.Error("expected unique index on id column")
	}

	// Check for unique index on public_id
	foundPublicIdIndex := false
	for _, idx := range table.Indexes {
		if len(idx.Columns) == 1 && idx.Columns[0] == "public_id" && idx.Unique {
			foundPublicIdIndex = true
			break
		}
	}
	if !foundPublicIdIndex {
		t.Error("expected unique index on public_id column")
	}
}

func TestAddTableWithDefaults_AdditionalColumnsAppended(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email").Unique()
		tb.Integer("age").Nullable()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]

	// Should have 5 default columns + 2 custom columns = 7 total
	if len(table.Columns) != 7 {
		t.Errorf("expected 7 columns (5 default + 2 custom), got %d", len(table.Columns))
	}

	// Verify order: default columns come first, then custom
	if table.Columns[0].Name != "id" {
		t.Errorf("expected first column to be 'id', got %q", table.Columns[0].Name)
	}
	if table.Columns[5].Name != "email" {
		t.Errorf("expected 6th column to be 'email', got %q", table.Columns[5].Name)
	}
	if table.Columns[6].Name != "age" {
		t.Errorf("expected 7th column to be 'age', got %q", table.Columns[6].Name)
	}
}

func TestAddTableWithDefaults_UpdatesSchema(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	if _, ok := plan.Schema.Tables["users"]; !ok {
		t.Error("expected users table to exist in Schema.Tables")
	}
}

func TestAddTableWithDefaults_AppendsMigration(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	if len(plan.Migrations) != 1 {
		t.Errorf("expected 1 migration, got %d", len(plan.Migrations))
	}

	// Migration name should be timestamped: YYYYMMDDHHMMSS_create_users
	migrationName := plan.Migrations[0].Name
	if !strings.Contains(migrationName, "_create_users") {
		t.Errorf("expected migration name to contain '_create_users', got %q", migrationName)
	}
	if len(migrationName) < 16 {
		t.Errorf("expected migration name to have timestamp prefix, got %q", migrationName)
	}
}

func TestAddTableWithDefaults_DuplicateError(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	// First add should succeed
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		return nil
	})
	if err != nil {
		t.Fatalf("first AddTable failed: %v", err)
	}

	// Second add of same table should fail
	_, err = plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		return nil
	})
	if err == nil {
		t.Error("expected error when adding duplicate table, got nil")
	}
}

func TestAddTableWithDefaults_GeneratesSQLForAllDatabases(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	migration := plan.Migrations[0]

	// All three SQL dialects should be non-empty
	if migration.Instructions.Postgres == "" {
		t.Error("expected Postgres SQL to be generated")
	}
	if migration.Instructions.MySQL == "" {
		t.Error("expected MySQL SQL to be generated")
	}
	if migration.Instructions.Sqlite == "" {
		t.Error("expected SQLite SQL to be generated")
	}
}

// =============================================================================
// UpdateTable Tests
// =============================================================================

func TestUpdateTable_ModifiesSchema(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	// First create the table
	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	// Now update it
	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]
	found := false
	for _, col := range table.Columns {
		if col.Name == "email" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'email' column to be added to schema")
	}
}

func TestUpdateTable_AddColumn(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.Integer("age").Nullable()
		ab.Bool("active").Default(false)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]
	columnsByName := make(map[string]ddl.ColumnDefinition)
	for _, col := range table.Columns {
		columnsByName[col.Name] = col
	}

	// Verify age column
	ageCol, ok := columnsByName["age"]
	if !ok {
		t.Error("expected 'age' column to exist")
	} else {
		if ageCol.Type != ddl.IntegerType {
			t.Errorf("expected age type %q, got %q", ddl.IntegerType, ageCol.Type)
		}
		if !ageCol.Nullable {
			t.Error("expected age to be nullable")
		}
	}

	// Verify active column
	activeCol, ok := columnsByName["active"]
	if !ok {
		t.Error("expected 'active' column to exist")
	} else {
		if activeCol.Type != ddl.BooleanType {
			t.Errorf("expected active type %q, got %q", ddl.BooleanType, activeCol.Type)
		}
		if activeCol.Default == nil || *activeCol.Default != "false" {
			var got string
			if activeCol.Default != nil {
				got = *activeCol.Default
			}
			t.Errorf("expected active default %q, got %q", "false", got)
		}
	}
}

func TestUpdateTable_DropColumn(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.DropColumn("email")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]
	for _, col := range table.Columns {
		if col.Name == "email" {
			t.Error("expected 'email' column to be removed from schema")
		}
	}
}

func TestUpdateTable_RenameColumn(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.RenameColumn("name", "full_name")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]
	foundOld := false
	foundNew := false
	for _, col := range table.Columns {
		if col.Name == "name" {
			foundOld = true
		}
		if col.Name == "full_name" {
			foundNew = true
		}
	}
	if foundOld {
		t.Error("expected 'name' column to be renamed")
	}
	if !foundNew {
		t.Error("expected 'full_name' column to exist after rename")
	}
}

func TestUpdateTable_AddIndex(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email")
		tb.String("status")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	initialIndexCount := len(plan.Schema.Tables["users"].Indexes)

	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		emailCol, _ := ab.ExistingColumn("email")
		statusCol, _ := ab.ExistingColumn("status")
		ab.AddUniqueIndex(emailCol, statusCol)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]
	if len(table.Indexes) <= initialIndexCount {
		t.Error("expected new index to be added")
	}

	// Check for the composite unique index
	found := false
	for _, idx := range table.Indexes {
		if len(idx.Columns) == 2 && idx.Columns[0] == "email" && idx.Columns[1] == "status" && idx.Unique {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected unique composite index on (email, status)")
	}
}

func TestUpdateTable_DropIndex(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email").Indexed()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	indexName := ddl.GenerateIndexName("users", []string{"email"})

	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.DropIndex(indexName)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	table := plan.Schema.Tables["users"]
	for _, idx := range table.Indexes {
		if idx.Name == indexName {
			t.Errorf("expected index %q to be removed", indexName)
		}
	}
}

func TestUpdateTable_AppendsMigration(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	initialMigrationCount := len(plan.Migrations)

	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	if len(plan.Migrations) != initialMigrationCount+1 {
		t.Errorf("expected %d migrations, got %d", initialMigrationCount+1, len(plan.Migrations))
	}
}

func TestUpdateTable_NotFoundError(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	err := plan.UpdateTable("nonexistent", func(ab *ddl.AlterTableBuilder) error {
		ab.String("email")
		return nil
	})
	if err == nil {
		t.Error("expected error when updating nonexistent table, got nil")
	}
}

// =============================================================================
// DropTable Tests
// =============================================================================

func TestDropTable_RemovesFromSchema(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	_, err = plan.DropTable("users")
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	if _, ok := plan.Schema.Tables["users"]; ok {
		t.Error("expected users table to be removed from Schema.Tables")
	}
}

func TestDropTable_AppendsMigration(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	initialMigrationCount := len(plan.Migrations)

	_, err = plan.DropTable("users")
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	if len(plan.Migrations) != initialMigrationCount+1 {
		t.Errorf("expected %d migrations, got %d", initialMigrationCount+1, len(plan.Migrations))
		return // Avoid panic on empty slice
	}

	lastMigration := plan.Migrations[len(plan.Migrations)-1]
	expectedName := "drop_users_table"
	if lastMigration.Name != expectedName {
		t.Errorf("expected migration name %q, got %q", expectedName, lastMigration.Name)
	}
}

func TestDropTable_NotFoundError(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	_, err := plan.DropTable("nonexistent")
	if err == nil {
		t.Error("expected error when dropping nonexistent table, got nil")
	}
}

// =============================================================================
// Migration Accumulation Tests
// =============================================================================

func TestMigrationPlan_MultipleMigrations(t *testing.T) {
	plan := &MigrationPlan{
		Schema: Schema{
			Name:   "test",
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}

	// 1. Create table
	_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	// 2. Update table
	err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
		ab.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	// 3. Drop table
	_, err = plan.DropTable("users")
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	// Verify 3 migrations in order
	if len(plan.Migrations) != 3 {
		t.Errorf("expected 3 migrations, got %d", len(plan.Migrations))
	}

	// Migration name should be timestamped: YYYYMMDDHHMMSS_create_users
	if !strings.Contains(plan.Migrations[0].Name, "_create_users") {
		t.Errorf("expected first migration to contain '_create_users', got %q", plan.Migrations[0].Name)
	}
	if plan.Migrations[1].Name != "alter_users_table" {
		t.Errorf("expected second migration to be 'alter_users_table', got %q", plan.Migrations[1].Name)
	}
	if plan.Migrations[2].Name != "drop_users_table" {
		t.Errorf("expected third migration to be 'drop_users_table', got %q", plan.Migrations[2].Name)
	}
}

func TestMigrationPlan_Idempotent(t *testing.T) {
	// Running same operations on fresh plan produces same migrations
	createPlan := func() *MigrationPlan {
		plan := &MigrationPlan{
			Schema: Schema{
				Name:   "test",
				Tables: make(map[string]ddl.Table),
			},
			Migrations: []Migration{},
		}
		return plan
	}

	buildMigrations := func(plan *MigrationPlan) error {
		_, err := plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
			tb.String("name")
			tb.Integer("age").Nullable()
			return nil
		})
		if err != nil {
			return err
		}

		err = plan.UpdateTable("users", func(ab *ddl.AlterTableBuilder) error {
			ab.String("email").Unique()
			return nil
		})
		return err
	}

	plan1 := createPlan()
	if err := buildMigrations(plan1); err != nil {
		t.Fatalf("buildMigrations(plan1) failed: %v", err)
	}

	plan2 := createPlan()
	if err := buildMigrations(plan2); err != nil {
		t.Fatalf("buildMigrations(plan2) failed: %v", err)
	}

	// Verify same number of migrations
	if len(plan1.Migrations) != len(plan2.Migrations) {
		t.Errorf("expected same number of migrations, got %d vs %d", len(plan1.Migrations), len(plan2.Migrations))
	}

	// Verify migration names have the same suffix pattern (timestamps will differ)
	// Migration names are now timestamped: YYYYMMDDHHMMSS_action_tablename
	for i := range plan1.Migrations {
		// Extract the suffix after the timestamp (position 15 onwards: after YYYYMMDDHHMMSS_)
		name1 := plan1.Migrations[i].Name
		name2 := plan2.Migrations[i].Name

		if len(name1) < 15 || len(name2) < 15 {
			t.Errorf("migration %d names too short: %q vs %q", i, name1, name2)
			continue
		}

		suffix1 := name1[15:] // After timestamp and underscore
		suffix2 := name2[15:]

		if suffix1 != suffix2 {
			t.Errorf("migration %d name suffixes differ: %q vs %q", i, suffix1, suffix2)
		}
	}

	// Verify same schema structure
	if len(plan1.Schema.Tables) != len(plan2.Schema.Tables) {
		t.Errorf("expected same number of tables, got %d vs %d", len(plan1.Schema.Tables), len(plan2.Schema.Tables))
	}

	for tableName, table1 := range plan1.Schema.Tables {
		table2, ok := plan2.Schema.Tables[tableName]
		if !ok {
			t.Errorf("table %q exists in plan1 but not plan2", tableName)
			continue
		}
		if len(table1.Columns) != len(table2.Columns) {
			t.Errorf("table %q has different column counts: %d vs %d", tableName, len(table1.Columns), len(table2.Columns))
		}
	}
}
