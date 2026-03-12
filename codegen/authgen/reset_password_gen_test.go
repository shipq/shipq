package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateResetPasswordHandler_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "reset_password.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated reset_password.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateResetPasswordHandler_ContainsResetPassword(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"ResetPassword",
		"ResetPasswordRequest",
		"HashToken",
		"FindPasswordResetToken",
		"MarkPasswordResetTokenUsed",
		"InvalidatePasswordResetTokens",
		"UpdateAccountPassword",
		"crypto.HashPassword",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateResetPasswordHandler_ContainsRequestFields(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"Token ",
		"Password ",
		`query:"token"`,
		`json:"password"`,
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateResetPasswordHandler_ValidatesEmptyToken(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"token is required"`) {
		t.Errorf("expected output to validate empty token, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResetPasswordHandler_ValidatesEmptyPassword(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"password is required"`) {
		t.Errorf("expected output to validate empty password, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResetPasswordHandler_RejectsInvalidToken(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"invalid or expired token"`) {
		t.Errorf("expected output to reject invalid/expired tokens, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResetPasswordHandler_InvalidatesAllTokensAfterReset(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	// After resetting the password, all outstanding tokens should be invalidated
	if !strings.Contains(codeStr, "InvalidatePasswordResetTokens") {
		t.Errorf("expected output to invalidate all tokens after reset, but it didn't.\nOutput:\n%s", codeStr)
	}

	// The specific token should also be marked as used
	if !strings.Contains(codeStr, "MarkPasswordResetTokenUsed") {
		t.Errorf("expected output to mark the used token, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResetPasswordHandler_UsesSHA256Lookup(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	// Should use HashToken (SHA-256) for token lookup, not bcrypt
	if !strings.Contains(codeStr, "HashToken(req.Token)") {
		t.Errorf("expected output to use HashToken for SHA-256 lookup, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResetPasswordHandler_ImportsRequired(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResetPasswordHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"example.com/myapp/shipq/lib/crypto",
		"example.com/myapp/shipq/lib/httperror",
		"example.com/myapp/shipq/queries",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to import %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}
