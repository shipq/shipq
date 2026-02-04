//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/proptest"

	_ "github.com/go-sql-driver/mysql"
)

// =============================================================================
// Cross-Database Equivalence Tests
//
// These tests verify that the same MigrationPlan produces logically equivalent
// schemas across PostgreSQL, MySQL, and SQLite.
// =============================================================================

// dbConnections holds connections to all three databases for cross-db tests
type dbConnections struct {
	postgres *pgx.Conn
	mysql    *sql.DB
	sqlite   *sql.DB
}

// connectAllDatabases attempts to connect to all three databases.
// Skips the test if any database is unavailable.
func connectAllDatabases(t *testing.T) *dbConnections {
	t.Helper()

	postgres := connectPostgres(t)
	mysql := connectMySQL(t)
	sqlite := connectSQLite(t)

	return &dbConnections{
		postgres: postgres,
		mysql:    mysql,
		sqlite:   sqlite,
	}
}

// closeAll closes all database connections
func (dbs *dbConnections) closeAll() {
	if dbs.postgres != nil {
		dbs.postgres.Close(context.Background())
	}
	if dbs.mysql != nil {
		dbs.mysql.Close()
	}
	if dbs.sqlite != nil {
		dbs.sqlite.Close()
	}
}

// dropAllTables drops the table from all databases
func (dbs *dbConnections) dropAllTables(t *testing.T, tableName string) {
	t.Helper()
	dropTableIfExists(t, dbs.postgres, tableName)
	dropMySQLTableIfExists(t, dbs.mysql, tableName)
	dbs.sqlite.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
}

// =============================================================================
// Cross-Database Equivalence Tests
// =============================================================================

func TestProperty_CrossDB_CreateTable_Equivalent(t *testing.T) {
	dbs := connectAllDatabases(t)
	defer dbs.closeAll()

	proptest.Check(t, "Same table definition produces equivalent schemas across DBs", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dbs.dropAllTables(t, tableName)
		defer dbs.dropAllTables(t, tableName)

		// Generate a simple table that works across all databases
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

		// Execute on all databases
		pgSQL := plan.Migrations[0].Instructions.Postgres
		mySQL := plan.Migrations[0].Instructions.MySQL
		sqSQL := plan.Migrations[0].Instructions.Sqlite

		_, err = dbs.postgres.Exec(context.Background(), pgSQL)
		if err != nil {
			t.Logf("Postgres execution failed: %v\nSQL: %s", err, pgSQL)
			return false
		}

		_, err = dbs.mysql.Exec(mySQL)
		if err != nil {
			t.Logf("MySQL execution failed: %v\nSQL: %s", err, mySQL)
			return false
		}

		_, err = dbs.sqlite.Exec(sqSQL)
		if err != nil {
			t.Logf("SQLite execution failed: %v\nSQL: %s", err, sqSQL)
			return false
		}

		// Introspect all databases
		pgCols := introspectPostgresColumns(t, dbs.postgres, tableName)
		myCols := introspectMySQLColumns(t, dbs.mysql, tableName)
		sqCols := introspectSQLiteColumns(t, dbs.sqlite, tableName)

		// Normalize all schemas
		pgNorm := normalizePostgresTable(tableName, pgCols, table.Columns)
		myNorm := normalizeMySQLTable(tableName, myCols, table.Columns)
		sqNorm := normalizeSQLiteTable(tableName, sqCols)

		// Compare
		pgMyDiffs := CompareNormalizedTables(pgNorm, myNorm)
		if len(pgMyDiffs) > 0 {
			t.Logf("Postgres vs MySQL differences for %s: %v", tableName, pgMyDiffs)
			// Log the actual types for debugging
			for i, col := range pgCols {
				t.Logf("PG col %d: %s (%s)", i, col.Name, col.DataType)
			}
			for i, col := range myCols {
				t.Logf("MY col %d: %s (%s)", i, col.Name, col.DataType)
			}
			return false
		}

		pgSqDiffs := CompareNormalizedTables(pgNorm, sqNorm)
		if len(pgSqDiffs) > 0 {
			t.Logf("Postgres vs SQLite differences for %s: %v", tableName, pgSqDiffs)
			return false
		}

		return true
	})
}

func TestProperty_CrossDB_AllTypes_Equivalent(t *testing.T) {
	dbs := connectAllDatabases(t)
	defer dbs.closeAll()

	// Test each column type for cross-DB equivalence
	for _, colType := range AllColumnTypes {
		t.Run(colType, func(t *testing.T) {
			proptest.Check(t, "type "+colType+" is equivalent across DBs", proptest.Config{NumTrials: 5}, func(g *proptest.Generator) bool {
				tableName := GenerateTableName(g)
				colName := "test_col"

				dbs.dropAllTables(t, tableName)
				defer dbs.dropAllTables(t, tableName)

				// Create table with this column type
				plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
				_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
					tb.Bigint("id").PrimaryKey()
					addColumnByType(tb, colName, colType, g)
					return nil
				})
				if err != nil {
					return false
				}

				// Execute on all databases
				_, err = dbs.postgres.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
				if err != nil {
					t.Logf("Postgres failed for %s: %v", colType, err)
					return false
				}

				_, err = dbs.mysql.Exec(plan.Migrations[0].Instructions.MySQL)
				if err != nil {
					t.Logf("MySQL failed for %s: %v", colType, err)
					return false
				}

				_, err = dbs.sqlite.Exec(plan.Migrations[0].Instructions.Sqlite)
				if err != nil {
					t.Logf("SQLite failed for %s: %v", colType, err)
					return false
				}

				// Introspect and compare
				pgCols := introspectPostgresColumns(t, dbs.postgres, tableName)
				myCols := introspectMySQLColumns(t, dbs.mysql, tableName)
				sqCols := introspectSQLiteColumns(t, dbs.sqlite, tableName)

				pgCol := findColumn(pgCols, colName)
				myCol := findMySQLColumn(myCols, colName)
				sqCol := findSQLiteColumn(sqCols, colName)

				if pgCol == nil || myCol == nil || sqCol == nil {
					t.Logf("Column %s not found in one or more databases", colName)
					return false
				}

				// Normalize types
				pgType := NormalizePostgresType(pgCol.DataType)
				myType := NormalizeMySQLType(myCol.DataType)
				sqType := NormalizeSQLiteType(sqCol.Type)
				expectedType := NormalizeDDLType(colType)

				// All should be equivalent to expected
				if !TypesEquivalent(expectedType, pgType) {
					t.Logf("Postgres type mismatch for %s: expected %s, got %s (%s)", colType, expectedType, pgType, pgCol.DataType)
					return false
				}

				if !TypesEquivalent(expectedType, myType) {
					t.Logf("MySQL type mismatch for %s: expected %s, got %s (%s)", colType, expectedType, myType, myCol.DataType)
					return false
				}

				if !TypesEquivalent(expectedType, sqType) {
					t.Logf("SQLite type mismatch for %s: expected %s, got %s (%s)", colType, expectedType, sqType, sqCol.Type)
					return false
				}

				return true
			})
		})
	}
}

func TestProperty_CrossDB_Nullable_Equivalent(t *testing.T) {
	dbs := connectAllDatabases(t)
	defer dbs.closeAll()

	proptest.Check(t, "Nullable columns are equivalent across DBs", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)
		nullable := g.Bool()

		dbs.dropAllTables(t, tableName)
		defer dbs.dropAllTables(t, tableName)

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

		// Execute on all
		_, err = dbs.postgres.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			return false
		}
		_, err = dbs.mysql.Exec(plan.Migrations[0].Instructions.MySQL)
		if err != nil {
			return false
		}
		_, err = dbs.sqlite.Exec(plan.Migrations[0].Instructions.Sqlite)
		if err != nil {
			return false
		}

		// Check nullable in all
		pgCols := introspectPostgresColumns(t, dbs.postgres, tableName)
		myCols := introspectMySQLColumns(t, dbs.mysql, tableName)
		sqCols := introspectSQLiteColumns(t, dbs.sqlite, tableName)

		pgCol := findColumn(pgCols, "name")
		myCol := findMySQLColumn(myCols, "name")
		sqCol := findSQLiteColumn(sqCols, "name")

		if pgCol == nil || myCol == nil || sqCol == nil {
			return false
		}

		pgNullable := pgCol.IsNullable
		myNullable := myCol.IsNullable
		sqNullable := !sqCol.NotNull

		// All should match expected
		if pgNullable != nullable {
			t.Logf("Postgres nullable mismatch: expected %v, got %v", nullable, pgNullable)
			return false
		}
		if myNullable != nullable {
			t.Logf("MySQL nullable mismatch: expected %v, got %v", nullable, myNullable)
			return false
		}
		if sqNullable != nullable {
			t.Logf("SQLite nullable mismatch: expected %v, got %v", nullable, sqNullable)
			return false
		}

		return true
	})
}

func TestProperty_CrossDB_ColumnCount_Equivalent(t *testing.T) {
	dbs := connectAllDatabases(t)
	defer dbs.closeAll()

	proptest.Check(t, "Same number of columns across DBs", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dbs.dropAllTables(t, tableName)
		defer dbs.dropAllTables(t, tableName)

		// Generate random table
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
			return false
		}

		// Execute on all
		dbs.postgres.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		dbs.mysql.Exec(plan.Migrations[0].Instructions.MySQL)
		dbs.sqlite.Exec(plan.Migrations[0].Instructions.Sqlite)

		// Count columns
		pgCols := introspectPostgresColumns(t, dbs.postgres, tableName)
		myCols := introspectMySQLColumns(t, dbs.mysql, tableName)
		sqCols := introspectSQLiteColumns(t, dbs.sqlite, tableName)

		expectedCount := len(table.Columns)

		if len(pgCols) != expectedCount {
			t.Logf("Postgres column count: expected %d, got %d", expectedCount, len(pgCols))
			return false
		}
		if len(myCols) != expectedCount {
			t.Logf("MySQL column count: expected %d, got %d", expectedCount, len(myCols))
			return false
		}
		if len(sqCols) != expectedCount {
			t.Logf("SQLite column count: expected %d, got %d", expectedCount, len(sqCols))
			return false
		}

		return true
	})
}

// =============================================================================
// Helper Functions for Normalization
// =============================================================================

func normalizePostgresTable(name string, cols []ColumnInfo, origCols []ddl.ColumnDefinition) NormalizedTable {
	normalized := make([]NormalizedColumn, len(cols))
	for i, col := range cols {
		isPrimary := false
		for _, orig := range origCols {
			if orig.Name == col.Name && orig.PrimaryKey {
				isPrimary = true
				break
			}
		}
		normalized[i] = NormalizePostgresColumn(col, isPrimary)
	}
	return NormalizedTable{Name: name, Columns: normalized}
}

func normalizeMySQLTable(name string, cols []MySQLColumnInfo, origCols []ddl.ColumnDefinition) NormalizedTable {
	normalized := make([]NormalizedColumn, len(cols))
	for i, col := range cols {
		isPrimary := false
		for _, orig := range origCols {
			if orig.Name == col.Name && orig.PrimaryKey {
				isPrimary = true
				break
			}
		}
		normalized[i] = NormalizeMySQLColumn(col, isPrimary)
	}
	return NormalizedTable{Name: name, Columns: normalized}
}

func normalizeSQLiteTable(name string, cols []SQLiteColumnInfo) NormalizedTable {
	normalized := make([]NormalizedColumn, len(cols))
	for i, col := range cols {
		normalized[i] = NormalizeSQLiteColumn(col)
	}
	return NormalizedTable{Name: name, Columns: normalized}
}

// =============================================================================
// Autoincrement Cross-Database Integration Tests
// =============================================================================

// TestProperty_CrossDB_Autoincrement_InsertReturnsGeneratedID verifies that
// inserting rows without specifying the PK column succeeds across all databases
// and returns unique, non-zero generated IDs.
func TestProperty_CrossDB_Autoincrement_InsertReturnsGeneratedID(t *testing.T) {
	dbs := connectAllDatabases(t)
	defer dbs.closeAll()

	proptest.Check(t, "autoincrement inserts return generated IDs across DBs", proptest.Config{NumTrials: 10}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dbs.dropAllTables(t, tableName)
		defer dbs.dropAllTables(t, tableName)

		// Create table with autoincrement-eligible PK
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			return nil
		})
		if err != nil {
			t.Logf("AddEmptyTable failed: %v", err)
			return false
		}

		// Execute on all databases
		pgSQL := plan.Migrations[0].Instructions.Postgres
		mySQL := plan.Migrations[0].Instructions.MySQL
		sqSQL := plan.Migrations[0].Instructions.Sqlite

		_, err = dbs.postgres.Exec(context.Background(), pgSQL)
		if err != nil {
			t.Logf("Postgres CREATE TABLE failed: %v\nSQL: %s", err, pgSQL)
			return false
		}

		_, err = dbs.mysql.Exec(mySQL)
		if err != nil {
			t.Logf("MySQL CREATE TABLE failed: %v\nSQL: %s", err, mySQL)
			return false
		}

		_, err = dbs.sqlite.Exec(sqSQL)
		if err != nil {
			t.Logf("SQLite CREATE TABLE failed: %v\nSQL: %s", err, sqSQL)
			return false
		}

		// Insert rows without specifying the PK column
		numRows := g.IntRange(3, 10)
		pgIDs := make([]int64, 0, numRows)
		myIDs := make([]int64, 0, numRows)
		sqIDs := make([]int64, 0, numRows)

		for i := 0; i < numRows; i++ {
			rowName := fmt.Sprintf("row_%d_%s", i, g.IdentifierLower(5))

			// PostgreSQL: INSERT ... RETURNING id
			var pgID int64
			pgInsert := fmt.Sprintf(`INSERT INTO "%s" ("name") VALUES ($1) RETURNING "id"`, tableName)
			err = dbs.postgres.QueryRow(context.Background(), pgInsert, rowName).Scan(&pgID)
			if err != nil {
				t.Logf("Postgres INSERT failed: %v", err)
				return false
			}
			pgIDs = append(pgIDs, pgID)

			// MySQL: INSERT then LAST_INSERT_ID()
			myInsert := fmt.Sprintf("INSERT INTO `%s` (`name`) VALUES (?)", tableName)
			result, err := dbs.mysql.Exec(myInsert, rowName)
			if err != nil {
				t.Logf("MySQL INSERT failed: %v", err)
				return false
			}
			myID, err := result.LastInsertId()
			if err != nil {
				t.Logf("MySQL LastInsertId failed: %v", err)
				return false
			}
			myIDs = append(myIDs, myID)

			// SQLite: INSERT then last_insert_rowid()
			sqInsert := fmt.Sprintf(`INSERT INTO "%s" ("name") VALUES (?)`, tableName)
			result, err = dbs.sqlite.Exec(sqInsert, rowName)
			if err != nil {
				t.Logf("SQLite INSERT failed: %v", err)
				return false
			}
			sqID, err := result.LastInsertId()
			if err != nil {
				t.Logf("SQLite LastInsertId failed: %v", err)
				return false
			}
			sqIDs = append(sqIDs, sqID)
		}

		// Verify: all IDs are non-zero
		for i, id := range pgIDs {
			if id == 0 {
				t.Logf("Postgres ID %d is zero", i)
				return false
			}
		}
		for i, id := range myIDs {
			if id == 0 {
				t.Logf("MySQL ID %d is zero", i)
				return false
			}
		}
		for i, id := range sqIDs {
			if id == 0 {
				t.Logf("SQLite ID %d is zero", i)
				return false
			}
		}

		// Verify: all IDs are unique within each database
		if !allUnique(pgIDs) {
			t.Logf("Postgres IDs are not unique: %v", pgIDs)
			return false
		}
		if !allUnique(myIDs) {
			t.Logf("MySQL IDs are not unique: %v", myIDs)
			return false
		}
		if !allUnique(sqIDs) {
			t.Logf("SQLite IDs are not unique: %v", sqIDs)
			return false
		}

		// Verify: IDs are monotonically increasing (each greater than previous)
		if !isMonotonicallyIncreasing(pgIDs) {
			t.Logf("Postgres IDs are not monotonically increasing: %v", pgIDs)
			return false
		}
		if !isMonotonicallyIncreasing(myIDs) {
			t.Logf("MySQL IDs are not monotonically increasing: %v", myIDs)
			return false
		}
		if !isMonotonicallyIncreasing(sqIDs) {
			t.Logf("SQLite IDs are not monotonically increasing: %v", sqIDs)
			return false
		}

		return true
	})
}

// TestProperty_CrossDB_Autoincrement_ExplicitIDAllowed verifies that explicit
// ID values can be inserted into autoincrement columns (GENERATED BY DEFAULT).
func TestProperty_CrossDB_Autoincrement_ExplicitIDAllowed(t *testing.T) {
	dbs := connectAllDatabases(t)
	defer dbs.closeAll()

	proptest.Check(t, "explicit IDs can be inserted into autoincrement columns", proptest.Config{NumTrials: 5}, func(g *proptest.Generator) bool {
		tableName := GenerateTableName(g)

		dbs.dropAllTables(t, tableName)
		defer dbs.dropAllTables(t, tableName)

		// Create table with autoincrement-eligible PK
		plan := &MigrationPlan{Schema: Schema{Tables: map[string]ddl.Table{}}}
		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			tb.Bigint("id").PrimaryKey()
			tb.String("name")
			return nil
		})
		if err != nil {
			return false
		}

		// Execute on all databases
		_, err = dbs.postgres.Exec(context.Background(), plan.Migrations[0].Instructions.Postgres)
		if err != nil {
			t.Logf("Postgres CREATE TABLE failed: %v", err)
			return false
		}
		_, err = dbs.mysql.Exec(plan.Migrations[0].Instructions.MySQL)
		if err != nil {
			t.Logf("MySQL CREATE TABLE failed: %v", err)
			return false
		}
		_, err = dbs.sqlite.Exec(plan.Migrations[0].Instructions.Sqlite)
		if err != nil {
			t.Logf("SQLite CREATE TABLE failed: %v", err)
			return false
		}

		// Insert with explicit ID
		explicitID := int64(g.IntRange(1000, 9999))

		// PostgreSQL
		pgInsert := fmt.Sprintf(`INSERT INTO "%s" ("id", "name") VALUES ($1, $2)`, tableName)
		_, err = dbs.postgres.Exec(context.Background(), pgInsert, explicitID, "explicit_row")
		if err != nil {
			t.Logf("Postgres INSERT with explicit ID failed: %v", err)
			return false
		}

		// MySQL
		myInsert := fmt.Sprintf("INSERT INTO `%s` (`id`, `name`) VALUES (?, ?)", tableName)
		_, err = dbs.mysql.Exec(myInsert, explicitID, "explicit_row")
		if err != nil {
			t.Logf("MySQL INSERT with explicit ID failed: %v", err)
			return false
		}

		// SQLite
		sqInsert := fmt.Sprintf(`INSERT INTO "%s" ("id", "name") VALUES (?, ?)`, tableName)
		_, err = dbs.sqlite.Exec(sqInsert, explicitID, "explicit_row")
		if err != nil {
			t.Logf("SQLite INSERT with explicit ID failed: %v", err)
			return false
		}

		// Verify the rows exist with the explicit ID
		var pgCount, myCount, sqCount int

		pgQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE "id" = $1`, tableName)
		dbs.postgres.QueryRow(context.Background(), pgQuery, explicitID).Scan(&pgCount)

		myQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE `id` = ?", tableName)
		dbs.mysql.QueryRow(myQuery, explicitID).Scan(&myCount)

		sqQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE "id" = ?`, tableName)
		dbs.sqlite.QueryRow(sqQuery, explicitID).Scan(&sqCount)

		if pgCount != 1 {
			t.Logf("Postgres: expected 1 row with explicit ID, got %d", pgCount)
			return false
		}
		if myCount != 1 {
			t.Logf("MySQL: expected 1 row with explicit ID, got %d", myCount)
			return false
		}
		if sqCount != 1 {
			t.Logf("SQLite: expected 1 row with explicit ID, got %d", sqCount)
			return false
		}

		return true
	})
}

// allUnique returns true if all values in the slice are unique.
func allUnique(ids []int64) bool {
	seen := make(map[int64]bool)
	for _, id := range ids {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// isMonotonicallyIncreasing returns true if each value is greater than the previous.
func isMonotonicallyIncreasing(ids []int64) bool {
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			return false
		}
	}
	return true
}
