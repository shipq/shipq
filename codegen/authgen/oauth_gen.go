package authgen

import (
	"bytes"
	"fmt"
	"strings"
)

// OAuthProviderDef captures everything needed to generate OAuth handlers
// for a single provider. Adding a new provider means defining one of these.
type OAuthProviderDef struct {
	// Name is the lowercase provider name, e.g., "google", "github".
	// Used for route paths, file names, ini keys, and env var prefixes.
	Name string

	// DisplayName is the human-readable name, e.g., "Google", "GitHub".
	DisplayName string

	// AuthURL is the provider's authorization endpoint.
	AuthURL string

	// TokenURL is the provider's token exchange endpoint.
	TokenURL string

	// Scopes is the space-separated list of OAuth scopes to request.
	Scopes string

	// ClientIDEnvVar is the env var name for the client ID.
	ClientIDEnvVar string

	// ClientSecretEnvVar is the env var name for the client secret.
	ClientSecretEnvVar string

	// FetchUserFunc is the name of the generated function that takes an
	// access token and returns (*oauthUser, error).
	// Each provider's user-info endpoint has a different JSON shape,
	// so this function is generated per-provider.
	FetchUserFunc string
}

// GoogleProvider is the built-in OAuthProviderDef for Google.
var GoogleProvider = OAuthProviderDef{
	Name:               "google",
	DisplayName:        "Google",
	AuthURL:            "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL:           "https://oauth2.googleapis.com/token",
	Scopes:             "openid email profile",
	ClientIDEnvVar:     "GOOGLE_CLIENT_ID",
	ClientSecretEnvVar: "GOOGLE_CLIENT_SECRET",
	FetchUserFunc:      "fetchGoogleUser",
}

// GitHubProvider is the built-in OAuthProviderDef for GitHub.
var GitHubProvider = OAuthProviderDef{
	Name:               "github",
	DisplayName:        "GitHub",
	AuthURL:            "https://github.com/login/oauth/authorize",
	TokenURL:           "https://github.com/login/oauth/access_token",
	Scopes:             "user:email",
	ClientIDEnvVar:     "GITHUB_CLIENT_ID",
	ClientSecretEnvVar: "GITHUB_CLIENT_SECRET",
	FetchUserFunc:      "fetchGitHubUser",
}

// ProviderByName returns the OAuthProviderDef for the given provider name,
// or nil if the provider is not recognized.
func ProviderByName(name string) *OAuthProviderDef {
	switch strings.ToLower(name) {
	case "google":
		p := GoogleProvider
		return &p
	case "github":
		p := GitHubProvider
		return &p
	default:
		return nil
	}
}

// AllProviderNames returns the list of all supported OAuth provider names.
func AllProviderNames() []string {
	return []string{"google", "github"}
}

// GenerateOAuthShared generates api/auth/oauth_shared.go with shared OAuth
// utilities: oauthUser struct, state cookie management, token exchange,
// findOrCreateOAuthAccount, session creation, and redirect URL helpers.
func GenerateOAuthShared(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"crypto/rand\"\n")
	buf.WriteString("\t\"encoding/hex\"\n")
	buf.WriteString("\t\"encoding/json\"\n")
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"io\"\n")
	buf.WriteString("\t\"net/http\"\n")
	buf.WriteString("\t\"net/url\"\n")
	buf.WriteString("\t\"strings\"\n")
	buf.WriteString("\t\"time\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/crypto"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/nanoid"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/config"))
	buf.WriteString(")\n\n")

	// oauthUser struct
	buf.WriteString(`// oauthUser holds the normalised user info returned by any OAuth provider.
type oauthUser struct {
	Email          string
	FirstName      string
	LastName       string
	ProviderUserID string // the user's unique ID on the provider (e.g., Google sub, GitHub user ID)
	AvatarURL      string // profile picture URL (may be empty)
}

`)

	// generateOAuthState
	buf.WriteString(`// generateOAuthState creates a signed CSRF state token and sets it as a cookie.
func generateOAuthState(w http.ResponseWriter) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)

	signed := crypto.SignCookie(state, []byte(config.Settings.COOKIE_SECRET))

	http.SetCookie(w, &http.Cookie{
		Name:     "__oauth_state",
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.Settings.GO_ENV != "development",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	return state, nil
}

`)

	// verifyOAuthState
	buf.WriteString(`// verifyOAuthState checks the state query param against the signed cookie.
func verifyOAuthState(r *http.Request, queryState string) error {
	cookie, err := r.Cookie("__oauth_state")
	if err != nil {
		return fmt.Errorf("missing oauth state cookie: %w", err)
	}
	stored, err := crypto.VerifyCookie(cookie.Value, []byte(config.Settings.COOKIE_SECRET))
	if err != nil {
		return fmt.Errorf("invalid oauth state cookie: %w", err)
	}
	if stored != queryState {
		return fmt.Errorf("oauth state mismatch")
	}
	return nil
}

`)

	// clearOAuthStateCookie
	buf.WriteString(`// clearOAuthStateCookie removes the state cookie after verification.
func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "__oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

`)

	// exchangeOAuthCode
	buf.WriteString(`// exchangeOAuthCode exchanges an authorization code for an access token.
func exchangeOAuthCode(tokenURL, code, clientID, clientSecret, redirectURI string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp struct {
		AccessToken string ` + "`json:\"access_token\"`" + `
		Error       string ` + "`json:\"error\"`" + `
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("oauth token error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}
	return tokenResp.AccessToken, nil
}

`)

	// findOrCreateOAuthAccount — branched on SignupEnabled
	if cfg.SignupEnabled {
		// SignupEnabled: true — auto-create account when no match is found
		buf.WriteString(`// findOrCreateOAuthAccount looks up an account by OAuth provider+ID first,
// then by email. If no account exists, it creates one with a NULL password
// hash. In all cases it ensures an oauth_accounts row links the provider to
// the account. All writes are wrapped in a transaction for atomicity.
func findOrCreateOAuthAccount(ctx context.Context, runner queries.Runner, provider string, user oauthUser) (int64, string, error) {
	// 1. Check if this provider+user_id is already linked to an account
	oauthAcct, err := runner.FindOAuthAccount(ctx, queries.FindOAuthAccountParams{
		Provider:       provider,
		ProviderUserId: user.ProviderUserID,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to lookup oauth account: %w", err)
	}
	if oauthAcct != nil {
		// Already linked — return the associated account
		return oauthAcct.AccountId, oauthAcct.PublicId, nil
	}

	// 2. Check if an account with this email already exists
	existing, err := runner.FindAccountByEmail(ctx, queries.FindAccountByEmailParams{
		Email: user.Email,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to lookup account: %w", err)
	}

	// 3. Start a transaction for all write operations
	txRunner, err := runner.BeginTx(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer txRunner.Rollback() // no-op after commit

	var accountID int64
	var accountPublicID string

	if existing != nil {
		// Account exists (e.g., signed up via email/password or another provider).
		// Link this OAuth provider to the existing account.
		accountID = existing.Id
		accountPublicID = existing.PublicId
	} else {
		// 4. No account found — create org + account with NULL password hash
		orgName := fmt.Sprintf("%s's Organization", user.FirstName)
		org, err := txRunner.SignupCreateOrganization(ctx, queries.SignupCreateOrganizationParams{
			PublicId:    nanoid.New(),
			Name:        orgName,
			Description: "",
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to create organization: %w", err)
		}

		account, err := txRunner.OAuthCreateAccount(ctx, queries.OAuthCreateAccountParams{
			PublicId:              nanoid.New(),
			FirstName:             user.FirstName,
			LastName:              user.LastName,
			Email:                 user.Email,
			DefaultOrganizationId: org.Id,
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to create account: %w", err)
		}

		accountID = account.Id
		accountPublicID = account.PublicId

		// Link account to organization
		_, err = txRunner.SignupCreateOrganizationUser(ctx, queries.SignupCreateOrganizationUserParams{
			PublicId:       nanoid.New(),
			OrganizationId: org.Id,
			AccountId:      accountID,
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to link account to org: %w", err)
		}
	}

	// 5. Create the oauth_accounts link
	_, err = txRunner.CreateOAuthAccount(ctx, queries.CreateOAuthAccountParams{
		PublicId:        nanoid.New(),
		AccountId:       accountID,
		AuthorAccountId: accountID,
		Provider:        provider,
		ProviderUserId:  user.ProviderUserID,
		Email:           user.Email,
		AvatarUrl:       user.AvatarURL,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to create oauth account link: %w", err)
	}

	if err := txRunner.Commit(); err != nil {
		return 0, "", fmt.Errorf("failed to commit: %w", err)
	}

	return accountID, accountPublicID, nil
}

`)
	} else {
		// SignupEnabled: false — find-only mode; reject if no account exists
		buf.WriteString(`// errNoAccount is returned when an OAuth login finds no existing account
// and signup is not enabled.
var errNoAccount = fmt.Errorf("no account found for this email; signup is not enabled")

// findOrCreateOAuthAccount looks up an account by OAuth provider+ID first,
// then by email. If no account exists, it returns errNoAccount because
// signup is not enabled.
func findOrCreateOAuthAccount(ctx context.Context, runner queries.Runner, provider string, user oauthUser) (int64, string, error) {
	// 1. Check if this provider+user_id is already linked to an account
	oauthAcct, err := runner.FindOAuthAccount(ctx, queries.FindOAuthAccountParams{
		Provider:       provider,
		ProviderUserId: user.ProviderUserID,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to lookup oauth account: %w", err)
	}
	if oauthAcct != nil {
		// Already linked — return the associated account
		return oauthAcct.AccountId, oauthAcct.PublicId, nil
	}

	// 2. Check if an account with this email already exists
	existing, err := runner.FindAccountByEmail(ctx, queries.FindAccountByEmailParams{
		Email: user.Email,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to lookup account: %w", err)
	}

	if existing == nil {
		// No account found and signup is not enabled — reject.
		return 0, "", errNoAccount
	}

	// Account exists — link this OAuth provider to it
	_, err = runner.CreateOAuthAccount(ctx, queries.CreateOAuthAccountParams{
		PublicId:        nanoid.New(),
		AccountId:       existing.Id,
		AuthorAccountId: existing.Id,
		Provider:        provider,
		ProviderUserId:  user.ProviderUserID,
		Email:           user.Email,
		AvatarUrl:       user.AvatarURL,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to create oauth account link: %w", err)
	}

	return existing.Id, existing.PublicId, nil
}

`)
	}

	// createOAuthSession
	buf.WriteString(`// createOAuthSession creates a session and sets the session cookie.
// This reuses the exact same cookie machinery as email/password login.
func createOAuthSession(ctx context.Context, w http.ResponseWriter, runner queries.Runner, accountID int64) error {
	session, err := runner.SignupCreateSession(ctx, queries.SignupCreateSessionParams{
		PublicId:  nanoid.New(),
		AccountId: accountID,
		ExpiresAt: time.Now().UTC().Add(14 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	signed := crypto.SignCookie(session.PublicId, []byte(config.Settings.COOKIE_SECRET))
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.Settings.GO_ENV != "development",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   14 * 24 * 60 * 60,
	})

	return nil
}

`)

	// oauthRedirectURL
	buf.WriteString(`// oauthRedirectURL returns the post-login redirect URL.
func oauthRedirectURL() string {
	if u := config.Settings.OAUTH_REDIRECT_URL; u != "" {
		return u
	}
	return "/"
}
`)

	return formatSource(buf.Bytes())
}

// GenerateOAuthProvider generates the provider-specific OAuth handler file
// (e.g., oauth_google.go or oauth_github.go). It contains the Login redirect
// handler, the Callback handler, and the fetchUser function.
func GenerateOAuthProvider(cfg AuthGenConfig, provider OAuthProviderDef) ([]byte, error) {
	switch provider.Name {
	case "google":
		return generateGoogleProvider(cfg, provider)
	case "github":
		return generateGitHubProvider(cfg, provider)
	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider.Name)
	}
}

// generateGoogleProvider generates oauth_google.go.
func generateGoogleProvider(cfg AuthGenConfig, provider OAuthProviderDef) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"encoding/json\"\n")
	if !cfg.SignupEnabled {
		buf.WriteString("\t\"errors\"\n")
	}
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"io\"\n")
	buf.WriteString("\t\"net/http\"\n")
	buf.WriteString("\t\"net/url\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/config"))
	buf.WriteString(")\n\n")

	// GoogleLogin
	buf.WriteString(`// GoogleLogin redirects the user to Google's OAuth consent screen.
func GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState(w)
	if err != nil {
		config.Logger.Error("google oauth: failed to generate state", "error", err.Error())
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}

	params := url.Values{}
	params.Set("client_id", config.Settings.GOOGLE_CLIENT_ID)
	params.Set("redirect_uri", config.Settings.OAUTH_REDIRECT_BASE_URL+"/auth/google/callback")
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("access_type", "online")

`)
	fmt.Fprintf(&buf, "\thttp.Redirect(w, r, %q+\"?\"+params.Encode(), http.StatusFound)\n", provider.AuthURL)
	buf.WriteString("}\n\n")

	// GoogleCallback
	buf.WriteString(`// GoogleCallback handles the OAuth callback from Google.
func GoogleCallback(w http.ResponseWriter, r *http.Request, runner queries.Runner) {
	// Verify CSRF state
	if err := verifyOAuthState(r, r.URL.Query().Get("state")); err != nil {
		config.Logger.Error("google oauth: invalid state", "error", err.Error())
		http.Error(w, "invalid state", http.StatusForbidden)
		return
	}
	clearOAuthStateCookie(w)

	// Check for errors from Google
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Error(w, "oauth error: "+errMsg, http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for access token
	redirectURI := config.Settings.OAUTH_REDIRECT_BASE_URL + "/auth/google/callback"
	accessToken, err := exchangeOAuthCode(
`)
	fmt.Fprintf(&buf, "\t\t%q,\n", provider.TokenURL)
	buf.WriteString(`		code,
		config.Settings.GOOGLE_CLIENT_ID,
		config.Settings.GOOGLE_CLIENT_SECRET,
		redirectURI,
	)
	if err != nil {
		config.Logger.Error("google oauth: token exchange failed", "error", err.Error())
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// Fetch user info
	user, err := fetchGoogleUser(accessToken)
	if err != nil {
		config.Logger.Error("google oauth: failed to get user info", "error", err.Error())
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	// Find or create account, create session
	accountID, _, err := findOrCreateOAuthAccount(r.Context(), runner, "google", *user)
	if err != nil {
`)
	if !cfg.SignupEnabled {
		buf.WriteString(`		if errors.Is(err, errNoAccount) {
			target := oauthRedirectURL() + "?error=no_account"
			http.Redirect(w, r, target, http.StatusFound)
		} else {
			config.Logger.Error("google oauth: findOrCreateOAuthAccount failed", "error", err.Error())
			http.Error(w, "failed to create account", http.StatusInternalServerError)
		}
`)
	} else {
		buf.WriteString(`		config.Logger.Error("google oauth: findOrCreateOAuthAccount failed", "error", err.Error())
		http.Error(w, "failed to create account", http.StatusInternalServerError)
`)
	}
	buf.WriteString(`		return
	}

	if err := createOAuthSession(r.Context(), w, runner, accountID); err != nil {
		config.Logger.Error("google oauth: failed to create session", "error", err.Error())
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, oauthRedirectURL(), http.StatusFound)
}

`)

	// fetchGoogleUser
	buf.WriteString(`// fetchGoogleUser calls the Google userinfo endpoint and returns
// normalised user info.
func fetchGoogleUser(accessToken string) (*oauthUser, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info struct {
		ID            string ` + "`json:\"id\"`" + `
		Email         string ` + "`json:\"email\"`" + `
		VerifiedEmail bool   ` + "`json:\"verified_email\"`" + `
		Name          string ` + "`json:\"name\"`" + `
		GivenName     string ` + "`json:\"given_name\"`" + `
		FamilyName    string ` + "`json:\"family_name\"`" + `
		Picture       string ` + "`json:\"picture\"`" + `
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}
	if info.Email == "" {
		return nil, fmt.Errorf("google did not return an email")
	}
	if info.ID == "" {
		return nil, fmt.Errorf("google did not return a user ID")
	}

	return &oauthUser{
		Email:          info.Email,
		FirstName:      info.GivenName,
		LastName:       info.FamilyName,
		ProviderUserID: info.ID,
		AvatarURL:      info.Picture,
	}, nil
}
`)

	return formatSource(buf.Bytes())
}

// generateGitHubProvider generates oauth_github.go.
func generateGitHubProvider(cfg AuthGenConfig, provider OAuthProviderDef) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"encoding/json\"\n")
	if !cfg.SignupEnabled {
		buf.WriteString("\t\"errors\"\n")
	}
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"io\"\n")
	buf.WriteString("\t\"net/http\"\n")
	buf.WriteString("\t\"net/url\"\n")
	buf.WriteString("\t\"strings\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/config"))
	buf.WriteString(")\n\n")

	// GitHubLogin
	buf.WriteString(`// GitHubLogin redirects the user to GitHub's OAuth consent screen.
func GitHubLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState(w)
	if err != nil {
		config.Logger.Error("github oauth: failed to generate state", "error", err.Error())
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}

	params := url.Values{}
	params.Set("client_id", config.Settings.GITHUB_CLIENT_ID)
	params.Set("redirect_uri", config.Settings.OAUTH_REDIRECT_BASE_URL+"/auth/github/callback")
	params.Set("scope", "user:email")
	params.Set("state", state)

`)
	fmt.Fprintf(&buf, "\thttp.Redirect(w, r, %q+\"?\"+params.Encode(), http.StatusFound)\n", provider.AuthURL)
	buf.WriteString("}\n\n")

	// GitHubCallback
	buf.WriteString(`// GitHubCallback handles the OAuth callback from GitHub.
func GitHubCallback(w http.ResponseWriter, r *http.Request, runner queries.Runner) {
	// Verify CSRF state
	if err := verifyOAuthState(r, r.URL.Query().Get("state")); err != nil {
		config.Logger.Error("github oauth: invalid state", "error", err.Error())
		http.Error(w, "invalid state", http.StatusForbidden)
		return
	}
	clearOAuthStateCookie(w)

	// Check for errors from GitHub
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Error(w, "oauth error: "+errMsg, http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for access token
	redirectURI := config.Settings.OAUTH_REDIRECT_BASE_URL + "/auth/github/callback"
	accessToken, err := exchangeOAuthCode(
`)
	fmt.Fprintf(&buf, "\t\t%q,\n", provider.TokenURL)
	buf.WriteString(`		code,
		config.Settings.GITHUB_CLIENT_ID,
		config.Settings.GITHUB_CLIENT_SECRET,
		redirectURI,
	)
	if err != nil {
		config.Logger.Error("github oauth: token exchange failed", "error", err.Error())
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// Fetch user info
	user, err := fetchGitHubUser(accessToken)
	if err != nil {
		config.Logger.Error("github oauth: failed to get user info", "error", err.Error())
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	// Find or create account, create session
	accountID, _, err := findOrCreateOAuthAccount(r.Context(), runner, "github", *user)
	if err != nil {
`)
	if !cfg.SignupEnabled {
		buf.WriteString(`		if errors.Is(err, errNoAccount) {
			target := oauthRedirectURL() + "?error=no_account"
			http.Redirect(w, r, target, http.StatusFound)
		} else {
			config.Logger.Error("github oauth: findOrCreateOAuthAccount failed", "error", err.Error())
			http.Error(w, "failed to create account", http.StatusInternalServerError)
		}
`)
	} else {
		buf.WriteString(`		config.Logger.Error("github oauth: findOrCreateOAuthAccount failed", "error", err.Error())
		http.Error(w, "failed to create account", http.StatusInternalServerError)
`)
	}
	buf.WriteString(`		return
	}

	if err := createOAuthSession(r.Context(), w, runner, accountID); err != nil {
		config.Logger.Error("github oauth: failed to create session", "error", err.Error())
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, oauthRedirectURL(), http.StatusFound)
}

`)

	// fetchGitHubUser
	buf.WriteString(`// fetchGitHubUser calls the GitHub user API and normalises the response.
// Ground truth from E2E tests:
//   - id is a number (float64 in JSON), e.g., 84721070
//   - name can be null (not just empty)
//   - email is null when the user's email is private
//   - /user/emails always has the primary verified email
func fetchGitHubUser(accessToken string) (*oauthUser, error) {
	// 1. GET https://api.github.com/user
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 2. Parse id, login, name, email, avatar_url
	var info struct {
		ID        float64 ` + "`json:\"id\"`" + `
		Login     string  ` + "`json:\"login\"`" + `
		Name      *string ` + "`json:\"name\"`" + `
		Email     *string ` + "`json:\"email\"`" + `
		AvatarURL string  ` + "`json:\"avatar_url\"`" + `
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub user info: %w", err)
	}
	if info.ID == 0 {
		return nil, fmt.Errorf("github did not return a user ID")
	}
	if info.Login == "" {
		return nil, fmt.Errorf("github did not return a login")
	}

	// 3. Always call /user/emails for the email (cannot rely on /user.email)
	email, err := fetchGitHubPrimaryEmail(accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub email: %w", err)
	}

	// 4. Split "name" into firstName/lastName on the first space
	//    Fallback: firstName = login, lastName = ""
	firstName := info.Login
	lastName := ""
	if info.Name != nil && *info.Name != "" {
		parts := strings.SplitN(*info.Name, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	// 5. Return oauthUser with ProviderUserID = fmt.Sprintf("%.0f", id)
	return &oauthUser{
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		ProviderUserID: fmt.Sprintf("%.0f", info.ID),
		AvatarURL:      info.AvatarURL,
	}, nil
}

// fetchGitHubPrimaryEmail calls /user/emails and returns the primary
// verified email address. This is necessary because /user may return
// email: null when the user's email is private (confirmed by E2E tests).
func fetchGitHubPrimaryEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []struct {
		Email    string ` + "`json:\"email\"`" + `
		Primary  bool   ` + "`json:\"primary\"`" + `
		Verified bool   ` + "`json:\"verified\"`" + `
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("failed to parse GitHub emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified && e.Email != "" {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no primary verified email found in GitHub /user/emails")
}
`)

	return formatSource(buf.Bytes())
}

// oauthProviderFileName returns the filename for a provider-specific OAuth handler file.
func oauthProviderFileName(providerName string) string {
	return "oauth_" + providerName + ".go"
}

// titleCase returns the first-letter-capitalized version of s for use in
// function names like "GoogleLogin" or "GitHubLogin".
func providerTitle(name string) string {
	p := ProviderByName(name)
	if p != nil {
		return p.DisplayName
	}
	if len(name) == 0 {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
