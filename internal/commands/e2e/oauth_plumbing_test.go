//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const oauthCallbackPort = "9876"

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// oauthCallbackServer starts a local HTTP server on localhost:9876 that
// listens for the OAuth callback. It captures the `code` and `state` query
// parameters and sends them on the returned channels. The callback handler
// responds with a small HTML page telling the user to close the tab.
//
// Returns: codeCh, stateCh, baseURL, shutdown function.
func oauthCallbackServer(t *testing.T, callbackPath string) (chan string, chan string, string, func()) {
	t.Helper()

	codeCh := make(chan string, 1)
	stateCh := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			t.Logf("OAuth callback received error: %s (description: %s)",
				errMsg, r.URL.Query().Get("error_description"))
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><h1>OAuth Error</h1><p>%s</p><p>You can close this tab.</p></body></html>`, errMsg)
			// Send empty code so the test unblocks and can fail
			codeCh <- ""
			stateCh <- state
			return
		}

		t.Logf("OAuth callback received code=%q state=%q", code, state)

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h1>Login complete</h1><p>You can close this tab.</p></body></html>`)

		codeCh <- code
		stateCh <- state
	})

	server := &http.Server{
		Addr:    ":" + oauthCallbackPort,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("OAuth callback server error: %v", err)
		}
	}()

	// Give the server a moment to start listening.
	time.Sleep(50 * time.Millisecond)

	baseURL := "http://localhost:" + oauthCallbackPort
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}

	return codeCh, stateCh, baseURL, shutdown
}

// openBrowser attempts to open the given URL in the default browser on macOS.
// It logs the URL regardless, so the tester can always click it manually.
// Does not fail the test if the open command errors.
func openBrowser(t *testing.T, url string) {
	t.Helper()
	t.Logf("\n╔══════════════════════════════════════════════════════════════╗")
	t.Logf("║  Please log in at the URL below (or it may open in your    ║")
	t.Logf("║  browser automatically):                                   ║")
	t.Logf("╚══════════════════════════════════════════════════════════════╝")
	t.Logf("\n  %s\n", url)

	cmd := exec.Command("open", url)
	if err := cmd.Run(); err != nil {
		t.Logf("Could not open browser automatically: %v (use the URL above)", err)
	}
}

// skipIfGoogleCredsAbsent skips the test if GOOGLE_CLIENT_ID or
// GOOGLE_CLIENT_SECRET environment variables are not set.
func skipIfGoogleCredsAbsent(t *testing.T) {
	t.Helper()
	if os.Getenv("GOOGLE_CLIENT_ID") == "" || os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
		t.Skip("Skipping: GOOGLE_CLIENT_ID and/or GOOGLE_CLIENT_SECRET not set")
	}
}

// skipIfGitHubCredsAbsent skips the test if GITHUB_CLIENT_ID or
// GITHUB_CLIENT_SECRET environment variables are not set.
func skipIfGitHubCredsAbsent(t *testing.T) {
	t.Helper()
	if os.Getenv("GITHUB_CLIENT_ID") == "" || os.Getenv("GITHUB_CLIENT_SECRET") == "" {
		t.Skip("Skipping: GITHUB_CLIENT_ID and/or GITHUB_CLIENT_SECRET not set")
	}
}

// generateState generates a random hex-encoded state string for CSRF protection.
func generateState(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("failed to generate random state: %v", err)
	}
	return hex.EncodeToString(b)
}

// exchangeCodeForToken POSTs to the token endpoint with
// grant_type=authorization_code and returns the full parsed JSON response.
// Fails the test if the response contains an error field or is missing
// access_token.
func exchangeCodeForToken(t *testing.T, tokenURL, code, clientID, clientSecret, redirectURI string) map[string]interface{} {
	t.Helper()

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		t.Fatalf("failed to create token exchange request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("token exchange request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read token response body: %v", err)
	}

	t.Logf("Token exchange response (status %d): %s", resp.StatusCode, string(body))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse token response JSON: %v\nBody: %s", err, string(body))
	}

	if errField, ok := result["error"]; ok {
		t.Fatalf("token exchange returned error: %v (description: %v)", errField, result["error_description"])
	}

	if _, ok := result["access_token"]; !ok {
		t.Fatalf("token response missing access_token field. Full response: %v", result)
	}

	accessToken, ok := result["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("access_token is not a non-empty string. Full response: %v", result)
	}

	return result
}

// exchangeCodeExpectError POSTs to the token endpoint and expects an error
// response. Returns the parsed JSON response. Does NOT fail if an error
// field is present — that's the expected case.
func exchangeCodeExpectError(t *testing.T, tokenURL, code, clientID, clientSecret, redirectURI string) map[string]interface{} {
	t.Helper()

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		t.Fatalf("failed to create token exchange request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("token exchange request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read token response body: %v", err)
	}

	t.Logf("Token exchange error response (status %d): %s", resp.StatusCode, string(body))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse token error response JSON: %v\nBody: %s", err, string(body))
	}

	return result
}

// fetchJSON GETs the given URL with a Bearer authorization header and
// returns the parsed JSON response. Fails the test on non-2xx status.
func fetchJSON(t *testing.T, url, bearerToken string) map[string]interface{} {
	t.Helper()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("failed to create GET request for %s: %v", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body from %s: %v", url, err)
	}

	t.Logf("GET %s (status %d): %s", url, resp.StatusCode, string(body))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("GET %s returned non-2xx status %d. Body: %s", url, resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON from %s: %v\nBody: %s", url, err, string(body))
	}

	return result
}

// fetchJSONArray GETs the given URL with a Bearer authorization header and
// returns the parsed JSON array. Fails the test on non-2xx status or if
// the response is not an array.
func fetchJSONArray(t *testing.T, url, bearerToken string) []interface{} {
	t.Helper()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("failed to create GET request for %s: %v", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body from %s: %v", url, err)
	}

	t.Logf("GET %s (status %d): %s", url, resp.StatusCode, string(body))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("GET %s returned non-2xx status %d. Body: %s", url, resp.StatusCode, string(body))
	}

	var result []interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON array from %s: %v\nBody: %s", url, err, string(body))
	}

	return result
}

// assertStringField asserts that the given map has a non-empty string at the
// given key.
func assertStringField(t *testing.T, m map[string]interface{}, key string) string {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("expected key %q to exist in response, but it was missing. Full response: %v", key, m)
		return ""
	}
	s, ok := v.(string)
	if !ok {
		t.Errorf("expected key %q to be a string, got %T (%v)", key, v, v)
		return ""
	}
	if s == "" {
		t.Errorf("expected key %q to be non-empty string", key)
	}
	return s
}

// assertKeyExists asserts that the given key exists in the map (value may be
// nil or any type).
func assertKeyExists(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()
	if _, ok := m[key]; !ok {
		t.Errorf("expected key %q to exist in response, but it was missing. Keys present: %v", key, mapKeys(m))
	}
}

// mapKeys returns the keys of a map for diagnostic output.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Google Tests
// ---------------------------------------------------------------------------

func TestPlumbing_Google_FullOAuthFlow(t *testing.T) {
	skipIfGoogleCredsAbsent(t)

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	callbackPath := "/auth/google/callback"
	redirectURI := "http://localhost:" + oauthCallbackPort + callbackPath

	// 1. Start callback server
	codeCh, stateCh, _, shutdown := oauthCallbackServer(t, callbackPath)
	defer shutdown()

	// 2. Build authorization URL
	state := generateState(t)
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("access_type", "online")

	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()

	// 3. Open browser
	openBrowser(t, authURL)

	// 4. Wait for the callback (2-minute timeout for user to log in)
	t.Log("Waiting for you to complete Google login in the browser...")
	var code, returnedState string
	select {
	case code = <-codeCh:
	case <-time.After(2 * time.Minute):
		t.Fatal("Timed out waiting for Google OAuth callback (2 minutes). Did you complete login in the browser?")
	}
	select {
	case returnedState = <-stateCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for state from callback")
	}

	// 5. Verify state round-trips
	if returnedState != state {
		t.Fatalf("State mismatch: sent %q, got back %q", state, returnedState)
	}
	t.Log("✓ State parameter round-tripped correctly")

	if code == "" {
		t.Fatal("Received empty authorization code from callback (OAuth error?)")
	}
	t.Logf("✓ Received authorization code: %s...", code[:min(len(code), 20)])

	// 6. Exchange code for access token
	tokenResp := exchangeCodeForToken(t, "https://oauth2.googleapis.com/token",
		code, clientID, clientSecret, redirectURI)

	accessToken := tokenResp["access_token"].(string)
	t.Logf("✓ Got access token: %s...", accessToken[:min(len(accessToken), 20)])

	// Verify token_type
	if tt, ok := tokenResp["token_type"].(string); ok {
		if !strings.EqualFold(tt, "Bearer") {
			t.Errorf("Expected token_type 'Bearer', got %q", tt)
		} else {
			t.Log("✓ token_type is Bearer")
		}
	} else {
		t.Error("token_type missing or not a string in token response")
	}

	// 7. Fetch user info
	userInfo := fetchJSON(t, "https://www.googleapis.com/oauth2/v2/userinfo", accessToken)

	// 8. Assert JSON shape
	t.Log("Verifying Google userinfo response shape...")

	id := assertStringField(t, userInfo, "id")
	t.Logf("  id: %s", id)

	email := assertStringField(t, userInfo, "email")
	t.Logf("  email: %s", email)

	// given_name and family_name should exist as keys (may be empty for some accounts)
	assertKeyExists(t, userInfo, "given_name")
	if gn, ok := userInfo["given_name"].(string); ok {
		t.Logf("  given_name: %s", gn)
	}

	assertKeyExists(t, userInfo, "family_name")
	if fn, ok := userInfo["family_name"].(string); ok {
		t.Logf("  family_name: %s", fn)
	}

	// picture: should be a string URL (may be absent for some accounts, but
	// typically present)
	if pic, ok := userInfo["picture"].(string); ok {
		t.Logf("  picture: %s", pic)
	} else {
		t.Log("  picture: (not present or not a string — this is okay for some accounts)")
	}

	// verified_email: check if present
	if ve, ok := userInfo["verified_email"]; ok {
		t.Logf("  verified_email: %v", ve)
	}

	t.Log("✓ Google userinfo response shape verified")
	t.Logf("Full userinfo response: %v", userInfo)
}

func TestPlumbing_Google_TokenExchangeRejectsInvalidCode(t *testing.T) {
	skipIfGoogleCredsAbsent(t)

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURI := "http://localhost:" + oauthCallbackPort + "/auth/google/callback"

	// POST to Google's token endpoint with an invalid code.
	result := exchangeCodeExpectError(t,
		"https://oauth2.googleapis.com/token",
		"invalid_code_that_does_not_exist",
		clientID, clientSecret, redirectURI,
	)

	// Google should return an error field (typically "invalid_grant").
	errField, ok := result["error"]
	if !ok {
		t.Fatalf("Expected error field in response, got: %v", result)
	}

	errStr, ok := errField.(string)
	if !ok {
		t.Fatalf("Expected error to be a string, got %T: %v", errField, errField)
	}

	t.Logf("✓ Google rejected invalid code with error: %q", errStr)

	if errStr != "invalid_grant" {
		t.Logf("Note: expected 'invalid_grant' but got %q — this may be fine, Google may use different error codes", errStr)
	}

	// Check for error_description if present
	if desc, ok := result["error_description"]; ok {
		t.Logf("  error_description: %v", desc)
	}
}

// ---------------------------------------------------------------------------
// GitHub Tests
// ---------------------------------------------------------------------------

func TestPlumbing_GitHub_FullOAuthFlow(t *testing.T) {
	skipIfGitHubCredsAbsent(t)

	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	callbackPath := "/auth/github/callback"
	redirectURI := "http://localhost:" + oauthCallbackPort + callbackPath

	// 1. Start callback server
	codeCh, stateCh, _, shutdown := oauthCallbackServer(t, callbackPath)
	defer shutdown()

	// 2. Build authorization URL
	state := generateState(t)
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "user:email")
	params.Set("state", state)

	authURL := "https://github.com/login/oauth/authorize?" + params.Encode()

	// 3. Open browser
	openBrowser(t, authURL)

	// 4. Wait for the callback (2-minute timeout for user to log in)
	t.Log("Waiting for you to complete GitHub login in the browser...")
	var code, returnedState string
	select {
	case code = <-codeCh:
	case <-time.After(2 * time.Minute):
		t.Fatal("Timed out waiting for GitHub OAuth callback (2 minutes). Did you complete login in the browser?")
	}
	select {
	case returnedState = <-stateCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for state from callback")
	}

	// 5. Verify state round-trips
	if returnedState != state {
		t.Fatalf("State mismatch: sent %q, got back %q", state, returnedState)
	}
	t.Log("✓ State parameter round-tripped correctly")

	if code == "" {
		t.Fatal("Received empty authorization code from callback (OAuth error?)")
	}
	t.Logf("✓ Received authorization code: %s...", code[:min(len(code), 20)])

	// 6. Exchange code for access token
	// IMPORTANT: GitHub requires Accept: application/json header, otherwise
	// it returns the token as form-encoded (application/x-www-form-urlencoded).
	tokenResp := exchangeCodeForToken(t, "https://github.com/login/oauth/access_token",
		code, clientID, clientSecret, redirectURI)

	accessToken := tokenResp["access_token"].(string)
	t.Logf("✓ Got access token: %s...", accessToken[:min(len(accessToken), 20)])

	// Check token_type if present
	if tt, ok := tokenResp["token_type"].(string); ok {
		t.Logf("  token_type: %s", tt)
	}

	// Check scope if present
	if scope, ok := tokenResp["scope"].(string); ok {
		t.Logf("  scope: %s", scope)
	}

	// 7. Fetch user info from /user
	userInfo := fetchJSON(t, "https://api.github.com/user", accessToken)

	t.Log("Verifying GitHub /user response shape...")

	// id: GitHub returns this as a number (float64 in JSON)
	if idVal, ok := userInfo["id"]; ok {
		switch v := idVal.(type) {
		case float64:
			t.Logf("  id: %.0f (number — as expected)", v)
		case string:
			t.Logf("  id: %s (string — UNEXPECTED, plan assumed number)", v)
			t.Error("GitHub returned id as string; the plan expected a number. Update fetchGitHubUser to handle this.")
		default:
			t.Errorf("  id: unexpected type %T: %v", v, v)
		}
	} else {
		t.Error("  id: missing from response")
	}

	// login: non-empty string
	login := assertStringField(t, userInfo, "login")
	t.Logf("  login: %s", login)

	// name: string or null
	assertKeyExists(t, userInfo, "name")
	if name, ok := userInfo["name"].(string); ok {
		t.Logf("  name: %s", name)
	} else {
		t.Log("  name: null (this is valid — some GitHub accounts have no name set)")
	}

	// avatar_url: string URL
	avatarURL := assertStringField(t, userInfo, "avatar_url")
	t.Logf("  avatar_url: %s", avatarURL)

	// email: string or null — GitHub may return null if the user's email is private
	assertKeyExists(t, userInfo, "email")
	if email, ok := userInfo["email"].(string); ok && email != "" {
		t.Logf("  email: %s (available directly from /user)", email)
	} else {
		t.Log("  email: null or empty (private email — will need /user/emails fallback)")
	}

	t.Log("✓ GitHub /user response shape verified")
	t.Logf("Full /user response: %v", userInfo)

	// 8. Fetch /user/emails — this is the fallback for private emails
	t.Log("Fetching /user/emails for email fallback verification...")
	emails := fetchJSONArray(t, "https://api.github.com/user/emails", accessToken)

	if len(emails) == 0 {
		t.Fatal("  /user/emails returned empty array — expected at least one email")
	}
	t.Logf("  /user/emails returned %d email(s)", len(emails))

	// Find the primary verified email
	var primaryEmail string
	for _, entry := range emails {
		emailObj, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		isPrimary, _ := emailObj["primary"].(bool)
		isVerified, _ := emailObj["verified"].(bool)
		emailAddr, _ := emailObj["email"].(string)

		t.Logf("  - email=%q primary=%v verified=%v", emailAddr, isPrimary, isVerified)

		if isPrimary && isVerified && emailAddr != "" {
			primaryEmail = emailAddr
		}
	}

	if primaryEmail == "" {
		t.Error("No primary+verified email found in /user/emails response")
	} else {
		t.Logf("✓ Found primary verified email: %s", primaryEmail)
	}

	t.Log("✓ GitHub /user/emails response verified")
}

func TestPlumbing_GitHub_TokenExchangeRejectsInvalidCode(t *testing.T) {
	skipIfGitHubCredsAbsent(t)

	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	redirectURI := "http://localhost:" + oauthCallbackPort + "/auth/github/callback"

	// POST to GitHub's token endpoint with an invalid code.
	// GitHub is special: even with an invalid code, it returns HTTP 200 with
	// an error field in the JSON body (when Accept: application/json is set).
	result := exchangeCodeExpectError(t,
		"https://github.com/login/oauth/access_token",
		"invalid_code_that_does_not_exist",
		clientID, clientSecret, redirectURI,
	)

	// GitHub should return an error field (typically "bad_verification_code").
	errField, ok := result["error"]
	if !ok {
		// If there's no error field, check if there's an access_token
		// (which would be very surprising for an invalid code).
		if _, hasToken := result["access_token"]; hasToken {
			t.Fatalf("GitHub returned an access_token for an invalid code — this is unexpected! Response: %v", result)
		}
		t.Fatalf("Expected error field in response, got: %v", result)
	}

	errStr, ok := errField.(string)
	if !ok {
		t.Fatalf("Expected error to be a string, got %T: %v", errField, errField)
	}

	t.Logf("✓ GitHub rejected invalid code with error: %q", errStr)

	if errStr != "bad_verification_code" {
		t.Logf("Note: expected 'bad_verification_code' but got %q — this may be fine, GitHub may use different error codes", errStr)
	}

	// Check for error_description if present
	if desc, ok := result["error_description"]; ok {
		t.Logf("  error_description: %v", desc)
	}

	// Check for error_uri if present (GitHub includes this)
	if uri, ok := result["error_uri"]; ok {
		t.Logf("  error_uri: %v", uri)
	}
}
