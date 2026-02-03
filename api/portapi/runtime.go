package portapi

import (
	"context"
	"fmt"
)

// CodedError represents an error with an HTTP status code and error code.
type CodedError interface {
	error
	StatusCode() int
	ErrorCode() string
}

// HTTPError is a convenience struct implementing CodedError.
type HTTPError struct {
	Status int    // HTTP status code (defaults to 500 if zero)
	Code   string // Error code (defaults to "internal_error" if empty)
	Msg    string // Human-readable message
}

// Error implements the error interface.
func (e HTTPError) Error() string {
	code := e.ErrorCode()
	status := e.StatusCode()
	if e.Msg != "" {
		return fmt.Sprintf("[%d %s] %s", status, code, e.Msg)
	}
	return fmt.Sprintf("[%d %s]", status, code)
}

// StatusCode returns the HTTP status code, defaulting to 500 if zero.
func (e HTTPError) StatusCode() int {
	if e.Status == 0 {
		return 500
	}
	return e.Status
}

// ErrorCode returns the error code, defaulting to "internal_error" if empty.
func (e HTTPError) ErrorCode() string {
	if e.Code == "" {
		return "internal_error"
	}
	return e.Code
}

// NotFoundError returns an HTTPError with status 404.
func NotFoundError(message string) error {
	return HTTPError{Status: 404, Code: "not_found", Msg: message}
}

// BadRequestError returns an HTTPError with status 400.
func BadRequestError(message string) error {
	return HTTPError{Status: 400, Code: "bad_request", Msg: message}
}

// UnauthorizedError returns an HTTPError with status 401.
func UnauthorizedError(message string) error {
	return HTTPError{Status: 401, Code: "unauthorized", Msg: message}
}

// ForbiddenError returns an HTTPError with status 403.
func ForbiddenError(message string) error {
	return HTTPError{Status: 403, Code: "forbidden", Msg: message}
}

// InternalError returns an HTTPError with status 500.
func InternalError(message string) error {
	return HTTPError{Status: 500, Code: "internal_error", Msg: message}
}

// HandlerResult represents a direct response from middleware or handlers.
type HandlerResult struct {
	Status    int  // HTTP status code
	JSON      any  // JSON response body (if present)
	NoContent bool // True for 204 No Content responses
}

// Validate checks that HandlerResult is in a valid state.
func (r HandlerResult) Validate() error {
	// Both JSON and NoContent set is invalid
	if r.JSON != nil && r.NoContent {
		return fmt.Errorf("HandlerResult cannot have both JSON and NoContent set")
	}
	// All other states are valid (including zero value)
	return nil
}

// Next is the function signature for calling the next middleware or handler in the chain.
type Next func(ctx context.Context) (HandlerResult, error)

// Middleware is the canonical middleware function signature.
type Middleware func(ctx context.Context, req *Request, next Next) (HandlerResult, error)

// Request is a read-only request view for middleware and handlers.
// It provides access to request metadata and decoded request data without exposing net/http.
type Request struct {
	Method  string // HTTP method (GET, POST, etc.)
	Pattern string // Route pattern (e.g., "/users/{id}")

	// Accessor closures (set by generator)
	Header    func(name string) (string, bool) // Get header value
	Cookie    func(name string) (string, bool) // Get cookie value
	Query     func(name string) []string       // Get query parameter values
	PathValue func(name string) string         // Get path variable value

	// DecodedReq provides access to the decoded request body after binding
	DecodedReq func() (any, bool)
}

// HeaderValue returns a header value in a nil-safe manner.
func (r *Request) HeaderValue(name string) (string, bool) {
	if r == nil || r.Header == nil {
		return "", false
	}
	return r.Header(name)
}

// CookieValue returns a cookie value in a nil-safe manner.
func (r *Request) CookieValue(name string) (string, bool) {
	if r == nil || r.Cookie == nil {
		return "", false
	}
	return r.Cookie(name)
}

// QueryValues returns query parameter values in a nil-safe manner.
func (r *Request) QueryValues(name string) []string {
	if r == nil || r.Query == nil {
		return nil
	}
	return r.Query(name)
}

// PathVar returns a path variable value in a nil-safe manner.
func (r *Request) PathVar(name string) string {
	if r == nil || r.PathValue == nil {
		return ""
	}
	return r.PathValue(name)
}

// DecodedReqValue returns the decoded request body in a nil-safe manner.
func (r *Request) DecodedReqValue() (any, bool) {
	if r == nil || r.DecodedReq == nil {
		return nil, false
	}
	return r.DecodedReq()
}
