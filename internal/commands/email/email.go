package email

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

// authMigrationSuffixes are the file suffixes used to detect existing auth migrations.
var authMigrationSuffixes = []string{
	"_organizations.go",
	"_accounts.go",
	"_organization_users.go",
	"_sessions.go",
	"_roles.go",
	"_account_roles.go",
	"_role_actions.go",
}

// authMigrationsExist checks if all auth migration files exist.
func authMigrationsExist(migrationsPath string) bool {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return false
	}

	found := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, suffix := range authMigrationSuffixes {
			if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
				found[suffix] = true
			}
		}
	}
	return len(found) == len(authMigrationSuffixes)
}

// enabledOAuthProvidersFromIni reads [auth] oauth_<name> flags from the ini
// file and returns the list of enabled provider names.
func enabledOAuthProvidersFromIni(ini *inifile.File) []string {
	var providers []string
	for _, name := range authgen.AllProviderNames() {
		if strings.ToLower(ini.Get("auth", "oauth_"+name)) == "true" {
			providers = append(providers, name)
		}
	}
	return providers
}

// EmailCmd handles "shipq email" — adds email verification, password reset,
// and SMTP support to an existing auth system.
//
// Prerequisites:
//   - [auth] must exist in shipq.ini (run `shipq auth` first)
//   - [workers] must exist in shipq.ini (run `shipq workers` first)
//
// This command:
//  1. Updates shipq.ini with [email] section and dev defaults
//  2. Generates 4 migrations (sent_emails, accounts_verified, password_reset_tokens, email_verification_tokens)
//  3. Runs migrate up
//  4. Generates email handler (api/auth/email.go)
//  5. Generates password reset handlers (forgot_password.go, reset_password.go)
//  6. Generates email verification handlers (verify_email.go, resend_verification.go)
//  7. Regenerates login.go (verified check), signup.go (verification email), oauth_shared.go (verified=true)
//  8. Regenerates querydefs, register.go, config
//  9. Runs db compile, go mod tidy, registry.Run
func EmailCmd() {
	// ---------------------------------------------------------------
	// Step 0: Load project config and check prerequisites
	// ---------------------------------------------------------------
	roots, err := project.FindProjectRoots()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdEmail, roots.ShipqRoot) {
		os.Exit(1)
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to get module path: %v\n", err)
		os.Exit(1)
	}
	modulePath := moduleInfo.FullImportPath("")

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse shipq.ini: %v\n", err)
		os.Exit(1)
	}

	// Check that [auth] exists
	if ini.Section("auth") == nil {
		fmt.Fprintln(os.Stderr, "error: [auth] not configured. Run `shipq auth` first.")
		os.Exit(1)
	}

	// Check that [workers] exists
	if ini.Section("workers") == nil {
		fmt.Fprintln(os.Stderr, "error: [workers] not configured. Email requires worker support.")
		fmt.Fprintln(os.Stderr, "Run `shipq workers` first to enable the worker queue.")
		os.Exit(1)
	}

	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	migrationsPath := filepath.Join(roots.ShipqRoot, migrationsDir)

	// Check that auth migrations exist
	if !authMigrationsExist(migrationsPath) {
		fmt.Fprintln(os.Stderr, "error: auth migrations not found. Run `shipq auth` first.")
		os.Exit(1)
	}

	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
	}

	testDatabaseURL := ""
	if databaseURL != "" {
		if u, err := dburl.TestDatabaseURL(databaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	scopeColumn := ini.Get("db", "scope")

	fmt.Println("Setting up email system...")
	fmt.Println("")

	// ---------------------------------------------------------------
	// Step 1: Update shipq.ini with [email] section
	// ---------------------------------------------------------------
	fmt.Println("Updating shipq.ini with email config...")

	if ini.Section("email") == nil {
		ini.Set("email", "smtp_host", "localhost")
		ini.Set("email", "smtp_port", "1025")
		ini.Set("email", "smtp_username", "")
		ini.Set("email", "smtp_password", "")
		ini.Set("email", "app_url", "http://localhost:3000")

		if writeErr := ini.WriteFile(shipqIniPath); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write shipq.ini: %v\n", writeErr)
			os.Exit(1)
		}
		fmt.Println("  Added [email] section with dev defaults")
	} else {
		fmt.Println("  [email] section already exists, skipping...")
	}

	// ---------------------------------------------------------------
	// Step 2: Generate email migrations (if not already present)
	// ---------------------------------------------------------------
	if authgen.EmailMigrationsExist(migrationsPath) {
		fmt.Println("")
		fmt.Println("Email migrations already exist, skipping migration generation...")
	} else {
		fmt.Println("")
		fmt.Println("Generating email migrations...")

		baseTime := codegenMigrate.NextMigrationBaseTime(migrationsPath)
		ts0 := baseTime.Format("20060102150405")
		ts1 := baseTime.Add(1 * time.Second).Format("20060102150405")
		ts2 := baseTime.Add(2 * time.Second).Format("20060102150405")
		ts3 := baseTime.Add(3 * time.Second).Format("20060102150405")

		type migrationDef struct {
			name     string
			generate func(timestamp, modulePath string) []byte
		}

		migrations := []migrationDef{
			{"sent_emails", authgen.GenerateSentEmailsMigration},
			{"accounts_verified", authgen.GenerateAccountsVerifiedMigration},
			{"password_reset_tokens", authgen.GeneratePasswordResetTokensMigration},
			{"email_verification_tokens", authgen.GenerateEmailVerificationTokensMigration},
		}
		timestamps := []string{ts0, ts1, ts2, ts3}

		for i, m := range migrations {
			code := m.generate(timestamps[i], modulePath)
			fileName := fmt.Sprintf("%s_%s.go", timestamps[i], m.name)
			filePath := filepath.Join(migrationsPath, fileName)

			if err := os.WriteFile(filePath, code, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", fileName, err)
				os.Exit(1)
			}

			relPath, _ := filepath.Rel(roots.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	// ---------------------------------------------------------------
	// Step 3: Run migrate up
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Running migrations...")
	up.MigrateUpCmd()

	fmt.Println("")
	fmt.Println("WARNING: Existing accounts now have verified = false.")
	fmt.Println("Run a data migration or UPDATE accounts SET verified = true to verify existing users.")

	// ---------------------------------------------------------------
	// Step 4: Re-read ini and build authCfg
	// ---------------------------------------------------------------
	ini, err = inifile.ParseFile(shipqIniPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to re-read shipq.ini: %v\n", err)
		os.Exit(1)
	}

	oauthProviders := enabledOAuthProvidersFromIni(ini)

	// Detect whether signup has been run (signup.go exists)
	authDir := filepath.Join(roots.ShipqRoot, "api", "auth")
	signupPath := filepath.Join(authDir, "signup.go")
	signupEnabled := false
	if _, statErr := os.Stat(signupPath); statErr == nil {
		signupEnabled = true
	}

	authCfg := authgen.AuthGenConfig{
		ModulePath:      modulePath,
		Dialect:         dialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     scopeColumn,
		OAuthProviders:  oauthProviders,
		SignupEnabled:   signupEnabled,
		EmailEnabled:    true,
	}

	// ---------------------------------------------------------------
	// Step 5: Generate email handler (api/auth/email.go)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Generating email handler...")

	if err := os.MkdirAll(authDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/auth directory: %v\n", err)
		os.Exit(1)
	}

	emailCode, err := authgen.GenerateEmailHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate email.go: %v\n", err)
		os.Exit(1)
	}
	emailPath := filepath.Join(authDir, "email.go")
	if changed, writeErr := codegen.WriteFileIfChanged(emailPath, emailCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write email.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, emailPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// Step 6: Generate password reset handlers
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Generating password reset handlers...")

	forgotCode, err := authgen.GenerateForgotPasswordHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate forgot_password.go: %v\n", err)
		os.Exit(1)
	}
	forgotPath := filepath.Join(authDir, "forgot_password.go")
	if changed, writeErr := codegen.WriteFileIfChanged(forgotPath, forgotCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write forgot_password.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, forgotPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	resetCode, err := authgen.GenerateResetPasswordHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate reset_password.go: %v\n", err)
		os.Exit(1)
	}
	resetPath := filepath.Join(authDir, "reset_password.go")
	if changed, writeErr := codegen.WriteFileIfChanged(resetPath, resetCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write reset_password.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, resetPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// Step 7: Generate email verification handlers
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Generating email verification handlers...")

	verifyCode, err := authgen.GenerateVerifyEmailHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate verify_email.go: %v\n", err)
		os.Exit(1)
	}
	verifyPath := filepath.Join(authDir, "verify_email.go")
	if changed, writeErr := codegen.WriteFileIfChanged(verifyPath, verifyCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write verify_email.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, verifyPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	resendCode, err := authgen.GenerateResendVerificationHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate resend_verification.go: %v\n", err)
		os.Exit(1)
	}
	resendPath := filepath.Join(authDir, "resend_verification.go")
	if changed, writeErr := codegen.WriteFileIfChanged(resendPath, resendCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write resend_verification.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, resendPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// Step 8: Regenerate login handler (verified check)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating login handler (email verification check)...")

	loginCode, err := authgen.GenerateLoginHandler(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate login.go: %v\n", err)
		os.Exit(1)
	}
	loginPath := filepath.Join(authDir, "login.go")
	if changed, writeErr := codegen.WriteFileIfChanged(loginPath, loginCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write login.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, loginPath)
		fmt.Printf("  Updated: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// Step 9: Regenerate signup handler (verification email dispatch)
	// ---------------------------------------------------------------
	if signupEnabled {
		fmt.Println("")
		fmt.Println("Regenerating signup handler (verification email dispatch)...")

		signupCode, err := authgen.GenerateSignupHandler(authCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to generate signup.go: %v\n", err)
			os.Exit(1)
		}
		if changed, writeErr := codegen.WriteFileIfChanged(signupPath, signupCode); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write signup.go: %v\n", writeErr)
			os.Exit(1)
		} else if changed {
			relPath, _ := filepath.Rel(roots.ShipqRoot, signupPath)
			fmt.Printf("  Updated: %s\n", relPath)
		}
	}

	// ---------------------------------------------------------------
	// Step 10: Regenerate OAuth shared (verified = true on OAuth accounts)
	// ---------------------------------------------------------------
	if len(oauthProviders) > 0 {
		fmt.Println("")
		fmt.Println("Regenerating OAuth shared utilities (auto-verify on OAuth)...")

		sharedCode, err := authgen.GenerateOAuthShared(authCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to generate oauth_shared.go: %v\n", err)
			os.Exit(1)
		}
		sharedPath := filepath.Join(authDir, "oauth_shared.go")
		if changed, writeErr := codegen.WriteFileIfChanged(sharedPath, sharedCode); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write oauth_shared.go: %v\n", writeErr)
			os.Exit(1)
		} else if changed {
			relPath, _ := filepath.Rel(roots.ShipqRoot, sharedPath)
			fmt.Printf("  Updated: %s\n", relPath)
		}
	}

	// ---------------------------------------------------------------
	// Step 11: Regenerate register.go (email routes)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating auth register (email routes)...")

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
	if changed, writeErr := codegen.WriteFileIfChanged(registerPath, registerCode); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write register.go: %v\n", writeErr)
		os.Exit(1)
	} else if changed {
		relPath, _ := filepath.Rel(roots.ShipqRoot, registerPath)
		fmt.Printf("  Updated: %s\n", relPath)
	}

	// ---------------------------------------------------------------
	// Step 12: Regenerate querydefs/auth/queries.go (email queries)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating auth query definitions...")

	authQueryDefs, err := authgen.GenerateAuthQueryDefs(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate auth query defs: %v\n", err)
		os.Exit(1)
	}

	queryDefsDir := filepath.Join(roots.ShipqRoot, "querydefs", "auth")
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
	// Step 13: Regenerate config/config.go (SMTP env vars + APP_URL)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Regenerating config package...")

	filesEnabled := ini.Section("files") != nil
	workersEnabled := ini.Section("workers") != nil
	oauthGoogle := strings.ToLower(ini.Get("auth", "oauth_google")) == "true"
	oauthGitHub := strings.ToLower(ini.Get("auth", "oauth_github")) == "true"

	devDefaults := configpkg.DevDefaults{
		DatabaseURL:  databaseURL,
		Port:         "8080",
		CookieSecret: ini.Get("auth", "cookie_secret"),
		// Email
		SMTPHost:     ini.Get("email", "smtp_host"),
		SMTPPort:     ini.Get("email", "smtp_port"),
		SMTPUsername: ini.Get("email", "smtp_username"),
		SMTPPassword: ini.Get("email", "smtp_password"),
		AppURL:       ini.Get("email", "app_url"),
	}

	if oauthGoogle || oauthGitHub {
		devDefaults.OAuthRedirectURL = ini.Get("auth", "oauth_redirect_url")
		devDefaults.OAuthRedirectBaseURL = ini.Get("auth", "oauth_redirect_base_url")
	}

	if workersEnabled {
		devDefaults.RedisURL = ini.Get("workers", "redis_url")
		devDefaults.CentrifugoAPIURL = ini.Get("workers", "centrifugo_api_url")
		devDefaults.CentrifugoAPIKey = ini.Get("workers", "centrifugo_api_key")
		devDefaults.CentrifugoHMACSecret = ini.Get("workers", "centrifugo_hmac_secret")
		devDefaults.CentrifugoWSURL = ini.Get("workers", "centrifugo_ws_url")
	}

	if filesEnabled {
		devDefaults.S3Bucket = ini.Get("files", "s3_bucket")
		devDefaults.S3Region = ini.Get("files", "s3_region")
		devDefaults.S3Endpoint = ini.Get("files", "s3_endpoint")
		devDefaults.AWSAccessKeyID = ini.Get("files", "aws_access_key_id")
		devDefaults.AWSSecretAccessKey = ini.Get("files", "aws_secret_access_key")
		devDefaults.MaxUploadSizeMB = ini.Get("files", "max_upload_size_mb")
		devDefaults.MultipartThresholdMB = ini.Get("files", "multipart_threshold_mb")
	}

	if err := registry.GenerateConfigEarlyWithFullOptions(registry.ConfigEarlyOptions{
		ShipqRoot:      roots.ShipqRoot,
		GoModRoot:      roots.GoModRoot,
		Dialect:        dialect,
		FilesEnabled:   filesEnabled,
		WorkersEnabled: workersEnabled,
		OAuthGoogle:    oauthGoogle,
		OAuthGitHub:    oauthGitHub,
		EmailEnabled:   true,
		DevDefaults:    devDefaults,
		CustomEnvVars:  registry.ParseCustomEnvVars(ini),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Updated: config/config.go")

	// ---------------------------------------------------------------
	// Step 14: go mod tidy
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = roots.GoModRoot
	if tidyOut, tidyErr := tidyCmd.CombinedOutput(); tidyErr != nil {
		fmt.Fprintf(os.Stderr, "error: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
		os.Exit(1)
	}
	fmt.Println("  go mod tidy done")

	// ---------------------------------------------------------------
	// Step 15: db compile (recompile queries with email query methods)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// ---------------------------------------------------------------
	// Step 16: registry.Run (regenerates api/server.go, test client, etc.)
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to compile registry: %v\n", err)
		os.Exit(1)
	}

	// ---------------------------------------------------------------
	// Step 17: Success message
	// ---------------------------------------------------------------
	fmt.Println("")
	fmt.Println("Email system added successfully!")
	fmt.Println("")
	fmt.Println("New routes:")
	fmt.Println("  POST /auth/forgot-password      - Request a password reset email")
	fmt.Println("  POST /auth/reset-password        - Reset password with a valid token")
	fmt.Println("  POST /auth/verify-email          - Verify email address with a valid token")
	fmt.Println("  POST /auth/resend-verification   - Resend the verification email")
	fmt.Println("")
	fmt.Println("Environment variables required (production):")
	fmt.Println("  SMTP_HOST     - SMTP server hostname (e.g., smtp.postmark.app)")
	fmt.Println("  SMTP_PORT     - SMTP server port (e.g., 587)")
	fmt.Println("  SMTP_USERNAME - SMTP auth username")
	fmt.Println("  SMTP_PASSWORD - SMTP auth password")
	fmt.Println("  APP_URL       - Base URL for links in emails (e.g., https://app.example.com)")
	fmt.Println("")
	fmt.Println("Dev defaults (from shipq.ini [email] section):")
	fmt.Println("  smtp_host = localhost")
	fmt.Println("  smtp_port = 1025     (Mailpit/MailHog)")
	fmt.Println("  app_url   = http://localhost:3000")
}
