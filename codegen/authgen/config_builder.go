package authgen

import (
	"strings"

	"github.com/shipq/shipq/inifile"
)

// AuthGenConfigParams holds the parameters needed to build an AuthGenConfig.
// This replaces the 4+ copy-pasted blocks in auth.go, oauth.go, email.go,
// and signup.go that each manually constructed AuthGenConfig.
type AuthGenConfigParams struct {
	ModulePath      string
	Dialect         string
	TestDatabaseURL string
	ScopeColumn     string
	OAuthProviders  []string
	SignupEnabled   bool
	EmailEnabled    bool
}

// BuildAuthGenConfig constructs an AuthGenConfig from project configuration,
// ini flags, and file-existence checks. This centralises the logic that was
// previously duplicated across auth.go, oauth.go, email.go, and signup.go.
//
// Callers that need to override individual fields (e.g. forcing
// SignupEnabled or EmailEnabled) can mutate the returned struct.
func BuildAuthGenConfig(params AuthGenConfigParams) AuthGenConfig {
	return AuthGenConfig{
		ModulePath:      params.ModulePath,
		Dialect:         params.Dialect,
		TestDatabaseURL: params.TestDatabaseURL,
		ScopeColumn:     params.ScopeColumn,
		OAuthProviders:  params.OAuthProviders,
		SignupEnabled:   params.SignupEnabled,
		EmailEnabled:    params.EmailEnabled,
	}
}

// BuildAuthGenConfigFromIni is a convenience constructor that reads OAuth
// providers and email-enabled state from the ini file. The caller must
// supply signupEnabled and the project-level fields.
func BuildAuthGenConfigFromIni(
	ini *inifile.File,
	modulePath, dialect, testDatabaseURL, scopeColumn string,
	signupEnabled bool,
) AuthGenConfig {
	oauthProviders := EnabledOAuthProvidersFromIni(ini)
	emailEnabled := ini.Section("email") != nil

	return AuthGenConfig{
		ModulePath:      modulePath,
		Dialect:         dialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     scopeColumn,
		OAuthProviders:  oauthProviders,
		SignupEnabled:   signupEnabled,
		EmailEnabled:    emailEnabled,
	}
}

// EnabledOAuthProvidersFromIni reads [auth] oauth_<name> flags from the ini
// file and returns the list of enabled provider names. This is the canonical
// implementation — the private copies in signup and email packages should
// delegate here.
func EnabledOAuthProvidersFromIni(ini *inifile.File) []string {
	var providers []string
	for _, name := range AllProviderNames() {
		if strings.ToLower(ini.Get("auth", "oauth_"+name)) == "true" {
			providers = append(providers, name)
		}
	}
	return providers
}

// GenerateRegisterFile generates the register.go file, choosing between the
// signup-aware and standard variants based on cfg.SignupEnabled. This replaces
// the duplicated conditional in oauth.go and email.go.
func GenerateRegisterFile(cfg AuthGenConfig) ([]byte, error) {
	if cfg.SignupEnabled {
		return GenerateSignupRegister(cfg)
	}
	return GenerateRegister(cfg)
}
