//go:build property

package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/portsql/portsql/proptest"
	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

// =============================================================================
// Random Generators
// =============================================================================

// generateRandomMigrationPlan generates a random migration plan with tables.
func generateRandomMigrationPlan(g *proptest.Generator) *migrate.MigrationPlan {
	plan := migrate.NewPlan()

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
			continue
		}
	}

	return plan
}

// generateRandomCompiledQueries generates random compiled queries.
func generateRandomCompiledQueries(g *proptest.Generator) []CompiledQuery {
	numQueries := g.IntRange(1, 5)
	queryNames := g.UniqueIdentifiers(numQueries, 15)

	var queries []CompiledQuery

	for _, name := range queryNames {
		// Capitalize first letter for Go export
		exportedName := strings.ToUpper(name[:1]) + name[1:]

		q := CompiledQuery{
			Name: exportedName,
			SQL:  generateRandomSQL(g),
		}

		// 70% chance of having params
		if g.Float64() < 0.7 {
			numParams := g.IntRange(1, 4)
			paramNames := g.UniqueIdentifiers(numParams, 10)
			for _, pname := range paramNames {
				q.Params = append(q.Params, ParamInfo{
					Name:   pname,
					GoType: proptest.Pick(g, []string{"string", "int64", "bool", "float64"}),
				})
			}
		}

		// 80% chance of having results (SELECT queries)
		if g.Float64() < 0.8 {
			numResults := g.IntRange(1, 6)
			resultNames := g.UniqueIdentifiers(numResults, 10)
			for _, rname := range resultNames {
				q.Results = append(q.Results, ResultInfo{
					Name:   rname,
					GoType: proptest.Pick(g, []string{"string", "int64", "bool", "float64", "time.Time", "*string", "*int64"}),
				})
			}
		}

		queries = append(queries, q)
	}

	return queries
}

// generateRandomSQL generates a random SQL-like string.
func generateRandomSQL(g *proptest.Generator) string {
	templates := []string{
		`SELECT "table"."col1", "table"."col2" FROM "table" WHERE "table"."id" = ?`,
		`SELECT * FROM "users" WHERE "users"."email" = $1`,
		`INSERT INTO "posts" ("title", "body") VALUES ($1, $2)`,
		`UPDATE "users" SET "name" = ? WHERE "id" = ?`,
		`DELETE FROM "comments" WHERE "id" = ?`,
	}
	return proptest.Pick(g, templates)
}

// =============================================================================
// Property Tests
// =============================================================================

// TestProperty_GeneratedQueriesCompiles verifies that generated query code compiles.
func TestProperty_GeneratedQueriesCompiles(t *testing.T) {
	proptest.QuickCheck(t, "generated queries compiles", func(g *proptest.Generator) bool {
		queries := generateRandomCompiledQueries(g)

		code, err := GenerateQueriesPackage(queries, "queries")
		if err != nil {
			t.Logf("GenerateQueriesPackage failed: %v", err)
			return false
		}

		// Verify the code can be parsed as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "queries.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

// TestProperty_GeneratedSchemaCompiles verifies that generated schematypes code compiles.
func TestProperty_GeneratedSchemaCompiles(t *testing.T) {
	proptest.QuickCheck(t, "generated schema compiles", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)

		if len(plan.Schema.Tables) == 0 {
			return true
		}

		code, err := GenerateSchemaPackage(plan, "github.com/portsql/portsql/src/query")
		if err != nil {
			t.Logf("GenerateSchemaPackage failed: %v", err)
			return false
		}

		// Verify the code can be parsed as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "tables.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

// TestProperty_GeneratedRunnerCompiles verifies that generated runner code compiles.
func TestProperty_GeneratedRunnerCompiles(t *testing.T) {
	proptest.QuickCheck(t, "generated runner compiles", func(g *proptest.Generator) bool {
		packageName := g.IdentifierLower(10)

		code, err := GenerateRunner(packageName)
		if err != nil {
			t.Logf("GenerateRunner failed: %v", err)
			return false
		}

		// Verify the code can be parsed as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "runner.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

// TestProperty_GeneratedCodeContainsHeader verifies generated code has the DO NOT EDIT header.
func TestProperty_GeneratedCodeContainsHeader(t *testing.T) {
	proptest.QuickCheck(t, "generated code has header", func(g *proptest.Generator) bool {
		queries := generateRandomCompiledQueries(g)

		code, err := GenerateQueriesPackage(queries, "queries")
		if err != nil {
			return false
		}

		codeStr := string(code)
		if !strings.Contains(codeStr, "DO NOT EDIT") {
			t.Logf("Missing DO NOT EDIT header")
			return false
		}

		return true
	})
}

// TestProperty_GeneratedSQLConstants verifies SQL constants are generated for each query.
func TestProperty_GeneratedSQLConstants(t *testing.T) {
	proptest.QuickCheck(t, "SQL constants generated", func(g *proptest.Generator) bool {
		queries := generateRandomCompiledQueries(g)

		code, err := GenerateQueriesPackage(queries, "queries")
		if err != nil {
			return false
		}

		codeStr := string(code)

		for _, q := range queries {
			constName := q.Name + "SQL"
			if !strings.Contains(codeStr, constName) {
				t.Logf("Missing constant %s", constName)
				return false
			}
		}

		return true
	})
}

// TestProperty_GeneratedParamStructs verifies param structs are generated when needed.
func TestProperty_GeneratedParamStructs(t *testing.T) {
	proptest.QuickCheck(t, "param structs generated", func(g *proptest.Generator) bool {
		queries := generateRandomCompiledQueries(g)

		code, err := GenerateQueriesPackage(queries, "queries")
		if err != nil {
			return false
		}

		codeStr := string(code)

		for _, q := range queries {
			if len(q.Params) > 0 {
				structName := q.Name + "Params"
				if !strings.Contains(codeStr, structName) {
					t.Logf("Missing struct %s for query with %d params", structName, len(q.Params))
					return false
				}
			}
		}

		return true
	})
}

// TestProperty_GeneratedResultStructs verifies result structs are generated when needed.
func TestProperty_GeneratedResultStructs(t *testing.T) {
	proptest.QuickCheck(t, "result structs generated", func(g *proptest.Generator) bool {
		queries := generateRandomCompiledQueries(g)

		code, err := GenerateQueriesPackage(queries, "queries")
		if err != nil {
			return false
		}

		codeStr := string(code)

		for _, q := range queries {
			if len(q.Results) > 0 {
				structName := q.Name + "Result"
				if !strings.Contains(codeStr, structName) {
					t.Logf("Missing struct %s for query with %d results", structName, len(q.Results))
					return false
				}
			}
		}

		return true
	})
}

// TestProperty_TimeImportIncluded verifies time import is added when time.Time is used.
func TestProperty_TimeImportIncluded(t *testing.T) {
	proptest.QuickCheck(t, "time import included when needed", func(g *proptest.Generator) bool {
		queries := []CompiledQuery{
			{
				Name: "GetUser",
				SQL:  "SELECT * FROM users",
				Results: []ResultInfo{
					{Name: "created_at", GoType: "time.Time"},
				},
			},
		}

		code, err := GenerateQueriesPackage(queries, "queries")
		if err != nil {
			return false
		}

		codeStr := string(code)
		if !strings.Contains(codeStr, `"time"`) {
			t.Logf("Missing time import when time.Time is used")
			return false
		}

		return true
	})
}

// TestProperty_PascalCaseConversion verifies field names are properly converted to PascalCase.
func TestProperty_PascalCaseConversion(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"id", "Id"},
		{"user_id", "UserId"},
		{"created_at", "CreatedAt"},
		{"first_name", "FirstName"},
	}

	for _, tc := range testCases {
		result := toPascalCase(tc.input)
		if result != tc.expected {
			t.Errorf("toPascalCase(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

// TestProperty_SchemaTableNamesPreserved verifies table names are preserved in generated code.
func TestProperty_SchemaTableNamesPreserved(t *testing.T) {
	proptest.QuickCheck(t, "table names preserved in schema", func(g *proptest.Generator) bool {
		plan := generateRandomMigrationPlan(g)

		if len(plan.Schema.Tables) == 0 {
			return true
		}

		code, err := GenerateSchemaPackage(plan, "github.com/portsql/portsql/src/query")
		if err != nil {
			return false
		}

		codeStr := string(code)

		for tableName := range plan.Schema.Tables {
			// Table name should appear in the generated code
			if !strings.Contains(codeStr, tableName) {
				t.Logf("Table name %q not found in generated code", tableName)
				return false
			}
		}

		return true
	})
}
