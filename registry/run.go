package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/channelcompile"
	"github.com/shipq/shipq/codegen/dbpkg"
	"github.com/shipq/shipq/codegen/discovery"
	"github.com/shipq/shipq/codegen/embed"
	"github.com/shipq/shipq/codegen/handlercompile"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	"github.com/shipq/shipq/db/portsql/codegen/queryrunner"
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

	// ── Read shipq.ini config early ──────────────────────────────────
	// We need dialect, feature flags, etc. before bootstrapping and
	// before the handler compile program is built.
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

	// Read feature flags from shipq.ini
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
	stripPrefix := ""
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		scopeColumn = ini.Get("db", "scope")
		if ini.Section("files") != nil {
			filesEnabled = true
		}
		if ini.Section("workers") != nil {
			workersEnabled = true
		}
		if ini.Section("auth") != nil {
			hasAuth = true
		}
		if strings.ToLower(ini.Get("auth", "oauth_google")) == "true" {
			oauthGoogle = true
		}
		if strings.ToLower(ini.Get("auth", "oauth_github")) == "true" {
			oauthGitHub = true
		}

		devDefaults = devDefaultsFromIni(ini, filesEnabled, workersEnabled)

		if oauthGoogle || oauthGitHub {
			devDefaults.OAuthRedirectURL = ini.Get("auth", "oauth_redirect_url")
			devDefaults.OAuthRedirectBaseURL = ini.Get("auth", "oauth_redirect_base_url")
		}

		customEnvVars = ParseCustomEnvVars(ini)

		tsFrameworks = ParseFrameworks(ini.Get("typescript", "framework"))
		if o := ini.Get("typescript", "http_output"); o != "" {
			tsHTTPOutput = o
		}
		if o := ini.Get("typescript", "channel_output"); o != "" {
			tsChannelOutput = o
		}

		if sp := ini.Get("server", "strip_prefix"); sp != "" {
			stripPrefix = strings.TrimRight(strings.TrimSpace(sp), "/")
		}
	}

	// ── Bootstrap: ensure all imported packages exist ────────────────
	// The handler compile program imports shipq/lib/handler, and the
	// generated server code imports shipq/lib/httpserver, shipq/queries,
	// config, etc. We must ensure these packages exist on disk BEFORE
	// building the compile program or generating server code.
	if err := bootstrapPackages(shipqRoot, importPrefix, dialect, filesEnabled, workersEnabled); err != nil {
		return fmt.Errorf("failed to bootstrap packages: %w", err)
	}

	// ── Discover and compile handlers ────────────────────────────────
	apiPkgs, err := discovery.DiscoverAPIPackages(goModRoot, shipqRoot, moduleInfo.ModulePath)
	if err != nil {
		return fmt.Errorf("failed to discover API packages: %w", err)
	}

	// Even with zero user handlers, we still run the full pipeline so that
	// the healthcheck endpoint (scaffolded by `shipq init`) and any
	// channel routes are generated.

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

	// ── Read remaining config from shipq.ini ─────────────────────────
	// Scope configuration (depends on handlers being known)
	tableScopes := make(map[string]string)
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		globalScope := ini.Get("db", "scope")
		if globalScope != "" {
			for _, h := range handlers {
				parts := strings.Split(h.PackagePath, "/")
				tableName := parts[len(parts)-1]
				if tableName != "" {
					tableScopes[tableName] = globalScope
				}
			}
		}
		for _, section := range ini.SectionsWithPrefix("crud.") {
			tableName := strings.TrimPrefix(section.Name, "crud.")
			if section.Get("scope") != "" {
				tableScopes[tableName] = section.Get("scope")
			} else if section.HasKey("scope") {
				delete(tableScopes, tableName)
			}
		}
	}

	// Read auto_migrate setting from [db] section
	autoMigrate := false
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		if strings.ToLower(ini.Get("db", "auto_migrate")) == "true" {
			schemaJSONPath := filepath.Join(shipqRoot, "shipq", "db", "migrate", "schema.json")
			if _, err := os.Stat(schemaJSONPath); err == nil {
				autoMigrate = true
			}
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
		AutoMigrate:     autoMigrate,
		FilesEnabled:    filesEnabled,
		WorkersEnabled:  workersEnabled,
		HasAuth:         hasAuth,
		OAuthGoogle:     oauthGoogle,
		OAuthGitHub:     oauthGitHub,
		Channels:        channels,
		DevDefaults:     devDefaults,
		CustomEnvVars:   customEnvVars,
		StripPrefix:     stripPrefix,
		TSFrameworks:    tsFrameworks,
		TSHTTPOutput:    tsHTTPOutput,
		TSChannelOutput: tsChannelOutput,
	}

	return CompileRegistry(compileCfg)
}

// bootstrapPackages ensures that all packages imported by generated code exist
// on disk. This is idempotent: if packages already exist (e.g. because
// `migrate up` or `db compile` already created them), nothing is overwritten
// unnecessarily.
//
// It handles three categories of packages:
//  1. Embedded library packages (shipq/lib/*) — via embed.EmbedAllPackages
//  2. Database helper package (shipq/db/db.go) — via dbpkg.EnsureDBPackage
//  3. Query runner stubs (shipq/queries/) — minimal Runner interface + QueryRunner
func bootstrapPackages(shipqRoot, importPrefix, dialect string, filesEnabled, workersEnabled bool) error {
	// 1. Embed library packages (handler, httpserver, httputil, logging, etc.)
	// The handler compile program imports shipq/lib/handler, and the generated
	// HTTP server code imports shipq/lib/httpserver, shipq/lib/logging, etc.
	embedOpts := embed.EmbedOptions{
		FilesEnabled:   filesEnabled,
		WorkersEnabled: workersEnabled,
		DBDialect:      dialect,
	}
	if err := embed.EmbedAllPackages(shipqRoot, importPrefix, embedOpts); err != nil {
		return fmt.Errorf("failed to embed library packages: %w", err)
	}

	// 2. Ensure shipq/db/db.go exists (driver import + DSN helpers).
	// EnsureDBPackage reads shipq.ini internally to get the dialect and URL.
	// Skip when dialect is empty (no database_url configured yet).
	if dialect != "" {
		dbGoPath := filepath.Join(shipqRoot, "shipq", "db", "db.go")
		if _, err := os.Stat(dbGoPath); os.IsNotExist(err) {
			if dbErr := dbpkg.EnsureDBPackage(shipqRoot); dbErr != nil {
				return fmt.Errorf("failed to bootstrap shipq/db package: %w", dbErr)
			}
		}
	}

	// 3. Ensure shipq/queries/types.go and shipq/queries/<dialect>/runner.go
	// exist. These are imported by the generated main.go and HTTP server code.
	// When no migrations have been run yet, generate stubs with zero queries
	// (minimal Runner interface with just BeginTx, empty QueryRunner, etc.).
	if dialect != "" {
		typesPath := filepath.Join(shipqRoot, "shipq", "queries", "types.go")
		if _, err := os.Stat(typesPath); os.IsNotExist(err) {
			if err := bootstrapQueryPackages(shipqRoot, importPrefix, dialect); err != nil {
				return err
			}
		}
	}

	return nil
}

// bootstrapQueryPackages generates minimal shipq/queries/types.go and
// shipq/queries/<dialect>/runner.go with zero user queries. This produces a
// valid Runner interface (with just BeginTx), QueryRunner struct,
// NewQueryRunner, WithTx, WithDB, and context helpers — everything the
// generated HTTP server code needs to compile.
//
// When `db compile` runs later (after `migrate up`), it regenerates these
// files with real query methods, fully overwriting the stubs.
func bootstrapQueryPackages(shipqRoot, importPrefix, dialect string) error {
	runnerCfg := queryrunner.UnifiedRunnerConfig{
		ModulePath:  importPrefix,
		Dialect:     dialect,
		UserQueries: nil, // no queries yet
	}

	// Generate types.go (Runner interface, TxRunner, context helpers)
	typesCode, err := queryrunner.GenerateSharedTypes(runnerCfg)
	if err != nil {
		return fmt.Errorf("failed to generate stub types.go: %w", err)
	}
	queriesDir := filepath.Join(shipqRoot, "shipq", "queries")
	if err := codegen.EnsureDir(queriesDir); err != nil {
		return fmt.Errorf("failed to create queries directory: %w", err)
	}
	typesPath := filepath.Join(queriesDir, "types.go")
	if _, err := codegen.WriteFileIfChanged(typesPath, typesCode); err != nil {
		return fmt.Errorf("failed to write stub types.go: %w", err)
	}

	// Generate dialect-specific runner.go (QueryRunner struct, NewQueryRunner, etc.)
	runnerCode, err := queryrunner.GenerateUnifiedRunner(runnerCfg)
	if err != nil {
		return fmt.Errorf("failed to generate stub runner.go: %w", err)
	}
	dialectDir := filepath.Join(queriesDir, dialect)
	if err := codegen.EnsureDir(dialectDir); err != nil {
		return fmt.Errorf("failed to create dialect directory: %w", err)
	}
	runnerPath := filepath.Join(dialectDir, "runner.go")
	if _, err := codegen.WriteFileIfChanged(runnerPath, runnerCode); err != nil {
		return fmt.Errorf("failed to write stub runner.go: %w", err)
	}

	return nil
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
