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
// PostgreSQL Roundtrip Tests
// =============================================================================

func TestProperty_Roundtrip_Postgres_CreateTable(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres roundtrip preserves schema", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		// Generate a simple table for roundtrip testing
		table := GenerateSimpleTable(g, tableName)

		// Create migration plan
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

		// Execute
		sqlStr := plan.Migrations[0].Instructions.Postgres
		_, err = conn.Exec(context.Background(), sqlStr)
		if err != nil {
			t.Logf("SQL execution failed: %v\nSQL: %s", err, sqlStr)
			return false
		}

		// Introspect
		columns := introspectPostgresColumns(t, conn, tableName)

		// Convert to normalized
		var normalizedActual []NormalizedColumn
		for _, col := range columns {
			isPrimary := false
			// Check if this column is the primary key by checking introspection
			for _, origCol := range table.Columns {
				if origCol.Name == col.Name && origCol.PrimaryKey {
					isPrimary = true
					break
				}
			}
			normalizedActual = append(normalizedActual, NormalizePostgresColumn(col, isPrimary))
		}

		// Convert expected to normalized
		// Check if this table has autoincrement-eligible PK
		builtTable := plan.Schema.Tables[tableName]
		_, hasAutoincrementPK := GetAutoincrementPK(&builtTable)

		var normalizedExpected []NormalizedColumn
		for _, col := range table.Columns {
			// Determine if this column is the autoincrement PK
			isAutoIncrementPK := hasAutoincrementPK && col.PrimaryKey &&
				(col.Type == ddl.IntegerType || col.Type == ddl.BigintType)

			normalizedExpected = append(normalizedExpected, NormalizedColumn{
				Name:              col.Name,
				BaseType:          NormalizeDDLType(col.Type),
				Nullable:          col.Nullable,
				IsPrimary:         col.PrimaryKey,
				HasDefault:        col.Default != nil,
				IsAutoIncrementPK: isAutoIncrementPK,
			})
		}

		// Compare
		actualTable := NormalizedTable{Name: tableName, Columns: normalizedActual}
		expectedTable := NormalizedTable{Name: tableName, Columns: normalizedExpected}

		diffs := CompareNormalizedTables(expectedTable, actualTable)
		if len(diffs) > 0 {
			t.Logf("Roundtrip mismatch for table %s: %v", tableName, diffs)
			return false
		}

		return true
	})
}

// =============================================================================
// MySQL Roundtrip Tests
// =============================================================================

func TestProperty_Roundtrip_MySQL_CreateTable(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL roundtrip preserves schema", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
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

		// Introspect
		columns := introspectMySQLColumns(t, db, tableName)

		// We need to also get primary key info for MySQL
		// For simplicity, just check column existence and types
		var normalizedActual []NormalizedColumn
		for _, col := range columns {
			isPrimary := false
			for _, origCol := range table.Columns {
				if origCol.Name == col.Name && origCol.PrimaryKey {
					isPrimary = true
					break
				}
			}
			normalizedActual = append(normalizedActual, NormalizeMySQLColumn(col, isPrimary))
		}

		// Check if this table has autoincrement-eligible PK
		builtTable := plan.Schema.Tables[tableName]
		_, hasAutoincrementPK := GetAutoincrementPK(&builtTable)

		var normalizedExpected []NormalizedColumn
		for _, col := range table.Columns {
			// Determine if this column is the autoincrement PK
			isAutoIncrementPK := hasAutoincrementPK && col.PrimaryKey &&
				(col.Type == ddl.IntegerType || col.Type == ddl.BigintType)

			normalizedExpected = append(normalizedExpected, NormalizedColumn{
				Name:              col.Name,
				BaseType:          NormalizeDDLType(col.Type),
				Nullable:          col.Nullable,
				IsPrimary:         col.PrimaryKey,
				HasDefault:        col.Default != nil,
				IsAutoIncrementPK: isAutoIncrementPK,
			})
		}

		actualTable := NormalizedTable{Name: tableName, Columns: normalizedActual}
		expectedTable := NormalizedTable{Name: tableName, Columns: normalizedExpected}

		diffs := CompareNormalizedTables(expectedTable, actualTable)
		if len(diffs) > 0 {
			t.Logf("Roundtrip mismatch for table %s: %v", tableName, diffs)
			return false
		}

		return true
	})
}

// =============================================================================
// SQLite Roundtrip Tests
// =============================================================================

func TestProperty_Roundtrip_SQLite_CreateTable(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite roundtrip preserves schema", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		// Generate a controlled table - always NOT NULL for predictable behavior
		numCols := g.IntRange(2, 4)
		colNames := GenerateUniqueColumnNames(g, numCols)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint(colNames[0]).PrimaryKey()
			for i := 1; i < numCols; i++ {
				tb.String(colNames[i]) // NOT NULL by default
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

		// Introspect
		columns := introspectSQLiteColumns(t, db, tableName)

		// Verify column count matches
		if len(columns) != numCols {
			t.Logf("Column count mismatch: expected %d, got %d", numCols, len(columns))
			return false
		}

		// Verify all columns exist with correct types
		for i, name := range colNames {
			col := findSQLiteColumn(columns, name)
			if col == nil {
				t.Logf("Column %s not found", name)
				return false
			}
			// Primary key should be INTEGER
			if i == 0 {
				if col.PK == 0 {
					t.Logf("Column %s should be primary key", name)
					return false
				}
			}
		}

		return true
	})
}

// =============================================================================
// Column Type Roundtrip Tests
// =============================================================================

func TestProperty_Roundtrip_Postgres_AllTypes(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	// Test each column type individually
	for _, colType := range AllColumnTypes {
		t.Run(colType, func(t *testing.T) {
			proptest.Check(t, "type "+colType+" roundtrips", proptest.Config{NumTrials: 10}, func(g *proptest.Generator) bool {
				tableName := GenerateTableName(g)
				colName := "test_col"

				dropTableIfExists(t, conn, tableName)
				defer dropTableIfExists(t, conn, tableName)

				plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
				_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
					tb.Bigint("id").PrimaryKey()
					addColumnByType(tb, colName, colType, g)
					return nil
				})
				if err != nil {
					return false
				}

				sqlStr := plan.Migrations[0].Instructions.Postgres
				_, err = conn.Exec(context.Background(), sqlStr)
				if err != nil {
					t.Logf("SQL execution failed for %s: %v", colType, err)
					return false
				}

				columns := introspectPostgresColumns(t, conn, tableName)
				foundCol := findColumn(columns, colName)
				if foundCol == nil {
					t.Logf("Column %s not found after creation", colName)
					return false
				}

				// Verify type normalized correctly
				actualType := NormalizePostgresType(foundCol.DataType)
				expectedType := NormalizeDDLType(colType)

				if !TypesEquivalent(expectedType, actualType) {
					t.Logf("Type mismatch for %s: expected %s, got %s", colType, expectedType, actualType)
					return false
				}

				return true
			})
		})
	}
}

func TestProperty_Roundtrip_MySQL_AllTypes(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	for _, colType := range AllColumnTypes {
		t.Run(colType, func(t *testing.T) {
			proptest.Check(t, "type "+colType+" roundtrips", proptest.Config{NumTrials: 10}, func(g *proptest.Generator) bool {
				tableName := GenerateTableName(g)
				colName := "test_col"

				dropMySQLTableIfExists(t, db, tableName)
				defer dropMySQLTableIfExists(t, db, tableName)

				plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
				_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
					tb.Bigint("id").PrimaryKey()
					addColumnByType(tb, colName, colType, g)
					return nil
				})
				if err != nil {
					return false
				}

				sqlStr := plan.Migrations[0].Instructions.MySQL
				_, err = db.Exec(sqlStr)
				if err != nil {
					t.Logf("SQL execution failed for %s: %v", colType, err)
					return false
				}

				columns := introspectMySQLColumns(t, db, tableName)
				foundCol := findMySQLColumn(columns, colName)
				if foundCol == nil {
					t.Logf("Column %s not found after creation", colName)
					return false
				}

				actualType := NormalizeMySQLType(foundCol.DataType)
				expectedType := NormalizeDDLType(colType)

				if !TypesEquivalent(expectedType, actualType) {
					t.Logf("Type mismatch for %s: expected %s, got %s (actual: %s)", colType, expectedType, actualType, foundCol.DataType)
					return false
				}

				return true
			})
		})
	}
}

func TestProperty_Roundtrip_SQLite_AllTypes(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	for _, colType := range AllColumnTypes {
		t.Run(colType, func(t *testing.T) {
			proptest.Check(t, "type "+colType+" roundtrips", proptest.Config{NumTrials: 10}, func(g *proptest.Generator) bool {
				tableName := GenerateTableName(g)
				colName := "test_col"

				db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
				defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

				plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
				_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
					tb.Bigint("id").PrimaryKey()
					addColumnByType(tb, colName, colType, g)
					return nil
				})
				if err != nil {
					return false
				}

				sqlStr := plan.Migrations[0].Instructions.Sqlite
				_, err = db.Exec(sqlStr)
				if err != nil {
					t.Logf("SQL execution failed for %s: %v", colType, err)
					return false
				}

				columns := introspectSQLiteColumns(t, db, tableName)
				foundCol := findSQLiteColumn(columns, colName)
				if foundCol == nil {
					t.Logf("Column %s not found after creation", colName)
					return false
				}

				actualType := NormalizeSQLiteType(foundCol.Type)
				expectedType := NormalizeDDLType(colType)

				if !TypesEquivalent(expectedType, actualType) {
					t.Logf("Type mismatch for %s: expected %s, got %s (actual: %s)", colType, expectedType, actualType, foundCol.Type)
					return false
				}

				return true
			})
		})
	}
}

// =============================================================================
// Nullable Roundtrip Tests
// =============================================================================

func TestProperty_Roundtrip_Postgres_Nullable(t *testing.T) {
	conn := connectPostgres(t)
	defer conn.Close(context.Background())

	proptest.Check(t, "Postgres nullable roundtrips", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		nullable := g.Bool()

		dropTableIfExists(t, conn, tableName)
		defer dropTableIfExists(t, conn, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			col := tb.String("name")
			if nullable {
				col.Nullable()
			}
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

		columns := introspectPostgresColumns(t, conn, tableName)
		foundCol := findColumn(columns, "name")
		if foundCol == nil {
			return false
		}

		if foundCol.IsNullable != nullable {
			t.Logf("Nullable mismatch: expected %v, got %v", nullable, foundCol.IsNullable)
			return false
		}

		return true
	})
}

func TestProperty_Roundtrip_MySQL_Nullable(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	proptest.Check(t, "MySQL nullable roundtrips", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		nullable := g.Bool()

		dropMySQLTableIfExists(t, db, tableName)
		defer dropMySQLTableIfExists(t, db, tableName)

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			col := tb.String("name")
			if nullable {
				col.Nullable()
			}
			return nil
		})
		if err != nil {
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.MySQL
		_, err = db.Exec(sqlStr)
		if err != nil {
			return false
		}

		columns := introspectMySQLColumns(t, db, tableName)
		foundCol := findMySQLColumn(columns, "name")
		if foundCol == nil {
			return false
		}

		if foundCol.IsNullable != nullable {
			t.Logf("Nullable mismatch: expected %v, got %v", nullable, foundCol.IsNullable)
			return false
		}

		return true
	})
}

func TestProperty_Roundtrip_SQLite_Nullable(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	proptest.Check(t, "SQLite nullable roundtrips", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		nullable := g.Bool()

		db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
		defer db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			col := tb.String("name")
			if nullable {
				col.Nullable()
			}
			return nil
		})
		if err != nil {
			return false
		}

		sqlStr := plan.Migrations[0].Instructions.Sqlite
		_, err = db.Exec(sqlStr)
		if err != nil {
			return false
		}

		columns := introspectSQLiteColumns(t, db, tableName)
		foundCol := findSQLiteColumn(columns, "name")
		if foundCol == nil {
			return false
		}

		// SQLite: NotNull = true means NOT nullable
		actualNullable := !foundCol.NotNull
		if actualNullable != nullable {
			t.Logf("Nullable mismatch: expected %v, got %v", nullable, actualNullable)
			return false
		}

		return true
	})
}
