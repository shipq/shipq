package httperror

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name:     "simple message",
			err:      New(400, "bad request"),
			expected: "bad request",
		},
		{
			name:     "with cause",
			err:      Wrap(500, "failed to process", errors.New("connection refused")),
			expected: "failed to process: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestError_Code(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected int
	}{
		{"400", BadRequest("test"), 400},
		{"401", Unauthorized("test"), 401},
		{"403", Forbidden("test"), 403},
		{"404", NotFound("test"), 404},
		{"409", Conflict("test"), 409},
		{"422", UnprocessableEntity("test"), 422},
		{"500", InternalError("test"), 500},
		{"503", ServiceUnavailable("test"), 503},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Code(); got != tt.expected {
				t.Errorf("Code() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestError_Message(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(500, "top level message", cause)

	if got := err.Message(); got != "top level message" {
		t.Errorf("Message() = %q, want %q", got, "top level message")
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(500, "wrapped", cause)

	if got := err.Unwrap(); got != cause {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}

	// Test nil cause
	err2 := New(400, "no cause")
	if got := err2.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestErrorsIs(t *testing.T) {
	cause := errors.New("specific error")
	err := Wrap(500, "wrapped", cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

func TestNew(t *testing.T) {
	err := New(418, "I'm a teapot")

	if err.Code() != 418 {
		t.Errorf("Code() = %d, want 418", err.Code())
	}
	if err.Message() != "I'm a teapot" {
		t.Errorf("Message() = %q, want %q", err.Message(), "I'm a teapot")
	}
}

func TestNewf(t *testing.T) {
	err := Newf(400, "invalid value: %d", 42)

	if err.Code() != 400 {
		t.Errorf("Code() = %d, want 400", err.Code())
	}
	if err.Message() != "invalid value: 42" {
		t.Errorf("Message() = %q, want %q", err.Message(), "invalid value: 42")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("db connection failed")
	err := Wrap(500, "could not fetch user", cause)

	if err.Code() != 500 {
		t.Errorf("Code() = %d, want 500", err.Code())
	}
	if err.Message() != "could not fetch user" {
		t.Errorf("Message() = %q, want %q", err.Message(), "could not fetch user")
	}
	if err.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
	}
}

func TestWrapf(t *testing.T) {
	cause := errors.New("not found")
	err := Wrapf(404, cause, "user %s not found", "alice")

	if err.Code() != 404 {
		t.Errorf("Code() = %d, want 404", err.Code())
	}
	if err.Message() != "user alice not found" {
		t.Errorf("Message() = %q, want %q", err.Message(), "user alice not found")
	}
	if err.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
	}
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("missing field")
	if err.Code() != 400 {
		t.Errorf("Code() = %d, want 400", err.Code())
	}
	if err.Message() != "missing field" {
		t.Errorf("Message() = %q, want %q", err.Message(), "missing field")
	}
}

func TestBadRequestf(t *testing.T) {
	err := BadRequestf("field %q is required", "email")
	if err.Code() != 400 {
		t.Errorf("Code() = %d, want 400", err.Code())
	}
	expected := `field "email" is required`
	if err.Message() != expected {
		t.Errorf("Message() = %q, want %q", err.Message(), expected)
	}
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("invalid token")
	if err.Code() != 401 {
		t.Errorf("Code() = %d, want 401", err.Code())
	}
}

func TestUnauthorizedf(t *testing.T) {
	err := Unauthorizedf("token expired at %s", "2024-01-01")
	if err.Code() != 401 {
		t.Errorf("Code() = %d, want 401", err.Code())
	}
	if err.Message() != "token expired at 2024-01-01" {
		t.Errorf("Message() = %q, want %q", err.Message(), "token expired at 2024-01-01")
	}
}

func TestForbidden(t *testing.T) {
	err := Forbidden("access denied")
	if err.Code() != 403 {
		t.Errorf("Code() = %d, want 403", err.Code())
	}
}

func TestForbiddenf(t *testing.T) {
	err := Forbiddenf("user %d cannot access resource %d", 1, 2)
	if err.Code() != 403 {
		t.Errorf("Code() = %d, want 403", err.Code())
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("resource not found")
	if err.Code() != 404 {
		t.Errorf("Code() = %d, want 404", err.Code())
	}
}

func TestNotFoundf(t *testing.T) {
	err := NotFoundf("user %q not found", "alice")
	if err.Code() != 404 {
		t.Errorf("Code() = %d, want 404", err.Code())
	}
	expected := `user "alice" not found`
	if err.Message() != expected {
		t.Errorf("Message() = %q, want %q", err.Message(), expected)
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("duplicate entry")
	if err.Code() != 409 {
		t.Errorf("Code() = %d, want 409", err.Code())
	}
}

func TestConflictf(t *testing.T) {
	err := Conflictf("email %q already exists", "test@example.com")
	if err.Code() != 409 {
		t.Errorf("Code() = %d, want 409", err.Code())
	}
}

func TestUnprocessableEntity(t *testing.T) {
	err := UnprocessableEntity("validation failed")
	if err.Code() != 422 {
		t.Errorf("Code() = %d, want 422", err.Code())
	}
}

func TestUnprocessableEntityf(t *testing.T) {
	err := UnprocessableEntityf("field %q must be positive", "amount")
	if err.Code() != 422 {
		t.Errorf("Code() = %d, want 422", err.Code())
	}
}

func TestInternalError(t *testing.T) {
	err := InternalError("unexpected error")
	if err.Code() != 500 {
		t.Errorf("Code() = %d, want 500", err.Code())
	}
}

func TestInternalErrorf(t *testing.T) {
	err := InternalErrorf("failed to process request: %s", "timeout")
	if err.Code() != 500 {
		t.Errorf("Code() = %d, want 500", err.Code())
	}
}

func TestServiceUnavailable(t *testing.T) {
	err := ServiceUnavailable("service temporarily unavailable")
	if err.Code() != 503 {
		t.Errorf("Code() = %d, want 503", err.Code())
	}
}

func TestServiceUnavailablef(t *testing.T) {
	err := ServiceUnavailablef("database %s is unavailable", "primary")
	if err.Code() != 503 {
		t.Errorf("Code() = %d, want 503", err.Code())
	}
}

// Test that Error implements the error interface
func TestErrorImplementsError(t *testing.T) {
	var _ error = (*Error)(nil)
}
