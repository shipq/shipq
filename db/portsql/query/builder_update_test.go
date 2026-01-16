package query

import (
	"testing"
)

func TestUpdate(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}

	ast := Update(authors).
		Set(nameCol, Param[string]("name")).
		Where(publicIDCol.Eq(Param[string]("public_id"))).
		Build()

	if ast.Kind != UpdateQuery {
		t.Errorf("expected Kind = UpdateQuery, got %v", ast.Kind)
	}
	if ast.FromTable.Name != "authors" {
		t.Errorf("expected FromTable.Name = %q, got %q", "authors", ast.FromTable.Name)
	}
	if len(ast.SetClauses) != 1 {
		t.Fatalf("expected 1 SetClause, got %d", len(ast.SetClauses))
	}
	if ast.SetClauses[0].Column.ColumnName() != "name" {
		t.Errorf("expected SetClause column = %q, got %q", "name", ast.SetClauses[0].Column.ColumnName())
	}
	if ast.Where == nil {
		t.Error("expected Where to be set")
	}
}

func TestUpdate_MultipleSetClauses(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	emailCol := StringColumn{Table: "authors", Name: "email"}
	updatedAtCol := TimeColumn{Table: "authors", Name: "updated_at"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}

	ast := Update(authors).
		Set(nameCol, Param[string]("name")).
		Set(emailCol, Param[string]("email")).
		Set(updatedAtCol, Now()).
		Where(publicIDCol.Eq(Param[string]("public_id"))).
		Build()

	if len(ast.SetClauses) != 3 {
		t.Fatalf("expected 3 SetClauses, got %d", len(ast.SetClauses))
	}

	expectedCols := []string{"name", "email", "updated_at"}
	for i, clause := range ast.SetClauses {
		if clause.Column.ColumnName() != expectedCols[i] {
			t.Errorf("expected SetClause %d column = %q, got %q", i, expectedCols[i], clause.Column.ColumnName())
		}
	}

	// Third value should be NOW()
	funcExpr, ok := ast.SetClauses[2].Value.(FuncExpr)
	if !ok {
		t.Fatalf("expected FuncExpr for updated_at, got %T", ast.SetClauses[2].Value)
	}
	if funcExpr.Name != "NOW" {
		t.Errorf("expected Name = %q, got %q", "NOW", funcExpr.Name)
	}
}

func TestUpdate_WithComplexWhere(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}
	deletedAtCol := NullTimeColumn{Table: "authors", Name: "deleted_at"}

	ast := Update(authors).
		Set(nameCol, Param[string]("name")).
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

func TestUpdate_WithLiteralValue(t *testing.T) {
	authors := mockTable{name: "authors"}
	activeCol := BoolColumn{Table: "authors", Name: "active"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}

	ast := Update(authors).
		Set(activeCol, Literal(false)).
		Where(publicIDCol.Eq(Param[string]("public_id"))).
		Build()

	if len(ast.SetClauses) != 1 {
		t.Fatalf("expected 1 SetClause, got %d", len(ast.SetClauses))
	}

	litExpr, ok := ast.SetClauses[0].Value.(LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr, got %T", ast.SetClauses[0].Value)
	}
	if litExpr.Value != false {
		t.Errorf("expected Value = false, got %v", litExpr.Value)
	}
}
