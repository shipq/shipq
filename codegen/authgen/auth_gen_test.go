package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// OAuth-related tests
// ---------------------------------------------------------------------------

func TestGenerateLoginHandler_WithOAuth_HasNilGuard(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "PasswordHash == nil") {
		t.Error("expected login handler to contain PasswordHash nil guard when OAuth is enabled")
	}
	if !strings.Contains(codeStr, "this account does not have a password") {
		t.Error("expected login handler to contain password nil error message")
	}

	// Verify it's still valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "login.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated login.go with OAuth is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateLoginHandler_WithoutOAuth_StillHasNilGuard(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		// OAuthProviders is empty
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	// password_hash is always nullable, so the nil guard must always be present
	if !strings.Contains(codeStr, "PasswordHash == nil") {
		t.Error("expected login handler to ALWAYS contain PasswordHash nil guard")
	}
	if !strings.Contains(codeStr, "this account does not have a password") {
		t.Error("expected login handler to contain password nil error message")
	}
}

func TestGenerateRegister_WithOAuth_EmitsOAuthRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google", "github"},
	}

	code, err := GenerateRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateRegister() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "RegisterOAuthRoutes") {
		t.Error("expected register.go to contain RegisterOAuthRoutes when OAuth is enabled")
	}
	if !strings.Contains(codeStr, "GET /auth/google") {
		t.Error("expected register.go to contain Google OAuth route")
	}
	if !strings.Contains(codeStr, "GET /auth/github") {
		t.Error("expected register.go to contain GitHub OAuth route")
	}
	if !strings.Contains(codeStr, "runner queries.Runner") {
		t.Error("expected RegisterOAuthRoutes to accept runner queries.Runner")
	}

	// Verify it's valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "register.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated register.go with OAuth is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateRegister_WithoutOAuth_OmitsOAuthRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateRegister() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "RegisterOAuthRoutes") {
		t.Error("register.go should NOT contain RegisterOAuthRoutes when OAuth is not enabled")
	}
}

func TestGenerateRegister_GoogleOnly(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateRegister() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "GET /auth/google") {
		t.Error("expected Google routes present")
	}
	if strings.Contains(codeStr, "GET /auth/github") {
		t.Error("expected GitHub routes absent when only Google is enabled")
	}
}

func TestGenerateSignupRegister_WithOAuth_EmitsOAuthRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateSignupRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupRegister() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "RegisterOAuthRoutes") {
		t.Error("expected signup register.go to contain RegisterOAuthRoutes when OAuth is enabled")
	}
	if !strings.Contains(codeStr, `app.Post("/signup", Signup)`) {
		t.Error("expected signup register.go to still contain /signup route")
	}

	// Verify it's valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "register.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated signup register.go with OAuth is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateSignupRegister_WithoutOAuth_OmitsOAuthRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateSignupRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupRegister() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "RegisterOAuthRoutes") {
		t.Error("signup register.go should NOT contain RegisterOAuthRoutes when OAuth is not enabled")
	}
}

func TestGenerateAuthQueryDefs_WithOAuth_ContainsOAuthQueries(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"FindOAuthAccount",
		"CreateOAuthAccount",
		"OAuthCreateAccount",
		"OauthAccounts",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected query defs with OAuth to contain %q", s)
		}
	}

	// Verify it's valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated queries.go with OAuth is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateAuthQueryDefs_WithoutOAuth_OmitsOAuthQueries(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		// OAuthProviders is empty
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "FindOAuthAccount") {
		t.Error("query defs should NOT contain FindOAuthAccount when OAuth is not enabled")
	}
	if strings.Contains(codeStr, "CreateOAuthAccount") {
		t.Error("query defs should NOT contain CreateOAuthAccount when OAuth is not enabled")
	}
	if strings.Contains(codeStr, "OAuthCreateAccount") {
		t.Error("query defs should NOT contain OAuthCreateAccount when OAuth is not enabled")
	}
}

func TestGenerateAuthQueryDefs_OAuthCreateAccount_OmitsPasswordHash(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	// OAuthCreateAccount should NOT include password_hash in its columns
	// Find the OAuthCreateAccount section and verify it doesn't reference PasswordHash
	oauthIdx := strings.Index(codeStr, "OAuthCreateAccount")
	if oauthIdx == -1 {
		t.Fatal("OAuthCreateAccount not found in output")
	}

	// Get the section between OAuthCreateAccount and the next query.MustDefine or end
	section := codeStr[oauthIdx:]
	nextQuery := strings.Index(section[1:], "query.MustDefine")
	if nextQuery > 0 {
		section = section[:nextQuery+1]
	}

	if strings.Contains(section, "PasswordHash") {
		t.Error("OAuthCreateAccount should NOT include PasswordHash column")
	}
}

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
		// signup.go removed -- generated by `shipq signup` instead
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

	// Should have RoleInfo struct with name and description
	if !strings.Contains(codeStr, "type RoleInfo struct") {
		t.Error("missing RoleInfo struct")
	}

	// MeResponse should have Roles field
	if !strings.Contains(codeStr, "Roles") {
		t.Error("missing Roles field in MeResponse")
	}

	// Me handler should map account.Roles to RoleInfo slice
	if !strings.Contains(codeStr, "account.Roles") {
		t.Error("missing account.Roles access in Me handler")
	}
}

func TestGenerateMeHandler_ScopedIncludesOrgLookup(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:  "example.com/myapp",
		ScopeColumn: "organization_id",
	}

	code, err := GenerateMeHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateMeHandler() error = %v", err)
	}

	codeStr := string(code)

	// Scoped variant should include organization lookup
	if !strings.Contains(codeStr, "FindOrganizationByInternalID") {
		t.Error("scoped Me handler should look up default organization")
	}
}

func TestGenerateMeHandler_UnscopedNoOrgLookup(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
		// ScopeColumn is empty = unscoped
	}

	code, err := GenerateMeHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateMeHandler() error = %v", err)
	}

	codeStr := string(code)

	// Unscoped variant should NOT include organization lookup
	if strings.Contains(codeStr, "FindOrganizationByInternalID") {
		t.Error("unscoped Me handler should NOT look up organization")
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

	// Should have login, logout, me routes (signup is NOT included by default)
	routes := []string{
		`app.Post("/login", Login)`,
		`app.Delete("/logout", Logout).Auth()`,
		`app.Get("/me", Me).Auth()`,
	}

	for _, route := range routes {
		if !strings.Contains(codeStr, route) {
			t.Errorf("missing route: %s", route)
		}
	}

	// Signup should NOT be present in default register.go
	if strings.Contains(codeStr, "/signup") {
		t.Error("default register.go should NOT contain /signup route (use shipq signup instead)")
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

	// Should import database/sql for sql.ErrNoRows classification
	if !strings.Contains(codeStr, `"database/sql"`) {
		t.Error("missing import: database/sql (needed for sql.ErrNoRows in TryGetCurrentSession)")
	}

	// Should have getCurrentSession helper (other DB operations go through the query runner)
	if !strings.Contains(codeStr, "func getCurrentSession") {
		t.Error("missing helper: func getCurrentSession")
	}

	// Should have exported GetCurrentSession
	if !strings.Contains(codeStr, "func GetCurrentSession") {
		t.Error("missing helper: func GetCurrentSession")
	}

	// Should have TryGetCurrentSession for optional auth
	if !strings.Contains(codeStr, "func TryGetCurrentSession") {
		t.Error("missing helper: func TryGetCurrentSession")
	}

	// Should have ErrNoValidSession sentinel
	if !strings.Contains(codeStr, "ErrNoValidSession") {
		t.Error("missing sentinel: ErrNoValidSession")
	}

	// TryGetCurrentSession should handle http.ErrNoCookie, crypto errors, sql.ErrNoRows, and nil session
	expectedChecks := []string{
		"http.ErrNoCookie",
		"crypto.ErrInvalidCookie",
		"crypto.ErrInvalidSignature",
		"sql.ErrNoRows",
		"session == nil",
	}
	for _, check := range expectedChecks {
		if !strings.Contains(codeStr, check) {
			t.Errorf("TryGetCurrentSession should check for %s", check)
		}
	}
}

func TestGenerateAuthTestFiles_ValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:      "example.com/myapp",
		Dialect:         "sqlite",
		TestDatabaseURL: "sqlite:./test.db",
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

	codeStr := string(code)

	// Should use TEST_DATABASE_URL, not DATABASE_URL
	if !strings.Contains(codeStr, "TEST_DATABASE_URL") {
		t.Error("missing TEST_DATABASE_URL environment variable")
	}
	if strings.Contains(codeStr, `os.Getenv("DATABASE_URL")`) {
		t.Error("should not reference DATABASE_URL, only TEST_DATABASE_URL")
	}

	// Should have localhost guard via config.IsLocalhostURL
	if !strings.Contains(codeStr, "config.IsLocalhostURL") {
		t.Error("missing config.IsLocalhostURL guard")
	}

	// Should import only the sqlite driver, not all three
	if !strings.Contains(codeStr, `"modernc.org/sqlite"`) {
		t.Error("missing sqlite driver import")
	}
	if strings.Contains(codeStr, `"github.com/lib/pq"`) {
		t.Error("should not import lib/pq")
	}
	if strings.Contains(codeStr, `"github.com/go-sql-driver/mysql"`) {
		t.Error("should not import mysql driver for sqlite dialect")
	}
}

// TestGeneratedAuthHandlers_NoRFC3339ForSQL verifies that generated signup and
// login handlers use a MySQL-compatible datetime format ("2006-01-02 15:04:05")
// rather than time.RFC3339, which MySQL rejects with Error 1292 (22007).
func TestGeneratedAuthHandlers_NoRFC3339ForSQL(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	tests := []struct {
		name     string
		generate func(AuthGenConfig) ([]byte, error)
	}{
		{"signup.go", GenerateSignupHandler},
		{"login.go", GenerateLoginHandler},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := tt.generate(cfg)
			if err != nil {
				t.Fatalf("generate %s failed: %v", tt.name, err)
			}

			codeStr := string(code)

			// These handlers create sessions with ExpiresAt. The datetime format
			// MUST be MySQL-compatible. time.RFC3339 produces "2006-01-02T15:04:05Z07:00"
			// which MySQL rejects.
			if strings.Contains(codeStr, "time.RFC3339") {
				t.Errorf("%s: contains time.RFC3339 which is incompatible with MySQL DATETIME columns.\n"+
					"Use \"2006-01-02 15:04:05\" instead", tt.name)
			}

			// Verify the correct format is present
			if !strings.Contains(codeStr, `"2006-01-02 15:04:05"`) {
				t.Errorf("%s: missing MySQL-compatible datetime format \"2006-01-02 15:04:05\"", tt.name)
			}
		})
	}
}

func TestGenerateAuthHandlerTests_ContainsExpectedTests(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:      "example.com/myapp",
		Dialect:         "postgres",
		TestDatabaseURL: "postgres://postgres@localhost:5432/myapp_test",
	}

	code, err := GenerateAuthHandlerTests(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthHandlerTests() error = %v", err)
	}

	codeStr := string(code)

	// Should have test functions (signup tests are generated separately by shipq signup)
	tests := []string{
		"func createTestUser",
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

// --- EmailEnabled branching tests ---

func TestGenerateLoginHandler_EmailEnabled_HasVerifiedCheck(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "Verified") {
		t.Error("expected login handler to contain Verified check when EmailEnabled is true")
	}
	if !strings.Contains(codeStr, `"email not verified"`) {
		t.Error("expected login handler to contain 'email not verified' error message")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "login.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated login.go with EmailEnabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateLoginHandler_EmailDisabled_NoVerifiedCheck(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: false,
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "Verified") {
		t.Error("expected login handler to NOT contain Verified check when EmailEnabled is false")
	}
	if strings.Contains(codeStr, `"email not verified"`) {
		t.Error("expected login handler to NOT contain 'email not verified' error message when EmailEnabled is false")
	}
}

func TestGenerateLoginHandler_EmailEnabledWithOAuth_BothGuards(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		EmailEnabled:   true,
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	// Should have both OAuth nil guard AND email verified check
	if !strings.Contains(codeStr, "PasswordHash == nil") {
		t.Error("expected login handler to contain PasswordHash nil guard")
	}
	if !strings.Contains(codeStr, "Verified") {
		t.Error("expected login handler to contain Verified check")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "login.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated login.go with OAuth+Email is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateSignupHandler_EmailEnabled_SendsVerificationEmail(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateSignupHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupHandler() error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"InsertEmailVerificationToken",
		"SendEmail",
		"SendEmailParams",
		"generateSecureToken",
		"HashToken",
		"Verify Your Email",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected signup handler with EmailEnabled to contain %q, but it didn't", s)
		}
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "signup.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated signup.go with EmailEnabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateSignupHandler_EmailDisabled_NoVerificationEmail(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: false,
	}

	code, err := GenerateSignupHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupHandler() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "InsertEmailVerificationToken") {
		t.Error("expected signup handler to NOT contain InsertEmailVerificationToken when EmailEnabled is false")
	}
	if strings.Contains(codeStr, "SendEmail") {
		t.Error("expected signup handler to NOT contain SendEmail when EmailEnabled is false")
	}
}

func TestGenerateRegister_EmailEnabled_HasEmailRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateRegister() error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"/forgot-password",
		"/reset-password",
		"/verify-email",
		"/resend-verification",
		"ForgotPassword",
		"ResetPassword",
		"VerifyEmail",
		"ResendVerification",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected register with EmailEnabled to contain %q, but it didn't", s)
		}
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "register.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated register.go with EmailEnabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateRegister_EmailDisabled_NoEmailRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: false,
	}

	code, err := GenerateRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateRegister() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "/forgot-password") {
		t.Error("expected register to NOT contain /forgot-password when EmailEnabled is false")
	}
	if strings.Contains(codeStr, "/verify-email") {
		t.Error("expected register to NOT contain /verify-email when EmailEnabled is false")
	}
}

func TestGenerateSignupRegister_EmailEnabled_HasEmailRoutes(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateSignupRegister(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupRegister() error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"/forgot-password",
		"/reset-password",
		"/verify-email",
		"/resend-verification",
		"/signup",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected signup register with EmailEnabled to contain %q, but it didn't", s)
		}
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "register.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated signup register.go with EmailEnabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateAuthQueryDefs_EmailEnabled_ContainsEmailQueries(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"InsertSentEmail",
		"InsertPasswordResetToken",
		"FindPasswordResetToken",
		"MarkPasswordResetTokenUsed",
		"InvalidatePasswordResetTokens",
		"UpdateAccountPassword",
		"VerifyAccount",
		"InsertEmailVerificationToken",
		"FindEmailVerificationToken",
		"MarkEmailVerificationTokenUsed",
		"schema.SentEmails",
		"schema.PasswordResetTokens",
		"schema.EmailVerificationTokens",
		"Verified()",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected auth query defs with EmailEnabled to contain %q, but it didn't", s)
		}
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated querydefs with EmailEnabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateAuthQueryDefs_EmailDisabled_OmitsEmailQueries(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: false,
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "InsertSentEmail") {
		t.Error("expected auth query defs to NOT contain InsertSentEmail when EmailEnabled is false")
	}
	if strings.Contains(codeStr, "FindPasswordResetToken") {
		t.Error("expected auth query defs to NOT contain FindPasswordResetToken when EmailEnabled is false")
	}
	if strings.Contains(codeStr, "schema.SentEmails") {
		t.Error("expected auth query defs to NOT contain schema.SentEmails when EmailEnabled is false")
	}
	if strings.Contains(codeStr, "Verified()") {
		t.Error("expected auth query defs to NOT select Verified() when EmailEnabled is false")
	}
}

func TestGenerateAuthQueryDefs_EmailEnabled_FindAccountByEmail_SelectsVerified(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:   "example.com/myapp",
		EmailEnabled: true,
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	// FindAccountByEmail should additionally select Verified when email is enabled
	if !strings.Contains(codeStr, "schema.Accounts.Verified()") {
		t.Error("expected FindAccountByEmail to select Verified column when EmailEnabled is true")
	}
}

func TestGenerateSignupFiles_ValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	files, err := GenerateSignupFiles(cfg)
	if err != nil {
		t.Fatalf("GenerateSignupFiles() error = %v", err)
	}

	// Should contain signup.go and register.go
	expectedFiles := []string{"signup.go", "register.go"}
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

	// register.go from GenerateSignupFiles SHOULD include the signup route
	registerCode := string(files["register.go"])
	if !strings.Contains(registerCode, `app.Post("/signup", Signup)`) {
		t.Error("signup register.go should include /signup route")
	}

	// Should also include the other auth routes
	if !strings.Contains(registerCode, `app.Post("/login", Login)`) {
		t.Error("signup register.go should include /login route")
	}
	if !strings.Contains(registerCode, `app.Get("/me", Me).Auth()`) {
		t.Error("signup register.go should include /me route")
	}
}

// ---------------------------------------------------------------------------
// Bug 4: TryGetCurrentSession must classify sql.ErrNoRows as ErrNoValidSession
// ---------------------------------------------------------------------------

// TestGenerateHelpers_TryGetCurrentSession_ClassifiesSqlErrNoRows is a targeted
// regression test for Bug 4. When a user has a validly-signed session cookie but
// the session row no longer exists in the database (expired, soft-deleted, or
// manually removed), FindActiveSession returns sql.ErrNoRows. TryGetCurrentSession
// must classify this as ErrNoValidSession so that WrapOptionalAuthHandler proceeds
// unauthenticated instead of returning 500 Internal Server Error.
func TestGenerateHelpers_TryGetCurrentSession_ClassifiesSqlErrNoRows(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateHelpers(cfg)
	if err != nil {
		t.Fatalf("GenerateHelpers() error = %v", err)
	}

	codeStr := string(code)

	// The generated TryGetCurrentSession must include sql.ErrNoRows in the
	// error classification block alongside http.ErrNoCookie and crypto errors.
	// Without this, expired sessions produce 500s on optional-auth routes.
	if !strings.Contains(codeStr, "sql.ErrNoRows") {
		t.Fatal("TryGetCurrentSession must classify sql.ErrNoRows as ErrNoValidSession")
	}

	// Verify sql.ErrNoRows appears in the same errors.Is block as the other
	// session-related errors (not in a separate, unrelated block).
	lines := strings.Split(codeStr, "\n")
	inClassificationBlock := false
	foundInBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// The classification block starts with the first errors.Is check
		if strings.Contains(trimmed, "errors.Is(err, http.ErrNoCookie)") {
			inClassificationBlock = true
		}
		if inClassificationBlock && strings.Contains(trimmed, "sql.ErrNoRows") {
			foundInBlock = true
			break
		}
		// The block ends at the closing brace + return
		if inClassificationBlock && strings.Contains(trimmed, "return nil, ErrNoValidSession") {
			break
		}
	}
	if !foundInBlock {
		t.Error("sql.ErrNoRows must be in the same errors.Is classification block as http.ErrNoCookie and crypto errors")
	}

	// Must import database/sql for sql.ErrNoRows
	if !strings.Contains(codeStr, `"database/sql"`) {
		t.Error("helpers.go must import database/sql for sql.ErrNoRows")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "helpers.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated helpers.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

// ---------------------------------------------------------------------------
// Bug 1: Centrifugo tokens must use account public ID, not session public ID
// ---------------------------------------------------------------------------

func TestGenerateAuthQueryDefs_FindActiveSession_SelectsAccountPublicId(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateAuthQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateAuthQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	// FindActiveSession must select the account's public_id with an alias
	// to disambiguate it from sessions.public_id.
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.PublicId(), "account_public_id")`) {
		t.Error("FindActiveSession must select schema.Accounts.PublicId() with alias \"account_public_id\"")
	}

	// It should still select the session's public_id as well (for cookie lookup)
	if !strings.Contains(codeStr, "schema.Sessions.PublicId()") {
		t.Error("FindActiveSession must still select schema.Sessions.PublicId()")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated queries.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateLoginHandler_AlwaysHasNilGuard(t *testing.T) {
	// Empty config: no OAuth, no email — nil guard must still be present
	// because password_hash is always nullable.
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	code, err := GenerateLoginHandler(cfg)
	if err != nil {
		t.Fatalf("GenerateLoginHandler() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "PasswordHash == nil") {
		t.Error("expected login handler to ALWAYS contain PasswordHash nil guard, even without OAuth or email")
	}
	if !strings.Contains(codeStr, "this account does not have a password") {
		t.Error("expected login handler to contain generic password nil error message")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "login.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated login.go with empty config is not valid Go: %v\n%s", parseErr, string(code))
	}
}
