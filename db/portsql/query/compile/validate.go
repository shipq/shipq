package compile

import (
	"fmt"
	"regexp"

	"github.com/shipq/shipq/db/portsql/query"
)

// identifierRegex matches valid SQL identifiers.
// Identifiers must start with a letter or underscore, followed by letters, digits, or underscores.
var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier checks that a name is a valid SQL identifier.
// Returns an error if the identifier is invalid.
// Valid identifiers match: ^[a-zA-Z_][a-zA-Z0-9_]*$
func ValidateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if !identifierRegex.MatchString(name) {
		return fmt.Errorf("invalid identifier %q: must start with a letter or underscore and contain only letters, digits, and underscores", name)
	}
	return nil
}

// ValidateAST validates basic AST invariants before compilation.
// This catches common errors early with clear messages rather than
// producing invalid SQL or panicking during compilation.
func ValidateAST(ast *query.AST) error {
	if ast == nil {
		return fmt.Errorf("AST cannot be nil")
	}

	// For set operations, skip table validation as it's handled by the branches
	if ast.SetOp == nil {
		// Validate FromTable for non-set-operation queries
		if ast.FromTable.Name == "" && len(ast.CTEs) == 0 {
			return fmt.Errorf("FROM table name cannot be empty")
		}
	}

	// Validate based on query kind
	switch ast.Kind {
	case query.SelectQuery:
		if err := validateSelect(ast); err != nil {
			return err
		}
	case query.InsertQuery:
		if err := validateInsert(ast); err != nil {
			return err
		}
	case query.UpdateQuery:
		if err := validateUpdate(ast); err != nil {
			return err
		}
	case query.DeleteQuery:
		if err := validateDelete(ast); err != nil {
			return err
		}
	}

	// Validate JOINs
	for i, join := range ast.Joins {
		if join.Table.Name == "" {
			return fmt.Errorf("JOIN %d: table name cannot be empty", i)
		}
		if join.Condition == nil {
			return fmt.Errorf("JOIN %d: condition cannot be nil", i)
		}
		if err := validateExpr(join.Condition, fmt.Sprintf("JOIN %d condition", i)); err != nil {
			return err
		}
	}

	// Validate WHERE clause
	if ast.Where != nil {
		if err := validateExpr(ast.Where, "WHERE clause"); err != nil {
			return err
		}
	}

	// Validate HAVING clause
	if ast.Having != nil {
		if err := validateExpr(ast.Having, "HAVING clause"); err != nil {
			return err
		}
	}

	// Validate SELECT expressions
	for i, sel := range ast.SelectCols {
		if err := validateExpr(sel.Expr, fmt.Sprintf("SELECT column %d", i)); err != nil {
			return err
		}
	}

	// Validate INSERT values (all rows)
	for ri, row := range ast.InsertRows {
		for ci, val := range row {
			ctx := fmt.Sprintf("INSERT row %d value %d", ri, ci)
			if err := validateExpr(val, ctx); err != nil {
				return err
			}
		}
	}

	// Validate SET clauses
	for i, set := range ast.SetClauses {
		if err := validateExpr(set.Value, fmt.Sprintf("SET clause %d", i)); err != nil {
			return err
		}
	}

	// Validate CTEs
	for i, cte := range ast.CTEs {
		if err := ValidateIdentifier(cte.Name); err != nil {
			return fmt.Errorf("CTE %d: %w", i, err)
		}
		// Validate column names in CTE
		for j, col := range cte.Columns {
			if err := ValidateIdentifier(col); err != nil {
				return fmt.Errorf("CTE %q column %d: %w", cte.Name, j, err)
			}
		}
		// Recursively validate CTE query
		if cte.Query != nil {
			if err := ValidateAST(cte.Query); err != nil {
				return fmt.Errorf("CTE %q: %w", cte.Name, err)
			}
		}
	}

	// Validate set operation branches
	if ast.SetOp != nil {
		if ast.SetOp.Left == nil {
			return fmt.Errorf("set operation left branch cannot be nil")
		}
		if err := ValidateAST(ast.SetOp.Left); err != nil {
			return fmt.Errorf("set operation left branch: %w", err)
		}
		if ast.SetOp.Right == nil {
			return fmt.Errorf("set operation right branch cannot be nil")
		}
		if err := ValidateAST(ast.SetOp.Right); err != nil {
			return fmt.Errorf("set operation right branch: %w", err)
		}
	}

	return nil
}

// validateExpr recursively validates an expression.
func validateExpr(expr query.Expr, context string) error {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case query.ParamExpr:
		if e.Name == "" {
			return fmt.Errorf("%s: parameter name cannot be empty", context)
		}

	case query.SubqueryExpr:
		if e.Query == nil {
			return fmt.Errorf("%s: subquery cannot be nil", context)
		}
		if err := ValidateAST(e.Query); err != nil {
			return fmt.Errorf("%s subquery: %w", context, err)
		}

	case query.ExistsExpr:
		if e.Subquery == nil {
			return fmt.Errorf("%s: EXISTS subquery cannot be nil", context)
		}
		if err := ValidateAST(e.Subquery); err != nil {
			return fmt.Errorf("%s EXISTS subquery: %w", context, err)
		}

	case query.JSONAggExpr:
		if len(e.Fields) == 0 && len(e.Columns) == 0 {
			return fmt.Errorf("%s: JSON aggregation requires at least one column or field", context)
		}
		for i, f := range e.Fields {
			if f.Expr != nil {
				if err := validateExpr(f.Expr, fmt.Sprintf("%s json_agg field %d", context, i)); err != nil {
					return err
				}
			}
		}

	case query.BinaryExpr:
		if err := validateExpr(e.Left, context+" left"); err != nil {
			return err
		}
		if err := validateExpr(e.Right, context+" right"); err != nil {
			return err
		}

	case query.UnaryExpr:
		if err := validateExpr(e.Expr, context); err != nil {
			return err
		}

	case query.FuncExpr:
		for i, arg := range e.Args {
			if err := validateExpr(arg, fmt.Sprintf("%s arg %d", context, i)); err != nil {
				return err
			}
		}

	case query.ListExpr:
		for i, val := range e.Values {
			if err := validateExpr(val, fmt.Sprintf("%s list item %d", context, i)); err != nil {
				return err
			}
		}

	case query.AggregateExpr:
		if e.Arg != nil {
			if err := validateExpr(e.Arg, context+" aggregate arg"); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateSelect(ast *query.AST) error {
	// SELECT validation - nothing additional needed for now
	return nil
}

func validateInsert(ast *query.AST) error {
	hasRows := len(ast.InsertRows) > 0
	hasSource := ast.InsertSource != nil

	// Mutual exclusivity
	if hasRows && hasSource {
		return fmt.Errorf(
			"INSERT cannot have both InsertRows and InsertSource — use VALUES or SELECT, not both")
	}

	// Must have one or the other
	if !hasRows && !hasSource {
		return fmt.Errorf("INSERT requires either VALUES rows or a SELECT source")
	}

	// Validate VALUES-based insert (existing logic)
	if hasRows {
		colCount := len(ast.InsertCols)
		for i, row := range ast.InsertRows {
			if len(row) == 0 {
				return fmt.Errorf("INSERT row %d has no values", i)
			}
			if colCount > 0 && len(row) != colCount {
				return fmt.Errorf(
					"INSERT row %d: column count (%d) does not match value count (%d)",
					i, colCount, len(row))
			}
		}
		// All rows must have the same width even without explicit columns.
		firstWidth := len(ast.InsertRows[0])
		for i, row := range ast.InsertRows[1:] {
			if len(row) != firstWidth {
				return fmt.Errorf(
					"INSERT row %d has %d values, but row 0 has %d — all rows must match",
					i+1, len(row), firstWidth)
			}
		}
	}

	// Validate SELECT-based insert
	if hasSource {
		if ast.InsertSource.Kind != query.SelectQuery {
			return fmt.Errorf(
				"INSERT ... SELECT source must be a SELECT query, got %s",
				ast.InsertSource.Kind)
		}
		// Recursively validate the source query
		if err := ValidateAST(ast.InsertSource); err != nil {
			return fmt.Errorf("INSERT source query: %w", err)
		}
	}

	return nil
}

func validateUpdate(ast *query.AST) error {
	// UPDATE must have at least one SET clause
	if len(ast.SetClauses) == 0 {
		return fmt.Errorf("UPDATE requires at least one SET clause")
	}
	return nil
}

func validateDelete(ast *query.AST) error {
	// DELETE validation - nothing additional needed for now
	return nil
}
