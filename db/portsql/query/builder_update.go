package query

// Update starts building an UPDATE query.
func Update(table Table) *UpdateBuilder {
	return &UpdateBuilder{
		ast: &AST{
			Kind:      UpdateQuery,
			FromTable: TableRef{Name: table.TableName()},
		},
	}
}

// UpdateBuilder builds UPDATE queries.
type UpdateBuilder struct {
	ast *AST
}

// Set adds a column = value clause to the UPDATE.
func (b *UpdateBuilder) Set(col Column, value Expr) *UpdateBuilder {
	b.ast.SetClauses = append(b.ast.SetClauses, SetClause{
		Column: col,
		Value:  value,
	})
	return b
}

// Where sets the WHERE clause.
func (b *UpdateBuilder) Where(expr Expr) *UpdateBuilder {
	b.ast.Where = expr
	return b
}

// Build returns the completed AST.
func (b *UpdateBuilder) Build() *AST {
	return b.ast
}
