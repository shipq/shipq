package compile

import (
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
)

func TestMySQL_SimpleSelect(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "authors", Name: "id"}}},
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "authors", Name: "name"}}},
		},
	}

	sql, params, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL uses backticks
	expected := "SELECT `authors`.`id`, `authors`.`name` FROM `authors`"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 0 {
		t.Errorf("expected no params, got %v", params)
	}
}

func TestMySQL_SelectStar(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "SELECT * FROM `authors`"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestMySQL_SelectWithWhere(t *testing.T) {
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

	sql, params, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL uses ? for params
	expected := "SELECT `authors`.`id` FROM `authors` WHERE (`authors`.`id` = ?)"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 1 || params[0] != "id" {
		t.Errorf("expected params [id], got %v", params)
	}
}

func TestMySQL_MultipleParams(t *testing.T) {
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

	sql, params, err := NewCompiler(MySQL).Compile(ast)
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

func TestMySQL_SelectWithJoin(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "LEFT JOIN `books` ON (`authors`.`id` = `books`.`author_id`)") {
		t.Errorf("SQL should contain LEFT JOIN clause: %s", sql)
	}
}

func TestMySQL_SelectWithOrderByLimitOffset(t *testing.T) {
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

	sql, params, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "ORDER BY `authors`.`created_at` DESC") {
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

func TestMySQL_SelectWithGroupBy(t *testing.T) {
	countryCol := query.StringColumn{Table: "authors", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
		GroupBy: []query.Column{countryCol},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "GROUP BY `authors`.`country`") {
		t.Errorf("SQL should contain GROUP BY: %s", sql)
	}
}

func TestMySQL_Insert_NoReturning(t *testing.T) {
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
		Returning: []query.Column{publicID}, // Should be IGNORED for MySQL
	}

	sql, params, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL: No RETURNING clause
	expected := "INSERT INTO `authors` (`public_id`, `name`) VALUES (?, ?)"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if containsStr(sql, "RETURNING") {
		t.Errorf("MySQL SQL should NOT contain RETURNING: %s", sql)
	}
	if len(params) != 2 || params[0] != "public_id" || params[1] != "name" {
		t.Errorf("expected params [public_id, name], got %v", params)
	}
}

func TestMySQL_InsertWithNow(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NOW()") {
		t.Errorf("SQL should contain NOW(): %s", sql)
	}
}

func TestMySQL_Update(t *testing.T) {
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

	sql, params, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "UPDATE `authors` SET `name` = ? WHERE (`authors`.`public_id` = ?)"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 2 || params[0] != "name" || params[1] != "public_id" {
		t.Errorf("expected params [name, public_id], got %v", params)
	}
}

func TestMySQL_Delete(t *testing.T) {
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

	sql, params, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "DELETE FROM `authors` WHERE (`authors`.`public_id` = ?)"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
	if len(params) != 1 || params[0] != "public_id" {
		t.Errorf("expected params [public_id], got %v", params)
	}
}

func TestMySQL_BooleanLiterals(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL uses 1/0 for booleans
	if !containsStr(sql, "= 1)") {
		t.Errorf("SQL should contain '= 1)': %s", sql)
	}
	if containsStr(sql, "TRUE") {
		t.Errorf("MySQL SQL should NOT contain TRUE: %s", sql)
	}
}

func TestMySQL_BooleanFalse(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "= 0)") {
		t.Errorf("SQL should contain '= 0)': %s", sql)
	}
	if containsStr(sql, "FALSE") {
		t.Errorf("MySQL SQL should NOT contain FALSE: %s", sql)
	}
}

func TestMySQL_NullLiteral(t *testing.T) {
	bio := query.NullStringColumn{Table: "users", Name: "bio"}

	ast := &query.AST{
		Kind:      query.UpdateQuery,
		FromTable: query.TableRef{Name: "users"},
		SetClauses: []query.SetClause{
			{Column: bio, Value: query.LiteralExpr{Value: nil}},
		},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NULL") {
		t.Errorf("SQL should contain NULL: %s", sql)
	}
}

func TestMySQL_InClause(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "IN ('pending', 'processing')") {
		t.Errorf("SQL should contain IN clause: %s", sql)
	}
}

func TestMySQL_IsNull(t *testing.T) {
	deletedAt := query.NullTimeColumn{Table: "users", Name: "deleted_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where:     query.UnaryExpr{Op: query.OpIsNull, Expr: query.ColumnExpr{Column: deletedAt}},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "`users`.`deleted_at` IS NULL") {
		t.Errorf("SQL should contain IS NULL: %s", sql)
	}
}

func TestMySQL_IsNotNull(t *testing.T) {
	deletedAt := query.NullTimeColumn{Table: "users", Name: "deleted_at"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Where:     query.UnaryExpr{Op: query.OpNotNull, Expr: query.ColumnExpr{Column: deletedAt}},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "`users`.`deleted_at` IS NOT NULL") {
		t.Errorf("SQL should contain IS NOT NULL: %s", sql)
	}
}

func TestMySQL_ILike(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL: ILIKE becomes LOWER() LIKE LOWER()
	if !containsStr(sql, "LOWER(`users`.`name`) LIKE LOWER('%john%')") {
		t.Errorf("SQL should contain LOWER() LIKE LOWER(): %s", sql)
	}
}

func TestMySQL_Like(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "LIKE '%john%'") {
		t.Errorf("SQL should contain LIKE: %s", sql)
	}
}

func TestMySQL_JSONAggregation(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL uses JSON_ARRAYAGG and JSON_OBJECT
	if !containsStr(sql, "JSON_ARRAYAGG") {
		t.Errorf("SQL should contain JSON_ARRAYAGG: %s", sql)
	}
	if !containsStr(sql, "JSON_OBJECT") {
		t.Errorf("SQL should contain JSON_OBJECT: %s", sql)
	}
	if !containsStr(sql, "JSON_ARRAY()") {
		t.Errorf("SQL should contain JSON_ARRAY() as empty fallback: %s", sql)
	}
	if containsStr(sql, "JSON_AGG") && !containsStr(sql, "JSON_ARRAYAGG") {
		t.Errorf("MySQL SQL should NOT contain Postgres JSON_AGG: %s", sql)
	}
}

func TestMySQL_StringEscaping(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Single quotes should be escaped by doubling
	if !containsStr(sql, "'O''Brien'") {
		t.Errorf("SQL should escape single quotes: %s", sql)
	}
}

func TestMySQL_OrderByStringCollation(t *testing.T) {
	nameCol := query.StringColumn{Table: "authors", Name: "name"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: nameCol}},
		},
		OrderBy: []query.OrderByExpr{
			{Expr: query.ColumnExpr{Column: nameCol}},
		},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// MySQL should add COLLATE utf8mb4_bin for case-sensitive ordering
	if !containsStr(sql, "COLLATE utf8mb4_bin") {
		t.Errorf("MySQL ORDER BY on string column should include COLLATE utf8mb4_bin: %s", sql)
	}
}

func TestMySQL_OrderByIntNoCollation(t *testing.T) {
	idCol := query.Int64Column{Table: "authors", Name: "id"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: idCol}},
		},
		OrderBy: []query.OrderByExpr{
			{Expr: query.ColumnExpr{Column: idCol}},
		},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Non-string columns should NOT have COLLATE
	if containsStr(sql, "COLLATE") {
		t.Errorf("MySQL ORDER BY on non-string column should NOT include COLLATE: %s", sql)
	}
}

func TestMySQL_ComparisonOperators(t *testing.T) {
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

			sql, _, err := NewCompiler(MySQL).Compile(ast)
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

func TestMySQL_CountStar(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}},
		},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "SELECT COUNT(*) FROM `users`"
	if sql != expected {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expected, sql)
	}
}

func TestMySQL_CountDistinct(t *testing.T) {
	emailCol := query.StringColumn{Table: "users", Name: "email"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: query.ColumnExpr{Column: emailCol}, Distinct: true}},
		},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "COUNT(DISTINCT") {
		t.Errorf("SQL should contain COUNT(DISTINCT: %s", sql)
	}
}

func TestMySQL_SumAvgMinMax(t *testing.T) {
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

			sql, _, err := NewCompiler(MySQL).Compile(ast)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}

			if !containsStr(sql, tt.expected) {
				t.Errorf("SQL should contain %s: %s", tt.expected, sql)
			}
		})
	}
}

func TestMySQL_SelectDistinct(t *testing.T) {
	countryCol := query.StringColumn{Table: "users", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		Distinct:  true,
		FromTable: query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
	}

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "SELECT DISTINCT") {
		t.Errorf("SQL should contain SELECT DISTINCT: %s", sql)
	}
}

func TestMySQL_Subquery(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "IN (SELECT") {
		t.Errorf("SQL should contain IN (SELECT: %s", sql)
	}
	if !containsStr(sql, "`vip_customers`") {
		t.Errorf("SQL should contain vip_customers: %s", sql)
	}
}

func TestMySQL_Exists(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "EXISTS (SELECT") {
		t.Errorf("SQL should contain EXISTS (SELECT: %s", sql)
	}
}

func TestMySQL_NotExists(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "NOT EXISTS (SELECT") {
		t.Errorf("SQL should contain NOT EXISTS (SELECT: %s", sql)
	}
}

func TestMySQL_Union(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "UNION") {
		t.Errorf("SQL should contain UNION: %s", sql)
	}
	if !containsStr(sql, "`active_users`") {
		t.Errorf("SQL should contain active_users: %s", sql)
	}
}

func TestMySQL_Intersect(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "INTERSECT") {
		t.Errorf("SQL should contain INTERSECT: %s", sql)
	}
}

func TestMySQL_CTE(t *testing.T) {
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

	sql, _, err := NewCompiler(MySQL).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !containsStr(sql, "WITH") {
		t.Errorf("SQL should contain WITH: %s", sql)
	}
	if !containsStr(sql, "`pending_orders`") {
		t.Errorf("SQL should contain pending_orders: %s", sql)
	}
	if !containsStr(sql, "AS (SELECT") {
		t.Errorf("SQL should contain AS (SELECT: %s", sql)
	}
}
