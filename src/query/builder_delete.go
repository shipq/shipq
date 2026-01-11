package query

// Delete starts building a DELETE query.
func Delete(table Table) *DeleteBuilder {
	return &DeleteBuilder{
		ast: &AST{
			Kind:      DeleteQuery,
			FromTable: TableRef{Name: table.TableName()},
		},
	}
}

// DeleteBuilder builds DELETE queries.
type DeleteBuilder struct {
	ast *AST
}

// Where sets the WHERE clause.
func (b *DeleteBuilder) Where(expr Expr) *DeleteBuilder {
	b.ast.Where = expr
	return b
}

// Build returns the completed AST.
func (b *DeleteBuilder) Build() *AST {
	return b.ast
}
