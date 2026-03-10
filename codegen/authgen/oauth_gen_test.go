package authgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateOAuthShared_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_shared.go is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthShared_ContainsFindOrCreate(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"findOrCreateOAuthAccount",
		"FindOAuthAccount",
		"CreateOAuthAccount",
		"OAuthCreateAccount",
		"ProviderUserID",
		"AvatarURL",
		"oauthUser",
		"generateOAuthState",
		"verifyOAuthState",
		"clearOAuthStateCookie",
		"exchangeOAuthCode",
		"createOAuthSession",
		"oauthRedirectURL",
		"OAUTH_REDIRECT_URL",
		"COOKIE_SECRET",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected output to contain %q, but it didn't", s)
		}
	}
}

func TestGenerateOAuthShared_ContainsPackageAuth(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	if !strings.Contains(string(code), "package auth") {
		t.Error("missing package auth")
	}
}

func TestGenerateOAuthShared_SignupDisabled_NoCreateBranch(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// Should NOT contain auto-create calls
	if strings.Contains(codeStr, "OAuthCreateAccount") {
		t.Error("expected signup-disabled output NOT to contain OAuthCreateAccount")
	}
	if strings.Contains(codeStr, "SignupCreateOrganization") {
		t.Error("expected signup-disabled output NOT to contain SignupCreateOrganization")
	}
	if strings.Contains(codeStr, "SignupCreateOrganizationUser") {
		t.Error("expected signup-disabled output NOT to contain SignupCreateOrganizationUser")
	}

	// Should contain errNoAccount sentinel
	if !strings.Contains(codeStr, "errNoAccount") {
		t.Error("expected signup-disabled output to contain errNoAccount sentinel")
	}
	if !strings.Contains(codeStr, "signup is not enabled") {
		t.Error("expected signup-disabled output to contain 'signup is not enabled' message")
	}

	// Should still contain the find-only logic
	if !strings.Contains(codeStr, "findOrCreateOAuthAccount") {
		t.Error("expected signup-disabled output to contain findOrCreateOAuthAccount")
	}
	if !strings.Contains(codeStr, "FindOAuthAccount") {
		t.Error("expected signup-disabled output to contain FindOAuthAccount")
	}
	if !strings.Contains(codeStr, "FindAccountByEmail") {
		t.Error("expected signup-disabled output to contain FindAccountByEmail")
	}
	// Should still link existing accounts
	if !strings.Contains(codeStr, "CreateOAuthAccount") {
		t.Error("expected signup-disabled output to contain CreateOAuthAccount (for linking existing accounts)")
	}
}

func TestGenerateOAuthShared_SignupEnabled_HasCreateBranch(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// Should contain auto-create calls
	if !strings.Contains(codeStr, "OAuthCreateAccount") {
		t.Error("expected signup-enabled output to contain OAuthCreateAccount")
	}
	if !strings.Contains(codeStr, "SignupCreateOrganization") {
		t.Error("expected signup-enabled output to contain SignupCreateOrganization")
	}

	// Should NOT contain errNoAccount sentinel
	if strings.Contains(codeStr, "errNoAccount") {
		t.Error("expected signup-enabled output NOT to contain errNoAccount")
	}
}

func TestGenerateOAuthShared_SignupDisabled_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_shared.go (signup disabled) is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthShared_SignupEnabled_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_shared.go (signup enabled) is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_Google_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_google.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_google.go is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_Google_SignupDisabled_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_google.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_google.go (signup disabled) is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_Google_SignupEnabled_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_google.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_google.go (signup enabled) is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_Google_SignupDisabled_HasErrNoAccountCheck(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "errors.Is(err, errNoAccount)") {
		t.Error("expected google provider (signup disabled) to contain errors.Is(err, errNoAccount)")
	}
	if !strings.Contains(codeStr, `"errors"`) {
		t.Error("expected google provider (signup disabled) to import \"errors\"")
	}
	if !strings.Contains(codeStr, "?error=no_account") {
		t.Error("expected google provider (signup disabled) to redirect with ?error=no_account")
	}
}

func TestGenerateOAuthProvider_Google_SignupEnabled_NoErrNoAccountCheck(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "errNoAccount") {
		t.Error("expected google provider (signup enabled) NOT to contain errNoAccount")
	}
	if strings.Contains(codeStr, `"errors"`) {
		t.Error("expected google provider (signup enabled) NOT to import \"errors\"")
	}
}

func TestGenerateOAuthProvider_Google_ContainsExpectedURLs(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"https://accounts.google.com/o/oauth2/v2/auth",
		"https://oauth2.googleapis.com/token",
		"https://www.googleapis.com/oauth2/v2/userinfo",
		"GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET",
		`"id"`,
		`"picture"`,
		"GoogleLogin",
		"GoogleCallback",
		"fetchGoogleUser",
		"/auth/google/callback",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected google provider output to contain %q, but it didn't", s)
		}
	}
}

func TestGenerateOAuthProvider_GitHub_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_github.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_github.go is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_GitHub_SignupDisabled_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_github.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_github.go (signup disabled) is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_GitHub_SignupEnabled_IsValidGo(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	_, err = parser.ParseFile(token.NewFileSet(), "oauth_github.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated oauth_github.go (signup enabled) is not valid Go: %v\n%s", err, string(code))
	}
}

func TestGenerateOAuthProvider_GitHub_SignupDisabled_HasErrNoAccountCheck(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "errors.Is(err, errNoAccount)") {
		t.Error("expected github provider (signup disabled) to contain errors.Is(err, errNoAccount)")
	}
	if !strings.Contains(codeStr, `"errors"`) {
		t.Error("expected github provider (signup disabled) to import \"errors\"")
	}
	if !strings.Contains(codeStr, "?error=no_account") {
		t.Error("expected github provider (signup disabled) to redirect with ?error=no_account")
	}
}

func TestGenerateOAuthProvider_GitHub_SignupEnabled_NoErrNoAccountCheck(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "errNoAccount") {
		t.Error("expected github provider (signup enabled) NOT to contain errNoAccount")
	}
	if strings.Contains(codeStr, `"errors"`) {
		t.Error("expected github provider (signup enabled) NOT to import \"errors\"")
	}
}

func TestGenerateOAuthProvider_GitHub_ContainsExpectedURLs(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	codeStr := string(code)

	expected := []string{
		"https://github.com/login/oauth/authorize",
		"https://github.com/login/oauth/access_token",
		"https://api.github.com/user",
		"https://api.github.com/user/emails",
		"GITHUB_CLIENT_ID",
		"GITHUB_CLIENT_SECRET",
		`"avatar_url"`,
		"GitHubLogin",
		"GitHubCallback",
		"fetchGitHubUser",
		"/auth/github/callback",
	}

	for _, s := range expected {
		if !strings.Contains(codeStr, s) {
			t.Errorf("expected github provider output to contain %q, but it didn't", s)
		}
	}
}

func TestGenerateOAuthProvider_GitHub_HasEmailFallback(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	codeStr := string(code)

	// Must have /user/emails fallback for private emails
	if !strings.Contains(codeStr, "/user/emails") {
		t.Error("expected github provider to contain /user/emails fallback")
	}

	if !strings.Contains(codeStr, "fetchGitHubPrimaryEmail") {
		t.Error("expected github provider to contain fetchGitHubPrimaryEmail function")
	}

	// Must check primary field
	if !strings.Contains(codeStr, "Primary") {
		t.Error("expected github provider to check Primary field in email response")
	}

	// Must handle nullable name
	if !strings.Contains(codeStr, "*string") {
		t.Error("expected github provider to use *string for nullable name")
	}

	// Must use fmt.Sprintf for id conversion (number -> string)
	if !strings.Contains(codeStr, `fmt.Sprintf("%.0f"`) {
		t.Error("expected github provider to use fmt.Sprintf for numeric id conversion")
	}
}

func TestGenerateOAuthProvider_UnsupportedProvider_ReturnsError(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath: "example.com/myapp",
	}

	provider := OAuthProviderDef{
		Name: "discord",
	}

	_, err := GenerateOAuthProvider(cfg, provider)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected error to mention 'unsupported', got: %v", err)
	}
}

func TestProviderByName_Google(t *testing.T) {
	p := ProviderByName("google")
	if p == nil {
		t.Fatal("ProviderByName(\"google\") returned nil")
	}
	if p.Name != "google" {
		t.Errorf("expected Name = \"google\", got %q", p.Name)
	}
	if p.DisplayName != "Google" {
		t.Errorf("expected DisplayName = \"Google\", got %q", p.DisplayName)
	}
	if p.ClientIDEnvVar != "GOOGLE_CLIENT_ID" {
		t.Errorf("expected ClientIDEnvVar = \"GOOGLE_CLIENT_ID\", got %q", p.ClientIDEnvVar)
	}
}

func TestProviderByName_GitHub(t *testing.T) {
	p := ProviderByName("github")
	if p == nil {
		t.Fatal("ProviderByName(\"github\") returned nil")
	}
	if p.Name != "github" {
		t.Errorf("expected Name = \"github\", got %q", p.Name)
	}
	if p.DisplayName != "GitHub" {
		t.Errorf("expected DisplayName = \"GitHub\", got %q", p.DisplayName)
	}
}

func TestProviderByName_CaseInsensitive(t *testing.T) {
	p := ProviderByName("Google")
	if p == nil {
		t.Fatal("ProviderByName(\"Google\") returned nil (should be case-insensitive)")
	}
	if p.Name != "google" {
		t.Errorf("expected Name = \"google\", got %q", p.Name)
	}
}

func TestProviderByName_Unknown(t *testing.T) {
	p := ProviderByName("discord")
	if p != nil {
		t.Errorf("expected ProviderByName(\"discord\") to return nil, got %+v", p)
	}
}

func TestAllProviderNames(t *testing.T) {
	names := AllProviderNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 provider names, got %d", len(names))
	}

	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}

	if !found["google"] {
		t.Error("expected AllProviderNames to include \"google\"")
	}
	if !found["github"] {
		t.Error("expected AllProviderNames to include \"github\"")
	}
}

func TestGenerateOAuthProvider_Google_HasRunnerParameter(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	codeStr := string(code)

	// Callback should accept runner as parameter
	if !strings.Contains(codeStr, "runner queries.Runner") {
		t.Error("expected GoogleCallback to accept runner queries.Runner parameter")
	}
}

func TestGenerateOAuthProvider_GitHub_HasRunnerParameter(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"github"},
	}

	code, err := GenerateOAuthProvider(cfg, GitHubProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(github) error = %v", err)
	}

	codeStr := string(code)

	// Callback should accept runner as parameter
	if !strings.Contains(codeStr, "runner queries.Runner") {
		t.Error("expected GitHubCallback to accept runner queries.Runner parameter")
	}
}

func TestGenerateOAuthShared_UsesCorrectModulePath(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "github.com/custom/project",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	expectedImports := []string{
		"github.com/custom/project/shipq/lib/crypto",
		"github.com/custom/project/shipq/lib/nanoid",
		"github.com/custom/project/shipq/queries",
		"github.com/custom/project/config",
	}

	for _, imp := range expectedImports {
		if !strings.Contains(codeStr, imp) {
			t.Errorf("expected import %q, not found in output", imp)
		}
	}
}

func TestGenerateOAuthShared_SignupEnabled_WrapsWritesInTransaction(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "runner.BeginTx(ctx)") {
		t.Error("expected findOrCreateOAuthAccount to call runner.BeginTx(ctx)")
	}
	if !strings.Contains(codeStr, "txRunner.Commit()") {
		t.Error("expected findOrCreateOAuthAccount to call txRunner.Commit()")
	}
	if !strings.Contains(codeStr, "defer txRunner.Rollback()") {
		t.Error("expected findOrCreateOAuthAccount to defer txRunner.Rollback()")
	}

	// All write operations should use txRunner, not runner
	if strings.Contains(codeStr, "runner.SignupCreateOrganization(") {
		t.Error("expected SignupCreateOrganization to use txRunner, not runner")
	}
	if strings.Contains(codeStr, "runner.OAuthCreateAccount(") {
		t.Error("expected OAuthCreateAccount to use txRunner, not runner")
	}
	if strings.Contains(codeStr, "runner.SignupCreateOrganizationUser(") {
		t.Error("expected SignupCreateOrganizationUser to use txRunner, not runner")
	}
	if strings.Contains(codeStr, "runner.CreateOAuthAccount(") {
		t.Error("expected CreateOAuthAccount to use txRunner, not runner")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_shared.go with signup enabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOAuthShared_SignupDisabled_NoTransaction(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// Signup disabled branch only does a single CreateOAuthAccount write,
	// so no transaction is strictly needed. But if it does appear, that's
	// fine — we just want to make sure the code is valid Go.
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_shared.go with signup disabled is not valid Go: %v\n%s", parseErr, string(code))
	}

	// The signup-disabled branch should NOT have org/account creation
	if strings.Contains(codeStr, "SignupCreateOrganization") {
		t.Error("signup-disabled branch should not create organizations")
	}
}

func TestGenerateOAuthShared_EmailEnabled_SetsVerifiedTrue(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
		EmailEnabled:   true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// The OAuthCreateAccountParams block should include Verified: true
	if !strings.Contains(codeStr, "Verified:") {
		t.Error("expected signup-enabled + email-enabled output to contain Verified field in OAuthCreateAccountParams")
	}
	if !strings.Contains(codeStr, "Verified:              true") {
		t.Error("expected Verified field to be set to true")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_shared.go with email enabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOAuthShared_EmailDisabled_NoVerifiedField(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
		EmailEnabled:   false,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// The OAuthCreateAccountParams block should NOT include Verified
	if strings.Contains(codeStr, "Verified:") {
		t.Error("expected signup-enabled + email-disabled output NOT to contain Verified field")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_shared.go with email disabled is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOAuthProvider_Google_ChecksVerifiedEmail(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	codeStr := string(code)

	// The fetchGoogleUser function should check VerifiedEmail, not just parse it
	if !strings.Contains(codeStr, "!info.VerifiedEmail") {
		t.Error("expected fetchGoogleUser to check !info.VerifiedEmail")
	}
	if !strings.Contains(codeStr, "is not verified") {
		t.Error("expected fetchGoogleUser to return an error when email is not verified")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_google.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_google.go is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOAuthProvider_Google_UsesCorrectModulePath(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "github.com/custom/project",
		OAuthProviders: []string{"google"},
	}

	code, err := GenerateOAuthProvider(cfg, GoogleProvider)
	if err != nil {
		t.Fatalf("GenerateOAuthProvider(google) error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "github.com/custom/project/shipq/queries") {
		t.Error("expected google provider to import correct queries package")
	}
	if !strings.Contains(codeStr, "github.com/custom/project/config") {
		t.Error("expected google provider to import correct config package")
	}
}

func TestGenerateOAuthShared_SignupEnabled_HasUniqueViolationRetry(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// Should contain the isOAuthUniqueViolation helper
	if !strings.Contains(codeStr, "func isOAuthUniqueViolation(err error) bool") {
		t.Error("expected generated code to contain isOAuthUniqueViolation helper")
	}

	// The signup-enabled branch wraps writes in a transaction.
	// After CreateOAuthAccount fails, it should check for unique violation
	// and retry by re-reading FindOAuthAccount.
	if !strings.Contains(codeStr, "isOAuthUniqueViolation(err)") {
		t.Error("expected signup-enabled findOrCreateOAuthAccount to check isOAuthUniqueViolation on CreateOAuthAccount error")
	}

	// After detecting a unique violation in the tx path, should rollback and re-read
	createIdx := strings.Index(codeStr, "failed to create oauth account link")
	if createIdx == -1 {
		t.Fatal("expected 'failed to create oauth account link' error in generated code")
	}

	// The retry block should call FindOAuthAccount a second time
	retrySection := codeStr[strings.Index(codeStr, "isOAuthUniqueViolation"):]
	if !strings.Contains(retrySection, "FindOAuthAccount") {
		t.Error("expected retry logic to re-read via FindOAuthAccount after unique violation")
	}
	if !strings.Contains(retrySection, "oauthAcct2") {
		t.Error("expected retry logic to use oauthAcct2 variable for re-read result")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_shared.go with retry logic is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOAuthShared_SignupDisabled_HasUniqueViolationRetry(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  false,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// Should contain the isOAuthUniqueViolation helper
	if !strings.Contains(codeStr, "func isOAuthUniqueViolation(err error) bool") {
		t.Error("expected generated code to contain isOAuthUniqueViolation helper")
	}

	// The signup-disabled branch does NOT use a transaction for CreateOAuthAccount,
	// but should still check for unique violation and retry.
	if !strings.Contains(codeStr, "isOAuthUniqueViolation(err)") {
		t.Error("expected signup-disabled findOrCreateOAuthAccount to check isOAuthUniqueViolation on CreateOAuthAccount error")
	}

	// The retry block should call FindOAuthAccount a second time
	retrySection := codeStr[strings.Index(codeStr, "isOAuthUniqueViolation"):]
	if !strings.Contains(retrySection, "FindOAuthAccount") {
		t.Error("expected retry logic to re-read via FindOAuthAccount after unique violation")
	}

	// Verify it's still valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "oauth_shared.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated oauth_shared.go (signup disabled) with retry logic is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateOAuthShared_IsOAuthUniqueViolation_ChecksCommonPatterns(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// The helper should match common DB driver error strings for:
	// - SQLite: "UNIQUE constraint failed" (contains "unique")
	// - PostgreSQL: "duplicate key value violates unique constraint" (contains "unique" and "duplicate")
	// - MySQL: "Duplicate entry" (contains "duplicate")
	if !strings.Contains(codeStr, `strings.Contains(msg, "unique")`) {
		t.Error("isOAuthUniqueViolation should check for 'unique' substring")
	}
	if !strings.Contains(codeStr, `strings.Contains(msg, "duplicate")`) {
		t.Error("isOAuthUniqueViolation should check for 'duplicate' substring")
	}
	if !strings.Contains(codeStr, "strings.ToLower") {
		t.Error("isOAuthUniqueViolation should do case-insensitive matching")
	}
}

func TestGenerateOAuthShared_SignupEnabled_RetryRollsBackTransaction(t *testing.T) {
	cfg := AuthGenConfig{
		ModulePath:     "example.com/myapp",
		OAuthProviders: []string{"google"},
		SignupEnabled:  true,
	}

	code, err := GenerateOAuthShared(cfg)
	if err != nil {
		t.Fatalf("GenerateOAuthShared() error = %v", err)
	}

	codeStr := string(code)

	// In the signup-enabled branch, the retry path must rollback the
	// transaction before re-reading, because the tx is in a failed state.
	uniqueIdx := strings.Index(codeStr, "isOAuthUniqueViolation(err)")
	if uniqueIdx == -1 {
		t.Fatal("expected isOAuthUniqueViolation check in generated code")
	}

	// Find the section between the unique violation check and the retry FindOAuthAccount
	retryBlock := codeStr[uniqueIdx:]
	findIdx := strings.Index(retryBlock, "FindOAuthAccount")
	if findIdx == -1 {
		t.Fatal("expected FindOAuthAccount call after unique violation check")
	}

	beforeRetry := retryBlock[:findIdx]
	if !strings.Contains(beforeRetry, "Rollback") {
		t.Error("expected transaction Rollback before retry FindOAuthAccount in signup-enabled branch")
	}
}
