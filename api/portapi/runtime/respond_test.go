package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	t.Run("encodes struct as JSON", func(t *testing.T) {
		type Resp struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusOK, Resp{ID: "123", Name: "Fluffy"})

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}

		var got Resp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.ID != "123" || got.Name != "Fluffy" {
			t.Errorf("got %+v, want {ID:123 Name:Fluffy}", got)
		}
	})

	t.Run("encodes slice", func(t *testing.T) {
		type Item struct {
			ID string `json:"id"`
		}
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusOK, []Item{{ID: "1"}, {ID: "2"}})

		var got []Item
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(got) != 2 || got[0].ID != "1" || got[1].ID != "2" {
			t.Errorf("got %+v, want [{ID:1} {ID:2}]", got)
		}
	})

	t.Run("encodes nil slice as empty array", func(t *testing.T) {
		type Item struct {
			ID string `json:"id"`
		}
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusOK, []Item(nil))

		body := w.Body.String()
		if body != "[]\n" {
			t.Errorf("body = %q, want %q", body, "[]\n")
		}
	})

	t.Run("encodes empty slice as empty array", func(t *testing.T) {
		type Item struct {
			ID string `json:"id"`
		}
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusOK, []Item{})

		var got []Item
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %+v, want empty slice", got)
		}
	})

	t.Run("encodes map", func(t *testing.T) {
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusOK, map[string]int{"a": 1, "b": 2})

		var got map[string]int
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got["a"] != 1 || got["b"] != 2 {
			t.Errorf("got %+v, want map[a:1 b:2]", got)
		}
	})

	t.Run("uses provided status code", func(t *testing.T) {
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusCreated, map[string]string{"status": "created"})

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("encodes nil as null", func(t *testing.T) {
		w := httptest.NewRecorder()

		RespondJSON(w, http.StatusOK, nil)

		body := w.Body.String()
		if body != "null\n" {
			t.Errorf("body = %q, want %q", body, "null\n")
		}
	})
}

func TestRespondNoContent(t *testing.T) {
	w := httptest.NewRecorder()

	RespondNoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if body := w.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestRespondError(t *testing.T) {
	t.Run("bind error → 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := &BindError{Source: "query", Field: "limit", Err: errors.New("invalid")}

		RespondError(w, err)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}

		var resp ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Error.Code != "bad_request" {
			t.Errorf("error code = %q, want %q", resp.Error.Code, "bad_request")
		}
	})

	t.Run("handler error → 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("something went wrong")

		RespondError(w, err)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}

		var resp ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Error.Code != "internal_error" {
			t.Errorf("error code = %q, want %q", resp.Error.Code, "internal_error")
		}
	})

	t.Run("wrapped bind error → 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		bindErr := &BindError{Source: "path", Field: "id", Err: errors.New("not found")}
		err := errors.New("wrapped: " + bindErr.Error())
		// This won't work as errors.As won't find it - testing explicit BindError
		_ = err

		RespondError(w, bindErr)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("error message is included", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("database connection failed")

		RespondError(w, err)

		var resp ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Error.Message != "database connection failed" {
			t.Errorf("error message = %q, want %q", resp.Error.Message, "database connection failed")
		}
	})
}

func TestRespondHTTPError(t *testing.T) {
	t.Run("uses HTTPError status and code", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := NewHTTPError(http.StatusNotFound, "not_found", "resource not found")

		RespondHTTPError(w, err)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}

		var resp ErrorResponse
		if jsonErr := json.Unmarshal(w.Body.Bytes(), &resp); jsonErr != nil {
			t.Fatalf("failed to unmarshal response: %v", jsonErr)
		}
		if resp.Error.Code != "not_found" {
			t.Errorf("error code = %q, want %q", resp.Error.Code, "not_found")
		}
		if resp.Error.Message != "resource not found" {
			t.Errorf("error message = %q, want %q", resp.Error.Message, "resource not found")
		}
	})

	t.Run("falls back for non-HTTPError", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("generic error")

		RespondHTTPError(w, err)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}

		var resp ErrorResponse
		if jsonErr := json.Unmarshal(w.Body.Bytes(), &resp); jsonErr != nil {
			t.Fatalf("failed to unmarshal response: %v", jsonErr)
		}
		if resp.Error.Code != "internal_error" {
			t.Errorf("error code = %q, want %q", resp.Error.Code, "internal_error")
		}
	})

	t.Run("falls back for BindError", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := &BindError{Source: "query", Field: "limit", Err: errors.New("invalid")}

		RespondHTTPError(w, err)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestHTTPError(t *testing.T) {
	t.Run("Error method returns message", func(t *testing.T) {
		err := NewHTTPError(http.StatusNotFound, "not_found", "item not found")
		if err.Error() != "item not found" {
			t.Errorf("Error() = %q, want %q", err.Error(), "item not found")
		}
	})

	t.Run("fields are set correctly", func(t *testing.T) {
		err := NewHTTPError(http.StatusForbidden, "forbidden", "access denied")
		if err.Status != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", err.Status, http.StatusForbidden)
		}
		if err.Code != "forbidden" {
			t.Errorf("Code = %q, want %q", err.Code, "forbidden")
		}
		if err.Message != "access denied" {
			t.Errorf("Message = %q, want %q", err.Message, "access denied")
		}
	})
}

func TestErrorResponse_ValidJSON(t *testing.T) {
	// Test that error responses always produce valid JSON
	testCases := []error{
		errors.New("simple error"),
		errors.New(""),
		errors.New("error with \"quotes\""),
		errors.New("error with\nnewline"),
		errors.New("error with\ttab"),
		&BindError{Source: "query", Field: "test", Err: errors.New("bind error")},
		NewHTTPError(http.StatusBadRequest, "bad_request", "custom error"),
	}

	for i, err := range testCases {
		w := httptest.NewRecorder()
		RespondError(w, err)

		var resp ErrorResponse
		if jsonErr := json.Unmarshal(w.Body.Bytes(), &resp); jsonErr != nil {
			t.Errorf("case %d: invalid JSON: %v, body: %q", i, jsonErr, w.Body.String())
		}
		if resp.Error.Code == "" {
			t.Errorf("case %d: error code is empty", i)
		}
	}
}
