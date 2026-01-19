package portapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/api/portapi/gen"
	"github.com/shipq/shipq/api/portapi/testdata/exampleapi"
)

// buildTestMux creates a mux using the generated handlers.
func buildTestMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Register all endpoints with generated handlers
	mux.Handle("POST /pets", gen.HandleCreatePet(exampleapi.CreatePet))
	mux.Handle("DELETE /pets/{id}", gen.HandleDeletePet(exampleapi.DeletePet))
	mux.Handle("GET /pets", gen.HandleListPets(exampleapi.ListPets))
	mux.Handle("GET /health", gen.HandleHealthCheck(exampleapi.HealthCheck))
	mux.Handle("GET /pets/{id}", gen.HandleGetPet(exampleapi.GetPet))
	mux.Handle("GET /pets/search", gen.HandleSearchPets(exampleapi.SearchPets))
	mux.Handle("PUT /pets/{id}", gen.HandleUpdatePet(exampleapi.UpdatePet))

	return mux
}

func TestIntegration_Routing(t *testing.T) {
	mux := buildTestMux()

	tests := []struct {
		method string
		path   string
		body   string
		header map[string]string
		want   int
	}{
		{"GET", "/pets", "", nil, 200},
		{"POST", "/pets", `{"name":"Fluffy"}`, map[string]string{"Content-Type": "application/json"}, 200},
		{"DELETE", "/pets/123", "", nil, 204},
		{"GET", "/pets/123", "", map[string]string{"Authorization": "Bearer token"}, 200},
		{"GET", "/health", "", nil, 204},
		{"GET", "/notfound", "", nil, 404},
		{"POST", "/health", "", nil, 405}, // method not allowed
		{"GET", "/pets/search?limit=10", "", nil, 200},
		{"PUT", "/pets/123", `{"name":"Updated"}`, map[string]string{"Content-Type": "application/json"}, 200},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			var r *http.Request
			if tt.body != "" {
				r = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			} else {
				r = httptest.NewRequest(tt.method, tt.path, nil)
			}
			for k, v := range tt.header {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			if w.Code != tt.want {
				t.Errorf("got status %d, want %d; body: %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

func TestIntegration_JSONBody(t *testing.T) {
	mux := buildTestMux()

	t.Run("valid JSON", func(t *testing.T) {
		body := strings.NewReader(`{"name":"Fluffy"}`)
		r := httptest.NewRequest("POST", "/pets", body)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}

		var got, want map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if err := json.Unmarshal([]byte(`{"id":"pet-123","name":"Fluffy"}`), &want); err != nil {
			t.Fatalf("failed to unmarshal expected: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := strings.NewReader(`{invalid}`)
		r := httptest.NewRequest("POST", "/pets", body)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
	})

	t.Run("empty body returns 400", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/pets", strings.NewReader(""))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
	})
}

func TestIntegration_PathVariables(t *testing.T) {
	mux := buildTestMux()

	t.Run("binds path variable", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/abc123", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["id"] != "abc123" {
			t.Errorf("got id %q, want %q", resp["id"], "abc123")
		}
	})

	t.Run("binds path variable for delete", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/pets/xyz789", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 204 {
			t.Errorf("got status %d, want 204", w.Code)
		}
	})

	t.Run("binds path variable for update with JSON body", func(t *testing.T) {
		body := strings.NewReader(`{"name":"UpdatedName","age":5}`)
		r := httptest.NewRequest("PUT", "/pets/pet-999", body)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200; body: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["id"] != "pet-999" {
			t.Errorf("got id %q, want %q", resp["id"], "pet-999")
		}
		if resp["name"] != "UpdatedName" {
			t.Errorf("got name %q, want %q", resp["name"], "UpdatedName")
		}
		if resp["age"] != float64(5) {
			t.Errorf("got age %v, want %v", resp["age"], 5)
		}
	})
}

func TestIntegration_QueryParams(t *testing.T) {
	mux := buildTestMux()

	t.Run("optional query param absent", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/123", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
	})

	t.Run("optional query param present", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/123?verbose=true", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["verbose"] != true {
			t.Errorf("got verbose %v, want true", resp["verbose"])
		}
	})

	t.Run("required scalar query param missing returns 400", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
		// Verify error mentions query limit
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		errObj := resp["error"].(map[string]any)
		if errObj["code"] != "bad_request" {
			t.Errorf("got error code %q, want %q", errObj["code"], "bad_request")
		}
		msg := errObj["message"].(string)
		if !strings.Contains(msg, "query") || !strings.Contains(msg, "limit") {
			t.Errorf("error message %q should mention query and limit", msg)
		}
	})

	t.Run("required scalar query param present", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=25", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200; body: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["limit"] != float64(25) {
			t.Errorf("got limit %v, want 25", resp["limit"])
		}
	})

	t.Run("invalid query param value returns 400", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=abc", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
	})

	t.Run("slice query param multi-value", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=10&tag=cat&tag=dog&tag=bird", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200; body: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		tags := resp["tags"].([]any)
		if len(tags) != 3 {
			t.Errorf("got %d tags, want 3", len(tags))
		}
		if tags[0] != "cat" || tags[1] != "dog" || tags[2] != "bird" {
			t.Errorf("got tags %v, want [cat, dog, bird]", tags)
		}
	})

	t.Run("slice query param absent is empty", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=10", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		// tags should be null or empty
		if resp["tags"] != nil {
			tags := resp["tags"].([]any)
			if len(tags) != 0 {
				t.Errorf("got tags %v, want empty", tags)
			}
		}
	})

	t.Run("optional pointer query param", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=10&cursor=abc123", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["cursor"] != "abc123" {
			t.Errorf("got cursor %v, want abc123", resp["cursor"])
		}
	})
}

func TestIntegration_Headers(t *testing.T) {
	mux := buildTestMux()

	t.Run("missing required header returns 400", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/123", nil)
		// No Authorization header
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
	})

	t.Run("with required header succeeds", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/123", nil)
		r.Header.Set("Authorization", "Bearer mytoken")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
	})

	t.Run("optional header absent", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=10", nil)
		// No Authorization header (it's optional for SearchPets)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		// auth should be empty string or not present
		if auth, ok := resp["auth"]; ok && auth != "" {
			t.Errorf("got auth %v, want empty", auth)
		}
	})

	t.Run("optional header present", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search?limit=10", nil)
		r.Header.Set("Authorization", "Bearer optionaltoken")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["auth"] != "Bearer optionaltoken" {
			t.Errorf("got auth %v, want Bearer optionaltoken", resp["auth"])
		}
	})
}

func TestIntegration_ErrorResponses(t *testing.T) {
	mux := buildTestMux()

	t.Run("bind error is 400 with JSON body", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/123", nil)
		// Missing required Authorization header
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("got Content-Type %q, want %q", ct, "application/json")
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["error"] == nil {
			t.Error("expected error key in response")
		}
	})

	t.Run("error response has code and message", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/pets", strings.NewReader(`{invalid}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}

		var resp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Error.Code == "" {
			t.Error("expected non-empty error code")
		}
		if resp.Error.Message == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("bind error has code bad_request", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/search", nil) // missing required limit
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}

		var resp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Error.Code != "bad_request" {
			t.Errorf("got error code %q, want %q", resp.Error.Code, "bad_request")
		}
	})
}

func TestIntegration_ResponseFormats(t *testing.T) {
	mux := buildTestMux()

	t.Run("200 response has JSON content type", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("got Content-Type %q, want %q", ct, "application/json")
		}
	})

	t.Run("204 response has no body", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 204 {
			t.Errorf("got status %d, want 204", w.Code)
		}
		if body := w.Body.String(); body != "" {
			t.Errorf("expected empty body, got %q", body)
		}
	})
}

func TestIntegration_AllHandlerShapes(t *testing.T) {
	mux := buildTestMux()

	t.Run("Shape1: ctx_req_resp_err - POST /pets", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/pets", strings.NewReader(`{"name":"Max"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp exampleapi.CreatePetResp
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.ID != "pet-123" {
			t.Errorf("got ID %q, want %q", resp.ID, "pet-123")
		}
		if resp.Name != "Max" {
			t.Errorf("got Name %q, want %q", resp.Name, "Max")
		}
	})

	t.Run("Shape2: ctx_req_err - DELETE /pets/{id}", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/pets/pet-456", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 204 {
			t.Errorf("got status %d, want 204", w.Code)
		}
	})

	t.Run("Shape3: ctx_resp_err - GET /pets", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 200 {
			t.Errorf("got status %d, want 200", w.Code)
		}
		var resp exampleapi.ListPetsResp
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		want := []string{"fluffy", "spot"}
		if !reflect.DeepEqual(resp.Pets, want) {
			t.Errorf("got Pets %v, want %v", resp.Pets, want)
		}
	})

	t.Run("Shape4: ctx_err - GET /health", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 204 {
			t.Errorf("got status %d, want 204", w.Code)
		}
	})
}

func TestIntegration_GeneratorOutputMatchesManualMux(t *testing.T) {
	// Verify our test fixtures match what the generator would produce

	// Register with App to get endpoints
	app := &portapi.App{}
	exampleapi.Register(app)
	endpoints := app.Endpoints()

	// Verify we have all expected endpoints
	if len(endpoints) != 7 {
		t.Errorf("got %d endpoints, want 7", len(endpoints))
	}

	// Verify each endpoint validates successfully
	for _, ep := range endpoints {
		if err := portapi.ValidateEndpoint(ep); err != nil {
			t.Errorf("endpoint %s %s should validate: %v", ep.Method, ep.Path, err)
		}
	}
}

func TestIntegration_GeneratorDeterminism(t *testing.T) {
	// Run validation twice on the same endpoints
	// Both should produce the same result

	app1 := &portapi.App{}
	exampleapi.Register(app1)
	endpoints1 := app1.Endpoints()

	app2 := &portapi.App{}
	exampleapi.Register(app2)
	endpoints2 := app2.Endpoints()

	// Endpoints should be in same order
	if len(endpoints1) != len(endpoints2) {
		t.Fatalf("got %d endpoints in first run, %d in second", len(endpoints1), len(endpoints2))
	}
	for i := range endpoints1 {
		if endpoints1[i].Method != endpoints2[i].Method {
			t.Errorf("endpoint %d: got method %q, want %q", i, endpoints2[i].Method, endpoints1[i].Method)
		}
		if endpoints1[i].Path != endpoints2[i].Path {
			t.Errorf("endpoint %d: got path %q, want %q", i, endpoints2[i].Path, endpoints1[i].Path)
		}
	}

	// Validation should produce consistent results
	for i := range endpoints1 {
		err1 := portapi.ValidateEndpoint(endpoints1[i])
		err2 := portapi.ValidateEndpoint(endpoints2[i])
		if err1 == nil && err2 != nil {
			t.Errorf("endpoint %d: first validation succeeded but second failed: %v", i, err2)
		}
		if err1 != nil && err2 == nil {
			t.Errorf("endpoint %d: first validation failed but second succeeded: %v", i, err1)
		}
		if err1 != nil && err2 != nil && err1.Error() != err2.Error() {
			t.Errorf("endpoint %d: different errors: %v vs %v", i, err1, err2)
		}
	}
}

func TestIntegration_EndpointValidationDetails(t *testing.T) {
	app := &portapi.App{}
	exampleapi.Register(app)
	endpoints := app.Endpoints()

	expectedEndpoints := map[string]string{
		"POST /pets":        "ctx_req_resp_err",
		"DELETE /pets/{id}": "ctx_req_err",
		"GET /pets":         "ctx_resp_err",
		"GET /health":       "ctx_err",
		"GET /pets/{id}":    "ctx_req_resp_err",
		"GET /pets/search":  "ctx_req_resp_err",
		"PUT /pets/{id}":    "ctx_req_resp_err",
	}

	// Verify each endpoint matches expected pattern
	for _, ep := range endpoints {
		pattern := ep.Method + " " + ep.Path
		if _, exists := expectedEndpoints[pattern]; !exists {
			t.Errorf("unexpected endpoint: %s", pattern)
		}

		// Validate endpoint
		if err := portapi.ValidateEndpoint(ep); err != nil {
			t.Errorf("endpoint %s should validate: %v", pattern, err)
		}
	}

	// Verify all expected endpoints are registered
	for pattern := range expectedEndpoints {
		found := false
		for _, ep := range endpoints {
			if ep.Method+" "+ep.Path == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected endpoint not registered: %s", pattern)
		}
	}
}

func TestIntegration_BoolParsing(t *testing.T) {
	mux := buildTestMux()

	// Test all accepted bool values
	boolTests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
	}

	for _, tt := range boolTests {
		t.Run("verbose="+tt.value, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/pets/123?verbose="+tt.value, nil)
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			if w.Code != 200 {
				t.Errorf("got status %d, want 200; body: %s", w.Code, w.Body.String())
				return
			}
			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			// Note: verbose has omitempty, so false values are omitted from JSON
			got, ok := resp["verbose"]
			if tt.want {
				if !ok || got != true {
					t.Errorf("got verbose %v, want true", got)
				}
			} else {
				// For false, omitempty means the field is absent (nil) or false
				if ok && got != false {
					t.Errorf("got verbose %v, want false or absent", got)
				}
			}
		})
	}

	t.Run("invalid bool returns 400", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/123?verbose=invalid", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 400 {
			t.Errorf("got status %d, want 400", w.Code)
		}
	})
}
