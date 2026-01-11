package codegen

import (
	"strings"
	"testing"

	"github.com/portsql/portsql/proptest"
	"github.com/portsql/portsql/src/ddl"
)

// =============================================================================
// Property Tests for CRUD SQL Generation
// =============================================================================

// Property: All three dialects generate valid SQL for INSERT with auto-filled columns.
func TestProperty_InsertSQL_AllDialectsConsistent(t *testing.T) {
	proptest.QuickCheck(t, "INSERT SQL consistency across dialects", func(g *proptest.Generator) bool {
		// Generate a random table with standard columns
		table := generateTableWithStandardColumns(g)

		// Generate SQL for all dialects
		pgSQL := GenerateCRUDSQL(table, DialectPostgres, CRUDOptions{})
		mySQL := GenerateCRUDSQL(table, DialectMySQL, CRUDOptions{})
		sqSQL := GenerateCRUDSQL(table, DialectSQLite, CRUDOptions{})

		// Property: All start with INSERT INTO
		if !strings.HasPrefix(pgSQL.InsertSQL, "INSERT INTO") {
			t.Logf("Postgres INSERT doesn't start with INSERT INTO: %s", pgSQL.InsertSQL)
			return false
		}
		if !strings.HasPrefix(mySQL.InsertSQL, "INSERT INTO") {
			t.Logf("MySQL INSERT doesn't start with INSERT INTO: %s", mySQL.InsertSQL)
			return false
		}
		if !strings.HasPrefix(sqSQL.InsertSQL, "INSERT INTO") {
			t.Logf("SQLite INSERT doesn't start with INSERT INTO: %s", sqSQL.InsertSQL)
			return false
		}

		// Property: If table has created_at, all should use NOW() or datetime('now')
		analysis := AnalyzeTable(table)
		if analysis.HasCreatedAt {
			if !strings.Contains(pgSQL.InsertSQL, "NOW()") {
				t.Logf("Postgres INSERT should have NOW() for created_at: %s", pgSQL.InsertSQL)
				return false
			}
			if !strings.Contains(mySQL.InsertSQL, "NOW()") {
				t.Logf("MySQL INSERT should have NOW() for created_at: %s", mySQL.InsertSQL)
				return false
			}
			if !strings.Contains(sqSQL.InsertSQL, "datetime('now')") {
				t.Logf("SQLite INSERT should have datetime('now') for created_at: %s", sqSQL.InsertSQL)
				return false
			}
		}

		// Property: If table has public_id, Postgres and SQLite should have RETURNING
		if analysis.HasPublicID {
			if !strings.Contains(pgSQL.InsertSQL, "RETURNING") {
				t.Logf("Postgres INSERT should have RETURNING for public_id: %s", pgSQL.InsertSQL)
				return false
			}
			if !strings.Contains(sqSQL.InsertSQL, "RETURNING") {
				t.Logf("SQLite INSERT should have RETURNING for public_id: %s", sqSQL.InsertSQL)
				return false
			}
			// MySQL should NOT have RETURNING
			if strings.Contains(mySQL.InsertSQL, "RETURNING") {
				t.Logf("MySQL INSERT should NOT have RETURNING: %s", mySQL.InsertSQL)
				return false
			}
		}

		return true
	})
}

// Property: All three dialects generate valid SQL for UPDATE with auto-filled updated_at.
func TestProperty_UpdateSQL_AllDialectsConsistent(t *testing.T) {
	proptest.QuickCheck(t, "UPDATE SQL consistency across dialects", func(g *proptest.Generator) bool {
		table := generateTableWithStandardColumns(g)

		pgSQL := GenerateCRUDSQL(table, DialectPostgres, CRUDOptions{})
		mySQL := GenerateCRUDSQL(table, DialectMySQL, CRUDOptions{})
		sqSQL := GenerateCRUDSQL(table, DialectSQLite, CRUDOptions{})

		// Property: All start with UPDATE
		if !strings.HasPrefix(pgSQL.UpdateSQL, "UPDATE") {
			t.Logf("Postgres UPDATE doesn't start with UPDATE: %s", pgSQL.UpdateSQL)
			return false
		}
		if !strings.HasPrefix(mySQL.UpdateSQL, "UPDATE") {
			t.Logf("MySQL UPDATE doesn't start with UPDATE: %s", mySQL.UpdateSQL)
			return false
		}
		if !strings.HasPrefix(sqSQL.UpdateSQL, "UPDATE") {
			t.Logf("SQLite UPDATE doesn't start with UPDATE: %s", sqSQL.UpdateSQL)
			return false
		}

		// Property: If table has updated_at, all should auto-fill it
		analysis := AnalyzeTable(table)
		if analysis.HasUpdatedAt {
			if !strings.Contains(pgSQL.UpdateSQL, "NOW()") {
				t.Logf("Postgres UPDATE should have NOW() for updated_at: %s", pgSQL.UpdateSQL)
				return false
			}
			if !strings.Contains(mySQL.UpdateSQL, "NOW()") {
				t.Logf("MySQL UPDATE should have NOW() for updated_at: %s", mySQL.UpdateSQL)
				return false
			}
			if !strings.Contains(sqSQL.UpdateSQL, "datetime('now')") {
				t.Logf("SQLite UPDATE should have datetime('now') for updated_at: %s", sqSQL.UpdateSQL)
				return false
			}
		}

		// Property: If table has deleted_at, UPDATE should exclude soft-deleted records
		if analysis.HasDeletedAt {
			if !strings.Contains(pgSQL.UpdateSQL, "deleted_at") || !strings.Contains(pgSQL.UpdateSQL, "IS NULL") {
				t.Logf("Postgres UPDATE should exclude soft-deleted: %s", pgSQL.UpdateSQL)
				return false
			}
			if !strings.Contains(mySQL.UpdateSQL, "deleted_at") || !strings.Contains(mySQL.UpdateSQL, "IS NULL") {
				t.Logf("MySQL UPDATE should exclude soft-deleted: %s", mySQL.UpdateSQL)
				return false
			}
			if !strings.Contains(sqSQL.UpdateSQL, "deleted_at") || !strings.Contains(sqSQL.UpdateSQL, "IS NULL") {
				t.Logf("SQLite UPDATE should exclude soft-deleted: %s", sqSQL.UpdateSQL)
				return false
			}
		}

		return true
	})
}

// Property: Soft delete generates UPDATE (not DELETE) when deleted_at exists.
func TestProperty_SoftDelete_GeneratesUpdate(t *testing.T) {
	proptest.QuickCheck(t, "soft delete generates UPDATE when deleted_at exists", func(g *proptest.Generator) bool {
		table := generateTableWithDeletedAt(g)

		for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Property: DeleteSQL should be UPDATE, not DELETE
			if strings.HasPrefix(strings.ToUpper(sqlSet.DeleteSQL), "DELETE") {
				t.Logf("%s: DeleteSQL should be UPDATE for soft delete, got: %s", dialect, sqlSet.DeleteSQL)
				return false
			}

			// Property: DeleteSQL should set deleted_at
			if !strings.Contains(sqlSet.DeleteSQL, "deleted_at") {
				t.Logf("%s: DeleteSQL should reference deleted_at: %s", dialect, sqlSet.DeleteSQL)
				return false
			}

			// Property: HardDeleteSQL should be actual DELETE
			if !strings.HasPrefix(strings.ToUpper(sqlSet.HardDeleteSQL), "DELETE") {
				t.Logf("%s: HardDeleteSQL should be DELETE, got: %s", dialect, sqlSet.HardDeleteSQL)
				return false
			}
		}

		return true
	})
}

// Property: Hard delete is used when no deleted_at column exists.
func TestProperty_HardDelete_WhenNoDeletedAt(t *testing.T) {
	proptest.QuickCheck(t, "hard delete when no deleted_at", func(g *proptest.Generator) bool {
		table := generateTableWithoutDeletedAt(g)

		for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Property: DeleteSQL should be DELETE when no deleted_at
			if !strings.HasPrefix(strings.ToUpper(sqlSet.DeleteSQL), "DELETE") {
				t.Logf("%s: DeleteSQL should be DELETE when no deleted_at, got: %s", dialect, sqlSet.DeleteSQL)
				return false
			}

			// Property: HardDeleteSQL should be empty (no separate hard delete needed)
			if sqlSet.HardDeleteSQL != "" {
				t.Logf("%s: HardDeleteSQL should be empty when no deleted_at, got: %s", dialect, sqlSet.HardDeleteSQL)
				return false
			}
		}

		return true
	})
}

// Property: Parameter placeholders use correct syntax per dialect.
func TestProperty_ParameterPlaceholders_CorrectPerDialect(t *testing.T) {
	proptest.QuickCheck(t, "parameter placeholders correct per dialect", func(g *proptest.Generator) bool {
		table := generateTableWithStandardColumns(g)

		pgSQL := GenerateCRUDSQL(table, DialectPostgres, CRUDOptions{})
		mySQL := GenerateCRUDSQL(table, DialectMySQL, CRUDOptions{})
		sqSQL := GenerateCRUDSQL(table, DialectSQLite, CRUDOptions{})

		// Property: Postgres uses $1, $2, etc.
		if !strings.Contains(pgSQL.InsertSQL, "$") {
			t.Logf("Postgres should use $ placeholders: %s", pgSQL.InsertSQL)
			return false
		}
		if strings.Contains(pgSQL.InsertSQL, "?") {
			t.Logf("Postgres should not use ? placeholders: %s", pgSQL.InsertSQL)
			return false
		}

		// Property: MySQL uses ?
		if !strings.Contains(mySQL.InsertSQL, "?") {
			t.Logf("MySQL should use ? placeholders: %s", mySQL.InsertSQL)
			return false
		}
		if strings.Contains(mySQL.InsertSQL, "$") {
			t.Logf("MySQL should not use $ placeholders: %s", mySQL.InsertSQL)
			return false
		}

		// Property: SQLite uses ?
		if !strings.Contains(sqSQL.InsertSQL, "?") {
			t.Logf("SQLite should use ? placeholders: %s", sqSQL.InsertSQL)
			return false
		}
		if strings.Contains(sqSQL.InsertSQL, "$") {
			t.Logf("SQLite should not use $ placeholders: %s", sqSQL.InsertSQL)
			return false
		}

		return true
	})
}

// Property: Identifier quoting is correct per dialect.
func TestProperty_IdentifierQuoting_CorrectPerDialect(t *testing.T) {
	proptest.QuickCheck(t, "identifier quoting correct per dialect", func(g *proptest.Generator) bool {
		table := generateTableWithStandardColumns(g)

		pgSQL := GenerateCRUDSQL(table, DialectPostgres, CRUDOptions{})
		mySQL := GenerateCRUDSQL(table, DialectMySQL, CRUDOptions{})
		sqSQL := GenerateCRUDSQL(table, DialectSQLite, CRUDOptions{})

		// Property: Postgres uses double quotes
		if !strings.Contains(pgSQL.InsertSQL, `"`) {
			t.Logf("Postgres should use double quotes: %s", pgSQL.InsertSQL)
			return false
		}
		if strings.Contains(pgSQL.InsertSQL, "`") {
			t.Logf("Postgres should not use backticks: %s", pgSQL.InsertSQL)
			return false
		}

		// Property: MySQL uses backticks
		if !strings.Contains(mySQL.InsertSQL, "`") {
			t.Logf("MySQL should use backticks: %s", mySQL.InsertSQL)
			return false
		}
		if strings.Contains(mySQL.InsertSQL, `"`) {
			t.Logf("MySQL should not use double quotes: %s", mySQL.InsertSQL)
			return false
		}

		// Property: SQLite uses double quotes
		if !strings.Contains(sqSQL.InsertSQL, `"`) {
			t.Logf("SQLite should use double quotes: %s", sqSQL.InsertSQL)
			return false
		}
		if strings.Contains(sqSQL.InsertSQL, "`") {
			t.Logf("SQLite should not use backticks: %s", sqSQL.InsertSQL)
			return false
		}

		return true
	})
}

// Property: Auto-filled columns are excluded from user params.
func TestProperty_AutoFilledColumns_ExcludedFromParams(t *testing.T) {
	proptest.QuickCheck(t, "auto-filled columns excluded from params", func(g *proptest.Generator) bool {
		table := generateTableWithAllStandardColumns(g)
		analysis := AnalyzeTable(table)

		// Property: UserColumns should not contain auto-filled columns
		for _, col := range analysis.UserColumns {
			if col.Name == "id" || col.Name == "public_id" ||
				col.Name == "created_at" || col.Name == "updated_at" ||
				col.Name == "deleted_at" {
				t.Logf("UserColumns should not contain %s", col.Name)
				return false
			}
		}

		// Property: ResultColumns should not contain internal id or deleted_at
		for _, col := range analysis.ResultColumns {
			if col.Name == "id" || col.Name == "deleted_at" {
				t.Logf("ResultColumns should not contain %s", col.Name)
				return false
			}
		}

		return true
	})
}

// Property: GET query excludes soft-deleted records.
func TestProperty_GetSQL_ExcludesSoftDeleted(t *testing.T) {
	proptest.QuickCheck(t, "GET excludes soft deleted", func(g *proptest.Generator) bool {
		table := generateTableWithDeletedAt(g)

		for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Property: GetSQL should have deleted_at IS NULL
			if !strings.Contains(sqlSet.GetSQL, "deleted_at") ||
				!strings.Contains(sqlSet.GetSQL, "IS NULL") {
				t.Logf("%s: GetSQL should exclude soft-deleted: %s", dialect, sqlSet.GetSQL)
				return false
			}
		}

		return true
	})
}

// Property: LIST query excludes soft-deleted records.
func TestProperty_ListSQL_ExcludesSoftDeleted(t *testing.T) {
	proptest.QuickCheck(t, "LIST excludes soft deleted", func(g *proptest.Generator) bool {
		table := generateTableWithDeletedAt(g)

		for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Property: ListSQL should have deleted_at IS NULL
			if !strings.Contains(sqlSet.ListSQL, "deleted_at") ||
				!strings.Contains(sqlSet.ListSQL, "IS NULL") {
				t.Logf("%s: ListSQL should exclude soft-deleted: %s", dialect, sqlSet.ListSQL)
				return false
			}
		}

		return true
	})
}

// =============================================================================
// Table Generators for Property Tests
// =============================================================================

// generateTableWithStandardColumns generates a table with some standard columns.
func generateTableWithStandardColumns(g *proptest.Generator) ddl.Table {
	tableName := "test_" + g.StringAlphaNum(g.IntRange(3, 10))

	columns := []ddl.ColumnDefinition{
		{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
	}

	// 70% chance of public_id
	if g.BoolWithProb(0.7) {
		columns = append(columns, ddl.ColumnDefinition{Name: "public_id", Type: ddl.StringType})
	}

	// Add 1-3 user columns
	numUserCols := g.IntRange(1, 3)
	for i := 0; i < numUserCols; i++ {
		colName := "col_" + g.StringAlphaNum(g.IntRange(3, 8))
		colType := proptest.Pick(g, []string{ddl.StringType, ddl.IntegerType, ddl.BooleanType})
		columns = append(columns, ddl.ColumnDefinition{
			Name:     colName,
			Type:     colType,
			Nullable: g.BoolWithProb(0.3),
		})
	}

	// 80% chance of created_at
	if g.BoolWithProb(0.8) {
		columns = append(columns, ddl.ColumnDefinition{Name: "created_at", Type: ddl.DatetimeType})
	}

	// 80% chance of updated_at
	if g.BoolWithProb(0.8) {
		columns = append(columns, ddl.ColumnDefinition{Name: "updated_at", Type: ddl.DatetimeType})
	}

	// 50% chance of deleted_at
	if g.BoolWithProb(0.5) {
		columns = append(columns, ddl.ColumnDefinition{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true})
	}

	return ddl.Table{Name: tableName, Columns: columns}
}

// generateTableWithDeletedAt generates a table that always has deleted_at.
func generateTableWithDeletedAt(g *proptest.Generator) ddl.Table {
	tableName := "test_" + g.StringAlphaNum(g.IntRange(3, 10))

	columns := []ddl.ColumnDefinition{
		{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
		{Name: "public_id", Type: ddl.StringType},
		{Name: "name", Type: ddl.StringType},
		{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
	}

	return ddl.Table{Name: tableName, Columns: columns}
}

// generateTableWithoutDeletedAt generates a table without deleted_at.
func generateTableWithoutDeletedAt(g *proptest.Generator) ddl.Table {
	tableName := "test_" + g.StringAlphaNum(g.IntRange(3, 10))

	columns := []ddl.ColumnDefinition{
		{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
	}

	// 70% chance of public_id
	if g.BoolWithProb(0.7) {
		columns = append(columns, ddl.ColumnDefinition{Name: "public_id", Type: ddl.StringType})
	}

	// Add user columns
	columns = append(columns, ddl.ColumnDefinition{Name: "name", Type: ddl.StringType})

	return ddl.Table{Name: tableName, Columns: columns}
}

// generateTableWithAllStandardColumns generates a table with ALL standard columns.
func generateTableWithAllStandardColumns(g *proptest.Generator) ddl.Table {
	tableName := "test_" + g.StringAlphaNum(g.IntRange(3, 10))

	columns := []ddl.ColumnDefinition{
		{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
		{Name: "public_id", Type: ddl.StringType},
		{Name: "name", Type: ddl.StringType},
		{Name: "email", Type: ddl.StringType},
		{Name: "created_at", Type: ddl.DatetimeType},
		{Name: "updated_at", Type: ddl.DatetimeType},
		{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
	}

	return ddl.Table{Name: tableName, Columns: columns}
}
