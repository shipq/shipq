//go:build property

package migrate

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/proptest"
)

// =============================================================================
// Autoincrement Property Test Generators
// =============================================================================

// generateTableWithSingleIntegerPK generates a table with a single integer/bigint PK.
func generateTableWithSingleIntegerPK(g *proptest.Generator) *ddl.Table {
	tableName := g.IdentifierLower(15)
	numExtraCols := g.IntRange(1, 5)
	columnNames := g.UniqueIdentifiers(numExtraCols+1, 12)

	// First column is the PK
	pkName := columnNames[0]
	pkType := proptest.Pick(g, []string{ddl.IntegerType, ddl.BigintType})

	columns := []ddl.ColumnDefinition{
		{
			Name:       pkName,
			Type:       pkType,
			PrimaryKey: true,
			Nullable:   false,
		},
	}

	// Add extra columns with various types
	for i := 1; i < len(columnNames); i++ {
		colType := proptest.Pick(g, []string{
			ddl.StringType,
			ddl.TextType,
			ddl.BooleanType,
			ddl.DatetimeType,
			ddl.FloatType,
		})
		col := ddl.ColumnDefinition{
			Name:     columnNames[i],
			Type:     colType,
			Nullable: g.Bool(),
		}
		if colType == ddl.StringType {
			length := g.IntRange(1, 255)
			col.Length = &length
		}
		columns = append(columns, col)
	}

	return &ddl.Table{
		Name:    tableName,
		Columns: columns,
		Indexes: []ddl.IndexDefinition{},
	}
}

// generateTableWithCompositePK generates a table with a composite (multi-column) PK.
func generateTableWithCompositePK(g *proptest.Generator) *ddl.Table {
	tableName := g.IdentifierLower(15)
	numPKCols := g.IntRange(2, 4)
	numExtraCols := g.IntRange(0, 3)
	columnNames := g.UniqueIdentifiers(numPKCols+numExtraCols, 12)

	columns := make([]ddl.ColumnDefinition, 0, len(columnNames))

	// PK columns (all integers)
	for i := 0; i < numPKCols; i++ {
		pkType := proptest.Pick(g, []string{ddl.IntegerType, ddl.BigintType})
		columns = append(columns, ddl.ColumnDefinition{
			Name:       columnNames[i],
			Type:       pkType,
			PrimaryKey: true,
			Nullable:   false,
		})
	}

	// Extra columns
	for i := numPKCols; i < len(columnNames); i++ {
		colType := proptest.Pick(g, []string{
			ddl.StringType,
			ddl.BooleanType,
			ddl.DatetimeType,
		})
		col := ddl.ColumnDefinition{
			Name:     columnNames[i],
			Type:     colType,
			Nullable: g.Bool(),
		}
		if colType == ddl.StringType {
			length := g.IntRange(1, 255)
			col.Length = &length
		}
		columns = append(columns, col)
	}

	return &ddl.Table{
		Name:    tableName,
		Columns: columns,
		Indexes: []ddl.IndexDefinition{},
	}
}

// generateTableWithNonIntegerPK generates a table with a non-integer (string) PK.
func generateTableWithNonIntegerPK(g *proptest.Generator) *ddl.Table {
	tableName := g.IdentifierLower(15)
	numExtraCols := g.IntRange(1, 5)
	columnNames := g.UniqueIdentifiers(numExtraCols+1, 12)

	// First column is the string PK
	pkName := columnNames[0]
	length := g.IntRange(1, 255)

	columns := []ddl.ColumnDefinition{
		{
			Name:       pkName,
			Type:       ddl.StringType,
			Length:     &length,
			PrimaryKey: true,
			Nullable:   false,
		},
	}

	// Add extra columns
	for i := 1; i < len(columnNames); i++ {
		colType := proptest.Pick(g, []string{
			ddl.IntegerType,
			ddl.BigintType,
			ddl.BooleanType,
			ddl.DatetimeType,
		})
		columns = append(columns, ddl.ColumnDefinition{
			Name:     columnNames[i],
			Type:     colType,
			Nullable: g.Bool(),
		})
	}

	return &ddl.Table{
		Name:    tableName,
		Columns: columns,
		Indexes: []ddl.IndexDefinition{},
	}
}

// generateJunctionTable generates a junction table with single integer PK but IsJunctionTable=true.
func generateJunctionTable(g *proptest.Generator) *ddl.Table {
	tableName := g.IdentifierLower(15) + "_junction"
	columnNames := g.UniqueIdentifiers(3, 12)

	columns := []ddl.ColumnDefinition{
		{
			Name:       columnNames[0],
			Type:       ddl.BigintType,
			PrimaryKey: true,
			Nullable:   false,
		},
		{
			Name:     columnNames[1],
			Type:     ddl.BigintType,
			Nullable: false,
		},
		{
			Name:     columnNames[2],
			Type:     ddl.BigintType,
			Nullable: false,
		},
	}

	return &ddl.Table{
		Name:            tableName,
		Columns:         columns,
		Indexes:         []ddl.IndexDefinition{},
		IsJunctionTable: true,
	}
}

// =============================================================================
// Property Tests - Invariant A: Non-eligible tables should NOT have autoincrement
// =============================================================================

// TestProperty_CompositePK_NoAutoincrement verifies that tables with composite PKs
// do not get autoincrement behavior in any dialect.
func TestProperty_CompositePK_NoAutoincrement(t *testing.T) {
	proptest.QuickCheck(t, "composite PK has no autoincrement", func(g *proptest.Generator) bool {
		table := generateTableWithCompositePK(g)

		// Check eligibility first
		_, ok := GetAutoincrementPK(table)
		if ok {
			t.Log("Composite PK table should not be autoincrement eligible")
			return false
		}

		// Generate SQL for all dialects and verify no autoincrement syntax
		pgSQL := generatePostgresCreateTable(table)
		if strings.Contains(pgSQL, "GENERATED") || strings.Contains(pgSQL, "IDENTITY") {
			t.Logf("Postgres SQL contains GENERATED/IDENTITY for composite PK:\n%s", pgSQL)
			return false
		}

		mySQL := generateMySQLCreateTable(table)
		if strings.Contains(mySQL, "AUTO_INCREMENT") {
			t.Logf("MySQL SQL contains AUTO_INCREMENT for composite PK:\n%s", mySQL)
			return false
		}

		sqliteSQL := generateSQLiteCreateTable(table)
		if strings.Contains(sqliteSQL, "AUTOINCREMENT") {
			t.Logf("SQLite SQL contains AUTOINCREMENT for composite PK:\n%s", sqliteSQL)
			return false
		}

		return true
	})
}

// TestProperty_NonIntegerPK_NoAutoincrement verifies that tables with non-integer PKs
// do not get autoincrement behavior in any dialect.
func TestProperty_NonIntegerPK_NoAutoincrement(t *testing.T) {
	proptest.QuickCheck(t, "non-integer PK has no autoincrement", func(g *proptest.Generator) bool {
		table := generateTableWithNonIntegerPK(g)

		// Check eligibility first
		_, ok := GetAutoincrementPK(table)
		if ok {
			t.Log("Non-integer PK table should not be autoincrement eligible")
			return false
		}

		// Generate SQL for all dialects and verify no autoincrement syntax
		pgSQL := generatePostgresCreateTable(table)
		if strings.Contains(pgSQL, "GENERATED") || strings.Contains(pgSQL, "IDENTITY") {
			t.Logf("Postgres SQL contains GENERATED/IDENTITY for non-integer PK:\n%s", pgSQL)
			return false
		}

		mySQL := generateMySQLCreateTable(table)
		if strings.Contains(mySQL, "AUTO_INCREMENT") {
			t.Logf("MySQL SQL contains AUTO_INCREMENT for non-integer PK:\n%s", mySQL)
			return false
		}

		sqliteSQL := generateSQLiteCreateTable(table)
		if strings.Contains(sqliteSQL, "AUTOINCREMENT") {
			t.Logf("SQLite SQL contains AUTOINCREMENT for non-integer PK:\n%s", sqliteSQL)
			return false
		}

		return true
	})
}

// TestProperty_JunctionTable_NoAutoincrement verifies that junction tables
// do not get autoincrement behavior even with single integer PK.
func TestProperty_JunctionTable_NoAutoincrement(t *testing.T) {
	proptest.QuickCheck(t, "junction table has no autoincrement", func(g *proptest.Generator) bool {
		table := generateJunctionTable(g)

		// Check eligibility first
		_, ok := GetAutoincrementPK(table)
		if ok {
			t.Log("Junction table should not be autoincrement eligible")
			return false
		}

		// Generate SQL for all dialects and verify no autoincrement syntax
		pgSQL := generatePostgresCreateTable(table)
		if strings.Contains(pgSQL, "GENERATED") || strings.Contains(pgSQL, "IDENTITY") {
			t.Logf("Postgres SQL contains GENERATED/IDENTITY for junction table:\n%s", pgSQL)
			return false
		}

		mySQL := generateMySQLCreateTable(table)
		if strings.Contains(mySQL, "AUTO_INCREMENT") {
			t.Logf("MySQL SQL contains AUTO_INCREMENT for junction table:\n%s", mySQL)
			return false
		}

		sqliteSQL := generateSQLiteCreateTable(table)
		if strings.Contains(sqliteSQL, "AUTOINCREMENT") {
			t.Logf("SQLite SQL contains AUTOINCREMENT for junction table:\n%s", sqliteSQL)
			return false
		}

		return true
	})
}

// =============================================================================
// Property Tests - Invariant B: Eligible tables MUST have autoincrement
// =============================================================================

// TestProperty_SingleIntegerPK_HasAutoincrement verifies that tables with
// single integer/bigint PK get appropriate autoincrement in each dialect.
func TestProperty_SingleIntegerPK_HasAutoincrement(t *testing.T) {
	proptest.QuickCheck(t, "single integer PK has autoincrement", func(g *proptest.Generator) bool {
		table := generateTableWithSingleIntegerPK(g)

		// Check eligibility first
		pkInfo, ok := GetAutoincrementPK(table)
		if !ok {
			t.Log("Single integer PK table should be autoincrement eligible")
			return false
		}

		// Postgres: should have GENERATED BY DEFAULT AS IDENTITY
		pgSQL := generatePostgresCreateTable(table)
		if !strings.Contains(pgSQL, "GENERATED BY DEFAULT AS IDENTITY") {
			t.Logf("Postgres SQL missing GENERATED BY DEFAULT AS IDENTITY for single integer PK:\n%s", pgSQL)
			return false
		}

		// MySQL: should have AUTO_INCREMENT
		mySQL := generateMySQLCreateTable(table)
		if !strings.Contains(mySQL, "AUTO_INCREMENT") {
			t.Logf("MySQL SQL missing AUTO_INCREMENT for single integer PK:\n%s", mySQL)
			return false
		}

		// SQLite: should have INTEGER PRIMARY KEY (for rowid alias)
		sqliteSQL := generateSQLiteCreateTable(table)
		expectedSQLite := `"` + pkInfo.ColumnName + `" INTEGER PRIMARY KEY`
		if !strings.Contains(sqliteSQL, expectedSQLite) {
			t.Logf("SQLite SQL missing INTEGER PRIMARY KEY for single integer PK:\n%s", sqliteSQL)
			return false
		}

		return true
	})
}

// =============================================================================
// Property Tests - Invariant C: Eligible PK columns should NOT have DEFAULT
// =============================================================================

// TestProperty_AutoincrementPK_NoDefaultInSQL verifies that even if a DEFAULT
// is specified on an autoincrement-eligible PK column, it is not emitted in SQL.
func TestProperty_AutoincrementPK_NoDefaultInSQL(t *testing.T) {
	proptest.QuickCheck(t, "autoincrement PK has no DEFAULT in SQL", func(g *proptest.Generator) bool {
		table := generateTableWithSingleIntegerPK(g)

		// Add a default value to the PK column (this should be ignored)
		defaultVal := "42"
		table.Columns[0].Default = &defaultVal

		// Check eligibility
		_, ok := GetAutoincrementPK(table)
		if !ok {
			t.Log("Single integer PK table should be autoincrement eligible")
			return false
		}

		// Postgres: should NOT have DEFAULT 42
		pgSQL := generatePostgresCreateTable(table)
		if strings.Contains(pgSQL, "DEFAULT 42") || strings.Contains(pgSQL, "DEFAULT '42'") {
			t.Logf("Postgres SQL contains DEFAULT for autoincrement PK:\n%s", pgSQL)
			return false
		}

		// MySQL: should NOT have DEFAULT 42
		mySQL := generateMySQLCreateTable(table)
		if strings.Contains(mySQL, "DEFAULT 42") || strings.Contains(mySQL, "DEFAULT '42'") {
			t.Logf("MySQL SQL contains DEFAULT for autoincrement PK:\n%s", mySQL)
			return false
		}

		// SQLite: should NOT have DEFAULT 42
		sqliteSQL := generateSQLiteCreateTable(table)
		if strings.Contains(sqliteSQL, "DEFAULT 42") || strings.Contains(sqliteSQL, "DEFAULT '42'") {
			t.Logf("SQLite SQL contains DEFAULT for autoincrement PK:\n%s", sqliteSQL)
			return false
		}

		return true
	})
}

// =============================================================================
// Property Tests - Dialect Consistency
// =============================================================================

// TestProperty_AllDialectsAgreeOnEligibility verifies that all dialects produce
// consistent autoincrement behavior based on the same eligibility rules.
func TestProperty_AllDialectsAgreeOnEligibility(t *testing.T) {
	proptest.QuickCheck(t, "all dialects agree on eligibility", func(g *proptest.Generator) bool {
		// Generate a random table (could be any type)
		tableType := g.IntRange(0, 3)
		var table *ddl.Table

		switch tableType {
		case 0:
			table = generateTableWithSingleIntegerPK(g)
		case 1:
			table = generateTableWithCompositePK(g)
		case 2:
			table = generateTableWithNonIntegerPK(g)
		case 3:
			table = generateJunctionTable(g)
		}

		_, eligible := GetAutoincrementPK(table)

		pgSQL := generatePostgresCreateTable(table)
		mySQL := generateMySQLCreateTable(table)
		sqliteSQL := generateSQLiteCreateTable(table)

		pgHasAutoincrement := strings.Contains(pgSQL, "GENERATED") && strings.Contains(pgSQL, "IDENTITY")
		myHasAutoincrement := strings.Contains(mySQL, "AUTO_INCREMENT")
		// Note: SQLite uses INTEGER PRIMARY KEY which provides autoincrement semantics
		// We just check for the absence of explicit AUTOINCREMENT keyword when not eligible

		// All dialects should agree on whether autoincrement syntax is present
		if eligible {
			if !pgHasAutoincrement {
				t.Logf("Eligible table missing Postgres autoincrement:\n%s", pgSQL)
				return false
			}
			if !myHasAutoincrement {
				t.Logf("Eligible table missing MySQL autoincrement:\n%s", mySQL)
				return false
			}
			// SQLite eligible tables just need INTEGER PRIMARY KEY, which is checked elsewhere
		} else {
			if pgHasAutoincrement {
				t.Logf("Non-eligible table has Postgres autoincrement:\n%s", pgSQL)
				return false
			}
			if myHasAutoincrement {
				t.Logf("Non-eligible table has MySQL autoincrement:\n%s", mySQL)
				return false
			}
			if strings.Contains(sqliteSQL, "AUTOINCREMENT") {
				t.Logf("Non-eligible table has SQLite AUTOINCREMENT:\n%s", sqliteSQL)
				return false
			}
		}

		return true
	})
}

// =============================================================================
// Property Tests - SQL Validity
// =============================================================================

// TestProperty_AutoincrementSQL_HasValidSyntax verifies that generated SQL
// for autoincrement tables has valid syntax patterns.
func TestProperty_AutoincrementSQL_HasValidSyntax(t *testing.T) {
	proptest.QuickCheck(t, "autoincrement SQL has valid syntax", func(g *proptest.Generator) bool {
		table := generateTableWithSingleIntegerPK(g)

		pkInfo, ok := GetAutoincrementPK(table)
		if !ok {
			return true // Skip non-eligible tables
		}

		// Postgres: column should have type followed by GENERATED BY DEFAULT AS IDENTITY
		pgSQL := generatePostgresCreateTable(table)
		expectedPgType := "INTEGER"
		if pkInfo.ColumnType == ddl.BigintType {
			expectedPgType = "BIGINT"
		}
		expectedPgPattern := `"` + pkInfo.ColumnName + `" ` + expectedPgType + ` GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY`
		if !strings.Contains(pgSQL, expectedPgPattern) {
			t.Logf("Postgres SQL has unexpected pattern for autoincrement PK. Expected pattern containing: %s\nGot:\n%s", expectedPgPattern, pgSQL)
			return false
		}

		// MySQL: column should have type followed by NOT NULL AUTO_INCREMENT PRIMARY KEY
		mySQL := generateMySQLCreateTable(table)
		expectedMyType := "INT"
		if pkInfo.ColumnType == ddl.BigintType {
			expectedMyType = "BIGINT"
		}
		expectedMyPattern := "`" + pkInfo.ColumnName + "` " + expectedMyType + " NOT NULL AUTO_INCREMENT PRIMARY KEY"
		if !strings.Contains(mySQL, expectedMyPattern) {
			t.Logf("MySQL SQL has unexpected pattern for autoincrement PK. Expected pattern containing: %s\nGot:\n%s", expectedMyPattern, mySQL)
			return false
		}

		// SQLite: column should have INTEGER PRIMARY KEY (exactly, for rowid alias)
		sqliteSQL := generateSQLiteCreateTable(table)
		expectedSQLitePattern := `"` + pkInfo.ColumnName + `" INTEGER PRIMARY KEY`
		if !strings.Contains(sqliteSQL, expectedSQLitePattern) {
			t.Logf("SQLite SQL has unexpected pattern for autoincrement PK. Expected pattern containing: %s\nGot:\n%s", expectedSQLitePattern, sqliteSQL)
			return false
		}

		return true
	})
}
