package query

import (
	"testing"
)

func TestCount(t *testing.T) {
	expr := Count()

	if expr.Func != AggCount {
		t.Errorf("expected Func = AggCount, got %v", expr.Func)
	}
	if expr.Arg != nil {
		t.Errorf("expected Arg = nil for COUNT(*), got %v", expr.Arg)
	}
	if expr.Distinct {
		t.Errorf("expected Distinct = false, got true")
	}
}

func TestCountCol(t *testing.T) {
	col := StringColumn{Table: "users", Name: "email"}
	expr := CountCol(col)

	if expr.Func != AggCount {
		t.Errorf("expected Func = AggCount, got %v", expr.Func)
	}
	if expr.Arg == nil {
		t.Fatal("expected Arg to be set")
	}
	colExpr, ok := expr.Arg.(ColumnExpr)
	if !ok {
		t.Fatalf("expected Arg to be ColumnExpr, got %T", expr.Arg)
	}
	if colExpr.Column.ColumnName() != "email" {
		t.Errorf("expected column name = %q, got %q", "email", colExpr.Column.ColumnName())
	}
}

func TestCountDistinct(t *testing.T) {
	col := StringColumn{Table: "users", Name: "email"}
	expr := CountDistinct(col)

	if expr.Func != AggCount {
		t.Errorf("expected Func = AggCount, got %v", expr.Func)
	}
	if !expr.Distinct {
		t.Errorf("expected Distinct = true, got false")
	}
}

func TestSum(t *testing.T) {
	col := Int64Column{Table: "orders", Name: "amount"}
	expr := Sum(col)

	if expr.Func != AggSum {
		t.Errorf("expected Func = AggSum, got %v", expr.Func)
	}
	if expr.Arg == nil {
		t.Fatal("expected Arg to be set")
	}
}

func TestAvg(t *testing.T) {
	col := Float64Column{Table: "products", Name: "price"}
	expr := Avg(col)

	if expr.Func != AggAvg {
		t.Errorf("expected Func = AggAvg, got %v", expr.Func)
	}
}

func TestMin(t *testing.T) {
	col := Int64Column{Table: "products", Name: "price"}
	expr := Min(col)

	if expr.Func != AggMin {
		t.Errorf("expected Func = AggMin, got %v", expr.Func)
	}
}

func TestMax(t *testing.T) {
	col := Int64Column{Table: "products", Name: "price"}
	expr := Max(col)

	if expr.Func != AggMax {
		t.Errorf("expected Func = AggMax, got %v", expr.Func)
	}
}

func TestSelectCount(t *testing.T) {
	users := mockTable{name: "users"}

	ast := From(users).
		SelectCount().
		Build()

	if len(ast.SelectCols) != 1 {
		t.Fatalf("expected 1 SelectCol, got %d", len(ast.SelectCols))
	}

	aggExpr, ok := ast.SelectCols[0].Expr.(AggregateExpr)
	if !ok {
		t.Fatalf("expected AggregateExpr, got %T", ast.SelectCols[0].Expr)
	}
	if aggExpr.Func != AggCount {
		t.Errorf("expected AggCount, got %v", aggExpr.Func)
	}
}

func TestSelectCountAs(t *testing.T) {
	users := mockTable{name: "users"}

	ast := From(users).
		SelectCountAs("total").
		Build()

	if len(ast.SelectCols) != 1 {
		t.Fatalf("expected 1 SelectCol, got %d", len(ast.SelectCols))
	}
	if ast.SelectCols[0].Alias != "total" {
		t.Errorf("expected alias = %q, got %q", "total", ast.SelectCols[0].Alias)
	}
}

func TestSelectCountDistinct(t *testing.T) {
	users := mockTable{name: "users"}
	emailCol := StringColumn{Table: "users", Name: "email"}

	ast := From(users).
		SelectCountDistinct(emailCol).
		Build()

	aggExpr, ok := ast.SelectCols[0].Expr.(AggregateExpr)
	if !ok {
		t.Fatalf("expected AggregateExpr, got %T", ast.SelectCols[0].Expr)
	}
	if !aggExpr.Distinct {
		t.Error("expected Distinct = true")
	}
}

func TestSelectSumAs(t *testing.T) {
	orders := mockTable{name: "orders"}
	amountCol := Int64Column{Table: "orders", Name: "amount"}

	ast := From(orders).
		SelectSumAs(amountCol, "total_amount").
		Build()

	aggExpr, ok := ast.SelectCols[0].Expr.(AggregateExpr)
	if !ok {
		t.Fatalf("expected AggregateExpr, got %T", ast.SelectCols[0].Expr)
	}
	if aggExpr.Func != AggSum {
		t.Errorf("expected AggSum, got %v", aggExpr.Func)
	}
	if ast.SelectCols[0].Alias != "total_amount" {
		t.Errorf("expected alias = %q, got %q", "total_amount", ast.SelectCols[0].Alias)
	}
}

func TestAggregateWithGroupBy(t *testing.T) {
	orders := mockTable{name: "orders"}
	customerID := Int64Column{Table: "orders", Name: "customer_id"}
	amountCol := Int64Column{Table: "orders", Name: "amount"}

	ast := From(orders).
		Select(customerID).
		SelectSumAs(amountCol, "total").
		GroupBy(customerID).
		Build()

	if len(ast.GroupBy) != 1 {
		t.Fatalf("expected 1 GroupBy column, got %d", len(ast.GroupBy))
	}
	if ast.GroupBy[0].ColumnName() != "customer_id" {
		t.Errorf("expected GroupBy column = %q, got %q", "customer_id", ast.GroupBy[0].ColumnName())
	}
}
