package query

import (
	"testing"
)

func TestUnion(t *testing.T) {
	activeUsers := mockTable{name: "active_users"}
	archivedUsers := mockTable{name: "archived_users"}
	emailCol1 := StringColumn{Table: "active_users", Name: "email"}
	emailCol2 := StringColumn{Table: "archived_users", Name: "email"}

	q1 := From(activeUsers).Select(emailCol1)
	q2 := From(archivedUsers).Select(emailCol2)

	ast := q1.Union(q2).Build()

	if ast.SetOp == nil {
		t.Fatal("expected SetOp to be set")
	}
	if ast.SetOp.Op != SetOpUnion {
		t.Errorf("expected Op = SetOpUnion, got %v", ast.SetOp.Op)
	}
	if ast.SetOp.Left == nil {
		t.Fatal("expected Left query to be set")
	}
	if ast.SetOp.Right == nil {
		t.Fatal("expected Right query to be set")
	}
	if ast.SetOp.Left.FromTable.Name != "active_users" {
		t.Errorf("expected Left.FromTable.Name = %q, got %q", "active_users", ast.SetOp.Left.FromTable.Name)
	}
	if ast.SetOp.Right.FromTable.Name != "archived_users" {
		t.Errorf("expected Right.FromTable.Name = %q, got %q", "archived_users", ast.SetOp.Right.FromTable.Name)
	}
}

func TestUnionAll(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)

	ast := q1.UnionAll(q2).Build()

	if ast.SetOp == nil {
		t.Fatal("expected SetOp to be set")
	}
	if ast.SetOp.Op != SetOpUnionAll {
		t.Errorf("expected Op = SetOpUnionAll, got %v", ast.SetOp.Op)
	}
}

func TestIntersect(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)

	ast := q1.Intersect(q2).Build()

	if ast.SetOp == nil {
		t.Fatal("expected SetOp to be set")
	}
	if ast.SetOp.Op != SetOpIntersect {
		t.Errorf("expected Op = SetOpIntersect, got %v", ast.SetOp.Op)
	}
}

func TestExcept(t *testing.T) {
	allUsers := mockTable{name: "all_users"}
	bannedUsers := mockTable{name: "banned_users"}
	col := StringColumn{Table: "all_users", Name: "email"}

	q1 := From(allUsers).Select(col)
	q2 := From(bannedUsers).Select(col)

	ast := q1.Except(q2).Build()

	if ast.SetOp == nil {
		t.Fatal("expected SetOp to be set")
	}
	if ast.SetOp.Op != SetOpExcept {
		t.Errorf("expected Op = SetOpExcept, got %v", ast.SetOp.Op)
	}
}

func TestSetOpWithOrderBy(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)

	ast := q1.Union(q2).
		OrderBy(OrderByExpr{Expr: ColumnExpr{Column: col}, Desc: false}).
		Build()

	if len(ast.OrderBy) != 1 {
		t.Fatalf("expected 1 OrderBy, got %d", len(ast.OrderBy))
	}
}

func TestSetOpWithLimit(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)

	ast := q1.Union(q2).
		Limit(LiteralExpr{Value: 10}).
		Build()

	if ast.Limit == nil {
		t.Fatal("expected Limit to be set")
	}
	lit, ok := ast.Limit.(LiteralExpr)
	if !ok {
		t.Fatalf("expected Limit to be LiteralExpr, got %T", ast.Limit)
	}
	if lit.Value != 10 {
		t.Errorf("expected Limit = 10, got %v", lit.Value)
	}
}

func TestSetOpWithOffset(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)

	ast := q1.Union(q2).
		Limit(LiteralExpr{Value: 10}).
		Offset(LiteralExpr{Value: 5}).
		Build()

	if ast.Offset == nil {
		t.Fatal("expected Offset to be set")
	}
}

func TestChainedUnion(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	table3 := mockTable{name: "table3"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)
	q3 := From(table3).Select(col)

	// (q1 UNION q2) UNION q3
	ast := q1.Union(q2).Union(q3).Build()

	if ast.SetOp == nil {
		t.Fatal("expected SetOp to be set")
	}
	if ast.SetOp.Op != SetOpUnion {
		t.Errorf("expected outer Op = SetOpUnion, got %v", ast.SetOp.Op)
	}

	// Left side should be another set operation
	if ast.SetOp.Left.SetOp == nil {
		t.Fatal("expected Left.SetOp to be set (chained)")
	}
	if ast.SetOp.Left.SetOp.Op != SetOpUnion {
		t.Errorf("expected inner Op = SetOpUnion, got %v", ast.SetOp.Left.SetOp.Op)
	}
}

func TestMixedSetOps(t *testing.T) {
	table1 := mockTable{name: "table1"}
	table2 := mockTable{name: "table2"}
	table3 := mockTable{name: "table3"}
	col := StringColumn{Table: "table1", Name: "col"}

	q1 := From(table1).Select(col)
	q2 := From(table2).Select(col)
	q3 := From(table3).Select(col)

	// (q1 UNION q2) EXCEPT q3
	ast := q1.Union(q2).Except(q3).Build()

	if ast.SetOp.Op != SetOpExcept {
		t.Errorf("expected outer Op = SetOpExcept, got %v", ast.SetOp.Op)
	}
	if ast.SetOp.Left.SetOp.Op != SetOpUnion {
		t.Errorf("expected inner Op = SetOpUnion, got %v", ast.SetOp.Left.SetOp.Op)
	}
}
