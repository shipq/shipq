// Package httpserver provides HTTP server infrastructure for generated servers.
package httpserver

import (
	"context"
	"database/sql"
)

// Querier is the interface for database operations.
// Both *sql.DB and *sql.Tx implement this interface.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Pinger is an interface for checking database connectivity.
// *sql.DB implements this interface.
type Pinger interface {
	Ping() error
}

// PingableQuerier combines Querier with Pinger for database connections
// that support both query operations and health checks.
// *sql.DB implements this interface.
type PingableQuerier interface {
	Querier
	Pinger
}

type querierKey struct{}

// WithQuerier returns a new context with the Querier attached.
func WithQuerier(ctx context.Context, q Querier) context.Context {
	return context.WithValue(ctx, querierKey{}, q)
}

// GetQuerier retrieves the Querier from context.
// Panics if no Querier is present (indicates programmer error).
func GetQuerier(ctx context.Context) Querier {
	q, ok := ctx.Value(querierKey{}).(Querier)
	if !ok {
		panic("httpserver: no Querier in context - did you forget to use WithQuerier?")
	}
	return q
}
