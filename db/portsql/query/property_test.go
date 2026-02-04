//go:build property

package query_test

import (
	"encoding/json"
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/db/portsql/query/compile"
	"github.com/shipq/shipq/proptest"
)

// =============================================================================
// Random Generators for Query AST
// =============================================================================

// generateRandomColumn generates a random column.
func generateRandomColumn(g *proptest.Generator, tableName string) query.Column {
	colName := g.IdentifierLower(15)
	nullable := g.Bool()

	switch g.IntRange(0, 5) {
	case 0:
		if nullable {
			return query.NullInt64Column{Table: tableName, Name: colName}
		}
		return query.Int64Column{Table: tableName, Name: colName}
	case 1:
		if nullable {
			return query.NullStringColumn{Table: tableName, Name: colName}
		}
		return query.StringColumn{Table: tableName, Name: colName}
	case 2:
		if nullable {
			return query.NullBoolColumn{Table: tableName, Name: colName}
		}
		return query.BoolColumn{Table: tableName, Name: colName}
	case 3:
		if nullable {
			return query.NullFloat64Column{Table: tableName, Name: colName}
		}
		return query.Float64Column{Table: tableName, Name: colName}
	case 4:
		if nullable {
			return query.NullTimeColumn{Table: tableName, Name: colName}
		}
		return query.TimeColumn{Table: tableName, Name: colName}
	default:
		if nullable {
			return query.NullInt32Column{Table: tableName, Name: colName}
		}
		return query.Int32Column{Table: tableName, Name: colName}
	}
}

// generateRandomExpr generates a random expression.
func generateRandomExpr(g *proptest.Generator, tableName string, depth int) query.Expr {
	// Limit recursion depth
	if depth > 3 {
		return query.ColumnExpr{Column: generateRandomColumn(g, tableName)}
	}

	switch g.IntRange(0, 4) {
	case 0:
		// ColumnExpr
		return query.ColumnExpr{Column: generateRandomColumn(g, tableName)}
	case 1:
		// ParamExpr
		paramTypes := []string{"string", "int64", "bool", "float64"}
		return query.ParamExpr{
			Name:   g.IdentifierLower(10),
			GoType: proptest.Pick(g, paramTypes),
		}
	case 2:
		// LiteralExpr
		switch g.IntRange(0, 3) {
		case 0:
			return query.LiteralExpr{Value: g.IntRange(-1000, 1000)}
		case 1:
			return query.LiteralExpr{Value: g.StringAlphaNum(20)}
		case 2:
			return query.LiteralExpr{Value: g.Bool()}
		default:
			return query.LiteralExpr{Value: g.Float64Range(-1000, 1000)}
		}
	case 3:
		// BinaryExpr
		ops := []query.BinaryOp{query.OpEq, query.OpNe, query.OpLt, query.OpLe, query.OpGt, query.OpGe}
		return query.BinaryExpr{
			Left:  generateRandomExpr(g, tableName, depth+1),
			Op:    proptest.Pick(g, ops),
			Right: generateRandomExpr(g, tableName, depth+1),
		}
	default:
		// UnaryExpr
		return query.UnaryExpr{
			Op:   query.OpIsNull,
			Expr: query.ColumnExpr{Column: generateRandomColumn(g, tableName)},
		}
	}
}

// generateRandomSelectQuery generates a random SELECT query AST.
func generateRandomSelectQuery(g *proptest.Generator) *query.AST {
	tableName := g.IdentifierLower(15)

	ast := &query.AST{
		Kind: query.SelectQuery,
		FromTable: query.TableRef{
			Name: tableName,
		},
	}

	// Generate 1-5 select columns
	numCols := g.IntRange(1, 5)
	for i := 0; i < numCols; i++ {
		col := generateRandomColumn(g, tableName)
		sel := query.SelectExpr{
			Expr: query.ColumnExpr{Column: col},
		}
		// 30% chance of alias
		if g.Float64() < 0.3 {
			sel.Alias = g.IdentifierLower(10)
		}
		ast.SelectCols = append(ast.SelectCols, sel)
	}

	// 50% chance of WHERE clause
	if g.Bool() {
		ast.Where = generateRandomExpr(g, tableName, 0)
	}

	// 30% chance of ORDER BY
	if g.Float64() < 0.3 {
		col := generateRandomColumn(g, tableName)
		ast.OrderBy = append(ast.OrderBy, query.OrderByExpr{
			Expr: query.ColumnExpr{Column: col},
			Desc: g.Bool(),
		})
	}

	// 30% chance of LIMIT
	if g.Float64() < 0.3 {
		ast.Limit = query.ParamExpr{Name: "limit", GoType: "int64"}
	}

	// 20% chance of OFFSET (only if LIMIT is present)
	if ast.Limit != nil && g.Float64() < 0.2 {
		ast.Offset = query.ParamExpr{Name: "offset", GoType: "int64"}
	}

	return ast
}

// =============================================================================
// Property Tests
// =============================================================================

// TestProperty_QueryJSONRoundTrip verifies that any valid query AST can be
// serialized to JSON and deserialized back.
func TestProperty_QueryJSONRoundTrip(t *testing.T) {
	proptest.QuickCheck(t, "query JSON round-trip", func(g *proptest.Generator) bool {
		ast := generateRandomSelectQuery(g)

		// Serialize to JSON
		jsonBytes, err := json.Marshal(ast)
		if err != nil {
			t.Logf("Marshal failed: %v", err)
			return false
		}

		// Deserialize back
		var restored query.AST
		if err := json.Unmarshal(jsonBytes, &restored); err != nil {
			t.Logf("Unmarshal failed: %v", err)
			return false
		}

		// Compare key fields
		if ast.Kind != restored.Kind {
			t.Logf("Kind mismatch: %v vs %v", ast.Kind, restored.Kind)
			return false
		}

		if ast.FromTable.Name != restored.FromTable.Name {
			t.Logf("Table name mismatch: %v vs %v", ast.FromTable.Name, restored.FromTable.Name)
			return false
		}

		if len(ast.SelectCols) != len(restored.SelectCols) {
			t.Logf("SelectCols count mismatch: %d vs %d", len(ast.SelectCols), len(restored.SelectCols))
			return false
		}

		return true
	})
}

// TestProperty_QueryCompilesToValidSQL verifies that any valid query AST
// compiles to non-empty SQL for all dialects.
func TestProperty_QueryCompilesToValidSQL(t *testing.T) {
	dialects := []string{"postgres", "mysql", "sqlite"}

	proptest.QuickCheck(t, "query compiles to valid SQL", func(g *proptest.Generator) bool {
		ast := generateRandomSelectQuery(g)
		dialect := proptest.Pick(g, dialects)

		var sql string
		var err error

		switch dialect {
		case "postgres":
			sql, _, err = compile.NewCompiler(compile.Postgres).Compile(ast)
		case "mysql":
			sql, _, err = compile.NewCompiler(compile.MySQL).Compile(ast)
		case "sqlite":
			sql, _, err = compile.NewCompiler(compile.SQLite).Compile(ast)
		}

		if err != nil {
			t.Logf("Compilation failed for dialect %s: %v", dialect, err)
			return false
		}

		if sql == "" {
			t.Logf("Empty SQL for dialect %s", dialect)
			return false
		}

		// Verify basic SQL structure
		if ast.Kind == query.SelectQuery {
			if len(sql) < 6 || sql[:6] != "SELECT" {
				t.Logf("SQL doesn't start with SELECT: %s", sql)
				return false
			}
		}

		return true
	})
}

// TestProperty_QueryParamCountConsistent verifies that the number of parameters
// in compiled SQL matches across dialects (though placeholders differ).
func TestProperty_QueryParamCountConsistent(t *testing.T) {
	proptest.QuickCheck(t, "param count consistent across dialects", func(g *proptest.Generator) bool {
		ast := generateRandomSelectQuery(g)

		// Compile for all dialects
		_, pgParams, err1 := compile.NewCompiler(compile.Postgres).Compile(ast)
		_, myParams, err2 := compile.NewCompiler(compile.MySQL).Compile(ast)
		_, sqParams, err3 := compile.NewCompiler(compile.SQLite).Compile(ast)

		if err1 != nil || err2 != nil || err3 != nil {
			// Compilation errors are tested elsewhere
			return true
		}

		// Parameter counts should match
		if len(pgParams) != len(myParams) || len(pgParams) != len(sqParams) {
			t.Logf("Param count mismatch: pg=%d, mysql=%d, sqlite=%d",
				len(pgParams), len(myParams), len(sqParams))
			return false
		}

		// Parameter names should match
		for i := range pgParams {
			if pgParams[i] != myParams[i] || pgParams[i] != sqParams[i] {
				t.Logf("Param name mismatch at %d: pg=%s, mysql=%s, sqlite=%s",
					i, pgParams[i], myParams[i], sqParams[i])
				return false
			}
		}

		return true
	})
}

// TestProperty_DistinctFlagPreserved verifies that DISTINCT flag is preserved.
func TestProperty_DistinctFlagPreserved(t *testing.T) {
	proptest.QuickCheck(t, "distinct flag preserved", func(g *proptest.Generator) bool {
		ast := generateRandomSelectQuery(g)
		ast.Distinct = g.Bool()

		jsonBytes, err := json.Marshal(ast)
		if err != nil {
			return false
		}

		var restored query.AST
		if err := json.Unmarshal(jsonBytes, &restored); err != nil {
			return false
		}

		if ast.Distinct != restored.Distinct {
			t.Logf("Distinct mismatch: %v vs %v", ast.Distinct, restored.Distinct)
			return false
		}

		return true
	})
}

// TestProperty_AllDialectsProduceSameParamNames verifies that all dialects produce
// the same parameter names in the same order.
func TestProperty_AllDialectsProduceSameParamNames(t *testing.T) {
	proptest.QuickCheck(t, "all dialects same param names", func(g *proptest.Generator) bool {
		ast := generateRandomSelectQuery(g)

		_, pgParams, _ := compile.NewCompiler(compile.Postgres).Compile(ast)
		_, myParams, _ := compile.NewCompiler(compile.MySQL).Compile(ast)
		_, sqParams, _ := compile.NewCompiler(compile.SQLite).Compile(ast)

		// All should have same length
		if len(pgParams) != len(myParams) || len(pgParams) != len(sqParams) {
			return false
		}

		// All should have same names in same order
		for i := range pgParams {
			if pgParams[i] != myParams[i] || pgParams[i] != sqParams[i] {
				return false
			}
		}

		return true
	})
}
