package query

import (
	"context"
	"database/sql"
)

// Dialect identifies the target database.
type Dialect int

const (
	Postgres Dialect = iota
	MySQL
	SQLite
)

// String returns the dialect name.
func (d Dialect) String() string {
	switch d {
	case Postgres:
		return "postgres"
	case MySQL:
		return "mysql"
	case SQLite:
		return "sqlite"
	default:
		return "unknown"
	}
}

// Querier is the interface for executing queries.
// Both *sql.DB and *sql.Tx implement this interface.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Compile-time checks that *sql.DB and *sql.Tx implement Querier
var (
	_ Querier = (*sql.DB)(nil)
	_ Querier = (*sql.Tx)(nil)
)

// QueryRunner holds pre-selected SQL strings for a specific dialect.
// This avoids switch statements on every query execution.
//
// The runner is created once at startup with the desired dialect,
// and query methods use the pre-selected SQL strings directly.
// This pattern provides:
// 1. O(1) dialect selection at construction time
// 2. No runtime overhead for dialect switching per query
// 3. Easy transaction support via WithTx
type QueryRunner struct {
	dialect Dialect
	db      Querier

	// Pre-selected SQL strings are set at construction time based on dialect.
	// The generated code will add fields here for each query, e.g.:
	//   getAuthorByIDSQL      string
	//   insertAuthorSQL       string
	//   etc.
	//
	// For Phase 1, we keep this minimal as actual SQL generation
	// comes in later phases.
}

// NewQueryRunner creates a runner for the given dialect.
// All SQL strings are selected once here, not on every query.
func NewQueryRunner(db Querier, dialect Dialect) *QueryRunner {
	return &QueryRunner{
		dialect: dialect,
		db:      db,
	}
}

// Dialect returns the runner's dialect.
func (r *QueryRunner) Dialect() Dialect {
	return r.dialect
}

// DB returns the runner's database connection.
func (r *QueryRunner) DB() Querier {
	return r.db
}

// WithTx returns a new QueryRunner using the given transaction.
// SQL strings are already selected, so no additional overhead.
func (r *QueryRunner) WithTx(tx *sql.Tx) *QueryRunner {
	return &QueryRunner{
		dialect: r.dialect,
		db:      tx,
		// Copy all pre-selected SQL strings from the original runner
		// (generated code will add assignments for each query field)
	}
}

// WithDB returns a new QueryRunner using the given database connection.
// This can be used to switch the underlying connection while keeping
// the same dialect and SQL strings.
func (r *QueryRunner) WithDB(db Querier) *QueryRunner {
	return &QueryRunner{
		dialect: r.dialect,
		db:      db,
	}
}
