package compile

import (
	"github.com/portsql/portsql/src/query"
)

// MySQLCompiler compiles AST to MySQL SQL.
// Deprecated: Use NewCompiler(MySQL) instead for new code.
// This type is kept for backward compatibility.
type MySQLCompiler struct{}

// Compile compiles an AST to MySQL SQL.
// Returns the SQL string and the parameter names in order.
func (c *MySQLCompiler) Compile(ast *query.AST) (sql string, paramOrder []string, err error) {
	compiler := NewCompiler(MySQL)
	return compiler.Compile(ast)
}

// CompileMySQL is a convenience function that compiles an AST to MySQL SQL.
func CompileMySQL(ast *query.AST) (string, []string, error) {
	compiler := NewCompiler(MySQL)
	return compiler.Compile(ast)
}
