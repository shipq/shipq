package middleware

import (
	"context"
	"sync/atomic"

	"github.com/shipq/shipq/api/portapi"
)

// afterCounter tracks how many times middleware "after" logic ran
var afterCounter int64

// GetAfterCount returns the number of times middleware after logic ran.
func GetAfterCount() int64 {
	return atomic.LoadInt64(&afterCounter)
}

// ResetAfterCount resets the after counter.
func ResetAfterCount() {
	atomic.StoreInt64(&afterCounter, 0)
}

// RegisterMiddleware registers all middleware functions.
func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(RequireAuth)
	reg.Use(TrackAfter)
}

// RequireAuth is a middleware that short-circuits with a 401 error.
func RequireAuth(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	// Check for auth header
	authHeader, ok := req.HeaderValue("Authorization")
	if !ok || authHeader != "Bearer valid-token" {
		return portapi.HandlerResult{}, portapi.HTTPError{
			Status: 401,
			Code:   "unauthorized",
			Msg:    "missing or invalid authorization",
		}
	}
	return next(ctx)
}

// TrackAfter is a middleware that uses defer to track "after" execution.
func TrackAfter(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	defer func() {
		atomic.AddInt64(&afterCounter, 1)
	}()
	return next(ctx)
}
