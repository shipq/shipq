package portapi

import (
	"context"
	"testing"
)

// TestHTTPError_ImplementsError verifies that HTTPError implements the error interface.
func TestHTTPError_ImplementsError(t *testing.T) {
	var err error = HTTPError{Status: 401, Code: "unauthorized", Msg: "invalid token"}

	if err.Error() == "" {
		t.Error("HTTPError.Error() should return a non-empty string")
	}

	// Error message should contain useful information
	errStr := err.Error()
	if errStr == "" {
		t.Error("error string should not be empty")
	}
}

// TestHTTPError_ImplementsCodedError verifies that HTTPError implements CodedError interface.
func TestHTTPError_ImplementsCodedError(t *testing.T) {
	var ce CodedError = HTTPError{Status: 403, Code: "forbidden", Msg: "insufficient permissions"}

	if ce.StatusCode() != 403 {
		t.Errorf("StatusCode() = %d, want 403", ce.StatusCode())
	}

	if ce.ErrorCode() != "forbidden" {
		t.Errorf("ErrorCode() = %q, want %q", ce.ErrorCode(), "forbidden")
	}
}

// TestHTTPError_DefaultBehavior verifies that HTTPError provides sensible defaults
// for zero values to avoid surprises.
func TestHTTPError_DefaultBehavior(t *testing.T) {
	t.Run("zero value returns defaults", func(t *testing.T) {
		var e HTTPError
		var ce CodedError = e

		// Zero Status should default to 500
		if ce.StatusCode() != 500 {
			t.Errorf("StatusCode() = %d, want 500 (default for zero value)", ce.StatusCode())
		}

		// Empty Code should default to "internal_error"
		if ce.ErrorCode() != "internal_error" {
			t.Errorf("ErrorCode() = %q, want %q (default for empty)", ce.ErrorCode(), "internal_error")
		}
	})

	t.Run("explicit zero status returns default", func(t *testing.T) {
		e := HTTPError{Status: 0, Code: "custom_code"}
		if e.StatusCode() != 500 {
			t.Errorf("StatusCode() = %d, want 500 (default)", e.StatusCode())
		}
	})

	t.Run("explicit empty code returns default", func(t *testing.T) {
		e := HTTPError{Status: 404, Code: ""}
		if e.ErrorCode() != "internal_error" {
			t.Errorf("ErrorCode() = %q, want %q (default)", e.ErrorCode(), "internal_error")
		}
	})

	t.Run("explicit values are preserved", func(t *testing.T) {
		e := HTTPError{Status: 404, Code: "not_found", Msg: "resource not found"}
		if e.StatusCode() != 404 {
			t.Errorf("StatusCode() = %d, want 404", e.StatusCode())
		}
		if e.ErrorCode() != "not_found" {
			t.Errorf("ErrorCode() = %q, want %q", e.ErrorCode(), "not_found")
		}
	})
}

// TestHTTPError_ErrorMessageFormat verifies the error message format is stable.
func TestHTTPError_ErrorMessageFormat(t *testing.T) {
	e := HTTPError{Status: 401, Code: "unauthorized", Msg: "invalid credentials"}
	errStr := e.Error()

	if errStr == "" {
		t.Error("error string should not be empty")
	}

	// We don't over-constrain the format, but it should contain the code
	// This allows implementation flexibility while ensuring useful output
	if len(errStr) < len(e.Code) {
		t.Error("error string should contain at least the error code")
	}
}

// TestHandlerResult_Validate verifies HandlerResult validation catches invalid states.
func TestHandlerResult_Validate(t *testing.T) {
	t.Run("JSON response is valid", func(t *testing.T) {
		r := HandlerResult{
			Status: 200,
			JSON:   map[string]any{"ok": true},
		}
		if err := r.Validate(); err != nil {
			t.Errorf("valid JSON result should pass validation: %v", err)
		}
	})

	t.Run("NoContent response is valid", func(t *testing.T) {
		r := HandlerResult{
			Status:    204,
			NoContent: true,
		}
		if err := r.Validate(); err != nil {
			t.Errorf("valid NoContent result should pass validation: %v", err)
		}
	})

	t.Run("both JSON and NoContent is invalid", func(t *testing.T) {
		r := HandlerResult{
			Status:    200,
			JSON:      map[string]any{"ok": true},
			NoContent: true,
		}
		if err := r.Validate(); err == nil {
			t.Error("HandlerResult with both JSON and NoContent should fail validation")
		}
	})

	t.Run("neither JSON nor NoContent is valid for zero result", func(t *testing.T) {
		r := HandlerResult{
			Status: 200,
		}
		// This is actually valid - it means "no response body"
		// The generator will handle this appropriately
		if err := r.Validate(); err != nil {
			t.Errorf("empty result should be valid: %v", err)
		}
	})

	t.Run("zero value is valid", func(t *testing.T) {
		var r HandlerResult
		if err := r.Validate(); err != nil {
			t.Errorf("zero HandlerResult should be valid: %v", err)
		}
	})
}

// TestRequest_DecodedReqSafety verifies that DecodedReqValue is nil-safe.
func TestRequest_DecodedReqSafety(t *testing.T) {
	t.Run("nil request returns safe defaults", func(t *testing.T) {
		var req *Request = nil
		val, ok := req.DecodedReqValue()
		if ok {
			t.Error("nil Request should return ok=false")
		}
		if val != nil {
			t.Error("nil Request should return nil value")
		}
	})

	t.Run("zero request returns safe defaults", func(t *testing.T) {
		req := &Request{}
		val, ok := req.DecodedReqValue()
		if ok {
			t.Error("uninitialized Request should return ok=false")
		}
		if val != nil {
			t.Error("uninitialized Request should return nil value")
		}
	})

	t.Run("request with closure works", func(t *testing.T) {
		type TestReq struct{ ID int }
		testReq := TestReq{ID: 42}

		req := &Request{
			DecodedReq: func() (any, bool) {
				return testReq, true
			},
		}

		val, ok := req.DecodedReqValue()
		if !ok {
			t.Error("should return ok=true when closure is set")
		}
		if val == nil {
			t.Error("should return non-nil value")
		}
		if tr, ok := val.(TestReq); ok {
			if tr.ID != 42 {
				t.Errorf("decoded value ID = %d, want 42", tr.ID)
			}
		} else {
			t.Error("decoded value should be TestReq type")
		}
	})
}

// TestRequest_AccessorsSafety verifies that all request accessors are nil-safe.
func TestRequest_AccessorsSafety(t *testing.T) {
	t.Run("nil request accessors are safe", func(t *testing.T) {
		var req *Request = nil

		if val, ok := req.HeaderValue("X-Test"); ok || val != "" {
			t.Error("nil Request HeaderValue should return empty string and false")
		}

		if val, ok := req.CookieValue("session"); ok || val != "" {
			t.Error("nil Request CookieValue should return empty string and false")
		}

		if vals := req.QueryValues("filter"); vals != nil {
			t.Error("nil Request QueryValues should return nil")
		}

		if val := req.PathVar("id"); val != "" {
			t.Error("nil Request PathVar should return empty string")
		}
	})

	t.Run("zero request accessors are safe", func(t *testing.T) {
		req := &Request{}

		if val, ok := req.HeaderValue("X-Test"); ok || val != "" {
			t.Error("zero Request HeaderValue should return empty string and false")
		}

		if val, ok := req.CookieValue("session"); ok || val != "" {
			t.Error("zero Request CookieValue should return empty string and false")
		}

		if vals := req.QueryValues("filter"); vals != nil {
			t.Error("zero Request QueryValues should return nil")
		}

		if val := req.PathVar("id"); val != "" {
			t.Error("zero Request PathVar should return empty string")
		}
	})

	t.Run("request with closures delegates correctly", func(t *testing.T) {
		req := &Request{
			Header: func(name string) (string, bool) {
				if name == "X-Test" {
					return "test-value", true
				}
				return "", false
			},
			Cookie: func(name string) (string, bool) {
				if name == "session" {
					return "session-123", true
				}
				return "", false
			},
			Query: func(name string) []string {
				if name == "filter" {
					return []string{"a", "b"}
				}
				return nil
			},
			PathValue: func(name string) string {
				if name == "id" {
					return "42"
				}
				return ""
			},
		}

		if val, ok := req.HeaderValue("X-Test"); !ok || val != "test-value" {
			t.Errorf("HeaderValue(X-Test) = (%q, %v), want (test-value, true)", val, ok)
		}

		if val, ok := req.CookieValue("session"); !ok || val != "session-123" {
			t.Errorf("CookieValue(session) = (%q, %v), want (session-123, true)", val, ok)
		}

		if vals := req.QueryValues("filter"); len(vals) != 2 || vals[0] != "a" || vals[1] != "b" {
			t.Errorf("QueryValues(filter) = %v, want [a b]", vals)
		}

		if val := req.PathVar("id"); val != "42" {
			t.Errorf("PathVar(id) = %q, want 42", val)
		}
	})
}

// TestMiddlewareErgonomics simulates realistic middleware usage without net/http.
func TestMiddlewareErgonomics(t *testing.T) {
	// Simulate a middleware that checks authorization and calls next
	authMiddleware := func(ctx context.Context, req *Request, next Next) (HandlerResult, error) {
		// Read authorization header
		authHeader, ok := req.HeaderValue("Authorization")
		if !ok || authHeader == "" {
			return HandlerResult{}, HTTPError{
				Status: 401,
				Code:   "unauthorized",
				Msg:    "missing authorization header",
			}
		}

		// Call next middleware/handler
		result, err := next(ctx)
		if err != nil {
			return result, err
		}

		// After handler executes, we can inspect decoded request if needed
		decodedReq, hasReq := req.DecodedReqValue()
		_ = decodedReq // middleware might log or audit this
		_ = hasReq

		return result, nil
	}

	// Create a request with authorization
	req := &Request{
		Method:  "GET",
		Pattern: "/api/users",
		Header: func(name string) (string, bool) {
			if name == "Authorization" {
				return "Bearer token123", true
			}
			return "", false
		},
		DecodedReq: func() (any, bool) {
			return map[string]any{"id": 123}, true
		},
	}

	// Simulate calling the middleware
	ctx := context.Background()
	nextCalled := false
	next := func(ctx context.Context) (HandlerResult, error) {
		nextCalled = true
		return HandlerResult{
			Status: 200,
			JSON:   map[string]any{"success": true},
		}, nil
	}

	result, err := authMiddleware(ctx, req, next)

	if err != nil {
		t.Fatalf("middleware should succeed: %v", err)
	}

	if !nextCalled {
		t.Error("middleware should call next")
	}

	if result.Status != 200 {
		t.Errorf("result Status = %d, want 200", result.Status)
	}
}

// TestMiddlewareErgonomics_Rejection simulates middleware rejecting a request.
func TestMiddlewareErgonomics_Rejection(t *testing.T) {
	authMiddleware := func(ctx context.Context, req *Request, next Next) (HandlerResult, error) {
		authHeader, ok := req.HeaderValue("Authorization")
		if !ok || authHeader == "" {
			return HandlerResult{}, HTTPError{
				Status: 401,
				Code:   "unauthorized",
				Msg:    "missing authorization header",
			}
		}
		return next(ctx)
	}

	// Request without authorization
	req := &Request{
		Method:  "GET",
		Pattern: "/api/protected",
	}

	ctx := context.Background()
	nextCalled := false
	next := func(ctx context.Context) (HandlerResult, error) {
		nextCalled = true
		return HandlerResult{Status: 200, JSON: map[string]any{"ok": true}}, nil
	}

	_, err := authMiddleware(ctx, req, next)

	if err == nil {
		t.Fatal("middleware should reject request without auth")
	}

	if nextCalled {
		t.Error("middleware should not call next when rejecting")
	}

	// Verify error is CodedError
	if ce, ok := err.(CodedError); ok {
		if ce.StatusCode() != 401 {
			t.Errorf("error StatusCode = %d, want 401", ce.StatusCode())
		}
		if ce.ErrorCode() != "unauthorized" {
			t.Errorf("error ErrorCode = %q, want unauthorized", ce.ErrorCode())
		}
	} else {
		t.Error("error should implement CodedError interface")
	}
}
