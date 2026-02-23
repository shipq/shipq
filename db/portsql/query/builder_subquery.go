package query

// =============================================================================
// Subquery Builders
// =============================================================================

// Subquery creates a subquery expression from a SelectBuilder.
func Subquery(builder *SelectBuilder) SubqueryExpr {
	return SubqueryExpr{Query: builder.Build()}
}

// SubqueryAST creates a subquery expression from an AST.
func SubqueryAST(ast *AST) SubqueryExpr {
	return SubqueryExpr{Query: ast}
}

// Exists creates an EXISTS (subquery) expression.
func Exists(builder *SelectBuilder) ExistsExpr {
	return ExistsExpr{Subquery: builder.Build(), Negated: false}
}

// ExistsAST creates an EXISTS (subquery) expression from an AST.
func ExistsAST(ast *AST) ExistsExpr {
	return ExistsExpr{Subquery: ast, Negated: false}
}

// NotExists creates a NOT EXISTS (subquery) expression.
func NotExists(builder *SelectBuilder) ExistsExpr {
	return ExistsExpr{Subquery: builder.Build(), Negated: true}
}

// NotExistsAST creates a NOT EXISTS (subquery) expression from an AST.
func NotExistsAST(ast *AST) ExistsExpr {
	return ExistsExpr{Subquery: ast, Negated: true}
}

// =============================================================================
// Column InSubquery Methods
// =============================================================================

// InSubquery creates a "column IN (subquery)" expression for StringColumn.
func (c StringColumn) InSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpIn,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// InSubquery creates a "column IN (subquery)" expression for Int64Column.
func (c Int64Column) InSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpIn,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// InSubquery creates a "column IN (subquery)" expression for Int32Column.
func (c Int32Column) InSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpIn,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// InSubquery creates a "column IN (subquery)" expression for NullStringColumn.
func (c NullStringColumn) InSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpIn,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// InSubquery creates a "column IN (subquery)" expression for NullInt64Column.
func (c NullInt64Column) InSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpIn,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// =============================================================================
// Scalar Subquery Comparison
// =============================================================================

// EqSubquery creates a "column = (scalar subquery)" expression.
func (c Int64Column) EqSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpEq,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// GtSubquery creates a "column > (scalar subquery)" expression.
func (c Int64Column) GtSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpGt,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}

// LtSubquery creates a "column < (scalar subquery)" expression.
func (c Int64Column) LtSubquery(builder *SelectBuilder) BinaryExpr {
	return BinaryExpr{
		Left:  ColumnExpr{Column: c},
		Op:    OpLt,
		Right: SubqueryExpr{Query: builder.Build()},
	}
}
