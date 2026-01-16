//go:build property

package migrate

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/proptest"

	_ "modernc.org/sqlite"
)

// =============================================================================
// Random Generators for Migration Plans
// =============================================================================

// generateRandomColumn generates a random column definition.
func generateRandomColumn(g *proptest.Generator) ddl.ColumnDefinition {
	colTypes := []string{
		ddl.BigintType,
		ddl.IntegerType,
		ddl.StringType,
		ddl.TextType,
		ddl.BooleanType,
		ddl.DatetimeType,
		ddl.FloatType,
	}

	col := ddl.ColumnDefinition{
		Name:     g.IdentifierLower(15),
		Type:     proptest.Pick(g, colTypes),
		Nullable: g.Bool(),
	}

	// Add length for string type
	if col.Type == ddl.StringType {
		length := g.IntRange(1, 255)
		col.Length = &length
	}

	// Add precision/scale for decimal (but we're not using decimal in this test)

	return col
}

// generateRandomTable generates a random table definition.
func generateRandomTable(g *proptest.Generator) ddl.Table {
	numColumns := g.IntRange(1, 10)

	// Generate unique column names
	columnNames := g.UniqueIdentifiers(numColumns, 15)
	columns := make([]ddl.ColumnDefinition, len(columnNames))

	for i, name := range columnNames {
		columns[i] = generateRandomColumn(g)
		columns[i].Name = name
	}

	// Make the first column a primary key
	if len(columns) > 0 {
		columns[0].PrimaryKey = true
		columns[0].Nullable = false
		columns[0].Type = ddl.BigintType
	}

	return ddl.Table{
		Name:    g.IdentifierLower(20),
		Columns: columns,
		Indexes: []ddl.IndexDefinition{},
	}
}

// generateRandomMigrationPlan generates a random migration plan with tables.
func generateRandomMigrationPlan(g *proptest.Generator) *MigrationPlan {
	plan := NewPlan()

	numTables := g.IntRange(1, 5)
	tableNames := g.UniqueIdentifiers(numTables, 15)

	for _, tableName := range tableNames {
		numColumns := g.IntRange(1, 8)
		columnNames := g.UniqueIdentifiers(numColumns, 12)

		_, err := plan.AddEmptyTable(tableName, func(tb *ddl.TableBuilder) error {
			// Always add an ID column first
			tb.Bigint("id").PrimaryKey()

			// Add random columns
			for _, colName := range columnNames {
				if colName == "id" {
					continue
				}

				switch g.IntRange(0, 5) {
				case 0:
					if g.Bool() {
						tb.Bigint(colName).Nullable()
					} else {
						tb.Bigint(colName)
					}
				case 1:
					if g.Bool() {
						tb.String(colName).Nullable()
					} else {
						tb.String(colName)
					}
				case 2:
					if g.Bool() {
						tb.Bool(colName).Nullable()
					} else {
						tb.Bool(colName)
					}
				case 3:
					if g.Bool() {
						tb.Datetime(colName).Nullable()
					} else {
						tb.Datetime(colName)
					}
				case 4:
					if g.Bool() {
						tb.Float(colName).Nullable()
					} else {
						tb.Float(colName)
					}
				case 5:
					if g.Bool() {
						tb.Text(colName).Nullable()
					} else {
						tb.Text(colName)
					}
				}
			}
			return nil
		})
		if err != nil {
			// Skip this table on error (e.g., duplicate name)
			continue
		}
	}

	return plan
}

// =============================================================================
// Property Tests
// =============================================================================

// TestProperty_SchemaJSONRoundTrip verifies that any valid MigrationPlan can be
// serialized to JSON and deserialized back to an equivalent plan.
func TestProperty_SchemaJSONRoundTrip(t *testing.T) {
	proptest.QuickCheck(t, "schema JSON round-trip", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)

		// Serialize to JSON
		jsonBytes, err := plan.ToJSON()
		if err != nil {
			t.Logf("ToJSON failed: %v", err)
			return false
		}

		// Deserialize back
		restored, err := PlanFromJSON(jsonBytes)
		if err != nil {
			t.Logf("PlanFromJSON failed: %v", err)
			return false
		}

		// Compare
		if !reflect.DeepEqual(plan.Schema, restored.Schema) {
			t.Logf("Schema mismatch:\nOriginal: %+v\nRestored: %+v", plan.Schema, restored.Schema)
			return false
		}

		if len(plan.Migrations) != len(restored.Migrations) {
			t.Logf("Migration count mismatch: %d vs %d", len(plan.Migrations), len(restored.Migrations))
			return false
		}

		return true
	})
}

// TestProperty_MigrationIdempotent verifies that running migrations twice produces
// the same database state.
func TestProperty_MigrationIdempotent(t *testing.T) {
	proptest.QuickCheck(t, "migration idempotent", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)

		// Skip if no tables were generated
		if len(plan.Schema.Tables) == 0 {
			return true
		}

		// Create in-memory SQLite database
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Logf("Failed to open database: %v", err)
			return false
		}
		defer db.Close()

		ctx := context.Background()

		// Run migrations first time
		if err := Run(ctx, db, plan, "sqlite"); err != nil {
			t.Logf("First migration run failed: %v", err)
			return false
		}

		// Capture table list after first run
		tables1, err := GetAllTables(ctx, db, "sqlite")
		if err != nil {
			t.Logf("Failed to get tables after first run: %v", err)
			return false
		}

		// Run migrations second time
		if err := Run(ctx, db, plan, "sqlite"); err != nil {
			t.Logf("Second migration run failed: %v", err)
			return false
		}

		// Capture table list after second run
		tables2, err := GetAllTables(ctx, db, "sqlite")
		if err != nil {
			t.Logf("Failed to get tables after second run: %v", err)
			return false
		}

		// Compare table lists
		if len(tables1) != len(tables2) {
			t.Logf("Table count changed: %d -> %d", len(tables1), len(tables2))
			return false
		}

		return true
	})
}

// TestProperty_MigrationSQLNotEmpty verifies that generated SQL is non-empty
// for any valid migration plan.
func TestProperty_MigrationSQLNotEmpty(t *testing.T) {
	dialects := []string{"postgres", "mysql", "sqlite"}

	proptest.QuickCheck(t, "migration SQL not empty", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)
		dialect := proptest.Pick(g, dialects)

		// Check that each migration has SQL for the dialect
		for _, migration := range plan.Migrations {
			var sql string
			switch dialect {
			case "postgres":
				sql = migration.Instructions.Postgres
			case "mysql":
				sql = migration.Instructions.MySQL
			case "sqlite":
				sql = migration.Instructions.Sqlite
			}

			if sql == "" {
				t.Logf("Empty SQL for migration %q in dialect %s", migration.Name, dialect)
				return false
			}
		}

		return true
	})
}

// TestProperty_MigrationTableNamesPreserved verifies that table names in the
// schema match the tables created by migrations.
func TestProperty_MigrationTableNamesPreserved(t *testing.T) {
	proptest.QuickCheck(t, "table names preserved", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)

		// Skip if no tables
		if len(plan.Schema.Tables) == 0 {
			return true
		}

		// Create database and run migrations
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			return false
		}
		defer db.Close()

		ctx := context.Background()
		if err := Run(ctx, db, plan, "sqlite"); err != nil {
			t.Logf("Migration failed: %v", err)
			return false
		}

		// Get actual tables
		tables, err := GetAllTables(ctx, db, "sqlite")
		if err != nil {
			return false
		}

		// Build set of table names (excluding tracking table)
		actualTables := make(map[string]bool)
		for _, t := range tables {
			if t != "_portsql_migrations" {
				actualTables[t] = true
			}
		}

		// Verify schema tables exist
		for tableName := range plan.Schema.Tables {
			if !actualTables[tableName] {
				t.Logf("Expected table %q not found in database", tableName)
				return false
			}
		}

		return true
	})
}

// TestProperty_MigrationColumnCountPreserved verifies that the number of columns
// in each table matches the schema.
func TestProperty_MigrationColumnCountPreserved(t *testing.T) {
	proptest.QuickCheck(t, "column count preserved", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)

		if len(plan.Schema.Tables) == 0 {
			return true
		}

		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			return false
		}
		defer db.Close()

		ctx := context.Background()
		if err := Run(ctx, db, plan, "sqlite"); err != nil {
			return false
		}

		// Verify column counts match
		for tableName, table := range plan.Schema.Tables {
			// Query column count from SQLite
			var count int
			query := `SELECT COUNT(*) FROM pragma_table_info(?)`
			if err := db.QueryRowContext(ctx, query, tableName).Scan(&count); err != nil {
				t.Logf("Failed to query columns for %s: %v", tableName, err)
				return false
			}

			if count != len(table.Columns) {
				t.Logf("Column count mismatch for %s: expected %d, got %d",
					tableName, len(table.Columns), count)
				return false
			}
		}

		return true
	})
}
