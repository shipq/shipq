package compile

import (
	"github.com/portsql/portsql/src/query"
)

// PostgresCompiler compiles AST to Postgres SQL.
// Deprecated: Use NewCompiler(Postgres) instead for new code.
// This type is kept for backward compatibility.
type PostgresCompiler struct{}

// Compile compiles an AST to Postgres SQL.
// Returns the SQL string and the parameter names in order.
func (c *PostgresCompiler) Compile(ast *query.AST) (sql string, paramOrder []string, err error) {
	compiler := NewCompiler(Postgres)
	return compiler.Compile(ast)
}

// CompilePostgres is a convenience function that compiles an AST to Postgres SQL.
func CompilePostgres(ast *query.AST) (string, []string, error) {
	compiler := NewCompiler(Postgres)
	return compiler.Compile(ast)
}
