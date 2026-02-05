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
