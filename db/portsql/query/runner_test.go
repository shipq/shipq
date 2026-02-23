package query

import (
	"context"
	"database/sql"
	"testing"
)

func TestDialectString(t *testing.T) {
	tests := []struct {
		dialect Dialect
		want    string
	}{
		{Postgres, "postgres"},
		{MySQL, "mysql"},
		{SQLite, "sqlite"},
		{Dialect(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.dialect.String(); got != tt.want {
			t.Errorf("Dialect(%d).String() = %q, want %q", tt.dialect, got, tt.want)
		}
	}
}

// mockQuerier implements Querier for testing without a real database
type mockQuerier struct {
	execCalled     bool
	queryCalled    bool
	queryRowCalled bool
}

func (m *mockQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.execCalled = true
	return nil, nil
}

func (m *mockQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	m.queryCalled = true
	return nil, nil
}

func (m *mockQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	m.queryRowCalled = true
	return nil
}

func TestNewQueryRunner(t *testing.T) {
	mock := &mockQuerier{}
	runner := NewQueryRunner(mock, Postgres)

	if runner.Dialect() != Postgres {
		t.Errorf("expected dialect Postgres, got %v", runner.Dialect())
	}

	if runner.DB() != mock {
		t.Error("expected DB() to return the mock querier")
	}
}

func TestQueryRunnerDialects(t *testing.T) {
	mock := &mockQuerier{}

	tests := []Dialect{Postgres, MySQL, SQLite}
	for _, dialect := range tests {
		runner := NewQueryRunner(mock, dialect)
		if runner.Dialect() != dialect {
			t.Errorf("expected dialect %v, got %v", dialect, runner.Dialect())
		}
	}
}

func TestQueryRunnerWithDB(t *testing.T) {
	mock1 := &mockQuerier{}
	mock2 := &mockQuerier{}

	runner := NewQueryRunner(mock1, MySQL)

	// Create a new runner with a different DB
	runner2 := runner.WithDB(mock2)

	// Original runner should still have mock1
	if runner.DB() != mock1 {
		t.Error("original runner should still have mock1")
	}

	// New runner should have mock2
	if runner2.DB() != mock2 {
		t.Error("new runner should have mock2")
	}

	// Dialect should be preserved
	if runner2.Dialect() != MySQL {
		t.Errorf("expected dialect MySQL, got %v", runner2.Dialect())
	}
}

// TestQuerierInterface verifies that the mockQuerier implements Querier
func TestQuerierInterface(t *testing.T) {
	var _ Querier = (*mockQuerier)(nil)
}
