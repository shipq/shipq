package compile

import (
	"fmt"
	"strings"

	"github.com/portsql/portsql/src/query"
)

// MySQLCompiler compiles AST to MySQL SQL.
type MySQLCompiler struct {
	params []string // Tracks param names in order
}

// Compile compiles an AST to MySQL SQL.
// Returns the SQL string and the parameter names in order.
func (c *MySQLCompiler) Compile(ast *query.AST) (sql string, paramOrder []string, err error) {
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

// CompileMySQL is a convenience function.
func CompileMySQL(ast *query.AST) (string, []string, error) {
	c := &MySQLCompiler{}
	return c.Compile(ast)
}

func (c *MySQLCompiler) compileSelect(ast *query.AST) (string, error) {
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
				if err := ValidateIdentifier(col.Alias); err != nil {
					return "", fmt.Errorf("invalid column alias: %w", err)
				}
				b.WriteString(" AS ")
				c.writeIdentifier(&b, col.Alias)
			}
		}
	}

	// FROM clause
	b.WriteString(" FROM ")
	c.writeIdentifier(&b, ast.FromTable.Name)
	if ast.FromTable.Alias != "" {
		if err := ValidateIdentifier(ast.FromTable.Alias); err != nil {
			return "", fmt.Errorf("invalid table alias: %w", err)
		}
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
			if err := ValidateIdentifier(join.Table.Alias); err != nil {
				return "", fmt.Errorf("invalid join table alias: %w", err)
			}
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
			if err := c.writeOrderByExpr(&b, ob.Expr); err != nil {
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

func (c *MySQLCompiler) compileInsert(ast *query.AST) (string, error) {
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

	// NOTE: No RETURNING clause for MySQL!
	// The generated Go code uses result.LastInsertId() instead

	return b.String(), nil
}

func (c *MySQLCompiler) compileUpdate(ast *query.AST) (string, error) {
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

func (c *MySQLCompiler) compileDelete(ast *query.AST) (string, error) {
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

func (c *MySQLCompiler) writeExpr(b *strings.Builder, expr query.Expr) error {
	switch e := expr.(type) {
	case query.ColumnExpr:
		c.writeColumn(b, e.Column)

	case query.ParamExpr:
		c.params = append(c.params, e.Name)
		b.WriteString("?")

	case query.LiteralExpr:
		if err := c.writeLiteral(b, e.Value); err != nil {
			return err
		}

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
		subCompiler := &MySQLCompiler{}
		subSQL, subParams, err := subCompiler.Compile(e.Query)
		if err != nil {
			return err
		}
		b.WriteString(subSQL)
		b.WriteString(")")
		c.params = append(c.params, subParams...)

	case query.ExistsExpr:
		if e.Negated {
			b.WriteString("NOT ")
		}
		b.WriteString("EXISTS (")
		subCompiler := &MySQLCompiler{}
		subSQL, subParams, err := subCompiler.Compile(e.Subquery)
		if err != nil {
			return err
		}
		b.WriteString(subSQL)
		b.WriteString(")")
		c.params = append(c.params, subParams...)

	default:
		return fmt.Errorf("unknown expression type: %T", expr)
	}

	return nil
}

// writeOrderByExpr writes an expression for ORDER BY, adding COLLATE utf8mb4_bin
// to string columns to ensure case-sensitive sorting that matches Postgres/SQLite behavior.
// MySQL's default collation is case-insensitive, which causes different ordering.
func (c *MySQLCompiler) writeOrderByExpr(b *strings.Builder, expr query.Expr) error {
	// Check if this is a string column that needs binary collation
	if colExpr, ok := expr.(query.ColumnExpr); ok {
		goType := colExpr.Column.GoType()
		// Apply binary collation to string types for case-sensitive ordering
		if goType == "string" || goType == "*string" {
			c.writeColumn(b, colExpr.Column)
			b.WriteString(" COLLATE utf8mb4_bin")
			return nil
		}
	}
	// For non-string columns, use normal expression writing
	return c.writeExpr(b, expr)
}

func (c *MySQLCompiler) writeIdentifier(b *strings.Builder, name string) {
	// MySQL uses backticks for identifiers
	fmt.Fprintf(b, "`%s`", name)
}

func (c *MySQLCompiler) writeColumn(b *strings.Builder, col query.Column) {
	c.writeIdentifier(b, col.TableName())
	b.WriteString(".")
	c.writeIdentifier(b, col.ColumnName())
}

func (c *MySQLCompiler) writeLiteral(b *strings.Builder, val any) error {
	switch v := val.(type) {
	case string:
		// Escape single quotes by doubling them
		escaped := strings.ReplaceAll(v, "'", "''")
		fmt.Fprintf(b, "'%s'", escaped)
	case bool:
		// MySQL uses 1/0 for booleans
		if v {
			b.WriteString("1")
		} else {
			b.WriteString("0")
		}
	case nil:
		b.WriteString("NULL")
	case int:
		fmt.Fprintf(b, "%d", v)
	case int8:
		fmt.Fprintf(b, "%d", v)
	case int16:
		fmt.Fprintf(b, "%d", v)
	case int32:
		fmt.Fprintf(b, "%d", v)
	case int64:
		fmt.Fprintf(b, "%d", v)
	case uint:
		fmt.Fprintf(b, "%d", v)
	case uint8:
		fmt.Fprintf(b, "%d", v)
	case uint16:
		fmt.Fprintf(b, "%d", v)
	case uint32:
		fmt.Fprintf(b, "%d", v)
	case uint64:
		fmt.Fprintf(b, "%d", v)
	case float32:
		fmt.Fprintf(b, "%g", v)
	case float64:
		fmt.Fprintf(b, "%g", v)
	default:
		return fmt.Errorf("unsupported literal type %T: only string, bool, nil, int*, uint*, and float* are allowed", val)
	}
	return nil
}

func (c *MySQLCompiler) writeFunc(b *strings.Builder, f query.FuncExpr) error {
	switch f.Name {
	case "NOW":
		b.WriteString("NOW()")
	case "ILIKE":
		// MySQL: ILIKE becomes LOWER() LIKE LOWER()
		if len(f.Args) != 2 {
			return fmt.Errorf("ILIKE requires exactly 2 arguments")
		}
		b.WriteString("LOWER(")
		if err := c.writeExpr(b, f.Args[0]); err != nil {
			return err
		}
		b.WriteString(") LIKE LOWER(")
		if err := c.writeExpr(b, f.Args[1]); err != nil {
			return err
		}
		b.WriteString(")")
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

func (c *MySQLCompiler) writeJSONAgg(b *strings.Builder, j query.JSONAggExpr) {
	// COALESCE(JSON_ARRAYAGG(JSON_OBJECT(...)), JSON_ARRAY())
	b.WriteString("COALESCE(JSON_ARRAYAGG(JSON_OBJECT(")
	for i, col := range j.Columns {
		if i > 0 {
			b.WriteString(", ")
		}
		// Key is column name as string literal
		fmt.Fprintf(b, "'%s', ", col.ColumnName())
		c.writeColumn(b, col)
	}
	b.WriteString(")), JSON_ARRAY())")
}

// =============================================================================
// CTE Compilation
// =============================================================================

func (c *MySQLCompiler) writeCTEs(b *strings.Builder, ctes []query.CTE) error {
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
		cteCompiler := &MySQLCompiler{}
		cteSQL, cteParams, err := cteCompiler.Compile(cte.Query)
		if err != nil {
			return err
		}
		b.WriteString(cteSQL)
		b.WriteString(")")
		c.params = append(c.params, cteParams...)
	}
	b.WriteString(" ")
	return nil
}

// =============================================================================
// Set Operation Compilation (UNION, INTERSECT, EXCEPT)
// =============================================================================

func (c *MySQLCompiler) compileSetOp(ast *query.AST) (string, error) {
	var b strings.Builder

	// Compile left query
	leftCompiler := &MySQLCompiler{}
	leftSQL, leftParams, err := leftCompiler.Compile(ast.SetOp.Left)
	if err != nil {
		return "", err
	}
	b.WriteString("(")
	b.WriteString(leftSQL)
	b.WriteString(")")
	c.params = append(c.params, leftParams...)

	// Write operator
	b.WriteString(" ")
	b.WriteString(string(ast.SetOp.Op))
	b.WriteString(" ")

	// Compile right query
	rightCompiler := &MySQLCompiler{}
	rightSQL, rightParams, err := rightCompiler.Compile(ast.SetOp.Right)
	if err != nil {
		return "", err
	}
	b.WriteString("(")
	b.WriteString(rightSQL)
	b.WriteString(")")
	c.params = append(c.params, rightParams...)

	// ORDER BY on combined result
	if len(ast.OrderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, ob := range ast.OrderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeOrderByExpr(&b, ob.Expr); err != nil {
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
