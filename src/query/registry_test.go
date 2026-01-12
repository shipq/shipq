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
	if queries["GetAuthorByName"].AST != ast {
		t.Error("registered query does not match returned query")
	}
	// DefineQuery defaults to ReturnMany for backward compatibility
	if queries["GetAuthorByName"].ReturnType != ReturnMany {
		t.Errorf("expected ReturnMany, got %v", queries["GetAuthorByName"].ReturnType)
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
	if queries["GetAuthor"].AST != ast1 {
		t.Error("GetAuthor not found or mismatched")
	}
	if queries["GetBook"].AST != ast2 {
		t.Error("GetBook not found or mismatched")
	}
	if queries["UpdateAuthor"].AST != ast3 {
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

// =============================================================================
// Tests for DefineOne, DefineMany, DefineExec
// =============================================================================

func TestDefineOne_RegistersWithCorrectReturnType(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	idCol := Int64Column{Table: "authors", Name: "id"}

	ast := DefineOne("GetAuthorById",
		From(authors).
			Select(nameCol).
			Where(idCol.Eq(Param[int64]("id"))).
			Build(),
	)

	if ast == nil {
		t.Fatal("DefineOne returned nil")
	}

	queries := GetRegisteredQueries()
	rq, ok := queries["GetAuthorById"]
	if !ok {
		t.Fatal("GetAuthorById not found in registry")
	}
	if rq.AST != ast {
		t.Error("registered AST does not match returned AST")
	}
	if rq.ReturnType != ReturnOne {
		t.Errorf("expected ReturnOne, got %v", rq.ReturnType)
	}
}

func TestDefineMany_RegistersWithCorrectReturnType(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}

	ast := DefineMany("ListAuthors",
		From(authors).
			Select(nameCol).
			Build(),
	)

	if ast == nil {
		t.Fatal("DefineMany returned nil")
	}

	queries := GetRegisteredQueries()
	rq, ok := queries["ListAuthors"]
	if !ok {
		t.Fatal("ListAuthors not found in registry")
	}
	if rq.AST != ast {
		t.Error("registered AST does not match returned AST")
	}
	if rq.ReturnType != ReturnMany {
		t.Errorf("expected ReturnMany, got %v", rq.ReturnType)
	}
}

func TestDefineExec_RegistersWithCorrectReturnType(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	idCol := Int64Column{Table: "authors", Name: "id"}

	ast := DefineExec("UpdateAuthorName",
		Update(authors).
			Set(nameCol, Param[string]("name")).
			Where(idCol.Eq(Param[int64]("id"))).
			Build(),
	)

	if ast == nil {
		t.Fatal("DefineExec returned nil")
	}

	queries := GetRegisteredQueries()
	rq, ok := queries["UpdateAuthorName"]
	if !ok {
		t.Fatal("UpdateAuthorName not found in registry")
	}
	if rq.AST != ast {
		t.Error("registered AST does not match returned AST")
	}
	if rq.ReturnType != ReturnExec {
		t.Errorf("expected ReturnExec, got %v", rq.ReturnType)
	}
}

func TestMixedQueryTypes(t *testing.T) {
	ClearRegistry()

	authors := mockTable{name: "authors"}
	nameCol := StringColumn{Table: "authors", Name: "name"}
	idCol := Int64Column{Table: "authors", Name: "id"}

	DefineOne("GetAuthor", From(authors).Select(nameCol).Where(idCol.Eq(Param[int64]("id"))).Build())
	DefineMany("ListAuthors", From(authors).Select(nameCol).Build())
	DefineExec("DeleteAuthor", Delete(authors).Where(idCol.Eq(Param[int64]("id"))).Build())

	if QueryCount() != 3 {
		t.Errorf("expected 3 queries, got %d", QueryCount())
	}

	queries := GetRegisteredQueries()

	if queries["GetAuthor"].ReturnType != ReturnOne {
		t.Errorf("GetAuthor: expected ReturnOne, got %v", queries["GetAuthor"].ReturnType)
	}
	if queries["ListAuthors"].ReturnType != ReturnMany {
		t.Errorf("ListAuthors: expected ReturnMany, got %v", queries["ListAuthors"].ReturnType)
	}
	if queries["DeleteAuthor"].ReturnType != ReturnExec {
		t.Errorf("DeleteAuthor: expected ReturnExec, got %v", queries["DeleteAuthor"].ReturnType)
	}
}

func TestDefineOne_PanicsOnDuplicateName(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate name")
		}
	}()

	authors := mockTable{name: "authors"}
	DefineOne("GetAuthor", From(authors).Build())
	DefineOne("GetAuthor", From(authors).Build()) // Should panic
}

func TestDefineMany_PanicsOnDuplicateName(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate name")
		}
	}()

	authors := mockTable{name: "authors"}
	DefineMany("ListAuthors", From(authors).Build())
	DefineMany("ListAuthors", From(authors).Build()) // Should panic
}

func TestDefineExec_PanicsOnDuplicateName(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate name")
		}
	}()

	authors := mockTable{name: "authors"}
	DefineExec("UpdateAuthor", Update(authors).Build())
	DefineExec("UpdateAuthor", Update(authors).Build()) // Should panic
}

func TestDifferentDefineTypes_SameNamePanics(t *testing.T) {
	ClearRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate name across different define types")
		}
	}()

	authors := mockTable{name: "authors"}
	DefineOne("SomeQuery", From(authors).Build())
	DefineMany("SomeQuery", From(authors).Build()) // Should panic even though different define type
}
