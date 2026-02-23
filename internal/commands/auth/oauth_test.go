package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/inifile"
)

func TestValidateOAuthProvider_RejectsUnknown(t *testing.T) {
	for _, name := range []string{"discord", "apple", "facebook", ""} {
		err := ValidateOAuthProvider(name)
		if err == nil {
			t.Errorf("ValidateOAuthProvider(%q) = nil, want error", name)
		}
		if err != nil && !strings.Contains(err.Error(), "unknown OAuth provider") {
			t.Errorf("ValidateOAuthProvider(%q) error = %v, want 'unknown OAuth provider' message", name, err)
		}
	}
}

func TestValidateOAuthProvider_AcceptsKnown(t *testing.T) {
	for _, name := range []string{"google", "github"} {
		err := ValidateOAuthProvider(name)
		if err != nil {
			t.Errorf("ValidateOAuthProvider(%q) = %v, want nil", name, err)
		}
	}
}

func TestSetOAuthIniFlags_SetAndRead(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "shipq.ini")

	// Start with a minimal ini file that has an [auth] section
	initial := "[db]\ndatabase_url = sqlite:///tmp/test.db\n\n[auth]\nprotect_by_default = true\ncookie_secret = abc123\n"
	if err := os.WriteFile(iniPath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial ini: %v", err)
	}

	ini, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	changed := SetOAuthIniFlags(ini, "google")
	if !changed {
		t.Error("SetOAuthIniFlags returned false on first call, want true")
	}

	if err := ini.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write ini: %v", err)
	}

	// Re-parse and verify the flags are set
	ini2, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to re-parse ini: %v", err)
	}

	if got := ini2.Get("auth", "oauth_google"); strings.ToLower(got) != "true" {
		t.Errorf("oauth_google = %q, want %q", got, "true")
	}
	if got := ini2.Get("auth", "oauth_redirect_url"); got == "" {
		t.Error("oauth_redirect_url is empty, want a default value")
	}
	if got := ini2.Get("auth", "oauth_redirect_base_url"); got == "" {
		t.Error("oauth_redirect_base_url is empty, want a default value")
	}

	// Original values should be preserved
	if got := ini2.Get("auth", "protect_by_default"); got != "true" {
		t.Errorf("protect_by_default = %q, want %q (should be preserved)", got, "true")
	}
	if got := ini2.Get("auth", "cookie_secret"); got != "abc123" {
		t.Errorf("cookie_secret = %q, want %q (should be preserved)", got, "abc123")
	}
}

func TestSetOAuthIniFlags_Idempotent(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "shipq.ini")

	initial := "[db]\ndatabase_url = sqlite:///tmp/test.db\n\n[auth]\nprotect_by_default = true\ncookie_secret = abc123\n"
	if err := os.WriteFile(iniPath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial ini: %v", err)
	}

	// First call: set the flags
	ini1, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}
	SetOAuthIniFlags(ini1, "google")
	if err := ini1.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write ini: %v", err)
	}

	// Read the file content after first write
	content1, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("failed to read ini: %v", err)
	}

	// Second call: same provider again
	ini2, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to re-parse ini: %v", err)
	}
	changed := SetOAuthIniFlags(ini2, "google")
	if changed {
		t.Error("SetOAuthIniFlags returned true on second call, want false (idempotent)")
	}
	if err := ini2.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write ini: %v", err)
	}

	// Read the file content after second write
	content2, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("failed to read ini: %v", err)
	}

	if string(content1) != string(content2) {
		t.Errorf("ini file changed on second SetOAuthIniFlags call:\n--- first ---\n%s\n--- second ---\n%s", content1, content2)
	}
}

func TestSetOAuthIniFlags_MultipleProviders(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "shipq.ini")

	initial := "[auth]\nprotect_by_default = true\n"
	if err := os.WriteFile(iniPath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial ini: %v", err)
	}

	ini, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	// Set google first
	SetOAuthIniFlags(ini, "google")

	// Set github second
	changed := SetOAuthIniFlags(ini, "github")
	if !changed {
		t.Error("SetOAuthIniFlags returned false for github after google, want true")
	}

	if err := ini.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write ini: %v", err)
	}

	// Re-parse and verify both are set
	ini2, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to re-parse ini: %v", err)
	}

	if got := ini2.Get("auth", "oauth_google"); strings.ToLower(got) != "true" {
		t.Errorf("oauth_google = %q, want %q", got, "true")
	}
	if got := ini2.Get("auth", "oauth_github"); strings.ToLower(got) != "true" {
		t.Errorf("oauth_github = %q, want %q", got, "true")
	}

	// Redirect URLs should be set only once (not duplicated)
	content, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("failed to read ini: %v", err)
	}
	// Count exact key occurrences by splitting on newlines and checking prefixes
	lines := strings.Split(string(content), "\n")
	redirectURLCount := 0
	redirectBaseURLCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "oauth_redirect_url ") || strings.HasPrefix(trimmed, "oauth_redirect_url=") {
			redirectURLCount++
		}
		if strings.HasPrefix(trimmed, "oauth_redirect_base_url ") || strings.HasPrefix(trimmed, "oauth_redirect_base_url=") {
			redirectBaseURLCount++
		}
	}
	if redirectURLCount != 1 {
		t.Errorf("expected 1 occurrence of oauth_redirect_url, got %d in:\n%s", redirectURLCount, content)
	}
	if redirectBaseURLCount != 1 {
		t.Errorf("expected 1 occurrence of oauth_redirect_base_url, got %d in:\n%s", redirectBaseURLCount, content)
	}
}

func TestSetOAuthIniFlags_DoesNotOverrideExistingRedirectURL(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "shipq.ini")

	initial := "[auth]\nprotect_by_default = true\noauth_redirect_url = https://myapp.com\noauth_redirect_base_url = https://api.myapp.com\n"
	if err := os.WriteFile(iniPath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial ini: %v", err)
	}

	ini, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	SetOAuthIniFlags(ini, "google")
	if err := ini.WriteFile(iniPath); err != nil {
		t.Fatalf("failed to write ini: %v", err)
	}

	ini2, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to re-parse ini: %v", err)
	}

	if got := ini2.Get("auth", "oauth_redirect_url"); got != "https://myapp.com" {
		t.Errorf("oauth_redirect_url = %q, want %q (should not be overwritten)", got, "https://myapp.com")
	}
	if got := ini2.Get("auth", "oauth_redirect_base_url"); got != "https://api.myapp.com" {
		t.Errorf("oauth_redirect_base_url = %q, want %q (should not be overwritten)", got, "https://api.myapp.com")
	}
}

func TestEnabledOAuthProviders_Empty(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[auth]\nprotect_by_default = true\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	providers := EnabledOAuthProviders(ini)
	if len(providers) != 0 {
		t.Errorf("EnabledOAuthProviders() = %v, want empty", providers)
	}
}

func TestEnabledOAuthProviders_GoogleOnly(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[auth]\noauth_google = true\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	providers := EnabledOAuthProviders(ini)
	if len(providers) != 1 || providers[0] != "google" {
		t.Errorf("EnabledOAuthProviders() = %v, want [google]", providers)
	}
}

func TestEnabledOAuthProviders_Both(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[auth]\noauth_google = true\noauth_github = true\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	providers := EnabledOAuthProviders(ini)
	if len(providers) != 2 {
		t.Fatalf("EnabledOAuthProviders() = %v, want 2 providers", providers)
	}

	found := make(map[string]bool)
	for _, p := range providers {
		found[p] = true
	}
	if !found["google"] {
		t.Error("expected google in providers")
	}
	if !found["github"] {
		t.Error("expected github in providers")
	}
}

func TestEnabledOAuthProviders_CaseInsensitive(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[auth]\noauth_google = True\noauth_github = TRUE\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	providers := EnabledOAuthProviders(ini)
	if len(providers) != 2 {
		t.Errorf("EnabledOAuthProviders() = %v, want 2 providers (case insensitive)", providers)
	}
}

func TestEnabledOAuthProviders_FalseValues(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[auth]\noauth_google = false\noauth_github = no\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	providers := EnabledOAuthProviders(ini)
	if len(providers) != 0 {
		t.Errorf("EnabledOAuthProviders() = %v, want empty (false values)", providers)
	}
}
