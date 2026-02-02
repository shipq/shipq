// Package portapi provides runtime support for generated test clients.
//
// This file contains helpers used by generated test client code to:
// - Build HTTP requests (path interpolation, query/header/cookie helpers)
// - Decode error responses into portapi.HTTPError (CodedError)
// - Encode/decode JSON bodies
// - Report client-side binding errors before sending HTTP requests
package portapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// Client-side Binding Errors
// =============================================================================

// ClientBindError represents an error that occurred during client-side request
// binding, before the HTTP request is sent. This allows tests to detect
// programmer errors (missing required fields, invalid values) without making
// a network call.
type ClientBindError struct {
	Source string // One of: "path", "query", "header", "cookie", "body"
	Field  string // The field name or binding name
	Err    error  // The underlying error
}

// Error implements the error interface.
func (e *ClientBindError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("bind %s %s: %v", e.Source, e.Field, e.Err)
	}
	return fmt.Sprintf("bind %s: %v", e.Source, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is/As.
func (e *ClientBindError) Unwrap() error {
	return e.Err
}

// errMissingRequired is the sentinel error for missing required values.
var errMissingRequired = fmt.Errorf("missing required value")

// errInvalidValue is the sentinel error for invalid value conversions.
var errInvalidValue = fmt.Errorf("invalid value")

// NewMissingRequired creates a ClientBindError for a missing required field.
func NewMissingRequired(source, field string) error {
	return &ClientBindError{
		Source: source,
		Field:  field,
		Err:    errMissingRequired,
	}
}

// NewInvalidValue creates a ClientBindError for an invalid field value.
func NewInvalidValue(source, field string, err error) error {
	return &ClientBindError{
		Source: source,
		Field:  field,
		Err:    fmt.Errorf("%w: %v", errInvalidValue, err),
	}
}

// =============================================================================
// Error Envelope Decoder
// =============================================================================

// errorEnvelope represents the standard JSON error response structure.
// Expected shape: { "error": { "code": "...", "message": "..." } }
type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// maxErrorBodySnippet is the maximum number of bytes to include in error
// messages when the body cannot be parsed.
const maxErrorBodySnippet = 256

// DecodeErrorEnvelope decodes a non-2xx HTTP response body into an HTTPError.
//
// It attempts to decode the standard error envelope format:
//
//	{ "error": { "code": "...", "message": "..." } }
//
// If decoding fails, it falls back to a generic error with a body snippet.
// The returned HTTPError always satisfies CodedError.
func DecodeErrorEnvelope(status int, header http.Header, body []byte) HTTPError {
	// Attempt to decode the standard error envelope
	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil {
		code := envelope.Error.Code
		msg := envelope.Error.Message

		// Fallback for empty code
		if code == "" {
			code = "unknown_error"
		}

		// Fallback for empty message
		if msg == "" {
			msg = deriveErrorMessage(status, body)
		}

		return HTTPError{
			Status: status,
			Code:   code,
			Msg:    msg,
		}
	}

	// JSON decode failed - construct fallback error
	return HTTPError{
		Status: status,
		Code:   "unknown_error",
		Msg:    deriveErrorMessage(status, body),
	}
}

// deriveErrorMessage creates a fallback error message from the response body.
func deriveErrorMessage(status int, body []byte) string {
	if len(body) == 0 {
		return "empty error response"
	}

	// Truncate body for the error message
	snippet := body
	if len(snippet) > maxErrorBodySnippet {
		snippet = snippet[:maxErrorBodySnippet]
	}

	// Check if it looks like it might be JSON that we failed to parse
	trimmed := bytes.TrimSpace(snippet)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return fmt.Sprintf("unparseable error response: %s", string(snippet))
	}

	// Probably not JSON - include as-is
	return fmt.Sprintf("error response: %s", string(snippet))
}

// =============================================================================
// Request Building Helpers
// =============================================================================

// AddQuery adds one or more values for a query parameter.
// Empty names cause a panic (programmer error in generator).
// Empty values are added as-is (the server may interpret them as present but empty).
func AddQuery(dst url.Values, name string, values ...string) {
	if name == "" {
		panic("AddQuery: name cannot be empty")
	}
	for _, v := range values {
		dst.Add(name, v)
	}
}

// SetHeader sets a header value on the request.
// Empty names cause a panic (programmer error in generator).
func SetHeader(req *http.Request, name, value string) {
	if name == "" {
		panic("SetHeader: name cannot be empty")
	}
	req.Header.Set(name, value)
}

// AddCookie adds a cookie to the request.
// Empty names cause a panic (programmer error in generator).
func AddCookie(req *http.Request, name, value string) {
	if name == "" {
		panic("AddCookie: name cannot be empty")
	}
	req.AddCookie(&http.Cookie{Name: name, Value: value})
}

// =============================================================================
// Stringification Helpers (for binding values to strings)
// =============================================================================

// FormatBool formats a boolean for query/header values.
// Output: "true" or "false" (lowercase).
func FormatBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// FormatInt formats an int for query/header values.
func FormatInt(v int) string {
	return strconv.Itoa(v)
}

// FormatInt64 formats an int64 for query/header values.
func FormatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

// FormatUint formats a uint for query/header values.
func FormatUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

// FormatUint64 formats a uint64 for query/header values.
func FormatUint64(v uint64) string {
	return strconv.FormatUint(v, 10)
}

// FormatFloat32 formats a float32 for query/header values.
func FormatFloat32(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

// FormatFloat64 formats a float64 for query/header values.
func FormatFloat64(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// FormatTime formats a time.Time for query/header values using RFC3339.
func FormatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// =============================================================================
// JSON Helpers
// =============================================================================

// EncodeJSON encodes a value as JSON and returns an io.Reader for the body.
// The Content-Type should be set to "application/json" by the caller.
func EncodeJSON(v any) (io.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	return bytes.NewReader(data), nil
}

// DecodeJSON decodes JSON data into a value of type T.
// This is the generic variant for Go 1.18+.
func DecodeJSON[T any](data []byte) (T, error) {
	var result T
	if len(data) == 0 {
		return result, fmt.Errorf("decode json: empty body")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("decode json: %w", err)
	}
	return result, nil
}

// DecodeJSONInto decodes JSON data into an existing pointer.
// This is the non-generic variant for cases where generics aren't suitable.
func DecodeJSONInto(data []byte, out any) error {
	if len(data) == 0 {
		return fmt.Errorf("decode json: empty body")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

// =============================================================================
// Path Interpolation
// =============================================================================

// InterpolatePath replaces {name} placeholders in the pattern with values from vars.
//
// Each value is URL-escaped using url.PathEscape.
// If a required variable is missing or empty in vars, a ClientBindError is returned.
//
// Wildcard patterns like {path...} are NOT supported in v1 and will return an error.
func InterpolatePath(pattern string, vars map[string]string) (string, error) {
	// Check for unsupported wildcard patterns
	if strings.Contains(pattern, "...}") {
		return "", &ClientBindError{
			Source: "path",
			Field:  "pattern",
			Err:    fmt.Errorf("wildcard path variables ({name...}) are not supported"),
		}
	}

	result := pattern
	start := 0

	for {
		// Find the next {
		openIdx := strings.Index(result[start:], "{")
		if openIdx == -1 {
			break
		}
		openIdx += start

		// Find the matching }
		closeIdx := strings.Index(result[openIdx:], "}")
		if closeIdx == -1 {
			// Malformed pattern - no closing brace
			break
		}
		closeIdx += openIdx

		// Extract the variable name
		varName := result[openIdx+1 : closeIdx]
		if varName == "" {
			return "", &ClientBindError{
				Source: "path",
				Field:  "pattern",
				Err:    fmt.Errorf("empty path variable name in pattern"),
			}
		}

		// Look up the value
		value, ok := vars[varName]
		if !ok {
			return "", NewMissingRequired("path", varName)
		}
		if value == "" {
			return "", NewMissingRequired("path", varName)
		}

		// Escape and substitute
		escaped := url.PathEscape(value)
		result = result[:openIdx] + escaped + result[closeIdx+1:]

		// Move past the substitution
		start = openIdx + len(escaped)
	}

	return result, nil
}

// =============================================================================
// Test Client Base
// =============================================================================

// TestClient is the base struct for generated test clients.
// Generated code will embed this or use it directly.
type TestClient struct {
	// BaseURL is the base URL for API requests (e.g., "http://localhost:8080").
	// It should not have a trailing slash.
	BaseURL string

	// HTTP is the HTTP client to use for requests.
	// If nil, http.DefaultClient is used.
	HTTP *http.Client
}

// httpClient returns the HTTP client to use, defaulting to http.DefaultClient.
func (c *TestClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// Do executes an HTTP request and handles the response.
//
// For 2xx responses, it decodes the body into `out` (if out is not nil).
// For non-2xx responses, it returns an HTTPError (satisfying CodedError).
// For transport/decode errors, it returns a non-CodedError error.
//
// Parameters:
//   - method: HTTP method (GET, POST, etc.)
//   - path: URL path (already interpolated)
//   - query: query parameters (can be nil)
//   - headers: headers to set (can be nil)
//   - body: request body (can be nil for no body)
//   - out: pointer to decode response into (can be nil for no-content responses)
func (c *TestClient) Do(
	req *http.Request,
	out any,
) error {
	// Execute request
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Read body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Check for non-2xx status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DecodeErrorEnvelope(resp.StatusCode, resp.Header, respBody)
	}

	// Decode success response if out is provided
	if out != nil && len(respBody) > 0 {
		if err := DecodeJSONInto(respBody, out); err != nil {
			return err
		}
	}

	return nil
}

// BuildRequest creates an HTTP request with the given parameters.
//
// Parameters:
//   - method: HTTP method (GET, POST, etc.)
//   - path: URL path (already interpolated, without base URL)
//   - query: query parameters (can be nil)
//   - body: request body reader (can be nil)
func (c *TestClient) BuildRequest(
	method string,
	path string,
	query url.Values,
	body io.Reader,
) (*http.Request, error) {
	// Build full URL
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	// Create request
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set content type for JSON bodies
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}
