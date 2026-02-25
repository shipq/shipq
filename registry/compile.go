package registry

import (
	"fmt"
	"strings"

	"github.com/shipq/shipq/codegen"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
)

// CompileConfig holds all configuration needed for registry compilation.
type CompileConfig struct {
	GoModRoot  string // Directory containing go.mod
	ShipqRoot  string // Directory containing shipq.ini
	ModulePath string
	Handlers   []codegen.SerializedHandlerInfo
	// OutputPkg is the package name for generated HTTP server code (e.g., "api").
	// Defaults to "api" if empty.
	OutputPkg string
	// OutputDir is the directory for generated HTTP server code relative to ShipqRoot.
	// Defaults to "api" if empty.
	OutputDir string
	// DBDialect is the database dialect for main.go generation ("mysql", "postgres", "sqlite").
	// Defaults to "mysql" if empty.
	DBDialect string
	// DatabaseURL is the full database_url from shipq.ini, used to derive the test database URL.
	DatabaseURL string
	// Port is the server port for main.go. Defaults to "8080" if empty.
	Port string
	// TableScopes maps table names to their scope columns (e.g., "organization_id").
	// Only tables with a scope column are included. Used for tenancy test generation.
	TableScopes map[string]string
	// ScopeColumn is the global scope column from [db] scope in shipq.ini.
	// When set (e.g., "organization_id"), RBAC queries filter by organization.
	ScopeColumn string
	// GenerateResourceTests enables generation of CRUD tests for full resources.
	// A "full resource" is a package that implements all 5 CRUD operations.
	GenerateResourceTests bool
	// FilesEnabled is true if [files] section exists in shipq.ini.
	// When true, S3 config fields are added to the generated SettingsConfig.
	FilesEnabled bool
	// WorkersEnabled is true if [workers] section exists in shipq.ini.
	// When true, worker environment variables (REDIS_URL, CENTRIFUGO_*) are added
	// to the generated SettingsConfig, and channel HTTP routes are generated.
	WorkersEnabled bool
	// HasAuth is true if [auth] section exists in shipq.ini.
	// When true, the job_results table includes author_account_id and the
	// generated dispatch handlers populate it in InsertJobResult calls.
	HasAuth bool
	// OAuthGoogle is true if [auth] oauth_google = true in shipq.ini.
	OAuthGoogle bool
	// OAuthGitHub is true if [auth] oauth_github = true in shipq.ini.
	OAuthGitHub bool
	// EmailEnabled is true if [email] section exists in shipq.ini.
	// When true, SMTP environment variables (SMTP_HOST, SMTP_PORT, etc.) and
	// APP_URL are added to the generated SettingsConfig, and email-related
	// auth routes (forgot-password, reset-password, verify-email, resend-verification)
	// are generated.
	EmailEnabled bool
	// DevDefaults holds compile-time dev default values read from shipq.ini.
	// These are baked into the generated config so that GO_ENV != "production"
	// requires zero env vars to run locally.
	DevDefaults configpkg.DevDefaults
	// CustomEnvVars holds user-defined environment variables from the [env]
	// section of shipq.ini. Each entry specifies an env var name and whether
	// it is required (fatal if missing in production) or optional (warn only).
	CustomEnvVars []configpkg.CustomEnvVar
	// Channels holds the serialized channel metadata from the channel compiler.
	// Only populated when WorkersEnabled is true.
	Channels []codegen.SerializedChannelInfo
	// TSFrameworks lists which framework integrations to generate.
	// Valid entries are "react" and "svelte". Parsed from the comma-separated
	// [typescript] framework value in shipq.ini. Defaults to ["react"].
	TSFrameworks []string
	// TSHTTPOutput is the directory for generated HTTP TypeScript client files,
	// relative to ShipqRoot. The base shipq-api.ts is written here; react/ and
	// svelte/ subdirectories are created within.
	TSHTTPOutput string
	// TSChannelOutput is the directory for generated channel TypeScript client
	// files, relative to ShipqRoot. The base shipq-channels.ts is written here;
	// react/ and svelte/ subdirectories are created within.
	TSChannelOutput string
	// Verbose enables additional logging.
	Verbose bool
}

// ParseFrameworks splits a comma-separated framework string into a slice.
// Valid entries are "react" and "svelte". Unknown values are silently dropped.
// Returns ["react"] if the input is empty.
func ParseFrameworks(raw string) []string {
	if raw == "" {
		return []string{"react"}
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		fw := strings.TrimSpace(strings.ToLower(s))
		if fw == "react" || fw == "svelte" {
			out = append(out, fw)
		}
	}
	if len(out) == 0 {
		return []string{"react"}
	}
	return out
}

// HasFramework returns true if fw is present in the frameworks slice.
func HasFramework(frameworks []string, fw string) bool {
	for _, f := range frameworks {
		if f == fw {
			return true
		}
	}
	return false
}

// CompileRegistry is the central hook for all codegen that depends on the
// handler registry. This function will grow to include:
//
//   - generateOpenAPI() ✓
//   - generateHTTPServer() ✓
//   - generateHTTPMain() ✓
//   - generateHTTPTestClient() ✓
//   - generateHTTPTestHarness() ✓
//   - generateResourceTests() ✓
//   - generateOpenAPITest() ✓
//   - generateTypeScriptHTTPClient() ✓
//   - generateTypeScriptReactHooks() ✓
//   - generateTypeScriptSvelteHooks() ✓
func CompileRegistry(cfg CompileConfig) error {
	setDefaults(&cfg)

	if cfg.DBDialect == "" {
		return fmt.Errorf(
			"could not determine database dialect; check that db.database_url in shipq.ini is a valid URL " +
				"(e.g. postgres://..., mysql://..., sqlite://...)",
		)
	}

	if cfg.Verbose {
		if err := printDebugRegistry(cfg.Handlers); err != nil {
			return err
		}
	}

	// Always generate config package first — it is imported by the HTTP
	// server, main.go, and test files regardless of whether auth is enabled.
	if err := generateConfig(cfg); err != nil {
		return err
	}

	// Generate OpenAPI spec and docs HTML first; these are passed into
	// the HTTP server generator to embed as dev-mode routes.
	oaData, err := generateOpenAPI(cfg)
	if err != nil {
		return err
	}

	// Generate admin panel HTML
	adminHTML := generateAdminPanel(cfg)

	if err := generateHTTPServer(cfg, oaData.SpecJSON, oaData.DocsHTML, adminHTML); err != nil {
		return err
	}

	if err := generateHTTPMain(cfg); err != nil {
		return err
	}

	// Generate test infrastructure
	if err := generateHTTPTestClient(cfg); err != nil {
		return err
	}

	if err := generateHTTPTestHarness(cfg); err != nil {
		return err
	}

	// Generate OpenAPI endpoint test
	if err := generateOpenAPITest(cfg); err != nil {
		return err
	}

	// Generate resource tests if enabled
	if cfg.GenerateResourceTests {
		if err := generateResourceTests(cfg); err != nil {
			return err
		}
	}

	// Generate tenancy isolation tests for scoped tables
	if len(cfg.TableScopes) > 0 {
		if err := generateTenancyTests(cfg); err != nil {
			return err
		}
	}

	// Generate RBAC integration tests when auth is present
	if err := generateRBACTests(cfg); err != nil {
		return err
	}

	// Generate channel HTTP routes when workers are enabled
	if cfg.WorkersEnabled && len(cfg.Channels) > 0 {
		if err := generateChannelRoutes(cfg); err != nil {
			return err
		}
	}

	// Generate TypeScript HTTP client and framework hooks when output is configured
	if len(cfg.Handlers) > 0 && cfg.TSHTTPOutput != "" {
		if err := generateTypeScriptHTTPClient(cfg); err != nil {
			return err
		}

		// Generate only the requested framework integrations
		if HasFramework(cfg.TSFrameworks, "react") {
			if err := generateTypeScriptReactHooks(cfg); err != nil {
				return err
			}
		}
		if HasFramework(cfg.TSFrameworks, "svelte") {
			if err := generateTypeScriptSvelteHooks(cfg); err != nil {
				return err
			}
		}
	}

	return nil
}
