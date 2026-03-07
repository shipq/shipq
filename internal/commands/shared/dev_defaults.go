package shared

import (
	"strings"

	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/registry"
)

// BuildDevDefaults reads all feature flags from the ini file and constructs
// a fully-populated DevDefaults struct. This replaces the 5+ copy-pasted
// blocks that each command used to maintain independently.
func BuildDevDefaults(ini *inifile.File, databaseURL string) configpkg.DevDefaults {
	dd := configpkg.DevDefaults{
		DatabaseURL:  databaseURL,
		Port:         "8080",
		CookieSecret: ini.Get("auth", "cookie_secret"),
	}

	// OAuth
	oauthGoogle := IsOAuthGoogleEnabled(ini)
	oauthGitHub := IsOAuthGitHubEnabled(ini)
	if oauthGoogle || oauthGitHub {
		dd.OAuthRedirectURL = ini.Get("auth", "oauth_redirect_url")
		dd.OAuthRedirectBaseURL = ini.Get("auth", "oauth_redirect_base_url")
	}

	// Workers / Centrifugo
	if IsFeatureEnabled(ini, "workers") {
		dd.RedisURL = ini.Get("workers", "redis_url")
		dd.CentrifugoAPIURL = ini.Get("workers", "centrifugo_api_url")
		dd.CentrifugoAPIKey = ini.Get("workers", "centrifugo_api_key")
		dd.CentrifugoHMACSecret = ini.Get("workers", "centrifugo_hmac_secret")
		dd.CentrifugoWSURL = ini.Get("workers", "centrifugo_ws_url")
	}

	// Files / S3
	if IsFeatureEnabled(ini, "files") {
		dd.S3Bucket = ini.Get("files", "s3_bucket")
		dd.S3Region = ini.Get("files", "s3_region")
		dd.S3Endpoint = ini.Get("files", "s3_endpoint")
		dd.AWSAccessKeyID = ini.Get("files", "aws_access_key_id")
		dd.AWSSecretAccessKey = ini.Get("files", "aws_secret_access_key")
		dd.MaxUploadSizeMB = ini.Get("files", "max_upload_size_mb")
		dd.MultipartThresholdMB = ini.Get("files", "multipart_threshold_mb")
	}

	// Email / SMTP
	if IsFeatureEnabled(ini, "email") {
		dd.SMTPHost = ini.Get("email", "smtp_host")
		dd.SMTPPort = ini.Get("email", "smtp_port")
		dd.SMTPUsername = ini.Get("email", "smtp_username")
		dd.SMTPPassword = ini.Get("email", "smtp_password")
		dd.AppURL = ini.Get("email", "app_url")
	}

	return dd
}

// RegenerateConfig is a convenience wrapper that builds DevDefaults from the
// ini file and calls GenerateConfigEarlyWithFullOptions. It replaces the
// 15–30 line boilerplate block that was duplicated in auth, oauth, email,
// files, and workers commands.
//
// Feature-flag overrides (e.g. filesEnabled, emailEnabled) let callers
// force a flag to true when the command itself is what creates the section
// (the section may not exist in the ini file yet at call time).
func RegenerateConfig(ini *inifile.File, cfg *ProjectConfig, overrides ...ConfigOverride) error {
	opts := buildConfigOptions(ini, cfg)

	// Apply caller-supplied overrides.
	for _, o := range overrides {
		o(&opts)
	}

	return registry.GenerateConfigEarlyWithFullOptions(opts)
}

// ConfigOverride is a functional option for RegenerateConfig.
type ConfigOverride func(*registry.ConfigEarlyOptions)

// WithFilesEnabled forces FilesEnabled to true.
func WithFilesEnabled() ConfigOverride {
	return func(o *registry.ConfigEarlyOptions) { o.FilesEnabled = true }
}

// WithWorkersEnabled forces WorkersEnabled to true.
func WithWorkersEnabled() ConfigOverride {
	return func(o *registry.ConfigEarlyOptions) { o.WorkersEnabled = true }
}

// WithEmailEnabled forces EmailEnabled to true.
func WithEmailEnabled() ConfigOverride {
	return func(o *registry.ConfigEarlyOptions) { o.EmailEnabled = true }
}

func buildConfigOptions(ini *inifile.File, cfg *ProjectConfig) registry.ConfigEarlyOptions {
	oauthGoogle := IsOAuthGoogleEnabled(ini)
	oauthGitHub := IsOAuthGitHubEnabled(ini)

	dd := BuildDevDefaults(ini, cfg.DatabaseURL)

	return registry.ConfigEarlyOptions{
		ShipqRoot:      cfg.ShipqRoot,
		GoModRoot:      cfg.GoModRoot,
		Dialect:        cfg.Dialect,
		FilesEnabled:   IsFeatureEnabled(ini, "files"),
		WorkersEnabled: IsFeatureEnabled(ini, "workers"),
		OAuthGoogle:    oauthGoogle,
		OAuthGitHub:    oauthGitHub,
		EmailEnabled:   IsFeatureEnabled(ini, "email"),
		DevDefaults:    dd,
		CustomEnvVars:  registry.ParseCustomEnvVars(ini),
	}
}

// IsOAuthEnabled returns true if any OAuth provider flag is set to "true" in [auth].
func IsOAuthEnabled(ini *inifile.File) bool {
	return IsOAuthGoogleEnabled(ini) || IsOAuthGitHubEnabled(ini)
}

// OAuthFlagsFromIni returns (google, github) booleans from [auth] section.
func OAuthFlagsFromIni(ini *inifile.File) (google bool, github bool) {
	return IsOAuthGoogleEnabled(ini), IsOAuthGitHubEnabled(ini)
}

// ReadOAuthFlagRaw reads a specific oauth_<name> flag as a raw bool.
func ReadOAuthFlagRaw(ini *inifile.File, provider string) bool {
	return strings.ToLower(ini.Get("auth", "oauth_"+provider)) == "true"
}
