package compile

import (
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
)

func TestWalkExpr_SimpleExpression(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}
	expr := query.BinaryExpr{
		Left:  query.ColumnExpr{Column: col},
		Op:    query.OpEq,
		Right: query.ParamExpr{Name: "id", GoType: "int64"},
	}

	var visited []string
	WalkExpr(expr, func(e query.Expr) bool {
		switch v := e.(type) {
		case query.BinaryExpr:
			visited = append(visited, "BinaryExpr")
		case query.ColumnExpr:
			visited = append(visited, "ColumnExpr")
		case query.ParamExpr:
			visited = append(visited, "ParamExpr:"+v.Name)
		}
		return true
	})

	if len(visited) != 3 {
		t.Errorf("Expected 3 visits, got %d: %v", len(visited), visited)
	}
}

func TestWalkExpr_StopOnFalse(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}
	expr := query.BinaryExpr{
		Left:  query.ColumnExpr{Column: col},
		Op:    query.OpEq,
		Right: query.ParamExpr{Name: "id", GoType: "int64"},
	}

	count := 0
	WalkExpr(expr, func(e query.Expr) bool {
		count++
		return false // Stop after first visit
	})

	if count != 1 {
		t.Errorf("Expected 1 visit when returning false, got %d", count)
	}
}

func TestCollectParams_UniqueParams(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}
	nameCol := query.StringColumn{Table: "users", Name: "name"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "id", GoType: "int64"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: nameCol},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "name", GoType: "string"},
			},
		},
	}

	params := CollectParams(ast)

	if len(params) != 2 {
		t.Errorf("Expected 2 unique params, got %d", len(params))
	}
}

func TestCollectParams_DuplicatesNotRepeated(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	// Same param used twice in WHERE
	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpGt,
				Right: query.ParamExpr{Name: "id", GoType: "int64"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpLt,
				Right: query.ParamExpr{Name: "id", GoType: "int64"}, // Same param
			},
		},
	}

	params := CollectParams(ast)

	if len(params) != 1 {
		t.Errorf("Expected 1 unique param (duplicates removed), got %d", len(params))
	}
}

func TestCollectParamOrder_WithDuplicates(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpGt,
				Right: query.ParamExpr{Name: "min", GoType: "int64"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col},
				Op:    query.OpLt,
				Right: query.ParamExpr{Name: "max", GoType: "int64"},
			},
		},
	}

	order := CollectParamOrder(ast)

	if len(order) != 2 {
		t.Errorf("Expected 2 params in order, got %d", len(order))
	}
}

func TestHasSubqueries_NoSubqueries(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: col},
			Op:    query.OpEq,
			Right: query.LiteralExpr{Value: 1},
		},
	}

	if HasSubqueries(ast) {
		t.Error("Expected no subqueries, but HasSubqueries returned true")
	}
}

func TestHasSubqueries_WithSubquery(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	subquery := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "admins"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
	}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: col},
			Op:    query.OpIn,
			Right: query.SubqueryExpr{Query: subquery},
		},
	}

	if !HasSubqueries(ast) {
		t.Error("Expected subqueries, but HasSubqueries returned false")
	}
}

func TestHasSubqueries_WithExists(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	subquery := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "admins"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
	}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where:      query.ExistsExpr{Subquery: subquery},
	}

	if !HasSubqueries(ast) {
		t.Error("Expected EXISTS subquery to be detected")
	}
}

func TestWalkAST_BulkInsert(t *testing.T) {
	// Construct a 3-row INSERT, walk the AST, collect all ParamExpr names
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "t"},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "a0", GoType: "string"}, query.ParamExpr{Name: "b0", GoType: "string"}},
			{query.ParamExpr{Name: "a1", GoType: "string"}, query.ParamExpr{Name: "b1", GoType: "string"}},
			{query.ParamExpr{Name: "a2", GoType: "string"}, query.ParamExpr{Name: "b2", GoType: "string"}},
		},
	}

	var paramNames []string
	WalkAST(ast, func(expr query.Expr) bool {
		if p, ok := expr.(query.ParamExpr); ok {
			paramNames = append(paramNames, p.Name)
		}
		return true
	})

	expected := []string{"a0", "b0", "a1", "b1", "a2", "b2"}
	if len(paramNames) != len(expected) {
		t.Fatalf("expected %d params, got %d: %v", len(expected), len(paramNames), paramNames)
	}
	for i, name := range expected {
		if paramNames[i] != name {
			t.Errorf("param %d: expected %q, got %q", i, name, paramNames[i])
		}
	}
}

func TestCollectParams_BulkInsert(t *testing.T) {
	// CollectParams should deduplicate
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "t"},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "name", GoType: "string"}, query.ParamExpr{Name: "email", GoType: "string"}},
			{query.ParamExpr{Name: "name", GoType: "string"}, query.ParamExpr{Name: "email", GoType: "string"}},
		},
	}

	params := CollectParams(ast)
	if len(params) != 2 {
		t.Errorf("Expected 2 unique params, got %d: %v", len(params), params)
	}
	if params[0].Name != "name" {
		t.Errorf("Expected first param 'name', got %q", params[0].Name)
	}
	if params[1].Name != "email" {
		t.Errorf("Expected second param 'email', got %q", params[1].Name)
	}
}

func TestCollectParamOrder_BulkInsert(t *testing.T) {
	// CollectParamOrder should return all occurrences in order
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "t"},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "a", GoType: "string"}, query.ParamExpr{Name: "b", GoType: "string"}},
			{query.ParamExpr{Name: "a", GoType: "string"}, query.ParamExpr{Name: "b", GoType: "string"}},
			{query.ParamExpr{Name: "a", GoType: "string"}, query.ParamExpr{Name: "b", GoType: "string"}},
		},
	}

	order := CollectParamOrder(ast)
	expected := []string{"a", "b", "a", "b", "a", "b"}
	if len(order) != len(expected) {
		t.Fatalf("Expected %d params in order, got %d: %v", len(expected), len(order), order)
	}
	for i, name := range expected {
		if order[i] != name {
			t.Errorf("param %d: expected %q, got %q", i, name, order[i])
		}
	}
}

func TestWalkAST_InsertSelect(t *testing.T) {
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "target"},
		InsertCols: []query.Column{query.StringColumn{Table: "target", Name: "name"}},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "status"}},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "status", GoType: "string"},
			},
		},
	}
	params := CollectParamOrder(ast)
	if len(params) != 1 || params[0] != "status" {
		t.Errorf("expected params [status], got %v", params)
	}
}

func TestWalkAST_InsertSelect_WithCTE(t *testing.T) {
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "target"},
		InsertCols: []query.Column{query.StringColumn{Table: "target", Name: "name"}},
		CTEs: []query.CTE{
			{
				Name: "filtered",
				Query: &query.AST{
					Kind:      query.SelectQuery,
					FromTable: query.TableRef{Name: "source"},
					SelectCols: []query.SelectExpr{
						{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
					},
					Where: query.BinaryExpr{
						Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "active"}},
						Op:    query.OpEq,
						Right: query.ParamExpr{Name: "p1", GoType: "bool"},
					},
				},
			},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "filtered"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "filtered", Name: "name"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "filtered", Name: "name"}},
				Op:    query.OpNe,
				Right: query.ParamExpr{Name: "p2", GoType: "string"},
			},
		},
	}
	params := CollectParamOrder(ast)
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %v", len(params), params)
	}
	if params[0] != "p2" {
		t.Errorf("expected first param 'p2', got %q", params[0])
	}
	if params[1] != "p1" {
		t.Errorf("expected second param 'p1', got %q", params[1])
	}
}

func TestCollectParams_InsertSelect(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
			query.StringColumn{Table: "target", Name: "status"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "status"}}},
			},
			Where: query.BinaryExpr{
				Left: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "status"}},
					Op:    query.OpEq,
					Right: query.ParamExpr{Name: "status", GoType: "string"},
				},
				Op: query.OpAnd,
				Right: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "status"}},
					Op:    query.OpNe,
					Right: query.ParamExpr{Name: "status", GoType: "string"}, // duplicate
				},
			},
		},
	}
	params := CollectParams(ast)
	if len(params) != 1 {
		t.Errorf("expected 1 unique param (deduplicated), got %d: %v", len(params), params)
	}
	if len(params) > 0 && params[0].Name != "status" {
		t.Errorf("expected param name 'status', got %q", params[0].Name)
	}
}

func TestCollectParamOrder_InsertSelect_WithCTE(t *testing.T) {
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "target"},
		InsertCols: []query.Column{query.StringColumn{Table: "target", Name: "val"}},
		CTEs: []query.CTE{
			{
				Name: "cte1",
				Query: &query.AST{
					Kind:      query.SelectQuery,
					FromTable: query.TableRef{Name: "t1"},
					SelectCols: []query.SelectExpr{
						{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "t1", Name: "val"}}},
					},
					Where: query.BinaryExpr{
						Left:  query.ColumnExpr{Column: query.StringColumn{Table: "t1", Name: "x"}},
						Op:    query.OpEq,
						Right: query.ParamExpr{Name: "cte_param", GoType: "string"},
					},
				},
			},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "cte1"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "cte1", Name: "val"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "cte1", Name: "val"}},
				Op:    query.OpNe,
				Right: query.ParamExpr{Name: "source_param", GoType: "string"},
			},
		},
	}
	order := CollectParamOrder(ast)
	if len(order) != 2 {
		t.Fatalf("expected 2 params in order, got %d: %v", len(order), order)
	}
	if order[0] != "source_param" {
		t.Errorf("expected first param 'source_param', got %q", order[0])
	}
	if order[1] != "cte_param" {
		t.Errorf("expected second param 'cte_param', got %q", order[1])
	}
}

func TestWalkAST_InsertSelect_NoParams(t *testing.T) {
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "target"},
		InsertCols: []query.Column{query.StringColumn{Table: "target", Name: "name"}},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "active"}},
				Op:    query.OpEq,
				Right: query.LiteralExpr{Value: true},
			},
		},
	}
	params := CollectParamOrder(ast)
	if len(params) != 0 {
		t.Errorf("expected 0 params, got %d: %v", len(params), params)
	}
}

func TestHasSubqueries_InsertSelect(t *testing.T) {
	// INSERT ... SELECT where the source SELECT contains a SubqueryExpr
	subAST := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "other"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "other", Name: "id"}}},
		},
	}

	astWithSubquery := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "target"},
		InsertCols: []query.Column{query.StringColumn{Table: "target", Name: "name"}},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.Int64Column{Table: "source", Name: "id"}},
				Op:    query.OpIn,
				Right: query.SubqueryExpr{Query: subAST},
			},
		},
	}

	if !HasSubqueries(astWithSubquery) {
		t.Error("expected HasSubqueries to return true for INSERT ... SELECT with SubqueryExpr")
	}

	// INSERT ... SELECT without any SubqueryExpr in the source
	astWithoutSubquery := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "target"},
		InsertCols: []query.Column{query.StringColumn{Table: "target", Name: "name"}},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "active"}},
				Op:    query.OpEq,
				Right: query.LiteralExpr{Value: true},
			},
		},
	}

	if HasSubqueries(astWithoutSubquery) {
		t.Error("expected HasSubqueries to return false for INSERT ... SELECT without SubqueryExpr")
	}
}
