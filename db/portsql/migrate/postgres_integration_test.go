//go:build integration

package migrate

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shipq/shipq/db/portsql/ddl"
)

// connectPostgres attempts to connect to PostgreSQL and returns a connection.
// Returns nil and skips the test if PostgreSQL is unavailable.
func connectPostgres(t *testing.T) *pgx.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect via Unix socket at /tmp/.s.PGSQL.5432, user "postgres", database "postgres"
	connString := "host=/tmp user=postgres database=postgres"
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		t.Skipf("PostgreSQL unavailable: %v. Please see the README for instructions about how to start all databases.", err)
		return nil
	}

	return conn
}

// ColumnInfo holds column metadata from information_schema
type ColumnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
	Default    *string
}

// IndexInfo holds index metadata from pg_indexes
type IndexInfo struct {
	Name    string
	Columns []string
	Unique  bool
}

// introspectPostgresColumns queries information_schema.columns for column metadata
func introspectPostgresColumns(t *testing.T, conn *pgx.Conn, tableName string) []ColumnInfo {
	t.Helper()

	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := conn.Query(context.Background(), query, tableName)
	if err != nil {
		t.Fatalf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		var defaultVal *string

		err := rows.Scan(&col.Name, &col.DataType, &isNullable, &defaultVal)
		if err != nil {
			t.Fatalf("failed to scan column: %v", err)
		}

		col.IsNullable = isNullable == "YES"
		col.Default = defaultVal
		columns = append(columns, col)
	}

	return columns
}

// introspectPostgresIndexes queries pg_indexes for index metadata
func introspectPostgresIndexes(t *testing.T, conn *pgx.Conn, tableName string) []IndexInfo {
	t.Helper()

	query := `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE tablename = $1
	`

	rows, err := conn.Query(context.Background(), query, tableName)
	if err != nil {
		t.Fatalf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var idx IndexInfo
		var indexDef string

		err := rows.Scan(&idx.Name, &indexDef)
		if err != nil {
			t.Fatalf("failed to scan index: %v", err)
		}

		// Parse UNIQUE from indexdef
		idx.Unique = len(indexDef) > 0 && indexDef[0:13] == "CREATE UNIQUE"
		indexes = append(indexes, idx)
	}

	return indexes
}

// tableExists checks if a table exists in the database
func tableExists(t *testing.T, conn *pgx.Conn, tableName string) bool {
	t.Helper()

	var exists bool
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = $1
		)
	`
	err := conn.QueryRow(context.Background(), query, tableName).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	return exists
}

// dropTableIfExists drops a table if it exists
func dropTableIfExists(t *testing.T, conn *pgx.Conn, tableName string) {
	t.Helper()
	_, err := conn.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS "%s" CASCADE`, tableName))
	if err != nil {
		t.Fatalf("failed to drop table: %v", err)
	}
}

// findColumn finds a column by name in a list of columns
func findColumn(columns []ColumnInfo, name string) *ColumnInfo {
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

func TestPostgresIntegration_Connection(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	var result int
	err := conn.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
	if result != 1 {
		t.Fatalf("unexpected result: got %d, want 1", result)
	}

	t.Log("PostgreSQL connection successful")
}

// =============================================================================
// CREATE TABLE Integration Tests
// =============================================================================

func TestPostgresIntegration_CreateTable_BasicColumns(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_basic_columns"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

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
	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	// Verify columns
	columns := introspectPostgresColumns(t, conn, tableName)

	ageCol := findColumn(columns, "age")
	if ageCol == nil {
		t.Error("expected 'age' column to exist")
	} else if ageCol.DataType != "integer" {
		t.Errorf("expected age type 'integer', got '%s'", ageCol.DataType)
	}

	nameCol := findColumn(columns, "name")
	if nameCol == nil {
		t.Error("expected 'name' column to exist")
	} else if nameCol.DataType != "character varying" {
		t.Errorf("expected name type 'character varying', got '%s'", nameCol.DataType)
	}

	activeCol := findColumn(columns, "active")
	if activeCol == nil {
		t.Error("expected 'active' column to exist")
	} else if activeCol.DataType != "boolean" {
		t.Errorf("expected active type 'boolean', got '%s'", activeCol.DataType)
	}
}

func TestPostgresIntegration_CreateTable_AllColumnTypes(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_all_types"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

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

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	columns := introspectPostgresColumns(t, conn, tableName)

	// Verify each type
	typeTests := []struct {
		name     string
		expected string
	}{
		{"int_col", "integer"},
		{"bigint_col", "bigint"},
		{"string_col", "character varying"},
		{"text_col", "text"},
		{"bool_col", "boolean"},
		{"decimal_col", "numeric"},
		{"float_col", "double precision"},
		{"datetime_col", "timestamp with time zone"},
		{"timestamp_col", "timestamp with time zone"},
		{"binary_col", "bytea"},
		{"json_col", "jsonb"},
	}

	for _, tt := range typeTests {
		col := findColumn(columns, tt.name)
		if col == nil {
			t.Errorf("expected '%s' column to exist", tt.name)
		} else if col.DataType != tt.expected {
			t.Errorf("expected %s type '%s', got '%s'", tt.name, tt.expected, col.DataType)
		}
	}
}

func TestPostgresIntegration_CreateTable_Nullable(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_nullable"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("required_field")
		tb.String("optional_field").Nullable()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	columns := introspectPostgresColumns(t, conn, tableName)

	requiredCol := findColumn(columns, "required_field")
	if requiredCol == nil {
		t.Error("expected 'required_field' column to exist")
	} else if requiredCol.IsNullable {
		t.Error("expected 'required_field' to be NOT NULL")
	}

	optionalCol := findColumn(columns, "optional_field")
	if optionalCol == nil {
		t.Error("expected 'optional_field' column to exist")
	} else if !optionalCol.IsNullable {
		t.Error("expected 'optional_field' to be nullable")
	}
}

func TestPostgresIntegration_CreateTable_Defaults(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_defaults"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

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

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	columns := introspectPostgresColumns(t, conn, tableName)

	// Check that defaults are set (PostgreSQL wraps string defaults in quotes)
	statusCol := findColumn(columns, "status")
	if statusCol == nil {
		t.Error("expected 'status' column to exist")
	} else if statusCol.Default == nil {
		t.Error("expected 'status' to have a default value")
	}

	countCol := findColumn(columns, "count")
	if countCol == nil {
		t.Error("expected 'count' column to exist")
	} else if countCol.Default == nil {
		t.Error("expected 'count' to have a default value")
	}

	activeCol := findColumn(columns, "active")
	if activeCol == nil {
		t.Error("expected 'active' column to exist")
	} else if activeCol.Default == nil {
		t.Error("expected 'active' to have a default value")
	}
}

func TestPostgresIntegration_CreateTable_PrimaryKey(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_primary_key"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	// Verify primary key exists by trying to insert duplicate
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (id, name) VALUES (1, 'test')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (id, name) VALUES (1, 'test2')`, tableName))
	if err == nil {
		t.Error("expected duplicate key error, but insert succeeded")
	}
}

func TestPostgresIntegration_CreateTable_Indexes(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_indexes"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

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

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	indexes := introspectPostgresIndexes(t, conn, tableName)

	// Should have at least 3 indexes: unique on email, regular on status, composite on first_name+last_name
	if len(indexes) < 3 {
		t.Errorf("expected at least 3 indexes, got %d", len(indexes))
	}

	// Verify unique index on email works by trying to insert duplicates
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (email, status, first_name, last_name) VALUES ('test@test.com', 'active', 'John', 'Doe')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (email, status, first_name, last_name) VALUES ('test@test.com', 'active', 'Jane', 'Doe')`, tableName))
	if err == nil {
		t.Error("expected unique constraint violation, but insert succeeded")
	}
}

// =============================================================================
// ALTER TABLE Integration Tests
// =============================================================================

func TestPostgresIntegration_AlterTable_AddColumn(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_alter_add"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sql)
	}

	// Add a column
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sql = plan.Migrations[1].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sql)
	}

	// Verify column exists
	columns := introspectPostgresColumns(t, conn, tableName)
	emailCol := findColumn(columns, "email")
	if emailCol == nil {
		t.Error("expected 'email' column to exist after ALTER")
	}
}

func TestPostgresIntegration_AlterTable_DropColumn(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_alter_drop"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

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

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sql)
	}

	// Drop a column
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.DropColumn("legacy_field")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sql = plan.Migrations[1].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sql)
	}

	// Verify column is gone
	columns := introspectPostgresColumns(t, conn, tableName)
	legacyCol := findColumn(columns, "legacy_field")
	if legacyCol != nil {
		t.Error("expected 'legacy_field' column to be dropped")
	}
}

func TestPostgresIntegration_AlterTable_RenameColumn(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_alter_rename"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sql)
	}

	// Rename column
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.RenameColumn("name", "full_name")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sql = plan.Migrations[1].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sql)
	}

	// Verify column is renamed
	columns := introspectPostgresColumns(t, conn, tableName)
	oldCol := findColumn(columns, "name")
	newCol := findColumn(columns, "full_name")

	if oldCol != nil {
		t.Error("expected 'name' column to be renamed")
	}
	if newCol == nil {
		t.Error("expected 'full_name' column to exist")
	}
}

func TestPostgresIntegration_AlterTable_AddIndex(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_alter_index"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sql)
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

	sql = plan.Migrations[1].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sql)
	}

	// Verify index works by inserting duplicates
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err == nil {
		t.Error("expected unique constraint violation after adding index")
	}
}

func TestPostgresIntegration_AlterTable_DropIndex(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_alter_dropidx"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	// Create initial table with index
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email").Unique()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sql)
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

	sql = plan.Migrations[1].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sql)
	}

	// Verify index is gone - duplicates should now be allowed
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (email) VALUES ('test@test.com')`, tableName))
	if err != nil {
		t.Error("expected duplicate insert to succeed after dropping index")
	}
}

// =============================================================================
// DROP TABLE Integration Tests
// =============================================================================

func TestPostgresIntegration_DropTable(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_drop_table"
	dropTableIfExists(t, conn, tableName)

	// Create table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sql)
	}

	// Verify table exists
	if !tableExists(t, conn, tableName) {
		t.Fatal("expected table to exist after creation")
	}

	// Drop table
	_, err = plan.DropTable(tableName)
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	sql = plan.Migrations[1].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to drop table: %v\nSQL: %s", err, sql)
	}

	// Verify table is gone
	if tableExists(t, conn, tableName) {
		t.Error("expected table to not exist after drop")
	}
}

// =============================================================================
// AddTable (with default columns) Integration Tests
// =============================================================================

func TestPostgresIntegration_AddTable_DefaultColumns(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_default_columns"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	// Create table with default columns using AddTable
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	// Verify all default columns exist
	columns := introspectPostgresColumns(t, conn, tableName)

	// Check id column
	idCol := findColumn(columns, "id")
	if idCol == nil {
		t.Error("expected 'id' column to exist")
	} else {
		if idCol.DataType != "bigint" {
			t.Errorf("expected id type 'bigint', got '%s'", idCol.DataType)
		}
		if idCol.IsNullable {
			t.Error("expected id to be NOT NULL")
		}
	}

	// Check public_id column
	publicIdCol := findColumn(columns, "public_id")
	if publicIdCol == nil {
		t.Error("expected 'public_id' column to exist")
	} else {
		if publicIdCol.DataType != "character varying" {
			t.Errorf("expected public_id type 'character varying', got '%s'", publicIdCol.DataType)
		}
		if publicIdCol.IsNullable {
			t.Error("expected public_id to be NOT NULL")
		}
	}

	// Check datetime columns
	for _, colName := range []string{"created_at", "deleted_at", "updated_at"} {
		col := findColumn(columns, colName)
		if col == nil {
			t.Errorf("expected '%s' column to exist", colName)
		} else {
			if col.DataType != "timestamp with time zone" {
				t.Errorf("expected %s type 'timestamp with time zone', got '%s'", colName, col.DataType)
			}
			if col.IsNullable {
				t.Errorf("expected %s to be NOT NULL", colName)
			}
		}
	}

	// Check custom column was also added
	emailCol := findColumn(columns, "email")
	if emailCol == nil {
		t.Error("expected 'email' column to exist")
	}
}

func TestPostgresIntegration_AddTable_PrimaryKeyConstraint(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_pk_constraint"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	// Insert first row
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Try to insert duplicate id - should fail
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'xyz789', '2024-01-01', '2024-01-01', '2024-01-01', 'test2')`, tableName))
	if err == nil {
		t.Error("expected duplicate id error, but insert succeeded")
	}
}

func TestPostgresIntegration_AddTable_UniquePublicIdConstraint(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	tableName := "test_unique_public_id"
	dropTableIfExists(t, conn, tableName)
	defer dropTableIfExists(t, conn, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sql := plan.Migrations[0].Instructions.Postgres
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}

	// Insert first row
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test')`, tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Try to insert duplicate public_id - should fail due to unique constraint
	_, err = conn.Exec(context.Background(), fmt.Sprintf(`INSERT INTO "%s" (id, public_id, created_at, deleted_at, updated_at, name) VALUES (2, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test2')`, tableName))
	if err == nil {
		t.Error("expected duplicate public_id error, but insert succeeded")
	}
}
