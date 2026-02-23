package httpserver

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
)

// mockQuerier is a simple mock that implements Querier for testing.
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

func TestWithQuerier_GetQuerier_RoundTrip(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{}

	ctx = WithQuerier(ctx, q)
	got := GetQuerier(ctx)

	if got != q {
		t.Errorf("GetQuerier() returned different Querier; want same instance")
	}
}

func TestGetQuerier_PanicsWithoutQuerier(t *testing.T) {
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("GetQuerier() did not panic when no Querier in context")
		}
	}()

	GetQuerier(ctx)
}

func TestWithQuerier_OverwritesPrevious(t *testing.T) {
	ctx := context.Background()
	q1 := &mockQuerier{}
	q2 := &mockQuerier{}

	ctx = WithQuerier(ctx, q1)
	ctx = WithQuerier(ctx, q2)
	got := GetQuerier(ctx)

	if got != q2 {
		t.Errorf("GetQuerier() returned first Querier; want second")
	}
}

func TestWithQuerier_PreservesOtherContextValues(t *testing.T) {
	type otherKey struct{}
	ctx := context.Background()
	ctx = context.WithValue(ctx, otherKey{}, "other value")

	q := &mockQuerier{}
	ctx = WithQuerier(ctx, q)

	// Verify Querier is accessible
	got := GetQuerier(ctx)
	if got != q {
		t.Errorf("GetQuerier() returned wrong Querier")
	}

	// Verify other value is still accessible
	if v := ctx.Value(otherKey{}); v != "other value" {
		t.Errorf("other context value lost; got %v, want %q", v, "other value")
	}
}

// Cookie tests

func TestWithCookieOps_SetCookie_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx, ops := WithCookieOps(ctx)

	cookie := &http.Cookie{Name: "session", Value: "abc123"}
	SetCookie(ctx, cookie)

	if len(*ops) != 1 {
		t.Fatalf("expected 1 cookie op, got %d", len(*ops))
	}
	if (*ops)[0].Cookie != cookie {
		t.Errorf("cookie op does not contain expected cookie")
	}
}

func TestSetCookie_MultipleCookies(t *testing.T) {
	ctx := context.Background()
	ctx, ops := WithCookieOps(ctx)

	cookie1 := &http.Cookie{Name: "session", Value: "abc"}
	cookie2 := &http.Cookie{Name: "preferences", Value: "xyz"}

	SetCookie(ctx, cookie1)
	SetCookie(ctx, cookie2)

	if len(*ops) != 2 {
		t.Fatalf("expected 2 cookie ops, got %d", len(*ops))
	}
	if (*ops)[0].Cookie != cookie1 {
		t.Errorf("first cookie op does not contain expected cookie")
	}
	if (*ops)[1].Cookie != cookie2 {
		t.Errorf("second cookie op does not contain expected cookie")
	}
}

func TestSetCookie_PanicsWithoutCookieOps(t *testing.T) {
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("SetCookie() did not panic when no cookie ops in context")
		}
	}()

	SetCookie(ctx, &http.Cookie{Name: "test", Value: "value"})
}

func TestWithRequestCookies_GetCookie_RoundTrip(t *testing.T) {
	ctx := context.Background()
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "preferences", Value: "dark-mode"},
	}
	ctx = WithRequestCookies(ctx, cookies)

	// Get existing cookie
	got, err := GetCookie(ctx, "session")
	if err != nil {
		t.Fatalf("GetCookie() error = %v", err)
	}
	if got.Value != "abc123" {
		t.Errorf("GetCookie() value = %q, want %q", got.Value, "abc123")
	}

	// Get another existing cookie
	got, err = GetCookie(ctx, "preferences")
	if err != nil {
		t.Fatalf("GetCookie() error = %v", err)
	}
	if got.Value != "dark-mode" {
		t.Errorf("GetCookie() value = %q, want %q", got.Value, "dark-mode")
	}
}

func TestGetCookie_NotFound(t *testing.T) {
	ctx := context.Background()
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
	}
	ctx = WithRequestCookies(ctx, cookies)

	_, err := GetCookie(ctx, "nonexistent")
	if err != http.ErrNoCookie {
		t.Errorf("GetCookie() error = %v, want http.ErrNoCookie", err)
	}
}

func TestGetCookie_NoCookiesInContext(t *testing.T) {
	ctx := context.Background()

	_, err := GetCookie(ctx, "session")
	if err != http.ErrNoCookie {
		t.Errorf("GetCookie() error = %v, want http.ErrNoCookie", err)
	}
}

func TestGetCookie_EmptyCookies(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestCookies(ctx, []*http.Cookie{})

	_, err := GetCookie(ctx, "session")
	if err != http.ErrNoCookie {
		t.Errorf("GetCookie() error = %v, want http.ErrNoCookie", err)
	}
}

// CookieWriter tests

func TestCookieWriter_WriteHeader_AppliesCookies(t *testing.T) {
	rec := &http.Response{}
	_ = rec // suppress unused warning for clarity
	w := &fakeResponseWriter{header: http.Header{}}
	ops := &[]CookieOp{
		{Cookie: &http.Cookie{Name: "session", Value: "abc123"}},
	}

	cw := NewCookieWriter(w, ops)
	cw.WriteHeader(http.StatusOK)

	setCookie := w.header.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header after WriteHeader, got none")
	}
	if w.statusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.statusCode)
	}
}

func TestCookieWriter_Write_AppliesCookies(t *testing.T) {
	w := &fakeResponseWriter{header: http.Header{}}
	ops := &[]CookieOp{
		{Cookie: &http.Cookie{Name: "token", Value: "xyz"}},
	}

	cw := NewCookieWriter(w, ops)
	n, err := cw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Write() returned %d, want 5", n)
	}

	setCookie := w.header.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header after Write, got none")
	}
}

func TestCookieWriter_Flush_AppliesCookiesWithoutWriting(t *testing.T) {
	w := &fakeResponseWriter{header: http.Header{}}
	ops := &[]CookieOp{
		{Cookie: &http.Cookie{Name: "pref", Value: "dark"}},
	}

	cw := NewCookieWriter(w, ops)
	cw.Flush()

	setCookie := w.header.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header after Flush, got none")
	}
	// Flush alone should NOT call WriteHeader or Write on the underlying writer
	if w.statusCode != 0 {
		t.Errorf("Flush should not write status; got %d", w.statusCode)
	}
	if len(w.body) != 0 {
		t.Errorf("Flush should not write body; got %q", w.body)
	}
}

func TestCookieWriter_Flush_IsIdempotent(t *testing.T) {
	w := &fakeResponseWriter{header: http.Header{}}
	ops := &[]CookieOp{
		{Cookie: &http.Cookie{Name: "a", Value: "1"}},
	}

	cw := NewCookieWriter(w, ops)
	cw.Flush()
	cw.Flush() // second call should be a no-op

	values := w.header.Values("Set-Cookie")
	if len(values) != 1 {
		t.Errorf("expected 1 Set-Cookie header, got %d: %v", len(values), values)
	}
}

func TestCookieWriter_MultipleCookies(t *testing.T) {
	w := &fakeResponseWriter{header: http.Header{}}
	ops := &[]CookieOp{
		{Cookie: &http.Cookie{Name: "a", Value: "1"}},
		{Cookie: &http.Cookie{Name: "b", Value: "2"}},
	}

	cw := NewCookieWriter(w, ops)
	cw.WriteHeader(http.StatusCreated)

	values := w.header.Values("Set-Cookie")
	if len(values) != 2 {
		t.Errorf("expected 2 Set-Cookie headers, got %d: %v", len(values), values)
	}
	if w.statusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.statusCode)
	}
}

func TestCookieWriter_NoCookies(t *testing.T) {
	w := &fakeResponseWriter{header: http.Header{}}
	ops := &[]CookieOp{}

	cw := NewCookieWriter(w, ops)
	cw.WriteHeader(http.StatusOK)

	values := w.header.Values("Set-Cookie")
	if len(values) != 0 {
		t.Errorf("expected 0 Set-Cookie headers, got %d", len(values))
	}
}

// fakeResponseWriter is a minimal http.ResponseWriter for testing.
type fakeResponseWriter struct {
	header     http.Header
	statusCode int
	body       []byte
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.header
}

func (f *fakeResponseWriter) WriteHeader(code int) {
	f.statusCode = code
}

func (f *fakeResponseWriter) Write(b []byte) (int, error) {
	f.body = append(f.body, b...)
	return len(b), nil
}
