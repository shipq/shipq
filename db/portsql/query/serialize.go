package query

import (
	"encoding/json"
	"sort"
)

// SerializedQuery is the JSON-serializable representation of a registered query.
type SerializedQuery struct {
	Name       string          `json:"name"`
	ReturnType QueryReturnType `json:"return_type"` // "one", "many", "exec"
	AST        *SerializedAST  `json:"ast"`
}

// SerializedAST is the JSON-serializable representation of a query AST.
type SerializedAST struct {
	Kind       string                 `json:"kind"` // "select", "insert", "update", "delete"
	FromTable  SerializedTableRef     `json:"from_table"`
	Distinct   bool                   `json:"distinct,omitempty"`
	SelectCols []SerializedSelectExpr `json:"select_cols,omitempty"`
	Joins      []SerializedJoin       `json:"joins,omitempty"`
	Where      *SerializedExpr        `json:"where,omitempty"`
	GroupBy    []SerializedColumn     `json:"group_by,omitempty"`
	Having     *SerializedExpr        `json:"having,omitempty"`
	OrderBy    []SerializedOrderBy    `json:"order_by,omitempty"`
	Limit      *SerializedExpr        `json:"limit,omitempty"`
	Offset     *SerializedExpr        `json:"offset,omitempty"`

	// INSERT specific
	InsertCols []SerializedColumn `json:"insert_cols,omitempty"`
	InsertVals []SerializedExpr   `json:"insert_vals,omitempty"`
	Returning  []SerializedColumn `json:"returning,omitempty"`

	// UPDATE specific
	SetClauses []SerializedSetClause `json:"set_clauses,omitempty"`

	// CTEs
	CTEs []SerializedCTE `json:"ctes,omitempty"`

	// Set operations
	SetOp *SerializedSetOp `json:"set_op,omitempty"`

	// Collected parameters
	Params []SerializedParamInfo `json:"params,omitempty"`
}

// SerializedTableRef represents a table reference.
type SerializedTableRef struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

// SerializedSelectExpr represents a SELECT column or expression.
type SerializedSelectExpr struct {
	Expr  SerializedExpr `json:"expr"`
	Alias string         `json:"alias,omitempty"`
}

// SerializedJoin represents a JOIN clause.
type SerializedJoin struct {
	Type      string             `json:"type"` // "INNER", "LEFT", "RIGHT", "FULL"
	Table     SerializedTableRef `json:"table"`
	Condition SerializedExpr     `json:"condition"`
}

// SerializedOrderBy represents ORDER BY clause.
type SerializedOrderBy struct {
	Expr SerializedExpr `json:"expr"`
	Desc bool           `json:"desc,omitempty"`
}

// SerializedSetClause represents column = value in UPDATE.
type SerializedSetClause struct {
	Column SerializedColumn `json:"column"`
	Value  SerializedExpr   `json:"value"`
}

// SerializedCTE represents a Common Table Expression.
type SerializedCTE struct {
	Name    string         `json:"name"`
	Columns []string       `json:"columns,omitempty"`
	Query   *SerializedAST `json:"query"`
}

// SerializedSetOp represents a set operation (UNION, INTERSECT, EXCEPT).
type SerializedSetOp struct {
	Left  *SerializedAST `json:"left"`
	Op    string         `json:"op"` // "UNION", "UNION ALL", "INTERSECT", "EXCEPT"
	Right *SerializedAST `json:"right"`
}

// SerializedParamInfo tracks parameters for codegen.
type SerializedParamInfo struct {
	Name   string `json:"name"`
	GoType string `json:"go_type"`
}

// SerializedExpr represents any expression in JSON form.
// Uses a tagged union pattern for type discrimination.
type SerializedExpr struct {
	Type string `json:"type"` // "column", "param", "literal", "binary", "unary", "func", "list", "aggregate", "json_agg", "subquery", "exists"

	// Fields used depending on Type:
	Column    *SerializedColumn  `json:"column,omitempty"`
	Param     *SerializedParam   `json:"param,omitempty"`
	Literal   any                `json:"literal,omitempty"`
	Binary    *SerializedBinary  `json:"binary,omitempty"`
	Unary     *SerializedUnary   `json:"unary,omitempty"`
	Func      *SerializedFunc    `json:"func,omitempty"`
	List      []SerializedExpr   `json:"list,omitempty"`
	Aggregate *SerializedAgg     `json:"aggregate,omitempty"`
	JSONAgg   *SerializedJSONAgg `json:"json_agg,omitempty"`
	Subquery  *SerializedAST     `json:"subquery,omitempty"`
	Exists    *SerializedExists  `json:"exists,omitempty"`
}

// SerializedColumn represents a column reference.
type SerializedColumn struct {
	Table  string `json:"table"`
	Name   string `json:"name"`
	GoType string `json:"go_type,omitempty"`
}

// SerializedParam represents a named parameter.
type SerializedParam struct {
	Name   string `json:"name"`
	GoType string `json:"go_type"`
}

// SerializedBinary represents a binary operation.
type SerializedBinary struct {
	Left  SerializedExpr `json:"left"`
	Op    string         `json:"op"`
	Right SerializedExpr `json:"right"`
}

// SerializedUnary represents a unary operation.
type SerializedUnary struct {
	Op   string         `json:"op"`
	Expr SerializedExpr `json:"expr"`
}

// SerializedFunc represents a function call.
type SerializedFunc struct {
	Name string           `json:"name"`
	Args []SerializedExpr `json:"args,omitempty"`
}

// SerializedAgg represents an aggregate function.
type SerializedAgg struct {
	Func     string          `json:"func"` // "COUNT", "SUM", "AVG", "MIN", "MAX"
	Arg      *SerializedExpr `json:"arg,omitempty"`
	Distinct bool            `json:"distinct,omitempty"`
}

// SerializedJSONAgg represents JSON aggregation.
type SerializedJSONAgg struct {
	FieldName string             `json:"field_name"`
	Columns   []SerializedColumn `json:"columns"`
}

// SerializedExists represents EXISTS (subquery).
type SerializedExists struct {
	Subquery *SerializedAST `json:"subquery"`
	Negated  bool           `json:"negated,omitempty"`
}

// =============================================================================
// Serialization Functions
// =============================================================================

// SerializeAST converts an AST to its serializable form.
func SerializeAST(ast *AST) *SerializedAST {
	if ast == nil {
		return nil
	}

	s := &SerializedAST{
		Kind: string(ast.Kind),
		FromTable: SerializedTableRef{
			Name:  ast.FromTable.Name,
			Alias: ast.FromTable.Alias,
		},
		Distinct: ast.Distinct,
	}

	// Select columns
	if len(ast.SelectCols) > 0 {
		s.SelectCols = make([]SerializedSelectExpr, len(ast.SelectCols))
		for i, col := range ast.SelectCols {
			s.SelectCols[i] = SerializedSelectExpr{
				Expr:  SerializeExpr(col.Expr),
				Alias: col.Alias,
			}
		}
	}

	// Joins
	if len(ast.Joins) > 0 {
		s.Joins = make([]SerializedJoin, len(ast.Joins))
		for i, join := range ast.Joins {
			s.Joins[i] = SerializedJoin{
				Type: string(join.Type),
				Table: SerializedTableRef{
					Name:  join.Table.Name,
					Alias: join.Table.Alias,
				},
				Condition: SerializeExpr(join.Condition),
			}
		}
	}

	// Where
	if ast.Where != nil {
		expr := SerializeExpr(ast.Where)
		s.Where = &expr
	}

	// Group By
	if len(ast.GroupBy) > 0 {
		s.GroupBy = make([]SerializedColumn, len(ast.GroupBy))
		for i, col := range ast.GroupBy {
			s.GroupBy[i] = serializeColumn(col)
		}
	}

	// Having
	if ast.Having != nil {
		expr := SerializeExpr(ast.Having)
		s.Having = &expr
	}

	// Order By
	if len(ast.OrderBy) > 0 {
		s.OrderBy = make([]SerializedOrderBy, len(ast.OrderBy))
		for i, ob := range ast.OrderBy {
			s.OrderBy[i] = SerializedOrderBy{
				Expr: SerializeExpr(ob.Expr),
				Desc: ob.Desc,
			}
		}
	}

	// Limit
	if ast.Limit != nil {
		expr := SerializeExpr(ast.Limit)
		s.Limit = &expr
	}

	// Offset
	if ast.Offset != nil {
		expr := SerializeExpr(ast.Offset)
		s.Offset = &expr
	}

	// INSERT specific
	if len(ast.InsertCols) > 0 {
		s.InsertCols = make([]SerializedColumn, len(ast.InsertCols))
		for i, col := range ast.InsertCols {
			s.InsertCols[i] = serializeColumn(col)
		}
	}

	if len(ast.InsertVals) > 0 {
		s.InsertVals = make([]SerializedExpr, len(ast.InsertVals))
		for i, val := range ast.InsertVals {
			s.InsertVals[i] = SerializeExpr(val)
		}
	}

	if len(ast.Returning) > 0 {
		s.Returning = make([]SerializedColumn, len(ast.Returning))
		for i, col := range ast.Returning {
			s.Returning[i] = serializeColumn(col)
		}
	}

	// UPDATE specific
	if len(ast.SetClauses) > 0 {
		s.SetClauses = make([]SerializedSetClause, len(ast.SetClauses))
		for i, sc := range ast.SetClauses {
			s.SetClauses[i] = SerializedSetClause{
				Column: serializeColumn(sc.Column),
				Value:  SerializeExpr(sc.Value),
			}
		}
	}

	// CTEs
	if len(ast.CTEs) > 0 {
		s.CTEs = make([]SerializedCTE, len(ast.CTEs))
		for i, cte := range ast.CTEs {
			s.CTEs[i] = SerializedCTE{
				Name:    cte.Name,
				Columns: cte.Columns,
				Query:   SerializeAST(cte.Query),
			}
		}
	}

	// Set operations
	if ast.SetOp != nil {
		s.SetOp = &SerializedSetOp{
			Left:  SerializeAST(ast.SetOp.Left),
			Op:    string(ast.SetOp.Op),
			Right: SerializeAST(ast.SetOp.Right),
		}
	}

	// Params
	if len(ast.Params) > 0 {
		s.Params = make([]SerializedParamInfo, len(ast.Params))
		for i, p := range ast.Params {
			s.Params[i] = SerializedParamInfo{
				Name:   p.Name,
				GoType: p.GoType,
			}
		}
	}

	return s
}

// SerializeExpr converts an Expr to its serializable form.
func SerializeExpr(expr Expr) SerializedExpr {
	if expr == nil {
		return SerializedExpr{Type: "null"}
	}

	switch e := expr.(type) {
	case ColumnExpr:
		col := serializeColumn(e.Column)
		return SerializedExpr{
			Type:   "column",
			Column: &col,
		}

	case ParamExpr:
		return SerializedExpr{
			Type: "param",
			Param: &SerializedParam{
				Name:   e.Name,
				GoType: e.GoType,
			},
		}

	case LiteralExpr:
		return SerializedExpr{
			Type:    "literal",
			Literal: e.Value,
		}

	case BinaryExpr:
		left := SerializeExpr(e.Left)
		right := SerializeExpr(e.Right)
		return SerializedExpr{
			Type: "binary",
			Binary: &SerializedBinary{
				Left:  left,
				Op:    string(e.Op),
				Right: right,
			},
		}

	case UnaryExpr:
		inner := SerializeExpr(e.Expr)
		return SerializedExpr{
			Type: "unary",
			Unary: &SerializedUnary{
				Op:   string(e.Op),
				Expr: inner,
			},
		}

	case FuncExpr:
		var args []SerializedExpr
		if len(e.Args) > 0 {
			args = make([]SerializedExpr, len(e.Args))
			for i, arg := range e.Args {
				args[i] = SerializeExpr(arg)
			}
		}
		return SerializedExpr{
			Type: "func",
			Func: &SerializedFunc{
				Name: e.Name,
				Args: args,
			},
		}

	case ListExpr:
		items := make([]SerializedExpr, len(e.Values))
		for i, v := range e.Values {
			items[i] = SerializeExpr(v)
		}
		return SerializedExpr{
			Type: "list",
			List: items,
		}

	case AggregateExpr:
		var arg *SerializedExpr
		if e.Arg != nil {
			a := SerializeExpr(e.Arg)
			arg = &a
		}
		return SerializedExpr{
			Type: "aggregate",
			Aggregate: &SerializedAgg{
				Func:     string(e.Func),
				Arg:      arg,
				Distinct: e.Distinct,
			},
		}

	case JSONAggExpr:
		cols := make([]SerializedColumn, len(e.Columns))
		for i, col := range e.Columns {
			cols[i] = serializeColumn(col)
		}
		return SerializedExpr{
			Type: "json_agg",
			JSONAgg: &SerializedJSONAgg{
				FieldName: e.FieldName,
				Columns:   cols,
			},
		}

	case SubqueryExpr:
		return SerializedExpr{
			Type:     "subquery",
			Subquery: SerializeAST(e.Query),
		}

	case ExistsExpr:
		return SerializedExpr{
			Type: "exists",
			Exists: &SerializedExists{
				Subquery: SerializeAST(e.Subquery),
				Negated:  e.Negated,
			},
		}

	default:
		// Unknown expression type - serialize as literal with type info
		return SerializedExpr{
			Type:    "unknown",
			Literal: expr,
		}
	}
}

// serializeColumn converts a Column interface to SerializedColumn.
func serializeColumn(col Column) SerializedColumn {
	return SerializedColumn{
		Table:  col.TableName(),
		Name:   col.ColumnName(),
		GoType: col.GoType(),
	}
}

// SerializeQueries converts all registered queries to JSON bytes.
func SerializeQueries() ([]byte, error) {
	queries := GetRegisteredQueries()
	result := make([]SerializedQuery, 0, len(queries))

	// Collect and sort query names for deterministic output
	names := make([]string, 0, len(queries))
	for name := range queries {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		rq := queries[name]
		result = append(result, SerializedQuery{
			Name:       name,
			ReturnType: rq.ReturnType,
			AST:        SerializeAST(rq.AST),
		})
	}

	return json.MarshalIndent(result, "", "  ")
}

// =============================================================================
// Deserialization Functions (for round-trip testing)
// =============================================================================

// DeserializeAST converts a SerializedAST back to an AST.
// Note: This creates a simplified AST suitable for compilation, but may not
// perfectly reconstruct all original Column types (they become SimpleColumn).
func DeserializeAST(s *SerializedAST) *AST {
	if s == nil {
		return nil
	}

	ast := &AST{
		Kind:     QueryKind(s.Kind),
		Distinct: s.Distinct,
		FromTable: TableRef{
			Name:  s.FromTable.Name,
			Alias: s.FromTable.Alias,
		},
	}

	// Select columns
	if len(s.SelectCols) > 0 {
		ast.SelectCols = make([]SelectExpr, len(s.SelectCols))
		for i, col := range s.SelectCols {
			ast.SelectCols[i] = SelectExpr{
				Expr:  DeserializeExpr(col.Expr),
				Alias: col.Alias,
			}
		}
	}

	// Joins
	if len(s.Joins) > 0 {
		ast.Joins = make([]JoinClause, len(s.Joins))
		for i, join := range s.Joins {
			ast.Joins[i] = JoinClause{
				Type: JoinType(join.Type),
				Table: TableRef{
					Name:  join.Table.Name,
					Alias: join.Table.Alias,
				},
				Condition: DeserializeExpr(join.Condition),
			}
		}
	}

	// Where
	if s.Where != nil {
		ast.Where = DeserializeExpr(*s.Where)
	}

	// Group By
	if len(s.GroupBy) > 0 {
		ast.GroupBy = make([]Column, len(s.GroupBy))
		for i, col := range s.GroupBy {
			ast.GroupBy[i] = deserializeColumn(col)
		}
	}

	// Having
	if s.Having != nil {
		ast.Having = DeserializeExpr(*s.Having)
	}

	// Order By
	if len(s.OrderBy) > 0 {
		ast.OrderBy = make([]OrderByExpr, len(s.OrderBy))
		for i, ob := range s.OrderBy {
			ast.OrderBy[i] = OrderByExpr{
				Expr: DeserializeExpr(ob.Expr),
				Desc: ob.Desc,
			}
		}
	}

	// Limit
	if s.Limit != nil {
		ast.Limit = DeserializeExpr(*s.Limit)
	}

	// Offset
	if s.Offset != nil {
		ast.Offset = DeserializeExpr(*s.Offset)
	}

	// INSERT specific
	if len(s.InsertCols) > 0 {
		ast.InsertCols = make([]Column, len(s.InsertCols))
		for i, col := range s.InsertCols {
			ast.InsertCols[i] = deserializeColumn(col)
		}
	}

	if len(s.InsertVals) > 0 {
		ast.InsertVals = make([]Expr, len(s.InsertVals))
		for i, val := range s.InsertVals {
			ast.InsertVals[i] = DeserializeExpr(val)
		}
	}

	if len(s.Returning) > 0 {
		ast.Returning = make([]Column, len(s.Returning))
		for i, col := range s.Returning {
			ast.Returning[i] = deserializeColumn(col)
		}
	}

	// UPDATE specific
	if len(s.SetClauses) > 0 {
		ast.SetClauses = make([]SetClause, len(s.SetClauses))
		for i, sc := range s.SetClauses {
			ast.SetClauses[i] = SetClause{
				Column: deserializeColumn(sc.Column),
				Value:  DeserializeExpr(sc.Value),
			}
		}
	}

	// CTEs
	if len(s.CTEs) > 0 {
		ast.CTEs = make([]CTE, len(s.CTEs))
		for i, cte := range s.CTEs {
			ast.CTEs[i] = CTE{
				Name:    cte.Name,
				Columns: cte.Columns,
				Query:   DeserializeAST(cte.Query),
			}
		}
	}

	// Set operations
	if s.SetOp != nil {
		ast.SetOp = &SetOperation{
			Left:  DeserializeAST(s.SetOp.Left),
			Op:    SetOpKind(s.SetOp.Op),
			Right: DeserializeAST(s.SetOp.Right),
		}
	}

	// Params
	if len(s.Params) > 0 {
		ast.Params = make([]ParamInfo, len(s.Params))
		for i, p := range s.Params {
			ast.Params[i] = ParamInfo{
				Name:   p.Name,
				GoType: p.GoType,
			}
		}
	}

	return ast
}

// DeserializeExpr converts a SerializedExpr back to an Expr.
func DeserializeExpr(s SerializedExpr) Expr {
	switch s.Type {
	case "null", "":
		return nil

	case "column":
		if s.Column == nil {
			return nil
		}
		return ColumnExpr{Column: deserializeColumn(*s.Column)}

	case "param":
		if s.Param == nil {
			return nil
		}
		return ParamExpr{
			Name:   s.Param.Name,
			GoType: s.Param.GoType,
		}

	case "literal":
		return LiteralExpr{Value: s.Literal}

	case "binary":
		if s.Binary == nil {
			return nil
		}
		return BinaryExpr{
			Left:  DeserializeExpr(s.Binary.Left),
			Op:    BinaryOp(s.Binary.Op),
			Right: DeserializeExpr(s.Binary.Right),
		}

	case "unary":
		if s.Unary == nil {
			return nil
		}
		return UnaryExpr{
			Op:   UnaryOp(s.Unary.Op),
			Expr: DeserializeExpr(s.Unary.Expr),
		}

	case "func":
		if s.Func == nil {
			return nil
		}
		var args []Expr
		if len(s.Func.Args) > 0 {
			args = make([]Expr, len(s.Func.Args))
			for i, arg := range s.Func.Args {
				args[i] = DeserializeExpr(arg)
			}
		}
		return FuncExpr{
			Name: s.Func.Name,
			Args: args,
		}

	case "list":
		values := make([]Expr, len(s.List))
		for i, v := range s.List {
			values[i] = DeserializeExpr(v)
		}
		return ListExpr{Values: values}

	case "aggregate":
		if s.Aggregate == nil {
			return nil
		}
		var arg Expr
		if s.Aggregate.Arg != nil {
			arg = DeserializeExpr(*s.Aggregate.Arg)
		}
		return AggregateExpr{
			Func:     AggregateFunc(s.Aggregate.Func),
			Arg:      arg,
			Distinct: s.Aggregate.Distinct,
		}

	case "json_agg":
		if s.JSONAgg == nil {
			return nil
		}
		cols := make([]Column, len(s.JSONAgg.Columns))
		for i, col := range s.JSONAgg.Columns {
			cols[i] = deserializeColumn(col)
		}
		return JSONAggExpr{
			FieldName: s.JSONAgg.FieldName,
			Columns:   cols,
		}

	case "subquery":
		return SubqueryExpr{Query: DeserializeAST(s.Subquery)}

	case "exists":
		if s.Exists == nil {
			return nil
		}
		return ExistsExpr{
			Subquery: DeserializeAST(s.Exists.Subquery),
			Negated:  s.Exists.Negated,
		}

	default:
		// Unknown type - return as literal
		return LiteralExpr{Value: s.Literal}
	}
}

// SimpleColumn is a generic column used for deserialization.
// It preserves the column metadata without needing the specific typed column.
type SimpleColumn struct {
	Table_  string
	Name_   string
	GoType_ string
}

func (c SimpleColumn) TableName() string  { return c.Table_ }
func (c SimpleColumn) ColumnName() string { return c.Name_ }
func (c SimpleColumn) IsNullable() bool   { return len(c.GoType_) > 0 && c.GoType_[0] == '*' }
func (c SimpleColumn) GoType() string     { return c.GoType_ }

// Verify SimpleColumn implements Column
var _ Column = SimpleColumn{}

// deserializeColumn converts a SerializedColumn to a Column interface.
func deserializeColumn(s SerializedColumn) Column {
	return SimpleColumn{
		Table_:  s.Table,
		Name_:   s.Name,
		GoType_: s.GoType,
	}
}
