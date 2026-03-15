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
// Values is mutually exclusive with FromSelect/FromSelectAST.
// Calling Values clears any previously set InsertSource.
func (b *InsertBuilder) Values(vals ...Expr) *InsertBuilder {
	b.ast.InsertSource = nil
	b.ast.InsertRows = [][]Expr{vals}
	return b
}

// AddRow appends a row of values for a multi-row (bulk) insert.
// Each call adds one row. Column count is validated at compile time.
// AddRow is mutually exclusive with FromSelect/FromSelectAST.
// Calling AddRow clears any previously set InsertSource.
func (b *InsertBuilder) AddRow(vals ...Expr) *InsertBuilder {
	b.ast.InsertSource = nil
	b.ast.InsertRows = append(b.ast.InsertRows, vals)
	return b
}

// BulkRows sets all rows at once for a bulk insert, replacing any
// previously added rows. Useful when the row count is dynamic.
// BulkRows is mutually exclusive with FromSelect/FromSelectAST.
// Calling BulkRows clears any previously set InsertSource.
func (b *InsertBuilder) BulkRows(rows [][]Expr) *InsertBuilder {
	b.ast.InsertSource = nil
	b.ast.InsertRows = rows
	return b
}

// FromSelect sets the source of the INSERT to a SELECT query.
// This produces INSERT INTO t (cols) SELECT ... FROM ...
//
// FromSelect is mutually exclusive with Values/AddRow/BulkRows.
// Calling FromSelect clears any previously set InsertRows.
func (b *InsertBuilder) FromSelect(source *SelectBuilder) *InsertBuilder {
	b.ast.InsertRows = nil
	b.ast.InsertSource = source.Build()
	return b
}

// FromSelectAST sets the source of the INSERT to an existing AST.
// This is the escape hatch for when the source query is built
// programmatically or comes from a CTE select builder.
//
// FromSelectAST is mutually exclusive with Values/AddRow/BulkRows.
// Calling FromSelectAST clears any previously set InsertRows.
func (b *InsertBuilder) FromSelectAST(source *AST) *InsertBuilder {
	b.ast.InsertRows = nil
	b.ast.InsertSource = source
	return b
}

// WithCTEs attaches Common Table Expressions to this INSERT query.
// The CTEs are emitted as a WITH clause before the INSERT statement.
func (b *InsertBuilder) WithCTEs(ctes ...CTE) *InsertBuilder {
	b.ast.CTEs = ctes
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
