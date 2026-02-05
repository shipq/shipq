package httpserver

import (
	"context"
	"database/sql"
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
