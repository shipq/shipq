//go:build property

package cli

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/db/proptest"
)

// =============================================================================
// Random Generators
// =============================================================================

// generateRandomAddTableSchema generates a random schema with AddTable tables.
func generateRandomAddTableSchema(g *proptest.Generator) *migrate.MigrationPlan {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}

	numTables := g.IntRange(1, 5)
	tableNames := g.UniqueIdentifiers(numTables, 15)

	for _, tableName := range tableNames {
		// Create an AddTable-style table (with standard columns)
		columns := []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType, Unique: true},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType},
		}

		// Add random user columns
		numUserCols := g.IntRange(1, 5)
		colNames := g.UniqueIdentifiers(numUserCols, 12)
		for _, colName := range colNames {
			// Skip reserved column names
			if colName == "id" || colName == "public_id" || colName == "created_at" ||
				colName == "updated_at" || colName == "deleted_at" {
				continue
			}

			colType := proptest.Pick(g, []string{
				ddl.StringType, ddl.BigintType, ddl.BooleanType, ddl.FloatType,
			})
			columns = append(columns, ddl.ColumnDefinition{
				Name:     colName,
				Type:     colType,
				Nullable: g.Bool(),
			})
		}

		plan.Schema.Tables[tableName] = ddl.Table{
			Name:    tableName,
			Columns: columns,
		}
	}

	return plan
}

// generateRandomCRUDConfig generates a random CRUD configuration.
func generateRandomCRUDConfig(g *proptest.Generator, tableNames []string) CRUDConfig {
	config := CRUDConfig{
		TableScopes: make(map[string]string),
	}

	// 70% chance of having a global scope
	if g.Float64() < 0.7 {
		config.GlobalScope = g.Identifier(10) + "_id"
	}

	// Random per-table overrides
	for _, tableName := range tableNames {
		// 30% chance of having a table-specific scope
		if g.Float64() < 0.3 {
			if g.Bool() {
				// Override with different scope
				config.TableScopes[tableName] = g.Identifier(8) + "_id"
			} else {
				// Override with empty (no scope)
				config.TableScopes[tableName] = ""
			}
		}
	}

	return config
}

// =============================================================================
// Property Tests
// =============================================================================

// TestProperty_CRUDGenerationProducesValidGo verifies CRUD generates valid Go.
func TestProperty_CRUDGenerationProducesValidGo(t *testing.T) {
	proptest.QuickCheck(t, "crud generates valid Go", func(g *proptest.Generator) bool {
		plan := generateRandomAddTableSchema(g)

		if len(plan.Schema.Tables) == 0 {
			return true
		}

		// Get table names for config generation
		var tableNames []string
		for name := range plan.Schema.Tables {
			tableNames = append(tableNames, name)
		}

		config := generateRandomCRUDConfig(g, tableNames)

		// Build table options
		tableOpts := make(map[string]codegen.CRUDOptions)
		for _, name := range tableNames {
			tableOpts[name] = codegen.CRUDOptions{
				ScopeColumn: config.GetScopeForTable(name),
			}
		}

		// Generate CRUD types
		code, err := codegen.GenerateSharedTypes(nil, plan, "queries", tableOpts)
		if err != nil {
			t.Logf("GenerateSharedTypes failed: %v", err)
			return false
		}

		// Verify the code can be parsed as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "crud_types.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

// TestProperty_ScopeResolutionIsConsistent verifies scope resolution is deterministic.
func TestProperty_ScopeResolutionIsConsistent(t *testing.T) {
	proptest.QuickCheck(t, "scope resolution is deterministic", func(g *proptest.Generator) bool {
		// Generate a random config
		numTables := g.IntRange(1, 10)
		tableNames := g.UniqueIdentifiers(numTables, 15)
		config := generateRandomCRUDConfig(g, tableNames)

		// Pick a random table
		tableName := proptest.Pick(g, tableNames)

		// Call GetScopeForTable multiple times - should always return same result
		first := config.GetScopeForTable(tableName)
		for i := 0; i < 5; i++ {
			result := config.GetScopeForTable(tableName)
			if result != first {
				t.Logf("Inconsistent scope for %q: got %q then %q", tableName, first, result)
				return false
			}
		}

		return true
	})
}

// TestProperty_OnlyAddTableTablesGetCRUD verifies only AddTable tables qualify for CRUD.
func TestProperty_OnlyAddTableTablesGetCRUD(t *testing.T) {
	proptest.QuickCheck(t, "only AddTable tables get CRUD", func(g *proptest.Generator) bool {
		plan := &migrate.MigrationPlan{
			Schema: migrate.Schema{
				Tables: make(map[string]ddl.Table),
			},
		}

		numTables := g.IntRange(2, 6)
		tableNames := g.UniqueIdentifiers(numTables, 15)

		// Track which tables should qualify
		qualifyingTables := make(map[string]bool)

		for _, name := range tableNames {
			// 50% chance of being an AddTable table
			isAddTable := g.Bool()

			var columns []ddl.ColumnDefinition
			if isAddTable {
				columns = []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "deleted_at", Type: ddl.DatetimeType},
					{Name: "name", Type: ddl.StringType},
				}
				qualifyingTables[name] = true
			} else {
				// AddEmptyTable - no standard columns
				columns = []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "name", Type: ddl.StringType},
				}
			}

			plan.Schema.Tables[name] = ddl.Table{
				Name:    name,
				Columns: columns,
			}
		}

		// Get CRUD tables
		crudTables := getCRUDTables(plan)

		// Verify only qualifying tables are included
		for _, table := range crudTables {
			if !qualifyingTables[table.Name] {
				t.Logf("Non-qualifying table %q was included in CRUD tables", table.Name)
				return false
			}
		}

		// Verify all qualifying tables are included
		crudTableNames := make(map[string]bool)
		for _, table := range crudTables {
			crudTableNames[table.Name] = true
		}

		for name := range qualifyingTables {
			if !crudTableNames[name] {
				t.Logf("Qualifying table %q was not included in CRUD tables", name)
				return false
			}
		}

		return true
	})
}

// TestProperty_TableOverrideTakesPrecedence verifies table-specific scope overrides global.
func TestProperty_TableOverrideTakesPrecedence(t *testing.T) {
	proptest.QuickCheck(t, "table override takes precedence", func(g *proptest.Generator) bool {
		globalScope := g.Identifier(10) + "_id"
		tableScope := g.Identifier(10) + "_id"

		config := CRUDConfig{
			GlobalScope: globalScope,
			TableScopes: map[string]string{
				"overridden": tableScope,
			},
		}

		// Table with override should get table scope
		if config.GetScopeForTable("overridden") != tableScope {
			return false
		}

		// Table without override should get global scope
		if config.GetScopeForTable("normal") != globalScope {
			return false
		}

		return true
	})
}

// TestProperty_EmptyOverrideRemovesScope verifies empty table scope overrides global.
func TestProperty_EmptyOverrideRemovesScope(t *testing.T) {
	proptest.QuickCheck(t, "empty override removes scope", func(g *proptest.Generator) bool {
		globalScope := g.Identifier(10) + "_id"

		config := CRUDConfig{
			GlobalScope: globalScope,
			TableScopes: map[string]string{
				"no_scope": "", // Explicit empty
			},
		}

		// Table with empty override should have no scope
		if config.GetScopeForTable("no_scope") != "" {
			return false
		}

		// Other tables should still have global scope
		if config.GetScopeForTable("other") != globalScope {
			return false
		}

		return true
	})
}
