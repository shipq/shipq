package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateAccountFixture_ValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "fixture.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated account fixture is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateAccountFixture_ImportsOrganizationFixture(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `organizationfixture "example.com/myapp/api/organizations/fixture"`) {
		t.Errorf("expected account fixture to import organizationfixture, got:\n%s", codeStr)
	}
}

func TestGenerateAccountFixture_ReturnsSignupCreateAccountResult(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "*queries.SignupCreateAccountResult") {
		t.Errorf("expected account fixture to return *queries.SignupCreateAccountResult, got:\n%s", codeStr)
	}
}

func TestGenerateAccountFixture_UsesSignupCreateAccount(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "runner.SignupCreateAccount(") {
		t.Errorf("expected account fixture to call runner.SignupCreateAccount, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, "queries.SignupCreateAccountParams{") {
		t.Errorf("expected account fixture to use queries.SignupCreateAccountParams, got:\n%s", codeStr)
	}
}

func TestGenerateAccountFixture_SetsPublicId(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "PublicId:") {
		t.Errorf("expected account fixture to set PublicId in params, got:\n%s", codeStr)
	}
}

func TestGenerateAccountFixture_PassesOrgIdNotPointer(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "DefaultOrganizationId: org.Id") {
		t.Errorf("expected account fixture to pass org.Id (int64), not &org.Id (*int64), got:\n%s", codeStr)
	}
	if strings.Contains(codeStr, "&org.Id") {
		t.Errorf("expected account fixture to NOT use &org.Id (pointer), got:\n%s", codeStr)
	}
}

func TestGenerateAccountFixture_SetsPasswordHash(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "PasswordHash") {
		t.Errorf("expected account fixture to set PasswordHash in CreateAccountParams, got:\n%s", codeStr)
	}
}

func TestGenerateAccountFixture_UsesRandomEmail(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateAccountFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateAccountFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "nanoid.New()") {
		t.Errorf("expected account fixture to use nanoid.New() for randomized email, got:\n%s", codeStr)
	}
}

func TestGenerateOrganizationFixture_ValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateOrganizationFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateOrganizationFixture() error = %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "fixture.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated organization fixture is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOrganizationFixture_ImportsNanoid(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateOrganizationFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateOrganizationFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/nanoid"`) {
		t.Errorf("expected organization fixture to import nanoid, got:\n%s", codeStr)
	}
}

func TestGenerateOrganizationFixture_SetsPublicId(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		Dialect:    "postgres",
	}

	code, err := GenerateOrganizationFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateOrganizationFixture() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "PublicId:") {
		t.Errorf("expected organization fixture to set PublicId in params, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, "nanoid.New()") {
		t.Errorf("expected organization fixture to use nanoid.New() for PublicId, got:\n%s", codeStr)
	}
}
