package authgen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateOAuthAccountsMigration_ContainsExpectedColumns(t *testing.T) {
	code := GenerateOAuthAccountsMigration("20250622120000", "example.com/myapp")
	codeStr := string(code)

	expected := []string{
		"oauth_accounts",
		`"provider"`,
		`"provider_user_id"`,
		`"avatar_url"`,
		`"account_id"`,
		`"email"`,
		"AddUniqueIndex",
		"References(accountsRef)",
		"Nullable()",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateOAuthAccountsMigration_IsValidGo(t *testing.T) {
	code := GenerateOAuthAccountsMigration("20250622120000", "example.com/myapp")

	_, err := parser.ParseFile(token.NewFileSet(), "oauth_accounts.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_accounts migration is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthAccountsMigration_HasCorrectTimestamp(t *testing.T) {
	code := GenerateOAuthAccountsMigration("20250622120000", "example.com/myapp")
	codeStr := string(code)

	if !strings.Contains(codeStr, "Migrate_20250622120000_oauth_accounts") {
		t.Errorf("expected migration function name to contain timestamp, got:\n%s", codeStr)
	}
}

func TestOAuthMigrationsExist_ReturnsFalse_WhenEmpty(t *testing.T) {
	dir := t.TempDir()

	if OAuthMigrationsExist(dir) {
		t.Error("expected OAuthMigrationsExist to return false for empty directory")
	}
}

func TestOAuthMigrationsExist_ReturnsFalse_WhenNonexistentDir(t *testing.T) {
	if OAuthMigrationsExist("/tmp/nonexistent-dir-oauth-test-12345") {
		t.Error("expected OAuthMigrationsExist to return false for nonexistent directory")
	}
}

func TestOAuthMigrationsExist_ReturnsTrue_WhenPresent(t *testing.T) {
	dir := t.TempDir()

	// Create a file that matches the suffix
	filePath := filepath.Join(dir, "20250622120000_oauth_accounts.go")
	if err := os.WriteFile(filePath, []byte("package migrations\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !OAuthMigrationsExist(dir) {
		t.Error("expected OAuthMigrationsExist to return true when _oauth_accounts.go file exists")
	}
}

func TestOAuthMigrationsExist_ReturnsFalse_WhenOtherMigrationsPresent(t *testing.T) {
	dir := t.TempDir()

	// Create a file that does NOT match the suffix
	filePath := filepath.Join(dir, "20250622120000_accounts.go")
	if err := os.WriteFile(filePath, []byte("package migrations\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if OAuthMigrationsExist(dir) {
		t.Error("expected OAuthMigrationsExist to return false when only non-oauth migration files exist")
	}
}

func TestOAuthMigrationsExist_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a directory that matches the suffix (should be ignored)
	subdir := filepath.Join(dir, "20250622120000_oauth_accounts.go")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	if OAuthMigrationsExist(dir) {
		t.Error("expected OAuthMigrationsExist to return false when matching entry is a directory")
	}
}
