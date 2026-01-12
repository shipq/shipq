package query

import (
	"testing"
)

func TestDefineQuery_RegistersQuery(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := DefineQuery("GetAuthorByName",
		From(authors).
			Select(nameCol).
			Where(nameCol.Eq(Param[string]("name"))).
			Build(),
	)

	if ast == nil {
		t.Fatal("DefineQuery returned nil")
	}
	if ast.Kind != SelectQuery {
		t.Errorf("expected Kind = SelectQuery, got %v", ast.Kind)
	}

	queries := GetRegisteredQueries()
	if len(queries) != 1 {
		t.Errorf("expected 1 registered query, got %d", len(queries))
	}
	if queries["GetAuthorByName"] != ast {
		t.Error("registered query does not match returned query")
	}
}

func TestDefineQuery_MultipleQueries(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	books := mockTable{name: "books"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	titleCol := StringColumn{Table: "books", Name: "title"}

	ast1 := DefineQuery("GetAuthor",
		From(authors).Select(nameCol).Build(),
	)
	ast2 := DefineQuery("GetBook",
		From(books).Select(titleCol).Build(),
	)
	ast3 := DefineQuery("UpdateAuthor",
		Update(authors).Set(nameCol, Param[string]("name")).Build(),
	)

	if QueryCount() != 3 {
		t.Errorf("expected 3 registered queries, got %d", QueryCount())
	}

	queries := GetRegisteredQueries()
	if queries["GetAuthor"] != ast1 {
		t.Error("GetAuthor not found or mismatched")
	}
	if queries["GetBook"] != ast2 {
		t.Error("GetBook not found or mismatched")
	}
	if queries["UpdateAuthor"] != ast3 {
		t.Error("UpdateAuthor not found or mismatched")
	}
}

func TestDefineQuery_AllQueryTypes(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	idCol := Int64Column{Table: "authors", Name: "id"}

	// SELECT
	selectAST := DefineQuery("SelectAuthor",
		From(authors).Select(nameCol).Build(),
	)
	if selectAST.Kind != SelectQuery {
		t.Errorf("expected SelectQuery, got %v", selectAST.Kind)
	}

	// INSERT
	insertAST := DefineQuery("InsertAuthor",
		InsertInto(authors).
			Columns(nameCol).
			Values(Param[string]("name")).
			Build(),
	)
	if insertAST.Kind != InsertQuery {
		t.Errorf("expected InsertQuery, got %v", insertAST.Kind)
	}

	// UPDATE
	updateAST := DefineQuery("UpdateAuthor",
		Update(authors).
			Set(nameCol, Param[string]("name")).
			Where(idCol.Eq(Param[int64]("id"))).
			Build(),
	)
	if updateAST.Kind != UpdateQuery {
		t.Errorf("expected UpdateQuery, got %v", updateAST.Kind)
	}

	// DELETE
	deleteAST := DefineQuery("DeleteAuthor",
		Delete(authors).
			Where(idCol.Eq(Param[int64]("id"))).
			Build(),
	)
	if deleteAST.Kind != DeleteQuery {
		t.Errorf("expected DeleteQuery, got %v", deleteAST.Kind)
	}

	if QueryCount() != 4 {
		t.Errorf("expected 4 registered queries, got %d", QueryCount())
	}
}

func TestDefineQuery_PanicsOnEmptyName(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty name")
		}
	}()

	authors := mockTable{name: "authors"}
	DefineQuery("", From(authors).Build())
}

func TestDefineQuery_PanicsOnNilAST(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil AST")
		}
	}()

	DefineQuery("SomeQuery", nil)
}

func TestDefineQuery_PanicsOnDuplicateName(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate name")
		}
	}()

	authors := mockTable{name: "authors"}
	DefineQuery("GetAuthor", From(authors).Build())
	DefineQuery("GetAuthor", From(authors).Build()) // Should panic
}

func TestGetRegisteredQueries_ReturnsCopy(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	DefineQuery("GetAuthor", From(authors).Build())

	queries1 := GetRegisteredQueries()
	queries2 := GetRegisteredQueries()

	// Modifying one shouldn't affect the other
	delete(queries1, "GetAuthor")

	if len(queries2) != 1 {
		t.Error("modifying returned map affected other calls")
	}
	if QueryCount() != 1 {
		t.Error("modifying returned map affected registry")
	}
}

func TestClearRegistry(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	DefineQuery("Query1", From(authors).Build())
	DefineQuery("Query2", From(authors).Build())

	if QueryCount() != 2 {
		t.Fatalf("expected 2 queries, got %d", QueryCount())
	}

	ClearRegistry()

	if QueryCount() != 0 {
		t.Errorf("expected 0 queries after clear, got %d", QueryCount())
	}

	queries := GetRegisteredQueries()
	if len(queries) != 0 {
		t.Errorf("expected empty map, got %d entries", len(queries))
	}
}

func TestDefineQuery_ReturnsOriginalAST(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	original := From(authors).Select(nameCol).Build()
	returned := DefineQuery("TestQuery", original)

	if returned != original {
		t.Error("DefineQuery should return the same AST pointer")
	}
}
