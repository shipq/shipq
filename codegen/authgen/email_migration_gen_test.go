package authgen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSentEmailsMigration_IsValidGo(t *testing.T) {
	code := GenerateSentEmailsMigration("20250701120000", "example.com/myapp")

	_, err := parser.ParseFile(token.NewFileSet(), "sent_emails.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated sent_emails migration is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateSentEmailsMigration_ContainsExpectedColumns(t *testing.T) {
	code := GenerateSentEmailsMigration("20250701120000", "example.com/myapp")
	codeStr := string(code)

	expected := []string{
		"sent_emails",
		`"to_email"`,
		`"subject"`,
		`"html_body"`,
		`"status"`,
		`"error_message"`,
		"Nullable()",
		"Text(",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateSentEmailsMigration_HasCorrectTimestamp(t *testing.T) {
	code := GenerateSentEmailsMigration("20250701120000", "example.com/myapp")
	codeStr := string(code)

	if !strings.Contains(codeStr, "Migrate_20250701120000_sent_emails") {
		t.Errorf("expected migration function name to contain timestamp, got:\n%s", codeStr)
	}
}

func TestGenerateAccountsVerifiedMigration_IsValidGo(t *testing.T) {
	code := GenerateAccountsVerifiedMigration("20250701120001", "example.com/myapp")

	_, err := parser.ParseFile(token.NewFileSet(), "accounts_verified.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated accounts_verified migration is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateAccountsVerifiedMigration_ContainsExpectedColumns(t *testing.T) {
	code := GenerateAccountsVerifiedMigration("20250701120001", "example.com/myapp")
	codeStr := string(code)

	expected := []string{
		`UpdateTable("accounts"`,
		`AddBoolean("verified")`,
		`Default(false)`,
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateAccountsVerifiedMigration_HasCorrectTimestamp(t *testing.T) {
	code := GenerateAccountsVerifiedMigration("20250701120001", "example.com/myapp")
	codeStr := string(code)

	if !strings.Contains(codeStr, "Migrate_20250701120001_accounts_verified") {
		t.Errorf("expected migration function name to contain timestamp, got:\n%s", codeStr)
	}
}

func TestGeneratePasswordResetTokensMigration_IsValidGo(t *testing.T) {
	code := GeneratePasswordResetTokensMigration("20250701120002", "example.com/myapp")

	_, err := parser.ParseFile(token.NewFileSet(), "password_reset_tokens.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated password_reset_tokens migration is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGeneratePasswordResetTokensMigration_ContainsExpectedColumns(t *testing.T) {
	code := GeneratePasswordResetTokensMigration("20250701120002", "example.com/myapp")
	codeStr := string(code)

	expected := []string{
		"password_reset_tokens",
		`"account_id"`,
		`"token_hash"`,
		`"expires_at"`,
		`"used"`,
		"References(accountsRef)",
		"Default(false)",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGeneratePasswordResetTokensMigration_HasCorrectTimestamp(t *testing.T) {
	code := GeneratePasswordResetTokensMigration("20250701120002", "example.com/myapp")
	codeStr := string(code)

	if !strings.Contains(codeStr, "Migrate_20250701120002_password_reset_tokens") {
		t.Errorf("expected migration function name to contain timestamp, got:\n%s", codeStr)
	}
}

func TestGenerateEmailVerificationTokensMigration_IsValidGo(t *testing.T) {
	code := GenerateEmailVerificationTokensMigration("20250701120003", "example.com/myapp")

	_, err := parser.ParseFile(token.NewFileSet(), "email_verification_tokens.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated email_verification_tokens migration is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateEmailVerificationTokensMigration_ContainsExpectedColumns(t *testing.T) {
	code := GenerateEmailVerificationTokensMigration("20250701120003", "example.com/myapp")
	codeStr := string(code)

	expected := []string{
		"email_verification_tokens",
		`"account_id"`,
		`"token_hash"`,
		`"expires_at"`,
		`"used"`,
		"References(accountsRef)",
		"Default(false)",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateEmailVerificationTokensMigration_HasCorrectTimestamp(t *testing.T) {
	code := GenerateEmailVerificationTokensMigration("20250701120003", "example.com/myapp")
	codeStr := string(code)

	if !strings.Contains(codeStr, "Migrate_20250701120003_email_verification_tokens") {
		t.Errorf("expected migration function name to contain timestamp, got:\n%s", codeStr)
	}
}

func TestEmailMigrationsExist_ReturnsFalse_WhenEmpty(t *testing.T) {
	dir := t.TempDir()

	if EmailMigrationsExist(dir) {
		t.Error("expected EmailMigrationsExist to return false for empty directory")
	}
}

func TestEmailMigrationsExist_ReturnsFalse_WhenNonexistentDir(t *testing.T) {
	if EmailMigrationsExist("/tmp/nonexistent-dir-email-test-12345") {
		t.Error("expected EmailMigrationsExist to return false for nonexistent directory")
	}
}

func TestEmailMigrationsExist_ReturnsTrue_WhenSentEmailsPresent(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "20250701120000_sent_emails.go")
	if err := os.WriteFile(filePath, []byte("package migrations\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !EmailMigrationsExist(dir) {
		t.Error("expected EmailMigrationsExist to return true when _sent_emails.go file exists")
	}
}

func TestEmailMigrationsExist_ReturnsTrue_WhenVerifiedPresent(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "20250701120001_accounts_verified.go")
	if err := os.WriteFile(filePath, []byte("package migrations\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !EmailMigrationsExist(dir) {
		t.Error("expected EmailMigrationsExist to return true when _accounts_verified.go file exists")
	}
}

func TestEmailMigrationsExist_ReturnsFalse_WhenOtherMigrationsPresent(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "20250701120000_accounts.go")
	if err := os.WriteFile(filePath, []byte("package migrations\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if EmailMigrationsExist(dir) {
		t.Error("expected EmailMigrationsExist to return false when only non-email migration files exist")
	}
}

func TestEmailMigrationsExist_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()

	subdir := filepath.Join(dir, "20250701120000_sent_emails.go")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	if EmailMigrationsExist(dir) {
		t.Error("expected EmailMigrationsExist to return false when matching entry is a directory")
	}
}
