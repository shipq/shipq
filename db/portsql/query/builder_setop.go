package query

// =============================================================================
// Set Operation Builders (UNION, INTERSECT, EXCEPT)
// =============================================================================

// SetOpBuilder builds set operation queries.
type SetOpBuilder struct {
	left    *AST
	op      SetOpKind
	right   *AST
	orderBy []OrderByExpr
	limit   Expr
	offset  Expr
}

// Union combines two queries with UNION (removes duplicates).
func (b *SelectBuilder) Union(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpUnion,
		right: other.Build(),
	}
}

// UnionAST combines a query with an AST using UNION.
func (b *SelectBuilder) UnionAST(other *AST) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpUnion,
		right: other,
	}
}

// UnionAll combines two queries with UNION ALL (keeps duplicates).
func (b *SelectBuilder) UnionAll(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpUnionAll,
		right: other.Build(),
	}
}

// UnionAllAST combines a query with an AST using UNION ALL.
func (b *SelectBuilder) UnionAllAST(other *AST) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpUnionAll,
		right: other,
	}
}

// Intersect combines two queries with INTERSECT.
func (b *SelectBuilder) Intersect(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpIntersect,
		right: other.Build(),
	}
}

// IntersectAST combines a query with an AST using INTERSECT.
func (b *SelectBuilder) IntersectAST(other *AST) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpIntersect,
		right: other,
	}
}

// Except combines two queries with EXCEPT.
func (b *SelectBuilder) Except(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpExcept,
		right: other.Build(),
	}
}

// ExceptAST combines a query with an AST using EXCEPT.
func (b *SelectBuilder) ExceptAST(other *AST) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpExcept,
		right: other,
	}
}

// =============================================================================
// SetOpBuilder Methods
// =============================================================================

// OrderBy adds an ORDER BY clause to the combined result.
func (b *SetOpBuilder) OrderBy(expr OrderByExpr) *SetOpBuilder {
	b.orderBy = append(b.orderBy, expr)
	return b
}

// Limit sets the LIMIT clause on the combined result.
func (b *SetOpBuilder) Limit(expr Expr) *SetOpBuilder {
	b.limit = expr
	return b
}

// Offset sets the OFFSET clause on the combined result.
func (b *SetOpBuilder) Offset(expr Expr) *SetOpBuilder {
	b.offset = expr
	return b
}

// Build returns the final AST for the set operation.
func (b *SetOpBuilder) Build() *AST {
	return &AST{
		Kind: SelectQuery,
		SetOp: &SetOperation{
			Left:  b.left,
			Op:    b.op,
			Right: b.right,
		},
		OrderBy: b.orderBy,
		Limit:   b.limit,
		Offset:  b.offset,
	}
}

// =============================================================================
// Chained Set Operations
// =============================================================================

// Union chains another UNION onto an existing set operation.
func (b *SetOpBuilder) Union(other *SelectBuilder) *SetOpBuilder {
	// Wrap current set op as the left side
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpUnion,
		right: other.Build(),
	}
}

// UnionAll chains another UNION ALL onto an existing set operation.
func (b *SetOpBuilder) UnionAll(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpUnionAll,
		right: other.Build(),
	}
}

// Intersect chains another INTERSECT onto an existing set operation.
func (b *SetOpBuilder) Intersect(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpIntersect,
		right: other.Build(),
	}
}

// Except chains another EXCEPT onto an existing set operation.
func (b *SetOpBuilder) Except(other *SelectBuilder) *SetOpBuilder {
	return &SetOpBuilder{
		left:  b.Build(),
		op:    SetOpExcept,
		right: other.Build(),
	}
}
