package query

import (
	"testing"
)

func TestInsertInto(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	emailCol := StringColumn{Table: "authors", Name: "email"}

	ast := InsertInto(authors).
		Columns(nameCol, emailCol).
		Values(Param[string]("name"), Param[string]("email")).
		Build()

	if ast.Kind != InsertQuery {
		t.Errorf("expected Kind = InsertQuery, got %v", ast.Kind)
	}
	if ast.FromTable.Name != "authors" {
		t.Errorf("expected FromTable.Name = %q, got %q", "authors", ast.FromTable.Name)
	}
	if len(ast.InsertCols) != 2 {
		t.Errorf("expected 2 InsertCols, got %d", len(ast.InsertCols))
	}
	if len(ast.InsertVals) != 2 {
		t.Errorf("expected 2 InsertVals, got %d", len(ast.InsertVals))
	}
}

func TestInsertInto_WithReturning(t *testing.T) {
	authors := mockTable{name: "authors"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := InsertInto(authors).
		Columns(publicIDCol, nameCol).
		Values(Param[string]("public_id"), Param[string]("name")).
		Returning(publicIDCol).
		Build()

	if len(ast.Returning) != 1 {
		t.Fatalf("expected 1 Returning column, got %d", len(ast.Returning))
	}
	if ast.Returning[0].ColumnName() != "public_id" {
		t.Errorf("expected Returning column = %q, got %q", "public_id", ast.Returning[0].ColumnName())
	}
}

func TestInsertInto_WithMultipleReturning(t *testing.T) {
	authors := mockTable{name: "authors"}
	idCol := Int64Column{Table: "authors", Name: "id"}
	publicIDCol := StringColumn{Table: "authors", Name: "public_id"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := InsertInto(authors).
		Columns(publicIDCol, nameCol).
		Values(Param[string]("public_id"), Param[string]("name")).
		Returning(idCol, publicIDCol).
		Build()

	if len(ast.Returning) != 2 {
		t.Fatalf("expected 2 Returning columns, got %d", len(ast.Returning))
	}
}

func TestInsertInto_WithNowFunction(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	createdAtCol := TimeColumn{Table: "authors", Name: "created_at"}

	ast := InsertInto(authors).
		Columns(nameCol, createdAtCol).
		Values(Param[string]("name"), Now()).
		Build()

	if len(ast.InsertVals) != 2 {
		t.Fatalf("expected 2 InsertVals, got %d", len(ast.InsertVals))
	}

	// Second value should be FuncExpr for NOW()
	funcExpr, ok := ast.InsertVals[1].(FuncExpr)
	if !ok {
		t.Fatalf("expected FuncExpr, got %T", ast.InsertVals[1])
	}
	if funcExpr.Name != "NOW" {
		t.Errorf("expected Name = %q, got %q", "NOW", funcExpr.Name)
	}
}

func TestInsertInto_ColumnsAndValues(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	emailCol := StringColumn{Table: "authors", Name: "email"}
	bioCol := NullStringColumn{Table: "authors", Name: "bio"}

	ast := InsertInto(authors).
		Columns(nameCol, emailCol, bioCol).
		Values(
			Param[string]("name"),
			Param[string]("email"),
			Param[*string]("bio"),
		).
		Build()

	if len(ast.InsertCols) != 3 {
		t.Errorf("expected 3 InsertCols, got %d", len(ast.InsertCols))
	}

	// Check that columns have correct names
	expectedCols := []string{"name", "email", "bio"}
	for i, col := range ast.InsertCols {
		if col.ColumnName() != expectedCols[i] {
			t.Errorf("expected column %d = %q, got %q", i, expectedCols[i], col.ColumnName())
		}
	}

	// Check that values are params
	for i, val := range ast.InsertVals {
		_, ok := val.(ParamExpr)
		if !ok {
			t.Errorf("expected value %d to be ParamExpr, got %T", i, val)
		}
	}
}
