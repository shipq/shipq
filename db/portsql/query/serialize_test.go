package query

import (
	"encoding/json"
	"testing"
)

func TestSerializeExpr_Column(t *testing.T) {
	col := StringColumn{Table: "users", Name: "email"}
	expr := ColumnExpr{Column: col}

	s := SerializeExpr(expr)

	if s.Type != "column" {
		t.Errorf("expected Type = %q, got %q", "column", s.Type)
	}
	if s.Column == nil {
		t.Fatal("expected Column to be non-nil")
	}
	if s.Column.Table != "users" {
		t.Errorf("expected Column.Table = %q, got %q", "users", s.Column.Table)
	}
	if s.Column.Name != "email" {
		t.Errorf("expected Column.Name = %q, got %q", "email", s.Column.Name)
	}
	if s.Column.GoType != "string" {
		t.Errorf("expected Column.GoType = %q, got %q", "string", s.Column.GoType)
	}
}

func TestSerializeExpr_Param(t *testing.T) {
	expr := ParamExpr{Name: "user_id", GoType: "int64"}

	s := SerializeExpr(expr)

	if s.Type != "param" {
		t.Errorf("expected Type = %q, got %q", "param", s.Type)
	}
	if s.Param == nil {
		t.Fatal("expected Param to be non-nil")
	}
	if s.Param.Name != "user_id" {
		t.Errorf("expected Param.Name = %q, got %q", "user_id", s.Param.Name)
	}
	if s.Param.GoType != "int64" {
		t.Errorf("expected Param.GoType = %q, got %q", "int64", s.Param.GoType)
	}
}

func TestSerializeExpr_Literal(t *testing.T) {
	expr := LiteralExpr{Value: 42}

	s := SerializeExpr(expr)

	if s.Type != "literal" {
		t.Errorf("expected Type = %q, got %q", "literal", s.Type)
	}
	if s.Literal != 42 {
		t.Errorf("expected Literal = 42, got %v", s.Literal)
	}
}

func TestSerializeExpr_Binary(t *testing.T) {
	left := ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}}
	right := ParamExpr{Name: "email", GoType: "string"}
	expr := BinaryExpr{Left: left, Op: OpEq, Right: right}

	s := SerializeExpr(expr)

	if s.Type != "binary" {
		t.Errorf("expected Type = %q, got %q", "binary", s.Type)
	}
	if s.Binary == nil {
		t.Fatal("expected Binary to be non-nil")
	}
	if s.Binary.Op != "=" {
		t.Errorf("expected Binary.Op = %q, got %q", "=", s.Binary.Op)
	}
	if s.Binary.Left.Type != "column" {
		t.Errorf("expected Binary.Left.Type = %q, got %q", "column", s.Binary.Left.Type)
	}
	if s.Binary.Right.Type != "param" {
		t.Errorf("expected Binary.Right.Type = %q, got %q", "param", s.Binary.Right.Type)
	}
}

func TestSerializeExpr_Unary(t *testing.T) {
	inner := ColumnExpr{Column: NullStringColumn{Table: "users", Name: "deleted_at"}}
	expr := UnaryExpr{Op: OpIsNull, Expr: inner}

	s := SerializeExpr(expr)

	if s.Type != "unary" {
		t.Errorf("expected Type = %q, got %q", "unary", s.Type)
	}
	if s.Unary == nil {
		t.Fatal("expected Unary to be non-nil")
	}
	if s.Unary.Op != "IS NULL" {
		t.Errorf("expected Unary.Op = %q, got %q", "IS NULL", s.Unary.Op)
	}
}

func TestSerializeExpr_Func(t *testing.T) {
	expr := FuncExpr{Name: "LOWER", Args: []Expr{ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}}}}

	s := SerializeExpr(expr)

	if s.Type != "func" {
		t.Errorf("expected Type = %q, got %q", "func", s.Type)
	}
	if s.Func == nil {
		t.Fatal("expected Func to be non-nil")
	}
	if s.Func.Name != "LOWER" {
		t.Errorf("expected Func.Name = %q, got %q", "LOWER", s.Func.Name)
	}
	if len(s.Func.Args) != 1 {
		t.Errorf("expected len(Func.Args) = 1, got %d", len(s.Func.Args))
	}
}

func TestSerializeExpr_List(t *testing.T) {
	expr := ListExpr{Values: []Expr{LiteralExpr{Value: 1}, LiteralExpr{Value: 2}, LiteralExpr{Value: 3}}}

	s := SerializeExpr(expr)

	if s.Type != "list" {
		t.Errorf("expected Type = %q, got %q", "list", s.Type)
	}
	if len(s.List) != 3 {
		t.Errorf("expected len(List) = 3, got %d", len(s.List))
	}
}

func TestSerializeExpr_Aggregate(t *testing.T) {
	expr := AggregateExpr{Func: AggCount, Arg: nil, Distinct: false}

	s := SerializeExpr(expr)

	if s.Type != "aggregate" {
		t.Errorf("expected Type = %q, got %q", "aggregate", s.Type)
	}
	if s.Aggregate == nil {
		t.Fatal("expected Aggregate to be non-nil")
	}
	if s.Aggregate.Func != "COUNT" {
		t.Errorf("expected Aggregate.Func = %q, got %q", "COUNT", s.Aggregate.Func)
	}
}

func TestSerializeExpr_AggregateWithDistinct(t *testing.T) {
	col := StringColumn{Table: "users", Name: "email"}
	expr := AggregateExpr{Func: AggCount, Arg: ColumnExpr{Column: col}, Distinct: true}

	s := SerializeExpr(expr)

	if s.Type != "aggregate" {
		t.Errorf("expected Type = %q, got %q", "aggregate", s.Type)
	}
	if !s.Aggregate.Distinct {
		t.Error("expected Aggregate.Distinct = true")
	}
	if s.Aggregate.Arg == nil {
		t.Fatal("expected Aggregate.Arg to be non-nil")
	}
}

func TestSerializeAST_SimpleSelect(t *testing.T) {
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name: "users",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}}},
			{Expr: ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}}},
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "email", GoType: "string"},
		},
	}

	s := SerializeAST(ast)

	if s.Kind != "select" {
		t.Errorf("expected Kind = %q, got %q", "select", s.Kind)
	}
	if s.FromTable.Name != "users" {
		t.Errorf("expected FromTable.Name = %q, got %q", "users", s.FromTable.Name)
	}
	if len(s.SelectCols) != 2 {
		t.Errorf("expected len(SelectCols) = 2, got %d", len(s.SelectCols))
	}
	if s.Where == nil {
		t.Fatal("expected Where to be non-nil")
	}
	if s.Where.Type != "binary" {
		t.Errorf("expected Where.Type = %q, got %q", "binary", s.Where.Type)
	}
}

func TestSerializeAST_WithJoin(t *testing.T) {
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name:  "users",
			Alias: "u",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "u", Name: "id"}}},
		},
		Joins: []JoinClause{
			{
				Type: LeftJoin,
				Table: TableRef{
					Name:  "orders",
					Alias: "o",
				},
				Condition: BinaryExpr{
					Left:  ColumnExpr{Column: Int64Column{Table: "o", Name: "user_id"}},
					Op:    OpEq,
					Right: ColumnExpr{Column: Int64Column{Table: "u", Name: "id"}},
				},
			},
		},
	}

	s := SerializeAST(ast)

	if len(s.Joins) != 1 {
		t.Fatalf("expected len(Joins) = 1, got %d", len(s.Joins))
	}
	if s.Joins[0].Type != "LEFT" {
		t.Errorf("expected Joins[0].Type = %q, got %q", "LEFT", s.Joins[0].Type)
	}
	if s.Joins[0].Table.Name != "orders" {
		t.Errorf("expected Joins[0].Table.Name = %q, got %q", "orders", s.Joins[0].Table.Name)
	}
	if s.Joins[0].Table.Alias != "o" {
		t.Errorf("expected Joins[0].Table.Alias = %q, got %q", "o", s.Joins[0].Table.Alias)
	}
}

func TestSerializeAST_Insert(t *testing.T) {
	ast := &AST{
		Kind: InsertQuery,
		FromTable: TableRef{
			Name: "users",
		},
		InsertCols: []Column{
			StringColumn{Table: "users", Name: "email"},
			StringColumn{Table: "users", Name: "name"},
		},
		InsertRows: [][]Expr{{
			ParamExpr{Name: "email", GoType: "string"},
			ParamExpr{Name: "name", GoType: "string"},
		}},
		Returning: []Column{
			Int64Column{Table: "users", Name: "id"},
		},
	}

	s := SerializeAST(ast)

	if s.Kind != "insert" {
		t.Errorf("expected Kind = %q, got %q", "insert", s.Kind)
	}
	if len(s.InsertCols) != 2 {
		t.Errorf("expected len(InsertCols) = 2, got %d", len(s.InsertCols))
	}
	if len(s.InsertRows) != 1 {
		t.Errorf("expected len(InsertRows) = 1, got %d", len(s.InsertRows))
	}
	if len(s.InsertRows[0]) != 2 {
		t.Errorf("expected len(InsertRows[0]) = 2, got %d", len(s.InsertRows[0]))
	}
	if len(s.Returning) != 1 {
		t.Errorf("expected len(Returning) = 1, got %d", len(s.Returning))
	}
}

func TestSerializeAST_Update(t *testing.T) {
	ast := &AST{
		Kind: UpdateQuery,
		FromTable: TableRef{
			Name: "users",
		},
		SetClauses: []SetClause{
			{
				Column: StringColumn{Table: "users", Name: "email"},
				Value:  ParamExpr{Name: "email", GoType: "string"},
			},
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "id", GoType: "int64"},
		},
	}

	s := SerializeAST(ast)

	if s.Kind != "update" {
		t.Errorf("expected Kind = %q, got %q", "update", s.Kind)
	}
	if len(s.SetClauses) != 1 {
		t.Fatalf("expected len(SetClauses) = 1, got %d", len(s.SetClauses))
	}
	if s.SetClauses[0].Column.Name != "email" {
		t.Errorf("expected SetClauses[0].Column.Name = %q, got %q", "email", s.SetClauses[0].Column.Name)
	}
}

func TestSerializeAST_Delete(t *testing.T) {
	ast := &AST{
		Kind: DeleteQuery,
		FromTable: TableRef{
			Name: "users",
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "id", GoType: "int64"},
		},
	}

	s := SerializeAST(ast)

	if s.Kind != "delete" {
		t.Errorf("expected Kind = %q, got %q", "delete", s.Kind)
	}
	if s.Where == nil {
		t.Fatal("expected Where to be non-nil")
	}
}

func TestSerializeAST_WithOrderByLimitOffset(t *testing.T) {
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name: "users",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}}},
		},
		OrderBy: []OrderByExpr{
			{Expr: ColumnExpr{Column: StringColumn{Table: "users", Name: "created_at"}}, Desc: true},
		},
		Limit:  ParamExpr{Name: "limit", GoType: "int"},
		Offset: ParamExpr{Name: "offset", GoType: "int"},
	}

	s := SerializeAST(ast)

	if len(s.OrderBy) != 1 {
		t.Fatalf("expected len(OrderBy) = 1, got %d", len(s.OrderBy))
	}
	if !s.OrderBy[0].Desc {
		t.Error("expected OrderBy[0].Desc = true")
	}
	if s.Limit == nil {
		t.Fatal("expected Limit to be non-nil")
	}
	if s.Offset == nil {
		t.Fatal("expected Offset to be non-nil")
	}
}

func TestSerializeAST_WithGroupByHaving(t *testing.T) {
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name: "orders",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "orders", Name: "user_id"}}},
			{Expr: AggregateExpr{Func: AggCount, Arg: nil}, Alias: "order_count"},
		},
		GroupBy: []Column{
			Int64Column{Table: "orders", Name: "user_id"},
		},
		Having: BinaryExpr{
			Left:  AggregateExpr{Func: AggCount, Arg: nil},
			Op:    OpGt,
			Right: LiteralExpr{Value: 5},
		},
	}

	s := SerializeAST(ast)

	if len(s.GroupBy) != 1 {
		t.Fatalf("expected len(GroupBy) = 1, got %d", len(s.GroupBy))
	}
	if s.GroupBy[0].Name != "user_id" {
		t.Errorf("expected GroupBy[0].Name = %q, got %q", "user_id", s.GroupBy[0].Name)
	}
	if s.Having == nil {
		t.Fatal("expected Having to be non-nil")
	}
}

func TestSerializeQueries_JSONOutput(t *testing.T) {
	// Clear registry first
	ClearRegistry()
	defer ClearRegistry()

	// Register a test query
	ast := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name: "users",
		},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}}},
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
	MustDefineOne("GetUserByEmail", ast)

	// Serialize
	data, err := SerializeQueries()
	if err != nil {
		t.Fatalf("SerializeQueries() failed: %v", err)
	}

	// Parse the JSON to verify it's valid
	var queries []SerializedQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}

	q := queries[0]
	if q.Name != "GetUserByEmail" {
		t.Errorf("expected Name = %q, got %q", "GetUserByEmail", q.Name)
	}
	if q.ReturnType != ReturnOne {
		t.Errorf("expected ReturnType = %q, got %q", ReturnOne, q.ReturnType)
	}
	if q.AST == nil {
		t.Fatal("expected AST to be non-nil")
	}
	if q.AST.Kind != "select" {
		t.Errorf("expected AST.Kind = %q, got %q", "select", q.AST.Kind)
	}
}

// Test round-trip: serialize then deserialize
func TestSerializeDeserialize_RoundTrip(t *testing.T) {
	original := &AST{
		Kind: SelectQuery,
		FromTable: TableRef{
			Name:  "users",
			Alias: "u",
		},
		Distinct: true,
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "u", Name: "id"}}},
			{Expr: ColumnExpr{Column: StringColumn{Table: "u", Name: "email"}}, Alias: "user_email"},
		},
		Joins: []JoinClause{
			{
				Type: LeftJoin,
				Table: TableRef{
					Name:  "orders",
					Alias: "o",
				},
				Condition: BinaryExpr{
					Left:  ColumnExpr{Column: Int64Column{Table: "o", Name: "user_id"}},
					Op:    OpEq,
					Right: ColumnExpr{Column: Int64Column{Table: "u", Name: "id"}},
				},
			},
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: StringColumn{Table: "u", Name: "email"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "email", GoType: "string"},
		},
		OrderBy: []OrderByExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "u", Name: "id"}}, Desc: true},
		},
		Limit:  LiteralExpr{Value: 10},
		Offset: LiteralExpr{Value: 0},
		Params: []ParamInfo{
			{Name: "email", GoType: "string"},
		},
	}

	// Serialize
	serialized := SerializeAST(original)

	// Convert to JSON and back to verify JSON compatibility
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var fromJSON SerializedAST
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	// Deserialize
	result := DeserializeAST(&fromJSON)

	// Verify key properties
	if result.Kind != original.Kind {
		t.Errorf("Kind: expected %q, got %q", original.Kind, result.Kind)
	}
	if result.Distinct != original.Distinct {
		t.Errorf("Distinct: expected %v, got %v", original.Distinct, result.Distinct)
	}
	if result.FromTable.Name != original.FromTable.Name {
		t.Errorf("FromTable.Name: expected %q, got %q", original.FromTable.Name, result.FromTable.Name)
	}
	if result.FromTable.Alias != original.FromTable.Alias {
		t.Errorf("FromTable.Alias: expected %q, got %q", original.FromTable.Alias, result.FromTable.Alias)
	}
	if len(result.SelectCols) != len(original.SelectCols) {
		t.Errorf("len(SelectCols): expected %d, got %d", len(original.SelectCols), len(result.SelectCols))
	}
	if len(result.Joins) != len(original.Joins) {
		t.Errorf("len(Joins): expected %d, got %d", len(original.Joins), len(result.Joins))
	}
	if result.Where == nil {
		t.Error("Where: expected non-nil")
	}
	if len(result.OrderBy) != len(original.OrderBy) {
		t.Errorf("len(OrderBy): expected %d, got %d", len(original.OrderBy), len(result.OrderBy))
	}
	if result.Limit == nil {
		t.Error("Limit: expected non-nil")
	}
	if result.Offset == nil {
		t.Error("Offset: expected non-nil")
	}
	if len(result.Params) != len(original.Params) {
		t.Errorf("len(Params): expected %d, got %d", len(original.Params), len(result.Params))
	}
}

func TestDeserializeExpr_RoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		expr Expr
	}{
		{
			name: "column",
			expr: ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}},
		},
		{
			name: "param",
			expr: ParamExpr{Name: "id", GoType: "int64"},
		},
		{
			name: "literal_int",
			expr: LiteralExpr{Value: 42},
		},
		{
			name: "literal_string",
			expr: LiteralExpr{Value: "hello"},
		},
		{
			name: "binary",
			expr: BinaryExpr{
				Left:  ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}},
				Op:    OpEq,
				Right: ParamExpr{Name: "id", GoType: "int64"},
			},
		},
		{
			name: "unary",
			expr: UnaryExpr{
				Op:   OpIsNull,
				Expr: ColumnExpr{Column: NullStringColumn{Table: "users", Name: "deleted_at"}},
			},
		},
		{
			name: "func",
			expr: FuncExpr{
				Name: "LOWER",
				Args: []Expr{ColumnExpr{Column: StringColumn{Table: "users", Name: "email"}}},
			},
		},
		{
			name: "list",
			expr: ListExpr{Values: []Expr{LiteralExpr{Value: 1}, LiteralExpr{Value: 2}}},
		},
		{
			name: "aggregate_count_star",
			expr: AggregateExpr{Func: AggCount, Arg: nil},
		},
		{
			name: "aggregate_sum",
			expr: AggregateExpr{
				Func: AggSum,
				Arg:  ColumnExpr{Column: Float64Column{Table: "orders", Name: "total"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize
			serialized := SerializeExpr(tc.expr)

			// Convert through JSON
			jsonData, err := json.Marshal(serialized)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var fromJSON SerializedExpr
			if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// Deserialize
			result := DeserializeExpr(fromJSON)

			// Verify type matches
			if result == nil {
				t.Fatal("result is nil")
			}

			// Re-serialize and compare JSON
			reserialized := SerializeExpr(result)
			jsonData2, err := json.Marshal(reserialized)
			if err != nil {
				t.Fatalf("failed to re-marshal: %v", err)
			}

			if string(jsonData) != string(jsonData2) {
				t.Errorf("round-trip mismatch:\noriginal:  %s\nresult:    %s", string(jsonData), string(jsonData2))
			}
		})
	}
}

func TestSimpleColumn(t *testing.T) {
	col := SimpleColumn{
		Table_:  "users",
		Name_:   "email",
		GoType_: "string",
	}

	if col.TableName() != "users" {
		t.Errorf("TableName() = %q, want %q", col.TableName(), "users")
	}
	if col.ColumnName() != "email" {
		t.Errorf("ColumnName() = %q, want %q", col.ColumnName(), "email")
	}
	if col.GoType() != "string" {
		t.Errorf("GoType() = %q, want %q", col.GoType(), "string")
	}
	if col.IsNullable() {
		t.Error("IsNullable() = true, want false")
	}
}

func TestSimpleColumn_Nullable(t *testing.T) {
	col := SimpleColumn{
		Table_:  "users",
		Name_:   "deleted_at",
		GoType_: "*time.Time",
	}

	if !col.IsNullable() {
		t.Error("IsNullable() = false, want true")
	}
}

func TestSerialize_BulkInsert_RoundTrip(t *testing.T) {
	original := &AST{
		Kind:      InsertQuery,
		FromTable: TableRef{Name: "authors"},
		InsertCols: []Column{
			StringColumn{Table: "authors", Name: "name"},
			StringColumn{Table: "authors", Name: "email"},
		},
		InsertRows: [][]Expr{
			{ParamExpr{Name: "name_0", GoType: "string"}, ParamExpr{Name: "email_0", GoType: "string"}},
			{ParamExpr{Name: "name_1", GoType: "string"}, ParamExpr{Name: "email_1", GoType: "string"}},
			{ParamExpr{Name: "name_2", GoType: "string"}, ParamExpr{Name: "email_2", GoType: "string"}},
		},
		Returning: []Column{
			StringColumn{Table: "authors", Name: "public_id"},
		},
	}

	// Serialize
	serialized := SerializeAST(original)

	// Verify the serialized form has InsertRows as a 2D array
	if len(serialized.InsertRows) != 3 {
		t.Fatalf("expected 3 InsertRows in serialized form, got %d", len(serialized.InsertRows))
	}
	for i, row := range serialized.InsertRows {
		if len(row) != 2 {
			t.Errorf("serialized row %d: expected 2 values, got %d", i, len(row))
		}
	}

	// Convert to JSON and back
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	// Verify JSON uses "insert_rows" key (not "insert_vals")
	jsonStr := string(jsonData)
	if !contains(jsonStr, `"insert_rows"`) {
		t.Errorf("expected JSON to contain \"insert_rows\" key, got: %s", jsonStr)
	}
	if contains(jsonStr, `"insert_vals"`) {
		t.Errorf("JSON should NOT contain \"insert_vals\" key, got: %s", jsonStr)
	}

	var fromJSON SerializedAST
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	// Deserialize
	result := DeserializeAST(&fromJSON)

	// Verify structural equality
	if result.Kind != original.Kind {
		t.Errorf("Kind: expected %q, got %q", original.Kind, result.Kind)
	}
	if result.FromTable.Name != original.FromTable.Name {
		t.Errorf("FromTable.Name: expected %q, got %q", original.FromTable.Name, result.FromTable.Name)
	}
	if len(result.InsertCols) != len(original.InsertCols) {
		t.Errorf("InsertCols length: expected %d, got %d", len(original.InsertCols), len(result.InsertCols))
	}
	if len(result.InsertRows) != len(original.InsertRows) {
		t.Fatalf("InsertRows length: expected %d, got %d", len(original.InsertRows), len(result.InsertRows))
	}
	for ri, row := range result.InsertRows {
		if len(row) != len(original.InsertRows[ri]) {
			t.Errorf("InsertRows[%d] length: expected %d, got %d", ri, len(original.InsertRows[ri]), len(row))
			continue
		}
		for ci, val := range row {
			p, ok := val.(ParamExpr)
			if !ok {
				t.Errorf("InsertRows[%d][%d]: expected ParamExpr, got %T", ri, ci, val)
				continue
			}
			origP := original.InsertRows[ri][ci].(ParamExpr)
			if p.Name != origP.Name {
				t.Errorf("InsertRows[%d][%d] param name: expected %q, got %q", ri, ci, origP.Name, p.Name)
			}
		}
	}
	if len(result.Returning) != len(original.Returning) {
		t.Errorf("Returning length: expected %d, got %d", len(original.Returning), len(result.Returning))
	}
}

func TestSerialize_BulkInsert_SingleRow_RoundTrip(t *testing.T) {
	// Verify single-row insert still round-trips correctly with InsertRows
	original := &AST{
		Kind:      InsertQuery,
		FromTable: TableRef{Name: "users"},
		InsertCols: []Column{
			StringColumn{Table: "users", Name: "email"},
		},
		InsertRows: [][]Expr{
			{ParamExpr{Name: "email", GoType: "string"}},
		},
	}

	serialized := SerializeAST(original)
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var fromJSON SerializedAST
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	result := DeserializeAST(&fromJSON)

	if len(result.InsertRows) != 1 {
		t.Fatalf("expected 1 InsertRows, got %d", len(result.InsertRows))
	}
	if len(result.InsertRows[0]) != 1 {
		t.Fatalf("expected 1 value in row 0, got %d", len(result.InsertRows[0]))
	}
	p, ok := result.InsertRows[0][0].(ParamExpr)
	if !ok {
		t.Fatalf("expected ParamExpr, got %T", result.InsertRows[0][0])
	}
	if p.Name != "email" {
		t.Errorf("expected param name %q, got %q", "email", p.Name)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSerializeQueries_MultipleQueries(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	// Register multiple queries
	MustDefineOne("GetUser", &AST{
		Kind:      SelectQuery,
		FromTable: TableRef{Name: "users"},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}}},
		},
	})

	MustDefineMany("ListUsers", &AST{
		Kind:      SelectQuery,
		FromTable: TableRef{Name: "users"},
		SelectCols: []SelectExpr{
			{Expr: ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}}},
		},
	})

	MustDefineExec("DeleteUser", &AST{
		Kind:      DeleteQuery,
		FromTable: TableRef{Name: "users"},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: Int64Column{Table: "users", Name: "id"}},
			Op:    OpEq,
			Right: ParamExpr{Name: "id", GoType: "int64"},
		},
	})

	data, err := SerializeQueries()
	if err != nil {
		t.Fatalf("SerializeQueries() failed: %v", err)
	}

	var queries []SerializedQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(queries) != 3 {
		t.Fatalf("expected 3 queries, got %d", len(queries))
	}

	// Verify they're sorted by name
	expectedNames := []string{"DeleteUser", "GetUser", "ListUsers"}
	for i, q := range queries {
		if q.Name != expectedNames[i] {
			t.Errorf("query %d: expected Name = %q, got %q", i, expectedNames[i], q.Name)
		}
	}

	// Verify return types
	returnTypes := map[string]QueryReturnType{
		"GetUser":    ReturnOne,
		"ListUsers":  ReturnMany,
		"DeleteUser": ReturnExec,
	}
	for _, q := range queries {
		expected := returnTypes[q.Name]
		if q.ReturnType != expected {
			t.Errorf("%s: expected ReturnType = %q, got %q", q.Name, expected, q.ReturnType)
		}
	}
}

func TestSerialize_InsertSelect_RoundTrip(t *testing.T) {
	ast := &AST{
		Kind:       InsertQuery,
		FromTable:  TableRef{Name: "target"},
		InsertCols: []Column{StringColumn{Table: "target", Name: "name"}},
		InsertSource: &AST{
			Kind:      SelectQuery,
			FromTable: TableRef{Name: "source"},
			SelectCols: []SelectExpr{
				{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "name"}}},
			},
		},
	}
	serialized := SerializeAST(ast)
	deserialized := DeserializeAST(serialized)

	if deserialized.Kind != InsertQuery {
		t.Errorf("expected Kind InsertQuery, got %s", deserialized.Kind)
	}
	if deserialized.InsertSource == nil {
		t.Fatal("expected InsertSource to be non-nil")
	}
	if deserialized.InsertSource.Kind != SelectQuery {
		t.Errorf("expected source Kind SelectQuery, got %s", deserialized.InsertSource.Kind)
	}
	if deserialized.InsertSource.FromTable.Name != "source" {
		t.Errorf("expected source table name 'source', got %s", deserialized.InsertSource.FromTable.Name)
	}
	if len(deserialized.InsertRows) != 0 {
		t.Errorf("expected no InsertRows, got %d", len(deserialized.InsertRows))
	}
}

func TestSerialize_InsertSelect_WithCTE_RoundTrip(t *testing.T) {
	ast := &AST{
		Kind:      InsertQuery,
		FromTable: TableRef{Name: "target"},
		CTEs: []CTE{
			{
				Name: "active_source",
				Query: &AST{
					Kind:      SelectQuery,
					FromTable: TableRef{Name: "source"},
					SelectCols: []SelectExpr{
						{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "name"}}},
					},
					Where: BinaryExpr{
						Left:  ColumnExpr{Column: StringColumn{Table: "source", Name: "active"}},
						Op:    OpEq,
						Right: LiteralExpr{Value: true},
					},
				},
			},
		},
		InsertCols: []Column{StringColumn{Table: "target", Name: "name"}},
		InsertSource: &AST{
			Kind:      SelectQuery,
			FromTable: TableRef{Name: "active_source"},
			SelectCols: []SelectExpr{
				{Expr: ColumnExpr{Column: StringColumn{Table: "active_source", Name: "name"}}},
			},
		},
	}

	serialized := SerializeAST(ast)
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var fromJSON SerializedAST
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	deserialized := DeserializeAST(&fromJSON)

	if deserialized.Kind != InsertQuery {
		t.Errorf("expected Kind InsertQuery, got %s", deserialized.Kind)
	}
	if len(deserialized.CTEs) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(deserialized.CTEs))
	}
	if deserialized.CTEs[0].Name != "active_source" {
		t.Errorf("expected CTE name 'active_source', got %s", deserialized.CTEs[0].Name)
	}
	if deserialized.CTEs[0].Query == nil {
		t.Fatal("expected CTE query to be non-nil")
	}
	if deserialized.CTEs[0].Query.Kind != SelectQuery {
		t.Errorf("expected CTE query Kind SelectQuery, got %s", deserialized.CTEs[0].Query.Kind)
	}
	if deserialized.InsertSource == nil {
		t.Fatal("expected InsertSource to be non-nil")
	}
	if deserialized.InsertSource.FromTable.Name != "active_source" {
		t.Errorf("expected InsertSource table 'active_source', got %s", deserialized.InsertSource.FromTable.Name)
	}
}

func TestSerialize_InsertSelect_JSON(t *testing.T) {
	ast := &AST{
		Kind:       InsertQuery,
		FromTable:  TableRef{Name: "target"},
		InsertCols: []Column{StringColumn{Table: "target", Name: "name"}},
		InsertSource: &AST{
			Kind:      SelectQuery,
			FromTable: TableRef{Name: "source"},
			SelectCols: []SelectExpr{
				{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "name"}}},
			},
		},
	}

	serialized := SerializeAST(ast)
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatalf("failed to unmarshal into map: %v", err)
	}

	if _, ok := raw["insert_source"]; !ok {
		t.Error("expected JSON to contain 'insert_source' key")
	}
	if _, ok := raw["insert_rows"]; ok {
		t.Error("expected JSON to NOT contain 'insert_rows' key")
	}
}

func TestSerialize_InsertSelect_WithParams_RoundTrip(t *testing.T) {
	ast := &AST{
		Kind:      InsertQuery,
		FromTable: TableRef{Name: "target"},
		InsertCols: []Column{
			StringColumn{Table: "target", Name: "name"},
			StringColumn{Table: "target", Name: "email"},
		},
		InsertSource: &AST{
			Kind:      SelectQuery,
			FromTable: TableRef{Name: "source"},
			SelectCols: []SelectExpr{
				{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "name"}}},
				{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "email"}}},
			},
			Where: BinaryExpr{
				Left:  ColumnExpr{Column: StringColumn{Table: "source", Name: "status"}},
				Op:    OpEq,
				Right: ParamExpr{Name: "status", GoType: "string"},
			},
		},
		Params: []ParamInfo{
			{Name: "status", GoType: "string"},
		},
	}

	serialized := SerializeAST(ast)
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var fromJSON SerializedAST
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	deserialized := DeserializeAST(&fromJSON)

	if deserialized.InsertSource == nil {
		t.Fatal("expected InsertSource to be non-nil")
	}
	if deserialized.InsertSource.Where == nil {
		t.Fatal("expected InsertSource.Where to be non-nil")
	}

	// Verify the param survived round-trip
	binExpr, ok := deserialized.InsertSource.Where.(BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", deserialized.InsertSource.Where)
	}
	paramExpr, ok := binExpr.Right.(ParamExpr)
	if !ok {
		t.Fatalf("expected ParamExpr on right side, got %T", binExpr.Right)
	}
	if paramExpr.Name != "status" {
		t.Errorf("expected param name 'status', got %s", paramExpr.Name)
	}
	if paramExpr.GoType != "string" {
		t.Errorf("expected param GoType 'string', got %s", paramExpr.GoType)
	}

	// Verify top-level Params survived
	if len(deserialized.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(deserialized.Params))
	}
	if deserialized.Params[0].Name != "status" {
		t.Errorf("expected param info name 'status', got %s", deserialized.Params[0].Name)
	}
}

func TestSerialize_InsertSelect_WithReturning_RoundTrip(t *testing.T) {
	ast := &AST{
		Kind:      InsertQuery,
		FromTable: TableRef{Name: "target"},
		InsertCols: []Column{
			StringColumn{Table: "target", Name: "name"},
		},
		InsertSource: &AST{
			Kind:      SelectQuery,
			FromTable: TableRef{Name: "source"},
			SelectCols: []SelectExpr{
				{Expr: ColumnExpr{Column: StringColumn{Table: "source", Name: "name"}}},
			},
		},
		Returning: []Column{
			Int64Column{Table: "target", Name: "id"},
			StringColumn{Table: "target", Name: "name"},
		},
	}

	serialized := SerializeAST(ast)
	jsonData, err := json.Marshal(serialized)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var fromJSON SerializedAST
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	deserialized := DeserializeAST(&fromJSON)

	if deserialized.InsertSource == nil {
		t.Fatal("expected InsertSource to be non-nil")
	}
	if deserialized.InsertSource.Kind != SelectQuery {
		t.Errorf("expected source Kind SelectQuery, got %s", deserialized.InsertSource.Kind)
	}
	if len(deserialized.Returning) != 2 {
		t.Fatalf("expected 2 Returning columns, got %d", len(deserialized.Returning))
	}
	if len(deserialized.InsertRows) != 0 {
		t.Errorf("expected no InsertRows, got %d", len(deserialized.InsertRows))
	}
}

func TestSerializeExpr_JSONAggWithFields(t *testing.T) {
	chapterTitle := StringColumn{Table: "chapters", Name: "title"}

	innerSubquery := &AST{
		Kind:      SelectQuery,
		FromTable: TableRef{Name: "chapters"},
		SelectCols: []SelectExpr{
			{Expr: JSONAggExpr{FieldName: "ch", Columns: []Column{chapterTitle}}},
		},
		Where: BinaryExpr{
			Left:  ColumnExpr{Column: Int64Column{Table: "chapters", Name: "book_id"}},
			Op:    OpEq,
			Right: ColumnExpr{Column: Int64Column{Table: "books", Name: "id"}},
		},
	}

	original := JSONAggExpr{
		FieldName: "books",
		Fields: []JSONAggField{
			{Key: "title", Column: StringColumn{Table: "books", Name: "title"}},
			{Key: "chapters", Expr: SubqueryExpr{Query: innerSubquery}},
		},
	}

	serialized := SerializeExpr(original)
	if serialized.Type != "json_agg" {
		t.Fatalf("expected type 'json_agg', got %q", serialized.Type)
	}
	if serialized.JSONAgg == nil {
		t.Fatal("expected JSONAgg to be non-nil")
	}
	if len(serialized.JSONAgg.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(serialized.JSONAgg.Fields))
	}
	if serialized.JSONAgg.Fields[0].Key != "title" {
		t.Errorf("expected first field key 'title', got %q", serialized.JSONAgg.Fields[0].Key)
	}
	if serialized.JSONAgg.Fields[1].Key != "chapters" {
		t.Errorf("expected second field key 'chapters', got %q", serialized.JSONAgg.Fields[1].Key)
	}
	if serialized.JSONAgg.Fields[1].Expr == nil {
		t.Fatal("expected second field to have an Expr (subquery)")
	}

	// Round-trip: deserialize and verify
	roundTripped := DeserializeExpr(serialized)
	jsonAgg, ok := roundTripped.(JSONAggExpr)
	if !ok {
		t.Fatalf("expected JSONAggExpr, got %T", roundTripped)
	}
	if jsonAgg.FieldName != "books" {
		t.Errorf("expected FieldName 'books', got %q", jsonAgg.FieldName)
	}
	if len(jsonAgg.Fields) != 2 {
		t.Fatalf("expected 2 fields after round-trip, got %d", len(jsonAgg.Fields))
	}
	if jsonAgg.Fields[0].Key != "title" {
		t.Errorf("expected field[0].Key = 'title', got %q", jsonAgg.Fields[0].Key)
	}
	if jsonAgg.Fields[1].Key != "chapters" {
		t.Errorf("expected field[1].Key = 'chapters', got %q", jsonAgg.Fields[1].Key)
	}
	if jsonAgg.Fields[1].Expr == nil {
		t.Error("expected field[1].Expr to be non-nil after round-trip")
	}
	sub, ok := jsonAgg.Fields[1].Expr.(SubqueryExpr)
	if !ok {
		t.Fatalf("expected SubqueryExpr, got %T", jsonAgg.Fields[1].Expr)
	}
	if sub.Query == nil {
		t.Error("expected subquery Query to be non-nil")
	}
}
