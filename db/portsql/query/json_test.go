package query

import (
	"encoding/json"
	"testing"
)

func TestASTJSONRoundtrip(t *testing.T) {
	// Create a simple SELECT query AST
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name: "users",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}}},
			{Expr: ColumnExpr{Column: StringColumn{Table: "users", Name: "name"}}},
			{Expr: ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}}, Alias: "user_email"},
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "email", GoType: "string"},
		},
		Params: []ParamInfo{
			{Name: "email", GoType: "string"},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("failed to marshal AST: %v", err)
	}

	// Unmarshal back
	var restored AST
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal AST: %v", err)
	}

	// Verify basic structure
	if restored.Kind != SelectQuery {
		t.Errorf("expected kind %q, got %q", SelectQuery, restored.Kind)
	}
	if restored.FromTable.Name != "users" {
		t.Errorf("expected table name 'users', got %q", restored.FromTable.Name)
	}
	if len(restored.SelectCols) != 3 {
		t.Errorf("expected 3 select columns, got %d", len(restored.SelectCols))
	}
	if len(restored.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(restored.Params))
	}
	if restored.Params[0].Name != "email" {
		t.Errorf("expected param name 'email', got %q", restored.Params[0].Name)
	}
}

func TestASTWithLimitAndOffset(t *testing.T) {
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name: "posts",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "posts", Name: "id"}}},
		},
		OrderBy: []OrderByExpr{
			{Expr: ColumnExpr{Column: TimeColumn{Table: "posts", Name: "created_at"}}, Desc: true},
		},
		Limit:  ParamExpr{Name: "limit", GoType: "int64"},
		Offset: ParamExpr{Name: "offset", GoType: "int64"},
		Params: []ParamInfo{
			{Name: "limit", GoType: "int64"},
			{Name: "offset", GoType: "int64"},
		},
	}

	data, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var restored AST
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if restored.Limit == nil {
		t.Error("expected Limit to be set")
	}
	if restored.Offset == nil {
		t.Error("expected Offset to be set")
	}
	if len(restored.OrderBy) != 1 {
		t.Errorf("expected 1 order by, got %d", len(restored.OrderBy))
	}
	if !restored.OrderBy[0].Desc {
		t.Error("expected DESC order")
	}
}

func TestASTUpdateQuery(t *testing.T) {
	ast := &AST{
		Kind: UpdateQuery,
		FromTable: TableRef{
			Name: "users",
		},
		SetClauses: []SetClause{
			{
				Column: StringColumn{Table: "users", Name: "name"},
				Value:  ParamExpr{Name: "name", GoType: "string"},
			},
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "id", GoType: "int64"},
		},
		Params: []ParamInfo{
			{Name: "name", GoType: "string"},
			{Name: "id", GoType: "int64"},
		},
	}

	data, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var restored AST
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if restored.Kind != UpdateQuery {
		t.Errorf("expected UpdateQuery, got %s", restored.Kind)
	}
	if len(restored.SetClauses) != 1 {
		t.Errorf("expected 1 set clause, got %d", len(restored.SetClauses))
	}
}

func TestColumnJSONMapping(t *testing.T) {
	tests := []struct {
		name   string
		column Column
		goType string
	}{
		{"Int32", Int32Column{Table: "t", Name: "c"}, "int32"},
		{"NullInt32", NullInt32Column{Table: "t", Name: "c"}, "*int32"},
		{"Int64", Int64Column{Table: "t", Name: "c"}, "int64"},
		{"NullInt64", NullInt64Column{Table: "t", Name: "c"}, "*int64"},
		{"Float64", Float64Column{Table: "t", Name: "c"}, "float64"},
		{"NullFloat64", NullFloat64Column{Table: "t", Name: "c"}, "*float64"},
		{"Bool", BoolColumn{Table: "t", Name: "c"}, "bool"},
		{"NullBool", NullBoolColumn{Table: "t", Name: "c"}, "*bool"},
		{"String", StringColumn{Table: "t", Name: "c"}, "string"},
		{"NullString", NullStringColumn{Table: "t", Name: "c"}, "*string"},
		{"Time", TimeColumn{Table: "t", Name: "c"}, "time.Time"},
		{"NullTime", NullTimeColumn{Table: "t", Name: "c"}, "*time.Time"},
		{"Bytes", BytesColumn{Table: "t", Name: "c"}, "[]byte"},
		{"JSON", JSONColumn{Table: "t", Name: "c"}, "json.RawMessage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to JSON
			j := columnToJSON(tt.column)

			if j.GoType != tt.goType {
				t.Errorf("expected GoType %q, got %q", tt.goType, j.GoType)
			}
			if j.Table != "t" {
				t.Errorf("expected table 't', got %q", j.Table)
			}
			if j.Name != "c" {
				t.Errorf("expected name 'c', got %q", j.Name)
			}

			// Convert back
			restored := j.ToColumn()
			if restored.GoType() != tt.goType {
				t.Errorf("restored GoType mismatch: expected %q, got %q", tt.goType, restored.GoType())
			}
		})
	}
}

func TestExpressionTypes(t *testing.T) {
	tests := []struct {
		name string
		expr Expr
	}{
		{"ColumnExpr", ColumnExpr{Column: StringColumn{Table: "t", Name: "c"}}},
		{"ParamExpr", ParamExpr{Name: "p", GoType: "string"}},
		{"LiteralExpr", LiteralExpr{Value: 42}},
		{"BinaryEq", BinaryExpr{
			Left:  ColumnExpr{Column: Int64Column{Table: "t", Name: "id"}},
			Op:    OpEq,
			Right: LiteralExpr{Value: 1},
		}},
		{"UnaryIsNull", UnaryExpr{
			Op:   OpIsNull,
			Expr: ColumnExpr{Column: NullStringColumn{Table: "t", Name: "c"}},
		}},
		{"FuncExpr", FuncExpr{Name: "UPPER", Args: []Expr{ColumnExpr{Column: StringColumn{Table: "t", Name: "c"}}}}},
		{"ListExpr", ListExpr{Values: []Expr{LiteralExpr{Value: 1}, LiteralExpr{Value: 2}}}},
		{"AggregateCount", AggregateExpr{Func: AggCount, Arg: nil}},
		{"AggregateSum", AggregateExpr{Func: AggSum, Arg: ColumnExpr{Column: Float64Column{Table: "t", Name: "amount"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to JSON
			j, err := exprToJSON(tt.expr)
			if err != nil {
				t.Fatalf("exprToJSON failed: %v", err)
			}

			// Convert back
			restored, err := j.FromJSON()
			if err != nil {
				t.Fatalf("FromJSON failed: %v", err)
			}

			// Basic check - ensure it's not nil
			if restored == nil {
				t.Error("restored expression is nil")
			}
		})
	}
}
