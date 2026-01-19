package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shipq/shipq/api/portapi"
)

// User represents an authenticated user.
type User struct {
	Username string
	Email    string
}

// --- Capability-token setup (new design) ------------------------------------
//
// In the redesigned system, middleware implementations do NOT depend on any
// generated zz_* context helpers (e.g. WithCurrentUser).
//
// Instead, middleware declares a typed ContextKey and obtains a typed Cap token
// at registration time. That token is then captured by middleware constructors
// and used for typed With/Get operations.
//
// This file assumes the following stable API exists in portapi (sketch):
//
//   - func NewContextKey[T any](name string) ContextKey[T]
//   - (ContextKey[T]) Provide(reg *MiddlewareRegistry) (Cap[T], *RegistryError)
//   - type Cap[T any]
//   - (Cap[T]) With(ctx context.Context, v T) context.Context
//   - (Cap[T]) Get(ctx context.Context) (T, bool)
//   - (Cap[T]) Must(ctx context.Context) T
//
// The generator can still emit optional convenience helpers, but middleware
// packages never need them to compile.

// Keys defined in the middleware package (typed, stable, non-generated).
var currentUserKey = portapi.NewContextKey[*User]("current_user")

// RegisterMiddleware registers all middleware functions and context keys.
func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	// Declare provided context keys and obtain capabilities.
	//
	// This both:
	//  1) informs discovery/generation via reg.Provide("current_user", TypeOf[*User]())
	//  2) returns a token that enables typed ctx access without generated helpers
	currentUser, err := currentUserKey.Provide(reg)
	if err != nil {
		panic(err)
	}

	// Register middleware in declaration order.
	//
	// Notice: Auth middleware is constructed with the capability token and closes
	// over it, so the implementation can set/get current_user without referring
	// to generated helpers.
	reg.Use(RequestLogger)
	reg.Use(RateLimiter)
	reg.Use(NewAuthOptional(currentUser))
	reg.Use(NewAuthRequired(currentUser))
}

// RequestLogger logs incoming requests.
func RequestLogger(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	start := time.Now()
	result, err := next(ctx)
	duration := time.Since(start)

	// Log request details (in a real app, use a proper logger)
	fmt.Printf("[%s] %s %s - %v\n", time.Now().Format(time.RFC3339), req.Method, req.Pattern, duration)

	return result, err
}

// RateLimiter implements simple in-memory rate limiting.
func RateLimiter(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	// Get client identifier (use Authorization header or IP in real app)
	clientID := "anonymous"
	if authHeader, ok := req.HeaderValue("Authorization"); ok && authHeader != "" {
		clientID = authHeader
	}

	if !rateLimiter.Allow(clientID) {
		return portapi.HandlerResult{}, portapi.HTTPError{
			Status: 429,
			Code:   "rate_limit_exceeded",
			Msg:    "too many requests, please try again later",
		}
	}

	return next(ctx)
}

// Simple in-memory rate limiter for demo purposes.
type simpleRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

var rateLimiter = &simpleRateLimiter{
	requests: make(map[string][]time.Time),
	limit:    100,
	window:   time.Minute,
}

func (r *simpleRateLimiter) Allow(clientID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// Get existing requests and filter out old ones
	times := r.requests[clientID]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Check if under limit
	if len(valid) >= r.limit {
		r.requests[clientID] = valid
		return false
	}

	// Add current request
	valid = append(valid, now)
	r.requests[clientID] = valid
	return true
}

// NewAuthOptional extracts user from Authorization header if present, but doesn't require it.
func NewAuthOptional(currentUser portapi.Cap[*User]) any {
	return func(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
		authHeader, ok := req.HeaderValue("Authorization")
		if ok && authHeader != "" {
			user := parseAuthHeader(authHeader)
			if user != nil {
				ctx = currentUser.With(ctx, user)
			}
		}
		return next(ctx)
	}
}

// NewAuthRequired requires a valid Authorization header and extracts the user.
func NewAuthRequired(currentUser portapi.Cap[*User]) any {
	return func(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
		authHeader, ok := req.HeaderValue("Authorization")
		if !ok || authHeader == "" {
			return portapi.HandlerResult{}, portapi.HTTPError{
				Status: 401,
				Code:   "unauthorized",
				Msg:    "missing authorization header",
			}
		}

		user := parseAuthHeader(authHeader)
		if user == nil {
			return portapi.HandlerResult{}, portapi.HTTPError{
				Status: 401,
				Code:   "unauthorized",
				Msg:    "invalid authorization header",
			}
		}

		ctx = currentUser.With(ctx, user)
		return next(ctx)
	}
}

// parseAuthHeader extracts user information from the Authorization header.
// For demo purposes, we use a simple "Bearer username:email" format.
func parseAuthHeader(header string) *User {
	if !strings.HasPrefix(header, "Bearer ") {
		return nil
	}

	token := strings.TrimPrefix(header, "Bearer ")
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	return &User{
		Username: parts[0],
		Email:    parts[1],
	}
}
