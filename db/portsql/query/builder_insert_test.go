package query

import (
	"fmt"
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
	if len(ast.InsertRows) != 1 {
		t.Errorf("expected 1 InsertRows, got %d", len(ast.InsertRows))
	}
	if len(ast.InsertRows[0]) != 2 {
		t.Errorf("expected 2 values in first row, got %d", len(ast.InsertRows[0]))
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

	if len(ast.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows, got %d", len(ast.InsertRows))
	}
	if len(ast.InsertRows[0]) != 2 {
		t.Fatalf("expected 2 values in first row, got %d", len(ast.InsertRows[0]))
	}

	// Second value should be FuncExpr for NOW()
	funcExpr, ok := ast.InsertRows[0][1].(FuncExpr)
	if !ok {
		t.Fatalf("expected FuncExpr, got %T", ast.InsertRows[0][1])
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
	for i, val := range ast.InsertRows[0] {
		_, ok := val.(ParamExpr)
		if !ok {
			t.Errorf("expected value %d to be ParamExpr, got %T", i, val)
		}
	}
}

func TestInsertInto_Values_BackwardCompat(t *testing.T) {
	// Calling .Values() should produce InsertRows with exactly 1 element.
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	emailCol := StringColumn{Table: "authors", Name: "email"}

	ast := InsertInto(authors).
		Columns(nameCol, emailCol).
		Values(Param[string]("name"), Param[string]("email")).
		Build()

	if len(ast.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows, got %d", len(ast.InsertRows))
	}
	if len(ast.InsertRows[0]) != 2 {
		t.Fatalf("expected 2 values in first row, got %d", len(ast.InsertRows[0]))
	}

	// Verify params are correct
	p0, ok := ast.InsertRows[0][0].(ParamExpr)
	if !ok {
		t.Fatalf("expected ParamExpr, got %T", ast.InsertRows[0][0])
	}
	if p0.Name != "name" {
		t.Errorf("expected param name %q, got %q", "name", p0.Name)
	}

	p1, ok := ast.InsertRows[0][1].(ParamExpr)
	if !ok {
		t.Fatalf("expected ParamExpr, got %T", ast.InsertRows[0][1])
	}
	if p1.Name != "email" {
		t.Errorf("expected param name %q, got %q", "email", p1.Name)
	}
}

func TestInsertInto_Values_ReplacesRows(t *testing.T) {
	// Calling .Values() multiple times should replace the previous row.
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := InsertInto(authors).
		Columns(nameCol).
		Values(Param[string]("name_first")).
		Values(Param[string]("name_second")).
		Build()

	if len(ast.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows after multiple Values() calls, got %d", len(ast.InsertRows))
	}
	p, ok := ast.InsertRows[0][0].(ParamExpr)
	if !ok {
		t.Fatalf("expected ParamExpr, got %T", ast.InsertRows[0][0])
	}
	if p.Name != "name_second" {
		t.Errorf("expected param name %q (last Values call wins), got %q", "name_second", p.Name)
	}
}

func TestInsertInto_AddRow_MultipleRows(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	emailCol := StringColumn{Table: "authors", Name: "email"}

	ast := InsertInto(authors).
		Columns(nameCol, emailCol).
		AddRow(Param[string]("name_0"), Param[string]("email_0")).
		AddRow(Param[string]("name_1"), Param[string]("email_1")).
		AddRow(Param[string]("name_2"), Param[string]("email_2")).
		Build()

	if len(ast.InsertRows) != 3 {
		t.Fatalf("expected 3 InsertRows, got %d", len(ast.InsertRows))
	}

	for i, row := range ast.InsertRows {
		if len(row) != 2 {
			t.Errorf("row %d: expected 2 values, got %d", i, len(row))
		}
		// Check first param in each row
		p, ok := row[0].(ParamExpr)
		if !ok {
			t.Errorf("row %d: expected ParamExpr, got %T", i, row[0])
			continue
		}
		expectedName := fmt.Sprintf("name_%d", i)
		if p.Name != expectedName {
			t.Errorf("row %d: expected param name %q, got %q", i, expectedName, p.Name)
		}
	}
}

func TestInsertInto_AddRow_WithNow(t *testing.T) {
	// Mix Param and Now() in bulk rows
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	createdAtCol := TimeColumn{Table: "authors", Name: "created_at"}

	ast := InsertInto(authors).
		Columns(nameCol, createdAtCol).
		AddRow(Param[string]("name_0"), Now()).
		AddRow(Param[string]("name_1"), Now()).
		Build()

	if len(ast.InsertRows) != 2 {
		t.Fatalf("expected 2 InsertRows, got %d", len(ast.InsertRows))
	}

	for i, row := range ast.InsertRows {
		if len(row) != 2 {
			t.Errorf("row %d: expected 2 values, got %d", i, len(row))
			continue
		}
		// First value should be ParamExpr
		_, ok := row[0].(ParamExpr)
		if !ok {
			t.Errorf("row %d: expected ParamExpr for first value, got %T", i, row[0])
		}
		// Second value should be FuncExpr for NOW()
		funcExpr, ok := row[1].(FuncExpr)
		if !ok {
			t.Errorf("row %d: expected FuncExpr for second value, got %T", i, row[1])
			continue
		}
		if funcExpr.Name != "NOW" {
			t.Errorf("row %d: expected func name %q, got %q", i, "NOW", funcExpr.Name)
		}
	}
}

func TestInsertInto_BulkRows(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	emailCol := StringColumn{Table: "authors", Name: "email"}

	rows := make([][]Expr, 5)
	for i := range rows {
		rows[i] = []Expr{
			Param[string](fmt.Sprintf("name_%d", i)),
			Param[string](fmt.Sprintf("email_%d", i)),
		}
	}

	ast := InsertInto(authors).
		Columns(nameCol, emailCol).
		BulkRows(rows).
		Build()

	if len(ast.InsertRows) != 5 {
		t.Fatalf("expected 5 InsertRows, got %d", len(ast.InsertRows))
	}

	for i, row := range ast.InsertRows {
		if len(row) != 2 {
			t.Errorf("row %d: expected 2 values, got %d", i, len(row))
		}
	}
}

func TestInsertInto_BulkRows_ReplacesExisting(t *testing.T) {
	// BulkRows should replace any previously added rows.
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := InsertInto(authors).
		Columns(nameCol).
		AddRow(Param[string]("old_name")).
		BulkRows([][]Expr{
			{Param[string]("new_name_0")},
			{Param[string]("new_name_1")},
		}).
		Build()

	if len(ast.InsertRows) != 2 {
		t.Fatalf("expected 2 InsertRows after BulkRows, got %d", len(ast.InsertRows))
	}

	p, ok := ast.InsertRows[0][0].(ParamExpr)
	if !ok {
		t.Fatalf("expected ParamExpr, got %T", ast.InsertRows[0][0])
	}
	if p.Name != "new_name_0" {
		t.Errorf("expected param name %q, got %q", "new_name_0", p.Name)
	}
}

func TestInsertInto_AddRow_SingleRow(t *testing.T) {
	// AddRow with a single row should work identically to Values.
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := InsertInto(authors).
		Columns(nameCol).
		AddRow(Param[string]("name")).
		Build()

	if len(ast.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows, got %d", len(ast.InsertRows))
	}
	if len(ast.InsertRows[0]) != 1 {
		t.Fatalf("expected 1 value in row, got %d", len(ast.InsertRows[0]))
	}
}

func TestInsertInto_FirstInsertRow(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	// Test with rows
	ast := InsertInto(authors).
		Columns(nameCol).
		Values(Param[string]("name")).
		Build()

	row := ast.FirstInsertRow()
	if row == nil {
		t.Fatal("FirstInsertRow returned nil")
	}
	if len(row) != 1 {
		t.Fatalf("expected 1 value, got %d", len(row))
	}

	// Test with no rows
	emptyAST := &AST{Kind: InsertQuery}
	if emptyAST.FirstInsertRow() != nil {
		t.Error("FirstInsertRow should return nil for empty InsertRows")
	}
}

func TestInsertInto_FromSelect_Basic(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	emailCol := StringColumn{Table: "target", Name: "email"}
	srcName := StringColumn{Table: "source", Name: "name"}
	srcEmail := StringColumn{Table: "source", Name: "email"}

	ast := InsertInto(target).
		Columns(nameCol, emailCol).
		FromSelect(From(source).Select(srcName, srcEmail)).
		Build()

	if ast.Kind != InsertQuery {
		t.Errorf("expected Kind = InsertQuery, got %v", ast.Kind)
	}
	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set")
	}
	if ast.InsertSource.Kind != SelectQuery {
		t.Errorf("expected InsertSource.Kind = SelectQuery, got %v", ast.InsertSource.Kind)
	}
	if ast.InsertSource.FromTable.Name != "source" {
		t.Errorf("expected InsertSource.FromTable.Name = %q, got %q", "source", ast.InsertSource.FromTable.Name)
	}
	if len(ast.InsertRows) != 0 {
		t.Errorf("expected 0 InsertRows, got %d", len(ast.InsertRows))
	}
	if len(ast.InsertCols) != 2 {
		t.Errorf("expected 2 InsertCols, got %d", len(ast.InsertCols))
	}
}

func TestInsertInto_FromSelect_ClearsValues(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	ast := InsertInto(target).
		Columns(nameCol).
		Values(Param[string]("name")).
		FromSelect(From(source).Select(srcName)).
		Build()

	if ast.InsertRows != nil {
		t.Errorf("expected InsertRows to be nil after FromSelect, got %v", ast.InsertRows)
	}
	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set after FromSelect")
	}
}

func TestInsertInto_Values_ClearsFromSelect(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelect(From(source).Select(srcName)).
		Values(Param[string]("name")).
		Build()

	if ast.InsertSource != nil {
		t.Errorf("expected InsertSource to be nil after Values, got %v", ast.InsertSource)
	}
	if len(ast.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows after Values, got %d", len(ast.InsertRows))
	}
}

func TestInsertInto_FromSelectAST(t *testing.T) {
	target := mockTable{name: "target"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	sourceAST := &AST{
		Kind:      SelectQuery,
		FromTable: TableRef{Name: "source"},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: srcName}},
		},
	}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelectAST(sourceAST).
		Build()

	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set")
	}
	if ast.InsertSource.Kind != SelectQuery {
		t.Errorf("expected InsertSource.Kind = SelectQuery, got %v", ast.InsertSource.Kind)
	}
	if ast.InsertSource.FromTable.Name != "source" {
		t.Errorf("expected InsertSource.FromTable.Name = %q, got %q", "source", ast.InsertSource.FromTable.Name)
	}
	if len(ast.InsertRows) != 0 {
		t.Errorf("expected 0 InsertRows, got %d", len(ast.InsertRows))
	}
}

func TestInsertInto_FromSelect_WithReturning(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	idCol := Int64Column{Table: "target", Name: "id"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelect(From(source).Select(srcName)).
		Returning(idCol, nameCol).
		Build()

	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set")
	}
	if len(ast.Returning) != 2 {
		t.Fatalf("expected 2 Returning columns, got %d", len(ast.Returning))
	}
	if ast.Returning[0].ColumnName() != "id" {
		t.Errorf("expected Returning[0] = %q, got %q", "id", ast.Returning[0].ColumnName())
	}
	if ast.Returning[1].ColumnName() != "name" {
		t.Errorf("expected Returning[1] = %q, got %q", "name", ast.Returning[1].ColumnName())
	}
}

func TestInsertInto_FromSelect_WithCTEs(t *testing.T) {
	target := mockTable{name: "target"}
	nameCol := StringColumn{Table: "target", Name: "name"}

	cteQuery := &AST{
		Kind:      SelectQuery,
		FromTable: TableRef{Name: "source"},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "name"}}},
		},
	}

	cteTable := CTERef("filtered")
	cteName := StringColumn{Table: "filtered", Name: "name"}

	selectAST := From(cteTable).Select(cteName).Build()

	ast := InsertInto(target).
		Columns(nameCol).
		WithCTEs(CTE{Name: "filtered", Query: cteQuery}).
		FromSelectAST(selectAST).
		Build()

	if len(ast.CTEs) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(ast.CTEs))
	}
	if ast.CTEs[0].Name != "filtered" {
		t.Errorf("expected CTE name = %q, got %q", "filtered", ast.CTEs[0].Name)
	}
	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set")
	}
}

func TestInsertInto_FromSelect_WithWhere(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}
	srcActive := Int64Column{Table: "source", Name: "active"}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelect(
			From(source).
				Select(srcName).
				Where(srcActive.Eq(Literal(1))),
		).
		Build()

	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set")
	}
	if ast.InsertSource.Where == nil {
		t.Fatal("expected InsertSource.Where to be set")
	}
	binExpr, ok := ast.InsertSource.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected Where to be BinaryExpr, got %T", ast.InsertSource.Where)
	}
	if binExpr.Op != OpEq {
		t.Errorf("expected Op = OpEq, got %v", binExpr.Op)
	}
}

func TestInsertInto_FromSelect_WithParams(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelect(
			From(source).
				Select(srcName).
				Where(srcName.Eq(Param[string]("filter_name"))),
		).
		Build()

	if ast.InsertSource == nil {
		t.Fatal("expected InsertSource to be set")
	}
	if ast.InsertSource.Where == nil {
		t.Fatal("expected InsertSource.Where to be set")
	}
	binExpr, ok := ast.InsertSource.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected Where to be BinaryExpr, got %T", ast.InsertSource.Where)
	}
	paramExpr, ok := binExpr.Right.(ParamExpr)
	if !ok {
		t.Fatalf("expected Right to be ParamExpr, got %T", binExpr.Right)
	}
	if paramExpr.Name != "filter_name" {
		t.Errorf("expected param name %q, got %q", "filter_name", paramExpr.Name)
	}
}

func TestInsertInto_AddRow_ClearsFromSelect(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelect(From(source).Select(srcName)).
		AddRow(Param[string]("name")).
		Build()

	if ast.InsertSource != nil {
		t.Errorf("expected InsertSource to be nil after AddRow, got %v", ast.InsertSource)
	}
	if len(ast.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows after AddRow, got %d", len(ast.InsertRows))
	}
}

func TestInsertInto_BulkRows_ClearsFromSelect(t *testing.T) {
	target := mockTable{name: "target"}
	source := mockTable{name: "source"}
	nameCol := StringColumn{Table: "target", Name: "name"}
	srcName := StringColumn{Table: "source", Name: "name"}

	rows := [][]Expr{
		{Param[string](fmt.Sprintf("name_%d", 0))},
		{Param[string](fmt.Sprintf("name_%d", 1))},
	}

	ast := InsertInto(target).
		Columns(nameCol).
		FromSelect(From(source).Select(srcName)).
		BulkRows(rows).
		Build()

	if ast.InsertSource != nil {
		t.Errorf("expected InsertSource to be nil after BulkRows, got %v", ast.InsertSource)
	}
	if len(ast.InsertRows) != 2 {
		t.Fatalf("expected 2 InsertRows after BulkRows, got %d", len(ast.InsertRows))
	}
}
