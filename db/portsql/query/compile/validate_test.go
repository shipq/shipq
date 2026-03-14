package compile

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
)

func TestValidateInsert_EmptyValues(t *testing.T) {
	// INSERT with no values should error
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{},
		InsertRows: [][]query.Expr{},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for INSERT with no values, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "INSERT requires either VALUES rows or a SELECT source") {
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
		InsertRows: [][]query.Expr{{query.ParamExpr{Name: "email", GoType: "string"}}},
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
		InsertRows: [][]query.Expr{{query.ParamExpr{Name: "email", GoType: "string"}}}, // Only 1 value
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
			Left: query.ColumnExpr{Column: col},
			Op:   query.OpIn,
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

func TestValidateInsert_MultipleRowsOK(t *testing.T) {
	// 3 rows, all same width, matching column count — should pass
	col1 := query.StringColumn{Table: "users", Name: "name"}
	col2 := query.StringColumn{Table: "users", Name: "email"}
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{col1, col2},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "name_0", GoType: "string"}, query.ParamExpr{Name: "email_0", GoType: "string"}},
			{query.ParamExpr{Name: "name_1", GoType: "string"}, query.ParamExpr{Name: "email_1", GoType: "string"}},
			{query.ParamExpr{Name: "name_2", GoType: "string"}, query.ParamExpr{Name: "email_2", GoType: "string"}},
		},
	}

	err := ValidateAST(ast)
	if err != nil {
		t.Errorf("Expected no error for valid multi-row INSERT, got: %v", err)
	}
}

func TestValidateInsert_MismatchedRowWidth(t *testing.T) {
	// Row 1 has 2 values, row 2 has 1 — should error
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "users"},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "a", GoType: "string"}, query.ParamExpr{Name: "b", GoType: "string"}},
			{query.ParamExpr{Name: "c", GoType: "string"}}, // wrong width
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for mismatched row widths, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "all rows must match") {
		t.Errorf("Expected error about row width mismatch, got: %v", err)
	}
}

func TestValidateInsert_EmptyRow(t *testing.T) {
	// One row is empty — should error
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "users"},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "a", GoType: "string"}},
			{}, // empty row
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for empty row, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "has no values") {
		t.Errorf("Expected error about empty row, got: %v", err)
	}
}

func TestValidateInsert_ColCountMismatch_BulkRow(t *testing.T) {
	// 3 columns but row has 2 values — should error
	col1 := query.StringColumn{Table: "users", Name: "name"}
	col2 := query.StringColumn{Table: "users", Name: "email"}
	col3 := query.StringColumn{Table: "users", Name: "bio"}
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{col1, col2, col3},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "name_0", GoType: "string"}, query.ParamExpr{Name: "email_0", GoType: "string"}, query.ParamExpr{Name: "bio_0", GoType: "string"}},
			{query.ParamExpr{Name: "name_1", GoType: "string"}, query.ParamExpr{Name: "email_1", GoType: "string"}}, // only 2 values
		},
	}

	err := ValidateAST(ast)
	if err == nil {
		t.Error("Expected error for column/value count mismatch in bulk row, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "column count") {
		t.Errorf("Expected error about column count mismatch, got: %v", err)
	}
}

func TestValidateInsert_SingleRowStillWorks(t *testing.T) {
	// Single row backward compatibility — should pass
	col := query.StringColumn{Table: "users", Name: "email"}
	ast := &query.AST{
		Kind:       query.InsertQuery,
		FromTable:  query.TableRef{Name: "users"},
		InsertCols: []query.Column{col},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "email", GoType: "string"}},
		},
	}

	err := ValidateAST(ast)
	if err != nil {
		t.Errorf("Expected no error for valid single-row INSERT, got: %v", err)
	}
}

func TestValidateInsert_NoColumnsMultipleRows(t *testing.T) {
	// No explicit columns, but all rows same width — should pass
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "users"},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "a", GoType: "string"}, query.ParamExpr{Name: "b", GoType: "string"}},
			{query.ParamExpr{Name: "c", GoType: "string"}, query.ParamExpr{Name: "d", GoType: "string"}},
		},
	}

	err := ValidateAST(ast)
	if err != nil {
		t.Errorf("Expected no error for multi-row INSERT without explicit columns, got: %v", err)
	}
}

func TestValidateInsert_SelectSource_OK(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
		},
	}
	if err := ValidateAST(ast); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateInsert_SelectSource_WithCTEs_OK(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		CTEs: []query.CTE{
			{
				Name: "active_sources",
				Query: &query.AST{
					Kind:      query.SelectQuery,
					FromTable: query.TableRef{Name: "source"},
					SelectCols: []query.SelectExpr{
						{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
					},
					Where: query.BinaryExpr{
						Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "active"}},
						Op:    query.OpEq,
						Right: query.LiteralExpr{Value: true},
					},
				},
			},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "active_sources"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "active_sources", Name: "name"}}},
			},
		},
	}
	if err := ValidateAST(ast); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateInsert_SelectSource_InvalidSourceKind(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		InsertSource: &query.AST{
			Kind:      query.UpdateQuery,
			FromTable: query.TableRef{Name: "source"},
		},
	}
	err := ValidateAST(ast)
	if err == nil {
		t.Error("expected error for non-SELECT InsertSource, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "must be a SELECT query") {
		t.Errorf("expected error about must be a SELECT query, got: %v", err)
	}
}

func TestValidateInsert_MutualExclusivity(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		InsertRows: [][]query.Expr{
			{query.ParamExpr{Name: "name", GoType: "string"}},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
		},
	}
	err := ValidateAST(ast)
	if err == nil {
		t.Error("expected error when both InsertRows and InsertSource are set, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot have both") {
		t.Errorf("expected error about cannot have both, got: %v", err)
	}
}

func TestValidateInsert_NeitherRowsNorSource(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		InsertRows:   [][]query.Expr{},
		InsertSource: nil,
	}
	err := ValidateAST(ast)
	if err == nil {
		t.Error("expected error when neither InsertRows nor InsertSource is set, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "requires either VALUES rows or a SELECT source") {
		t.Errorf("expected error about requires either VALUES rows or a SELECT source, got: %v", err)
	}
}

func TestValidateInsert_SelectSource_InvalidSubquery(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ParamExpr{Name: "", GoType: "string"}},
			},
		},
	}
	err := ValidateAST(ast)
	if err == nil {
		t.Error("expected error for invalid expression in InsertSource, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "parameter name cannot be empty") {
		t.Errorf("expected wrapped error about empty parameter name, got: %v", err)
	}
}

func TestValidateInsert_SelectSource_WithParams(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
			},
			Where: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "active"}},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "is_active", GoType: "bool"},
			},
		},
	}
	if err := ValidateAST(ast); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateInsert_SelectSource_ColumnValuesConsistency(t *testing.T) {
	ast := &query.AST{
		Kind:      query.InsertQuery,
		FromTable: query.TableRef{Name: "target"},
		InsertCols: []query.Column{
			query.StringColumn{Table: "target", Name: "name"},
			query.StringColumn{Table: "target", Name: "email"},
			query.StringColumn{Table: "target", Name: "bio"},
		},
		InsertSource: &query.AST{
			Kind:      query.SelectQuery,
			FromTable: query.TableRef{Name: "source"},
			SelectCols: []query.SelectExpr{
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "name"}}},
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "email"}}},
				{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "source", Name: "bio"}}},
			},
		},
	}
	if err := ValidateAST(ast); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
