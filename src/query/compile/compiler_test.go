package compile

import (
	"strings"
	"testing"

	"github.com/portsql/portsql/src/query"
)

// TestAllDialects runs a suite of tests against all dialects to ensure
// the unified compiler produces correct output for each.
func TestAllDialects(t *testing.T) {
	dialects := []struct {
		name    string
		dialect Dialect
	}{
		{"Postgres", Postgres},
		{"MySQL", MySQL},
		{"SQLite", SQLite},
	}

	for _, d := range dialects {
		t.Run(d.name, func(t *testing.T) {
			runDialectTests(t, d.dialect)
		})
	}
}

func runDialectTests(t *testing.T, dialect Dialect) {
	t.Run("SimpleSelect", func(t *testing.T) {
		testSimpleSelect(t, dialect)
	})
	t.Run("SelectWithParams", func(t *testing.T) {
		testSelectWithParams(t, dialect)
	})
	t.Run("Insert", func(t *testing.T) {
		testInsert(t, dialect)
	})
	t.Run("Update", func(t *testing.T) {
		testUpdate(t, dialect)
	})
	t.Run("Delete", func(t *testing.T) {
		testDelete(t, dialect)
	})
	t.Run("Aggregates", func(t *testing.T) {
		testAggregates(t, dialect)
	})
	t.Run("Subquery", func(t *testing.T) {
		testSubquery(t, dialect)
	})
}

// =============================================================================
// Shared Test Cases
// =============================================================================

func testSimpleSelect(t *testing.T, dialect Dialect) {
	col := query.StringColumn{Table: "users", Name: "name"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
	}

	compiler := NewCompiler(dialect)
	sql, params, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(params) != 0 {
		t.Errorf("Expected 0 params, got %d", len(params))
	}

	// Check that the SQL contains the table and column
	if !strings.Contains(sql, "users") {
		t.Errorf("SQL should contain 'users': %s", sql)
	}
	if !strings.Contains(sql, "name") {
		t.Errorf("SQL should contain 'name': %s", sql)
	}
	if !strings.Contains(sql, "SELECT") {
		t.Errorf("SQL should contain 'SELECT': %s", sql)
	}
	if !strings.Contains(sql, "FROM") {
		t.Errorf("SQL should contain 'FROM': %s", sql)
	}
}

func testSelectWithParams(t *testing.T, dialect Dialect) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: col},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "user_id", GoType: "int64"},
		},
	}

	compiler := NewCompiler(dialect)
	sql, params, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(params) != 1 {
		t.Errorf("Expected 1 param, got %d", len(params))
	}
	if params[0] != "user_id" {
		t.Errorf("Expected param 'user_id', got %q", params[0])
	}

	// Check placeholder format
	if dialect.Name() == "postgres" {
		if !strings.Contains(sql, "$1") {
			t.Errorf("Postgres SQL should contain '$1': %s", sql)
		}
	} else {
		if !strings.Contains(sql, "?") {
			t.Errorf("%s SQL should contain '?': %s", dialect.Name(), sql)
		}
	}
}

func testInsert(t *testing.T, dialect Dialect) {
	col := query.StringColumn{Table: "users", Name: "email"}

	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{col},
		InsertVals: []query.Expr{query.ParamExpr{Name: "email", GoType: "string"}},
	}

	compiler := NewCompiler(dialect)
	sql, params, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(params) != 1 {
		t.Errorf("Expected 1 param, got %d", len(params))
	}

	if !strings.Contains(sql, "INSERT INTO") {
		t.Errorf("SQL should contain 'INSERT INTO': %s", sql)
	}
	if !strings.Contains(sql, "VALUES") {
		t.Errorf("SQL should contain 'VALUES': %s", sql)
	}
}

func testUpdate(t *testing.T, dialect Dialect) {
	nameCol := query.StringColumn{Table: "users", Name: "name"}
	idCol := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "users"},
		SetClauses: []query.SetClause{
			{Column: nameCol, Value: query.ParamExpr{Name: "new_name", GoType: "string"}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: idCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "user_id", GoType: "int64"},
		},
	}

	compiler := NewCompiler(dialect)
	sql, params, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("Expected 2 params, got %d", len(params))
	}

	if !strings.Contains(sql, "UPDATE") {
		t.Errorf("SQL should contain 'UPDATE': %s", sql)
	}
	if !strings.Contains(sql, "SET") {
		t.Errorf("SQL should contain 'SET': %s", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain 'WHERE': %s", sql)
	}
}

func testDelete(t *testing.T, dialect Dialect) {
	idCol := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:      query.DeleteQuery,
		FromTable: query.TableRef{Name: "users"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: idCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "user_id", GoType: "int64"},
		},
	}

	compiler := NewCompiler(dialect)
	sql, params, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(params) != 1 {
		t.Errorf("Expected 1 param, got %d", len(params))
	}

	if !strings.Contains(sql, "DELETE FROM") {
		t.Errorf("SQL should contain 'DELETE FROM': %s", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain 'WHERE': %s", sql)
	}
}

func testAggregates(t *testing.T, dialect Dialect) {
	col := query.Float64Column{Table: "orders", Name: "amount"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{
				Expr:  query.AggregateExpr{Func: query.AggCount, Arg: nil},
				Alias: "total",
			},
			{
				Expr:  query.AggregateExpr{Func: query.AggSum, Arg: query.ColumnExpr{Column: col}},
				Alias: "sum_amount",
			},
		},
	}

	compiler := NewCompiler(dialect)
	sql, _, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(sql, "COUNT(*)") {
		t.Errorf("SQL should contain 'COUNT(*)': %s", sql)
	}
	if !strings.Contains(sql, "SUM(") {
		t.Errorf("SQL should contain 'SUM(': %s", sql)
	}
}

func testSubquery(t *testing.T, dialect Dialect) {
	outerCol := query.Int64Column{Table: "users", Name: "id"}
	innerCol := query.Int64Column{Table: "orders", Name: "user_id"}

	innerAST := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: innerCol}}},
	}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: outerCol}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: outerCol},
			Op:    query.OpIn,
			Right: query.SubqueryExpr{Query: innerAST},
		},
	}

	compiler := NewCompiler(dialect)
	sql, _, err := compiler.Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(sql, "IN") {
		t.Errorf("SQL should contain 'IN': %s", sql)
	}
	if !strings.Contains(sql, "orders") {
		t.Errorf("SQL should contain 'orders' from subquery: %s", sql)
	}
}

// =============================================================================
// Dialect-Specific Tests
// =============================================================================

func TestDialect_QuoteIdentifier(t *testing.T) {
	tests := []struct {
		dialect  Dialect
		input    string
		expected string
	}{
		{Postgres, "users", `"users"`},
		{MySQL, "users", "`users`"},
		{SQLite, "users", `"users"`},
	}

	for _, tt := range tests {
		t.Run(tt.dialect.Name(), func(t *testing.T) {
			result := tt.dialect.QuoteIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDialect_QuoteIdentifier_EscapesEmbeddedQuotes(t *testing.T) {
	// Identifiers containing quote characters must be escaped by doubling the quote.
	// This is critical for security and correctness.
	tests := []struct {
		dialect  Dialect
		input    string
		expected string
	}{
		// Postgres uses double quotes, so embedded " must become ""
		{Postgres, `table"name`, `"table""name"`},
		{Postgres, `a"b"c`, `"a""b""c"`},
		{Postgres, `"already"quoted"`, `"""already""quoted"""`},

		// MySQL uses backticks, so embedded ` must become ``
		{MySQL, "table`name", "`table``name`"},
		{MySQL, "a`b`c", "`a``b``c`"},
		{MySQL, "`already`quoted`", "```already``quoted```"},

		// SQLite uses double quotes like Postgres
		{SQLite, `table"name`, `"table""name"`},
		{SQLite, `a"b"c`, `"a""b""c"`},
	}

	for _, tt := range tests {
		t.Run(tt.dialect.Name()+"_"+tt.input, func(t *testing.T) {
			result := tt.dialect.QuoteIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("QuoteIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDialect_Placeholder(t *testing.T) {
	tests := []struct {
		dialect  Dialect
		index    int
		expected string
	}{
		{Postgres, 1, "$1"},
		{Postgres, 5, "$5"},
		{MySQL, 1, "?"},
		{MySQL, 5, "?"},
		{SQLite, 1, "?"},
		{SQLite, 5, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect.Name(), func(t *testing.T) {
			result := tt.dialect.Placeholder(tt.index)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDialect_BoolLiteral(t *testing.T) {
	tests := []struct {
		dialect       Dialect
		expectedTrue  string
		expectedFalse string
	}{
		{Postgres, "TRUE", "FALSE"},
		{MySQL, "1", "0"},
		{SQLite, "1", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect.Name(), func(t *testing.T) {
			trueResult := tt.dialect.BoolLiteral(true)
			if trueResult != tt.expectedTrue {
				t.Errorf("BoolLiteral(true): expected %q, got %q", tt.expectedTrue, trueResult)
			}
			falseResult := tt.dialect.BoolLiteral(false)
			if falseResult != tt.expectedFalse {
				t.Errorf("BoolLiteral(false): expected %q, got %q", tt.expectedFalse, falseResult)
			}
		})
	}
}

func TestDialect_NowFunc(t *testing.T) {
	tests := []struct {
		dialect  Dialect
		expected string
	}{
		{Postgres, "NOW()"},
		{MySQL, "NOW()"},
		{SQLite, "datetime('now')"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect.Name(), func(t *testing.T) {
			result := tt.dialect.NowFunc()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDialect_WrapSetOpQueries(t *testing.T) {
	tests := []struct {
		dialect  Dialect
		expected bool
	}{
		{Postgres, true},
		{MySQL, true},
		{SQLite, false},
	}

	for _, tt := range tests {
		t.Run(tt.dialect.Name(), func(t *testing.T) {
			result := tt.dialect.WrapSetOpQueries()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestRepeatedParams verifies that repeated parameters are tracked correctly.
// This is a critical test for the param ordering fix (Phase A1).
func TestRepeatedParams(t *testing.T) {
	col := query.StringColumn{Table: "users", Name: "status"}

	// WHERE (status = :status) OR (status = :status)
	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "status", GoType: "string"},
			},
			Op: query.OpOr,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "status", GoType: "string"},
			},
		},
	}

	for _, dialect := range []Dialect{Postgres, MySQL, SQLite} {
		t.Run(dialect.Name(), func(t *testing.T) {
			compiler := NewCompiler(dialect)
			_, params, err := compiler.Compile(ast)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			// The params list should have the param twice (occurrence order)
			if len(params) != 2 {
				t.Errorf("Expected 2 params (both occurrences), got %d: %v", len(params), params)
			}
			if params[0] != "status" || params[1] != "status" {
				t.Errorf("Expected params [status, status], got %v", params)
			}
		})
	}
}

// TestNewCompilerAPI ensures the new API works correctly
func TestNewCompilerAPI(t *testing.T) {
	col := query.StringColumn{Table: "users", Name: "name"}
	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
	}

	// Test that NewCompiler works for all dialects
	for _, dialect := range []Dialect{Postgres, MySQL, SQLite} {
		t.Run(dialect.Name(), func(t *testing.T) {
			compiler := NewCompiler(dialect)
			sql, _, err := compiler.Compile(ast)
			if err != nil {
				t.Fatalf("NewCompiler(%s).Compile failed: %v", dialect.Name(), err)
			}
			if sql == "" {
				t.Error("Expected non-empty SQL")
			}
		})
	}
}

func TestJSONAgg_EmptyColumns_ReturnsError(t *testing.T) {
	// Manually construct an AST with empty JSONAggExpr.Columns.
	// This should return a compile error, not panic.
	// The builder's SelectJSONAgg panics on empty columns, but if someone
	// constructs an AST manually (tests, tools), the compiler should handle it gracefully.

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{
				Expr: query.JSONAggExpr{
					FieldName: "books",
					Columns:   []query.Column{}, // Empty columns
				},
				Alias: "books",
			},
		},
	}

	// Test all dialects - each should return an error for empty columns
	for _, dialect := range []Dialect{Postgres, MySQL, SQLite} {
		t.Run(dialect.Name(), func(t *testing.T) {
			compiler := NewCompiler(dialect)
			_, _, err := compiler.Compile(ast)
			if err == nil {
				t.Error("Expected error for empty JSON agg columns, but got nil")
			}
		})
	}
}
