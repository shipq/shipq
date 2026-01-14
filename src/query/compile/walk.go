package compile

import "github.com/portsql/portsql/src/query"

// ExprVisitor is called for each expression during a walk.
// Return false to stop walking the current branch.
type ExprVisitor func(expr query.Expr) bool

// WalkExpr traverses an expression tree in depth-first order, calling the visitor
// for each expression. If the visitor returns false, children of that expression
// are not visited.
func WalkExpr(expr query.Expr, visit ExprVisitor) {
	if expr == nil {
		return
	}

	if !visit(expr) {
		return
	}

	switch e := expr.(type) {
	case query.BinaryExpr:
		WalkExpr(e.Left, visit)
		WalkExpr(e.Right, visit)

	case query.UnaryExpr:
		WalkExpr(e.Expr, visit)

	case query.FuncExpr:
		for _, arg := range e.Args {
			WalkExpr(arg, visit)
		}

	case query.ListExpr:
		for _, val := range e.Values {
			WalkExpr(val, visit)
		}

	case query.AggregateExpr:
		WalkExpr(e.Arg, visit)

	case query.SubqueryExpr:
		if e.Query != nil {
			WalkAST(e.Query, visit)
		}

	case query.ExistsExpr:
		if e.Subquery != nil {
			WalkAST(e.Subquery, visit)
		}

	// These expression types have no child expressions:
	// - ColumnExpr
	// - ParamExpr
	// - LiteralExpr
	// - JSONAggExpr (columns are not expressions)
	}
}

// WalkAST traverses all expressions in an AST in depth-first order.
// The visitor is called for each expression found.
func WalkAST(ast *query.AST, visit ExprVisitor) {
	if ast == nil {
		return
	}

	// Walk SELECT columns
	for _, sel := range ast.SelectCols {
		WalkExpr(sel.Expr, visit)
	}

	// Walk joins
	for _, join := range ast.Joins {
		WalkExpr(join.Condition, visit)
	}

	// Walk WHERE
	WalkExpr(ast.Where, visit)

	// Walk HAVING
	WalkExpr(ast.Having, visit)

	// Note: GroupBy contains Columns, not Exprs, so we don't walk them here

	// Walk ORDER BY
	for _, ob := range ast.OrderBy {
		WalkExpr(ob.Expr, visit)
	}

	// Walk LIMIT and OFFSET
	WalkExpr(ast.Limit, visit)
	WalkExpr(ast.Offset, visit)

	// Walk INSERT values
	for _, val := range ast.InsertVals {
		WalkExpr(val, visit)
	}

	// Walk SET clauses
	for _, set := range ast.SetClauses {
		WalkExpr(set.Value, visit)
	}

	// Walk CTEs
	for _, cte := range ast.CTEs {
		WalkAST(cte.Query, visit)
	}

	// Walk set operation branches
	if ast.SetOp != nil {
		WalkAST(ast.SetOp.Left, visit)
		WalkAST(ast.SetOp.Right, visit)
	}
}

// CollectParams extracts all unique parameters from an AST.
// This is a convenience function that uses WalkAST.
func CollectParams(ast *query.AST) []query.ParamInfo {
	var params []query.ParamInfo
	seen := make(map[string]bool)

	WalkAST(ast, func(expr query.Expr) bool {
		if p, ok := expr.(query.ParamExpr); ok {
			if !seen[p.Name] {
				params = append(params, query.ParamInfo{
					Name:   p.Name,
					GoType: p.GoType,
				})
				seen[p.Name] = true
			}
		}
		return true
	})

	return params
}

// CollectParamOrder extracts parameters in occurrence order (with duplicates).
// This is useful for database drivers that use positional parameters.
func CollectParamOrder(ast *query.AST) []string {
	var order []string

	WalkAST(ast, func(expr query.Expr) bool {
		if p, ok := expr.(query.ParamExpr); ok {
			order = append(order, p.Name)
		}
		return true
	})

	return order
}

// HasSubqueries returns true if the AST contains any subquery expressions.
func HasSubqueries(ast *query.AST) bool {
	found := false
	WalkAST(ast, func(expr query.Expr) bool {
		switch expr.(type) {
		case query.SubqueryExpr, query.ExistsExpr:
			found = true
			return false // Stop walking
		}
		return true
	})
	return found
}
