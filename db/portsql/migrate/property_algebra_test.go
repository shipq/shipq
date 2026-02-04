//go:build integration

package migrate

import (
	"context"
	"fmt"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/proptest"
)

// =============================================================================
// Operation Algebra Tests
//
// These tests verify that certain sequences of operations produce predictable
// results, following algebraic properties like:
// - AddColumn(x) + DropColumn(x) = no-op (inverse operations)
// - AddIndex(x) + DropIndex(x) = no-op
// - Operations on independent columns commute
// =============================================================================

// =============================================================================
// PostgreSQL Algebra Tests
// =============================================================================

func TestProperty_Algebra_Postgres_AddDropColumn_NoOp(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "AddColumn + DropColumn = no-op", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		newColName := GenerateColumnName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Create initial table
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			return nil
		})
		if err != nil {
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("Initial table creation failed: %v", err)
			return false
		}

		// Get initial schema
		initialColumns := introspectPostgresColumns(t, conn, tableName)

		// Add column
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.String(newColName)
			return nil
		})
		if err != nil {
			return false
		}

		addSql := plan.Migrations[1].Instructions.Postgres
		_, err = conn.Exec(context.Background(), addSql)
		if err != nil {
			t.Logf("Add column failed: %v", err)
			return false
		}

		// Drop the same column
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.DropColumn(newColName)
			return nil
		})
		if err != nil {
			return false
		}

		dropSql := plan.Migrations[2].Instructions.Postgres
		_, err = conn.Exec(context.Background(), dropSql)
		if err != nil {
			t.Logf("Drop column failed: %v", err)
			return false
		}

		// Get final schema
		finalColumns := introspectPostgresColumns(t, conn, tableName)

		// Schemas should match
		if len(initialColumns) != len(finalColumns) {
			t.Logf("Column count mismatch: initial=%d, final=%d", len(initialColumns), len(finalColumns))
			return false
		}

		for i, initCol := range initialColumns {
			if initCol.Name != finalColumns[i].Name {
				t.Logf("Column name mismatch at %d: %s vs %s", i, initCol.Name, finalColumns[i].Name)
				return false
			}
		}

		return true
	})
}

func TestProperty_Algebra_Postgres_AddDropIndex_NoOp(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "AddIndex + DropIndex = no-op", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Create initial table
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			tb.Integer("age")
			return nil
		})
		if err != nil {
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			return false
		}

		// Get initial indexes
		initialIndexes := introspectPostgresIndexes(t, conn, tableName)

		// Add index
		indexName := ddl.GenerateIndexName(tableName, []string{"name"})
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			nameCol, _ := ab.ExistingColumn("name")
			ab.AddIndex(nameCol)
			return nil
		})
		if err != nil {
			return false
		}

		addSql := plan.Migrations[1].Instructions.Postgres
		_, err = conn.Exec(context.Background(), addSql)
		if err != nil {
			t.Logf("Add index failed: %v", err)
			return false
		}

		// Drop the same index
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.DropIndex(indexName)
			return nil
		})
		if err != nil {
			return false
		}

		dropSql := plan.Migrations[2].Instructions.Postgres
		_, err = conn.Exec(context.Background(), dropSql)
		if err != nil {
			t.Logf("Drop index failed: %v", err)
			return false
		}

		// Get final indexes
		finalIndexes := introspectPostgresIndexes(t, conn, tableName)

		// Should have same number of indexes
		if len(initialIndexes) != len(finalIndexes) {
			t.Logf("Index count mismatch: initial=%d, final=%d", len(initialIndexes), len(finalIndexes))
			return false
		}

		return true
	})
}

// =============================================================================
// MySQL Algebra Tests
// =============================================================================

func TestProperty_Algebra_MySQL_AddDropColumn_NoOp(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "AddColumn + DropColumn = no-op", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		newColName := GenerateColumnName(g)

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

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("Initial table creation failed: %v", err)
			return false
		}

		initialColumns := introspectMySQLColumns(t, db, tableName)

		// Add column
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.String(newColName)
			return nil
		})
		if err != nil {
			return false
		}

		addSql := plan.Migrations[1].Instructions.MySQL
		_, err = db.Exec(addSql)
		if err != nil {
			t.Logf("Add column failed: %v", err)
			return false
		}

		// Drop column
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.DropColumn(newColName)
			return nil
		})
		if err != nil {
			return false
		}

		dropSql := plan.Migrations[2].Instructions.MySQL
		_, err = db.Exec(dropSql)
		if err != nil {
			t.Logf("Drop column failed: %v", err)
			return false
		}

		finalColumns := introspectMySQLColumns(t, db, tableName)

		if len(initialColumns) != len(finalColumns) {
			t.Logf("Column count mismatch: initial=%d, final=%d", len(initialColumns), len(finalColumns))
			return false
		}

		return true
	})
}

// =============================================================================
// SQLite Algebra Tests
// =============================================================================

func TestProperty_Algebra_SQLite_AddDropColumn_NoOp(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "AddColumn + DropColumn = no-op", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		newColName := GenerateColumnName(g)

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

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			t.Logf("Initial table creation failed: %v", err)
			return false
		}

		initialColumns := introspectSQLiteColumns(t, db, tableName)

		// Add column
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.String(newColName)
			return nil
		})
		if err != nil {
			return false
		}

		addSql := plan.Migrations[1].Instructions.Sqlite
		_, err = db.Exec(addSql)
		if err != nil {
			t.Logf("Add column failed: %v", err)
			return false
		}

		// Drop column
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.DropColumn(newColName)
			return nil
		})
		if err != nil {
			return false
		}

		dropSql := plan.Migrations[2].Instructions.Sqlite
		_, err = db.Exec(dropSql)
		if err != nil {
			t.Logf("Drop column failed: %v", err)
			return false
		}

		finalColumns := introspectSQLiteColumns(t, db, tableName)

		if len(initialColumns) != len(finalColumns) {
			t.Logf("Column count mismatch: initial=%d, final=%d", len(initialColumns), len(finalColumns))
			return false
		}

		return true
	})
}

// =============================================================================
// Independent Operations Commute Tests
// =============================================================================

func TestProperty_Algebra_Postgres_IndependentColumnsCommute(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Independent column additions commute", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName1 := GenerateTableName(g)
		tableName2 := tableName1 + "_2"

		dropTableIfExists(t, conn, tableName1)
		dropTableIfExists(t, conn, tableName2)
		defer dropTableIfExists(t, conn, tableName1)
		defer dropTableIfExists(t, conn, tableName2)

		col1 := GenerateColumnName(g)
		col2 := col1 + "_b" // Ensure different names

		// Create two identical base tables
		createBase := func(name string) error {
			plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
			_, err := plan.AddEmptyTable(name, func(tb *ddl.TableBuilder) error {
				tb.Bigint("id").PrimaryKey()
				return nil
			})
			if err != nil {
				return err
			}
			_, err = conn.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
			return err
		}

		if err := createBase(tableName1); err != nil {
			return false
		}
		if err := createBase(tableName2); err != nil {
			return false
		}

		// Table 1: Add col1 then col2
		plan1 := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{tableName1: {Name: tableName1, Columns: []ddl.ColumnDefinition{{Name: "id", Type: ddl.BigintType, PrimaryKey: true}}}}}}
		plan1.UpdateTable(tableName1, func(ab *ddl.AlterTableBuilder) error {
			ab.String(col1)
			return nil
		})
		conn.Exec(context.Background(), plan1.Migrations[0].Instructions.Postgres)

		plan1.UpdateTable(tableName1, func(ab *ddl.AlterTableBuilder) error {
			ab.Integer(col2)
			return nil
		})
		conn.Exec(context.Background(), plan1.Migrations[1].Instructions.Postgres)

		// Table 2: Add col2 then col1 (reversed order)
		plan2 := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{tableName2: {Name: tableName2, Columns: []ddl.ColumnDefinition{{Name: "id", Type: ddl.BigintType, PrimaryKey: true}}}}}}
		plan2.UpdateTable(tableName2, func(ab *ddl.AlterTableBuilder) error {
			ab.Integer(col2)
			return nil
		})
		conn.Exec(context.Background(), plan2.Migrations[0].Instructions.Postgres)

		plan2.UpdateTable(tableName2, func(ab *ddl.AlterTableBuilder) error {
			ab.String(col1)
			return nil
		})
		conn.Exec(context.Background(), plan2.Migrations[1].Instructions.Postgres)

		// Both tables should have the same columns (regardless of order)
		cols1 := introspectPostgresColumns(t, conn, tableName1)
		cols2 := introspectPostgresColumns(t, conn, tableName2)

		if len(cols1) != len(cols2) {
			t.Logf("Column count mismatch: %d vs %d", len(cols1), len(cols2))
			return false
		}

		// Check that both have all expected columns
		colNames1 := make(map[string]bool)
		for _, c := range cols1 {
			colNames1[c.Name] = true
		}
		colNames2 := make(map[string]bool)
		for _, c := range cols2 {
			colNames2[c.Name] = true
		}

		expected := []string{"id", col1, col2}
		for _, name := range expected {
			if !colNames1[name] {
				t.Logf("Table 1 missing column: %s", name)
				return false
			}
			if !colNames2[name] {
				t.Logf("Table 2 missing column: %s", name)
				return false
			}
		}

		return true
	})
}

// =============================================================================
// Rename Back and Forth Tests
// =============================================================================

func TestProperty_Algebra_Postgres_RenameRename_Identity(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Rename(a→b) + Rename(b→a) = identity", proptest.Config{NumTrials: 15}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		origName := "original_col"
		tempName := "temp_col"

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Create table with original column
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String(origName)
			return nil
		})
		if err != nil {
			return false
		}

		_, err = conn.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			return false
		}

		// Get initial state
		initialCols := introspectPostgresColumns(t, conn, tableName)

		// Rename a→b
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.RenameColumn(origName, tempName)
			return nil
		})
		if err != nil {
			return false
		}
		_, err = conn.Exec(context.Background(), plan.Migrations[1].Instructions.Postgres)
		if err != nil {
			t.Logf("First rename failed: %v", err)
			return false
		}

		// Rename b→a
		err = plan.UpdateTable(tableName, func(ab *ddl.AlterTableBuilder) error {
			ab.RenameColumn(tempName, origName)
			return nil
		})
		if err != nil {
			return false
		}
		_, err = conn.Exec(context.Background(), plan.Migrations[2].Instructions.Postgres)
		if err != nil {
			t.Logf("Second rename failed: %v", err)
			return false
		}

		// Get final state
		finalCols := introspectPostgresColumns(t, conn, tableName)

		// Column names should match initial state
		if len(initialCols) != len(finalCols) {
			return false
		}

		for i, initCol := range initialCols {
			if initCol.Name != finalCols[i].Name {
				t.Logf("Column name mismatch at %d: %s vs %s", i, initCol.Name, finalCols[i].Name)
				return false
			}
		}

		return true
	})
}
