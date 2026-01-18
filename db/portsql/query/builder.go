package query

import (
	"log"
)

// Table is implemented by generated table structs.
type Table interface {
	TableName() string
}

// From starts building a SELECT query from the given table.
func From(table Table) *SelectBuilder {
	return &SelectBuilder{
		ast: &AST{
			Kind: SelectQuery,
			FromTable: TableRef{
				Name: table.TableName(),
			},
		},
	}
}

// SelectBuilder builds SELECT queries.
type SelectBuilder struct {
	ast *AST
}

// Distinct sets the DISTINCT flag for the SELECT.
func (b *SelectBuilder) Distinct() *SelectBuilder {
	b.ast.Distinct = true
	return b
}

// Select adds columns to the SELECT clause.
func (b *SelectBuilder) Select(cols ...Column) *SelectBuilder {
	for _, col := range cols {
		b.ast.SelectCols = append(b.ast.SelectCols, SelectExpr{
			Expr: ColumnExpr{Column: col},
		})
	}
	return b
}

// SelectExpr adds an expression to the SELECT clause.
func (b *SelectBuilder) SelectExpr(expr Expr) *SelectBuilder {
	b.ast.SelectCols = append(b.ast.SelectCols, SelectExpr{
		Expr: expr,
	})
	return b
}

// SelectAs adds a column with an alias to the SELECT clause.
func (b *SelectBuilder) SelectAs(col Column, alias string) *SelectBuilder {
	b.ast.SelectCols = append(b.ast.SelectCols, SelectExpr{
		Expr:  ColumnExpr{Column: col},
		Alias: alias,
	})
	return b
}

// SelectExprAs adds an expression with an alias to the SELECT clause.
func (b *SelectBuilder) SelectExprAs(expr Expr, alias string) *SelectBuilder {
	b.ast.SelectCols = append(b.ast.SelectCols, SelectExpr{
		Expr:  expr,
		Alias: alias,
	})
	return b
}

// SelectJSONAgg adds a JSON aggregation to the SELECT clause.
// Panics if no columns are provided, as JSON aggregation requires at least one column.
func (b *SelectBuilder) SelectJSONAgg(fieldName string, cols ...Column) *SelectBuilder {
	if len(cols) == 0 {
		log.Fatalln("SelectJSONAgg requires at least one column - please fix your query and try again.")
	}
	b.ast.SelectCols = append(b.ast.SelectCols, SelectExpr{
		Expr: JSONAggExpr{
			FieldName: fieldName,
			Columns:   cols,
		},
		Alias: fieldName,
	})
	return b
}

// Join adds an INNER JOIN to the query.
func (b *SelectBuilder) Join(table Table) *JoinBuilder {
	return &JoinBuilder{
		parent: b,
		join: JoinClause{
			Type:  InnerJoin,
			Table: TableRef{Name: table.TableName()},
		},
	}
}

// LeftJoin adds a LEFT JOIN to the query.
func (b *SelectBuilder) LeftJoin(table Table) *JoinBuilder {
	return &JoinBuilder{
		parent: b,
		join: JoinClause{
			Type:  LeftJoin,
			Table: TableRef{Name: table.TableName()},
		},
	}
}

// RightJoin adds a RIGHT JOIN to the query.
func (b *SelectBuilder) RightJoin(table Table) *JoinBuilder {
	return &JoinBuilder{
		parent: b,
		join: JoinClause{
			Type:  RightJoin,
			Table: TableRef{Name: table.TableName()},
		},
	}
}

// FullJoin adds a FULL OUTER JOIN to the query.
func (b *SelectBuilder) FullJoin(table Table) *JoinBuilder {
	return &JoinBuilder{
		parent: b,
		join: JoinClause{
			Type:  FullJoin,
			Table: TableRef{Name: table.TableName()},
		},
	}
}

// Where sets the WHERE clause.
func (b *SelectBuilder) Where(expr Expr) *SelectBuilder {
	b.ast.Where = expr
	return b
}

// GroupBy sets the GROUP BY clause.
func (b *SelectBuilder) GroupBy(cols ...Column) *SelectBuilder {
	b.ast.GroupBy = cols
	return b
}

// Having sets the HAVING clause.
func (b *SelectBuilder) Having(expr Expr) *SelectBuilder {
	b.ast.Having = expr
	return b
}

// OrderBy adds an ORDER BY clause.
func (b *SelectBuilder) OrderBy(expr OrderByExpr) *SelectBuilder {
	b.ast.OrderBy = append(b.ast.OrderBy, expr)
	return b
}

// Limit sets the LIMIT clause.
func (b *SelectBuilder) Limit(expr Expr) *SelectBuilder {
	b.ast.Limit = expr
	return b
}

// Offset sets the OFFSET clause.
func (b *SelectBuilder) Offset(expr Expr) *SelectBuilder {
	b.ast.Offset = expr
	return b
}

// Build returns the completed AST.
func (b *SelectBuilder) Build() *AST {
	return b.ast
}

// JoinBuilder handles the ON clause for joins.
type JoinBuilder struct {
	parent *SelectBuilder
	join   JoinClause
}

// On sets the join condition and returns to the parent builder.
func (b *JoinBuilder) On(condition Expr) *SelectBuilder {
	b.join.Condition = condition
	b.parent.ast.Joins = append(b.parent.ast.Joins, b.join)
	return b.parent
}

// As sets an alias for the joined table.
func (b *JoinBuilder) As(alias string) *JoinBuilder {
	b.join.Table.Alias = alias
	return b
}
