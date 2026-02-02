package query

import (
	"encoding/json"
	"fmt"
)

// =============================================================================
// JSON Serialization for Query AST
// =============================================================================

// ASTJson is the JSON-serializable form of AST.
type ASTJson struct {
	Kind       QueryKind         `json:"kind"`
	Distinct   bool              `json:"distinct,omitempty"`
	FromTable  TableRef          `json:"from_table"`
	Joins      []JoinClauseJson  `json:"joins,omitempty"`
	SelectCols []SelectExprJson  `json:"select_cols,omitempty"`
	Where      *ExprJson         `json:"where,omitempty"`
	GroupBy    []ColumnJson      `json:"group_by,omitempty"`
	Having     *ExprJson         `json:"having,omitempty"`
	OrderBy    []OrderByExprJson `json:"order_by,omitempty"`
	Limit      *ExprJson         `json:"limit,omitempty"`
	Offset     *ExprJson         `json:"offset,omitempty"`

	InsertCols []ColumnJson `json:"insert_cols,omitempty"`
	InsertVals []*ExprJson  `json:"insert_vals,omitempty"`
	Returning  []ColumnJson `json:"returning,omitempty"`

	SetClauses []SetClauseJson `json:"set_clauses,omitempty"`

	SetOp *SetOperationJson `json:"set_op,omitempty"`
	CTEs  []CTEJson         `json:"ctes,omitempty"`

	Params []ParamInfo `json:"params,omitempty"`
}

// JoinClauseJson is the JSON-serializable form of JoinClause.
type JoinClauseJson struct {
	Type      JoinType  `json:"type"`
	Table     TableRef  `json:"table"`
	Condition *ExprJson `json:"condition"`
}

// SelectExprJson is the JSON-serializable form of SelectExpr.
type SelectExprJson struct {
	Expr  *ExprJson `json:"expr"`
	Alias string    `json:"alias,omitempty"`
}

// OrderByExprJson is the JSON-serializable form of OrderByExpr.
type OrderByExprJson struct {
	Expr *ExprJson `json:"expr"`
	Desc bool      `json:"desc,omitempty"`
}

// SetClauseJson is the JSON-serializable form of SetClause.
type SetClauseJson struct {
	Column *ColumnJson `json:"column"`
	Value  *ExprJson   `json:"value"`
}

// SetOperationJson is the JSON-serializable form of SetOperation.
type SetOperationJson struct {
	Left  *ASTJson  `json:"left"`
	Op    SetOpKind `json:"op"`
	Right *ASTJson  `json:"right"`
}

// CTEJson is the JSON-serializable form of CTE.
type CTEJson struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns,omitempty"`
	Query   *ASTJson `json:"query"`
}

// ColumnJson is the JSON-serializable form of a Column.
type ColumnJson struct {
	Table    string `json:"table"`
	Name     string `json:"name"`
	GoType   string `json:"go_type"`
	Nullable bool   `json:"nullable,omitempty"`
}

// ExprJson is the JSON-serializable form of an expression.
type ExprJson struct {
	Type string `json:"type"` // "column", "param", "literal", "binary", "unary", "func", "list", "aggregate", "subquery", "exists", "json_agg"

	// For ColumnExpr
	Column *ColumnJson `json:"column,omitempty"`

	// For ParamExpr
	ParamName   string `json:"param_name,omitempty"`
	ParamGoType string `json:"param_go_type,omitempty"`

	// For LiteralExpr
	LiteralValue any    `json:"literal_value,omitempty"`
	LiteralType  string `json:"literal_type,omitempty"`

	// For BinaryExpr
	Left  *ExprJson `json:"left,omitempty"`
	Op    string    `json:"op,omitempty"`
	Right *ExprJson `json:"right,omitempty"`

	// For UnaryExpr
	UnaryOp string    `json:"unary_op,omitempty"`
	Expr    *ExprJson `json:"expr,omitempty"`

	// For FuncExpr
	FuncName string      `json:"func_name,omitempty"`
	FuncArgs []*ExprJson `json:"func_args,omitempty"`

	// For ListExpr
	ListValues []*ExprJson `json:"list_values,omitempty"`

	// For AggregateExpr
	AggFunc     string    `json:"agg_func,omitempty"`
	AggArg      *ExprJson `json:"agg_arg,omitempty"`
	AggDistinct bool      `json:"agg_distinct,omitempty"`

	// For SubqueryExpr and ExistsExpr
	Subquery *ASTJson `json:"subquery,omitempty"`
	Negated  bool     `json:"negated,omitempty"`

	// For JSONAggExpr
	JSONFieldName string        `json:"json_field_name,omitempty"`
	JSONColumns   []*ColumnJson `json:"json_columns,omitempty"`
}

// =============================================================================
// AST -> JSON conversion
// =============================================================================

// ToJSON converts an AST to its JSON representation.
func (ast *AST) ToJSON() (*ASTJson, error) {
	if ast == nil {
		return nil, nil
	}

	j := &ASTJson{
		Kind:      ast.Kind,
		Distinct:  ast.Distinct,
		FromTable: ast.FromTable,
		Params:    ast.Params,
	}

	// Convert joins
	for _, join := range ast.Joins {
		condJson, err := exprToJSON(join.Condition)
		if err != nil {
			return nil, err
		}
		j.Joins = append(j.Joins, JoinClauseJson{
			Type:      join.Type,
			Table:     join.Table,
			Condition: condJson,
		})
	}

	// Convert select columns
	for _, sel := range ast.SelectCols {
		exprJson, err := exprToJSON(sel.Expr)
		if err != nil {
			return nil, err
		}
		j.SelectCols = append(j.SelectCols, SelectExprJson{
			Expr:  exprJson,
			Alias: sel.Alias,
		})
	}

	// Convert WHERE
	if ast.Where != nil {
		whereJson, err := exprToJSON(ast.Where)
		if err != nil {
			return nil, err
		}
		j.Where = whereJson
	}

	// Convert GROUP BY
	for _, col := range ast.GroupBy {
		j.GroupBy = append(j.GroupBy, columnToJSON(col))
	}

	// Convert HAVING
	if ast.Having != nil {
		havingJson, err := exprToJSON(ast.Having)
		if err != nil {
			return nil, err
		}
		j.Having = havingJson
	}

	// Convert ORDER BY
	for _, ob := range ast.OrderBy {
		exprJson, err := exprToJSON(ob.Expr)
		if err != nil {
			return nil, err
		}
		j.OrderBy = append(j.OrderBy, OrderByExprJson{
			Expr: exprJson,
			Desc: ob.Desc,
		})
	}

	// Convert LIMIT
	if ast.Limit != nil {
		limitJson, err := exprToJSON(ast.Limit)
		if err != nil {
			return nil, err
		}
		j.Limit = limitJson
	}

	// Convert OFFSET
	if ast.Offset != nil {
		offsetJson, err := exprToJSON(ast.Offset)
		if err != nil {
			return nil, err
		}
		j.Offset = offsetJson
	}

	// Convert INSERT columns
	for _, col := range ast.InsertCols {
		j.InsertCols = append(j.InsertCols, columnToJSON(col))
	}

	// Convert INSERT values
	for _, val := range ast.InsertVals {
		valJson, err := exprToJSON(val)
		if err != nil {
			return nil, err
		}
		j.InsertVals = append(j.InsertVals, valJson)
	}

	// Convert RETURNING
	for _, col := range ast.Returning {
		j.Returning = append(j.Returning, columnToJSON(col))
	}

	// Convert SET clauses
	for _, set := range ast.SetClauses {
		colJson := columnToJSON(set.Column)
		valJson, err := exprToJSON(set.Value)
		if err != nil {
			return nil, err
		}
		j.SetClauses = append(j.SetClauses, SetClauseJson{
			Column: &colJson,
			Value:  valJson,
		})
	}

	// Convert SetOp
	if ast.SetOp != nil {
		leftJson, err := ast.SetOp.Left.ToJSON()
		if err != nil {
			return nil, err
		}
		rightJson, err := ast.SetOp.Right.ToJSON()
		if err != nil {
			return nil, err
		}
		j.SetOp = &SetOperationJson{
			Left:  leftJson,
			Op:    ast.SetOp.Op,
			Right: rightJson,
		}
	}

	// Convert CTEs
	for _, cte := range ast.CTEs {
		queryJson, err := cte.Query.ToJSON()
		if err != nil {
			return nil, err
		}
		j.CTEs = append(j.CTEs, CTEJson{
			Name:    cte.Name,
			Columns: cte.Columns,
			Query:   queryJson,
		})
	}

	return j, nil
}

// columnToJSON converts a Column to ColumnJson.
func columnToJSON(col Column) ColumnJson {
	return ColumnJson{
		Table:    col.TableName(),
		Name:     col.ColumnName(),
		GoType:   col.GoType(),
		Nullable: col.IsNullable(),
	}
}

// exprToJSON converts an Expr to ExprJson.
func exprToJSON(expr Expr) (*ExprJson, error) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case ColumnExpr:
		col := columnToJSON(e.Column)
		return &ExprJson{
			Type:   "column",
			Column: &col,
		}, nil

	case ParamExpr:
		return &ExprJson{
			Type:        "param",
			ParamName:   e.Name,
			ParamGoType: e.GoType,
		}, nil

	case LiteralExpr:
		return &ExprJson{
			Type:         "literal",
			LiteralValue: e.Value,
			LiteralType:  fmt.Sprintf("%T", e.Value),
		}, nil

	case BinaryExpr:
		left, err := exprToJSON(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := exprToJSON(e.Right)
		if err != nil {
			return nil, err
		}
		return &ExprJson{
			Type:  "binary",
			Left:  left,
			Op:    string(e.Op),
			Right: right,
		}, nil

	case UnaryExpr:
		inner, err := exprToJSON(e.Expr)
		if err != nil {
			return nil, err
		}
		return &ExprJson{
			Type:    "unary",
			UnaryOp: string(e.Op),
			Expr:    inner,
		}, nil

	case FuncExpr:
		var args []*ExprJson
		for _, arg := range e.Args {
			argJson, err := exprToJSON(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, argJson)
		}
		return &ExprJson{
			Type:     "func",
			FuncName: e.Name,
			FuncArgs: args,
		}, nil

	case ListExpr:
		var values []*ExprJson
		for _, val := range e.Values {
			valJson, err := exprToJSON(val)
			if err != nil {
				return nil, err
			}
			values = append(values, valJson)
		}
		return &ExprJson{
			Type:       "list",
			ListValues: values,
		}, nil

	case AggregateExpr:
		var argJson *ExprJson
		if e.Arg != nil {
			var err error
			argJson, err = exprToJSON(e.Arg)
			if err != nil {
				return nil, err
			}
		}
		return &ExprJson{
			Type:        "aggregate",
			AggFunc:     string(e.Func),
			AggArg:      argJson,
			AggDistinct: e.Distinct,
		}, nil

	case SubqueryExpr:
		subJson, err := e.Query.ToJSON()
		if err != nil {
			return nil, err
		}
		return &ExprJson{
			Type:     "subquery",
			Subquery: subJson,
		}, nil

	case ExistsExpr:
		subJson, err := e.Subquery.ToJSON()
		if err != nil {
			return nil, err
		}
		return &ExprJson{
			Type:     "exists",
			Subquery: subJson,
			Negated:  e.Negated,
		}, nil

	case JSONAggExpr:
		var cols []*ColumnJson
		for _, col := range e.Columns {
			colJson := columnToJSON(col)
			cols = append(cols, &colJson)
		}
		return &ExprJson{
			Type:          "json_agg",
			JSONFieldName: e.FieldName,
			JSONColumns:   cols,
		}, nil

	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

// =============================================================================
// JSON -> AST conversion
// =============================================================================

// FromJSON converts an ASTJson back to an AST.
func (j *ASTJson) FromJSON() (*AST, error) {
	if j == nil {
		return nil, nil
	}

	ast := &AST{
		Kind:      j.Kind,
		Distinct:  j.Distinct,
		FromTable: j.FromTable,
		Params:    j.Params,
	}

	// Convert joins
	for _, join := range j.Joins {
		cond, err := join.Condition.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.Joins = append(ast.Joins, JoinClause{
			Type:      join.Type,
			Table:     join.Table,
			Condition: cond,
		})
	}

	// Convert select columns
	for _, sel := range j.SelectCols {
		expr, err := sel.Expr.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.SelectCols = append(ast.SelectCols, SelectExpr{
			Expr:  expr,
			Alias: sel.Alias,
		})
	}

	// Convert WHERE
	if j.Where != nil {
		where, err := j.Where.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.Where = where
	}

	// Convert GROUP BY
	for _, col := range j.GroupBy {
		ast.GroupBy = append(ast.GroupBy, col.ToColumn())
	}

	// Convert HAVING
	if j.Having != nil {
		having, err := j.Having.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.Having = having
	}

	// Convert ORDER BY
	for _, ob := range j.OrderBy {
		expr, err := ob.Expr.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.OrderBy = append(ast.OrderBy, OrderByExpr{
			Expr: expr,
			Desc: ob.Desc,
		})
	}

	// Convert LIMIT
	if j.Limit != nil {
		limit, err := j.Limit.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.Limit = limit
	}

	// Convert OFFSET
	if j.Offset != nil {
		offset, err := j.Offset.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.Offset = offset
	}

	// Convert INSERT columns
	for _, col := range j.InsertCols {
		ast.InsertCols = append(ast.InsertCols, col.ToColumn())
	}

	// Convert INSERT values
	for _, val := range j.InsertVals {
		valExpr, err := val.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.InsertVals = append(ast.InsertVals, valExpr)
	}

	// Convert RETURNING
	for _, col := range j.Returning {
		ast.Returning = append(ast.Returning, col.ToColumn())
	}

	// Convert SET clauses
	for _, set := range j.SetClauses {
		val, err := set.Value.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.SetClauses = append(ast.SetClauses, SetClause{
			Column: set.Column.ToColumn(),
			Value:  val,
		})
	}

	// Convert SetOp
	if j.SetOp != nil {
		left, err := j.SetOp.Left.FromJSON()
		if err != nil {
			return nil, err
		}
		right, err := j.SetOp.Right.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.SetOp = &SetOperation{
			Left:  left,
			Op:    j.SetOp.Op,
			Right: right,
		}
	}

	// Convert CTEs
	for _, cte := range j.CTEs {
		query, err := cte.Query.FromJSON()
		if err != nil {
			return nil, err
		}
		ast.CTEs = append(ast.CTEs, CTE{
			Name:    cte.Name,
			Columns: cte.Columns,
			Query:   query,
		})
	}

	return ast, nil
}

// ToColumn converts ColumnJson back to a Column.
func (c ColumnJson) ToColumn() Column {
	// Return appropriate column type based on GoType and Nullable
	if c.Nullable {
		switch c.GoType {
		case "*int32":
			return NullInt32Column{Table: c.Table, Name: c.Name}
		case "*int64":
			return NullInt64Column{Table: c.Table, Name: c.Name}
		case "*float64":
			return NullFloat64Column{Table: c.Table, Name: c.Name}
		case "*bool":
			return NullBoolColumn{Table: c.Table, Name: c.Name}
		case "*string":
			return NullStringColumn{Table: c.Table, Name: c.Name}
		case "*time.Time":
			return NullTimeColumn{Table: c.Table, Name: c.Name}
		case "json.RawMessage":
			return NullJSONColumn{Table: c.Table, Name: c.Name}
		default:
			return NullStringColumn{Table: c.Table, Name: c.Name}
		}
	}

	switch c.GoType {
	case "int32":
		return Int32Column{Table: c.Table, Name: c.Name}
	case "int64":
		return Int64Column{Table: c.Table, Name: c.Name}
	case "float64":
		return Float64Column{Table: c.Table, Name: c.Name}
	case "bool":
		return BoolColumn{Table: c.Table, Name: c.Name}
	case "string":
		return StringColumn{Table: c.Table, Name: c.Name}
	case "time.Time":
		return TimeColumn{Table: c.Table, Name: c.Name}
	case "[]byte":
		return BytesColumn{Table: c.Table, Name: c.Name}
	case "json.RawMessage":
		return JSONColumn{Table: c.Table, Name: c.Name}
	default:
		return StringColumn{Table: c.Table, Name: c.Name}
	}
}

// FromJSON converts ExprJson back to an Expr.
func (e *ExprJson) FromJSON() (Expr, error) {
	if e == nil {
		return nil, nil
	}

	switch e.Type {
	case "column":
		return ColumnExpr{Column: e.Column.ToColumn()}, nil

	case "param":
		return ParamExpr{Name: e.ParamName, GoType: e.ParamGoType}, nil

	case "literal":
		return LiteralExpr{Value: e.LiteralValue}, nil

	case "binary":
		left, err := e.Left.FromJSON()
		if err != nil {
			return nil, err
		}
		right, err := e.Right.FromJSON()
		if err != nil {
			return nil, err
		}
		return BinaryExpr{
			Left:  left,
			Op:    BinaryOp(e.Op),
			Right: right,
		}, nil

	case "unary":
		inner, err := e.Expr.FromJSON()
		if err != nil {
			return nil, err
		}
		return UnaryExpr{
			Op:   UnaryOp(e.UnaryOp),
			Expr: inner,
		}, nil

	case "func":
		var args []Expr
		for _, arg := range e.FuncArgs {
			argExpr, err := arg.FromJSON()
			if err != nil {
				return nil, err
			}
			args = append(args, argExpr)
		}
		return FuncExpr{
			Name: e.FuncName,
			Args: args,
		}, nil

	case "list":
		var values []Expr
		for _, val := range e.ListValues {
			valExpr, err := val.FromJSON()
			if err != nil {
				return nil, err
			}
			values = append(values, valExpr)
		}
		return ListExpr{Values: values}, nil

	case "aggregate":
		var arg Expr
		if e.AggArg != nil {
			var err error
			arg, err = e.AggArg.FromJSON()
			if err != nil {
				return nil, err
			}
		}
		return AggregateExpr{
			Func:     AggregateFunc(e.AggFunc),
			Arg:      arg,
			Distinct: e.AggDistinct,
		}, nil

	case "subquery":
		sub, err := e.Subquery.FromJSON()
		if err != nil {
			return nil, err
		}
		return SubqueryExpr{Query: sub}, nil

	case "exists":
		sub, err := e.Subquery.FromJSON()
		if err != nil {
			return nil, err
		}
		return ExistsExpr{
			Subquery: sub,
			Negated:  e.Negated,
		}, nil

	case "json_agg":
		var cols []Column
		for _, col := range e.JSONColumns {
			cols = append(cols, col.ToColumn())
		}
		return JSONAggExpr{
			FieldName: e.JSONFieldName,
			Columns:   cols,
		}, nil

	default:
		return nil, fmt.Errorf("unknown expression type: %s", e.Type)
	}
}

// =============================================================================
// MarshalJSON / UnmarshalJSON for AST
// =============================================================================

// MarshalJSON implements json.Marshaler for AST.
func (ast *AST) MarshalJSON() ([]byte, error) {
	j, err := ast.ToJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements json.Unmarshaler for AST.
func (ast *AST) UnmarshalJSON(data []byte) error {
	var j ASTJson
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	parsed, err := j.FromJSON()
	if err != nil {
		return err
	}
	*ast = *parsed
	return nil
}
