package query

import (
	"testing"
)

func TestDelete(t *testing.T) {
	authors := mockTable{name: "authors"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}

	ast := Delete(authors).
		Where(publicIDCol.Eq(Param[string]("public_id"))).
		Build()

	if ast.Kind != DeleteQuery {
		t.Errorf("expected Kind = DeleteQuery, got %v", ast.Kind)
	}
	if ast.FromTable.Name != "authors" {
		t.Errorf("expected FromTable.Name = %q, got %q", "authors", ast.FromTable.Name)
	}
	if ast.Where == nil {
		t.Error("expected Where to be set")
	}
}

func TestDelete_WithComplexWhere(t *testing.T) {
	authors := mockTable{name: "authors"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}
	deletedAtCol := NullTimeColumn{Table: "authors", Name: "deleted_at"}

	ast := Delete(authors).
		Where(And(
			publicIDCol.Eq(Param[string]("public_id")),
			deletedAtCol.IsNull(),
		)).
		Build()

	if ast.Where == nil {
		t.Fatal("expected Where to be set")
	}

	binExpr, ok := ast.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ast.Where)
	}
	if binExpr.Op != OpAnd {
		t.Errorf("expected Op = OpAnd, got %v", binExpr.Op)
	}
}

func TestDelete_WithOrCondition(t *testing.T) {
	authors := mockTable{name: "authors"}
	statusCol := StringColumn{Table: "authors", Name: "status"}

	ast := Delete(authors).
		Where(Or(
			statusCol.Eq(Literal("deleted")),
			statusCol.Eq(Literal("banned")),
		)).
		Build()

	if ast.Where == nil {
		t.Fatal("expected Where to be set")
	}

	binExpr, ok := ast.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ast.Where)
	}
	if binExpr.Op != OpOr {
		t.Errorf("expected Op = OpOr, got %v", binExpr.Op)
	}
}

func TestDelete_WithInCondition(t *testing.T) {
	authors := mockTable{name: "authors"}
	idCol := Int64Column{Table: "authors", Name: "id"}

	ast := Delete(authors).
		Where(idCol.In(Literal(1), Literal(2), Literal(3))).
		Build()

	if ast.Where == nil {
		t.Fatal("expected Where to be set")
	}

	binExpr, ok := ast.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ast.Where)
	}
	if binExpr.Op != OpIn {
		t.Errorf("expected Op = OpIn, got %v", binExpr.Op)
	}

	listExpr, ok := binExpr.Right.(ListExpr)
	if !ok {
		t.Fatalf("expected ListExpr, got %T", binExpr.Right)
	}
	if len(listExpr.Values) != 3 {
		t.Errorf("expected 3 values in list, got %d", len(listExpr.Values))
	}
}

func TestDelete_NoWhere(t *testing.T) {
	authors := mockTable{name: "authors"}

	ast := Delete(authors).Build()

	if ast.Kind != DeleteQuery {
		t.Errorf("expected Kind = DeleteQuery, got %v", ast.Kind)
	}
	if ast.Where != nil {
		t.Error("expected Where to be nil when not set")
	}
}
