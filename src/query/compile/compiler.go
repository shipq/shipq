package compile

import (
	"fmt"
	"strings"

	"github.com/portsql/portsql/src/query"
)

// Compiler compiles AST to SQL for a specific dialect.
type Compiler struct {
	dialect Dialect
	state   *CompilerState
}

// NewCompiler creates a new compiler for the given dialect.
func NewCompiler(dialect Dialect) *Compiler {
	return &Compiler{
		dialect: dialect,
		state:   &CompilerState{},
	}
}

// Compile compiles an AST to SQL.
// Returns the SQL string and the parameter names in order (including duplicates).
func (c *Compiler) Compile(ast *query.AST) (sql string, paramOrder []string, err error) {
	// Validate AST invariants early
	if err := ValidateAST(ast); err != nil {
		return "", nil, err
	}

	// Reset state once at the top level
	c.state.ParamCount = 0
	c.state.Params = nil

	var b strings.Builder
	if err := c.compileInto(ast, &b); err != nil {
		return "", nil, err
	}

	return b.String(), c.state.Params, nil
}

// compileInto is the internal compilation method that does NOT reset state.
// This allows nested compilation (subqueries, CTEs, set operations) to share
// state with the parent compilation, ensuring correct parameter numbering.
func (c *Compiler) compileInto(ast *query.AST, b *strings.Builder) error {
	// Handle CTEs (WITH clause) first
	if len(ast.CTEs) > 0 {
		if err := c.writeCTEs(b, ast.CTEs); err != nil {
			return err
		}
	}

	// Handle set operations
	if ast.SetOp != nil {
		return c.compileSetOpInto(ast, b)
	}

	var sql string
	var err error

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
		return err
	}

	b.WriteString(sql)
	return nil
}

// =============================================================================
// SELECT Compilation
// =============================================================================

func (c *Compiler) compileSelect(ast *query.AST) (string, error) {
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

// =============================================================================
// INSERT Compilation
// =============================================================================

func (c *Compiler) compileInsert(ast *query.AST) (string, error) {
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

	// RETURNING clause (Postgres and SQLite support this, MySQL doesn't)
	// Note: MySQL codegen handles RETURNING differently by using result.LastInsertId()
	if len(ast.Returning) > 0 && c.dialect.SupportsReturning() {
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

// =============================================================================
// UPDATE Compilation
// =============================================================================

func (c *Compiler) compileUpdate(ast *query.AST) (string, error) {
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

// =============================================================================
// DELETE Compilation
// =============================================================================

func (c *Compiler) compileDelete(ast *query.AST) (string, error) {
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

// =============================================================================
// Expression Writing
// =============================================================================

func (c *Compiler) writeExpr(b *strings.Builder, expr query.Expr) error {
	switch e := expr.(type) {
	case query.ColumnExpr:
		c.writeColumn(b, e.Column)

	case query.ParamExpr:
		c.state.ParamCount++
		c.state.Params = append(c.state.Params, e.Name)
		b.WriteString(c.dialect.Placeholder(c.state.ParamCount))

	case query.LiteralExpr:
		if err := c.writeLiteral(b, e.Value); err != nil {
			return err
		}

	case query.BinaryExpr:
		if e.Op == query.OpIn {
			// Wrap IN expression in parentheses for consistency with other binary operators
			b.WriteString("(")
			if err := c.writeExpr(b, e.Left); err != nil {
				return err
			}
			b.WriteString(" IN ")
			// Handle both ListExpr and SubqueryExpr
			switch right := e.Right.(type) {
			case query.ListExpr:
				// Validate non-empty list - IN () is invalid SQL
				if len(right.Values) == 0 {
					return fmt.Errorf("IN clause requires at least one value")
				}
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
			b.WriteString(")")
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
		if err := c.writeJSONAgg(b, e); err != nil {
			return err
		}

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
		// Use compileInto to share state with parent, ensuring correct param numbering
		b.WriteString("(")
		if err := c.compileInto(e.Query, b); err != nil {
			return err
		}
		b.WriteString(")")

	case query.ExistsExpr:
		if e.Negated {
			b.WriteString("NOT ")
		}
		b.WriteString("EXISTS (")
		// Use compileInto to share state with parent, ensuring correct param numbering
		if err := c.compileInto(e.Subquery, b); err != nil {
			return err
		}
		b.WriteString(")")

	default:
		return fmt.Errorf("unknown expression type: %T", expr)
	}

	return nil
}

func (c *Compiler) writeIdentifier(b *strings.Builder, name string) {
	b.WriteString(c.dialect.QuoteIdentifier(name))
}

func (c *Compiler) writeColumn(b *strings.Builder, col query.Column) {
	c.writeIdentifier(b, col.TableName())
	b.WriteString(".")
	c.writeIdentifier(b, col.ColumnName())
}

func (c *Compiler) writeLiteral(b *strings.Builder, val any) error {
	switch v := val.(type) {
	case string:
		// Escape single quotes by doubling them
		escaped := strings.ReplaceAll(v, "'", "''")
		fmt.Fprintf(b, "'%s'", escaped)
	case bool:
		b.WriteString(c.dialect.BoolLiteral(v))
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

func (c *Compiler) writeFunc(b *strings.Builder, f query.FuncExpr) error {
	switch f.Name {
	case "NOW":
		b.WriteString(c.dialect.NowFunc())
	case "ILIKE":
		return c.dialect.WriteILIKE(b, f.Args, func(e query.Expr) error {
			return c.writeExpr(b, e)
		})
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

func (c *Compiler) writeJSONAgg(b *strings.Builder, j query.JSONAggExpr) error {
	return c.dialect.WriteJSONAgg(b, j.Columns, func(col query.Column) {
		c.writeColumn(b, col)
	})
}

func (c *Compiler) writeOrderByExpr(b *strings.Builder, expr query.Expr) error {
	return c.dialect.WriteOrderByExpr(b, expr, func(e query.Expr) error {
		return c.writeExpr(b, e)
	}, func(col query.Column) {
		c.writeColumn(b, col)
	})
}

// =============================================================================
// CTE Compilation
// =============================================================================

func (c *Compiler) writeCTEs(b *strings.Builder, ctes []query.CTE) error {
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
		// Use compileInto to share state with parent, ensuring correct param numbering
		if err := c.compileInto(cte.Query, b); err != nil {
			return err
		}
		b.WriteString(")")
	}
	b.WriteString(" ")
	return nil
}

// =============================================================================
// Set Operation Compilation (UNION, INTERSECT, EXCEPT)
// =============================================================================

// compileSetOpInto compiles a set operation into the provided builder,
// sharing state with the parent compilation for correct param numbering.
func (c *Compiler) compileSetOpInto(ast *query.AST, b *strings.Builder) error {
	// Compile left query using compileInto to share state
	if c.dialect.WrapSetOpQueries() {
		b.WriteString("(")
	}
	if err := c.compileInto(ast.SetOp.Left, b); err != nil {
		return err
	}
	if c.dialect.WrapSetOpQueries() {
		b.WriteString(")")
	}

	// Write operator
	b.WriteString(" ")
	b.WriteString(string(ast.SetOp.Op))
	b.WriteString(" ")

	// Compile right query using compileInto to share state
	if c.dialect.WrapSetOpQueries() {
		b.WriteString("(")
	}
	if err := c.compileInto(ast.SetOp.Right, b); err != nil {
		return err
	}
	if c.dialect.WrapSetOpQueries() {
		b.WriteString(")")
	}

	// ORDER BY on combined result
	if len(ast.OrderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, ob := range ast.OrderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := c.writeOrderByExpr(b, ob.Expr); err != nil {
				return err
			}
			if ob.Desc {
				b.WriteString(" DESC")
			}
		}
	}

	// LIMIT on combined result
	if ast.Limit != nil {
		b.WriteString(" LIMIT ")
		if err := c.writeExpr(b, ast.Limit); err != nil {
			return err
		}
	}

	// OFFSET on combined result
	if ast.Offset != nil {
		b.WriteString(" OFFSET ")
		if err := c.writeExpr(b, ast.Offset); err != nil {
			return err
		}
	}

	return nil
}
