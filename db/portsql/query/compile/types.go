package compile

// Result holds the output of compiling an AST to SQL.
// This is the canonical "single source of truth" for compilation output.
type Result struct {
	// SQL is the compiled SQL string for the query.
	SQL string

	// ParamOrder contains the parameter names in the order they appear in the SQL.
	// This includes duplicates - if a param is used twice, it appears twice.
	// This list should be used to build the args slice when executing the query.
	ParamOrder []string
}

// CompileResult creates a Result from a SQL string and param order.
// This is a convenience constructor.
func CompileResult(sql string, paramOrder []string) Result {
	return Result{
		SQL:        sql,
		ParamOrder: paramOrder,
	}
}
