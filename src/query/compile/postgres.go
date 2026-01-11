package compile

import (
	"fmt"
	"strings"

	"github.com/portsql/portsql/src/query"
)

// PostgresCompiler compiles AST to Postgres SQL.
type PostgresCompiler struct {
	paramCount int
	params     []string // Tracks param names in order
}

// Compile compiles an AST to Postgres SQL.
// Returns the SQL string and the parameter names in order.
func (c *PostgresCompiler) Compile(ast *query.AST) (sql string, paramOrder []string, err error) {
	c.paramCount = 0
	c.params = nil

	var b strings.Builder

	// Handle CTEs (WITH clause) first
	if len(ast.CTEs) > 0 {
		if err := c.writeCTEs(&b, ast.CTEs); err != nil {
			return "", nil, err
		}
	}

	// Handle set operations
	if ast.SetOp != nil {
		setSQL, err := c.compileSetOp(ast)
		if err != nil {
			return "", nil, err
		}
		b.WriteString(setSQL)
		return b.String(), c.params, nil
	}

	switch ast.Kind {
	case query.SelectQuery:
		sql, err = c.compileSelect(ast)
	case query.InsertQuery:
		sql, err = c.compileInsert(ast)
	case query.UpdateQuery:
		sql, err = c.compileUpdate(ast)
	case query.DeleteQuery:
		sql, err = c.compileDelete(ast)
	default:
		err = fmt.Errorf("unknown query kind: %s", ast.Kind)
	}

	if err != nil {
		return "", nil, err
	}

	b.WriteString(sql)
	return b.String(), c.params, nil
}

// CompilePostgres is a convenience function.
func CompilePostgres(ast *query.AST) (string, []string, error) {
	c := &PostgresCompiler{}
	return c.Compile(ast)
}

func (c *PostgresCompiler) compileSelect(ast *query.AST) (string, error) {
	var b strings.Builder

	// SELECT clause
	b.WriteString("SELECT ")
	if ast.Distinct {
		b.WriteString("DISTINCT ")
	}
	if len(ast.SelectCols) == 0 {
		b.WriteString("*")
	} else {
		for i, col := range ast.SelectCols {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeExpr(&b, col.Expr); err != nil {
				return "", err
			}
			if col.Alias != "" {
				b.WriteString(" AS ")
				c.writeIdentifier(&b, col.Alias)
			}
		}
	}

	// FROM clause
	b.WriteString(" FROM ")
	c.writeIdentifier(&b, ast.FromTable.Name)
	if ast.FromTable.Alias != "" {
		b.WriteString(" AS ")
		c.writeIdentifier(&b, ast.FromTable.Alias)
	}

	// JOIN clauses
	for _, join := range ast.Joins {
		b.WriteString(" ")
		b.WriteString(string(join.Type))
		b.WriteString(" JOIN ")
		c.writeIdentifier(&b, join.Table.Name)
		if join.Table.Alias != "" {
			b.WriteString(" AS ")
			c.writeIdentifier(&b, join.Table.Alias)
		}
		b.WriteString(" ON ")
		if err := c.writeExpr(&b, join.Condition); err != nil {
			return "", err
		}
	}

	// WHERE clause
	if ast.Where != nil {
		b.WriteString(" WHERE ")
		if err := c.writeExpr(&b, ast.Where); err != nil {
			return "", err
		}
	}

	// GROUP BY clause
	if len(ast.GroupBy) > 0 {
		b.WriteString(" GROUP BY ")
		for i, col := range ast.GroupBy {
			if i > 0 {
				b.WriteString(", ")
			}
			c.writeColumn(&b, col)
		}
	}

	// HAVING clause
	if ast.Having != nil {
		b.WriteString(" HAVING ")
		if err := c.writeExpr(&b, ast.Having); err != nil {
			return "", err
		}
	}

	// ORDER BY clause
	if len(ast.OrderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, ob := range ast.OrderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeExpr(&b, ob.Expr); err != nil {
				return "", err
			}
			if ob.Desc {
				b.WriteString(" DESC")
			}
		}
	}

	// LIMIT clause
	if ast.Limit != nil {
		b.WriteString(" LIMIT ")
		if err := c.writeExpr(&b, ast.Limit); err != nil {
			return "", err
		}
	}

	// OFFSET clause
	if ast.Offset != nil {
		b.WriteString(" OFFSET ")
		if err := c.writeExpr(&b, ast.Offset); err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func (c *PostgresCompiler) compileInsert(ast *query.AST) (string, error) {
	var b strings.Builder

	b.WriteString("INSERT INTO ")
	c.writeIdentifier(&b, ast.FromTable.Name)

	// Column list
	if len(ast.InsertCols) > 0 {
		b.WriteString(" (")
		for i, col := range ast.InsertCols {
			if i > 0 {
				b.WriteString(", ")
			}
			c.writeIdentifier(&b, col.ColumnName())
		}
		b.WriteString(")")
	}

	// VALUES clause
	b.WriteString(" VALUES (")
	for i, val := range ast.InsertVals {
		if i > 0 {
			b.WriteString(", ")
		}
		if err := c.writeExpr(&b, val); err != nil {
			return "", err
		}
	}
	b.WriteString(")")

	// RETURNING clause (Postgres-specific)
	if len(ast.Returning) > 0 {
		b.WriteString(" RETURNING ")
		for i, col := range ast.Returning {
			if i > 0 {
				b.WriteString(", ")
			}
			c.writeIdentifier(&b, col.ColumnName())
		}
	}

	return b.String(), nil
}

func (c *PostgresCompiler) compileUpdate(ast *query.AST) (string, error) {
	var b strings.Builder

	b.WriteString("UPDATE ")
	c.writeIdentifier(&b, ast.FromTable.Name)

	// SET clause
	b.WriteString(" SET ")
	for i, set := range ast.SetClauses {
		if i > 0 {
			b.WriteString(", ")
		}
		c.writeIdentifier(&b, set.Column.ColumnName())
		b.WriteString(" = ")
		if err := c.writeExpr(&b, set.Value); err != nil {
			return "", err
		}
	}

	// WHERE clause
	if ast.Where != nil {
		b.WriteString(" WHERE ")
		if err := c.writeExpr(&b, ast.Where); err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func (c *PostgresCompiler) compileDelete(ast *query.AST) (string, error) {
	var b strings.Builder

	b.WriteString("DELETE FROM ")
	c.writeIdentifier(&b, ast.FromTable.Name)

	// WHERE clause
	if ast.Where != nil {
		b.WriteString(" WHERE ")
		if err := c.writeExpr(&b, ast.Where); err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func (c *PostgresCompiler) writeExpr(b *strings.Builder, expr query.Expr) error {
	switch e := expr.(type) {
	case query.ColumnExpr:
		c.writeColumn(b, e.Column)

	case query.ParamExpr:
		c.paramCount++
		c.params = append(c.params, e.Name)
		fmt.Fprintf(b, "$%d", c.paramCount)

	case query.LiteralExpr:
		c.writeLiteral(b, e.Value)

	case query.BinaryExpr:
		if e.Op == query.OpIn {
			if err := c.writeExpr(b, e.Left); err != nil {
				return err
			}
			b.WriteString(" IN ")
			// Handle both ListExpr and SubqueryExpr
			switch right := e.Right.(type) {
			case query.ListExpr:
				b.WriteString("(")
				for i, v := range right.Values {
					if i > 0 {
						b.WriteString(", ")
					}
					if err := c.writeExpr(b, v); err != nil {
						return err
					}
				}
				b.WriteString(")")
			case query.SubqueryExpr:
				if err := c.writeExpr(b, right); err != nil {
					return err
				}
			default:
				return fmt.Errorf("IN operator requires ListExpr or SubqueryExpr on right side, got %T", e.Right)
			}
		} else {
			b.WriteString("(")
			if err := c.writeExpr(b, e.Left); err != nil {
				return err
			}
			fmt.Fprintf(b, " %s ", e.Op)
			if err := c.writeExpr(b, e.Right); err != nil {
				return err
			}
			b.WriteString(")")
		}

	case query.UnaryExpr:
		if e.Op == query.OpIsNull || e.Op == query.OpNotNull {
			if err := c.writeExpr(b, e.Expr); err != nil {
				return err
			}
			fmt.Fprintf(b, " %s", e.Op)
		} else {
			fmt.Fprintf(b, "%s ", e.Op)
			if err := c.writeExpr(b, e.Expr); err != nil {
				return err
			}
		}

	case query.FuncExpr:
		if err := c.writeFunc(b, e); err != nil {
			return err
		}

	case query.JSONAggExpr:
		c.writeJSONAgg(b, e)

	case query.ListExpr:
		// ListExpr on its own (not inside IN)
		b.WriteString("(")
		for i, v := range e.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeExpr(b, v); err != nil {
				return err
			}
		}
		b.WriteString(")")

	case query.AggregateExpr:
		// Write aggregate function: COUNT, SUM, AVG, MIN, MAX
		b.WriteString(string(e.Func))
		b.WriteString("(")
		if e.Distinct {
			b.WriteString("DISTINCT ")
		}
		if e.Arg == nil {
			// COUNT(*)
			b.WriteString("*")
		} else {
			if err := c.writeExpr(b, e.Arg); err != nil {
				return err
			}
		}
		b.WriteString(")")

	case query.SubqueryExpr:
		// Write subquery wrapped in parentheses
		b.WriteString("(")
		subCompiler := &PostgresCompiler{paramCount: c.paramCount}
		subSQL, subParams, err := subCompiler.Compile(e.Query)
		if err != nil {
			return err
		}
		b.WriteString(subSQL)
		b.WriteString(")")
		// Update param count and collect params
		c.paramCount = subCompiler.paramCount
		c.params = append(c.params, subParams...)

	case query.ExistsExpr:
		if e.Negated {
			b.WriteString("NOT ")
		}
		b.WriteString("EXISTS (")
		subCompiler := &PostgresCompiler{paramCount: c.paramCount}
		subSQL, subParams, err := subCompiler.Compile(e.Subquery)
		if err != nil {
			return err
		}
		b.WriteString(subSQL)
		b.WriteString(")")
		c.paramCount = subCompiler.paramCount
		c.params = append(c.params, subParams...)

	default:
		return fmt.Errorf("unknown expression type: %T", expr)
	}

	return nil
}

func (c *PostgresCompiler) writeIdentifier(b *strings.Builder, name string) {
	// Postgres uses double quotes for identifiers
	fmt.Fprintf(b, `"%s"`, name)
}

func (c *PostgresCompiler) writeColumn(b *strings.Builder, col query.Column) {
	c.writeIdentifier(b, col.TableName())
	b.WriteString(".")
	c.writeIdentifier(b, col.ColumnName())
}

func (c *PostgresCompiler) writeLiteral(b *strings.Builder, val any) {
	switch v := val.(type) {
	case string:
		// Escape single quotes by doubling them
		escaped := strings.ReplaceAll(v, "'", "''")
		fmt.Fprintf(b, "'%s'", escaped)
	case bool:
		if v {
			b.WriteString("TRUE")
		} else {
			b.WriteString("FALSE")
		}
	case nil:
		b.WriteString("NULL")
	default:
		fmt.Fprintf(b, "%v", v)
	}
}

func (c *PostgresCompiler) writeFunc(b *strings.Builder, f query.FuncExpr) error {
	switch f.Name {
	case "NOW":
		b.WriteString("NOW()")
	case "ILIKE":
		// Postgres has native ILIKE
		if len(f.Args) != 2 {
			return fmt.Errorf("ILIKE requires exactly 2 arguments")
		}
		if err := c.writeExpr(b, f.Args[0]); err != nil {
			return err
		}
		b.WriteString(" ILIKE ")
		if err := c.writeExpr(b, f.Args[1]); err != nil {
			return err
		}
	default:
		b.WriteString(f.Name)
		b.WriteString("(")
		for i, arg := range f.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeExpr(b, arg); err != nil {
				return err
			}
		}
		b.WriteString(")")
	}
	return nil
}

func (c *PostgresCompiler) writeJSONAgg(b *strings.Builder, j query.JSONAggExpr) {
	// COALESCE(JSON_AGG(JSON_BUILD_OBJECT(...)) FILTER (WHERE ... IS NOT NULL), '[]')
	b.WriteString("COALESCE(JSON_AGG(JSON_BUILD_OBJECT(")
	for i, col := range j.Columns {
		if i > 0 {
			b.WriteString(", ")
		}
		// Key is column name
		fmt.Fprintf(b, "'%s', ", col.ColumnName())
		c.writeColumn(b, col)
	}
	b.WriteString(")) FILTER (WHERE ")
	// Use first column for null check
	c.writeColumn(b, j.Columns[0])
	b.WriteString(" IS NOT NULL), '[]')")
}

// =============================================================================
// CTE Compilation
// =============================================================================

func (c *PostgresCompiler) writeCTEs(b *strings.Builder, ctes []query.CTE) error {
	b.WriteString("WITH ")
	for i, cte := range ctes {
		if i > 0 {
			b.WriteString(", ")
		}
		c.writeIdentifier(b, cte.Name)
		// Optional column list
		if len(cte.Columns) > 0 {
			b.WriteString(" (")
			for j, col := range cte.Columns {
				if j > 0 {
					b.WriteString(", ")
				}
				c.writeIdentifier(b, col)
			}
			b.WriteString(")")
		}
		b.WriteString(" AS (")
		// Compile CTE query
		cteCompiler := &PostgresCompiler{paramCount: c.paramCount}
		cteSQL, cteParams, err := cteCompiler.Compile(cte.Query)
		if err != nil {
			return err
		}
		b.WriteString(cteSQL)
		b.WriteString(")")
		c.paramCount = cteCompiler.paramCount
		c.params = append(c.params, cteParams...)
	}
	b.WriteString(" ")
	return nil
}

// =============================================================================
// Set Operation Compilation (UNION, INTERSECT, EXCEPT)
// =============================================================================

func (c *PostgresCompiler) compileSetOp(ast *query.AST) (string, error) {
	var b strings.Builder

	// Compile left query
	leftCompiler := &PostgresCompiler{paramCount: c.paramCount}
	leftSQL, leftParams, err := leftCompiler.Compile(ast.SetOp.Left)
	if err != nil {
		return "", err
	}
	b.WriteString("(")
	b.WriteString(leftSQL)
	b.WriteString(")")
	c.paramCount = leftCompiler.paramCount
	c.params = append(c.params, leftParams...)

	// Write operator
	b.WriteString(" ")
	b.WriteString(string(ast.SetOp.Op))
	b.WriteString(" ")

	// Compile right query
	rightCompiler := &PostgresCompiler{paramCount: c.paramCount}
	rightSQL, rightParams, err := rightCompiler.Compile(ast.SetOp.Right)
	if err != nil {
		return "", err
	}
	b.WriteString("(")
	b.WriteString(rightSQL)
	b.WriteString(")")
	c.paramCount = rightCompiler.paramCount
	c.params = append(c.params, rightParams...)

	// ORDER BY on combined result
	if len(ast.OrderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, ob := range ast.OrderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeExpr(&b, ob.Expr); err != nil {
				return "", err
			}
			if ob.Desc {
				b.WriteString(" DESC")
			}
		}
	}

	// LIMIT on combined result
	if ast.Limit != nil {
		b.WriteString(" LIMIT ")
		if err := c.writeExpr(&b, ast.Limit); err != nil {
			return "", err
		}
	}

	// OFFSET on combined result
	if ast.Offset != nil {
		b.WriteString(" OFFSET ")
		if err := c.writeExpr(&b, ast.Offset); err != nil {
			return "", err
		}
	}

	return b.String(), nil
}
