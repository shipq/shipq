package codegen

import "fmt"

// CRUDOptions configures CRUD generation behavior.
type CRUDOptions struct {
	// ScopeColumn, if set, adds this column to WHERE clauses.
	// The column must exist in the table.
	// Example: "organization_id", "tenant_id", "user_id"
	ScopeColumn string

	// OrderAsc, if true, orders by created_at ASC (oldest first).
	// Default is false (newest first, DESC).
	OrderAsc bool
}

// SQLDialect represents a database dialect for SQL generation.
type SQLDialect string

const (
	SQLDialectPostgres SQLDialect = "postgres"
	SQLDialectMySQL    SQLDialect = "mysql"
	SQLDialectSQLite   SQLDialect = "sqlite"
)

// QuoteIdentifier quotes an identifier based on dialect.
func QuoteIdentifier(name string, dialect SQLDialect) string {
	switch dialect {
	case SQLDialectMySQL:
		return "`" + name + "`"
	default: // Postgres, SQLite use double quotes
		return `"` + name + `"`
	}
}

// Placeholder returns the parameter placeholder for the given index (1-based).
func Placeholder(index int, dialect SQLDialect) string {
	switch dialect {
	case SQLDialectPostgres:
		return fmt.Sprintf("$%d", index)
	default: // MySQL, SQLite use ?
		return "?"
	}
}

// NowFunc returns the NOW() function for the dialect.
func NowFunc(dialect SQLDialect) string {
	switch dialect {
	case SQLDialectSQLite:
		return "datetime('now')"
	default: // Postgres, MySQL
		return "NOW()"
	}
}
