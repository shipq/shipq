package query

// QueryKind identifies the type of query.
type QueryKind string

const (
	SelectQuery QueryKind = "select"
	InsertQuery QueryKind = "insert"
	UpdateQuery QueryKind = "update"
	DeleteQuery QueryKind = "delete"
)

// AST is the root of a query abstract syntax tree.
type AST struct {
	Kind       QueryKind
	Distinct   bool // SELECT DISTINCT
	FromTable  TableRef
	Joins      []JoinClause
	SelectCols []SelectExpr
	Where      Expr
	GroupBy    []Column
	Having     Expr
	OrderBy    []OrderByExpr
	Limit      Expr
	Offset     Expr

	// For INSERT
	InsertCols []Column
	InsertVals []Expr
	Returning  []Column

	// For UPDATE
	SetClauses []SetClause

	// For set operations (UNION, INTERSECT, EXCEPT)
	SetOp *SetOperation

	// For CTEs (WITH clause)
	CTEs []CTE

	// Collected parameters (for validation and codegen)
	Params []ParamInfo
}

// =============================================================================
// Set Operations (UNION, INTERSECT, EXCEPT)
// =============================================================================

// SetOpKind represents set operation types.
type SetOpKind string

const (
	SetOpUnion     SetOpKind = "UNION"
	SetOpUnionAll  SetOpKind = "UNION ALL"
	SetOpIntersect SetOpKind = "INTERSECT"
	SetOpExcept    SetOpKind = "EXCEPT"
)

// SetOperation represents a set operation between two queries.
type SetOperation struct {
	Left  *AST
	Op    SetOpKind
	Right *AST
}

// =============================================================================
// Common Table Expressions (CTEs)
// =============================================================================

// CTE represents a Common Table Expression (WITH clause).
type CTE struct {
	Name    string   // The CTE alias
	Columns []string // Optional column list
	Query   *AST     // The CTE query
}

// TableRef references a table, optionally with an alias.
type TableRef struct {
	Name  string
	Alias string
}

// JoinClause represents a JOIN.
type JoinClause struct {
	Type      JoinType
	Table     TableRef
	Condition Expr
}

// JoinType represents the type of join.
type JoinType string

const (
	InnerJoin JoinType = "INNER"
	LeftJoin  JoinType = "LEFT"
	RightJoin JoinType = "RIGHT"
	FullJoin  JoinType = "FULL"
)

// SelectExpr is a column or expression in a SELECT clause.
type SelectExpr struct {
	Expr  Expr
	Alias string
}

// OrderByExpr represents ORDER BY column [ASC|DESC].
type OrderByExpr struct {
	Expr Expr
	Desc bool
}

// SetClause represents column = value in UPDATE.
type SetClause struct {
	Column Column
	Value  Expr
}

// ParamInfo tracks parameters for codegen.
type ParamInfo struct {
	Name   string
	GoType string
}
