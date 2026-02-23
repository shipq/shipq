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
