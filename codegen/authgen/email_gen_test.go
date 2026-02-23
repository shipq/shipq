package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateEmailHandler_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "email.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated email.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateEmailHandler_ContainsSendEmail(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"SendEmail",
		"sendViaSMTP",
		"SensitiveTokens",
		"InsertSentEmail",
		"ReplaceAll",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateEmailHandler_ContainsSMTPConfig(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"SMTP_HOST",
		"SMTP_PORT",
		"SMTP_USERNAME",
		"SMTP_PASSWORD",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateEmailHandler_ContainsRedaction(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"*****"`) {
		t.Errorf("expected output to contain redaction marker \"*****\", but it didn't.\nOutput:\n%s", codeStr)
	}

	if !strings.Contains(codeStr, "ReplaceAll") {
		t.Errorf("expected output to contain ReplaceAll for token redaction, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateEmailHandler_ContainsHashToken(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"HashToken",
		"sha256.Sum256",
		"hex.EncodeToString",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateEmailHandler_ContainsGenerateSecureToken(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "generateSecureToken") {
		t.Errorf("expected output to contain generateSecureToken, but it didn't.\nOutput:\n%s", codeStr)
	}

	if !strings.Contains(codeStr, "crypto/rand") {
		t.Errorf("expected output to import crypto/rand, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateEmailHandler_ContainsSTARTTLS(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "StartTLS") {
		t.Errorf("expected output to contain StartTLS, but it didn't.\nOutput:\n%s", codeStr)
	}

	if !strings.Contains(codeStr, `"587"`) {
		t.Errorf("expected output to contain port 587 check, but it didn't.\nOutput:\n%s", codeStr)
	}
}

func TestGenerateEmailHandler_ContainsSendEmailParams(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"type SendEmailParams struct",
		"From ",
		"To ",
		"Subject ",
		"HTMLBody ",
		"SensitiveTokens []string",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}

func TestGenerateEmailHandler_ContainsStatusTracking(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateEmailHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateEmailHandler failed: %v", err)
	}

	codeStr := string(code)

	expected := []string{
		`"sent"`,
		`"failed"`,
		"Status:",
		"ErrorMessage:",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, codeStr)
		}
	}
}
