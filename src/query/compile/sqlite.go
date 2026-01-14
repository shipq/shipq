package compile

import (
	"github.com/portsql/portsql/src/query"
)

// SQLiteCompiler compiles AST to SQLite SQL.
// Deprecated: Use NewCompiler(SQLite) instead for new code.
// This type is kept for backward compatibility.
type SQLiteCompiler struct{}

// Compile compiles an AST to SQLite SQL.
// Returns the SQL string and the parameter names in order.
func (c *SQLiteCompiler) Compile(ast *query.AST) (sql string, paramOrder []string, err error) {
	compiler := NewCompiler(SQLite)
	return compiler.Compile(ast)
}

// CompileSQLite is a convenience function that compiles an AST to SQLite SQL.
func CompileSQLite(ast *query.AST) (string, []string, error) {
	compiler := NewCompiler(SQLite)
	return compiler.Compile(ast)
}
