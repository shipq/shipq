package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrapCtxReqRespErr(t *testing.T) {
	type Req struct {
		ID string `path:"id"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	t.Run("success", func(t *testing.T) {
		handler := func(ctx context.Context, req Req) (Resp, error) {
			return Resp{Name: "Pet-" + req.ID}, nil
		}

		wrapped := WrapCtxReqRespErr(handler)

		r := httptest.NewRequest("GET", "/pets/123", nil)
		r.SetPathValue("id", "123")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), `"name":"Pet-123"`) {
			t.Errorf("body = %q, want to contain 'name:Pet-123'", w.Body.String())
		}
	})

	t.Run("bind error", func(t *testing.T) {
		type ReqInt struct {
			ID int `path:"id"`
		}
		handler := func(ctx context.Context, req ReqInt) (Resp, error) {
			return Resp{}, nil
		}

		wrapped := WrapCtxReqRespErr(handler)

		r := httptest.NewRequest("GET", "/pets/notanint", nil)
		r.SetPathValue("id", "notanint")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("handler error", func(t *testing.T) {
		handler := func(ctx context.Context, req Req) (Resp, error) {
			return Resp{}, errors.New("db connection failed")
		}

		wrapped := WrapCtxReqRespErr(handler)

		r := httptest.NewRequest("GET", "/pets/123", nil)
		r.SetPathValue("id", "123")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("with JSON body", func(t *testing.T) {
		type ReqWithBody struct {
			ID   string `path:"id"`
			Name string `json:"name"`
		}
		handler := func(ctx context.Context, req ReqWithBody) (Resp, error) {
			return Resp{Name: req.Name + "-" + req.ID}, nil
		}

		wrapped := WrapCtxReqRespErr(handler)

		body := strings.NewReader(`{"name":"Fluffy"}`)
		r := httptest.NewRequest("PUT", "/pets/123", body)
		r.SetPathValue("id", "123")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), `"name":"Fluffy-123"`) {
			t.Errorf("body = %q, want to contain 'name:Fluffy-123'", w.Body.String())
		}
	})
}

func TestWrapCtxReqErr(t *testing.T) {
	type Req struct {
		ID string `path:"id"`
	}

	t.Run("success → 204", func(t *testing.T) {
		handler := func(ctx context.Context, req Req) error {
			return nil
		}

		wrapped := WrapCtxReqErr(handler)

		r := httptest.NewRequest("DELETE", "/pets/123", nil)
		r.SetPathValue("id", "123")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if body := w.Body.String(); body != "" {
			t.Errorf("body = %q, want empty", body)
		}
	})

	t.Run("bind error", func(t *testing.T) {
		type ReqInt struct {
			ID int `path:"id"`
		}
		handler := func(ctx context.Context, req ReqInt) error {
			return nil
		}

		wrapped := WrapCtxReqErr(handler)

		r := httptest.NewRequest("DELETE", "/pets/notanint", nil)
		r.SetPathValue("id", "notanint")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("handler error", func(t *testing.T) {
		handler := func(ctx context.Context, req Req) error {
			return errors.New("deletion failed")
		}

		wrapped := WrapCtxReqErr(handler)

		r := httptest.NewRequest("DELETE", "/pets/123", nil)
		r.SetPathValue("id", "123")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestWrapCtxRespErr(t *testing.T) {
	type Resp struct {
		Items []string `json:"items"`
	}

	t.Run("success", func(t *testing.T) {
		handler := func(ctx context.Context) (Resp, error) {
			return Resp{Items: []string{"a", "b"}}, nil
		}

		wrapped := WrapCtxRespErr(handler)

		r := httptest.NewRequest("GET", "/items", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), `"items":["a","b"]`) {
			t.Errorf("body = %q, want to contain items array", w.Body.String())
		}
	})

	t.Run("handler error", func(t *testing.T) {
		handler := func(ctx context.Context) (Resp, error) {
			return Resp{}, errors.New("query failed")
		}

		wrapped := WrapCtxRespErr(handler)

		r := httptest.NewRequest("GET", "/items", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("nil slice response", func(t *testing.T) {
		handler := func(ctx context.Context) (Resp, error) {
			return Resp{Items: nil}, nil
		}

		wrapped := WrapCtxRespErr(handler)

		r := httptest.NewRequest("GET", "/items", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}

func TestWrapCtxErr(t *testing.T) {
	t.Run("success → 204", func(t *testing.T) {
		handler := func(ctx context.Context) error {
			return nil
		}

		wrapped := WrapCtxErr(handler)

		r := httptest.NewRequest("POST", "/ping", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if body := w.Body.String(); body != "" {
			t.Errorf("body = %q, want empty", body)
		}
	})

	t.Run("handler error", func(t *testing.T) {
		handler := func(ctx context.Context) error {
			return errors.New("ping failed")
		}

		wrapped := WrapCtxErr(handler)

		r := httptest.NewRequest("POST", "/ping", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestWrap_ContextPropagation(t *testing.T) {
	t.Run("WrapCtxErr passes request context", func(t *testing.T) {
		type ctxKey string
		var capturedCtx context.Context
		handler := func(ctx context.Context) error {
			capturedCtx = ctx
			return nil
		}

		wrapped := WrapCtxErr(handler)

		ctx := context.WithValue(context.Background(), ctxKey("key"), "value")
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if capturedCtx.Value(ctxKey("key")) != "value" {
			t.Error("context value not propagated")
		}
	})

	t.Run("WrapCtxRespErr passes request context", func(t *testing.T) {
		type ctxKey string
		type Resp struct{}
		var capturedCtx context.Context
		handler := func(ctx context.Context) (Resp, error) {
			capturedCtx = ctx
			return Resp{}, nil
		}

		wrapped := WrapCtxRespErr(handler)

		ctx := context.WithValue(context.Background(), ctxKey("key"), "testval")
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if capturedCtx.Value(ctxKey("key")) != "testval" {
			t.Error("context value not propagated")
		}
	})

	t.Run("WrapCtxReqErr passes request context", func(t *testing.T) {
		type ctxKey string
		type Req struct{}
		var capturedCtx context.Context
		handler := func(ctx context.Context, req Req) error {
			capturedCtx = ctx
			return nil
		}

		wrapped := WrapCtxReqErr(handler)

		ctx := context.WithValue(context.Background(), ctxKey("key"), "reqval")
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if capturedCtx.Value(ctxKey("key")) != "reqval" {
			t.Error("context value not propagated")
		}
	})

	t.Run("WrapCtxReqRespErr passes request context", func(t *testing.T) {
		type ctxKey string
		type Req struct{}
		type Resp struct{}
		var capturedCtx context.Context
		handler := func(ctx context.Context, req Req) (Resp, error) {
			capturedCtx = ctx
			return Resp{}, nil
		}

		wrapped := WrapCtxReqRespErr(handler)

		ctx := context.WithValue(context.Background(), ctxKey("key"), "fullval")
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if capturedCtx.Value(ctxKey("key")) != "fullval" {
			t.Error("context value not propagated")
		}
	})
}

func TestWrap_ContentType(t *testing.T) {
	t.Run("success response has JSON content type", func(t *testing.T) {
		type Resp struct {
			OK bool `json:"ok"`
		}
		handler := func(ctx context.Context) (Resp, error) {
			return Resp{OK: true}, nil
		}

		wrapped := WrapCtxRespErr(handler)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
	})

	t.Run("error response has JSON content type", func(t *testing.T) {
		handler := func(ctx context.Context) error {
			return errors.New("failed")
		}

		wrapped := WrapCtxErr(handler)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, r)

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
	})
}
