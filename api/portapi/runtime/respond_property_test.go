//go:build property

package runtime

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"unicode/utf8"
)

// TestProperty_RespondError_AlwaysValidJSON tests that RespondError always produces valid JSON
// regardless of the error message content.
func TestProperty_RespondError_AlwaysValidJSON(t *testing.T) {
	// Generate random error messages including edge cases
	edgeCases := []string{
		"",
		" ",
		"\t\n\r",
		"simple error",
		"error with \"quotes\"",
		"error with 'single quotes'",
		"error with\nnewline",
		"error with\ttab",
		"error with\rcarriage return",
		"error with\x00null byte",
		"error with unicode: 日本語",
		"error with emoji: 🔥💀",
		"error with backslash: \\path\\to\\file",
		"error with </script> tag",
		"error with & ampersand",
		"error with < and > brackets",
		`{"attempted":"json injection"}`,
		`["array", "injection"]`,
		"very long error: " + string(make([]byte, 10000)),
	}

	// Add some random strings
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		length := r.Intn(500)
		bytes := make([]byte, length)
		for j := 0; j < length; j++ {
			bytes[j] = byte(r.Intn(256))
		}
		// Convert to valid UTF-8
		if utf8.Valid(bytes) {
			edgeCases = append(edgeCases, string(bytes))
		}
	}

	for i, msg := range edgeCases {
		t.Run("", func(t *testing.T) {
			w := httptest.NewRecorder()
			err := errors.New(msg)

			RespondError(w, err)

			// Verify it's valid JSON
			var resp ErrorResponse
			if jsonErr := json.Unmarshal(w.Body.Bytes(), &resp); jsonErr != nil {
				t.Errorf("case %d: invalid JSON for message %q: %v", i, truncateForLog(msg, 50), jsonErr)
			}

			// Verify structure
			if resp.Error.Code == "" {
				t.Errorf("case %d: error code is empty", i)
			}
		})
	}
}

// TestProperty_RespondError_StableStructure tests that error responses always have
// the expected structure with "error" containing "code" and "message".
func TestProperty_RespondError_StableStructure(t *testing.T) {
	testErrors := []error{
		errors.New("generic error"),
		&BindError{Source: "query", Field: "limit", Err: errors.New("invalid")},
		&BindError{Source: "path", Field: "id", Err: errors.New("missing")},
		&BindError{Source: "header", Field: "auth", Err: errors.New("required")},
		&BindError{Source: "body", Err: errors.New("invalid json")},
	}

	for _, err := range testErrors {
		w := httptest.NewRecorder()
		RespondError(w, err)

		// Parse as generic map to verify structure
		var raw map[string]interface{}
		if jsonErr := json.Unmarshal(w.Body.Bytes(), &raw); jsonErr != nil {
			t.Errorf("invalid JSON: %v", jsonErr)
			continue
		}

		// Check "error" key exists
		errorObj, ok := raw["error"]
		if !ok {
			t.Error("response missing 'error' key")
			continue
		}

		// Check error is an object
		errorMap, ok := errorObj.(map[string]interface{})
		if !ok {
			t.Error("'error' is not an object")
			continue
		}

		// Check "code" exists and is string
		code, ok := errorMap["code"]
		if !ok {
			t.Error("error missing 'code' key")
		} else if _, ok := code.(string); !ok {
			t.Error("'code' is not a string")
		}

		// Check "message" exists and is string
		msg, ok := errorMap["message"]
		if !ok {
			t.Error("error missing 'message' key")
		} else if _, ok := msg.(string); !ok {
			t.Error("'message' is not a string")
		}
	}
}

// TestProperty_RespondJSON_AlwaysValidJSON tests that RespondJSON produces valid JSON
// for various data types.
func TestProperty_RespondJSON_AlwaysValidJSON(t *testing.T) {
	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	testData := []interface{}{
		nil,
		"string",
		42,
		3.14,
		true,
		false,
		[]string{},
		[]string{"a", "b"},
		[]string(nil),
		[]Item{},
		[]Item{{ID: "1", Name: "one"}},
		[]Item(nil),
		map[string]string{},
		map[string]string{"key": "value"},
		map[string]int{"a": 1, "b": 2},
		Item{ID: "123", Name: "test"},
		struct {
			Nested Item `json:"nested"`
		}{Nested: Item{ID: "n1", Name: "nested"}},
	}

	for i, data := range testData {
		w := httptest.NewRecorder()
		RespondJSON(w, http.StatusOK, data)

		// Verify Content-Type
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("case %d: Content-Type = %q, want %q", i, ct, "application/json")
		}

		// Verify valid JSON (or valid JSON primitive)
		body := w.Body.Bytes()
		if !json.Valid(body) {
			t.Errorf("case %d: invalid JSON: %q", i, string(body))
		}
	}
}

// TestProperty_RespondJSON_NilSliceConsistency tests that nil slices always render as [].
func TestProperty_RespondJSON_NilSliceConsistency(t *testing.T) {
	type Item struct {
		ID string `json:"id"`
	}

	nilSlices := []interface{}{
		[]string(nil),
		[]int(nil),
		[]Item(nil),
		[]interface{}(nil),
	}

	for i, data := range nilSlices {
		for j := 0; j < 10; j++ {
			w := httptest.NewRecorder()
			RespondJSON(w, http.StatusOK, data)

			body := w.Body.String()
			if body != "[]\n" {
				t.Errorf("case %d iteration %d: nil slice rendered as %q, want %q", i, j, body, "[]\n")
			}
		}
	}
}

// TestProperty_RespondError_StatusCodeConsistency tests that error types consistently
// produce the same status codes.
func TestProperty_RespondError_StatusCodeConsistency(t *testing.T) {
	bindErrors := []*BindError{
		{Source: "query", Field: "a", Err: errors.New("err1")},
		{Source: "path", Field: "b", Err: errors.New("err2")},
		{Source: "header", Field: "c", Err: errors.New("err3")},
		{Source: "body", Err: errors.New("err4")},
	}

	for _, bindErr := range bindErrors {
		for i := 0; i < 10; i++ {
			w := httptest.NewRecorder()
			RespondError(w, bindErr)

			if w.Code != http.StatusBadRequest {
				t.Errorf("BindError should always produce 400, got %d", w.Code)
			}
		}
	}

	genericErrors := []error{
		errors.New("error1"),
		errors.New("error2"),
		errors.New(""),
	}

	for _, err := range genericErrors {
		for i := 0; i < 10; i++ {
			w := httptest.NewRecorder()
			RespondError(w, err)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("generic error should always produce 500, got %d", w.Code)
			}
		}
	}
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
