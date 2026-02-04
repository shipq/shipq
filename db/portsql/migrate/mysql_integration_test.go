//go:build integration

package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shipq/shipq/db/portsql/ddl"
)

// connectMySQL attempts to connect to MySQL and returns a database connection.
// Returns nil and skips the test if MySQL is unavailable.
func connectMySQL(t *testing.T) *sql.DB {
	t.Helper()

	// Find the MySQL socket path
	// The socket is at $PROJECT_ROOT/db/databases/.mysql-data/mysql.sock
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// Try to find it relative to the test file
		// We're in db/portsql/migrate, so go up 3 levels to project root
		cwd, err := os.Getwd()
		if err != nil {
			t.Skipf("MySQL unavailable: cannot determine working directory: %v", err)
			return nil
		}
		projectRoot = filepath.Join(cwd, "..", "..", "..")
	}

	socketPath := filepath.Join(projectRoot, "db", "databases", ".mysql-data", "mysql.sock")

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skipf("MySQL unavailable: socket not found at %s. Please see the README for instructions about how to start all databases.", socketPath)
		return nil
	}

	// Connect via Unix socket (no database specified initially)
	// DSN format: user:password@unix(/path/to/socket)/database
	dsn := "root@unix(" + socketPath + ")/?multiStatements=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("MySQL unavailable: %v. Please see the README for instructions about how to start all databases.", err)
		return nil
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: %v. Please see the README for instructions about how to start all databases.", err)
		return nil
	}

	// Create test database if it doesn't exist and use it
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS test")
	if err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: cannot create test database: %v", err)
		return nil
	}

	_, err = db.Exec("USE test")
	if err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: cannot use test database: %v", err)
		return nil
	}

	return db
}

// MySQLColumnInfo holds column metadata from information_schema
type MySQLColumnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
	Default    *string
	Extra      *string // Contains "auto_increment" for autoincrement columns
}

// introspectMySQLColumns queries information_schema.columns for column metadata
func introspectMySQLColumns(t *testing.T, db *sql.DB, tableName string) []MySQLColumnInfo {
	t.Helper()

	query := `
		SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := db.Query(query, tableName)
	if err != nil {
		t.Fatalf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns []MySQLColumnInfo
	for rows.Next() {
		var col MySQLColumnInfo
		var isNullable string
		var defaultVal *string
		var extra *string

		err := rows.Scan(&col.Name, &col.DataType, &isNullable, &defaultVal, &extra)
		if err != nil {
			t.Fatalf("failed to scan column: %v", err)
		}

		col.IsNullable = isNullable == "YES"
		col.Default = defaultVal
		col.Extra = extra
		columns = append(columns, col)
	}

	return columns
}

// mysqlTableExists checks if a table exists in the database
func mysqlTableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	var exists int
	query := `
		SELECT COUNT(*) FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`
	err := db.QueryRow(query, tableName).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	return exists > 0
}

// dropMySQLTableIfExists drops a table if it exists
func dropMySQLTableIfExists(t *testing.T, db *sql.DB, tableName string) {
	t.Helper()
	_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName))
	if err != nil {
		t.Fatalf("failed to drop table: %v", err)
	}
}

// findMySQLColumn finds a column by name in a list of columns
func findMySQLColumn(columns []MySQLColumnInfo, name string) *MySQLColumnInfo {
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

func TestMySQLIntegration_Connection(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	var result int
	err := db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}
	if result != 1 {
		t.Fatalf("unexpected result: got %d, want 1", result)
	}

	t.Log("MySQL connection successful")
}

// =============================================================================
// CREATE TABLE Integration Tests
// =============================================================================

func TestMySQLIntegration_CreateTable_BasicColumns(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_basic_columns"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

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
	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify columns
	columns := introspectMySQLColumns(t, db, tableName)

	ageCol := findMySQLColumn(columns, "age")
	if ageCol == nil {
		t.Error("expected 'age' column to exist")
	} else if ageCol.DataType != "int" {
		t.Errorf("expected age type 'int', got '%s'", ageCol.DataType)
	}

	nameCol := findMySQLColumn(columns, "name")
	if nameCol == nil {
		t.Error("expected 'name' column to exist")
	} else if nameCol.DataType != "varchar" {
		t.Errorf("expected name type 'varchar', got '%s'", nameCol.DataType)
	}

	activeCol := findMySQLColumn(columns, "active")
	if activeCol == nil {
		t.Error("expected 'active' column to exist")
	} else if activeCol.DataType != "tinyint" {
		t.Errorf("expected active type 'tinyint', got '%s'", activeCol.DataType)
	}
}

func TestMySQLIntegration_CreateTable_AllColumnTypes(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_all_types"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

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
		tb.Timestamp("timestamp_col").Nullable() // TIMESTAMP needs NULL to avoid auto-update
		tb.Binary("binary_col")
		tb.JSON("json_col")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	columns := introspectMySQLColumns(t, db, tableName)

	// Verify each type
	typeTests := []struct {
		name     string
		expected string
	}{
		{"int_col", "int"},
		{"bigint_col", "bigint"},
		{"string_col", "varchar"},
		{"text_col", "text"},
		{"bool_col", "tinyint"},
		{"decimal_col", "decimal"},
		{"float_col", "double"},
		{"datetime_col", "datetime"},
		{"timestamp_col", "timestamp"},
		{"binary_col", "blob"},
		{"json_col", "json"},
	}

	for _, tt := range typeTests {
		col := findMySQLColumn(columns, tt.name)
		if col == nil {
			t.Errorf("expected '%s' column to exist", tt.name)
		} else if col.DataType != tt.expected {
			t.Errorf("expected %s type '%s', got '%s'", tt.name, tt.expected, col.DataType)
		}
	}
}

func TestMySQLIntegration_CreateTable_Nullable(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_nullable"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("required_field")
		tb.String("optional_field").Nullable()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	columns := introspectMySQLColumns(t, db, tableName)

	requiredCol := findMySQLColumn(columns, "required_field")
	if requiredCol == nil {
		t.Error("expected 'required_field' column to exist")
	} else if requiredCol.IsNullable {
		t.Error("expected 'required_field' to be NOT NULL")
	}

	optionalCol := findMySQLColumn(columns, "optional_field")
	if optionalCol == nil {
		t.Error("expected 'optional_field' column to exist")
	} else if !optionalCol.IsNullable {
		t.Error("expected 'optional_field' to be nullable")
	}
}

func TestMySQLIntegration_CreateTable_Defaults(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_defaults"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

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

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	columns := introspectMySQLColumns(t, db, tableName)

	// Check that defaults are set
	statusCol := findMySQLColumn(columns, "status")
	if statusCol == nil {
		t.Error("expected 'status' column to exist")
	} else if statusCol.Default == nil {
		t.Error("expected 'status' to have a default value")
	}

	countCol := findMySQLColumn(columns, "count")
	if countCol == nil {
		t.Error("expected 'count' column to exist")
	} else if countCol.Default == nil {
		t.Error("expected 'count' to have a default value")
	}

	activeCol := findMySQLColumn(columns, "active")
	if activeCol == nil {
		t.Error("expected 'active' column to exist")
	} else if activeCol.Default == nil {
		t.Error("expected 'active' to have a default value")
	}
}

func TestMySQLIntegration_CreateTable_PrimaryKey(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_primary_key"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify primary key exists by trying to insert duplicate
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (id, name) VALUES (1, 'test')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (id, name) VALUES (1, 'test2')", tableName))
	if err == nil {
		t.Error("expected duplicate key error, but insert succeeded")
	}
}

func TestMySQLIntegration_CreateTable_Indexes(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_indexes"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

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

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify unique index on email works by trying to insert duplicates
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (email, status, first_name, last_name) VALUES ('test@test.com', 'active', 'John', 'Doe')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (email, status, first_name, last_name) VALUES ('test@test.com', 'active', 'Jane', 'Doe')", tableName))
	if err == nil {
		t.Error("expected unique constraint violation, but insert succeeded")
	}
}

// =============================================================================
// ALTER TABLE Integration Tests
// =============================================================================

func TestMySQLIntegration_AlterTable_AddColumn(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_alter_add"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Add a column
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify column exists
	columns := introspectMySQLColumns(t, db, tableName)
	emailCol := findMySQLColumn(columns, "email")
	if emailCol == nil {
		t.Error("expected 'email' column to exist after ALTER")
	}
}

func TestMySQLIntegration_AlterTable_DropColumn(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_alter_drop"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

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

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Drop a column
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.DropColumn("legacy_field")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify column is gone
	columns := introspectMySQLColumns(t, db, tableName)
	legacyCol := findMySQLColumn(columns, "legacy_field")
	if legacyCol != nil {
		t.Error("expected 'legacy_field' column to be dropped")
	}
}

func TestMySQLIntegration_AlterTable_RenameColumn(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_alter_rename"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Rename column
	err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
		ab.RenameColumn("name", "full_name")
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify column is renamed
	columns := introspectMySQLColumns(t, db, tableName)
	oldCol := findMySQLColumn(columns, "name")
	newCol := findMySQLColumn(columns, "full_name")

	if oldCol != nil {
		t.Error("expected 'name' column to be renamed")
	}
	if newCol == nil {
		t.Error("expected 'full_name' column to exist")
	}
}

func TestMySQLIntegration_AlterTable_AddIndex(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_alter_index"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	// Create initial table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
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

	sqlStr = plan.Migrations[1].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify index works by inserting duplicates
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (email) VALUES ('test@test.com')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (email) VALUES ('test@test.com')", tableName))
	if err == nil {
		t.Error("expected unique constraint violation after adding index")
	}
}

func TestMySQLIntegration_AlterTable_DropIndex(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_alter_dropidx"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	// Create initial table with index
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email").Unique()
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
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

	sqlStr = plan.Migrations[1].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to alter table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify index is gone - duplicates should now be allowed
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (email) VALUES ('test@test.com')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (email) VALUES ('test@test.com')", tableName))
	if err != nil {
		t.Error("expected duplicate insert to succeed after dropping index")
	}
}

// =============================================================================
// DROP TABLE Integration Tests
// =============================================================================

func TestMySQLIntegration_DropTable(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_drop_table"
	dropMySQLTableIfExists(t, db, tableName)

	// Create table
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to create table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify table exists
	if !mysqlTableExists(t, db, tableName) {
		t.Fatal("expected table to exist after creation")
	}

	// Drop table
	_, err = plan.DropTable(tableName)
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	sqlStr = plan.Migrations[1].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to drop table: %v\nSQL: %s", err, sqlStr)
	}

	// Verify table is gone
	if mysqlTableExists(t, db, tableName) {
		t.Error("expected table to not exist after drop")
	}
}

// =============================================================================
// AddTable (with default columns) Integration Tests
// =============================================================================

func TestMySQLIntegration_AddTable_DefaultColumns(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_default_columns"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	// Create table with default columns using AddTable
	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("email")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Verify all default columns exist
	columns := introspectMySQLColumns(t, db, tableName)

	// Check id column
	idCol := findMySQLColumn(columns, "id")
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
	publicIdCol := findMySQLColumn(columns, "public_id")
	if publicIdCol == nil {
		t.Error("expected 'public_id' column to exist")
	} else {
		if publicIdCol.DataType != "varchar" {
			t.Errorf("expected public_id type 'varchar', got '%s'", publicIdCol.DataType)
		}
		if publicIdCol.IsNullable {
			t.Error("expected public_id to be NOT NULL")
		}
	}

	// Check datetime columns
	for _, colName := range []string{"created_at", "deleted_at", "updated_at"} {
		col := findMySQLColumn(columns, colName)
		if col == nil {
			t.Errorf("expected '%s' column to exist", colName)
		} else {
			if col.DataType != "datetime" {
				t.Errorf("expected %s type 'datetime', got '%s'", colName, col.DataType)
			}
			if col.IsNullable {
				t.Errorf("expected %s to be NOT NULL", colName)
			}
		}
	}

	// Check custom column was also added
	emailCol := findMySQLColumn(columns, "email")
	if emailCol == nil {
		t.Error("expected 'email' column to exist")
	}
}

func TestMySQLIntegration_AddTable_PrimaryKeyConstraint(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_pk_constraint"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Insert first row
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Try to insert duplicate id - should fail
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'xyz789', '2024-01-01', '2024-01-01', '2024-01-01', 'test2')", tableName))
	if err == nil {
		t.Error("expected duplicate id error, but insert succeeded")
	}
}

func TestMySQLIntegration_AddTable_UniquePublicIdConstraint(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tableName := "test_unique_public_id"
	dropMySQLTableIfExists(t, db, tableName)
	defer dropMySQLTableIfExists(t, db, tableName)

	plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
	_, err := plan.AddTable(tableName, func(tb *ddl.TableBuilder) error {
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("AddTable failed: %v", err)
	}

	sqlStr := plan.Migrations[0].Instructions.MySQL
	_, err = db.Exec(sqlStr)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStr)
	}

	// Insert first row
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (id, public_id, created_at, deleted_at, updated_at, name) VALUES (1, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test')", tableName))
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Try to insert duplicate public_id - should fail due to unique constraint
	_, err = db.Exec(fmt.Sprintf("INSERT INTO `%s` (id, public_id, created_at, deleted_at, updated_at, name) VALUES (2, 'abc123', '2024-01-01', '2024-01-01', '2024-01-01', 'test2')", tableName))
	if err == nil {
		t.Error("expected duplicate public_id error, but insert succeeded")
	}
}
