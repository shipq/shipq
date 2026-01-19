package portapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestProperty_JSONRoundtrip(t *testing.T) {
	mux := buildTestMux()

	// Test with various valid pet names
	testNames := []string{
		"Fluffy",
		"",
		"Max",
		"A very long pet name that goes on and on and on",
		"名前", // Unicode
		"Pet with \"quotes\"",
		"Pet with\nnewline",
		"🐕🐈", // Emojis
	}

	for _, name := range testNames {
		t.Run("name="+name, func(t *testing.T) {
			reqBody, err := json.Marshal(map[string]string{"name": name})
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			r := httptest.NewRequest("POST", "/pets", strings.NewReader(string(reqBody)))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			if w.Code != 200 {
				t.Errorf("got status %d, want 200", w.Code)
			}

			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if resp["name"] != name {
				t.Errorf("response name = %q, want %q", resp["name"], name)
			}
		})
	}
}

func TestProperty_PathVariableRoundtrip(t *testing.T) {
	mux := buildTestMux()

	// Test with various valid IDs (alphanumeric, no special chars that would conflict with URL)
	testIDs := []string{
		"123",
		"abc",
		"abc123",
		"ABC123",
		"a",
		"1",
		"very-long-id-with-dashes",
		"id_with_underscores",
	}

	for _, id := range testIDs {
		t.Run("id="+id, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/pets/"+id, nil)
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			if w.Code != 200 {
				t.Errorf("got status %d, want 200", w.Code)
			}

			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if resp["id"] != id {
				t.Errorf("response id = %q, want %q", resp["id"], id)
			}
		})
	}
}

func TestProperty_InvalidJSON_Never500(t *testing.T) {
	mux := buildTestMux()

	// Various invalid JSON strings
	invalidJSONs := []string{
		`{invalid}`,
		`{"name":}`,
		`{"name": undefined}`,
		`{name: "value"}`,
		`[{"name": "value"}`, // unclosed array
		`{"name": "value"`,   // unclosed object
		`{"name": 'single quotes'}`,
		`not json at all`,
		`12345`,          // valid JSON but not an object
		`["array"]`,      // valid JSON but array not object
		`{"name": "\x"}`, // invalid escape
	}

	for _, invalidJSON := range invalidJSONs {
		t.Run("json="+truncate(invalidJSON, 20), func(t *testing.T) {
			r := httptest.NewRequest("POST", "/pets", strings.NewReader(invalidJSON))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			// Should be 400 (bad request), never 500 (server error)
			if w.Code != 400 {
				t.Errorf("got status %d, want 400 for invalid JSON", w.Code)
			}
		})
	}
}

func TestProperty_ErrorResponse_AlwaysValidJSON(t *testing.T) {
	mux := buildTestMux()

	// Requests that should cause errors
	errorCausingRequests := []struct {
		method string
		path   string
		body   string
		desc   string
	}{
		{"POST", "/pets", `{invalid}`, "invalid JSON body"},
		{"POST", "/pets", ``, "empty body"},
		{"GET", "/pets/123", "", "missing required header"},
		{"GET", "/pets/123?verbose=notabool", "", "invalid query param type"},
	}

	for _, tc := range errorCausingRequests {
		t.Run(tc.desc, func(t *testing.T) {
			var r *http.Request
			if tc.body != "" {
				r = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				r.Header.Set("Content-Type", "application/json")
			} else {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			}
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			// Should be an error status
			if w.Code < 400 {
				t.Errorf("got status %d, want >= 400", w.Code)
			}

			// Response should be valid JSON
			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Errorf("error response body should be valid JSON: %v", err)
			}

			// Should have an "error" key
			if _, ok := resp["error"]; !ok {
				t.Error("error response should have 'error' key")
			}

			// Content-Type should be application/json
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("got Content-Type %q, want %q", ct, "application/json")
			}
		})
	}
}

func TestProperty_ValidRequest_Never500(t *testing.T) {
	mux := buildTestMux()

	// All valid requests should never result in 500
	validRequests := []struct {
		method string
		path   string
		body   string
		header map[string]string
	}{
		{"GET", "/pets", "", nil},
		{"GET", "/health", "", nil},
		{"POST", "/pets", `{"name":"Test"}`, map[string]string{"Content-Type": "application/json"}},
		{"DELETE", "/pets/123", "", nil},
		{"GET", "/pets/123", "", map[string]string{"Authorization": "Bearer token"}},
		{"GET", "/pets/123?verbose=true", "", map[string]string{"Authorization": "Bearer token"}},
		{"GET", "/pets/123?verbose=false", "", map[string]string{"Authorization": "Bearer token"}},
	}

	for _, tc := range validRequests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var r *http.Request
			if tc.body != "" {
				r = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			} else {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			}
			for k, v := range tc.header {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			if w.Code >= 500 {
				t.Errorf("got status %d, valid request should never result in 5xx error", w.Code)
			}
		})
	}
}

func TestProperty_ContentType_AlwaysSet(t *testing.T) {
	mux := buildTestMux()

	// All requests that return a body should have Content-Type set
	requests := []struct {
		method string
		path   string
		body   string
		header map[string]string
	}{
		{"GET", "/pets", "", nil},
		{"POST", "/pets", `{"name":"Test"}`, map[string]string{"Content-Type": "application/json"}},
		{"GET", "/pets/123", "", map[string]string{"Authorization": "Bearer token"}},
		// Error responses
		{"POST", "/pets", `{invalid}`, map[string]string{"Content-Type": "application/json"}},
		{"GET", "/pets/123", "", nil}, // missing header causes error
	}

	for _, tc := range requests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var r *http.Request
			if tc.body != "" {
				r = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			} else {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			}
			for k, v := range tc.header {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			// If there's a body, Content-Type should be set
			if w.Body.Len() > 0 {
				if ct := w.Header().Get("Content-Type"); ct == "" {
					t.Error("responses with body should have Content-Type set")
				}
			}
		})
	}
}

func TestProperty_ResponseBody_AlwaysValidUTF8(t *testing.T) {
	mux := buildTestMux()

	requests := []struct {
		method string
		path   string
		body   string
		header map[string]string
	}{
		{"GET", "/pets", "", nil},
		{"POST", "/pets", `{"name":"Test"}`, map[string]string{"Content-Type": "application/json"}},
		{"POST", "/pets", `{"name":"名前🐕"}`, map[string]string{"Content-Type": "application/json"}},
		{"GET", "/pets/123", "", map[string]string{"Authorization": "Bearer token"}},
	}

	for _, tc := range requests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var r *http.Request
			if tc.body != "" {
				r = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			} else {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			}
			for k, v := range tc.header {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			if !utf8.Valid(w.Body.Bytes()) {
				t.Error("response body should be valid UTF-8")
			}
		})
	}
}

// truncate shortens a string for display in test names
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
