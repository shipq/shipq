package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationsExist_RequireAll(t *testing.T) {
	t.Run("returns false for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, true) {
			t.Error("expected false for empty directory")
		}
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		if MigrationsExist("/nonexistent/path/xyz", []string{"_foo.go"}, true) {
			t.Error("expected false for non-existent directory")
		}
	})

	t.Run("returns false when only some suffixes match", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260101000000_foo.go"), []byte("package m"), 0644)
		if MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, true) {
			t.Error("expected false when only 1 of 2 suffixes found")
		}
	})

	t.Run("returns true when all suffixes match", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260101000000_foo.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260101000001_bar.go"), []byte("package m"), 0644)
		if !MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, true) {
			t.Error("expected true when all suffixes found")
		}
	})

	t.Run("ignores directories with matching names", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "20260101000000_foo.go"), 0755)
		if MigrationsExist(dir, []string{"_foo.go"}, true) {
			t.Error("expected false when matching name is a directory")
		}
	})

	t.Run("returns false for empty suffix list", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "something.go"), []byte("package m"), 0644)
		// With requireAll and zero suffixes, len(found)==0 == len(suffixes)==0, so true
		if !MigrationsExist(dir, []string{}, true) {
			t.Error("expected true for empty suffix list with requireAll (vacuously true)")
		}
	})

	t.Run("requires filename to be longer than suffix", func(t *testing.T) {
		dir := t.TempDir()
		// File name exactly equals the suffix — should not match because
		// len(name) must be > len(suffix) (there must be a timestamp prefix).
		os.WriteFile(filepath.Join(dir, "_foo.go"), []byte("package m"), 0644)
		if MigrationsExist(dir, []string{"_foo.go"}, true) {
			t.Error("expected false when filename equals suffix (no timestamp prefix)")
		}
	})
}

func TestMigrationsExist_RequireAny(t *testing.T) {
	t.Run("returns false for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, false) {
			t.Error("expected false for empty directory")
		}
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		if MigrationsExist("/nonexistent/path/xyz", []string{"_foo.go"}, false) {
			t.Error("expected false for non-existent directory")
		}
	})

	t.Run("returns true when one suffix matches", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260101000000_foo.go"), []byte("package m"), 0644)
		if !MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, false) {
			t.Error("expected true when at least one suffix found")
		}
	})

	t.Run("returns true when all suffixes match", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260101000000_foo.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260101000001_bar.go"), []byte("package m"), 0644)
		if !MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, false) {
			t.Error("expected true when all suffixes found")
		}
	})

	t.Run("returns false when no suffixes match", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260101000000_baz.go"), []byte("package m"), 0644)
		if MigrationsExist(dir, []string{"_foo.go", "_bar.go"}, false) {
			t.Error("expected false when no suffixes match")
		}
	})

	t.Run("returns false for empty suffix list", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "something.go"), []byte("package m"), 0644)
		if MigrationsExist(dir, []string{}, false) {
			t.Error("expected false for empty suffix list with requireAny")
		}
	})
}

func TestAuthMigrationsExist(t *testing.T) {
	t.Run("returns false for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if AuthMigrationsExist(dir) {
			t.Error("expected false for empty directory")
		}
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		if AuthMigrationsExist("/nonexistent/path/that/does/not/exist") {
			t.Error("expected false for non-existent directory")
		}
	})

	t.Run("returns false when only some migrations exist", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260205120000_organizations.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120001_accounts.go"), []byte("package m"), 0644)
		if AuthMigrationsExist(dir) {
			t.Error("expected false when only 2 of 7 migrations exist")
		}
	})

	t.Run("returns true when all seven migrations exist", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260205120000_organizations.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120001_accounts.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120002_organization_users.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120003_sessions.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120004_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120005_account_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120006_role_actions.go"), []byte("package m"), 0644)
		if !AuthMigrationsExist(dir) {
			t.Error("expected true when all 7 migrations exist")
		}
	})

	t.Run("returns true with different timestamps", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20250101000000_organizations.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20250601000000_accounts.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260101000000_organization_users.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120003_sessions.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120004_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120005_account_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120006_role_actions.go"), []byte("package m"), 0644)
		if !AuthMigrationsExist(dir) {
			t.Error("expected true regardless of timestamps")
		}
	})

	t.Run("verifies AuthMigrationSuffixes has 7 entries", func(t *testing.T) {
		if len(AuthMigrationSuffixes) != 7 {
			t.Errorf("expected 7 auth migration suffixes, got %d", len(AuthMigrationSuffixes))
		}
	})
}
