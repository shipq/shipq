package query

import (
	"encoding/json"
	"testing"

	"github.com/shipq/shipq/proptest"
)

// NewGenerator creates a proptest generator (wrapper for consistency)
func NewGenerator(seed int64) *proptest.Generator {
	return proptest.New(seed)
}

// TestProperty_SerializeDeserialize_Roundtrip tests that serializing and
// deserializing an AST preserves the essential structure.
func TestProperty_SerializeDeserialize_Roundtrip(t *testing.T) {
	gen := NewGenerator(12345)

	for i := 0; i < 100; i++ {
		// Generate random AST
		ast := generateRandomAST(gen)

		// Serialize
		serialized := SerializeAST(ast)

		// Convert to JSON and back (this is what happens in practice)
		jsonData, err := json.Marshal(serialized)
		if err != nil {
			t.Fatalf("iteration %d: failed to marshal to JSON: %v", i, err)
		}

		var fromJSON SerializedAST
		if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
			t.Fatalf("iteration %d: failed to unmarshal from JSON: %v", i, err)
		}

		// Deserialize
		deserialized := DeserializeAST(&fromJSON)

		// Verify key properties are preserved
		if deserialized.Kind != ast.Kind {
			t.Errorf("iteration %d: Kind mismatch: got %q, want %q", i, deserialized.Kind, ast.Kind)
		}
		if deserialized.Distinct != ast.Distinct {
			t.Errorf("iteration %d: Distinct mismatch: got %v, want %v", i, deserialized.Distinct, ast.Distinct)
		}
		if deserialized.FromTable.Name != ast.FromTable.Name {
			t.Errorf("iteration %d: FromTable.Name mismatch: got %q, want %q", i, deserialized.FromTable.Name, ast.FromTable.Name)
		}
		if deserialized.FromTable.Alias != ast.FromTable.Alias {
			t.Errorf("iteration %d: FromTable.Alias mismatch: got %q, want %q", i, deserialized.FromTable.Alias, ast.FromTable.Alias)
		}
		if len(deserialized.SelectCols) != len(ast.SelectCols) {
			t.Errorf("iteration %d: SelectCols length mismatch: got %d, want %d", i, len(deserialized.SelectCols), len(ast.SelectCols))
		}
		if len(deserialized.Joins) != len(ast.Joins) {
			t.Errorf("iteration %d: Joins length mismatch: got %d, want %d", i, len(deserialized.Joins), len(ast.Joins))
		}
		if len(deserialized.OrderBy) != len(ast.OrderBy) {
			t.Errorf("iteration %d: OrderBy length mismatch: got %d, want %d", i, len(deserialized.OrderBy), len(ast.OrderBy))
		}
		if len(deserialized.Params) != len(ast.Params) {
			t.Errorf("iteration %d: Params length mismatch: got %d, want %d", i, len(deserialized.Params), len(ast.Params))
		}

		// Verify Where presence matches
		if (deserialized.Where == nil) != (ast.Where == nil) {
			t.Errorf("iteration %d: Where presence mismatch", i)
		}
	}
}

// TestProperty_SerializeExpr_Roundtrip tests that all expression types
// can be serialized and deserialized correctly.
func TestProperty_SerializeExpr_Roundtrip(t *testing.T) {
	gen := NewGenerator(54321)

	for i := 0; i < 200; i++ {
		// Generate random expression
		expr := generateRandomExpr(gen, 3) // max depth of 3

		// Serialize
		serialized := SerializeExpr(expr)

		// Convert to JSON and back
		jsonData, err := json.Marshal(serialized)
		if err != nil {
			t.Fatalf("iteration %d: failed to marshal to JSON: %v", i, err)
		}

		var fromJSON SerializedExpr
		if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
			t.Fatalf("iteration %d: failed to unmarshal from JSON: %v", i, err)
		}

		// Deserialize
		deserialized := DeserializeExpr(fromJSON)

		// Re-serialize and compare JSON
		reserialized := SerializeExpr(deserialized)
		reserializedJSON, err := json.Marshal(reserialized)
		if err != nil {
			t.Fatalf("iteration %d: failed to re-marshal: %v", i, err)
		}

		if string(jsonData) != string(reserializedJSON) {
			t.Errorf("iteration %d: JSON mismatch after roundtrip\noriginal:     %s\nreserialized: %s",
				i, string(jsonData), string(reserializedJSON))
		}
	}
}

// TestProperty_ParamOrder_Preserved tests that parameter order is preserved
// through serialization.
func TestProperty_ParamOrder_Preserved(t *testing.T) {
	gen := NewGenerator(99999)

	for i := 0; i < 50; i++ {
		// Generate random params
		numParams := gen.IntRange(1, 10)
		params := make([]ParamInfo, numParams)
		for j := 0; j < numParams; j++ {
			params[j] = ParamInfo{
				Name:   gen.Identifier(20),
				GoType: randomGoType(gen),
			}
		}

		ast := &AST{
			Kind:      SelectQuery,
			FromTable: TableRef{Name: gen.Identifier(20)},
			SelectCols: []SelectExpr{
				{Expr: ColumnExpr{Column: StringColumn{Table: "t", Name: "c"}}},
			},
			Params: params,
		}

		// Serialize and deserialize
		serialized := SerializeAST(ast)
		jsonData, _ := json.Marshal(serialized)
		var fromJSON SerializedAST
		json.Unmarshal(jsonData, &fromJSON)
		deserialized := DeserializeAST(&fromJSON)

		// Verify param order is preserved
		if len(deserialized.Params) != len(params) {
			t.Errorf("iteration %d: params length mismatch", i)
			continue
		}

		for j, p := range params {
			if deserialized.Params[j].Name != p.Name {
				t.Errorf("iteration %d: param %d name mismatch: got %q, want %q",
					i, j, deserialized.Params[j].Name, p.Name)
			}
			if deserialized.Params[j].GoType != p.GoType {
				t.Errorf("iteration %d: param %d type mismatch: got %q, want %q",
					i, j, deserialized.Params[j].GoType, p.GoType)
			}
		}
	}
}

// TestProperty_SerializedQueries_Deterministic tests that SerializeQueries
// produces deterministic output (sorted by name).
func TestProperty_SerializedQueries_Deterministic(t *testing.T) {
	gen := NewGenerator(11111)

	for i := 0; i < 20; i++ {
		ClearRegistry()

		// Register random queries
		numQueries := gen.IntRange(3, 10)
		for j := 0; j < numQueries; j++ {
			name := gen.Identifier(10) + "_" + gen.Identifier(10) // Make names more unique
			ast := &AST{
				Kind:      SelectQuery,
				FromTable: TableRef{Name: gen.Identifier(20)},
				SelectCols: []SelectExpr{
					{Expr: ColumnExpr{Column: StringColumn{Table: "t", Name: "c"}}},
				},
			}
			// Use TryDefineOne to avoid panics on duplicate names
			TryDefineOne(name, ast)
		}

		// Serialize twice
		json1, err1 := SerializeQueries()
		json2, err2 := SerializeQueries()

		if err1 != nil || err2 != nil {
			t.Fatalf("iteration %d: serialization failed: %v, %v", i, err1, err2)
		}

		// Output should be identical
		if string(json1) != string(json2) {
			t.Errorf("iteration %d: non-deterministic output", i)
		}

		// Verify queries are sorted by name
		var queries []SerializedQuery
		json.Unmarshal(json1, &queries)
		for j := 1; j < len(queries); j++ {
			if queries[j-1].Name > queries[j].Name {
				t.Errorf("iteration %d: queries not sorted: %q > %q",
					i, queries[j-1].Name, queries[j].Name)
			}
		}
	}

	ClearRegistry()
}

// =============================================================================
// Random AST Generators
// =============================================================================

func generateRandomAST(gen *proptest.Generator) *AST {
	kinds := []QueryKind{SelectQuery, InsertQuery, UpdateQuery, DeleteQuery}
	kind := kinds[gen.IntRange(0, len(kinds)-1)]

	ast := &AST{
		Kind:     kind,
		Distinct: gen.Bool(),
		FromTable: TableRef{
			Name:  gen.Identifier(20),
			Alias: maybeString(gen, 0.3),
		},
	}

	// Add select columns for SELECT queries
	if kind == SelectQuery {
		numCols := gen.IntRange(1, 5)
		ast.SelectCols = make([]SelectExpr, numCols)
		for i := 0; i < numCols; i++ {
			ast.SelectCols[i] = SelectExpr{
				Expr:  generateRandomExpr(gen, 2),
				Alias: maybeString(gen, 0.2),
			}
		}
	}

	// Add joins for SELECT queries
	if kind == SelectQuery && gen.Float64() < 0.3 {
		numJoins := gen.IntRange(1, 2)
		ast.Joins = make([]JoinClause, numJoins)
		for i := 0; i < numJoins; i++ {
			ast.Joins[i] = JoinClause{
				Type: randomJoinType(gen),
				Table: TableRef{
					Name:  gen.Identifier(20),
					Alias: maybeString(gen, 0.5),
				},
				Condition: generateRandomExpr(gen, 2),
			}
		}
	}

	// Add WHERE clause
	if gen.Float64() < 0.7 {
		ast.Where = generateRandomExpr(gen, 3)
	}

	// Add ORDER BY for SELECT
	if kind == SelectQuery && gen.Float64() < 0.4 {
		numOrderBy := gen.IntRange(1, 3)
		ast.OrderBy = make([]OrderByExpr, numOrderBy)
		for i := 0; i < numOrderBy; i++ {
			ast.OrderBy[i] = OrderByExpr{
				Expr: generateRandomExpr(gen, 1),
				Desc: gen.Bool(),
			}
		}
	}

	// Add LIMIT/OFFSET for SELECT
	if kind == SelectQuery {
		if gen.Float64() < 0.3 {
			ast.Limit = LiteralExpr{Value: gen.IntRange(1, 100)}
		}
		if gen.Float64() < 0.2 {
			ast.Offset = LiteralExpr{Value: gen.IntRange(0, 50)}
		}
	}

	// Add params
	numParams := gen.IntRange(0, 5)
	ast.Params = make([]ParamInfo, numParams)
	for i := 0; i < numParams; i++ {
		ast.Params[i] = ParamInfo{
			Name:   gen.Identifier(20),
			GoType: randomGoType(gen),
		}
	}

	// Add INSERT-specific fields
	if kind == InsertQuery {
		numCols := gen.IntRange(1, 4)
		ast.InsertCols = make([]Column, numCols)
		ast.InsertVals = make([]Expr, numCols)
		for i := 0; i < numCols; i++ {
			ast.InsertCols[i] = StringColumn{Table: ast.FromTable.Name, Name: gen.Identifier(20)}
			ast.InsertVals[i] = generateRandomExpr(gen, 1)
		}
	}

	// Add UPDATE-specific fields
	if kind == UpdateQuery {
		numSets := gen.IntRange(1, 3)
		ast.SetClauses = make([]SetClause, numSets)
		for i := 0; i < numSets; i++ {
			ast.SetClauses[i] = SetClause{
				Column: StringColumn{Table: ast.FromTable.Name, Name: gen.Identifier(20)},
				Value:  generateRandomExpr(gen, 1),
			}
		}
	}

	return ast
}

func generateRandomExpr(gen *proptest.Generator, maxDepth int) Expr {
	if maxDepth <= 0 {
		// At max depth, only generate leaf expressions
		return generateLeafExpr(gen)
	}

	// Choose expression type
	exprTypes := []int{0, 1, 2, 3, 4, 5, 6} // column, param, literal, binary, unary, func, aggregate
	exprType := exprTypes[gen.IntRange(0, len(exprTypes)-1)]

	switch exprType {
	case 0: // Column
		return ColumnExpr{Column: randomColumn(gen)}
	case 1: // Param
		return ParamExpr{Name: gen.Identifier(20), GoType: randomGoType(gen)}
	case 2: // Literal
		return randomLiteral(gen)
	case 3: // Binary
		return BinaryExpr{
			Left:  generateRandomExpr(gen, maxDepth-1),
			Op:    randomBinaryOp(gen),
			Right: generateRandomExpr(gen, maxDepth-1),
		}
	case 4: // Unary
		return UnaryExpr{
			Op:   randomUnaryOp(gen),
			Expr: generateRandomExpr(gen, maxDepth-1),
		}
	case 5: // Func
		numArgs := gen.IntRange(0, 3)
		args := make([]Expr, numArgs)
		for i := 0; i < numArgs; i++ {
			args[i] = generateRandomExpr(gen, maxDepth-1)
		}
		return FuncExpr{Name: randomFuncName(gen), Args: args}
	case 6: // Aggregate
		var arg Expr
		if gen.Bool() {
			arg = generateRandomExpr(gen, maxDepth-1)
		}
		return AggregateExpr{
			Func:     randomAggFunc(gen),
			Arg:      arg,
			Distinct: gen.Bool(),
		}
	default:
		return generateLeafExpr(gen)
	}
}

func generateLeafExpr(gen *proptest.Generator) Expr {
	switch gen.IntRange(0, 2) {
	case 0:
		return ColumnExpr{Column: randomColumn(gen)}
	case 1:
		return ParamExpr{Name: gen.Identifier(20), GoType: randomGoType(gen)}
	default:
		return randomLiteral(gen)
	}
}

func randomColumn(gen *proptest.Generator) Column {
	tableName := gen.Identifier(20)
	colName := gen.Identifier(20)

	switch gen.IntRange(0, 5) {
	case 0:
		return Int64Column{Table: tableName, Name: colName}
	case 1:
		return StringColumn{Table: tableName, Name: colName}
	case 2:
		return BoolColumn{Table: tableName, Name: colName}
	case 3:
		return Float64Column{Table: tableName, Name: colName}
	case 4:
		return TimeColumn{Table: tableName, Name: colName}
	default:
		return NullStringColumn{Table: tableName, Name: colName}
	}
}

func randomLiteral(gen *proptest.Generator) LiteralExpr {
	switch gen.IntRange(0, 4) {
	case 0:
		return LiteralExpr{Value: gen.IntRange(-1000, 1000)}
	case 1:
		return LiteralExpr{Value: gen.Identifier(20)}
	case 2:
		return LiteralExpr{Value: gen.Bool()}
	case 3:
		return LiteralExpr{Value: gen.Float64()}
	default:
		return LiteralExpr{Value: nil}
	}
}

func randomGoType(gen *proptest.Generator) string {
	types := []string{"string", "int64", "int", "bool", "float64", "time.Time", "*string", "*int64"}
	return types[gen.IntRange(0, len(types)-1)]
}

func randomBinaryOp(gen *proptest.Generator) BinaryOp {
	ops := []BinaryOp{OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpAnd, OpOr, OpLike}
	return ops[gen.IntRange(0, len(ops)-1)]
}

func randomUnaryOp(gen *proptest.Generator) UnaryOp {
	ops := []UnaryOp{OpNot, OpIsNull, OpNotNull}
	return ops[gen.IntRange(0, len(ops)-1)]
}

func randomFuncName(gen *proptest.Generator) string {
	funcs := []string{"NOW", "LOWER", "UPPER", "TRIM", "COALESCE", "LENGTH"}
	return funcs[gen.IntRange(0, len(funcs)-1)]
}

func randomAggFunc(gen *proptest.Generator) AggregateFunc {
	funcs := []AggregateFunc{AggCount, AggSum, AggAvg, AggMin, AggMax}
	return funcs[gen.IntRange(0, len(funcs)-1)]
}

func randomJoinType(gen *proptest.Generator) JoinType {
	types := []JoinType{InnerJoin, LeftJoin, RightJoin, FullJoin}
	return types[gen.IntRange(0, len(types)-1)]
}

func maybeString(gen *proptest.Generator, probability float64) string {
	if gen.Float64() < probability {
		return gen.Identifier(20)
	}
	return ""
}
