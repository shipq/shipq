package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// BindError represents an error that occurred during request binding.
type BindError struct {
	Source string // "path", "query", "header", "body"
	Field  string
	Err    error
}

func (e *BindError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s %s: %s", e.Source, e.Field, e.Err.Error())
	}
	return fmt.Sprintf("%s: %s", e.Source, e.Err.Error())
}

func (e *BindError) Unwrap() error {
	return e.Err
}

// Common binding errors
var (
	errMissing = errors.New("missing required value")
)

// HTTPError is an error that carries an HTTP status code.
type HTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates a new HTTPError with the given status, code, and message.
func NewHTTPError(status int, code, message string) *HTTPError {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

// ErrorResponse is the standard error response structure.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the error code and message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeNoContent writes a 204 No Content response with no body.
func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// writeError writes an error response with appropriate status code.
// BindErrors result in 400 Bad Request, HTTPErrors use their status,
// other errors result in 500 Internal Server Error.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"

	var bindErr *BindError
	var httpErr *HTTPError

	if errors.As(err, &bindErr) {
		status = http.StatusBadRequest
		code = "bad_request"
	} else if errors.As(err, &httpErr) {
		status = httpErr.Status
		code = httpErr.Code
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: err.Error(),
		},
	})
}
