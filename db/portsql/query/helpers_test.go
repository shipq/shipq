package query

import (
	"testing"
	"time"
)

func TestParam_Int64(t *testing.T) {
	p := Param[int64]("user_id")

	if p.Name != "user_id" {
		t.Errorf("expected Name = %q, got %q", "user_id", p.Name)
	}
	if p.GoType != "int64" {
		t.Errorf("expected GoType = %q, got %q", "int64", p.GoType)
	}
}

func TestParam_String(t *testing.T) {
	p := Param[string]("name")

	if p.Name != "name" {
		t.Errorf("expected Name = %q, got %q", "name", p.Name)
	}
	if p.GoType != "string" {
		t.Errorf("expected GoType = %q, got %q", "string", p.GoType)
	}
}

func TestParam_Int(t *testing.T) {
	p := Param[int]("limit")

	if p.Name != "limit" {
		t.Errorf("expected Name = %q, got %q", "limit", p.Name)
	}
	if p.GoType != "int" {
		t.Errorf("expected GoType = %q, got %q", "int", p.GoType)
	}
}

func TestParam_Bool(t *testing.T) {
	p := Param[bool]("active")

	if p.Name != "active" {
		t.Errorf("expected Name = %q, got %q", "active", p.Name)
	}
	if p.GoType != "bool" {
		t.Errorf("expected GoType = %q, got %q", "bool", p.GoType)
	}
}

func TestParam_Float64(t *testing.T) {
	p := Param[float64]("price")

	if p.Name != "price" {
		t.Errorf("expected Name = %q, got %q", "price", p.Name)
	}
	if p.GoType != "float64" {
		t.Errorf("expected GoType = %q, got %q", "float64", p.GoType)
	}
}

func TestLiteral_Int(t *testing.T) {
	l := Literal(42)

	if l.Value != 42 {
		t.Errorf("expected Value = 42, got %v", l.Value)
	}
}

func TestLiteral_String(t *testing.T) {
	l := Literal("hello")

	if l.Value != "hello" {
		t.Errorf("expected Value = %q, got %v", "hello", l.Value)
	}
}

func TestLiteral_Bool(t *testing.T) {
	l := Literal(true)

	if l.Value != true {
		t.Errorf("expected Value = true, got %v", l.Value)
	}
}

func TestNow(t *testing.T) {
	n := Now()

	if n.Name != "NOW" {
		t.Errorf("expected Name = %q, got %q", "NOW", n.Name)
	}
	if n.Args != nil {
		t.Errorf("expected Args = nil, got %v", n.Args)
	}
}

func TestTypeNameOf(t *testing.T) {
	tests := []struct {
		value    any
		expected string
	}{
		{int(0), "int"},
		{int8(0), "int8"},
		{int16(0), "int16"},
		{int32(0), "int32"},
		{int64(0), "int64"},
		{uint(0), "uint"},
		{uint8(0), "uint8"},
		{uint16(0), "uint16"},
		{uint32(0), "uint32"},
		{uint64(0), "uint64"},
		{float32(0), "float32"},
		{float64(0), "float64"},
		{string(""), "string"},
		{bool(false), "bool"},
		{[]byte{}, "[]byte"},
		{time.Time{}, "time.Time"},
		{(*int)(nil), "*int"},
		{(*int64)(nil), "*int64"},
		{(*int32)(nil), "*int32"},
		{(*string)(nil), "*string"},
		{(*bool)(nil), "*bool"},
		{(*float64)(nil), "*float64"},
		{(*time.Time)(nil), "*time.Time"},
		{struct{}{}, "any"}, // unknown types become "any"
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := typeNameOf(tt.value)
			if result != tt.expected {
				t.Errorf("typeNameOf(%T) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestParamExpr_ImplementsExpr(t *testing.T) {
	var _ Expr = ParamExpr{}
}

func TestLiteralExpr_ImplementsExpr(t *testing.T) {
	var _ Expr = LiteralExpr{}
}

func TestFuncExpr_ImplementsExpr(t *testing.T) {
	var _ Expr = FuncExpr{}
}

func TestCoalesce_TwoArgs(t *testing.T) {
	col := NullStringColumn{Table: "job_results", Name: "started_at"}
	expr := Coalesce(Param[*string]("startedAt"), ColumnExpr{Column: col})

	if expr.Name != "COALESCE" {
		t.Errorf("expected Name = %q, got %q", "COALESCE", expr.Name)
	}
	if len(expr.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(expr.Args))
	}

	// First arg should be a ParamExpr
	paramExpr, ok := expr.Args[0].(ParamExpr)
	if !ok {
		t.Fatalf("expected first arg to be ParamExpr, got %T", expr.Args[0])
	}
	if paramExpr.Name != "startedAt" {
		t.Errorf("expected param name = %q, got %q", "startedAt", paramExpr.Name)
	}
	if paramExpr.GoType != "*string" {
		t.Errorf("expected param GoType = %q, got %q", "*string", paramExpr.GoType)
	}

	// Second arg should be a ColumnExpr
	colExpr, ok := expr.Args[1].(ColumnExpr)
	if !ok {
		t.Fatalf("expected second arg to be ColumnExpr, got %T", expr.Args[1])
	}
	if colExpr.Column.ColumnName() != "started_at" {
		t.Errorf("expected column name = %q, got %q", "started_at", colExpr.Column.ColumnName())
	}
}

func TestCoalesce_SingleArg(t *testing.T) {
	expr := Coalesce(Param[*string]("value"))

	if expr.Name != "COALESCE" {
		t.Errorf("expected Name = %q, got %q", "COALESCE", expr.Name)
	}
	if len(expr.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(expr.Args))
	}
}

func TestCoalesce_NoArgs(t *testing.T) {
	expr := Coalesce()

	if expr.Name != "COALESCE" {
		t.Errorf("expected Name = %q, got %q", "COALESCE", expr.Name)
	}
	if len(expr.Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(expr.Args))
	}
}

func TestCoalesce_ImplementsExpr(t *testing.T) {
	var _ Expr = Coalesce(Param[*string]("x"), Literal("default"))
}

func TestCoalesce_InSetClause(t *testing.T) {
	// Verify Coalesce can be used as the value in an Update().Set() clause,
	// which requires the value to satisfy the Expr interface.
	col := NullStringColumn{Table: "jobs", Name: "started_at"}
	coalesceExpr := Coalesce(Param[*string]("startedAt"), ColumnExpr{Column: col})

	// Build an UPDATE with the COALESCE in a SET clause
	table := mockTable{name: "jobs"}
	ast := Update(table).
		Set(col, coalesceExpr).
		Where(StringColumn{Table: "jobs", Name: "id"}.Eq(Param[string]("id"))).
		Build()

	if len(ast.SetClauses) != 1 {
		t.Fatalf("expected 1 set clause, got %d", len(ast.SetClauses))
	}

	funcExpr, ok := ast.SetClauses[0].Value.(FuncExpr)
	if !ok {
		t.Fatalf("expected FuncExpr in set clause value, got %T", ast.SetClauses[0].Value)
	}
	if funcExpr.Name != "COALESCE" {
		t.Errorf("expected COALESCE, got %q", funcExpr.Name)
	}
	if len(funcExpr.Args) != 2 {
		t.Errorf("expected 2 args in COALESCE, got %d", len(funcExpr.Args))
	}
}
