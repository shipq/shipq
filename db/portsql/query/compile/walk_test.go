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
