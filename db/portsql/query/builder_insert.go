package query

// InsertInto starts building an INSERT query.
func InsertInto(table Table) *InsertBuilder {
	return &InsertBuilder{
		ast: &AST{
			Kind:      InsertQuery,
			FromTable: TableRef{Name: table.TableName()},
		},
	}
}

// InsertBuilder builds INSERT queries.
type InsertBuilder struct {
	ast *AST
}

// Columns sets the columns to insert into.
func (b *InsertBuilder) Columns(cols ...Column) *InsertBuilder {
	b.ast.InsertCols = cols
	return b
}

// Values sets the values to insert.
func (b *InsertBuilder) Values(vals ...Expr) *InsertBuilder {
	b.ast.InsertVals = vals
	return b
}

// Returning sets the columns to return after insert.
func (b *InsertBuilder) Returning(cols ...Column) *InsertBuilder {
	b.ast.Returning = cols
	return b
}

// Build returns the completed AST.
func (b *InsertBuilder) Build() *AST {
	return b.ast
}
