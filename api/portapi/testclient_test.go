package portapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// =============================================================================
// ClientBindError Tests
// =============================================================================

func TestClientBindError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ClientBindError
		expected string
	}{
		{
			name: "with field",
			err: &ClientBindError{
				Source: "path",
				Field:  "id",
				Err:    errMissingRequired,
			},
			expected: "bind path id: missing required value",
		},
		{
			name: "without field",
			err: &ClientBindError{
				Source: "body",
				Field:  "",
				Err:    errors.New("invalid json"),
			},
			expected: "bind body: invalid json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientBindError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &ClientBindError{
		Source: "query",
		Field:  "limit",
		Err:    underlying,
	}

	if !errors.Is(err, underlying) {
		t.Error("Unwrap() should allow errors.Is to find underlying error")
	}
}

func TestNewMissingRequired(t *testing.T) {
	err := NewMissingRequired("path", "id")

	var bindErr *ClientBindError
	if !errors.As(err, &bindErr) {
		t.Fatal("expected ClientBindError")
	}

	if bindErr.Source != "path" {
		t.Errorf("Source = %q, want %q", bindErr.Source, "path")
	}
	if bindErr.Field != "id" {
		t.Errorf("Field = %q, want %q", bindErr.Field, "id")
	}
	if !errors.Is(bindErr.Err, errMissingRequired) {
		t.Error("expected errMissingRequired as underlying error")
	}
}

func TestNewInvalidValue(t *testing.T) {
	underlying := errors.New("not a number")
	err := NewInvalidValue("query", "limit", underlying)

	var bindErr *ClientBindError
	if !errors.As(err, &bindErr) {
		t.Fatal("expected ClientBindError")
	}

	if bindErr.Source != "query" {
		t.Errorf("Source = %q, want %q", bindErr.Source, "query")
	}
	if bindErr.Field != "limit" {
		t.Errorf("Field = %q, want %q", bindErr.Field, "limit")
	}
	if !errors.Is(bindErr.Err, errInvalidValue) {
		t.Error("expected errInvalidValue as underlying error")
	}
}

// =============================================================================
// DecodeErrorEnvelope Tests
// =============================================================================

func TestDecodeErrorEnvelope_ValidJSON(t *testing.T) {
	body := []byte(`{"error":{"code":"not_found","message":"Pet not found"}}`)

	err := DecodeErrorEnvelope(404, nil, body)

	if err.StatusCode() != 404 {
		t.Errorf("StatusCode() = %d, want 404", err.StatusCode())
	}
	if err.ErrorCode() != "not_found" {
		t.Errorf("ErrorCode() = %q, want %q", err.ErrorCode(), "not_found")
	}
	if err.Msg != "Pet not found" {
		t.Errorf("Msg = %q, want %q", err.Msg, "Pet not found")
	}
}

func TestDecodeErrorEnvelope_EmptyCode(t *testing.T) {
	body := []byte(`{"error":{"code":"","message":"Something went wrong"}}`)

	err := DecodeErrorEnvelope(500, nil, body)

	if err.ErrorCode() != "unknown_error" {
		t.Errorf("ErrorCode() = %q, want %q", err.ErrorCode(), "unknown_error")
	}
	if err.Msg != "Something went wrong" {
		t.Errorf("Msg = %q, want %q", err.Msg, "Something went wrong")
	}
}

func TestDecodeErrorEnvelope_EmptyMessage(t *testing.T) {
	body := []byte(`{"error":{"code":"bad_request","message":""}}`)

	err := DecodeErrorEnvelope(400, nil, body)

	if err.ErrorCode() != "bad_request" {
		t.Errorf("ErrorCode() = %q, want %q", err.ErrorCode(), "bad_request")
	}
	// Should fall back to derived message
	if err.Msg == "" {
		t.Error("Msg should not be empty")
	}
}

func TestDecodeErrorEnvelope_InvalidJSON(t *testing.T) {
	body := []byte(`not json at all`)

	err := DecodeErrorEnvelope(500, nil, body)

	if err.StatusCode() != 500 {
		t.Errorf("StatusCode() = %d, want 500", err.StatusCode())
	}
	if err.ErrorCode() != "unknown_error" {
		t.Errorf("ErrorCode() = %q, want %q", err.ErrorCode(), "unknown_error")
	}
	if err.Msg == "" {
		t.Error("Msg should contain body snippet")
	}
}

func TestDecodeErrorEnvelope_EmptyBody(t *testing.T) {
	err := DecodeErrorEnvelope(500, nil, []byte{})

	if err.StatusCode() != 500 {
		t.Errorf("StatusCode() = %d, want 500", err.StatusCode())
	}
	if err.ErrorCode() != "unknown_error" {
		t.Errorf("ErrorCode() = %q, want %q", err.ErrorCode(), "unknown_error")
	}
	if err.Msg != "empty error response" {
		t.Errorf("Msg = %q, want %q", err.Msg, "empty error response")
	}
}

func TestDecodeErrorEnvelope_MalformedJSON(t *testing.T) {
	body := []byte(`{"error": {"code": "bad"`)

	err := DecodeErrorEnvelope(400, nil, body)

	if err.ErrorCode() != "unknown_error" {
		t.Errorf("ErrorCode() = %q, want %q", err.ErrorCode(), "unknown_error")
	}
	// Should mention unparseable
	if err.Msg == "" {
		t.Error("Msg should contain error info")
	}
}

func TestDecodeErrorEnvelope_SatisfiesCodedError(t *testing.T) {
	body := []byte(`{"error":{"code":"test_error","message":"test"}}`)
	httpErr := DecodeErrorEnvelope(400, nil, body)

	// Verify it satisfies CodedError interface
	var ce CodedError = httpErr
	if ce.StatusCode() != 400 {
		t.Errorf("StatusCode() = %d, want 400", ce.StatusCode())
	}
	if ce.ErrorCode() != "test_error" {
		t.Errorf("ErrorCode() = %q, want %q", ce.ErrorCode(), "test_error")
	}
}

// =============================================================================
// Request Building Helper Tests
// =============================================================================

func TestAddQuery(t *testing.T) {
	t.Run("single value", func(t *testing.T) {
		v := make(url.Values)
		AddQuery(v, "limit", "10")

		if got := v.Get("limit"); got != "10" {
			t.Errorf("Get(limit) = %q, want %q", got, "10")
		}
	})

	t.Run("multiple values", func(t *testing.T) {
		v := make(url.Values)
		AddQuery(v, "tag", "a", "b", "c")

		got := v["tag"]
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Errorf("tag values = %v, want [a b c]", got)
		}
	})

	t.Run("empty name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty name")
			}
		}()
		v := make(url.Values)
		AddQuery(v, "", "value")
	})
}

func TestSetHeader(t *testing.T) {
	t.Run("sets header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		SetHeader(req, "Authorization", "Bearer token")

		if got := req.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Header = %q, want %q", got, "Bearer token")
		}
	})

	t.Run("empty name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty name")
			}
		}()
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		SetHeader(req, "", "value")
	})
}

func TestAddCookie(t *testing.T) {
	t.Run("adds cookie", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		AddCookie(req, "session", "abc123")

		cookie, err := req.Cookie("session")
		if err != nil {
			t.Fatalf("Cookie not found: %v", err)
		}
		if cookie.Value != "abc123" {
			t.Errorf("Cookie value = %q, want %q", cookie.Value, "abc123")
		}
	})

	t.Run("empty name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty name")
			}
		}()
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		AddCookie(req, "", "value")
	})
}

// =============================================================================
// Stringification Helper Tests
// =============================================================================

func TestFormatBool(t *testing.T) {
	if got := FormatBool(true); got != "true" {
		t.Errorf("FormatBool(true) = %q, want %q", got, "true")
	}
	if got := FormatBool(false); got != "false" {
		t.Errorf("FormatBool(false) = %q, want %q", got, "false")
	}
}

func TestFormatInt(t *testing.T) {
	if got := FormatInt(42); got != "42" {
		t.Errorf("FormatInt(42) = %q, want %q", got, "42")
	}
	if got := FormatInt(-123); got != "-123" {
		t.Errorf("FormatInt(-123) = %q, want %q", got, "-123")
	}
}

func TestFormatInt64(t *testing.T) {
	if got := FormatInt64(9223372036854775807); got != "9223372036854775807" {
		t.Errorf("FormatInt64(max) = %q, want %q", got, "9223372036854775807")
	}
}

func TestFormatUint(t *testing.T) {
	if got := FormatUint(42); got != "42" {
		t.Errorf("FormatUint(42) = %q, want %q", got, "42")
	}
}

func TestFormatUint64(t *testing.T) {
	if got := FormatUint64(18446744073709551615); got != "18446744073709551615" {
		t.Errorf("FormatUint64(max) = %q, want %q", got, "18446744073709551615")
	}
}

func TestFormatFloat32(t *testing.T) {
	got := FormatFloat32(3.14)
	// Float32 precision means we might not get exactly "3.14"
	if got != "3.14" && got != "3.1400001" {
		t.Errorf("FormatFloat32(3.14) = %q", got)
	}
}

func TestFormatFloat64(t *testing.T) {
	if got := FormatFloat64(3.14159265359); got != "3.14159265359" {
		t.Errorf("FormatFloat64(pi) = %q, want %q", got, "3.14159265359")
	}
}

func TestFormatTime(t *testing.T) {
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expected := "2024-01-15T10:30:00Z"

	if got := FormatTime(tm); got != expected {
		t.Errorf("FormatTime() = %q, want %q", got, expected)
	}
}

// =============================================================================
// JSON Helper Tests
// =============================================================================

func TestEncodeJSON(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	r, err := EncodeJSON(TestStruct{Name: "Alice", Age: 30})
	if err != nil {
		t.Fatalf("EncodeJSON error: %v", err)
	}

	data, _ := io.ReadAll(r)
	expected := `{"name":"Alice","age":30}`
	if string(data) != expected {
		t.Errorf("EncodeJSON = %q, want %q", string(data), expected)
	}
}

func TestDecodeJSON(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("valid json", func(t *testing.T) {
		data := []byte(`{"name":"Bob","age":25}`)
		result, err := DecodeJSON[TestStruct](data)
		if err != nil {
			t.Fatalf("DecodeJSON error: %v", err)
		}
		if result.Name != "Bob" || result.Age != 25 {
			t.Errorf("DecodeJSON = %+v, want {Name:Bob Age:25}", result)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		_, err := DecodeJSON[TestStruct]([]byte{})
		if err == nil {
			t.Error("expected error for empty body")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := DecodeJSON[TestStruct]([]byte(`not json`))
		if err == nil {
			t.Error("expected error for invalid json")
		}
	})
}

func TestDecodeJSONInto(t *testing.T) {
	type TestStruct struct {
		Value int `json:"value"`
	}

	t.Run("valid json", func(t *testing.T) {
		var out TestStruct
		err := DecodeJSONInto([]byte(`{"value":42}`), &out)
		if err != nil {
			t.Fatalf("DecodeJSONInto error: %v", err)
		}
		if out.Value != 42 {
			t.Errorf("Value = %d, want 42", out.Value)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		var out TestStruct
		err := DecodeJSONInto([]byte{}, &out)
		if err == nil {
			t.Error("expected error for empty body")
		}
	})
}

// =============================================================================
// Path Interpolation Tests
// =============================================================================

func TestInterpolatePath(t *testing.T) {
	t.Run("simple substitution", func(t *testing.T) {
		result, err := InterpolatePath("/pets/{id}", map[string]string{"id": "123"})
		if err != nil {
			t.Fatalf("InterpolatePath error: %v", err)
		}
		if result != "/pets/123" {
			t.Errorf("result = %q, want %q", result, "/pets/123")
		}
	})

	t.Run("multiple variables", func(t *testing.T) {
		result, err := InterpolatePath("/users/{user_id}/posts/{post_id}", map[string]string{
			"user_id": "alice",
			"post_id": "42",
		})
		if err != nil {
			t.Fatalf("InterpolatePath error: %v", err)
		}
		if result != "/users/alice/posts/42" {
			t.Errorf("result = %q, want %q", result, "/users/alice/posts/42")
		}
	})

	t.Run("url escaping", func(t *testing.T) {
		result, err := InterpolatePath("/files/{name}", map[string]string{"name": "my file.txt"})
		if err != nil {
			t.Fatalf("InterpolatePath error: %v", err)
		}
		if result != "/files/my%20file.txt" {
			t.Errorf("result = %q, want %q", result, "/files/my%20file.txt")
		}
	})

	t.Run("special characters escaped", func(t *testing.T) {
		result, err := InterpolatePath("/items/{id}", map[string]string{"id": "a/b"})
		if err != nil {
			t.Fatalf("InterpolatePath error: %v", err)
		}
		// url.PathEscape escapes / as %2F
		if result != "/items/a%2Fb" {
			t.Errorf("result = %q, want %q", result, "/items/a%2Fb")
		}
	})

	t.Run("missing variable", func(t *testing.T) {
		_, err := InterpolatePath("/pets/{id}", map[string]string{})
		if err == nil {
			t.Fatal("expected error for missing variable")
		}

		var bindErr *ClientBindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("expected ClientBindError, got %T", err)
		}
		if bindErr.Source != "path" {
			t.Errorf("Source = %q, want %q", bindErr.Source, "path")
		}
		if bindErr.Field != "id" {
			t.Errorf("Field = %q, want %q", bindErr.Field, "id")
		}
	})

	t.Run("empty variable value", func(t *testing.T) {
		_, err := InterpolatePath("/pets/{id}", map[string]string{"id": ""})
		if err == nil {
			t.Fatal("expected error for empty variable")
		}

		var bindErr *ClientBindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("expected ClientBindError, got %T", err)
		}
	})

	t.Run("wildcard not supported", func(t *testing.T) {
		_, err := InterpolatePath("/files/{path...}", map[string]string{"path": "a/b/c"})
		if err == nil {
			t.Fatal("expected error for wildcard pattern")
		}

		var bindErr *ClientBindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("expected ClientBindError, got %T", err)
		}
	})

	t.Run("no variables", func(t *testing.T) {
		result, err := InterpolatePath("/health", map[string]string{})
		if err != nil {
			t.Fatalf("InterpolatePath error: %v", err)
		}
		if result != "/health" {
			t.Errorf("result = %q, want %q", result, "/health")
		}
	})
}

// =============================================================================
// TestClient Tests
// =============================================================================

func TestTestClient_httpClient(t *testing.T) {
	t.Run("returns custom client", func(t *testing.T) {
		custom := &http.Client{Timeout: 5 * time.Second}
		c := &TestClient{HTTP: custom}

		if c.httpClient() != custom {
			t.Error("expected custom client to be returned")
		}
	})

	t.Run("returns default client when nil", func(t *testing.T) {
		c := &TestClient{}

		if c.httpClient() != http.DefaultClient {
			t.Error("expected http.DefaultClient to be returned")
		}
	})
}

func TestTestClient_BuildRequest(t *testing.T) {
	c := &TestClient{BaseURL: "http://localhost:8080"}

	t.Run("simple request", func(t *testing.T) {
		req, err := c.BuildRequest("GET", "/health", nil, nil)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}

		if req.URL.String() != "http://localhost:8080/health" {
			t.Errorf("URL = %q, want %q", req.URL.String(), "http://localhost:8080/health")
		}
		if req.Method != "GET" {
			t.Errorf("Method = %q, want %q", req.Method, "GET")
		}
	})

	t.Run("with query params", func(t *testing.T) {
		query := url.Values{"limit": {"10"}, "offset": {"20"}}
		req, err := c.BuildRequest("GET", "/items", query, nil)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}

		// URL should include query string
		if !bytes.Contains([]byte(req.URL.String()), []byte("limit=10")) {
			t.Errorf("URL should contain query params: %s", req.URL.String())
		}
	})

	t.Run("with body sets content-type", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"name":"test"}`))
		req, err := c.BuildRequest("POST", "/items", nil, body)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}

		if ct := req.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
	})
}
