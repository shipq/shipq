package auth

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/authgen"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	codegenMigrate "github.com/shipq/shipq/codegen/migrate"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

// ValidateOAuthProvider checks that the given provider name is supported.
// Returns an error for unknown providers instead of calling os.Exit,
// making it testable.
func ValidateOAuthProvider(name string) error {
	provider := authgen.ProviderByName(name)
	if provider == nil {
		return fmt.Errorf("unknown OAuth provider %q; supported providers: %s",
			name, strings.Join(authgen.AllProviderNames(), ", "))
	}
	return nil
}

// SetOAuthIniFlags sets the oauth_<provider> = true flag and the default
// redirect URLs in the given ini file. Returns true if any value was changed.
func SetOAuthIniFlags(ini *inifile.File, providerName string) bool {
	changed := false

	key := "oauth_" + providerName
	if strings.ToLower(ini.Get("auth", key)) != "true" {
		ini.Set("auth", key, "true")
		changed = true
	}

	if ini.Get("auth", "oauth_redirect_url") == "" {
		ini.Set("auth", "oauth_redirect_url", "http://localhost:3000")
		changed = true
	}

	if ini.Get("auth", "oauth_redirect_base_url") == "" {
		ini.Set("auth", "oauth_redirect_base_url", "http://localhost:8080")
		changed = true
	}

	return changed
}

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

// AuthOAuthCmd handles "shipq auth <provider>" — adds OAuth support for the
// given provider to an existing auth system.
func AuthOAuthCmd(providerName string) {
	// Validate provider name
	if err := ValidateOAuthProvider(providerName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	provider := authgen.ProviderByName(providerName)

	// Load project config
	cfg, err := loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// DAG prerequisite check (alongside existing checks)
	dagCmd := shipqdag.CmdAuthGoogle
	if providerName == "github" {
		dagCmd = shipqdag.CmdAuthGitHub
	}
	if !shipqdag.CheckPrerequisites(dagCmd, cfg.ShipqRoot) {
		os.Exit(1)
	}

	// Precondition: auth migrations must already exist
	if !authMigrationsExist(cfg.MigrationsPath) {
		fmt.Fprintln(os.Stderr, "error: auth migrations not found. Run `shipq auth` first.")
		os.Exit(1)
	}

	// ---------------------------------------------------------------
	// 1. Update shipq.ini with oauth flags
	// ---------------------------------------------------------------
	fmt.Println("Updating shipq.ini with OAuth config...")
	shipqIniPath := filepath.Join(cfg.ShipqRoot, project.ShipqIniFile)
	ini, iniErr := inifile.ParseFile(shipqIniPath)
	if iniErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse shipq.ini: %v\n", iniErr)
		os.Exit(1)
	}

	SetOAuthIniFlags(ini, providerName)

	if writeErr := ini.WriteFile(shipqIniPath); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write shipq.ini: %v\n", writeErr)
		os.Exit(1)
	}
	fmt.Printf("  Set [auth] oauth_%s = true\n", providerName)

	// ---------------------------------------------------------------
	// 2. Generate OAuth migrations (if not already present)
	// ---------------------------------------------------------------
	if authgen.OAuthMigrationsExist(cfg.MigrationsPath) {
		fmt.Println("")
		fmt.Println("OAuth migrations already exist, skipping migration generation...")
	} else {
		fmt.Println("")
		fmt.Println("Generating OAuth migrations...")

		ts0 := codegenMigrate.NextMigrationBaseTime(cfg.MigrationsPath).Format("20060102150405")

		migrations := []struct {
			name     string
			generate func(timestamp, modulePath string) []byte
		}{
			{"oauth_accounts", authgen.GenerateOAuthAccountsMigration},
		}

		timestamps := []string{ts0}

		for i, m := range migrations {
			code := m.generate(timestamps[i], cfg.ModulePath)
			fileName := fmt.Sprintf("%s_%s.go", timestamps[i], m.name)
			filePath := filepath.Join(cfg.MigrationsPath, fileName)

			if err := os.WriteFile(filePath, code, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", fileName, err)
				os.Exit(1)
			}

			relPath, _ := filepath.Rel(cfg.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	// ---------------------------------------------------------------
	// 3. Run migrate up
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Running migrations...")
	up.MigrateUpCmd()

	// ---------------------------------------------------------------
	// 4. Re-read ini (in case migrate up changed it) and collect all
	//    enabled OAuth providers for code generation
	// ---------------------------------------------------------------
	ini, iniErr = inifile.ParseFile(shipqIniPath)
	if iniErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to re-read shipq.ini: %v\n", iniErr)
		os.Exit(1)
	}
	allProviders := EnabledOAuthProviders(ini)

	testDatabaseURL := ""
	if cfg.DatabaseURL != "" {
		if u, err := dburl.TestDatabaseURL(cfg.DatabaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	// Detect whether signup has been run (signup.go exists)
	authDir := filepath.Join(cfg.ShipqRoot, "api", "auth")
	signupPath := filepath.Join(authDir, "signup.go")
	signupEnabled := false
	if _, statErr := os.Stat(signupPath); statErr == nil {
		signupEnabled = true
	}

	// Detect whether email has been configured
	emailEnabled := ini.Section("email") != nil

	authCfg := authgen.AuthGenConfig{
		ModulePath:      cfg.ModulePath,
		Dialect:         cfg.Dialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     cfg.ScopeColumn,
		OAuthProviders:  allProviders,
		SignupEnabled:   signupEnabled,
		EmailEnabled:    emailEnabled,
	}

	// ---------------------------------------------------------------
	// 5. Generate api/auth/oauth_shared.go
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Generating OAuth shared utilities...")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/auth directory: %v\n", err)
		os.Exit(1)
	}

	sharedCode, err := authgen.GenerateOAuthShared(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate oauth_shared.go: %v\n", err)
		os.Exit(1)
	}
	sharedPath := filepath.Join(authDir, "oauth_shared.go")
	changed, err := codegen.WriteFileIfChanged(sharedPath, sharedCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write oauth_shared.go: %v\n", err)
		os.Exit(1)
	}
	if changed {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, sharedPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// 6. Generate api/auth/oauth_<provider>.go
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Printf("Generating OAuth %s handler...\n", provider.DisplayName)
	providerCode, err := authgen.GenerateOAuthProvider(authCfg, *provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate oauth_%s.go: %v\n", providerName, err)
		os.Exit(1)
	}
	providerPath := filepath.Join(authDir, fmt.Sprintf("oauth_%s.go", providerName))
	changed, err = codegen.WriteFileIfChanged(providerPath, providerCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write oauth_%s.go: %v\n", providerName, err)
		os.Exit(1)
	}
	if changed {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, providerPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// 7. Regenerate querydefs/auth/queries.go with OAuth queries
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating auth query definitions...")
	authQueryDefs, err := authgen.GenerateAuthQueryDefs(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate auth query defs: %v\n", err)
		os.Exit(1)
	}

	queryDefsDir := filepath.Join(cfg.ShipqRoot, "querydefs", "auth")
	if err := os.MkdirAll(queryDefsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create querydefs/auth directory: %v\n", err)
		os.Exit(1)
	}
	queryDefsPath := filepath.Join(queryDefsDir, "queries.go")
	if err := os.WriteFile(queryDefsPath, authQueryDefs, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write querydefs/auth/queries.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Updated: querydefs/auth/queries.go")

	// ---------------------------------------------------------------
	// 8. Regenerate api/auth/login.go (nil guard on PasswordHash)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating login handler (PasswordHash nil guard)...")
	loginCode, err := authgen.GenerateLoginHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate login.go: %v\n", err)
		os.Exit(1)
	}
	loginPath := filepath.Join(authDir, "login.go")
	changed, err = codegen.WriteFileIfChanged(loginPath, loginCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write login.go: %v\n", err)
		os.Exit(1)
	}
	if changed {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, loginPath)
		fmt.Printf("  Updated: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// 9. Regenerate api/auth/register.go (OAuth routes)
	//
	// When signup is enabled, use GenerateSignupRegister so the /signup
	// route is preserved. Previously we wrote register.go without signup
	// AND a separate signup_register.go with signup, causing duplicate
	// Register / RegisterOAuthRoutes declarations.
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating auth register (OAuth routes)...")

	var registerCode []byte
	if signupEnabled {
		registerCode, err = authgen.GenerateSignupRegister(authCfg)
	} else {
		registerCode, err = authgen.GenerateRegister(authCfg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate register.go: %v\n", err)
		os.Exit(1)
	}
	registerPath := filepath.Join(authDir, "register.go")
	changed, err = codegen.WriteFileIfChanged(registerPath, registerCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write register.go: %v\n", err)
		os.Exit(1)
	}
	if changed {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, registerPath)
		fmt.Printf("  Updated: %s\n", relPath)
	}

	// Clean up stale signup_register.go if it exists from a previous run,
	// since the signup routes are now consolidated into register.go.
	staleSignupRegister := filepath.Join(authDir, "signup_register.go")
	if _, statErr := os.Stat(staleSignupRegister); statErr == nil {
		if removeErr := os.Remove(staleSignupRegister); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove stale signup_register.go: %v\n", removeErr)
		}
	}

	// ---------------------------------------------------------------
	// 10. Regenerate config/config.go with OAuth env vars
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating config package...")
	filesEnabled := ini.Section("files") != nil
	workersEnabled := ini.Section("workers") != nil

	oauthGoogle := strings.ToLower(ini.Get("auth", "oauth_google")) == "true"
	oauthGitHub := strings.ToLower(ini.Get("auth", "oauth_github")) == "true"

	devDefaults := configpkg.DevDefaults{
		DatabaseURL:  cfg.DatabaseURL,
		Port:         "8080",
		CookieSecret: ini.Get("auth", "cookie_secret"),
	}
	if oauthGoogle || oauthGitHub {
		devDefaults.OAuthRedirectURL = ini.Get("auth", "oauth_redirect_url")
		devDefaults.OAuthRedirectBaseURL = ini.Get("auth", "oauth_redirect_base_url")
	}

	if emailEnabled {
		devDefaults.SMTPHost = ini.Get("email", "smtp_host")
		devDefaults.SMTPPort = ini.Get("email", "smtp_port")
		devDefaults.SMTPUsername = ini.Get("email", "smtp_username")
		devDefaults.SMTPPassword = ini.Get("email", "smtp_password")
		devDefaults.AppURL = ini.Get("email", "app_url")
	}

	if err := registry.GenerateConfigEarlyWithFullOptions(registry.ConfigEarlyOptions{
		ShipqRoot:      cfg.ShipqRoot,
		GoModRoot:      cfg.GoModRoot,
		Dialect:        cfg.Dialect,
		FilesEnabled:   filesEnabled,
		WorkersEnabled: workersEnabled,
		OAuthGoogle:    oauthGoogle,
		OAuthGitHub:    oauthGitHub,
		EmailEnabled:   emailEnabled,
		DevDefaults:    devDefaults,
		CustomEnvVars:  registry.ParseCustomEnvVars(ini),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Updated: config/config.go")

	// ---------------------------------------------------------------
	// 11. go mod tidy
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = cfg.GoModRoot
	if tidyOut, tidyErr := tidyCmd.CombinedOutput(); tidyErr != nil {
		fmt.Fprintf(os.Stderr, "error: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
		os.Exit(1)
	}
	fmt.Println("  go mod tidy done")

	// ---------------------------------------------------------------
	// 12. db compile
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// ---------------------------------------------------------------
	// 13. registry.Run (regenerates api/server.go, test client, etc.)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(cfg.ShipqRoot, cfg.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to compile registry: %v\n", err)
		os.Exit(1)
	}

	// ---------------------------------------------------------------
	// 14. Success message
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Printf("OAuth %s added successfully!\n", provider.DisplayName)
	fmt.Println("")
	fmt.Println("Generated routes:")
	fmt.Printf("  GET /auth/%s/login    - Redirect to %s consent screen\n", providerName, provider.DisplayName)
	fmt.Printf("  GET /auth/%s/callback - Handle %s OAuth callback\n", providerName, provider.DisplayName)
	fmt.Println("")
	fmt.Println("Environment variables required:")
	fmt.Printf("  %s - %s OAuth client ID\n", provider.ClientIDEnvVar, provider.DisplayName)
	fmt.Printf("  %s - %s OAuth client secret\n", provider.ClientSecretEnvVar, provider.DisplayName)
	fmt.Println("  OAUTH_REDIRECT_URL      - Post-login redirect URL (default: http://localhost:3000)")
	fmt.Println("  OAUTH_REDIRECT_BASE_URL - Base URL for callback (default: http://localhost:8080)")
}
