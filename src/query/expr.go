package query

// Expr is the interface for all expressions in a query AST.
type Expr interface {
	exprNode() // marker method to identify expression types
}

// ColumnExpr wraps a Column as an expression.
type ColumnExpr struct {
	Column Column
}

func (ColumnExpr) exprNode() {}

// ParamExpr represents a query parameter.
type ParamExpr struct {
	Name   string
	GoType string
}

func (ParamExpr) exprNode() {}

// LiteralExpr represents a literal value.
type LiteralExpr struct {
	Value any
}

func (LiteralExpr) exprNode() {}

// BinaryExpr represents a binary operation (left op right).
type BinaryExpr struct {
	Left  Expr
	Op    BinaryOp
	Right Expr
}

func (BinaryExpr) exprNode() {}

// BinaryOp represents binary operators.
type BinaryOp string

const (
	OpEq   BinaryOp = "="
	OpNe   BinaryOp = "<>"
	OpLt   BinaryOp = "<"
	OpLe   BinaryOp = "<="
	OpGt   BinaryOp = ">"
	OpGe   BinaryOp = ">="
	OpAnd  BinaryOp = "AND"
	OpOr   BinaryOp = "OR"
	OpLike BinaryOp = "LIKE"
	OpIn   BinaryOp = "IN"
)

// UnaryExpr represents a unary operation (op expr).
type UnaryExpr struct {
	Op   UnaryOp
	Expr Expr
}

func (UnaryExpr) exprNode() {}

// UnaryOp represents unary operators.
type UnaryOp string

const (
	OpNot     UnaryOp = "NOT"
	OpIsNull  UnaryOp = "IS NULL"
	OpNotNull UnaryOp = "IS NOT NULL"
)

// FuncExpr represents a function call.
type FuncExpr struct {
	Name string
	Args []Expr
}

func (FuncExpr) exprNode() {}

// ListExpr represents a list of values (for IN clause).
type ListExpr struct {
	Values []Expr
}

func (ListExpr) exprNode() {}

// JSONAggExpr represents JSON aggregation.
type JSONAggExpr struct {
	FieldName string   // The key in the result struct
	Columns   []Column // Columns to aggregate
}

func (JSONAggExpr) exprNode() {}

// =============================================================================
// Aggregate Expressions (COUNT, SUM, AVG, MIN, MAX)
// =============================================================================

// AggregateFunc represents an aggregate function type.
type AggregateFunc string

const (
	AggCount AggregateFunc = "COUNT"
	AggSum   AggregateFunc = "SUM"
	AggAvg   AggregateFunc = "AVG"
	AggMin   AggregateFunc = "MIN"
	AggMax   AggregateFunc = "MAX"
)

// AggregateExpr represents an aggregate function call.
// Examples: COUNT(*), SUM(amount), AVG(price), COUNT(DISTINCT email)
type AggregateExpr struct {
	Func     AggregateFunc
	Arg      Expr // The column/expression to aggregate (nil for COUNT(*))
	Distinct bool // COUNT(DISTINCT ...) or other distinct aggregates
}

func (AggregateExpr) exprNode() {}

// =============================================================================
// Subquery Expressions
// =============================================================================

// SubqueryExpr represents a subquery used as an expression.
// Can be used with IN, comparison operators, or as a scalar.
type SubqueryExpr struct {
	Query *AST // The nested query
}

func (SubqueryExpr) exprNode() {}

// ExistsExpr represents EXISTS (subquery) or NOT EXISTS (subquery).
type ExistsExpr struct {
	Subquery *AST
	Negated  bool // true for NOT EXISTS
}

func (ExistsExpr) exprNode() {}

// Compile-time verification that all expression types implement Expr
var (
	_ Expr = ColumnExpr{}
	_ Expr = ParamExpr{}
	_ Expr = LiteralExpr{}
	_ Expr = BinaryExpr{}
	_ Expr = UnaryExpr{}
	_ Expr = FuncExpr{}
	_ Expr = ListExpr{}
	_ Expr = JSONAggExpr{}
	_ Expr = AggregateExpr{}
	_ Expr = SubqueryExpr{}
	_ Expr = ExistsExpr{}
)
