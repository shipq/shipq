package compile

import (
	"testing"

	"github.com/portsql/portsql/src/query"
)

func TestPostgres_SimpleSelect(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "authors", Name: "id"}}},
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "authors", Name: "name"}}},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `SELECT "authors"."id", "authors"."name" FROM "authors"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 0 {
		t.Errorf("expected no params, got %v", params)
	}
}

func TestPostgres_SelectStar(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `SELECT * FROM "authors"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestPostgres_SelectWithWhere(t *testing.T) {
	idCol := query.Int64Column{Table: "authors", Name: "id"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: idCol}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: idCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "id", GoType: "int64"},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `SELECT "authors"."id" FROM "authors" WHERE ("authors"."id" = $1)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 1 || params[0] != "id" {
		t.Errorf("expected params [id], got %v", params)
	}
}

func TestPostgres_SelectWithMultipleParams(t *testing.T) {
	nameCol := query.StringColumn{Table: "authors", Name: "name"}
	emailCol := query.StringColumn{Table: "authors", Name: "email"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: nameCol}},
		},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: nameCol},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "name", GoType: "string"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: emailCol},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "email", GoType: "string"},
			},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params[0] != "name" || params[1] != "email" {
		t.Errorf("expected params [name, email], got %v", params)
	}

	// Should contain $1 and $2
	if !containsStr(sql, "$1") || !containsStr(sql, "$2") {
		t.Errorf("SQL should contain $1 and $2: %s", sql)
	}
}

func TestPostgres_SelectWithJoin(t *testing.T) {
	authorID := query.Int64Column{Table: "authors", Name: "id"}
	bookAuthorID := query.Int64Column{Table: "books", Name: "author_id"}
	authorName := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: authorName}},
		},
		Joins: []query.JoinClause{
			{
				Type:  query.LeftJoin,
				Table: query.TableRef{Name: "books"},
				Condition: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: authorID},
					Op:    query.OpEq,
					Right: query.ColumnExpr{Column: bookAuthorID},
				},
			},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `LEFT JOIN "books" ON ("authors"."id" = "books"."author_id")`) {
		t.Errorf("SQL should contain LEFT JOIN clause: %s", sql)
	}
}

func TestPostgres_SelectWithInnerJoin(t *testing.T) {
	authorID := query.Int64Column{Table: "authors", Name: "id"}
	bookAuthorID := query.Int64Column{Table: "books", Name: "author_id"}
	authorName := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: authorName}},
		},
		Joins: []query.JoinClause{
			{
				Type:  query.InnerJoin,
				Table: query.TableRef{Name: "books"},
				Condition: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: authorID},
					Op:    query.OpEq,
					Right: query.ColumnExpr{Column: bookAuthorID},
				},
			},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `INNER JOIN "books"`) {
		t.Errorf("SQL should contain INNER JOIN: %s", sql)
	}
}

func TestPostgres_SelectWithJoinAlias(t *testing.T) {
	authorID := query.Int64Column{Table: "authors", Name: "id"}
	commentAuthorID := query.Int64Column{Table: "comment_authors", Name: "id"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: authorID}},
		},
		Joins: []query.JoinClause{
			{
				Type:  query.LeftJoin,
				Table: query.TableRef{Name: "authors", Alias: "comment_authors"},
				Condition: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: authorID},
					Op:    query.OpEq,
					Right: query.ColumnExpr{Column: commentAuthorID},
				},
			},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `LEFT JOIN "authors" AS "comment_authors"`) {
		t.Errorf("SQL should contain aliased JOIN: %s", sql)
	}
}

func TestPostgres_SelectWithOrderByLimitOffset(t *testing.T) {
	createdAt := query.TimeColumn{Table: "authors", Name: "created_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: createdAt}},
		},
		OrderBy: []query.OrderByExpr{
			{Expr: query.ColumnExpr{Column: createdAt}, Desc: true},
		},
		Limit:  query.ParamExpr{Name: "limit", GoType: "int"},
		Offset: query.ParamExpr{Name: "offset", GoType: "int"},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `ORDER BY "authors"."created_at" DESC`) {
		t.Errorf("SQL should contain ORDER BY DESC: %s", sql)
	}
	if !containsStr(sql, `LIMIT $1`) {
		t.Errorf("SQL should contain LIMIT $1: %s", sql)
	}
	if !containsStr(sql, `OFFSET $2`) {
		t.Errorf("SQL should contain OFFSET $2: %s", sql)
	}
	if len(params) != 2 || params[0] != "limit" || params[1] != "offset" {
		t.Errorf("expected params [limit, offset], got %v", params)
	}
}

func TestPostgres_SelectWithGroupBy(t *testing.T) {
	countryCol := query.StringColumn{Table: "authors", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
		GroupBy: []query.Column{countryCol},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `GROUP BY "authors"."country"`) {
		t.Errorf("SQL should contain GROUP BY: %s", sql)
	}
}

func TestPostgres_SelectWithAlias(t *testing.T) {
	nameCol := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: nameCol}, Alias: "author_name"},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `AS "author_name"`) {
		t.Errorf("SQL should contain AS alias: %s", sql)
	}
}

func TestPostgres_Insert(t *testing.T) {
	publicID := query.StringColumn{Table: "authors", Name: "public_id"}
	name := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "authors"},
		InsertCols: []query.Column{publicID, name},
		InsertVals: []query.Expr{
			query.ParamExpr{Name: "public_id", GoType: "string"},
			query.ParamExpr{Name: "name", GoType: "string"},
		},
		Returning: []query.Column{publicID},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `INSERT INTO "authors" ("public_id", "name") VALUES ($1, $2) RETURNING "public_id"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "public_id" || params[1] != "name" {
		t.Errorf("expected params [public_id, name], got %v", params)
	}
}

func TestPostgres_InsertWithNow(t *testing.T) {
	name := query.StringColumn{Table: "authors", Name: "name"}
	createdAt := query.TimeColumn{Table: "authors", Name: "created_at"}

	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "authors"},
		InsertCols: []query.Column{name, createdAt},
		InsertVals: []query.Expr{
			query.ParamExpr{Name: "name", GoType: "string"},
			query.FuncExpr{Name: "NOW"},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NOW()") {
		t.Errorf("SQL should contain NOW(): %s", sql)
	}
}

func TestPostgres_Update(t *testing.T) {
	name := query.StringColumn{Table: "authors", Name: "name"}
	publicID := query.StringColumn{Table: "authors", Name: "public_id"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "authors"},
		SetClauses: []query.SetClause{
			{Column: name, Value: query.ParamExpr{Name: "name", GoType: "string"}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: publicID},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "public_id", GoType: "string"},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `UPDATE "authors" SET "name" = $1 WHERE ("authors"."public_id" = $2)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "name" || params[1] != "public_id" {
		t.Errorf("expected params [name, public_id], got %v", params)
	}
}

func TestPostgres_UpdateMultipleSets(t *testing.T) {
	name := query.StringColumn{Table: "authors", Name: "name"}
	email := query.StringColumn{Table: "authors", Name: "email"}
	publicID := query.StringColumn{Table: "authors", Name: "public_id"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "authors"},
		SetClauses: []query.SetClause{
			{Column: name, Value: query.ParamExpr{Name: "name", GoType: "string"}},
			{Column: email, Value: query.ParamExpr{Name: "email", GoType: "string"}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: publicID},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "public_id", GoType: "string"},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"name" = $1`) || !containsStr(sql, `"email" = $2`) {
		t.Errorf("SQL should contain multiple SET clauses: %s", sql)
	}
	if len(params) != 3 {
		t.Errorf("expected 3 params, got %v", params)
	}
}

func TestPostgres_Delete(t *testing.T) {
	publicID := query.StringColumn{Table: "authors", Name: "public_id"}

	ast := &query.AST{
		Kind:      query.DeleteQuery,
		FromTable: query.TableRef{Name: "authors"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: publicID},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "public_id", GoType: "string"},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `DELETE FROM "authors" WHERE ("authors"."public_id" = $1)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 1 || params[0] != "public_id" {
		t.Errorf("expected params [public_id], got %v", params)
	}
}

func TestPostgres_BooleanLiterals(t *testing.T) {
	active := query.BoolColumn{Table: "users", Name: "active"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: active},
			Op:    query.OpEq,
			Right: query.LiteralExpr{Value: true},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "TRUE") {
		t.Errorf("SQL should contain TRUE: %s", sql)
	}
}

func TestPostgres_BooleanFalse(t *testing.T) {
	active := query.BoolColumn{Table: "users", Name: "active"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: active},
			Op:    query.OpEq,
			Right: query.LiteralExpr{Value: false},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "FALSE") {
		t.Errorf("SQL should contain FALSE: %s", sql)
	}
}

func TestPostgres_NullLiteral(t *testing.T) {
	bio := query.NullStringColumn{Table: "users", Name: "bio"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "users"},
		SetClauses: []query.SetClause{
			{Column: bio, Value: query.LiteralExpr{Value: nil}},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NULL") {
		t.Errorf("SQL should contain NULL: %s", sql)
	}
}

func TestPostgres_InClause(t *testing.T) {
	status := query.StringColumn{Table: "orders", Name: "status"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		Where: query.BinaryExpr{
			Left: query.ColumnExpr{Column: status},
			Op:   query.OpIn,
			Right: query.ListExpr{
				Values: []query.Expr{
					query.LiteralExpr{Value: "pending"},
					query.LiteralExpr{Value: "processing"},
				},
			},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `IN ('pending', 'processing')`) {
		t.Errorf("SQL should contain IN clause: %s", sql)
	}
}

func TestPostgres_InClauseWithParams(t *testing.T) {
	id := query.Int64Column{Table: "orders", Name: "id"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		Where: query.BinaryExpr{
			Left: query.ColumnExpr{Column: id},
			Op:   query.OpIn,
			Right: query.ListExpr{
				Values: []query.Expr{
					query.ParamExpr{Name: "id1", GoType: "int64"},
					query.ParamExpr{Name: "id2", GoType: "int64"},
				},
			},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `IN ($1, $2)`) {
		t.Errorf("SQL should contain IN clause with params: %s", sql)
	}
	if len(params) != 2 {
		t.Errorf("expected 2 params, got %v", params)
	}
}

func TestPostgres_IsNull(t *testing.T) {
	deletedAt := query.NullTimeColumn{Table: "users", Name: "deleted_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where:     query.UnaryExpr{Op: query.OpIsNull, Expr: query.ColumnExpr{Column: deletedAt}},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"users"."deleted_at" IS NULL`) {
		t.Errorf("SQL should contain IS NULL: %s", sql)
	}
}

func TestPostgres_IsNotNull(t *testing.T) {
	deletedAt := query.NullTimeColumn{Table: "users", Name: "deleted_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where:     query.UnaryExpr{Op: query.OpNotNull, Expr: query.ColumnExpr{Column: deletedAt}},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"users"."deleted_at" IS NOT NULL`) {
		t.Errorf("SQL should contain IS NOT NULL: %s", sql)
	}
}

func TestPostgres_ILike(t *testing.T) {
	name := query.StringColumn{Table: "users", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where: query.FuncExpr{
			Name: "ILIKE",
			Args: []query.Expr{
				query.ColumnExpr{Column: name},
				query.LiteralExpr{Value: "%john%"},
			},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Postgres has native ILIKE
	if !containsStr(sql, `ILIKE '%john%'`) {
		t.Errorf("SQL should contain ILIKE: %s", sql)
	}
}

func TestPostgres_Like(t *testing.T) {
	name := query.StringColumn{Table: "users", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: name},
			Op:    query.OpLike,
			Right: query.LiteralExpr{Value: "%john%"},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `LIKE '%john%'`) {
		t.Errorf("SQL should contain LIKE: %s", sql)
	}
}

func TestPostgres_JSONAgg(t *testing.T) {
	bookID := query.Int64Column{Table: "books", Name: "id"}
	bookTitle := query.StringColumn{Table: "books", Name: "title"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{
				Expr: query.JSONAggExpr{
					FieldName: "books",
					Columns:   []query.Column{bookID, bookTitle},
				},
				Alias: "books",
			},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "COALESCE(JSON_AGG(JSON_BUILD_OBJECT(") {
		t.Errorf("SQL should contain COALESCE(JSON_AGG(JSON_BUILD_OBJECT(): %s", sql)
	}
	if !containsStr(sql, "'id'") {
		t.Errorf("SQL should contain 'id' key: %s", sql)
	}
	if !containsStr(sql, "'title'") {
		t.Errorf("SQL should contain 'title' key: %s", sql)
	}
	if !containsStr(sql, "FILTER (WHERE") {
		t.Errorf("SQL should contain FILTER (WHERE: %s", sql)
	}
	if !containsStr(sql, "'[]')") {
		t.Errorf("SQL should contain empty array fallback: %s", sql)
	}
}

func TestPostgres_StringEscaping(t *testing.T) {
	name := query.StringColumn{Table: "users", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: name},
			Op:    query.OpEq,
			Right: query.LiteralExpr{Value: "O'Brien"},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Single quotes should be escaped by doubling
	if !containsStr(sql, "'O''Brien'") {
		t.Errorf("SQL should escape single quotes: %s", sql)
	}
}

func TestPostgres_ComparisonOperators(t *testing.T) {
	age := query.Int64Column{Table: "users", Name: "age"}

	tests := []struct {
		op       query.BinaryOp
		expected string
	}{
		{query.OpEq, "="},
		{query.OpNe, "<>"},
		{query.OpLt, "<"},
		{query.OpLe, "<="},
		{query.OpGt, ">"},
		{query.OpGe, ">="},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			ast := &query.AST{
				Kind:      query.SelectQuery,
				FromTable: query.TableRef{Name: "users"},
				Where: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: age},
					Op:    tt.op,
					Right: query.LiteralExpr{Value: 18},
				},
			}

			sql, _, err := NewCompiler(Postgres).Compile(ast)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			if !containsStr(sql, tt.expected) {
				t.Errorf("SQL should contain %s: %s", tt.expected, sql)
			}
		})
	}
}

// Helper function
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Phase 7: Advanced SQL Features Tests
// =============================================================================

func TestPostgres_CountStar(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestPostgres_CountColumn(t *testing.T) {
	emailCol := query.StringColumn{Table: "users", Name: "email"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: query.ColumnExpr{Column: emailCol}}},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `COUNT("users"."email")`) {
		t.Errorf("SQL should contain COUNT column: %s", sql)
	}
}

func TestPostgres_CountDistinct(t *testing.T) {
	emailCol := query.StringColumn{Table: "users", Name: "email"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: query.ColumnExpr{Column: emailCol}, Distinct: true}},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "COUNT(DISTINCT") {
		t.Errorf("SQL should contain COUNT(DISTINCT: %s", sql)
	}
}

func TestPostgres_SumAvgMinMax(t *testing.T) {
	amountCol := query.Int64Column{Table: "orders", Name: "amount"}

	tests := []struct {
		name     string
		agg      query.AggregateFunc
		expected string
	}{
		{"SUM", query.AggSum, "SUM("},
		{"AVG", query.AggAvg, "AVG("},
		{"MIN", query.AggMin, "MIN("},
		{"MAX", query.AggMax, "MAX("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast := &query.AST{
				Kind:      query.SelectQuery,
				FromTable: query.TableRef{Name: "orders"},
				SelectCols: []query.SelectExpr{
					{Expr: query.AggregateExpr{Func: tt.agg, Arg: query.ColumnExpr{Column: amountCol}}},
				},
			}

			sql, _, err := NewCompiler(Postgres).Compile(ast)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			if !containsStr(sql, tt.expected) {
				t.Errorf("SQL should contain %s: %s", tt.expected, sql)
			}
		})
	}
}

func TestPostgres_SelectDistinct(t *testing.T) {
	countryCol := query.StringColumn{Table: "users", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		Distinct:  true,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "SELECT DISTINCT") {
		t.Errorf("SQL should contain SELECT DISTINCT: %s", sql)
	}
}

func TestPostgres_Subquery(t *testing.T) {
	// SELECT * FROM orders WHERE customer_id IN (SELECT id FROM vip_customers)
	vipSubquery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "vip_customers"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "vip_customers", Name: "id"}}},
		},
	}

	customerID := query.Int64Column{Table: "orders", Name: "customer_id"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: customerID},
			Op:    query.OpIn,
			Right: query.SubqueryExpr{Query: vipSubquery},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "IN (SELECT") {
		t.Errorf("SQL should contain IN (SELECT: %s", sql)
	}
	if !containsStr(sql, `"vip_customers"`) {
		t.Errorf("SQL should contain vip_customers: %s", sql)
	}
}

func TestPostgres_Exists(t *testing.T) {
	subquery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.LiteralExpr{Value: 1}},
		},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "customers"},
		Where:     query.ExistsExpr{Subquery: subquery, Negated: false},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "EXISTS (SELECT") {
		t.Errorf("SQL should contain EXISTS (SELECT: %s", sql)
	}
}

func TestPostgres_NotExists(t *testing.T) {
	subquery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.LiteralExpr{Value: 1}},
		},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "customers"},
		Where:     query.ExistsExpr{Subquery: subquery, Negated: true},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NOT EXISTS (SELECT") {
		t.Errorf("SQL should contain NOT EXISTS (SELECT: %s", sql)
	}
}

func TestPostgres_Union(t *testing.T) {
	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "active_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "active_users", Name: "email"}}},
		},
	}
	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "archived_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "archived_users", Name: "email"}}},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpUnion,
			Right: right,
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "UNION") {
		t.Errorf("SQL should contain UNION: %s", sql)
	}
	if !containsStr(sql, `"active_users"`) {
		t.Errorf("SQL should contain active_users: %s", sql)
	}
	if !containsStr(sql, `"archived_users"`) {
		t.Errorf("SQL should contain archived_users: %s", sql)
	}
}

func TestPostgres_UnionAll(t *testing.T) {
	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table1"},
	}
	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table2"},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpUnionAll,
			Right: right,
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "UNION ALL") {
		t.Errorf("SQL should contain UNION ALL: %s", sql)
	}
}

func TestPostgres_Intersect(t *testing.T) {
	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table1"},
	}
	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table2"},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpIntersect,
			Right: right,
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "INTERSECT") {
		t.Errorf("SQL should contain INTERSECT: %s", sql)
	}
}

func TestPostgres_Except(t *testing.T) {
	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table1"},
	}
	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table2"},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpExcept,
			Right: right,
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "EXCEPT") {
		t.Errorf("SQL should contain EXCEPT: %s", sql)
	}
}

func TestPostgres_CTE(t *testing.T) {
	cteQuery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "orders", Name: "id"}}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: query.StringColumn{Table: "orders", Name: "status"}},
			Op:    query.OpEq,
			Right: query.LiteralExpr{Value: "pending"},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		CTEs: []query.CTE{
			{Name: "pending_orders", Query: cteQuery},
		},
		FromTable: query.TableRef{Name: "pending_orders"},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "WITH") {
		t.Errorf("SQL should contain WITH: %s", sql)
	}
	if !containsStr(sql, `"pending_orders"`) {
		t.Errorf("SQL should contain pending_orders: %s", sql)
	}
	if !containsStr(sql, "AS (SELECT") {
		t.Errorf("SQL should contain AS (SELECT: %s", sql)
	}
}

func TestPostgres_CTEWithColumns(t *testing.T) {
	cteQuery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "orders", Name: "id"}}},
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "orders", Name: "amount"}}},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		CTEs: []query.CTE{
			{Name: "order_summary", Columns: []string{"order_id", "total"}, Query: cteQuery},
		},
		FromTable: query.TableRef{Name: "order_summary"},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"order_id"`) {
		t.Errorf("SQL should contain order_id column: %s", sql)
	}
	if !containsStr(sql, `"total"`) {
		t.Errorf("SQL should contain total column: %s", sql)
	}
}

func TestPostgres_MultipleCTEs(t *testing.T) {
	cte1 := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
	}
	cte2 := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "customers"},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		CTEs: []query.CTE{
			{Name: "recent_orders", Query: cte1},
			{Name: "vip_customers", Query: cte2},
		},
		FromTable: query.TableRef{Name: "recent_orders"},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"recent_orders"`) {
		t.Errorf("SQL should contain recent_orders: %s", sql)
	}
	if !containsStr(sql, `"vip_customers"`) {
		t.Errorf("SQL should contain vip_customers: %s", sql)
	}
}

func TestPostgres_SetOpWithOrderByLimit(t *testing.T) {
	emailCol := query.StringColumn{Table: "t", Name: "email"}

	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table1"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: emailCol}},
		},
	}
	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "table2"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: emailCol}},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpUnion,
			Right: right,
		},
		OrderBy: []query.OrderByExpr{
			{Expr: query.ColumnExpr{Column: emailCol}, Desc: true},
		},
		Limit:  query.LiteralExpr{Value: 10},
		Offset: query.LiteralExpr{Value: 5},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "ORDER BY") {
		t.Errorf("SQL should contain ORDER BY: %s", sql)
	}
	if !containsStr(sql, "DESC") {
		t.Errorf("SQL should contain DESC: %s", sql)
	}
	if !containsStr(sql, "LIMIT 10") {
		t.Errorf("SQL should contain LIMIT 10: %s", sql)
	}
	if !containsStr(sql, "OFFSET 5") {
		t.Errorf("SQL should contain OFFSET 5: %s", sql)
	}
}

// =============================================================================
// Nested Parameter Numbering Tests (Postgres-specific)
// These tests verify that parameters are correctly numbered across nested
// compilation boundaries (subqueries, CTEs, set operations).
// =============================================================================

func TestPostgres_SubqueryWithOuterAndInnerParams(t *testing.T) {
	// SELECT * FROM orders WHERE status = $1 AND customer_id IN (SELECT id FROM customers WHERE tier = $2)
	// The outer query has a param, and the subquery also has a param.
	// They should be numbered $1 and $2 respectively.

	statusCol := query.StringColumn{Table: "orders", Name: "status"}
	customerIDCol := query.Int64Column{Table: "orders", Name: "customer_id"}
	tierCol := query.StringColumn{Table: "customers", Name: "tier"}
	custIDCol := query.Int64Column{Table: "customers", Name: "id"}

	subquery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "customers"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: custIDCol}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: tierCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "tier", GoType: "string"},
		},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: statusCol},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "status", GoType: "string"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: customerIDCol},
				Op:    query.OpIn,
				Right: query.SubqueryExpr{Query: subquery},
			},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check param order
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %v", len(params), params)
	}
	if params[0] != "status" {
		t.Errorf("expected first param to be 'status', got %q", params[0])
	}
	if params[1] != "tier" {
		t.Errorf("expected second param to be 'tier', got %q", params[1])
	}

	// Check that $1 appears for outer query and $2 for subquery
	if !containsStr(sql, "$1") {
		t.Errorf("SQL should contain $1: %s", sql)
	}
	if !containsStr(sql, "$2") {
		t.Errorf("SQL should contain $2: %s", sql)
	}
	// The subquery param should be $2, not $1
	// We can verify by checking the structure of the SQL
	// The outer WHERE should have $1, and the inner WHERE should have $2
	if containsStr(sql, "tier = $1") {
		t.Errorf("subquery param should be $2, not $1: %s", sql)
	}
}

func TestPostgres_CTEWithParams(t *testing.T) {
	// WITH filtered AS (SELECT * FROM orders WHERE status = $1)
	// SELECT * FROM filtered WHERE amount > $2
	// CTE has a param, main query has a param - they should be $1 and $2.

	statusCol := query.StringColumn{Table: "orders", Name: "status"}
	amountCol := query.Int64Column{Table: "filtered", Name: "amount"}
	orderIDCol := query.Int64Column{Table: "orders", Name: "id"}
	filteredIDCol := query.Int64Column{Table: "filtered", Name: "id"}

	cteQuery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: orderIDCol}},
			{Expr: query.ColumnExpr{Column: amountCol}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: statusCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "status", GoType: "string"},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		CTEs: []query.CTE{
			{Name: "filtered", Query: cteQuery},
		},
		FromTable: query.TableRef{Name: "filtered"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: filteredIDCol}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: amountCol},
			Op:    query.OpGt,
			Right: query.ParamExpr{Name: "min_amount", GoType: "int64"},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check param order
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %v", len(params), params)
	}
	if params[0] != "status" {
		t.Errorf("expected first param to be 'status', got %q", params[0])
	}
	if params[1] != "min_amount" {
		t.Errorf("expected second param to be 'min_amount', got %q", params[1])
	}

	// Check that both $1 and $2 appear
	if !containsStr(sql, "$1") {
		t.Errorf("SQL should contain $1: %s", sql)
	}
	if !containsStr(sql, "$2") {
		t.Errorf("SQL should contain $2: %s", sql)
	}
	// The main query param should be $2, not $1
	if containsStr(sql, "amount > $1") {
		t.Errorf("main query param should be $2, not $1: %s", sql)
	}
}

func TestPostgres_SetOpWithParams(t *testing.T) {
	// (SELECT * FROM t1 WHERE a = $1) UNION (SELECT * FROM t2 WHERE b = $2)
	// Left branch has a param, right branch has a param - they should be $1 and $2.

	aCol := query.StringColumn{Table: "t1", Name: "a"}
	bCol := query.StringColumn{Table: "t2", Name: "b"}

	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "t1"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: aCol}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: aCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "val_a", GoType: "string"},
		},
	}

	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "t2"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: bCol}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: bCol},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "val_b", GoType: "string"},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpUnion,
			Right: right,
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check param order
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %v", len(params), params)
	}
	if params[0] != "val_a" {
		t.Errorf("expected first param to be 'val_a', got %q", params[0])
	}
	if params[1] != "val_b" {
		t.Errorf("expected second param to be 'val_b', got %q", params[1])
	}

	// Check that both $1 and $2 appear
	if !containsStr(sql, "$1") {
		t.Errorf("SQL should contain $1: %s", sql)
	}
	if !containsStr(sql, "$2") {
		t.Errorf("SQL should contain $2: %s", sql)
	}
	// The right branch param should be $2, not $1
	if containsStr(sql, `"b" = $1`) {
		t.Errorf("right branch param should be $2, not $1: %s", sql)
	}
}

func TestPostgres_ExistsWithOuterAndInnerParams(t *testing.T) {
	// SELECT * FROM customers WHERE tier = $1 AND EXISTS (SELECT 1 FROM orders WHERE amount > $2)
	// Outer query has a param, EXISTS subquery has a param - they should be $1 and $2.

	tierCol := query.StringColumn{Table: "customers", Name: "tier"}
	amountCol := query.Int64Column{Table: "orders", Name: "amount"}

	subquery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.LiteralExpr{Value: 1}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: amountCol},
			Op:    query.OpGt,
			Right: query.ParamExpr{Name: "min_amount", GoType: "int64"},
		},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "customers"},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: tierCol},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "tier", GoType: "string"},
			},
			Op:    query.OpAnd,
			Right: query.ExistsExpr{Subquery: subquery, Negated: false},
		},
	}

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check param order
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %v", len(params), params)
	}
	if params[0] != "tier" {
		t.Errorf("expected first param to be 'tier', got %q", params[0])
	}
	if params[1] != "min_amount" {
		t.Errorf("expected second param to be 'min_amount', got %q", params[1])
	}

	// Check that both $1 and $2 appear
	if !containsStr(sql, "$1") {
		t.Errorf("SQL should contain $1: %s", sql)
	}
	if !containsStr(sql, "$2") {
		t.Errorf("SQL should contain $2: %s", sql)
	}
	// The subquery param should be $2, not $1
	if containsStr(sql, "amount > $1") {
		t.Errorf("EXISTS subquery param should be $2, not $1: %s", sql)
	}
}
