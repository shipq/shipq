package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateAuthHandlerFiles_ValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	files, err := GenerateAuthHandlerFiles(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthHandlerFiles() error = %v", err)
	}

	expectedFiles := []string{
		"login.go",
		"logout.go",
		"me.go",
		"signup.go",
		"register.go",
		"helpers.go",
	}

	for _, filename := range expectedFiles {
		code, ok := files[filename]
		if !ok {
			t.Errorf("missing expected file: %s", filename)
			continue
		}

		// Verify it's valid Go
		_, err := parser.ParseFile(token.NewFileSet(), filename, code, parser.AllErrors)
		if err != nil {
			t.Errorf("generated %s is not valid Go: %v\n%s", filename, err, string(code))
		}
	}
}

func TestGenerateLoginHandler_ContainsExpectedElements(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	// Should have package auth
	if !strings.Contains(codeStr, "package auth") {
		t.Error("missing package auth")
	}

	// Should have LoginRequest struct
	if !strings.Contains(codeStr, "type LoginRequest struct") {
		t.Error("missing LoginRequest struct")
	}

	// Should have LoginResponse struct
	if !strings.Contains(codeStr, "type LoginResponse struct") {
		t.Error("missing LoginResponse struct")
	}

	// Should have Login function
	if !strings.Contains(codeStr, "func Login(ctx context.Context") {
		t.Error("missing Login function")
	}

	// Should use crypto package
	if !strings.Contains(codeStr, "crypto.VerifyPassword") {
		t.Error("missing crypto.VerifyPassword call")
	}

	// Should set session cookie
	if !strings.Contains(codeStr, "setSessionCookie") {
		t.Error("missing setSessionCookie call")
	}
}

func TestGenerateSignupHandler_ContainsExpectedElements(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateSignupHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupHandler() error = %v", err)
	}

	codeStr := string(code)

	// Should have SignupRequest struct
	if !strings.Contains(codeStr, "type SignupRequest struct") {
		t.Error("missing SignupRequest struct")
	}

	// Should have SignupResponse struct
	if !strings.Contains(codeStr, "type SignupResponse struct") {
		t.Error("missing SignupResponse struct")
	}

	// Should have Signup function
	if !strings.Contains(codeStr, "func Signup(ctx context.Context") {
		t.Error("missing Signup function")
	}

	// Should hash password
	if !strings.Contains(codeStr, "crypto.HashPassword") {
		t.Error("missing crypto.HashPassword call")
	}

	// Should create organization with auto-generated name
	if !strings.Contains(codeStr, `fmt.Sprintf("%s's Organization"`) {
		t.Error("missing auto-generated organization name")
	}
}

func TestGenerateMeHandler_ContainsExpectedElements(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateMeHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateMeHandler() error = %v", err)
	}

	codeStr := string(code)

	// Should have MeResponse struct
	if !strings.Contains(codeStr, "type MeResponse struct") {
		t.Error("missing MeResponse struct")
	}

	// Should have Me function
	if !strings.Contains(codeStr, "func Me(ctx context.Context") {
		t.Error("missing Me function")
	}

	// Should get current session
	if !strings.Contains(codeStr, "getCurrentSession") {
		t.Error("missing getCurrentSession call")
	}

	// Should include organization info
	if !strings.Contains(codeStr, "OrgInfo") {
		t.Error("missing OrgInfo struct")
	}
}

func TestGenerateLogoutHandler_ContainsExpectedElements(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateLogoutHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLogoutHandler() error = %v", err)
	}

	codeStr := string(code)

	// Should have LogoutResponse struct
	if !strings.Contains(codeStr, "type LogoutResponse struct") {
		t.Error("missing LogoutResponse struct")
	}

	// Should have Logout function
	if !strings.Contains(codeStr, "func Logout(ctx context.Context") {
		t.Error("missing Logout function")
	}

	// Should clear session cookie
	if !strings.Contains(codeStr, "clearSessionCookie") {
		t.Error("missing clearSessionCookie call")
	}
}

func TestGenerateRegister_ContainsExpectedRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateRegister() error = %v", err)
	}

	codeStr := string(code)

	// Should have all four routes
	routes := []string{
		`app.Post("/login", Login)`,
		`app.Delete("/logout", Logout)`,
		`app.Get("/me", Me)`,
		`app.Post("/signup", Signup)`,
	}

	for _, route := range routes {
		if !strings.Contains(codeStr, route) {
			t.Errorf("missing route: %s", route)
		}
	}
}

func TestGenerateHelpers_ContainsExpectedFunctions(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateHelpers(cfg)
	if err != nil {
		t.Fatalf("GenerateHelpers() error = %v", err)
	}

	codeStr := string(code)

	// Should have helper functions
	helpers := []string{
		"func findAccountByEmail",
		"func findAccountByID",
		"func findOrganizationByID",
		"func createSession",
		"func getCurrentSession",
		"func deleteSession",
		"func createOrganization",
		"func createAccount",
		"func createOrganizationUser",
	}

	for _, helper := range helpers {
		if !strings.Contains(codeStr, helper) {
			t.Errorf("missing helper: %s", helper)
		}
	}
}

func TestGenerateAuthTestFiles_ValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	files, err := GenerateAuthTestFiles(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthTestFiles() error = %v", err)
	}

	code, ok := files["handlers_http_test.go"]
	if !ok {
		t.Fatal("missing handlers_http_test.go")
	}

	// Verify it's valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "handlers_http_test.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated test file is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateAuthHandlerTests_ContainsExpectedTests(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateAuthHandlerTests(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthHandlerTests() error = %v", err)
	}

	codeStr := string(code)

	// Should have test functions
	tests := []string{
		"func TestSignup_Success",
		"func TestSignup_DuplicateEmail",
		"func TestLogin_Success",
		"func TestLogin_InvalidPassword",
		"func TestLogin_NonexistentEmail",
		"func TestMe_Authenticated",
		"func TestMe_Unauthenticated",
		"func TestLogout_Success",
		"func TestLogout_Unauthenticated",
	}

	for _, test := range tests {
		if !strings.Contains(codeStr, test) {
			t.Errorf("missing test: %s", test)
		}
	}
}
