package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateVerifyEmailHandler_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "verify_email.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated verify_email.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateVerifyEmailHandler_ContainsVerifyEmail(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"VerifyEmail",
		"VerifyEmailRequest",
		"HashToken",
		"FindEmailVerificationToken",
		"VerifyAccount",
		"MarkEmailVerificationTokenUsed",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateVerifyEmailHandler_ContainsRequestFields(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"type VerifyEmailRequest struct",
		"Token string",
		`json:"token"`,
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateVerifyEmailHandler_ValidatesEmptyToken(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"token is required"`) {
		t.Errorf("expected output to validate empty token, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateVerifyEmailHandler_RejectsInvalidToken(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"invalid or expired verification token"`) {
		t.Errorf("expected output to reject invalid/expired tokens, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateVerifyEmailHandler_UsesSHA256Lookup(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "HashToken(req.Token)") {
		t.Errorf("expected output to use HashToken for SHA-256 lookup, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateVerifyEmailHandler_ImportsRequired(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateVerifyEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateVerifyEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"example.com/myapp/shipq/lib/httperror",
		"example.com/myapp/shipq/queries",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to import %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

// --- ResendVerification tests ---

func TestGenerateResendVerificationHandler_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "resend_verification.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated resend_verification.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateResendVerificationHandler_ContainsResendVerification(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"ResendVerification",
		"ResendVerificationRequest",
		"FindAccountByEmail",
		"InsertEmailVerificationToken",
		"HashToken",
		"generateSecureToken",
		"SendEmail",
		"SendEmailParams",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateResendVerificationHandler_IsTimingSafe(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
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

func TestGenerateResendVerificationHandler_SkipsAlreadyVerified(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	// Should check if account is already verified and silently succeed
	if !strings.Contains(codeStr, "account.Verified") {
		t.Errorf("expected output to check Verified status, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResendVerificationHandler_ContainsTokenRedaction(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	// SensitiveTokens should include the raw token for redaction
	if !strings.Contains(codeStr, "SensitiveTokens: []string{rawToken}") {
		t.Errorf("expected output to redact rawToken in SensitiveTokens, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResendVerificationHandler_ContainsVerificationLink(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"APP_URL",
		"/verify-email?token=",
		"Verify Your Email",
		"24 hours",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateResendVerificationHandler_Contains24HourExpiry(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "24 * time.Hour") {
		t.Errorf("expected output to contain 24-hour expiry, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateResendVerificationHandler_ContainsRequestField(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"type ResendVerificationRequest struct",
		"Email string",
		`json:"email"`,
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateResendVerificationHandler_ImportsRequired(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateResendVerificationHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateResendVerificationHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"example.com/myapp/config",
		"example.com/myapp/shipq/lib/httperror",
		"example.com/myapp/shipq/lib/nanoid",
		"example.com/myapp/shipq/queries",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to import %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}
