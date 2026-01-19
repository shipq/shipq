package runtime

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBind_PathVariables(t *testing.T) {
	type Req struct {
		ID string `path:"id"`
	}

	t.Run("binds path variable", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/abc123", nil)
		r.SetPathValue("id", "abc123")

		var req Req
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.ID != "abc123" {
			t.Errorf("got %q, want %q", req.ID, "abc123")
		}
	})

	t.Run("converts path variable to int", func(t *testing.T) {
		type ReqInt struct {
			ID int `path:"id"`
		}
		r := httptest.NewRequest("GET", "/pets/42", nil)
		r.SetPathValue("id", "42")

		var req ReqInt
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.ID != 42 {
			t.Errorf("got %d, want %d", req.ID, 42)
		}
	})

	t.Run("converts path variable to int64", func(t *testing.T) {
		type ReqInt64 struct {
			ID int64 `path:"id"`
		}
		r := httptest.NewRequest("GET", "/pets/9223372036854775807", nil)
		r.SetPathValue("id", "9223372036854775807")

		var req ReqInt64
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.ID != 9223372036854775807 {
			t.Errorf("got %d, want %d", req.ID, int64(9223372036854775807))
		}
	})

	t.Run("missing path variable", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets/", nil)
		// No SetPathValue called

		var req Req
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "id") {
			t.Errorf("error should mention 'id', got: %v", err)
		}
	})

	t.Run("invalid path variable conversion", func(t *testing.T) {
		type ReqInt struct {
			ID int `path:"id"`
		}
		r := httptest.NewRequest("GET", "/pets/notanint", nil)
		r.SetPathValue("id", "notanint")

		var req ReqInt
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		bindErr, ok := err.(*BindError)
		if !ok {
			t.Fatalf("expected *BindError, got %T", err)
		}
		if bindErr.Source != "path" {
			t.Errorf("Source = %q, want %q", bindErr.Source, "path")
		}
	})

	t.Run("multiple path variables", func(t *testing.T) {
		type ReqMulti struct {
			UserID string `path:"user_id"`
			PostID int    `path:"post_id"`
		}
		r := httptest.NewRequest("GET", "/users/user123/posts/456", nil)
		r.SetPathValue("user_id", "user123")
		r.SetPathValue("post_id", "456")

		var req ReqMulti
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.UserID != "user123" {
			t.Errorf("UserID = %q, want %q", req.UserID, "user123")
		}
		if req.PostID != 456 {
			t.Errorf("PostID = %d, want %d", req.PostID, 456)
		}
	})
}

func TestBind_QueryParams(t *testing.T) {
	type Req struct {
		Limit int      `query:"limit"`
		Tags  []string `query:"tag"`
	}

	t.Run("binds query params", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets?limit=10&tag=cute&tag=small", nil)

		var req Req
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Limit != 10 {
			t.Errorf("Limit = %d, want %d", req.Limit, 10)
		}
		if len(req.Tags) != 2 || req.Tags[0] != "cute" || req.Tags[1] != "small" {
			t.Errorf("Tags = %v, want [cute small]", req.Tags)
		}
	})

	t.Run("missing required query param", func(t *testing.T) {
		type ReqRequired struct {
			Limit int `query:"limit"`
		}
		r := httptest.NewRequest("GET", "/pets", nil)

		var req ReqRequired
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		bindErr, ok := err.(*BindError)
		if !ok {
			t.Fatalf("expected *BindError, got %T", err)
		}
		if bindErr.Source != "query" {
			t.Errorf("Source = %q, want %q", bindErr.Source, "query")
		}
	})

	t.Run("optional query param (pointer) - absent", func(t *testing.T) {
		type ReqOptional struct {
			Limit *int `query:"limit"`
		}
		r := httptest.NewRequest("GET", "/pets", nil)

		var req ReqOptional
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Limit != nil {
			t.Errorf("Limit should be nil, got %v", *req.Limit)
		}
	})

	t.Run("optional query param (pointer) - present", func(t *testing.T) {
		type ReqOptional struct {
			Limit *int `query:"limit"`
		}
		r := httptest.NewRequest("GET", "/pets?limit=5", nil)

		var req ReqOptional
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Limit == nil {
			t.Fatal("Limit should not be nil")
		}
		if *req.Limit != 5 {
			t.Errorf("*Limit = %d, want %d", *req.Limit, 5)
		}
	})

	t.Run("query param conversion error", func(t *testing.T) {
		type ReqInt struct {
			Limit int `query:"limit"`
		}
		r := httptest.NewRequest("GET", "/pets?limit=notanint", nil)

		var req ReqInt
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("slice query param - int", func(t *testing.T) {
		type ReqIntSlice struct {
			IDs []int `query:"id"`
		}
		r := httptest.NewRequest("GET", "/items?id=1&id=2&id=3", nil)

		var req ReqIntSlice
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.IDs) != 3 || req.IDs[0] != 1 || req.IDs[1] != 2 || req.IDs[2] != 3 {
			t.Errorf("IDs = %v, want [1 2 3]", req.IDs)
		}
	})

	t.Run("empty slice query param", func(t *testing.T) {
		type ReqSlice struct {
			Tags []string `query:"tag"`
		}
		r := httptest.NewRequest("GET", "/pets", nil)

		var req ReqSlice
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Tags) != 0 {
			t.Errorf("Tags should be nil or empty, got %v", req.Tags)
		}
	})

	t.Run("bool query param", func(t *testing.T) {
		type ReqBool struct {
			Active bool `query:"active"`
		}
		r := httptest.NewRequest("GET", "/items?active=true", nil)

		var req ReqBool
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Active != true {
			t.Errorf("Active = %v, want true", req.Active)
		}
	})

	t.Run("optional string pointer query param", func(t *testing.T) {
		type ReqOptStr struct {
			Name *string `query:"name"`
		}
		r := httptest.NewRequest("GET", "/items?name=test", nil)

		var req ReqOptStr
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Name == nil || *req.Name != "test" {
			t.Errorf("Name = %v, want 'test'", req.Name)
		}
	})
}

func TestBind_Headers(t *testing.T) {
	type Req struct {
		RequestID string `header:"X-Request-Id"`
	}

	t.Run("binds header", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets", nil)
		r.Header.Set("X-Request-Id", "req-123")

		var req Req
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.RequestID != "req-123" {
			t.Errorf("RequestID = %q, want %q", req.RequestID, "req-123")
		}
	})

	t.Run("missing required header", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets", nil)

		var req Req
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		bindErr, ok := err.(*BindError)
		if !ok {
			t.Fatalf("expected *BindError, got %T", err)
		}
		if bindErr.Source != "header" {
			t.Errorf("Source = %q, want %q", bindErr.Source, "header")
		}
	})

	t.Run("optional header (pointer) - absent", func(t *testing.T) {
		type ReqOptional struct {
			TraceID *string `header:"X-Trace-Id"`
		}
		r := httptest.NewRequest("GET", "/pets", nil)

		var req ReqOptional
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.TraceID != nil {
			t.Errorf("TraceID should be nil, got %v", *req.TraceID)
		}
	})

	t.Run("optional header (pointer) - present", func(t *testing.T) {
		type ReqOptional struct {
			TraceID *string `header:"X-Trace-Id"`
		}
		r := httptest.NewRequest("GET", "/pets", nil)
		r.Header.Set("X-Trace-Id", "trace-456")

		var req ReqOptional
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.TraceID == nil {
			t.Fatal("TraceID should not be nil")
		}
		if *req.TraceID != "trace-456" {
			t.Errorf("*TraceID = %q, want %q", *req.TraceID, "trace-456")
		}
	})

	t.Run("multiple headers", func(t *testing.T) {
		type ReqMulti struct {
			Auth      string `header:"Authorization"`
			RequestID string `header:"X-Request-Id"`
		}
		r := httptest.NewRequest("GET", "/pets", nil)
		r.Header.Set("Authorization", "Bearer token123")
		r.Header.Set("X-Request-Id", "req-789")

		var req ReqMulti
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Auth != "Bearer token123" {
			t.Errorf("Auth = %q, want %q", req.Auth, "Bearer token123")
		}
		if req.RequestID != "req-789" {
			t.Errorf("RequestID = %q, want %q", req.RequestID, "req-789")
		}
	})
}

func TestBind_JSONBody(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("binds JSON body", func(t *testing.T) {
		body := strings.NewReader(`{"name":"Fluffy","age":3}`)
		r := httptest.NewRequest("POST", "/pets", body)
		r.Header.Set("Content-Type", "application/json")

		var req Req
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Name != "Fluffy" {
			t.Errorf("Name = %q, want %q", req.Name, "Fluffy")
		}
		if req.Age != 3 {
			t.Errorf("Age = %d, want %d", req.Age, 3)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		body := strings.NewReader(`{invalid}`)
		r := httptest.NewRequest("POST", "/pets", body)
		r.Header.Set("Content-Type", "application/json")

		var req Req
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		bindErr, ok := err.(*BindError)
		if !ok {
			t.Fatalf("expected *BindError, got %T", err)
		}
		if bindErr.Source != "body" {
			t.Errorf("Source = %q, want %q", bindErr.Source, "body")
		}
	})

	t.Run("empty body when JSON expected", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/pets", strings.NewReader(""))
		r.Header.Set("Content-Type", "application/json")

		var req Req
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("nil body when JSON expected", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/pets", nil)
		r.Header.Set("Content-Type", "application/json")

		var req Req
		err := Bind(r, &req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("partial JSON - extra fields ignored", func(t *testing.T) {
		body := strings.NewReader(`{"name":"Fluffy","age":3,"extra":"ignored"}`)
		r := httptest.NewRequest("POST", "/pets", body)

		var req Req
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Name != "Fluffy" {
			t.Errorf("Name = %q, want %q", req.Name, "Fluffy")
		}
	})

	t.Run("JSON with nested struct", func(t *testing.T) {
		type Address struct {
			City string `json:"city"`
		}
		type ReqNested struct {
			Name    string  `json:"name"`
			Address Address `json:"address"`
		}
		body := strings.NewReader(`{"name":"John","address":{"city":"NYC"}}`)
		r := httptest.NewRequest("POST", "/users", body)

		var req ReqNested
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Name != "John" {
			t.Errorf("Name = %q, want %q", req.Name, "John")
		}
		if req.Address.City != "NYC" {
			t.Errorf("Address.City = %q, want %q", req.Address.City, "NYC")
		}
	})
}

func TestBind_Mixed(t *testing.T) {
	type Req struct {
		ID    string `path:"id"`
		Limit int    `query:"limit"`
		Auth  string `header:"Authorization"`
		Name  string `json:"name"`
	}

	t.Run("binds from all sources", func(t *testing.T) {
		body := strings.NewReader(`{"name":"Fluffy"}`)
		r := httptest.NewRequest("PUT", "/pets/pet123?limit=10", body)
		r.SetPathValue("id", "pet123")
		r.Header.Set("Authorization", "Bearer token")
		r.Header.Set("Content-Type", "application/json")

		var req Req
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.ID != "pet123" {
			t.Errorf("ID = %q, want %q", req.ID, "pet123")
		}
		if req.Limit != 10 {
			t.Errorf("Limit = %d, want %d", req.Limit, 10)
		}
		if req.Auth != "Bearer token" {
			t.Errorf("Auth = %q, want %q", req.Auth, "Bearer token")
		}
		if req.Name != "Fluffy" {
			t.Errorf("Name = %q, want %q", req.Name, "Fluffy")
		}
	})

	t.Run("path and query only - no JSON", func(t *testing.T) {
		type ReqNoJSON struct {
			ID    string `path:"id"`
			Limit int    `query:"limit"`
		}
		r := httptest.NewRequest("GET", "/pets/pet123?limit=5", nil)
		r.SetPathValue("id", "pet123")

		var req ReqNoJSON
		err := Bind(r, &req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.ID != "pet123" {
			t.Errorf("ID = %q, want %q", req.ID, "pet123")
		}
		if req.Limit != 5 {
			t.Errorf("Limit = %d, want %d", req.Limit, 5)
		}
	})
}

func TestBind_Errors(t *testing.T) {
	t.Run("non-pointer req", func(t *testing.T) {
		type Req struct {
			ID string `path:"id"`
		}
		r := httptest.NewRequest("GET", "/pets/123", nil)
		r.SetPathValue("id", "123")

		var req Req
		err := Bind(r, req) // Not a pointer
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("pointer to non-struct", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/pets", nil)

		var s string
		err := Bind(r, &s)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestBindError_Error(t *testing.T) {
	t.Run("with field", func(t *testing.T) {
		err := &BindError{
			Source: "query",
			Field:  "limit",
			Err:    errors.New("invalid value"),
		}
		got := err.Error()
		if got != "query limit: invalid value" {
			t.Errorf("got %q, want %q", got, "query limit: invalid value")
		}
	})

	t.Run("without field", func(t *testing.T) {
		err := &BindError{
			Source: "body",
			Err:    errors.New("parse error"),
		}
		got := err.Error()
		if got != "body: parse error" {
			t.Errorf("got %q, want %q", got, "body: parse error")
		}
	})
}
