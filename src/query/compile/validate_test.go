package compile

import (
	"strings"
	"testing"

	"github.com/portsql/portsql/src/query"
)

func TestValidateInsert_EmptyValues(t *testing.T) {
	// INSERT with no values should error
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{},
		InsertVals: []query.Expr{},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for INSERT with no values, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "INSERT requires at least one value") {
		t.Errorf("Expected error about INSERT requiring values, got: %v", err)
	}
}

func TestValidateInsert_WithValues(t *testing.T) {
	// INSERT with values should pass validation
	col := query.StringColumn{Table: "users", Name: "email"}
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{col},
		InsertVals: []query.Expr{query.ParamExpr{Name: "email", GoType: "string"}},
	}

	err := ValidateAST(ast)
	if err != nil {
		t.Errorf("Expected no error for valid INSERT, got: %v", err)
	}
}

func TestValidateInsert_ColumnValueMismatch(t *testing.T) {
	// INSERT with mismatched column/value counts should error
	col1 := query.StringColumn{Table: "users", Name: "email"}
	col2 := query.StringColumn{Table: "users", Name: "name"}
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{col1, col2},
		InsertVals: []query.Expr{query.ParamExpr{Name: "email", GoType: "string"}}, // Only 1 value
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for column/value count mismatch, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "column count") {
		t.Errorf("Expected error about column count, got: %v", err)
	}
}

func TestValidateUpdate_NoSetClauses(t *testing.T) {
	// UPDATE with no SET clauses should error
	ast := &query.AST{
		Kind:       query.UpdateQuery,
		FromTable:  query.TableRef{Name: "users"},
		SetClauses: []query.SetClause{},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for UPDATE with no SET clauses, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "UPDATE requires at least one SET clause") {
		t.Errorf("Expected error about SET clause, got: %v", err)
	}
}

func TestValidateIdentifier_Valid(t *testing.T) {
	tests := []string{
		"users",
		"user_id",
		"_private",
		"User123",
		"a",
	}
	for _, name := range tests {
		if err := ValidateIdentifier(name); err != nil {
			t.Errorf("ValidateIdentifier(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateIdentifier_Invalid(t *testing.T) {
	tests := []string{
		"",          // empty
		"123abc",    // starts with digit
		"user-name", // contains hyphen
		"user.name", // contains dot
		"user name", // contains space
	}
	for _, name := range tests {
		if err := ValidateIdentifier(name); err == nil {
			t.Errorf("ValidateIdentifier(%q) = nil, want error", name)
		}
	}
}

func TestValidateAST_Nil(t *testing.T) {
	err := ValidateAST(nil)
	if err == nil {
		t.Error("Expected error for nil AST, got nil")
	}
}

func TestCompile_EmptyINList(t *testing.T) {
	// IN () with empty list should error during compilation
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: col},
			Op:    query.OpIn,
			Right: query.ListExpr{Values: []query.Expr{}}, // Empty list
		},
	}

	compiler := NewCompiler(Postgres)
	_, _, err := compiler.Compile(ast)
	if err == nil {
		t.Error("Expected error for empty IN list, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "IN clause requires at least one value") {
		t.Errorf("Expected error about IN clause requiring values, got: %v", err)
	}
}

func TestCompile_NonEmptyINList(t *testing.T) {
	// IN with values should compile successfully
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: col},
			Op:    query.OpIn,
			Right: query.ListExpr{Values: []query.Expr{
				query.LiteralExpr{Value: 1},
				query.LiteralExpr{Value: 2},
			}},
		},
	}

	compiler := NewCompiler(Postgres)
	sql, _, err := compiler.Compile(ast)
	if err != nil {
		t.Errorf("Expected no error for valid IN list, got: %v", err)
	}
	if !strings.Contains(sql, "IN") {
		t.Errorf("Expected SQL to contain IN, got: %s", sql)
	}
}

func TestValidate_EmptyFromTable(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: ""}, // Empty
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for empty FROM table name")
	}
	if err != nil && !strings.Contains(err.Error(), "FROM table name cannot be empty") {
		t.Errorf("Expected error about empty FROM table, got: %v", err)
	}
}

func TestValidate_EmptyJoinTable(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Joins: []query.JoinClause{
			{
				Type:      query.InnerJoin,
				Table:     query.TableRef{Name: ""}, // Empty
				Condition: query.BinaryExpr{Left: query.ColumnExpr{Column: col}, Op: query.OpEq, Right: query.LiteralExpr{Value: 1}},
			},
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for empty JOIN table name")
	}
	if err != nil && !strings.Contains(err.Error(), "table name cannot be empty") {
		t.Errorf("Expected error about empty JOIN table, got: %v", err)
	}
}

func TestValidate_NilJoinCondition(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "users"},
		Joins: []query.JoinClause{
			{
				Type:      query.InnerJoin,
				Table:     query.TableRef{Name: "orders"},
				Condition: nil, // Nil condition
			},
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for nil JOIN condition")
	}
	if err != nil && !strings.Contains(err.Error(), "condition cannot be nil") {
		t.Errorf("Expected error about nil JOIN condition, got: %v", err)
	}
}

func TestValidate_EmptyParamName(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where:      query.ParamExpr{Name: "", GoType: "int64"}, // Empty name
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for empty parameter name")
	}
	if err != nil && !strings.Contains(err.Error(), "parameter name cannot be empty") {
		t.Errorf("Expected error about empty parameter name, got: %v", err)
	}
}

func TestValidate_NilSubquery(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: col},
			Op:    query.OpIn,
			Right: query.SubqueryExpr{Query: nil}, // Nil subquery
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for nil subquery")
	}
	if err != nil && !strings.Contains(err.Error(), "subquery cannot be nil") {
		t.Errorf("Expected error about nil subquery, got: %v", err)
	}
}

func TestValidate_EmptyJSONAggColumns(t *testing.T) {
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "authors"},
		SelectCols: []query.SelectExpr{
			{
				Expr: query.JSONAggExpr{
					FieldName: "books",
					Columns:   []query.Column{}, // Empty columns
				},
				Alias: "books",
			},
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for empty JSON agg columns")
	}
	if err != nil && !strings.Contains(err.Error(), "JSON aggregation requires at least one column") {
		t.Errorf("Expected error about JSON aggregation columns, got: %v", err)
	}
}

func TestValidate_SetOpNilLeftBranch(t *testing.T) {
	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Op:    query.SetOpUnion,
			Left:  nil, // Nil left branch
			Right: &query.AST{Kind: query.SelectQuery, FromTable: query.TableRef{Name: "users"}},
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for nil set operation left branch")
	}
	if err != nil && !strings.Contains(err.Error(), "left branch cannot be nil") {
		t.Errorf("Expected error about nil left branch, got: %v", err)
	}
}

func TestValidate_SetOpNilRightBranch(t *testing.T) {
	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Op:    query.SetOpUnion,
			Left:  &query.AST{Kind: query.SelectQuery, FromTable: query.TableRef{Name: "users"}},
			Right: nil, // Nil right branch
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for nil set operation right branch")
	}
	if err != nil && !strings.Contains(err.Error(), "right branch cannot be nil") {
		t.Errorf("Expected error about nil right branch, got: %v", err)
	}
}

func TestValidate_NilExistsSubquery(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind:       query.SelectQuery,
		FromTable:  query.TableRef{Name: "users"},
		SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
		Where:      query.ExistsExpr{Subquery: nil}, // Nil EXISTS subquery
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for nil EXISTS subquery")
	}
	if err != nil && !strings.Contains(err.Error(), "EXISTS subquery cannot be nil") {
		t.Errorf("Expected error about nil EXISTS subquery, got: %v", err)
	}
}

func TestValidate_ValidSetOperation(t *testing.T) {
	col := query.Int64Column{Table: "users", Name: "id"}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Op: query.SetOpUnion,
			Left: &query.AST{
				Kind:       query.SelectQuery,
				FromTable:  query.TableRef{Name: "users"},
				SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
			},
			Right: &query.AST{
				Kind:       query.SelectQuery,
				FromTable:  query.TableRef{Name: "admins"},
				SelectCols: []query.SelectExpr{{Expr: query.ColumnExpr{Column: col}}},
			},
		},
	}

	err := ValidateAST(ast)
	if err != nil {
		t.Errorf("Expected no error for valid set operation, got: %v", err)
	}
}
