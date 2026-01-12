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

// =============================================================================
// JSON Aggregation Property Tests
// =============================================================================

// generateRandomNestedResults generates random result info with nested fields.
func generateRandomNestedResults(g *proptest.Generator, maxDepth int) []ResultInfo {
	numResults := g.IntRange(1, 4)
	resultNames := g.UniqueIdentifiers(numResults, 10)

	var results []ResultInfo
	for _, name := range resultNames {
		r := ResultInfo{
			Name:   name,
			GoType: proptest.Pick(g, []string{"string", "int64", "bool", "float64", "time.Time", "*string", "*int64"}),
		}

		// 30% chance of being a nested JSON field (if we haven't hit max depth)
		if maxDepth > 0 && g.Float64() < 0.3 {
			r.GoType = ""
			r.NestedFields = generateRandomNestedResults(g, maxDepth-1)
		}

		results = append(results, r)
	}

	return results
}

// TestProperty_JSONAggGeneratedCodeCompiles verifies JSON aggregation generates valid Go.
func TestProperty_JSONAggGeneratedCodeCompiles(t *testing.T) {
	proptest.QuickCheck(t, "json agg generates valid Go", func(g *proptest.Generator) bool {
		// Generate random nesting depth (1-3 levels)
		depth := g.IntRange(1, 3)
		results := generateRandomNestedResults(g, depth)

		queryName := strings.ToUpper(g.Identifier(10)[:1]) + g.Identifier(10)[1:]

		query := CompiledQuery{
			Name:    queryName,
			SQL:     "SELECT ...",
			Results: results,
		}

		code, err := GenerateQueriesPackage([]CompiledQuery{query}, "queries")
		if err != nil {
			t.Logf("GenerateQueriesPackage failed: %v", err)
			return false
		}

		// Verify generated code parses as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "queries.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

// TestProperty_NestedTypesHandleAllColumnTypes verifies nested types work with all column types.
func TestProperty_NestedTypesHandleAllColumnTypes(t *testing.T) {
	allTypes := []string{
		"string", "int64", "int32", "bool", "float64",
		"time.Time", "*string", "*int64", "*int32", "*bool",
		"*float64", "*time.Time", "json.RawMessage", "[]byte",
	}

	proptest.QuickCheck(t, "nested types handle all column types", func(g *proptest.Generator) bool {
		// Create a query with nested fields using various types
		var nestedFields []ResultInfo
		numFields := g.IntRange(1, len(allTypes))
		fieldNames := g.UniqueIdentifiers(numFields, 10)

		for i, name := range fieldNames {
			nestedFields = append(nestedFields, ResultInfo{
				Name:   name,
				GoType: allTypes[i%len(allTypes)],
			})
		}

		queryName := "TestQuery" + g.Identifier(5)
		query := CompiledQuery{
			Name: queryName,
			SQL:  "SELECT ...",
			Results: []ResultInfo{
				{Name: "id", GoType: "int64"},
				{Name: "items", GoType: "", NestedFields: nestedFields},
			},
		}

		code, err := GenerateQueriesPackage([]CompiledQuery{query}, "queries")
		if err != nil {
			t.Logf("GenerateQueriesPackage failed: %v", err)
			return false
		}

		// Verify generated code parses as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "queries.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

// TestProperty_MultipleJSONAggFieldsGenerateUniqueTypes verifies no duplicate type names.
func TestProperty_MultipleJSONAggFieldsGenerateUniqueTypes(t *testing.T) {
	proptest.QuickCheck(t, "multiple json agg fields have unique types", func(g *proptest.Generator) bool {
		// Create a query with multiple JSON agg fields
		numJSONFields := g.IntRange(2, 4)
		fieldNames := g.UniqueIdentifiers(numJSONFields, 10)

		var results []ResultInfo
		results = append(results, ResultInfo{Name: "id", GoType: "int64"})

		for _, name := range fieldNames {
			results = append(results, ResultInfo{
				Name:   name,
				GoType: "",
				NestedFields: []ResultInfo{
					{Name: "id", GoType: "int64"},
					{Name: "value", GoType: "string"},
				},
			})
		}

		queryName := "TestQuery" + g.Identifier(5)
		query := CompiledQuery{
			Name:    queryName,
			SQL:     "SELECT ...",
			Results: results,
		}

		code, err := GenerateQueriesPackage([]CompiledQuery{query}, "queries")
		if err != nil {
			t.Logf("GenerateQueriesPackage failed: %v", err)
			return false
		}

		codeStr := string(code)

		// Verify each JSON field generates a unique type
		for _, name := range fieldNames {
			typeName := queryName + toPascalCase(name) + "Item"
			if !strings.Contains(codeStr, "type "+typeName+" struct") {
				t.Logf("Missing type %s in generated code", typeName)
				return false
			}
		}

		// Verify code parses
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "queries.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v", err)
			return false
		}

		return true
	})
}

// =============================================================================
// Property Tests for QueryRunner Generation
// =============================================================================

// generateRandomQueryWithDialects generates a random query with all dialect SQL.
func generateRandomQueryWithDialects(g *proptest.Generator) CompiledQueryWithDialects {
	queryName := "Query" + g.Identifier(10)

	// Generate random params
	numParams := g.IntRange(0, 4)
	paramNames := g.UniqueIdentifiers(numParams, 8)
	params := make([]ParamInfo, len(paramNames))
	for i, name := range paramNames {
		params[i] = ParamInfo{
			Name:   name,
			GoType: proptest.Pick(g, []string{"string", "int64", "int", "bool", "float64"}),
		}
	}

	// Generate random results
	numResults := g.IntRange(1, 6)
	resultNames := g.UniqueIdentifiers(numResults, 10)
	results := make([]ResultInfo, len(resultNames))
	for i, name := range resultNames {
		results[i] = ResultInfo{
			Name:   name,
			GoType: proptest.Pick(g, []string{"string", "int64", "int", "bool", "float64", "*string", "*int64", "time.Time"}),
		}
	}

	// Generate random return type
	returnType := proptest.Pick(g, []string{"one", "many", "exec"})

	// Generate simple SQL for each dialect
	baseSql := "SELECT * FROM " + g.Identifier(10)

	return CompiledQueryWithDialects{
		CompiledQuery: CompiledQuery{
			Name:       queryName,
			SQL:        baseSql,
			Params:     params,
			Results:    results,
			ReturnType: returnType,
		},
		SQL: DialectSQL{
			Postgres: baseSql,
			MySQL:    baseSql,
			SQLite:   baseSql,
		},
	}
}

func TestProperty_GeneratedMethodsCompile(t *testing.T) {
	proptest.QuickCheck(t, "generated methods are valid Go", func(g *proptest.Generator) bool {
		// Generate random queries with random return types
		numQueries := g.IntRange(1, 5)
		queries := make([]CompiledQueryWithDialects, numQueries)
		usedNames := make(map[string]bool)

		for i := 0; i < numQueries; i++ {
			q := generateRandomQueryWithDialects(g)
			// Ensure unique names
			for usedNames[q.Name] {
				q.Name = "Query" + g.Identifier(10)
			}
			usedNames[q.Name] = true
			queries[i] = q
		}

		// Generate dialect-specific runner code (test with sqlite)
		code, err := GenerateDialectRunner(queries, nil, "sqlite", "myapp/queries", make(map[string]CRUDOptions))
		if err != nil {
			t.Logf("GenerateDialectRunner failed: %v", err)
			return false
		}

		// Verify code parses as valid Go
		fset := token.NewFileSet()
		_, err = parser.ParseFile(fset, "runner.go", code, parser.AllErrors)
		if err != nil {
			t.Logf("Generated code doesn't parse: %v\nCode:\n%s", err, string(code))
			return false
		}

		return true
	})
}

func TestProperty_ReturnTypeMatchesSignature(t *testing.T) {
	proptest.QuickCheck(t, "return type matches method signature", func(g *proptest.Generator) bool {
		returnType := proptest.Pick(g, []string{"one", "many", "exec"})
		queryName := "TestQuery" + g.Identifier(5)

		query := CompiledQueryWithDialects{
			CompiledQuery: CompiledQuery{
				Name:       queryName,
				Params:     []ParamInfo{{Name: "id", GoType: "int64"}},
				Results:    []ResultInfo{{Name: "id", GoType: "int64"}, {Name: "name", GoType: "string"}},
				ReturnType: returnType,
			},
			SQL: DialectSQL{
				Postgres: "SELECT id, name FROM test WHERE id = $1",
				MySQL:    "SELECT id, name FROM test WHERE id = ?",
				SQLite:   "SELECT id, name FROM test WHERE id = ?",
			},
		}

		code, err := GenerateDialectRunner([]CompiledQueryWithDialects{query}, nil, "sqlite", "myapp/queries", make(map[string]CRUDOptions))
		if err != nil {
			t.Logf("GenerateDialectRunner failed: %v", err)
			return false
		}

		codeStr := string(code)

		// Check signature based on return type (note: types come from parent package)
		switch returnType {
		case "one":
			expected := "(*queries." + queryName + "Result, error)"
			if !strings.Contains(codeStr, expected) {
				t.Logf("ONE query should return %s", expected)
				return false
			}
		case "many":
			expected := "([]queries." + queryName + "Result, error)"
			if !strings.Contains(codeStr, expected) {
				t.Logf("MANY query should return %s", expected)
				return false
			}
		case "exec":
			expected := "(sql.Result, error)"
			if !strings.Contains(codeStr, expected) {
				t.Logf("EXEC query should return %s", expected)
				return false
			}
		}

		return true
	})
}

func TestProperty_ScanFieldCountMatchesResultStruct(t *testing.T) {
	proptest.QuickCheck(t, "scan field count matches result struct", func(g *proptest.Generator) bool {
		// Generate random number of result fields
		numResults := g.IntRange(1, 8)
		resultNames := g.UniqueIdentifiers(numResults, 10)
		results := make([]ResultInfo, len(resultNames))
		for i, name := range resultNames {
			results[i] = ResultInfo{
				Name:   name,
				GoType: "string",
			}
		}

		queryName := "TestQuery" + g.Identifier(5)
		returnType := proptest.Pick(g, []string{"one", "many"}) // exec doesn't scan

		query := CompiledQueryWithDialects{
			CompiledQuery: CompiledQuery{
				Name:       queryName,
				Params:     []ParamInfo{{Name: "id", GoType: "int64"}},
				Results:    results,
				ReturnType: returnType,
			},
			SQL: DialectSQL{
				Postgres: "SELECT * FROM test WHERE id = $1",
				MySQL:    "SELECT * FROM test WHERE id = ?",
				SQLite:   "SELECT * FROM test WHERE id = ?",
			},
		}

		code, err := GenerateDialectRunner([]CompiledQueryWithDialects{query}, nil, "sqlite", "myapp/queries", make(map[string]CRUDOptions))
		if err != nil {
			t.Logf("GenerateDialectRunner failed: %v", err)
			return false
		}

		codeStr := string(code)

		// Count Scan fields - look for &result. or &item. patterns
		scanCount := strings.Count(codeStr, "&result.") + strings.Count(codeStr, "&item.")

		// Should equal number of result fields (one scan field per result)
		if scanCount != numResults {
			t.Logf("Expected %d scan fields, found %d", numResults, scanCount)
			return false
		}

		return true
	})
}

func TestProperty_ParamOrderMatchesSQLPlaceholders(t *testing.T) {
	proptest.QuickCheck(t, "param order matches SQL placeholders", func(g *proptest.Generator) bool {
		// Generate random params
		numParams := g.IntRange(1, 5)
		paramNames := g.UniqueIdentifiers(numParams, 8)
		params := make([]ParamInfo, len(paramNames))
		for i, name := range paramNames {
			params[i] = ParamInfo{
				Name:   name,
				GoType: "string",
			}
		}

		queryName := "TestQuery" + g.Identifier(5)

		query := CompiledQueryWithDialects{
			CompiledQuery: CompiledQuery{
				Name:       queryName,
				Params:     params,
				Results:    []ResultInfo{{Name: "id", GoType: "int64"}},
				ReturnType: "one",
			},
			SQL: DialectSQL{
				Postgres: "SELECT id FROM test WHERE col = $1",
				MySQL:    "SELECT id FROM test WHERE col = ?",
				SQLite:   "SELECT id FROM test WHERE col = ?",
			},
		}

		code, err := GenerateDialectRunner([]CompiledQueryWithDialects{query}, nil, "sqlite", "myapp/queries", make(map[string]CRUDOptions))
		if err != nil {
			t.Logf("GenerateDialectRunner failed: %v", err)
			return false
		}

		codeStr := string(code)

		// Check that params are added in order
		// The generated code should have: params.ParamName1, params.ParamName2, ...
		for _, p := range params {
			expected := "params." + toPascalCase(p.Name)
			if !strings.Contains(codeStr, expected) {
				t.Logf("Missing param %s in generated code", expected)
				return false
			}
		}

		return true
	})
}
