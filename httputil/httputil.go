// Package httputil provides shared HTTP helper functions for generated server code.
// This package is embedded into user projects via the shipq embed system, landing
// at {modulePath}/shipq/lib/httputil.
package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/shipq/shipq/httperror"
	"github.com/shipq/shipq/httpserver"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response. If the error is an *httperror.Error,
// the corresponding HTTP status code and message are used. Otherwise, a generic
// 500 Internal Server Error is returned.
func WriteError(w http.ResponseWriter, err error) {
	var httpErr *httperror.Error
	if errors.As(err, &httpErr) {
		WriteJSON(w, httpErr.Code(), map[string]string{"error": httpErr.Message()})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// WrapHandler wraps an HTTP handler with Querier injection, cookie management,
// and custom context setup. The injectCtx function is called to add
// project-specific values (e.g., query runner) to the request context.
func WrapHandler(q httpserver.Querier, injectCtx func(ctx context.Context) context.Context, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := httpserver.WithQuerier(r.Context(), q)
		ctx = injectCtx(ctx)
		ctx = httpserver.WithRequestCookies(ctx, r.Cookies())
		ctx, cookieOps := httpserver.WithCookieOps(ctx)
		r = r.WithContext(ctx)

		cw := httpserver.NewCookieWriter(w, cookieOps)
		h(cw, r)
		cw.Flush()
	})
}

// sessionContextKey is the context key for storing the authenticated account's internal ID.
type sessionContextKey struct{}

// WithSessionAccountID returns a new context with the authenticated account's internal ID.
func WithSessionAccountID(ctx context.Context, accountID int64) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, accountID)
}

// SessionAccountIDFromContext extracts the authenticated account's internal ID from the context.
// Returns (0, false) if no session account ID is present.
func SessionAccountIDFromContext(ctx context.Context) (int64, bool) {
	v := ctx.Value(sessionContextKey{})
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// orgContextKey is the context key for storing the authenticated user's organization ID.
type orgContextKey struct{}

// WithOrganizationID returns a new context with the authenticated user's organization ID.
func WithOrganizationID(ctx context.Context, orgID int64) context.Context {
	return context.WithValue(ctx, orgContextKey{}, orgID)
}

// OrganizationIDFromContext extracts the authenticated user's organization ID from the context.
// Returns (0, false) if no organization ID is present.
func OrganizationIDFromContext(ctx context.Context) (int64, bool) {
	v := ctx.Value(orgContextKey{})
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// WrapAuthHandler is like WrapHandler but also enforces authentication.
// The checkAuth function should verify the current session and return the
// authenticated account's internal ID and organization ID, or an error if
// not authenticated. Both values are injected into the request context so
// that downstream handlers can access them via SessionAccountIDFromContext
// and OrganizationIDFromContext.
// Returns 401 Unauthorized if checkAuth returns an error.
func WrapAuthHandler(q httpserver.Querier, injectCtx func(ctx context.Context) context.Context, checkAuth func(ctx context.Context) (accountID int64, orgID int64, err error), h http.HandlerFunc) http.Handler {
	return WrapHandler(q, injectCtx, func(w http.ResponseWriter, r *http.Request) {
		accountID, orgID, err := checkAuth(r.Context())
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		ctx := WithSessionAccountID(r.Context(), accountID)
		ctx = WithOrganizationID(ctx, orgID)
		r = r.WithContext(ctx)
		h(w, r)
	})
}

// WrapOptionalAuthHandler is like WrapHandler but attempts to parse the session.
// If checkAuth succeeds, the account and org IDs are injected into context.
// If checkAuth returns an error for which isNoSession returns true, the request
// proceeds without authentication (context will have no account ID).
// If checkAuth returns any other error (DB failure, etc.), a 500 is returned.
func WrapOptionalAuthHandler(
	q httpserver.Querier,
	injectCtx func(ctx context.Context) context.Context,
	checkAuth func(ctx context.Context) (accountID int64, orgID int64, err error),
	isNoSession func(err error) bool,
	h http.HandlerFunc,
) http.Handler {
	return WrapHandler(q, injectCtx, func(w http.ResponseWriter, r *http.Request) {
		accountID, orgID, err := checkAuth(r.Context())
		if err != nil {
			if isNoSession(err) {
				// No valid session — proceed unauthenticated
				h(w, r)
				return
			}
			// Real error (DB failure, etc.)
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		ctx := WithSessionAccountID(r.Context(), accountID)
		ctx = WithOrganizationID(ctx, orgID)
		r = r.WithContext(ctx)
		h(w, r)
	})
}

// ForbiddenError represents a 403 Forbidden error. Used by RBAC middleware
// to distinguish "access denied" from other errors.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string { return e.Message }

// Forbidden returns a new ForbiddenError with the given message.
func Forbidden(msg string) error { return &ForbiddenError{Message: msg} }

// WrapRBACHandler is like WrapAuthHandler but also enforces RBAC (role-based
// access control) after authentication. The checkRBAC callback always receives
// orgID from the auth check; in the unscoped case the generated closure ignores it.
//
// Decision flow:
//  1. checkAuth fails -> 401 Unauthorized
//  2. checkRBAC returns ForbiddenError -> 403 Forbidden
//  3. checkRBAC returns other error -> 500 Internal Server Error
//  4. Both pass -> handler is invoked
func WrapRBACHandler(
	q httpserver.Querier,
	injectCtx func(ctx context.Context) context.Context,
	checkAuth func(ctx context.Context) (accountID int64, orgID int64, err error),
	checkRBAC func(ctx context.Context, accountID int64, orgID int64, routePath, method string) error,
	routePath string,
	method string,
	h http.HandlerFunc,
) http.Handler {
	return WrapHandler(q, injectCtx, func(w http.ResponseWriter, r *http.Request) {
		accountID, orgID, err := checkAuth(r.Context())
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		ctx := WithSessionAccountID(r.Context(), accountID)
		ctx = WithOrganizationID(ctx, orgID)
		r = r.WithContext(ctx)

		if rbacErr := checkRBAC(r.Context(), accountID, orgID, routePath, method); rbacErr != nil {
			var forbidden *ForbiddenError
			if errors.As(rbacErr, &forbidden) {
				WriteJSON(w, http.StatusForbidden, map[string]string{"error": forbidden.Message})
				return
			}
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		h(w, r)
	})
}

// AddAuth adds the session cookie to the request if present.
// This is used by generated test clients.
func AddAuth(req *http.Request, sessionCookie string) {
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: sessionCookie,
		})
	}
}
