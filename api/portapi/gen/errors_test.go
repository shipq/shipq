package gen

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindError_Error(t *testing.T) {
	tests := []struct {
		name   string
		err    *BindError
		want   string
		wantIn []string // substrings that must be present
	}{
		{
			name: "with field",
			err: &BindError{
				Source: "query",
				Field:  "limit",
				Err:    errors.New("invalid value"),
			},
			want: "query limit: invalid value",
		},
		{
			name: "without field",
			err: &BindError{
				Source: "body",
				Err:    errors.New("empty request body"),
			},
			want: "body: empty request body",
		},
		{
			name: "path error",
			err: &BindError{
				Source: "path",
				Field:  "id",
				Err:    errMissing,
			},
			wantIn: []string{"path", "id", "missing"},
		},
		{
			name: "header error",
			err: &BindError{
				Source: "header",
				Field:  "Authorization",
				Err:    errMissing,
			},
			wantIn: []string{"header", "Authorization", "missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if tt.want != "" && got != tt.want {
				t.Errorf("BindError.Error() = %q, want %q", got, tt.want)
			}
			for _, s := range tt.wantIn {
				if !strings.Contains(got, s) {
					t.Errorf("BindError.Error() = %q, want to contain %q", got, s)
				}
			}
		})
	}
}

func TestBindError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := &BindError{
		Source: "query",
		Field:  "limit",
		Err:    inner,
	}

	if unwrapped := err.Unwrap(); unwrapped != inner {
		t.Errorf("BindError.Unwrap() = %v, want %v", unwrapped, inner)
	}

	// Test that errors.Is works
	if !errors.Is(err, inner) {
		t.Error("errors.Is should return true for inner error")
	}
}

func TestHTTPError(t *testing.T) {
	err := NewHTTPError(http.StatusNotFound, "not_found", "Resource not found")

	t.Run("Error method returns message", func(t *testing.T) {
		if got := err.Error(); got != "Resource not found" {
			t.Errorf("HTTPError.Error() = %q, want %q", got, "Resource not found")
		}
	})

	t.Run("fields are set correctly", func(t *testing.T) {
		if err.Status != http.StatusNotFound {
			t.Errorf("HTTPError.Status = %d, want %d", err.Status, http.StatusNotFound)
		}
		if err.Code != "not_found" {
			t.Errorf("HTTPError.Code = %q, want %q", err.Code, "not_found")
		}
		if err.Message != "Resource not found" {
			t.Errorf("HTTPError.Message = %q, want %q", err.Message, "Resource not found")
		}
	})
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
		wantBody   string
	}{
		{
			name:       "struct response",
			status:     http.StatusOK,
			data:       struct{ Name string }{"test"},
			wantStatus: 200,
			wantBody:   `{"Name":"test"}`,
		},
		{
			name:       "slice response",
			status:     http.StatusOK,
			data:       []string{"a", "b"},
			wantStatus: 200,
			wantBody:   `["a","b"]`,
		},
		{
			name:       "map response",
			status:     http.StatusOK,
			data:       map[string]int{"count": 42},
			wantStatus: 200,
			wantBody:   `{"count":42}`,
		},
		{
			name:       "custom status code",
			status:     http.StatusCreated,
			data:       struct{ ID string }{"123"},
			wantStatus: 201,
			wantBody:   `{"ID":"123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeJSON(w, tt.status, tt.data)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			// Parse and compare JSON to ignore whitespace differences
			var got, want any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantBody), &want); err != nil {
				t.Fatalf("failed to unmarshal expected: %v", err)
			}

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("body = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestWriteNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	writeNoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	if body := w.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name: "bind error returns 400",
			err: &BindError{
				Source: "query",
				Field:  "limit",
				Err:    errors.New("invalid"),
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:       "http error uses its status and code",
			err:        NewHTTPError(http.StatusNotFound, "not_found", "Resource not found"),
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "http error forbidden",
			err:        NewHTTPError(http.StatusForbidden, "forbidden", "Access denied"),
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "generic error returns 500",
			err:        errors.New("something went wrong"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
		{
			name:       "wrapped bind error still returns 400",
			err:        errors.Join(errors.New("wrapper"), &BindError{Source: "body", Err: errors.New("parse error")}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", resp.Error.Code, tt.wantCode)
			}

			if resp.Error.Message == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestErrorResponse_ValidJSON(t *testing.T) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    "bad_request",
			Message: "invalid input",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal ErrorResponse: %v", err)
	}

	want := `{"error":{"code":"bad_request","message":"invalid input"}}`
	if string(data) != want {
		t.Errorf("ErrorResponse JSON = %s, want %s", data, want)
	}
}

func TestWriteError_MessageContent(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantContain string
	}{
		{
			name: "bind error message contains source and field",
			err: &BindError{
				Source: "query",
				Field:  "limit",
				Err:    errors.New("must be positive"),
			},
			wantContain: "query limit",
		},
		{
			name:        "http error message is preserved",
			err:         NewHTTPError(http.StatusNotFound, "not_found", "User with ID 123 not found"),
			wantContain: "User with ID 123 not found",
		},
		{
			name:        "generic error message is preserved",
			err:         errors.New("database connection failed"),
			wantContain: "database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if !strings.Contains(resp.Error.Message, tt.wantContain) {
				t.Errorf("error message = %q, want to contain %q", resp.Error.Message, tt.wantContain)
			}
		})
	}
}
