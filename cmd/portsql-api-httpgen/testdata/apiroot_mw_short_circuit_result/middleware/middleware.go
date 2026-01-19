package middleware

import (
	"context"

	"github.com/shipq/shipq/api/portapi"
)

// RegisterMiddleware registers all middleware functions.
func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(CacheCheck)
}

// CacheCheck is a middleware that short-circuits with a HandlerResult.
// It returns a 204 No Content response without calling the handler.
func CacheCheck(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	// Check for cache-control header requesting no-cache
	cacheControl, ok := req.HeaderValue("Cache-Control")
	if ok && cacheControl == "no-cache" {
		// Skip cache and call handler
		return next(ctx)
	}

	// Return cached response (simulate cache hit)
	// For this test, we'll return 204 No Content to make assertion simple
	return portapi.HandlerResult{
		Status:    204,
		NoContent: true,
	}, nil
}
