package query

import (
	"testing"
)

func TestColumn_Eq_WithParam(t *testing.T) {
	col := Int64Column{Table: "users", Name: "id"}
	expr := col.Eq(Param[int64]("user_id"))

	binExpr, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}

	if binExpr.Op != OpEq {
		t.Errorf("expected Op = OpEq, got %v", binExpr.Op)
	}

	// Left should be column
	left, ok := binExpr.Left.(ColumnExpr)
	if !ok {
		t.Fatalf("expected left to be ColumnExpr, got %T", binExpr.Left)
	}
	if left.Column.ColumnName() != "id" {
		t.Errorf("expected column name = %q, got %q", "id", left.Column.ColumnName())
	}

	// Right should be param
	right, ok := binExpr.Right.(ParamExpr)
	if !ok {
		t.Fatalf("expected right to be ParamExpr, got %T", binExpr.Right)
	}
	if right.Name != "user_id" {
		t.Errorf("expected param name = %q, got %q", "user_id", right.Name)
	}
	if right.GoType != "int64" {
		t.Errorf("expected param GoType = %q, got %q", "int64", right.GoType)
	}
}

func TestColumn_Eq_WithColumn(t *testing.T) {
	authorID := Int64Column{Table: "authors", Name: "id"}
	bookAuthorID := Int64Column{Table: "books", Name: "author_id"}

	expr := authorID.Eq(bookAuthorID)

	binExpr, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}

	// Both sides should be columns
	left, ok := binExpr.Left.(ColumnExpr)
	if !ok {
		t.Fatalf("expected left to be ColumnExpr, got %T", binExpr.Left)
	}
	if left.Column.TableName() != "authors" {
		t.Errorf("expected left table = %q, got %q", "authors", left.Column.TableName())
	}

	right, ok := binExpr.Right.(ColumnExpr)
	if !ok {
		t.Fatalf("expected right to be ColumnExpr, got %T", binExpr.Right)
	}
	if right.Column.TableName() != "books" {
		t.Errorf("expected right table = %q, got %q", "books", right.Column.TableName())
	}
}

func TestColumn_Eq_WithLiteral(t *testing.T) {
	statusCol := StringColumn{Table: "orders", Name: "status"}
	expr := statusCol.Eq(Literal("pending"))

	binExpr, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}

	right, ok := binExpr.Right.(LiteralExpr)
	if !ok {
		t.Fatalf("expected right to be LiteralExpr, got %T", binExpr.Right)
	}
	if right.Value != "pending" {
		t.Errorf("expected value = %q, got %v", "pending", right.Value)
	}
}

func TestColumn_Comparison_Operators(t *testing.T) {
	col := Int64Column{Table: "t", Name: "val"}

	tests := []struct {
		name     string
		expr     Expr
		expected BinaryOp
	}{
		{"Ne", col.Ne(Literal(1)), OpNe},
		{"Lt", col.Lt(Literal(1)), OpLt},
		{"Le", col.Le(Literal(1)), OpLe},
		{"Gt", col.Gt(Literal(1)), OpGt},
		{"Ge", col.Ge(Literal(1)), OpGe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binExpr, ok := tt.expr.(BinaryExpr)
			if !ok {
				t.Fatalf("expected BinaryExpr, got %T", tt.expr)
			}
			if binExpr.Op != tt.expected {
				t.Errorf("expected Op = %v, got %v", tt.expected, binExpr.Op)
			}
		})
	}
}

func TestColumn_In(t *testing.T) {
	statusCol := StringColumn{Table: "orders", Name: "status"}
	expr := statusCol.In("pending", "processing", "shipped")

	binExpr, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if binExpr.Op != OpIn {
		t.Errorf("expected Op = OpIn, got %v", binExpr.Op)
	}

	list, ok := binExpr.Right.(ListExpr)
	if !ok {
		t.Fatalf("expected right to be ListExpr, got %T", binExpr.Right)
	}
	if len(list.Values) != 3 {
		t.Errorf("expected 3 values in list, got %d", len(list.Values))
	}
}

func TestColumn_IsNull(t *testing.T) {
	col := NullTimeColumn{Table: "users", Name: "deleted_at"}
	expr := col.IsNull()

	unaryExpr, ok := expr.(UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", expr)
	}
	if unaryExpr.Op != OpIsNull {
		t.Errorf("expected Op = OpIsNull, got %v", unaryExpr.Op)
	}
}

func TestColumn_IsNotNull(t *testing.T) {
	col := NullTimeColumn{Table: "users", Name: "deleted_at"}
	expr := col.IsNotNull()

	unaryExpr, ok := expr.(UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", expr)
	}
	if unaryExpr.Op != OpNotNull {
		t.Errorf("expected Op = OpNotNull, got %v", unaryExpr.Op)
	}
}

func TestColumn_Asc(t *testing.T) {
	col := TimeColumn{Table: "users", Name: "created_at"}
	orderBy := col.Asc()

	if orderBy.Desc {
		t.Error("expected Desc = false")
	}

	colExpr, ok := orderBy.Expr.(ColumnExpr)
	if !ok {
		t.Fatalf("expected ColumnExpr, got %T", orderBy.Expr)
	}
	if colExpr.Column.ColumnName() != "created_at" {
		t.Errorf("expected column = %q, got %q", "created_at", colExpr.Column.ColumnName())
	}
}

func TestColumn_Desc(t *testing.T) {
	col := TimeColumn{Table: "users", Name: "created_at"}
	orderBy := col.Desc()

	if !orderBy.Desc {
		t.Error("expected Desc = true")
	}
}

func TestStringColumn_Like(t *testing.T) {
	col := StringColumn{Table: "users", Name: "name"}
	expr := col.Like("%john%")

	binExpr, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if binExpr.Op != OpLike {
		t.Errorf("expected Op = OpLike, got %v", binExpr.Op)
	}

	right, ok := binExpr.Right.(LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr, got %T", binExpr.Right)
	}
	if right.Value != "%john%" {
		t.Errorf("expected pattern = %q, got %v", "%john%", right.Value)
	}
}

func TestStringColumn_ILike(t *testing.T) {
	col := StringColumn{Table: "users", Name: "name"}
	expr := col.ILike("%john%")

	funcExpr, ok := expr.(FuncExpr)
	if !ok {
		t.Fatalf("expected FuncExpr, got %T", expr)
	}
	if funcExpr.Name != "ILIKE" {
		t.Errorf("expected Name = %q, got %q", "ILIKE", funcExpr.Name)
	}
	if len(funcExpr.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(funcExpr.Args))
	}
}

func TestAnd_MultipleConditions(t *testing.T) {
	col1 := Int64Column{Table: "t", Name: "a"}
	col2 := Int64Column{Table: "t", Name: "b"}
	col3 := Int64Column{Table: "t", Name: "c"}

	expr := And(
		col1.Gt(Literal(10)),
		col2.Lt(Literal(20)),
		col3.Eq(Literal(30)),
	)

	// Should create nested AND expressions
	bin1, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if bin1.Op != OpAnd {
		t.Errorf("expected Op = OpAnd, got %v", bin1.Op)
	}

	// Left of outer AND should also be AND
	bin2, ok := bin1.Left.(BinaryExpr)
	if !ok {
		t.Fatalf("expected left to be BinaryExpr, got %T", bin1.Left)
	}
	if bin2.Op != OpAnd {
		t.Errorf("expected Op = OpAnd, got %v", bin2.Op)
	}
}

func TestAnd_SingleCondition(t *testing.T) {
	col := Int64Column{Table: "t", Name: "a"}
	cond := col.Eq(Literal(1))

	expr := And(cond)

	// Should return the single expression unchanged
	if expr != cond {
		t.Error("And with single condition should return that condition")
	}
}

func TestAnd_Empty(t *testing.T) {
	expr := And()

	if expr != nil {
		t.Error("And with no conditions should return nil")
	}
}

func TestOr_MultipleConditions(t *testing.T) {
	statusCol := StringColumn{Table: "orders", Name: "status"}

	expr := Or(
		statusCol.Eq(Literal("pending")),
		statusCol.Eq(Literal("processing")),
	)

	binExpr, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if binExpr.Op != OpOr {
		t.Errorf("expected Op = OpOr, got %v", binExpr.Op)
	}
}

func TestOr_SingleCondition(t *testing.T) {
	col := Int64Column{Table: "t", Name: "a"}
	cond := col.Eq(Literal(1))

	expr := Or(cond)

	if expr != cond {
		t.Error("Or with single condition should return that condition")
	}
}

func TestOr_Empty(t *testing.T) {
	expr := Or()

	if expr != nil {
		t.Error("Or with no conditions should return nil")
	}
}

func TestNot(t *testing.T) {
	col := BoolColumn{Table: "users", Name: "active"}
	expr := Not(col.Eq(Literal(true)))

	unaryExpr, ok := expr.(UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", expr)
	}
	if unaryExpr.Op != OpNot {
		t.Errorf("expected Op = OpNot, got %v", unaryExpr.Op)
	}
}

func TestToExpr_WithExpr(t *testing.T) {
	original := LiteralExpr{Value: 42}
	result := toExpr(original)

	if result != original {
		t.Error("toExpr should return Expr unchanged")
	}
}

func TestToExpr_WithColumn(t *testing.T) {
	col := Int64Column{Table: "t", Name: "id"}
	result := toExpr(col)

	colExpr, ok := result.(ColumnExpr)
	if !ok {
		t.Fatalf("expected ColumnExpr, got %T", result)
	}
	if colExpr.Column != col {
		t.Error("toExpr should wrap Column in ColumnExpr")
	}
}

func TestToExpr_WithLiteral(t *testing.T) {
	result := toExpr(42)

	litExpr, ok := result.(LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr, got %T", result)
	}
	if litExpr.Value != 42 {
		t.Errorf("expected value = 42, got %v", litExpr.Value)
	}
}
