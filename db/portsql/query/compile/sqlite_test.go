package compile

import (
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
)

func TestSQLite_SimpleSelect(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "authors", Name: "id"}}},
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "authors", Name: "name"}}},
		},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite uses double quotes (like Postgres)
	expected := `SELECT "authors"."id", "authors"."name" FROM "authors"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 0 {
		t.Errorf("expected no params, got %v", params)
	}
}

func TestSQLite_SelectStar(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `SELECT * FROM "authors"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestSQLite_SelectWithWhere(t *testing.T) {
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

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite uses ? for params (like MySQL)
	expected := `SELECT "authors"."id" FROM "authors" WHERE ("authors"."id" = ?)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 1 || params[0] != "id" {
		t.Errorf("expected params [id], got %v", params)
	}
}

func TestSQLite_MultipleParams(t *testing.T) {
	col1 := query.Int64Column{Table: "t", Name: "a"}
	col2 := query.Int64Column{Table: "t", Name: "b"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "t"},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col1},
				Op:    query.OpGt,
				Right: query.ParamExpr{Name: "min", GoType: "int64"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: col2},
				Op:    query.OpLt,
				Right: query.ParamExpr{Name: "max", GoType: "int64"},
			},
		},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// All params should be ?
	if !containsStr(sql, "?) AND") {
		t.Errorf("SQL should contain '?) AND': %s", sql)
	}
	if !containsStr(sql, "< ?)") {
		t.Errorf("SQL should contain '< ?)': %s", sql)
	}
	// Param order preserved
	if len(params) != 2 || params[0] != "min" || params[1] != "max" {
		t.Errorf("expected params [min, max], got %v", params)
	}
}

func TestSQLite_SelectWithJoin(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `LEFT JOIN "books" ON ("authors"."id" = "books"."author_id")`) {
		t.Errorf("SQL should contain LEFT JOIN clause: %s", sql)
	}
}

func TestSQLite_SelectWithOrderByLimitOffset(t *testing.T) {
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

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `ORDER BY "authors"."created_at" DESC`) {
		t.Errorf("SQL should contain ORDER BY DESC: %s", sql)
	}
	if !containsStr(sql, "LIMIT ?") {
		t.Errorf("SQL should contain LIMIT ?: %s", sql)
	}
	if !containsStr(sql, "OFFSET ?") {
		t.Errorf("SQL should contain OFFSET ?: %s", sql)
	}
	if len(params) != 2 || params[0] != "limit" || params[1] != "offset" {
		t.Errorf("expected params [limit, offset], got %v", params)
	}
}

func TestSQLite_SelectWithGroupBy(t *testing.T) {
	countryCol := query.StringColumn{Table: "authors", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
		GroupBy: []query.Column{countryCol},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `GROUP BY "authors"."country"`) {
		t.Errorf("SQL should contain GROUP BY: %s", sql)
	}
}

// Regression: GROUP BY on string columns should NOT add COLLATE for SQLite
// because SQLite already uses binary collation by default, which matches the
// behavior we enforce on Postgres (COLLATE "C") and MySQL (COLLATE utf8mb4_bin).
func TestSQLite_GroupByStringNoCollation(t *testing.T) {
	nameCol := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: nameCol}},
		},
		GroupBy: []query.Column{nameCol},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite uses binary collation by default — no COLLATE needed
	if containsStr(sql, "COLLATE") {
		t.Errorf("SQLite GROUP BY on string column should NOT include COLLATE: %s", sql)
	}
}

func TestSQLite_GroupByIntNoCollation(t *testing.T) {
	idCol := query.Int64Column{Table: "authors", Name: "id"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: idCol}},
		},
		GroupBy: []query.Column{idCol},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if containsStr(sql, "COLLATE") {
		t.Errorf("SQLite GROUP BY on non-string column should NOT include COLLATE: %s", sql)
	}
}

func TestSQLite_GroupByNullableStringNoCollation(t *testing.T) {
	nickCol := query.NullStringColumn{Table: "authors", Name: "nickname"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: nickCol}},
		},
		GroupBy: []query.Column{nickCol},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if containsStr(sql, "COLLATE") {
		t.Errorf("SQLite GROUP BY on nullable string column should NOT include COLLATE: %s", sql)
	}
}

func TestSQLite_InsertWithReturning(t *testing.T) {
	publicID := query.StringColumn{Table: "authors", Name: "public_id"}
	name := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "authors"},
		InsertCols: []query.Column{publicID, name},
		InsertRows: [][]query.Expr{{
			query.ParamExpr{Name: "public_id", GoType: "string"},
			query.ParamExpr{Name: "name", GoType: "string"},
		}},
		Returning: []query.Column{publicID},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite supports RETURNING (3.35+)
	expected := `INSERT INTO "authors" ("public_id", "name") VALUES (?, ?) RETURNING "public_id"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "public_id" || params[1] != "name" {
		t.Errorf("expected params [public_id, name], got %v", params)
	}
}

func TestSQLite_InsertWithNow(t *testing.T) {
	name := query.StringColumn{Table: "authors", Name: "name"}
	createdAt := query.TimeColumn{Table: "authors", Name: "created_at"}

	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "authors"},
		InsertCols: []query.Column{name, createdAt},
		InsertRows: [][]query.Expr{{
			query.ParamExpr{Name: "name", GoType: "string"},
			query.FuncExpr{Name: "NOW"},
		}},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite uses strftime for ms-precision timestamps
	if !containsStr(sql, "strftime('%Y-%m-%dT%H:%M:%fZ','now')") {
		t.Errorf("SQL should contain strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ','now'): %s", sql)
	}
	if containsStr(sql, "NOW()") {
		t.Errorf("SQLite SQL should NOT contain NOW(): %s", sql)
	}
}

func TestSQLite_Update(t *testing.T) {
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

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `UPDATE "authors" SET "name" = ? WHERE ("authors"."public_id" = ?)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "name" || params[1] != "public_id" {
		t.Errorf("expected params [name, public_id], got %v", params)
	}
}

func TestSQLite_Delete(t *testing.T) {
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

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `DELETE FROM "authors" WHERE ("authors"."public_id" = ?)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 1 || params[0] != "public_id" {
		t.Errorf("expected params [public_id], got %v", params)
	}
}

func TestSQLite_BooleanLiterals(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite uses 1/0 for booleans
	if !containsStr(sql, "= 1)") {
		t.Errorf("SQL should contain '= 1)': %s", sql)
	}
	if containsStr(sql, "TRUE") {
		t.Errorf("SQLite SQL should NOT contain TRUE: %s", sql)
	}
}

func TestSQLite_BooleanFalse(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "= 0)") {
		t.Errorf("SQL should contain '= 0)': %s", sql)
	}
	if containsStr(sql, "FALSE") {
		t.Errorf("SQLite SQL should NOT contain FALSE: %s", sql)
	}
}

func TestSQLite_NullLiteral(t *testing.T) {
	bio := query.NullStringColumn{Table: "users", Name: "bio"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "users"},
		SetClauses: []query.SetClause{
			{Column: bio, Value: query.LiteralExpr{Value: nil}},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NULL") {
		t.Errorf("SQL should contain NULL: %s", sql)
	}
}

func TestSQLite_InClause(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "IN ('pending', 'processing')") {
		t.Errorf("SQL should contain IN clause: %s", sql)
	}
}

func TestSQLite_IsNull(t *testing.T) {
	deletedAt := query.NullTimeColumn{Table: "users", Name: "deleted_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where:     query.UnaryExpr{Op: query.OpIsNull, Expr: query.ColumnExpr{Column: deletedAt}},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"users"."deleted_at" IS NULL`) {
		t.Errorf("SQL should contain IS NULL: %s", sql)
	}
}

func TestSQLite_IsNotNull(t *testing.T) {
	deletedAt := query.NullTimeColumn{Table: "users", Name: "deleted_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where:     query.UnaryExpr{Op: query.OpNotNull, Expr: query.ColumnExpr{Column: deletedAt}},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, `"users"."deleted_at" IS NOT NULL`) {
		t.Errorf("SQL should contain IS NOT NULL: %s", sql)
	}
}

func TestSQLite_ILike(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite: ILIKE becomes LOWER() LIKE LOWER()
	if !containsStr(sql, `LOWER("users"."name") LIKE LOWER('%john%')`) {
		t.Errorf("SQL should contain LOWER() LIKE LOWER(): %s", sql)
	}
}

func TestSQLite_Like(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "LIKE '%john%'") {
		t.Errorf("SQL should contain LIKE: %s", sql)
	}
}

func TestSQLite_JSONAggregation(t *testing.T) {
	bookID := query.Int64Column{Table: "books", Name: "id"}
	bookTitle := query.StringColumn{Table: "books", Name: "title"}

	jsonAgg := query.JSONAggExpr{
		FieldName: "books",
		Columns:   []query.Column{bookID, bookTitle},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "books"},
		SelectCols: []query.SelectExpr{
			{Expr: jsonAgg, Alias: "books"},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// SQLite uses JSON_GROUP_ARRAY and JSON_OBJECT
	if !containsStr(sql, "JSON_GROUP_ARRAY") {
		t.Errorf("SQL should contain JSON_GROUP_ARRAY: %s", sql)
	}
	if !containsStr(sql, "JSON_OBJECT") {
		t.Errorf("SQL should contain JSON_OBJECT: %s", sql)
	}
	if !containsStr(sql, "'[]'") {
		t.Errorf("SQL should contain '[]' as empty fallback: %s", sql)
	}
	if containsStr(sql, "JSON_AGG") && !containsStr(sql, "JSON_GROUP_ARRAY") {
		t.Errorf("SQLite SQL should NOT contain Postgres JSON_AGG: %s", sql)
	}
	if containsStr(sql, "JSON_ARRAYAGG") {
		t.Errorf("SQLite SQL should NOT contain MySQL JSON_ARRAYAGG: %s", sql)
	}
}

func TestSQLite_StringEscaping(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Single quotes should be escaped by doubling
	if !containsStr(sql, "'O''Brien'") {
		t.Errorf("SQL should escape single quotes: %s", sql)
	}
}

func TestSQLite_ComparisonOperators(t *testing.T) {
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

			sql, _, err := NewCompiler(SQLite).Compile(ast)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			if !containsStr(sql, tt.expected) {
				t.Errorf("SQL should contain %s: %s", tt.expected, sql)
			}
		})
	}
}

// =============================================================================
// Phase 7: Advanced SQL Features Tests
// =============================================================================

func TestSQLite_CountStar(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestSQLite_CountDistinct(t *testing.T) {
	emailCol := query.StringColumn{Table: "users", Name: "email"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: query.ColumnExpr{Column: emailCol}, Distinct: true}},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "COUNT(DISTINCT") {
		t.Errorf("SQL should contain COUNT(DISTINCT: %s", sql)
	}
}

func TestSQLite_SumAvgMinMax(t *testing.T) {
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

			sql, _, err := NewCompiler(SQLite).Compile(ast)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			if !containsStr(sql, tt.expected) {
				t.Errorf("SQL should contain %s: %s", tt.expected, sql)
			}
		})
	}
}

func TestSQLite_SelectDistinct(t *testing.T) {
	countryCol := query.StringColumn{Table: "users", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		Distinct:  true,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "SELECT DISTINCT") {
		t.Errorf("SQL should contain SELECT DISTINCT: %s", sql)
	}
}

func TestSQLite_Subquery(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
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

func TestSQLite_Exists(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "EXISTS (SELECT") {
		t.Errorf("SQL should contain EXISTS (SELECT: %s", sql)
	}
}

func TestSQLite_NotExists(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NOT EXISTS (SELECT") {
		t.Errorf("SQL should contain NOT EXISTS (SELECT: %s", sql)
	}
}

func TestSQLite_Union(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "UNION") {
		t.Errorf("SQL should contain UNION: %s", sql)
	}
	if !containsStr(sql, `"active_users"`) {
		t.Errorf("SQL should contain active_users: %s", sql)
	}
}

func TestSQLite_Intersect(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "INTERSECT") {
		t.Errorf("SQL should contain INTERSECT: %s", sql)
	}
}

func TestSQLite_CTE(t *testing.T) {
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

	sql, _, err := NewCompiler(SQLite).Compile(ast)
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

func TestSQLite_UpdateWithCoalesce(t *testing.T) {
	startedAt := query.NullStringColumn{Table: "job_results", Name: "started_at"}
	publicID := query.StringColumn{Table: "job_results", Name: "public_id"}
	status := query.StringColumn{Table: "job_results", Name: "status"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "job_results"},
		SetClauses: []query.SetClause{
			{Column: status, Value: query.ParamExpr{Name: "status", GoType: "string"}},
			{Column: startedAt, Value: query.FuncExpr{
				Name: "COALESCE",
				Args: []query.Expr{
					query.ParamExpr{Name: "startedAt", GoType: "*string"},
					query.ColumnExpr{Column: startedAt},
				},
			}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: publicID},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "publicId", GoType: "string"},
		},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `UPDATE "job_results" SET "status" = ?, "started_at" = COALESCE(?, "job_results"."started_at") WHERE ("job_results"."public_id" = ?)`
	if !containsStr(sql, "COALESCE(?, ") {
		t.Errorf("SQL should contain COALESCE with param placeholder: %s", sql)
	}
	if !containsStr(sql, `"job_results"."started_at"`) {
		t.Errorf("SQL should contain column reference in COALESCE fallback: %s", sql)
	}
	// Check full SQL (status SET + COALESCE SET + WHERE)
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 3 {
		t.Fatalf("expected 3 params, got %d: %v", len(params), params)
	}
	if params[0] != "status" || params[1] != "startedAt" || params[2] != "publicId" {
		t.Errorf("expected params [status, startedAt, publicId], got %v", params)
	}
}

func TestSQLite_UpdateWithArithmetic(t *testing.T) {
	score := query.Int64Column{Table: "posts", Name: "score"}
	id := query.Int64Column{Table: "posts", Name: "id"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "posts"},
		SetClauses: []query.SetClause{
			{Column: score, Value: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: score},
				Op:    query.OpAdd,
				Right: query.ParamExpr{Name: "delta", GoType: "int"},
			}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: id},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "id", GoType: "int64"},
		},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `UPDATE "posts" SET "score" = ("posts"."score" + ?) WHERE ("posts"."id" = ?)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "delta" || params[1] != "id" {
		t.Errorf("expected params [delta, id], got %v", params)
	}
}

func TestSQLite_UpdateWithSubtraction(t *testing.T) {
	score := query.Int64Column{Table: "posts", Name: "score"}
	id := query.Int64Column{Table: "posts", Name: "id"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "posts"},
		SetClauses: []query.SetClause{
			{Column: score, Value: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: score},
				Op:    query.OpSub,
				Right: query.ParamExpr{Name: "delta", GoType: "int"},
			}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: id},
			Op:    query.OpEq,
			Right: query.ParamExpr{Name: "id", GoType: "int64"},
		},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `UPDATE "posts" SET "score" = ("posts"."score" - ?) WHERE ("posts"."id" = ?)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "delta" || params[1] != "id" {
		t.Errorf("expected params [delta, id], got %v", params)
	}
}

func TestSQLite_BulkInsert(t *testing.T) {
	publicID := query.StringColumn{Table: "authors", Name: "public_id"}
	name := query.StringColumn{Table: "authors", Name: "name"}
	email := query.StringColumn{Table: "authors", Name: "email"}

	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "authors"},
		InsertCols: []query.Column{name, email},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "name_0", GoType: "string"}, query.ParamExpr{Name: "email_0", GoType: "string"}},
			{query.ParamExpr{Name: "name_1", GoType: "string"}, query.ParamExpr{Name: "email_1", GoType: "string"}},
		},
		Returning: []query.Column{publicID},
	}

	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := `INSERT INTO "authors" ("name", "email") VALUES (?, ?), (?, ?) RETURNING "public_id"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 4 {
		t.Errorf("expected 4 params, got %d: %v", len(params), params)
	}
}

// =============================================================================
// INSERT ... SELECT Tests
// =============================================================================

func TestSQLite_InsertSelect(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
			query.StringColumn{Table: "target", Name: "email"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "email"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.BoolColumn{Table: "source", Name: "active"}},
				Op:    query.OpEq,
				Right: query.LiteralExpr{Value: true},
			},
		},
	}
	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	expected := `INSERT INTO "target" ("name", "email") SELECT "source"."name", "source"."email" FROM "source" WHERE ("source"."active" = 1)`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 0 {
		t.Errorf("expected no params, got %v", params)
	}
}

func TestSQLite_InsertSelect_WithReturning(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
			query.StringColumn{Table: "target", Name: "email"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "email"}}},
			},
		},
		Returning: []query.Column{query.Int64Column{Table: "target", Name: "id"}},
	}
	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	expected := `INSERT INTO "target" ("name", "email") SELECT "source"."name", "source"."email" FROM "source" RETURNING "id"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestSQLite_InsertSelect_WithCTE(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "archive"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "archive", Name: "name"},
			query.StringColumn{Table: "archive", Name: "email"},
		},
		CTEs: []query.CTE{
			{
				Name: "active_users",
				Query: &query.AST{
					Kind:      query.SelectQuery,
					FromTable: query.TableRef{Name: "users"},
					SelectCols: []query.SelectExpr{
						{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "users", Name: "name"}}},
						{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "users", Name: "email"}}},
					},
					Where: query.BinaryExpr{
						Left:  query.ColumnExpr{Column: query.BoolColumn{Table: "users", Name: "active"}},
						Op:    query.OpEq,
						Right: query.LiteralExpr{Value: true},
					},
				},
			},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "active_users"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "active_users", Name: "name"}}},
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "active_users", Name: "email"}}},
			},
		},
	}
	sql, params, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	expected := `WITH "active_users" AS (SELECT "users"."name", "users"."email" FROM "users" WHERE ("users"."active" = 1)) INSERT INTO "archive" ("name", "email") SELECT "active_users"."name", "active_users"."email" FROM "active_users"`
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 0 {
		t.Errorf("expected no params, got %v", params)
	}
}

// =============================================================================
// Regression tests for Bug 2: time.Time fields inside JSONAGG break on SQLite
// =============================================================================

// TestSQLite_JSONAgg_DateTimeColumn_UsesStrftime verifies that SQLite's
// WriteJSONAgg wraps time.Time columns with strftime to produce RFC3339.
// Regression test: SQLite JSON_OBJECT serializes datetime in SQLite's native
// format which breaks Go's json.Unmarshal into time.Time (expects RFC3339).
func TestSQLite_JSONAgg_DateTimeColumn_UsesStrftime(t *testing.T) {
	bookTitle := query.StringColumn{Table: "books", Name: "title"}
	createdAt := query.TimeColumn{Table: "books", Name: "created_at"}

	jsonAgg := query.JSONAggExpr{
		FieldName: "books",
		Columns:   []query.Column{bookTitle, createdAt},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "books"},
		SelectCols: []query.SelectExpr{
			{Expr: jsonAgg, Alias: "books"},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Must wrap created_at with strftime for RFC3339 output
	if !containsStr(sql, "strftime(") {
		t.Errorf("SQL should contain strftime for time column: %s", sql)
	}
	if !containsStr(sql, "'created_at', strftime(") {
		t.Errorf("SQL should wrap created_at value with strftime: %s", sql)
	}
	// Non-time column (title) should NOT be wrapped
	if containsStr(sql, "'title', strftime(") {
		t.Errorf("SQL should NOT wrap non-time column with strftime: %s", sql)
	}
	// Must use RFC3339-compatible format string
	if !containsStr(sql, "%Y-%m-%dT%H:%M:%fZ") {
		t.Errorf("strftime should use RFC3339-compatible format: %s", sql)
	}
}

// TestSQLite_JSONAgg_NullableDateTimeColumn_UsesStrftime verifies that nullable
// time columns (*time.Time) are also wrapped with strftime.
func TestSQLite_JSONAgg_NullableDateTimeColumn_UsesStrftime(t *testing.T) {
	bookTitle := query.StringColumn{Table: "books", Name: "title"}
	deletedAt := query.NullTimeColumn{Table: "books", Name: "deleted_at"}

	jsonAgg := query.JSONAggExpr{
		FieldName: "books",
		Columns:   []query.Column{bookTitle, deletedAt},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "books"},
		SelectCols: []query.SelectExpr{
			{Expr: jsonAgg, Alias: "books"},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	if !containsStr(sql, "'deleted_at', strftime(") {
		t.Errorf("SQL should wrap nullable time column with strftime: %s", sql)
	}
}

// TestSQLite_JSONAgg_NoTimeColumns_NoStrftime verifies that JSON_OBJECT without
// time columns does NOT emit strftime.
func TestSQLite_JSONAgg_NoTimeColumns_NoStrftime(t *testing.T) {
	bookID := query.Int64Column{Table: "books", Name: "id"}
	bookTitle := query.StringColumn{Table: "books", Name: "title"}

	jsonAgg := query.JSONAggExpr{
		FieldName: "books",
		Columns:   []query.Column{bookID, bookTitle},
	}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "books"},
		SelectCols: []query.SelectExpr{
			{Expr: jsonAgg, Alias: "books"},
		},
	}

	sql, _, err := NewCompiler(SQLite).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if containsStr(sql, "strftime") {
		t.Errorf("SQL should NOT contain strftime when no time columns: %s", sql)
	}
}
