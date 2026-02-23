package query

import (
	"testing"
)

// mockTable implements Table for testing
type mockTable struct {
	name string
}

func (m mockTable) TableName() string { return m.name }

func TestFrom_SimpleSelect(t *testing.T) {
	authors := mockTable{name: "authors"}
	idCol := Int64Column{Table: "authors", Name: "id"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := From(authors).
		Select(idCol, nameCol).
		Build()

	if ast.Kind != SelectQuery {
		t.Errorf("expected Kind = SelectQuery, got %v", ast.Kind)
	}
	if ast.FromTable.Name != "authors" {
		t.Errorf("expected FromTable.Name = %q, got %q", "authors", ast.FromTable.Name)
	}
	if len(ast.SelectCols) != 2 {
		t.Errorf("expected 2 SelectCols, got %d", len(ast.SelectCols))
	}
}

func TestFrom_WithWhere(t *testing.T) {
	authors := mockTable{name: "authors"}
	idCol := Int64Column{Table: "authors", Name: "id"}

	ast := From(authors).
		Select(idCol).
		Where(idCol.Eq(Param[int64]("id"))).
		Build()

	if ast.Where == nil {
		t.Fatal("expected Where to be set")
	}

	binExpr, ok := ast.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected Where to be BinaryExpr, got %T", ast.Where)
	}
	if binExpr.Op != OpEq {
		t.Errorf("expected Op = OpEq, got %v", binExpr.Op)
	}
}

func TestFrom_WithJoin(t *testing.T) {
	authors := mockTable{name: "authors"}
	books := mockTable{name: "books"}
	authorID := Int64Column{Table: "authors", Name: "id"}
	bookAuthorID := Int64Column{Table: "books", Name: "author_id"}

	ast := From(authors).
		LeftJoin(books).On(authorID.Eq(bookAuthorID)).
		Select(authorID).
		Build()

	if len(ast.Joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(ast.Joins))
	}
	if ast.Joins[0].Type != LeftJoin {
		t.Errorf("expected LeftJoin, got %v", ast.Joins[0].Type)
	}
	if ast.Joins[0].Table.Name != "books" {
		t.Errorf("expected join table = %q, got %q", "books", ast.Joins[0].Table.Name)
	}
}

func TestFrom_WithInnerJoin(t *testing.T) {
	authors := mockTable{name: "authors"}
	books := mockTable{name: "books"}
	authorID := Int64Column{Table: "authors", Name: "id"}
	bookAuthorID := Int64Column{Table: "books", Name: "author_id"}

	ast := From(authors).
		Join(books).On(authorID.Eq(bookAuthorID)).
		Select(authorID).
		Build()

	if len(ast.Joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(ast.Joins))
	}
	if ast.Joins[0].Type != InnerJoin {
		t.Errorf("expected InnerJoin, got %v", ast.Joins[0].Type)
	}
}

func TestFrom_WithOrderByAndLimit(t *testing.T) {
	authors := mockTable{name: "authors"}
	createdAt := TimeColumn{Table: "authors", Name: "created_at"}

	ast := From(authors).
		Select(createdAt).
		OrderBy(createdAt.Desc()).
		Limit(Param[int]("limit")).
		Offset(Param[int]("offset")).
		Build()

	if len(ast.OrderBy) != 1 {
		t.Fatalf("expected 1 OrderBy, got %d", len(ast.OrderBy))
	}
	if !ast.OrderBy[0].Desc {
		t.Error("expected Desc = true")
	}
	if ast.Limit == nil {
		t.Error("expected Limit to be set")
	}
	if ast.Offset == nil {
		t.Error("expected Offset to be set")
	}
}

func TestFrom_WithGroupBy(t *testing.T) {
	authors := mockTable{name: "authors"}
	countryCol := StringColumn{Table: "authors", Name: "country"}

	ast := From(authors).
		Select(countryCol).
		GroupBy(countryCol).
		Build()

	if len(ast.GroupBy) != 1 {
		t.Fatalf("expected 1 GroupBy, got %d", len(ast.GroupBy))
	}
	if ast.GroupBy[0].ColumnName() != "country" {
		t.Errorf("expected GroupBy column = %q, got %q", "country", ast.GroupBy[0].ColumnName())
	}
}

func TestFrom_WithHaving(t *testing.T) {
	authors := mockTable{name: "authors"}
	countryCol := StringColumn{Table: "authors", Name: "country"}

	ast := From(authors).
		Select(countryCol).
		GroupBy(countryCol).
		Having(countryCol.Eq(Literal("USA"))).
		Build()

	if ast.Having == nil {
		t.Fatal("expected Having to be set")
	}
}

func TestFrom_SelectAs(t *testing.T) {
	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := From(authors).
		SelectAs(nameCol, "author_name").
		Build()

	if len(ast.SelectCols) != 1 {
		t.Fatalf("expected 1 SelectCol, got %d", len(ast.SelectCols))
	}
	if ast.SelectCols[0].Alias != "author_name" {
		t.Errorf("expected alias = %q, got %q", "author_name", ast.SelectCols[0].Alias)
	}
}

func TestFrom_SelectJSONAgg(t *testing.T) {
	authors := mockTable{name: "authors"}
	books := mockTable{name: "books"}
	authorID := Int64Column{Table: "authors", Name: "id"}
	bookID := Int64Column{Table: "books", Name: "id"}
	bookTitle := StringColumn{Table: "books", Name: "title"}
	bookAuthorID := Int64Column{Table: "books", Name: "author_id"}

	ast := From(authors).
		LeftJoin(books).On(authorID.Eq(bookAuthorID)).
		Select(authorID).
		SelectJSONAgg("books", bookID, bookTitle).
		GroupBy(authorID).
		Build()

	if len(ast.SelectCols) != 2 {
		t.Fatalf("expected 2 SelectCols, got %d", len(ast.SelectCols))
	}

	// Second select should be JSON agg
	jsonAgg, ok := ast.SelectCols[1].Expr.(JSONAggExpr)
	if !ok {
		t.Fatalf("expected JSONAggExpr, got %T", ast.SelectCols[1].Expr)
	}
	if jsonAgg.FieldName != "books" {
		t.Errorf("expected FieldName = %q, got %q", "books", jsonAgg.FieldName)
	}
	if len(jsonAgg.Columns) != 2 {
		t.Errorf("expected 2 columns in JSONAgg, got %d", len(jsonAgg.Columns))
	}
}

func TestFrom_MultipleJoins(t *testing.T) {
	authors := mockTable{name: "authors"}
	books := mockTable{name: "books"}
	reviews := mockTable{name: "reviews"}

	authorID := Int64Column{Table: "authors", Name: "id"}
	bookAuthorID := Int64Column{Table: "books", Name: "author_id"}
	bookID := Int64Column{Table: "books", Name: "id"}
	reviewBookID := Int64Column{Table: "reviews", Name: "book_id"}

	ast := From(authors).
		LeftJoin(books).On(authorID.Eq(bookAuthorID)).
		LeftJoin(reviews).On(bookID.Eq(reviewBookID)).
		Select(authorID).
		Build()

	if len(ast.Joins) != 2 {
		t.Fatalf("expected 2 joins, got %d", len(ast.Joins))
	}
	if ast.Joins[0].Table.Name != "books" {
		t.Errorf("expected first join table = %q, got %q", "books", ast.Joins[0].Table.Name)
	}
	if ast.Joins[1].Table.Name != "reviews" {
		t.Errorf("expected second join table = %q, got %q", "reviews", ast.Joins[1].Table.Name)
	}
}

func TestJoin_WithAlias(t *testing.T) {
	authors := mockTable{name: "authors"}
	authorID := Int64Column{Table: "authors", Name: "id"}
	commentAuthorID := Int64Column{Table: "comment_authors", Name: "id"}

	ast := From(authors).
		LeftJoin(authors).As("comment_authors").On(authorID.Eq(commentAuthorID)).
		Select(authorID).
		Build()

	if len(ast.Joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(ast.Joins))
	}
	if ast.Joins[0].Table.Alias != "comment_authors" {
		t.Errorf("expected alias = %q, got %q", "comment_authors", ast.Joins[0].Table.Alias)
	}
}
