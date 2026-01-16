//go:build integration

package migrate

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	_ "modernc.org/sqlite"
)

// connectSQLite opens an in-memory SQLite database and returns a connection.
// Uses the pure-Go modernc.org/sqlite driver (no CGO required).
func connectSQLite(t *testing.T) *sql.DB {
	t.Helper()

	// Open an in-memory database using the modernc.org/sqlite driver
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skipf("SQLite unavailable: %v", err)
		return nil
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("SQLite unavailable: %v", err)
		return nil
	}

	return db
}

// SQLiteColumnInfo holds column metadata from PRAGMA table_info
type SQLiteColumnInfo struct {
	CID        int
	Name       string
	Type       string
	NotNull    bool
	DefaultVal *string
	PK         int
}

// introspectSQLiteColumns queries PRAGMA table_info for column metadata
func introspectSQLiteColumns(t *testing.T, db *sql.DB, tableName string) []SQLiteColumnInfo {
	t.Helper()

	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)

	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns []SQLiteColumnInfo
	for rows.Next() {
		var col SQLiteColumnInfo
		var notNull int
		var defaultVal *string

		err := rows.Scan(&col.CID, &col.Name, &col.Type, &notNull, &defaultVal, &col.PK)
		if err != nil {
			t.Fatalf("failed to scan column: %v", err)
		}

		col.NotNull = notNull == 1
		col.DefaultVal = defaultVal
		columns = append(columns, col)
	}

	return columns
}

// sqliteTableExists checks if a table exists in the database
func sqliteTableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	var name string
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
	err := db.QueryRow(query, tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	return true
}

// findSQLiteColumn finds a column by name in a list of columns
func findSQLiteColumn(columns []SQLiteColumnInfo, name string) *SQLiteColumnInfo {
	for _, col := range columns {
		if col.Name == name {
			return &col
		}
	}
	return nil
}

// =============================================================================
// Connection Test
// =============================================================================

func TestSQLiteIntegration_Connection(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	var result int
	err := db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
	if result != 1 {
		t.Fatalf("unexpected result: got %d, want 1", result)
	}

	t.Log("SQLite connection successful")
}

// =============================================================================
// CREATE TABLE Integration Tests
// =============================================================================

func TestSQLiteIntegration_CreateTable_BasicColumns(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_basic_columns"

	// Create migration plan
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Integer("age")
		tb.String("name")
		tb.Bool("active")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	// Execute the SQL
	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify columns
	columns := introspectSQLiteColumns(t, db, tableName)

	ageCol := findSQLiteColumn(columns, "age")
	if ageCol == nil {
		t.Error("expected 'age' column to exist")
	} else if ageCol.Type != "INTEGER" {
		t.Errorf("expected age type 'INTEGER', got '%s'", ageCol.Type)
	}

	nameCol := findSQLiteColumn(columns, "name")
	if nameCol == nil {
		t.Error("expected 'name' column to exist")
	} else if nameCol.Type != "TEXT" {
		t.Errorf("expected name type 'TEXT', got '%s'", nameCol.Type)
	}

	// Boolean is INTEGER in SQLite
	activeCol := findSQLiteColumn(columns, "active")
	if activeCol == nil {
		t.Error("expected 'active' column to exist")
	} else if activeCol.Type != "INTEGER" {
		t.Errorf("expected active type 'INTEGER', got '%s'", activeCol.Type)
	}
}

func TestSQLiteIntegration_CreateTable_AllColumnTypes(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_all_types"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Integer("int_col")
		tb.Bigint("bigint_col")
		tb.String("string_col")
		tb.Text("text_col")
		tb.Bool("bool_col")
		tb.Decimal("decimal_col", 10, 2)
		tb.Float("float_col")
		tb.Datetime("datetime_col")
		tb.Timestamp("timestamp_col")
		tb.Binary("binary_col")
		tb.JSON("json_col")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	columns := introspectSQLiteColumns(t, db, tableName)

	// Verify each type (SQLite type affinity)
	typeTests := []struct {
		name     string
		expected string
	}{
		{"int_col", "INTEGER"},
		{"bigint_col", "INTEGER"}, // BIGINT → INTEGER
		{"string_col", "TEXT"},    // VARCHAR → TEXT
		{"text_col", "TEXT"},
		{"bool_col", "INTEGER"}, // BOOLEAN → INTEGER
		{"decimal_col", "REAL"}, // DECIMAL → REAL
		{"float_col", "REAL"},
		{"datetime_col", "TEXT"},  // DATETIME → TEXT
		{"timestamp_col", "TEXT"}, // TIMESTAMP → TEXT
		{"binary_col", "BLOB"},
		{"json_col", "TEXT"}, // JSON → TEXT
	}

	for _, tt := range typeTests {
		col := findSQLiteColumn(columns, tt.name)
		if col == nil {
			t.Errorf("expected '%s' column to exist", tt.name)
		} else if col.Type != tt.expected {
			t.Errorf("expected %s type '%s', got '%s'", tt.name, tt.expected, col.Type)
		}
	}
}

func TestSQLiteIntegration_CreateTable_Nullable(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_nullable"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("required_field")
		tb.String("optional_field").Nullable()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	columns := introspectSQLiteColumns(t, db, tableName)

	requiredCol := findSQLiteColumn(columns, "required_field")
	if requiredCol == nil {
		t.Error("expected 'required_field' column to exist")
	} else if !requiredCol.NotNull {
		t.Error("expected 'required_field' to be NOT NULL")
	}

	optionalCol := findSQLiteColumn(columns, "optional_field")
	if optionalCol == nil {
		t.Error("expected 'optional_field' column to exist")
	} else if optionalCol.NotNull {
		t.Error("expected 'optional_field' to be nullable")
	}
}

func TestSQLiteIntegration_CreateTable_Defaults(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_defaults"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("status").Default("pending")
		tb.Integer("count").Default(0)
		tb.Bool("active").Default(true)
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	columns := introspectSQLiteColumns(t, db, tableName)

	// Check that defaults are set
	statusCol := findSQLiteColumn(columns, "status")
	if statusCol == nil {
		t.Error("expected 'status' column to exist")
	} else if statusCol.DefaultVal == nil {
		t.Error("expected 'status' to have a default value")
	}

	countCol := findSQLiteColumn(columns, "count")
	if countCol == nil {
		t.Error("expected 'count' column to exist")
	} else if countCol.DefaultVal == nil {
		t.Error("expected 'count' to have a default value")
	}

	activeCol := findSQLiteColumn(columns, "active")
	if activeCol == nil {
		t.Error("expected 'active' column to exist")
	} else if activeCol.DefaultVal == nil {
		t.Error("expected 'active' to have a default value")
	}
}

// TestSQLiteIntegration_DefaultAppliedOnInsert verifies that DEFAULT values
// are actually used when a column is omitted from INSERT
func TestSQLiteIntegration_DefaultAppliedOnInsert(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_default_insert"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name").Nullable().Default("mydefault")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	t.Logf("Generated SQL: %s", sqlStr)

	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Insert without specifying the 'name' column
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id) VALUES (1)`, tableName))
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Query the value - use sql.NullString to handle potential NULL
	var name sql.NullString
	err = db.QueryRow(fmt.Sprintf(`SELECT name FROM "%s" WHERE id = 1`, tableName)).Scan(&name)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	t.Logf("Query result: Valid=%v, String=%q", name.Valid, name.String)

	if !name.Valid {
		t.Errorf("expected default value 'mydefault', but got NULL")
	} else if name.String != "mydefault" {
		t.Errorf("expected 'mydefault', got %q", name.String)
	}
}

func TestSQLiteIntegration_CreateTable_PrimaryKey(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_primary_key"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify primary key exists by trying to insert duplicate
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id, name) VALUES (1, 'test')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id, name) VALUES (1, 'test2')`, tableName))
	if err == nil {
		t.Error("expected duplicate key error, but insert succeeded")
	}
}

func TestSQLiteIntegration_CreateTable_Indexes(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_indexes"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email").Unique()
		tb.String("status").Indexed()
		firstName := tb.String("first_name")
		lastName := tb.String("last_name")
		tb.AddIndex(firstName.Col(), lastName.Col())
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify unique index on email works by trying to insert duplicates
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (email, status, first_name, last_name) VALUES ('test@test.com', 'active', 'John', 'Doe')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (email, status, first_name, last_name) VALUES ('test@test.com', 'active', 'Jane', 'Doe')`, tableName))
	if err == nil {
		t.Error("expected unique constraint violation, but insert succeeded")
	}
}

// =============================================================================
// ALTER TABLE Integration Tests
// =============================================================================

func TestSQLiteIntegration_AlterTable_AddColumn(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_alter_add"

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Add a column (nullable to avoid default requirement)
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.String("email").Nullable()
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify column exists
	columns := introspectSQLiteColumns(t, db, tableName)
	emailCol := findSQLiteColumn(columns, "email")
	if emailCol == nil {
		t.Error("expected 'email' column to exist after ALTER")
	}
}

func TestSQLiteIntegration_AlterTable_DropColumn(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_alter_drop"

	// Create initial table with two columns
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("legacy_field")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Drop a column (SQLite 3.35.0+)
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.DropColumn("legacy_field")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify column is gone
	columns := introspectSQLiteColumns(t, db, tableName)
	legacyCol := findSQLiteColumn(columns, "legacy_field")
	if legacyCol != nil {
		t.Error("expected 'legacy_field' column to be dropped")
	}
}

func TestSQLiteIntegration_AlterTable_RenameColumn(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_alter_rename"

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Rename column (SQLite 3.25.0+)
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.RenameColumn("name", "full_name")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify column is renamed
	columns := introspectSQLiteColumns(t, db, tableName)
	oldCol := findSQLiteColumn(columns, "name")
	newCol := findSQLiteColumn(columns, "full_name")

	if oldCol != nil {
		t.Error("expected 'name' column to be renamed")
	}
	if newCol == nil {
		t.Error("expected 'full_name' column to exist")
	}
}

func TestSQLiteIntegration_AlterTable_AddIndex(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_alter_index"

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Add index
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		emailCol, _ := ab.ExistingColumn("email")
		ab.AddUniqueIndex(emailCol)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify index works by inserting duplicates
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err == nil {
		t.Error("expected unique constraint violation after adding index")
	}
}

func TestSQLiteIntegration_AlterTable_DropIndex(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_alter_dropidx"

	// Create initial table with index
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email").Unique()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	indexName := ddl.GenerateIndexName(tableName, []string{"email"})

	// Drop index
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.DropIndex(indexName)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify index is gone - duplicates should now be allowed
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err != nil {
		t.Error("expected duplicate insert to succeed after dropping index")
	}
}

// =============================================================================
// DROP TABLE Integration Tests
// =============================================================================

func TestSQLiteIntegration_DropTable(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_drop_table"

	// Create table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify table exists
	if !sqliteTableExists(t, db, tableName) {
		t.Fatal("expected table to exist after creation")
	}

	// Drop table
	_, err = plan.DropTable(tableName)
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to drop table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify table is gone
	if sqliteTableExists(t, db, tableName) {
		t.Error("expected table to not exist after drop")
	}
}

// =============================================================================
// AddTable (with default columns) Integration Tests
// =============================================================================

func TestSQLiteIntegration_AddTable_DefaultColumns(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_default_columns"

	// Create table with default columns using AddTable
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify all default columns exist
	columns := introspectSQLiteColumns(t, db, tableName)

	// Check id column
	idCol := findSQLiteColumn(columns, "id")
	if idCol == nil {
		t.Error("expected 'id' column to exist")
	} else {
		if idCol.Type != "INTEGER" {
			t.Errorf("expected id type 'INTEGER', got '%s'", idCol.Type)
		}
		if idCol.PK == 0 {
			t.Error("expected id to be primary key")
		}
	}

	// Check public_id column
	publicIdCol := findSQLiteColumn(columns, "public_id")
	if publicIdCol == nil {
		t.Error("expected 'public_id' column to exist")
	} else {
		if publicIdCol.Type != "TEXT" {
			t.Errorf("expected public_id type 'TEXT', got '%s'", publicIdCol.Type)
		}
		if !publicIdCol.NotNull {
			t.Error("expected public_id to be NOT NULL")
		}
	}

	// Check datetime columns
	for _, colName := range []string{"created_at", "deleted_at", "updated_at"} {
		col := findSQLiteColumn(columns, colName)
		if col == nil {
			t.Errorf("expected '%s' column to exist", colName)
		} else {
			if col.Type != "TEXT" {
				t.Errorf("expected %s type 'TEXT', got '%s'", colName, col.Type)
			}
			if !col.NotNull {
				t.Errorf("expected %s to be NOT NULL", colName)
			}
		}
	}

	// Check custom column was also added
	emailCol := findSQLiteColumn(columns, "email")
	if emailCol == nil {
		t.Error("expected 'email' column to exist")
	}
}

func TestSQLiteIntegration_AddTable_PrimaryKeyConstraint(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_pk_constraint"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Insert first row
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Try to insert duplicate id - should fail
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'xyz789', '2024-01-01', '2024-01-01', '2024-01-01', 'test2')`, tableName))
	if err == nil {
		t.Error("expected duplicate id error, but insert succeeded")
	}
}

func TestSQLiteIntegration_AddTable_UniquePublicIdConstraint(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	tableName := "test_unique_public_id"

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.Sqlite
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Insert first row
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Try to insert duplicate public_id - should fail due to unique constraint
	_, err = db.Exec(fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (2, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test2')`, tableName))
	if err == nil {
		t.Error("expected duplicate public_id error, but insert succeeded")
	}
}
