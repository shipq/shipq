package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/channelcompile"
	"github.com/shipq/shipq/codegen/discovery"
	"github.com/shipq/shipq/codegen/handlercompile"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

// Run executes the full handler compile pipeline:
//  1. Discover API packages
//  2. Generate and run the compile program
//  3. Call CompileRegistry with the results
//
// Parameters:
//   - shipqRoot: directory containing shipq.ini (where api/ directory lives)
//   - goModRoot: directory containing go.mod
//
// This is the function called by CLI commands.
func Run(shipqRoot, goModRoot string) error {
	// Get module info (raw module path + monorepo subpath)
	moduleInfo, err := codegen.GetModuleInfo(goModRoot, shipqRoot)
	if err != nil {
		return fmt.Errorf("failed to get module info: %w", err)
	}
	importPrefix := moduleInfo.FullImportPath("")

	// Discover API packages (in shipq root, but import paths relative to go.mod root).
	// Discovery uses filepath.Rel(goModRoot, ...) so it must receive the raw module path.
	apiPkgs, err := discovery.DiscoverAPIPackages(goModRoot, shipqRoot, moduleInfo.ModulePath)
	if err != nil {
		return fmt.Errorf("failed to discover API packages: %w", err)
	}

	// If no API packages found, nothing to compile
	if len(apiPkgs) == 0 {
		fmt.Fprintln(os.Stderr, "No API packages found with Register functions.")
		return nil
	}

	// Build and run the compile program (uses goModRoot for replace directive)
	cfg := handlercompile.HandlerCompileProgramConfig{
		ModulePath:  importPrefix,
		GoModModule: moduleInfo.ModulePath,
		APIPkgs:     apiPkgs,
	}

	handlers, err := handlercompile.BuildAndRunHandlerCompileProgram(goModRoot, cfg)
	if err != nil {
		return fmt.Errorf("failed to compile handlers: %w", err)
	}

	// Read DB dialect and URL from shipq.ini
	dialect := ""
	databaseURL := ""
	shipqIniPath := filepath.Join(shipqRoot, project.ShipqIniFile)
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		if u := ini.Get("db", "database_url"); u != "" {
			databaseURL = u
			if d, err := dburl.InferDialectFromDBUrl(u); err == nil {
				dialect = d
			}
		}
	}

	// Read scope configuration from shipq.ini
	tableScopes := make(map[string]string)
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		globalScope := ini.Get("db", "scope")
		if globalScope != "" {
			// Apply global scope to all table-like handler packages
			for _, h := range handlers {
				// Extract table name from the package path (last segment)
				parts := strings.Split(h.PackagePath, "/")
				tableName := parts[len(parts)-1]
				if tableName != "" {
					tableScopes[tableName] = globalScope
				}
			}
		}
		// Per-table overrides
		for _, section := range ini.SectionsWithPrefix("crud.") {
			tableName := strings.TrimPrefix(section.Name, "crud.")
			if section.Get("scope") != "" {
				tableScopes[tableName] = section.Get("scope")
			} else if section.HasKey("scope") {
				// Explicit empty scope disables it
				delete(tableScopes, tableName)
			}
		}
	}

	// Read global scope column for RBAC
	scopeColumn := ""
	filesEnabled := false
	workersEnabled := false
	hasAuth := false
	oauthGoogle := false
	oauthGitHub := false
	var devDefaults configpkg.DevDefaults
	var customEnvVars []configpkg.CustomEnvVar
	tsFrameworks := []string{"react"}
	tsHTTPOutput := ""
	tsChannelOutput := ""
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		scopeColumn = ini.Get("db", "scope")
		// Detect if [files] section exists
		if ini.Section("files") != nil {
			filesEnabled = true
		}
		// Detect if [workers] section exists
		if ini.Section("workers") != nil {
			workersEnabled = true
		}
		// Detect if [auth] section exists
		if ini.Section("auth") != nil {
			hasAuth = true
		}
		// Detect OAuth providers
		if strings.ToLower(ini.Get("auth", "oauth_google")) == "true" {
			oauthGoogle = true
		}
		if strings.ToLower(ini.Get("auth", "oauth_github")) == "true" {
			oauthGitHub = true
		}

		// Populate dev defaults from shipq.ini
		devDefaults = devDefaultsFromIni(ini, filesEnabled, workersEnabled)

		// Populate OAuth dev defaults
		if oauthGoogle || oauthGitHub {
			devDefaults.OAuthRedirectURL = ini.Get("auth", "oauth_redirect_url")
			devDefaults.OAuthRedirectBaseURL = ini.Get("auth", "oauth_redirect_base_url")
		}

		// Parse user-defined env vars from [env] section
		customEnvVars = ParseCustomEnvVars(ini)

		// Read TypeScript config
		tsFrameworks = ParseFrameworks(ini.Get("typescript", "framework"))
		if o := ini.Get("typescript", "http_output"); o != "" {
			tsHTTPOutput = o
		}
		if o := ini.Get("typescript", "channel_output"); o != "" {
			tsChannelOutput = o
		}
	}

	// Compile channel registry if workers are enabled
	var channels []codegen.SerializedChannelInfo
	if workersEnabled {
		compiledChannels, compErr := channelcompile.BuildAndRunChannelCompileProgram(goModRoot, shipqRoot, moduleInfo)
		if compErr != nil {
			return fmt.Errorf("failed to compile channels: %w", compErr)
		}
		channels = compiledChannels
	}

	// Run the registry compilation (the central codegen hook)
	compileCfg := CompileConfig{
		GoModRoot:       goModRoot,
		ShipqRoot:       shipqRoot,
		ModulePath:      importPrefix,
		Handlers:        handlers,
		DBDialect:       dialect,
		DatabaseURL:     databaseURL,
		TableScopes:     tableScopes,
		ScopeColumn:     scopeColumn,
		FilesEnabled:    filesEnabled,
		WorkersEnabled:  workersEnabled,
		HasAuth:         hasAuth,
		OAuthGoogle:     oauthGoogle,
		OAuthGitHub:     oauthGitHub,
		Channels:        channels,
		DevDefaults:     devDefaults,
		CustomEnvVars:   customEnvVars,
		TSFrameworks:    tsFrameworks,
		TSHTTPOutput:    tsHTTPOutput,
		TSChannelOutput: tsChannelOutput,
	}

	return CompileRegistry(compileCfg)
}

// ParseCustomEnvVars reads the [env] section from a parsed shipq.ini file and
// returns a slice of CustomEnvVar. Each key in the section becomes an
// uppercase env var name; the value is either "required" (fatal if missing in
// production) or "optional" (warn only). Any other value is treated as
// "optional".
//
// Example shipq.ini:
//
//	[env]
//	OPENAI_API_KEY = required
//	ANTHROPIC_API_KEY = optional
func ParseCustomEnvVars(ini *inifile.File) []configpkg.CustomEnvVar {
	section := ini.Section("env")
	if section == nil {
		return nil
	}

	var vars []configpkg.CustomEnvVar
	for _, kv := range section.Values {
		name := strings.ToUpper(kv.Key)
		required := strings.ToLower(strings.TrimSpace(kv.Value)) == "required"
		vars = append(vars, configpkg.CustomEnvVar{
			Name:     name,
			Required: required,
		})
	}
	return vars
}

// devDefaultsFromIni reads dev default values from a parsed shipq.ini file.
func devDefaultsFromIni(ini *inifile.File, filesEnabled, workersEnabled bool) configpkg.DevDefaults {
	d := configpkg.DevDefaults{
		DatabaseURL:  ini.Get("db", "database_url"),
		Port:         "8080",
		CookieSecret: ini.Get("auth", "cookie_secret"),
	}

	if filesEnabled {
		d.S3Bucket = ini.Get("files", "s3_bucket")
		d.S3Region = ini.Get("files", "s3_region")
		d.S3Endpoint = ini.Get("files", "s3_endpoint")
		d.AWSAccessKeyID = ini.Get("files", "aws_access_key_id")
		d.AWSSecretAccessKey = ini.Get("files", "aws_secret_access_key")
		d.MaxUploadSizeMB = ini.Get("files", "max_upload_size_mb")
		d.MultipartThresholdMB = ini.Get("files", "multipart_threshold_mb")
	}

	if workersEnabled {
		d.RedisURL = ini.Get("workers", "redis_url")
		d.CentrifugoAPIURL = ini.Get("workers", "centrifugo_api_url")
		d.CentrifugoAPIKey = ini.Get("workers", "centrifugo_api_key")
		d.CentrifugoHMACSecret = ini.Get("workers", "centrifugo_hmac_secret")
		d.CentrifugoWSURL = ini.Get("workers", "centrifugo_ws_url")
	}

	return d
}
