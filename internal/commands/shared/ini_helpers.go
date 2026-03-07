package shared

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen/authgen"
	"github.com/shipq/shipq/inifile"
)

// EnabledOAuthProviders reads [auth] oauth_<name> flags from the ini file and
// returns the list of enabled provider names.
func EnabledOAuthProviders(ini *inifile.File) []string {
	var providers []string
	for _, name := range authgen.AllProviderNames() {
		if strings.ToLower(ini.Get("auth", "oauth_"+name)) == "true" {
			providers = append(providers, name)
		}
	}
	return providers
}

// IsOAuthGoogleEnabled returns true if oauth_google is set to "true" in [auth].
func IsOAuthGoogleEnabled(ini *inifile.File) bool {
	return strings.ToLower(ini.Get("auth", "oauth_google")) == "true"
}

// IsOAuthGitHubEnabled returns true if oauth_github is set to "true" in [auth].
func IsOAuthGitHubEnabled(ini *inifile.File) bool {
	return strings.ToLower(ini.Get("auth", "oauth_github")) == "true"
}

// IsFeatureEnabled returns true if the given section exists in the ini file.
// This is used to detect whether [files], [workers], [email], or [auth]
// features are configured.
func IsFeatureEnabled(ini *inifile.File, section string) bool {
	return ini.Section(section) != nil
}

// IsSignupEnabled checks whether signup has been configured by looking for
// the existence of api/auth/signup.go in the project.
func IsSignupEnabled(shipqRoot string) bool {
	signupPath := filepath.Join(shipqRoot, "api", "auth", "signup.go")
	_, err := os.Stat(signupPath)
	return err == nil
}
