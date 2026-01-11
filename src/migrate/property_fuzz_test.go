//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/portsql/portsql/proptest"
	"github.com/portsql/portsql/src/ddl"
)

// =============================================================================
// PostgreSQL Fuzz Tests
// =============================================================================

func TestProperty_Fuzz_Postgres_ValidSQL_CreateTable(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres CREATE TABLE SQL is always valid", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		// Clean up before test
		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Generate a random table
		cfg := DefaultTableConfig()
		cfg.MaxColumns = 5 // Keep it reasonable for faster tests
		table := GenerateTable(g, tableName, cfg)

		// Create migration plan and generate SQL
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			for _, col := range table.Columns {
				if err := addColumnToBuilder(tb, col); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		// Execute the SQL
		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("SQL execution failed for table %s: %v\nSQL: %s", tableName, err, sqlStr)
			return false
		}

		// Verify table exists
		if !tableExists(t, conn, tableName) {
			t.Logf("Table %s does not exist after creation", tableName)
			return false
		}

		return true
	})
}

func TestProperty_Fuzz_Postgres_ValidSQL_SimpleTable(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres simple table SQL is always valid", proptest.Config{NumTrials: 100}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Generate a simple table (fewer edge cases)
		table := GenerateSimpleTable(g, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			for _, col := range table.Columns {
				if err := addColumnToBuilder(tb, col); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("SQL execution failed: %v\nSQL: %s", err, sqlStr)
			return false
		}

		return tableExists(t, conn, tableName)
	})
}

// =============================================================================
// MySQL Fuzz Tests
// =============================================================================

func TestProperty_Fuzz_MySQL_ValidSQL_CreateTable(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL CREATE TABLE SQL is always valid", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		cfg := DefaultTableConfig()
		cfg.MaxColumns = 5
		table := GenerateTable(g, tableName, cfg)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			for _, col := range table.Columns {
				if err := addColumnToBuilder(tb, col); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed for table %s: %v\nSQL: %s", tableName, err, sqlStr)
			return false
		}

		return mysqlTableExists(t, db, tableName)
	})
}

func TestProperty_Fuzz_MySQL_ValidSQL_SimpleTable(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL simple table SQL is always valid", proptest.Config{NumTrials: 100}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		table := GenerateSimpleTable(g, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			for _, col := range table.Columns {
				if err := addColumnToBuilder(tb, col); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed: %v\nSQL: %s", err, sqlStr)
			return false
		}

		return mysqlTableExists(t, db, tableName)
	})
}

// =============================================================================
// SQLite Fuzz Tests
// =============================================================================

func TestProperty_Fuzz_SQLite_ValidSQL_CreateTable(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite CREATE TABLE SQL is always valid", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		// SQLite in-memory, clean via drop
		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		cfg := DefaultTableConfig()
		cfg.MaxColumns = 5
		table := GenerateTable(g, tableName, cfg)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			for _, col := range table.Columns {
				if err := addColumnToBuilder(tb, col); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed for table %s: %v\nSQL: %s", tableName, err, sqlStr)
			return false
		}

		return sqliteTableExists(t, db, tableName)
	})
}

func TestProperty_Fuzz_SQLite_ValidSQL_SimpleTable(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite simple table SQL is always valid", proptest.Config{NumTrials: 100}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		table := GenerateSimpleTable(g, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			for _, col := range table.Columns {
				if err := addColumnToBuilder(tb, col); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed: %v\nSQL: %s", err, sqlStr)
			return false
		}

		return sqliteTableExists(t, db, tableName)
	})
}

// =============================================================================
// Reserved Word Identifier Tests
// =============================================================================

func TestProperty_Fuzz_Postgres_ReservedWordColumns(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres handles reserved word columns", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Pick a reserved word for the column name
		reservedWord := GenerateReservedWordIdentifier(g)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String(reservedWord) // Use reserved word as column name
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with reserved word %q: %v\nSQL: %s", reservedWord, err, sqlStr)
			return false
		}

		return tableExists(t, conn, tableName)
	})
}

func TestProperty_Fuzz_MySQL_ReservedWordColumns(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL handles reserved word columns", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		reservedWord := GenerateReservedWordIdentifier(g)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String(reservedWord)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with reserved word %q: %v\nSQL: %s", reservedWord, err, sqlStr)
			return false
		}

		return mysqlTableExists(t, db, tableName)
	})
}

func TestProperty_Fuzz_SQLite_ReservedWordColumns(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite handles reserved word columns", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		reservedWord := GenerateReservedWordIdentifier(g)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String(reservedWord)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with reserved word %q: %v\nSQL: %s", reservedWord, err, sqlStr)
			return false
		}

		return sqliteTableExists(t, db, tableName)
	})
}

// =============================================================================
// String Default Value Tests
// =============================================================================

func TestProperty_Fuzz_Postgres_StringDefaults(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres handles string defaults", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		defaultVal := GenerateSafeStringDefault(g)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name").Default(defaultVal)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with default %q: %v\nSQL: %s", defaultVal, err, sqlStr)
			return false
		}

		return tableExists(t, conn, tableName)
	})
}

func TestProperty_Fuzz_MySQL_StringDefaults(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL handles string defaults", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		defaultVal := GenerateSafeStringDefault(g)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name").Default(defaultVal)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with default %q: %v\nSQL: %s", defaultVal, err, sqlStr)
			return false
		}

		return mysqlTableExists(t, db, tableName)
	})
}

func TestProperty_Fuzz_SQLite_StringDefaults(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite handles string defaults", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		defaultVal := GenerateSafeStringDefault(g)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name").Default(defaultVal)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with default %q: %v\nSQL: %s", defaultVal, err, sqlStr)
			return false
		}

		return sqliteTableExists(t, db, tableName)
	})
}

// =============================================================================
// All Column Types Test
// =============================================================================

func TestProperty_Fuzz_Postgres_AllColumnTypes(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres handles all column types", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Pick a random column type
		colType := proptest.Pick(g, AllColumnTypes)
		colName := "test_col"

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			addColumnByType(tb, colName, colType, g)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with type %q: %v\nSQL: %s", colType, err, sqlStr)
			return false
		}

		return tableExists(t, conn, tableName)
	})
}

func TestProperty_Fuzz_MySQL_AllColumnTypes(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL handles all column types", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		colType := proptest.Pick(g, AllColumnTypes)
		colName := "test_col"

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			addColumnByType(tb, colName, colType, g)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with type %q: %v\nSQL: %s", colType, err, sqlStr)
			return false
		}

		return mysqlTableExists(t, db, tableName)
	})
}

func TestProperty_Fuzz_SQLite_AllColumnTypes(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite handles all column types", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		colType := proptest.Pick(g, AllColumnTypes)
		colName := "test_col"

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			addColumnByType(tb, colName, colType, g)
			return nil
		})
		if err != nil {
			t.Logf("AddTable failed: %v", err)
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("SQL execution failed with type %q: %v\nSQL: %s", colType, err, sqlStr)
			return false
		}

		return sqliteTableExists(t, db, tableName)
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// addColumnByType adds a column of the specified type to a table builder
func addColumnByType(tb *ddl.TableBuilder, name string, colType string, g *proptest.Generator) {
	switch colType {
	case ddl.IntegerType:
		tb.Integer(name)
	case ddl.BigintType:
		tb.Bigint(name)
	case ddl.StringType:
		tb.String(name)
	case ddl.TextType:
		tb.Text(name)
	case ddl.BooleanType:
		tb.Bool(name)
	case ddl.FloatType:
		tb.Float(name)
	case ddl.DecimalType:
		precision := g.IntRange(5, 18)
		scale := g.IntRange(0, precision-1)
		tb.Decimal(name, precision, scale)
	case ddl.DatetimeType:
		tb.Datetime(name)
	case ddl.BinaryType:
		tb.Binary(name)
	case ddl.JSONType:
		tb.JSON(name)
	}
}

// Ensure unused imports are used
var _ = time.Second
var _ pgx.Conn
var _ sql.DB
