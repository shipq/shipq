package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateForgotPasswordHandler_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateForgotPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateForgotPasswordHandler failed: %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "forgot_password.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated forgot_password.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateForgotPasswordHandler_ContainsForgotPassword(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateForgotPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateForgotPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"ForgotPassword",
		"ForgotPasswordRequest",
		"HashToken",
		"InvalidatePasswordResetTokens",
		"InsertPasswordResetToken",
		"SendEmail",
		"SendEmailParams",
		"generateSecureToken",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateForgotPasswordHandler_IsTimingSafe(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateForgotPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateForgotPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	// The handler should return success even when account is not found
	if !strings.Contains(codeStr, "account == nil") {
		t.Errorf("expected output to check for nil account, but it didn't.\nOutput:\n%s", codeStr)
	}

	// Should return struct{}{} for not-found case (timing-safe)
	if !strings.Contains(codeStr, "return &struct{}{}, nil") {
		t.Errorf("expected output to return empty success for not-found case, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateForgotPasswordHandler_ContainsTokenRedaction(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateForgotPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateForgotPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	// SensitiveTokens should include the raw token for redaction
	if !strings.Contains(codeStr, "SensitiveTokens: []string{rawToken}") {
		t.Errorf("expected output to redact rawToken in SensitiveTokens, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateForgotPasswordHandler_ContainsResetLink(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateForgotPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateForgotPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"APP_URL",
		"/reset-password?token=",
		"Reset Your Password",
		"1 hour",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateForgotPasswordHandler_ContainsOneHourExpiry(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateForgotPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateForgotPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "1 * time.Hour") {
		t.Errorf("expected output to contain 1-hour expiry, but it didn't.\nOutput:\n%s", codeStr)
	}
}
