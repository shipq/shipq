//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/portsql/portsql/proptest"
	"github.com/portsql/portsql/src/ddl"
)

// =============================================================================
// Constraint Enforcement Tests
//
// These tests verify that database constraints actually enforce their rules:
// - PRIMARY KEY rejects duplicate values
// - UNIQUE indexes reject duplicate values
// - NOT NULL rejects NULL inserts
// =============================================================================

// =============================================================================
// PostgreSQL Constraint Tests
// =============================================================================

func TestProperty_Constraint_Postgres_PrimaryKey_RejectsDuplicates(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres PRIMARY KEY rejects duplicates", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		pkValue := g.IntRange(1, 10000)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Create table with primary key
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			return nil
		})
		if err != nil {
			return false
		}

		_, err = conn.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			return false
		}

		// Insert first row
		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id, name) VALUES ($1, 'first')`, tableName)
		_, err = conn.Exec(context.Background(), insertSQL, pkValue)
		if err != nil {
			t.Logf("First insert failed: %v", err)
			return false
		}

		// Try to insert duplicate - should fail
		_, err = conn.Exec(context.Background(), insertSQL, pkValue)
		if err == nil {
			t.Logf("Duplicate insert should have failed")
			return false
		}

		// Error should be about duplicate key
		if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "unique") {
			t.Logf("Expected duplicate key error, got: %v", err)
			return false
		}

		return true
	})
}

func TestProperty_Constraint_Postgres_Unique_RejectsDuplicates(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres UNIQUE index rejects duplicates", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		uniqueValue := g.StringAlphaNum(10)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Create table with unique column
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("email").Unique()
			return nil
		})
		if err != nil {
			return false
		}

		_, err = conn.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			return false
		}

		// Insert first row
		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id, email) VALUES ($1, $2)`, tableName)
		_, err = conn.Exec(context.Background(), insertSQL, 1, uniqueValue)
		if err != nil {
			t.Logf("First insert failed: %v", err)
			return false
		}

		// Try duplicate - should fail
		_, err = conn.Exec(context.Background(), insertSQL, 2, uniqueValue)
		if err == nil {
			t.Logf("Duplicate insert should have failed")
			return false
		}

		return true
	})
}

func TestProperty_Constraint_Postgres_NotNull_RejectsNull(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres NOT NULL rejects NULL values", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Create table with NOT NULL column
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("required_field") // NOT NULL by default
			return nil
		})
		if err != nil {
			return false
		}

		_, err = conn.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			return false
		}

		// Try to insert NULL - should fail
		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id, required_field) VALUES ($1, NULL)`, tableName)
		_, err = conn.Exec(context.Background(), insertSQL, 1)
		if err == nil {
			t.Logf("NULL insert should have failed")
			return false
		}

		// Error should mention null violation
		if !strings.Contains(strings.ToLower(err.Error()), "null") && !strings.Contains(strings.ToLower(err.Error()), "not-null") {
			t.Logf("Expected null violation error, got: %v", err)
			return false
		}

		return true
	})
}

// =============================================================================
// MySQL Constraint Tests
// =============================================================================

func TestProperty_Constraint_MySQL_PrimaryKey_RejectsDuplicates(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL PRIMARY KEY rejects duplicates", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		pkValue := g.IntRange(1, 10000)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.MySQL)
		if err != nil {
			return false
		}

		// Insert first row
		insertSQL := fmt.Sprintf("INSERT INTO `%s` (id, name) VALUES (?, 'first')", tableName)
		_, err = db.Exec(insertSQL, pkValue)
		if err != nil {
			t.Logf("First insert failed: %v", err)
			return false
		}

		// Try duplicate - should fail
		_, err = db.Exec(insertSQL, pkValue)
		if err == nil {
			t.Logf("Duplicate insert should have failed")
			return false
		}

		return true
	})
}

func TestProperty_Constraint_MySQL_Unique_RejectsDuplicates(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL UNIQUE index rejects duplicates", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		uniqueValue := g.StringAlphaNum(10)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("email").Unique()
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.MySQL)
		if err != nil {
			return false
		}

		insertSQL := fmt.Sprintf("INSERT INTO `%s` (id, email) VALUES (?, ?)", tableName)
		_, err = db.Exec(insertSQL, 1, uniqueValue)
		if err != nil {
			t.Logf("First insert failed: %v", err)
			return false
		}

		_, err = db.Exec(insertSQL, 2, uniqueValue)
		if err == nil {
			t.Logf("Duplicate insert should have failed")
			return false
		}

		return true
	})
}

func TestProperty_Constraint_MySQL_NotNull_RejectsNull(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL NOT NULL rejects NULL values", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("required_field")
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.MySQL)
		if err != nil {
			return false
		}

		// Try to insert NULL
		insertSQL := fmt.Sprintf("INSERT INTO `%s` (id, required_field) VALUES (?, NULL)", tableName)
		_, err = db.Exec(insertSQL, 1)
		if err == nil {
			t.Logf("NULL insert should have failed")
			return false
		}

		return true
	})
}

// =============================================================================
// SQLite Constraint Tests
// =============================================================================

func TestProperty_Constraint_SQLite_PrimaryKey_RejectsDuplicates(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite PRIMARY KEY rejects duplicates", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		pkValue := g.IntRange(1, 10000)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.Sqlite)
		if err != nil {
			return false
		}

		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id, name) VALUES (?, 'first')`, tableName)
		_, err = db.Exec(insertSQL, pkValue)
		if err != nil {
			t.Logf("First insert failed: %v", err)
			return false
		}

		_, err = db.Exec(insertSQL, pkValue)
		if err == nil {
			t.Logf("Duplicate insert should have failed")
			return false
		}

		return true
	})
}

func TestProperty_Constraint_SQLite_Unique_RejectsDuplicates(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite UNIQUE index rejects duplicates", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		uniqueValue := g.StringAlphaNum(10)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("email").Unique()
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.Sqlite)
		if err != nil {
			return false
		}

		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id, email) VALUES (?, ?)`, tableName)
		_, err = db.Exec(insertSQL, 1, uniqueValue)
		if err != nil {
			t.Logf("First insert failed: %v", err)
			return false
		}

		_, err = db.Exec(insertSQL, 2, uniqueValue)
		if err == nil {
			t.Logf("Duplicate insert should have failed")
			return false
		}

		return true
	})
}

func TestProperty_Constraint_SQLite_NotNull_RejectsNull(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite NOT NULL rejects NULL values", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("required_field")
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.Sqlite)
		if err != nil {
			return false
		}

		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id, required_field) VALUES (?, NULL)`, tableName)
		_, err = db.Exec(insertSQL, 1)
		if err == nil {
			t.Logf("NULL insert should have failed")
			return false
		}

		return true
	})
}

// =============================================================================
// Default Value Tests
// =============================================================================

func TestProperty_Constraint_Postgres_Default_Applied(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres DEFAULT is applied when value omitted", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		defaultVal := g.StringAlphaNum(10)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			// Use nullable so we can test default is applied when value omitted
			tb.String("name").Nullable().Default(defaultVal)
			return nil
		})
		if err != nil {
			return false
		}

		_, err = conn.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			return false
		}

		// Insert without specifying the column
		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id) VALUES ($1)`, tableName)
		_, err = conn.Exec(context.Background(), insertSQL, 1)
		if err != nil {
			t.Logf("Insert failed: %v", err)
			return false
		}

		// Check value
		var actualValue string
		selectSQL := fmt.Sprintf(`SELECT name FROM "%s" WHERE id = 1`, tableName)
		err = conn.QueryRow(context.Background(), selectSQL).Scan(&actualValue)
		if err != nil {
			t.Logf("Select failed: %v", err)
			return false
		}

		if actualValue != defaultVal {
			t.Logf("Default value mismatch: expected %q, got %q", defaultVal, actualValue)
			return false
		}

		return true
	})
}

func TestProperty_Constraint_MySQL_Default_Applied(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL DEFAULT is applied when value omitted", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		defaultVal := g.StringAlphaNum(10)

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			// Use nullable so we can test default is applied when value omitted
			tb.String("name").Nullable().Default(defaultVal)
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.MySQL)
		if err != nil {
			return false
		}

		insertSQL := fmt.Sprintf("INSERT INTO `%s` (id) VALUES (?)", tableName)
		_, err = db.Exec(insertSQL, 1)
		if err != nil {
			t.Logf("Insert failed: %v", err)
			return false
		}

		var actualValue string
		selectSQL := fmt.Sprintf("SELECT name FROM `%s` WHERE id = 1", tableName)
		err = db.QueryRow(selectSQL).Scan(&actualValue)
		if err != nil {
			t.Logf("Select failed: %v", err)
			return false
		}

		if actualValue != defaultVal {
			t.Logf("Default value mismatch: expected %q, got %q", defaultVal, actualValue)
			return false
		}

		return true
	})
}

func TestProperty_Constraint_SQLite_Default_Applied(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite DEFAULT is applied when value omitted", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		// Use StringFromN to ensure at least 1 character (empty string means "no default" in current API)
		defaultVal := g.StringFromN(proptest.CharsetAlphaNum, 1, 10)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			// Make nullable so we can insert without explicit value and see default applied
			tb.String("name").Nullable().Default(defaultVal)
			return nil
		})
		if err != nil {
			return false
		}

		_, err = db.Exec(plan.Migrations[0].Instructions.Sqlite)
		if err != nil {
			return false
		}

		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (id) VALUES (?)`, tableName)
		_, err = db.Exec(insertSQL, 1)
		if err != nil {
			t.Logf("Insert failed: %v", err)
			return false
		}

		var actualValue string
		selectSQL := fmt.Sprintf(`SELECT name FROM "%s" WHERE id = 1`, tableName)
		err = db.QueryRow(selectSQL).Scan(&actualValue)
		if err != nil {
			t.Logf("Select failed: %v", err)
			return false
		}

		if actualValue != defaultVal {
			t.Logf("Default value mismatch: expected %q, got %q", defaultVal, actualValue)
			return false
		}

		return true
	})
}

// Ensure unused import is used
var _ sql.DB
