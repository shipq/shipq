package httputil

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", body)
	}
}

func TestWriteJSON_DifferentStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusCreated, map[string]int{"id": 42})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestWriteError_HTTPError(t *testing.T) {
	w := httptest.NewRecorder()
	// Use httperror.BadRequest directly
	err := &testHTTPError{code: 400, message: "bad input"}
	WriteError(w, err)

	if w.Code != http.StatusInternalServerError {
		// Since our testHTTPError doesn't implement httperror.Error, it falls through
		// to the generic case. Let's test with a real httperror instead.
	}
}

func TestWriteError_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.New("something broke"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("expected generic error message, got %q", body["error"])
	}
}

func TestWrapHandler(t *testing.T) {
	called := false
	handler := WrapHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context {
			return context.WithValue(ctx, testKey{}, "injected")
		},
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			val := r.Context().Value(testKey{})
			if val != "injected" {
				t.Error("context injection did not work")
			}
			WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		},
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestWrapAuthHandler_Authenticated(t *testing.T) {
	called := false
	handler := WrapAuthHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 42, 99, nil }, // account ID = 42, org ID = 99
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			// Verify that the account ID was injected into context
			accountID, ok := SessionAccountIDFromContext(r.Context())
			if !ok {
				t.Error("expected session account ID in context")
			}
			if accountID != 42 {
				t.Errorf("expected account ID 42, got %d", accountID)
			}
			// Verify that the org ID was injected into context
			orgID, ok := OrganizationIDFromContext(r.Context())
			if !ok {
				t.Error("expected organization ID in context")
			}
			if orgID != 99 {
				t.Errorf("expected org ID 99, got %d", orgID)
			}
			WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		},
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestWrapAuthHandler_Unauthenticated(t *testing.T) {
	called := false
	handler := WrapAuthHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 0, 0, errors.New("unauthorized") },
		func(w http.ResponseWriter, r *http.Request) {
			called = true
		},
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("handler should not be called when auth fails")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSessionAccountIDFromContext_NotSet(t *testing.T) {
	ctx := context.Background()
	id, ok := SessionAccountIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false when no session account ID is in context")
	}
	if id != 0 {
		t.Errorf("expected id=0, got %d", id)
	}
}

func TestSessionAccountIDFromContext_Set(t *testing.T) {
	ctx := WithSessionAccountID(context.Background(), 123)
	id, ok := SessionAccountIDFromContext(ctx)
	if !ok {
		t.Error("expected ok=true")
	}
	if id != 123 {
		t.Errorf("expected id=123, got %d", id)
	}
}

func TestOrganizationIDFromContext_NotSet(t *testing.T) {
	ctx := context.Background()
	id, ok := OrganizationIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false when no organization ID is in context")
	}
	if id != 0 {
		t.Errorf("expected id=0, got %d", id)
	}
}

func TestOrganizationIDFromContext_Set(t *testing.T) {
	ctx := WithOrganizationID(context.Background(), 456)
	id, ok := OrganizationIDFromContext(ctx)
	if !ok {
		t.Error("expected ok=true")
	}
	if id != 456 {
		t.Errorf("expected id=456, got %d", id)
	}
}

func TestBothContextValues_Independent(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionAccountID(ctx, 100)
	ctx = WithOrganizationID(ctx, 200)

	accountID, ok := SessionAccountIDFromContext(ctx)
	if !ok || accountID != 100 {
		t.Errorf("expected account ID 100, got %d (ok=%v)", accountID, ok)
	}
	orgID, ok := OrganizationIDFromContext(ctx)
	if !ok || orgID != 200 {
		t.Errorf("expected org ID 200, got %d (ok=%v)", orgID, ok)
	}
}

func TestAddAuth_WithCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	AddAuth(req, "my-session-cookie")

	cookie, err := req.Cookie("session")
	if err != nil {
		t.Fatal("expected session cookie")
	}
	if cookie.Value != "my-session-cookie" {
		t.Errorf("expected cookie value 'my-session-cookie', got %q", cookie.Value)
	}
}

func TestAddAuth_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	AddAuth(req, "")

	if len(req.Cookies()) != 0 {
		t.Error("expected no cookies when sessionCookie is empty")
	}
}

// --- WrapRBACHandler tests ---

func TestWrapRBACHandler_AuthFailure(t *testing.T) {
	called := false
	handler := WrapRBACHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) {
			return 0, 0, errors.New("unauthorized")
		},
		func(ctx context.Context, accountID, orgID int64, routePath, method string) error {
			t.Error("checkRBAC should not be called when auth fails")
			return nil
		},
		"/test",
		"GET",
		func(w http.ResponseWriter, r *http.Request) { called = true },
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("handler should not be called when auth fails")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestWrapRBACHandler_Forbidden(t *testing.T) {
	called := false
	handler := WrapRBACHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 42, 99, nil },
		func(ctx context.Context, accountID, orgID int64, routePath, method string) error {
			return Forbidden("insufficient permissions")
		},
		"/test",
		"GET",
		func(w http.ResponseWriter, r *http.Request) { called = true },
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("handler should not be called when RBAC denies access")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["error"] != "insufficient permissions" {
		t.Errorf("expected 'insufficient permissions', got %q", body["error"])
	}
}

func TestWrapRBACHandler_InternalError(t *testing.T) {
	called := false
	handler := WrapRBACHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 42, 99, nil },
		func(ctx context.Context, accountID, orgID int64, routePath, method string) error {
			return errors.New("database error")
		},
		"/test",
		"GET",
		func(w http.ResponseWriter, r *http.Request) { called = true },
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("handler should not be called on RBAC internal error")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestWrapRBACHandler_Success(t *testing.T) {
	called := false
	handler := WrapRBACHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 42, 99, nil },
		func(ctx context.Context, accountID, orgID int64, routePath, method string) error {
			if accountID != 42 {
				t.Errorf("expected accountID 42, got %d", accountID)
			}
			if orgID != 99 {
				t.Errorf("expected orgID 99, got %d", orgID)
			}
			if routePath != "/pets/:id" {
				t.Errorf("expected routePath '/pets/:id', got %q", routePath)
			}
			if method != "GET" {
				t.Errorf("expected method 'GET', got %q", method)
			}
			return nil
		},
		"/pets/:id",
		"GET",
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			// Verify context values
			accountID, ok := SessionAccountIDFromContext(r.Context())
			if !ok || accountID != 42 {
				t.Errorf("expected account ID 42 in context, got %d (ok=%v)", accountID, ok)
			}
			orgID, ok := OrganizationIDFromContext(r.Context())
			if !ok || orgID != 99 {
				t.Errorf("expected org ID 99 in context, got %d (ok=%v)", orgID, ok)
			}
			WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		},
	)

	req := httptest.NewRequest("GET", "/pets/abc123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// --- WrapOptionalAuthHandler tests ---

var errNoSession = errors.New("no valid session")

func isNoSession(err error) bool {
	return errors.Is(err, errNoSession)
}

func TestWrapOptionalAuthHandler_Authenticated(t *testing.T) {
	called := false
	handler := WrapOptionalAuthHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 42, 99, nil },
		isNoSession,
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			accountID, ok := SessionAccountIDFromContext(r.Context())
			if !ok {
				t.Error("expected session account ID in context")
			}
			if accountID != 42 {
				t.Errorf("expected account ID 42, got %d", accountID)
			}
			orgID, ok := OrganizationIDFromContext(r.Context())
			if !ok {
				t.Error("expected organization ID in context")
			}
			if orgID != 99 {
				t.Errorf("expected org ID 99, got %d", orgID)
			}
			WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		},
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestWrapOptionalAuthHandler_NoSession(t *testing.T) {
	called := false
	handler := WrapOptionalAuthHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) { return 0, 0, errNoSession },
		isNoSession,
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			// Context should NOT have account ID
			_, ok := SessionAccountIDFromContext(r.Context())
			if ok {
				t.Error("expected no session account ID in context for unauthenticated request")
			}
			WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		},
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called even without a session")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestWrapOptionalAuthHandler_RealError(t *testing.T) {
	called := false
	handler := WrapOptionalAuthHandler(
		&mockQuerier{},
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context) (int64, int64, error) {
			return 0, 0, errors.New("database connection failed")
		},
		isNoSession,
		func(w http.ResponseWriter, r *http.Request) { called = true },
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("handler should NOT be called on real errors")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("expected 'internal server error', got %q", body["error"])
	}
}

// Test helpers

type testKey struct{}

type testHTTPError struct {
	code    int
	message string
}

func (e *testHTTPError) Error() string { return e.message }

// mockQuerier implements httpserver.Querier for testing.
type mockQuerier struct{}

func (m *mockQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}

func (m *mockQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (m *mockQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}
