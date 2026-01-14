package compile

import (
	"fmt"
	"strings"

	"github.com/portsql/portsql/src/query"
)

// Dialect defines the SQL dialect-specific behavior for compilation.
// Each dialect (Postgres, MySQL, SQLite) implements this interface
// to customize identifier quoting, placeholders, literals, and special functions.
type Dialect interface {
	// Name returns the dialect name for debugging/logging.
	Name() string

	// QuoteIdentifier quotes an identifier (table name, column name, alias).
	QuoteIdentifier(name string) string

	// Placeholder returns the parameter placeholder for the given index (1-based).
	// Postgres uses $1, $2, etc. MySQL and SQLite use ?.
	Placeholder(index int) string

	// BoolLiteral returns the SQL literal for a boolean value.
	// Postgres uses TRUE/FALSE, MySQL and SQLite use 1/0.
	BoolLiteral(val bool) string

	// NowFunc returns the SQL function for current timestamp.
	// Postgres/MySQL use NOW(), SQLite uses datetime('now').
	NowFunc() string

	// WrapSetOpQueries returns true if set operation queries should be wrapped in parentheses.
	// Postgres and MySQL require this, SQLite does not support it.
	WrapSetOpQueries() bool

	// SupportsReturning returns true if the dialect supports the RETURNING clause
	// in INSERT/UPDATE/DELETE statements. Postgres and SQLite (3.35+) support this,
	// MySQL does not (it uses LAST_INSERT_ID() instead).
	SupportsReturning() bool

	// WriteILIKE writes a case-insensitive LIKE expression.
	// Postgres has native ILIKE, others need LOWER() LIKE LOWER().
	// The writeExpr callback should be used to write the arguments.
	WriteILIKE(b *strings.Builder, args []query.Expr, writeExpr func(query.Expr) error) error

	// WriteJSONAgg writes a JSON aggregation expression.
	// Each dialect has different JSON functions.
	// The writeColumn callback should be used to write column references.
	// Returns an error if cols is empty.
	WriteJSONAgg(b *strings.Builder, cols []query.Column, writeColumn func(query.Column)) error

	// WriteOrderByExpr writes an expression for ORDER BY clause.
	// MySQL needs special COLLATE handling for string columns.
	// The writeExpr and writeColumn callbacks are for writing sub-expressions.
	WriteOrderByExpr(b *strings.Builder, expr query.Expr, writeExpr func(query.Expr) error, writeColumn func(query.Column)) error
}

// CompilerState holds the mutable state during compilation.
// This is separate from Dialect to allow proper subquery handling.
type CompilerState struct {
	ParamCount int
	Params     []string
}

// =============================================================================
// Shared Helpers
// =============================================================================

// writeILIKEWithLower is a shared helper for dialects that don't have native ILIKE.
// It emulates ILIKE using LOWER(x) LIKE LOWER(y).
func writeILIKEWithLower(b *strings.Builder, args []query.Expr, writeExpr func(query.Expr) error) error {
	if len(args) != 2 {
		return fmt.Errorf("ILIKE requires exactly 2 arguments")
	}
	b.WriteString("LOWER(")
	if err := writeExpr(args[0]); err != nil {
		return err
	}
	b.WriteString(") LIKE LOWER(")
	if err := writeExpr(args[1]); err != nil {
		return err
	}
	b.WriteString(")")
	return nil
}

// =============================================================================
// Postgres Dialect
// =============================================================================

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct{}

func (d *PostgresDialect) Name() string { return "postgres" }

func (d *PostgresDialect) QuoteIdentifier(name string) string {
	// Escape embedded double quotes by doubling them
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}

func (d *PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (d *PostgresDialect) BoolLiteral(val bool) string {
	if val {
		return "TRUE"
	}
	return "FALSE"
}

func (d *PostgresDialect) NowFunc() string {
	return "NOW()"
}

func (d *PostgresDialect) WrapSetOpQueries() bool {
	return true
}

func (d *PostgresDialect) SupportsReturning() bool {
	return true
}

func (d *PostgresDialect) WriteILIKE(b *strings.Builder, args []query.Expr, writeExpr func(query.Expr) error) error {
	// Postgres has native ILIKE
	if len(args) != 2 {
		return fmt.Errorf("ILIKE requires exactly 2 arguments")
	}
	if err := writeExpr(args[0]); err != nil {
		return err
	}
	b.WriteString(" ILIKE ")
	return writeExpr(args[1])
}

func (d *PostgresDialect) WriteJSONAgg(b *strings.Builder, cols []query.Column, writeColumn func(query.Column)) error {
	if len(cols) == 0 {
		return fmt.Errorf("JSON aggregation requires at least one column")
	}
	// COALESCE(JSON_AGG(JSON_BUILD_OBJECT(...)) FILTER (WHERE ... IS NOT NULL), '[]')
	b.WriteString("COALESCE(JSON_AGG(JSON_BUILD_OBJECT(")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		// Key is column name
		fmt.Fprintf(b, "'%s', ", col.ColumnName())
		writeColumn(col)
	}
	b.WriteString(")) FILTER (WHERE ")
	// Use first column for null check
	writeColumn(cols[0])
	b.WriteString(" IS NOT NULL), '[]')")
	return nil
}

func (d *PostgresDialect) WriteOrderByExpr(b *strings.Builder, expr query.Expr, writeExpr func(query.Expr) error, writeColumn func(query.Column)) error {
	// Postgres: no special handling needed
	return writeExpr(expr)
}

// =============================================================================
// MySQL Dialect
// =============================================================================

// MySQLDialect implements Dialect for MySQL.
type MySQLDialect struct{}

func (d *MySQLDialect) Name() string { return "mysql" }

func (d *MySQLDialect) QuoteIdentifier(name string) string {
	// Escape embedded backticks by doubling them
	escaped := strings.ReplaceAll(name, "`", "``")
	return "`" + escaped + "`"
}

func (d *MySQLDialect) Placeholder(index int) string {
	return "?"
}

func (d *MySQLDialect) BoolLiteral(val bool) string {
	if val {
		return "1"
	}
	return "0"
}

func (d *MySQLDialect) NowFunc() string {
	return "NOW()"
}

func (d *MySQLDialect) WrapSetOpQueries() bool {
	return true
}

func (d *MySQLDialect) SupportsReturning() bool {
	return false // MySQL uses LAST_INSERT_ID() instead
}

func (d *MySQLDialect) WriteILIKE(b *strings.Builder, args []query.Expr, writeExpr func(query.Expr) error) error {
	// MySQL doesn't have native ILIKE, use LOWER() LIKE LOWER()
	return writeILIKEWithLower(b, args, writeExpr)
}

func (d *MySQLDialect) WriteJSONAgg(b *strings.Builder, cols []query.Column, writeColumn func(query.Column)) error {
	if len(cols) == 0 {
		return fmt.Errorf("JSON aggregation requires at least one column")
	}
	// COALESCE(JSON_ARRAYAGG(CASE WHEN col IS NOT NULL THEN JSON_OBJECT(...) END), JSON_ARRAY())
	// Use CASE WHEN to produce null entries for LEFT JOIN no-match rows,
	// which are filtered out during Go unmarshal.
	b.WriteString("COALESCE(JSON_ARRAYAGG(CASE WHEN ")
	writeColumn(cols[0])
	b.WriteString(" IS NOT NULL THEN JSON_OBJECT(")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		// Key is column name as string literal
		fmt.Fprintf(b, "'%s', ", col.ColumnName())
		writeColumn(col)
	}
	b.WriteString(") END), JSON_ARRAY())")
	return nil
}

func (d *MySQLDialect) WriteOrderByExpr(b *strings.Builder, expr query.Expr, writeExpr func(query.Expr) error, writeColumn func(query.Column)) error {
	// MySQL: Add COLLATE utf8mb4_bin to string columns for case-sensitive sorting
	if colExpr, ok := expr.(query.ColumnExpr); ok {
		goType := colExpr.Column.GoType()
		// Apply binary collation to string types for case-sensitive ordering
		if goType == "string" || goType == "*string" {
			writeColumn(colExpr.Column)
			b.WriteString(" COLLATE utf8mb4_bin")
			return nil
		}
	}
	// For non-string columns, use normal expression writing
	return writeExpr(expr)
}

// =============================================================================
// SQLite Dialect
// =============================================================================

// SQLiteDialect implements Dialect for SQLite.
type SQLiteDialect struct{}

func (d *SQLiteDialect) Name() string { return "sqlite" }

func (d *SQLiteDialect) QuoteIdentifier(name string) string {
	// Escape embedded double quotes by doubling them
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}

func (d *SQLiteDialect) Placeholder(index int) string {
	return "?"
}

func (d *SQLiteDialect) BoolLiteral(val bool) string {
	if val {
		return "1"
	}
	return "0"
}

func (d *SQLiteDialect) NowFunc() string {
	return "datetime('now')"
}

func (d *SQLiteDialect) WrapSetOpQueries() bool {
	return false
}

func (d *SQLiteDialect) SupportsReturning() bool {
	return true // SQLite 3.35+ supports RETURNING
}

func (d *SQLiteDialect) WriteILIKE(b *strings.Builder, args []query.Expr, writeExpr func(query.Expr) error) error {
	// SQLite doesn't have native ILIKE, use LOWER() LIKE LOWER()
	return writeILIKEWithLower(b, args, writeExpr)
}

func (d *SQLiteDialect) WriteJSONAgg(b *strings.Builder, cols []query.Column, writeColumn func(query.Column)) error {
	if len(cols) == 0 {
		return fmt.Errorf("JSON aggregation requires at least one column")
	}
	// COALESCE(JSON_GROUP_ARRAY(CASE WHEN col IS NOT NULL THEN JSON_OBJECT(...) END), '[]')
	// Use CASE WHEN to produce null entries for LEFT JOIN no-match rows,
	// which are filtered out during Go unmarshal.
	b.WriteString("COALESCE(JSON_GROUP_ARRAY(CASE WHEN ")
	writeColumn(cols[0])
	b.WriteString(" IS NOT NULL THEN JSON_OBJECT(")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		// Key is column name as string literal
		fmt.Fprintf(b, "'%s', ", col.ColumnName())
		writeColumn(col)
	}
	b.WriteString(") END), '[]')")
	return nil
}

func (d *SQLiteDialect) WriteOrderByExpr(b *strings.Builder, expr query.Expr, writeExpr func(query.Expr) error, writeColumn func(query.Column)) error {
	// SQLite: no special handling needed
	return writeExpr(expr)
}

// =============================================================================
// Dialect Singletons
// =============================================================================

var (
	// Postgres is the singleton PostgreSQL dialect.
	Postgres Dialect = &PostgresDialect{}

	// MySQL is the singleton MySQL dialect.
	MySQL Dialect = &MySQLDialect{}

	// SQLite is the singleton SQLite dialect.
	SQLite Dialect = &SQLiteDialect{}
)
