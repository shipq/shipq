// Package httpserver provides HTTP server infrastructure for generated servers.
package httpserver

import (
	"context"
	"database/sql"
	"net/http"
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

// Cookie operation types and context keys

type cookieOpsKey struct{}
type requestCookiesKey struct{}

// CookieOp represents a cookie operation to be applied after the handler returns.
type CookieOp struct {
	Cookie *http.Cookie
}

// WithCookieOps returns a new context with a cookie operations slice attached.
// The returned slice pointer can be used to collect cookies set by handlers.
// This is called by the generated wrapHandler function.
func WithCookieOps(ctx context.Context) (context.Context, *[]CookieOp) {
	ops := &[]CookieOp{}
	return context.WithValue(ctx, cookieOpsKey{}, ops), ops
}

// SetCookie queues a cookie to be set on the response.
// The cookie will be applied after the handler returns.
// Panics if WithCookieOps was not called (indicates programmer error).
func SetCookie(ctx context.Context, cookie *http.Cookie) {
	ops, ok := ctx.Value(cookieOpsKey{}).(*[]CookieOp)
	if !ok {
		panic("httpserver: no cookie ops in context - did you forget to use WithCookieOps?")
	}
	*ops = append(*ops, CookieOp{Cookie: cookie})
}

// WithRequestCookies returns a new context with the request cookies attached.
// This is called by the generated wrapHandler function.
func WithRequestCookies(ctx context.Context, cookies []*http.Cookie) context.Context {
	return context.WithValue(ctx, requestCookiesKey{}, cookies)
}

// GetCookie retrieves a cookie from the request by name.
// Returns http.ErrNoCookie if the cookie is not found.
func GetCookie(ctx context.Context, name string) (*http.Cookie, error) {
	cookies, ok := ctx.Value(requestCookiesKey{}).([]*http.Cookie)
	if !ok {
		return nil, http.ErrNoCookie
	}
	for _, c := range cookies {
		if c.Name == name {
			return c, nil
		}
	}
	return nil, http.ErrNoCookie
}
