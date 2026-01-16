package query

// =============================================================================
// CTE (Common Table Expression) Builders
// =============================================================================

// CTEBuilder helps construct queries with CTEs (WITH clause).
type CTEBuilder struct {
	ctes []CTE
}

// With starts a CTE definition.
// Usage: With("recent_orders", query).Select(table)...
func With(name string, builder *SelectBuilder) *CTEBuilder {
	return &CTEBuilder{
		ctes: []CTE{{Name: name, Query: builder.Build()}},
	}
}

// WithAST starts a CTE definition from an AST.
func WithAST(name string, ast *AST) *CTEBuilder {
	return &CTEBuilder{
		ctes: []CTE{{Name: name, Query: ast}},
	}
}

// WithColumns starts a CTE definition with explicit column names.
// Usage: WithColumns("cte", []string{"a", "b"}, query)
func WithColumns(name string, columns []string, builder *SelectBuilder) *CTEBuilder {
	return &CTEBuilder{
		ctes: []CTE{{Name: name, Columns: columns, Query: builder.Build()}},
	}
}

// And adds another CTE to the WITH clause.
func (b *CTEBuilder) And(name string, builder *SelectBuilder) *CTEBuilder {
	b.ctes = append(b.ctes, CTE{Name: name, Query: builder.Build()})
	return b
}

// AndAST adds another CTE from an AST.
func (b *CTEBuilder) AndAST(name string, ast *AST) *CTEBuilder {
	b.ctes = append(b.ctes, CTE{Name: name, Query: ast})
	return b
}

// AndColumns adds another CTE with explicit column names.
func (b *CTEBuilder) AndColumns(name string, columns []string, builder *SelectBuilder) *CTEBuilder {
	b.ctes = append(b.ctes, CTE{Name: name, Columns: columns, Query: builder.Build()})
	return b
}

// Select starts the main query that uses the CTEs.
func (b *CTEBuilder) Select(table Table) *CTESelectBuilder {
	return &CTESelectBuilder{
		ctes: b.ctes,
		builder: &SelectBuilder{
			ast: &AST{
				Kind:      SelectQuery,
				FromTable: TableRef{Name: table.TableName()},
			},
		},
	}
}

// =============================================================================
// CTESelectBuilder - Wraps SelectBuilder with CTEs
// =============================================================================

// CTESelectBuilder wraps SelectBuilder and includes CTEs in the final AST.
type CTESelectBuilder struct {
	ctes    []CTE
	builder *SelectBuilder
}

// Distinct sets the DISTINCT flag.
func (b *CTESelectBuilder) Distinct() *CTESelectBuilder {
	b.builder.Distinct()
	return b
}

// Select adds columns to the SELECT clause.
func (b *CTESelectBuilder) Select(cols ...Column) *CTESelectBuilder {
	b.builder.Select(cols...)
	return b
}

// SelectExpr adds an expression to the SELECT clause.
func (b *CTESelectBuilder) SelectExpr(expr Expr) *CTESelectBuilder {
	b.builder.SelectExpr(expr)
	return b
}

// SelectAs adds a column with an alias.
func (b *CTESelectBuilder) SelectAs(col Column, alias string) *CTESelectBuilder {
	b.builder.SelectAs(col, alias)
	return b
}

// SelectExprAs adds an expression with an alias.
func (b *CTESelectBuilder) SelectExprAs(expr Expr, alias string) *CTESelectBuilder {
	b.builder.SelectExprAs(expr, alias)
	return b
}

// SelectCount adds COUNT(*).
func (b *CTESelectBuilder) SelectCount() *CTESelectBuilder {
	b.builder.SelectCount()
	return b
}

// SelectCountAs adds COUNT(*) AS alias.
func (b *CTESelectBuilder) SelectCountAs(alias string) *CTESelectBuilder {
	b.builder.SelectCountAs(alias)
	return b
}

// Join adds an INNER JOIN.
func (b *CTESelectBuilder) Join(table Table) *CTEJoinBuilder {
	return &CTEJoinBuilder{
		parent:      b,
		joinBuilder: b.builder.Join(table),
	}
}

// LeftJoin adds a LEFT JOIN.
func (b *CTESelectBuilder) LeftJoin(table Table) *CTEJoinBuilder {
	return &CTEJoinBuilder{
		parent:      b,
		joinBuilder: b.builder.LeftJoin(table),
	}
}

// Where sets the WHERE clause.
func (b *CTESelectBuilder) Where(expr Expr) *CTESelectBuilder {
	b.builder.Where(expr)
	return b
}

// GroupBy sets the GROUP BY clause.
func (b *CTESelectBuilder) GroupBy(cols ...Column) *CTESelectBuilder {
	b.builder.GroupBy(cols...)
	return b
}

// Having sets the HAVING clause.
func (b *CTESelectBuilder) Having(expr Expr) *CTESelectBuilder {
	b.builder.Having(expr)
	return b
}

// OrderBy adds an ORDER BY clause.
func (b *CTESelectBuilder) OrderBy(expr OrderByExpr) *CTESelectBuilder {
	b.builder.OrderBy(expr)
	return b
}

// Limit sets the LIMIT clause.
func (b *CTESelectBuilder) Limit(expr Expr) *CTESelectBuilder {
	b.builder.Limit(expr)
	return b
}

// Offset sets the OFFSET clause.
func (b *CTESelectBuilder) Offset(expr Expr) *CTESelectBuilder {
	b.builder.Offset(expr)
	return b
}

// Build returns the final AST with CTEs included.
func (b *CTESelectBuilder) Build() *AST {
	ast := b.builder.Build()
	ast.CTEs = b.ctes
	return ast
}

// =============================================================================
// CTEJoinBuilder - Wraps JoinBuilder for CTE queries
// =============================================================================

// CTEJoinBuilder handles JOIN for CTE queries.
type CTEJoinBuilder struct {
	parent      *CTESelectBuilder
	joinBuilder *JoinBuilder
}

// On sets the join condition.
func (b *CTEJoinBuilder) On(condition Expr) *CTESelectBuilder {
	b.joinBuilder.On(condition)
	return b.parent
}

// As sets an alias for the joined table.
func (b *CTEJoinBuilder) As(alias string) *CTEJoinBuilder {
	b.joinBuilder.As(alias)
	return b
}

// =============================================================================
// CTETable - Reference a CTE as a table
// =============================================================================

// CTETable represents a reference to a CTE for use in FROM/JOIN.
type CTETable struct {
	name string
}

// CTE creates a reference to a CTE by name.
func CTERef(name string) CTETable {
	return CTETable{name: name}
}

// TableName returns the CTE name.
func (t CTETable) TableName() string {
	return t.name
}
