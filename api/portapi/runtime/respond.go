package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
)

// RespondJSON writes a JSON response with the given status code.
// If data is a nil slice, it writes an empty JSON array instead of null.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Handle nil slices - render as [] instead of null
	if data != nil {
		rv := reflect.ValueOf(data)
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			_, _ = w.Write([]byte("[]\n"))
			return
		}
	}

	_ = json.NewEncoder(w).Encode(data)
}

// RespondNoContent writes a 204 No Content response with no body.
func RespondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
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

// RespondError writes an error response with appropriate status code.
// BindErrors result in 400 Bad Request, other errors result in 500 Internal Server Error.
func RespondError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"

	var bindErr *BindError
	if errors.As(err, &bindErr) {
		status = http.StatusBadRequest
		code = "bad_request"
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

// RespondHTTPError writes an HTTPError response.
// If the error is an HTTPError, it uses its status and code.
// Otherwise, it falls back to RespondError behavior.
func RespondHTTPError(w http.ResponseWriter, err error) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpErr.Status)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error: ErrorDetail{
				Code:    httpErr.Code,
				Message: httpErr.Message,
			},
		})
		return
	}

	// Fall back to default error handling
	RespondError(w, err)
}
