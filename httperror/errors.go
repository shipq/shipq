// Package httperror provides HTTP error types and constructors for use in handlers.
package httperror

import "fmt"

// Error implements the error interface with HTTP status code support.
type Error struct {
	code    int
	message string
	cause   error
}

// Error returns the error message.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Code returns the HTTP status code.
func (e *Error) Code() int { return e.code }

// Message returns the error message without the cause.
func (e *Error) Message() string { return e.message }

// Unwrap returns the underlying cause for errors.As/errors.Is support.
func (e *Error) Unwrap() error { return e.cause }

// New creates a new HTTP error with the given code and message.
func New(code int, message string) *Error {
	return &Error{code: code, message: message}
}

// Newf creates a new HTTP error with the given code and formatted message.
func Newf(code int, format string, args ...any) *Error {
	return &Error{code: code, message: fmt.Sprintf(format, args...)}
}

// Wrap wraps an underlying error with an HTTP error.
func Wrap(code int, message string, cause error) *Error {
	return &Error{code: code, message: message, cause: cause}
}

// Wrapf wraps an underlying error with an HTTP error and formatted message.
func Wrapf(code int, cause error, format string, args ...any) *Error {
	return &Error{code: code, message: fmt.Sprintf(format, args...), cause: cause}
}

// 400 Bad Request

// BadRequest creates a 400 Bad Request error.
func BadRequest(message string) *Error {
	return &Error{code: 400, message: message}
}

// BadRequestf creates a 400 Bad Request error with a formatted message.
func BadRequestf(format string, args ...any) *Error {
	return &Error{code: 400, message: fmt.Sprintf(format, args...)}
}

// 401 Unauthorized

// Unauthorized creates a 401 Unauthorized error.
func Unauthorized(message string) *Error {
	return &Error{code: 401, message: message}
}

// Unauthorizedf creates a 401 Unauthorized error with a formatted message.
func Unauthorizedf(format string, args ...any) *Error {
	return &Error{code: 401, message: fmt.Sprintf(format, args...)}
}

// 403 Forbidden

// Forbidden creates a 403 Forbidden error.
func Forbidden(message string) *Error {
	return &Error{code: 403, message: message}
}

// Forbiddenf creates a 403 Forbidden error with a formatted message.
func Forbiddenf(format string, args ...any) *Error {
	return &Error{code: 403, message: fmt.Sprintf(format, args...)}
}

// 404 Not Found

// NotFound creates a 404 Not Found error.
func NotFound(message string) *Error {
	return &Error{code: 404, message: message}
}

// NotFoundf creates a 404 Not Found error with a formatted message.
func NotFoundf(format string, args ...any) *Error {
	return &Error{code: 404, message: fmt.Sprintf(format, args...)}
}

// 409 Conflict

// Conflict creates a 409 Conflict error.
func Conflict(message string) *Error {
	return &Error{code: 409, message: message}
}

// Conflictf creates a 409 Conflict error with a formatted message.
func Conflictf(format string, args ...any) *Error {
	return &Error{code: 409, message: fmt.Sprintf(format, args...)}
}

// 422 Unprocessable Entity

// UnprocessableEntity creates a 422 Unprocessable Entity error.
func UnprocessableEntity(message string) *Error {
	return &Error{code: 422, message: message}
}

// UnprocessableEntityf creates a 422 Unprocessable Entity error with a formatted message.
func UnprocessableEntityf(format string, args ...any) *Error {
	return &Error{code: 422, message: fmt.Sprintf(format, args...)}
}

// 500 Internal Server Error

// InternalError creates a 500 Internal Server Error.
func InternalError(message string) *Error {
	return &Error{code: 500, message: message}
}

// InternalErrorf creates a 500 Internal Server Error with a formatted message.
func InternalErrorf(format string, args ...any) *Error {
	return &Error{code: 500, message: fmt.Sprintf(format, args...)}
}

// 503 Service Unavailable

// ServiceUnavailable creates a 503 Service Unavailable error.
func ServiceUnavailable(message string) *Error {
	return &Error{code: 503, message: message}
}

// ServiceUnavailablef creates a 503 Service Unavailable error with a formatted message.
func ServiceUnavailablef(format string, args ...any) *Error {
	return &Error{code: 503, message: fmt.Sprintf(format, args...)}
}
