package dbops_test

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/internal/dbops"
)

func TestMySQLURLToDSN(t *testing.T) {
	t.Run("basic URL without query params adds defaults", func(t *testing.T) {
		dsn, err := dbops.MySQLURLToDSN("mysql://root@localhost:3306/mydb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(dsn, "root@tcp(localhost:3306)/mydb?") {
			t.Fatalf("unexpected DSN prefix: %s", dsn)
		}
		if !strings.Contains(dsn, "loc=Local") {
			t.Errorf("DSN missing loc=Local: %s", dsn)
		}
		if !strings.Contains(dsn, "parseTime=true") {
			t.Errorf("DSN missing parseTime=true: %s", dsn)
		}
	})

	t.Run("preserves user:password", func(t *testing.T) {
		dsn, err := dbops.MySQLURLToDSN("mysql://admin:secret@localhost:3306/mydb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(dsn, "admin:secret@tcp(localhost:3306)/mydb?") {
			t.Errorf("DSN should start with admin:secret@tcp(...): %s", dsn)
		}
	})

	t.Run("preserves explicit query params", func(t *testing.T) {
		dsn, err := dbops.MySQLURLToDSN("mysql://root@localhost:3306/mydb?charset=utf8mb4&timeout=5s")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(dsn, "charset=utf8mb4") {
			t.Errorf("DSN missing charset param: %s", dsn)
		}
		if !strings.Contains(dsn, "timeout=5s") {
			t.Errorf("DSN missing timeout param: %s", dsn)
		}
		if !strings.Contains(dsn, "loc=Local") {
			t.Errorf("DSN should still have loc=Local default: %s", dsn)
		}
		if !strings.Contains(dsn, "parseTime=true") {
			t.Errorf("DSN should still have parseTime=true default: %s", dsn)
		}
	})

	t.Run("does not override explicit loc", func(t *testing.T) {
		dsn, err := dbops.MySQLURLToDSN("mysql://root@localhost:3306/mydb?loc=UTC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(dsn, "loc=UTC") {
			t.Errorf("DSN should preserve explicit loc=UTC: %s", dsn)
		}
		if strings.Contains(dsn, "loc=Local") {
			t.Errorf("DSN should not contain loc=Local when loc=UTC was set: %s", dsn)
		}
	})

	t.Run("does not override explicit parseTime", func(t *testing.T) {
		dsn, err := dbops.MySQLURLToDSN("mysql://root@localhost:3306/mydb?parseTime=false")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(dsn, "parseTime=false") {
			t.Errorf("DSN should preserve explicit parseTime=false: %s", dsn)
		}
	})

	t.Run("handles URL without database name", func(t *testing.T) {
		dsn, err := dbops.MySQLURLToDSN("mysql://root@localhost:3306/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(dsn, "@tcp(localhost:3306)/") {
			t.Errorf("DSN missing tcp format: %s", dsn)
		}
	})

	t.Run("rejects non-mysql scheme", func(t *testing.T) {
		_, err := dbops.MySQLURLToDSN("postgres://root@localhost:5432/mydb")
		if err == nil {
			t.Fatal("expected error for non-mysql scheme")
		}
		if !strings.Contains(err.Error(), "unexpected scheme") {
			t.Errorf("error should mention unexpected scheme: %v", err)
		}
	})

	t.Run("rejects invalid URL", func(t *testing.T) {
		_, err := dbops.MySQLURLToDSN("://not-a-url")
		if err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})
}
