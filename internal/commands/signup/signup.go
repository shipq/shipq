package signup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/authgen"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
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

// SignupCmd handles "shipq signup" - generates the signup handler and route.
// This must be run after "shipq auth" has been configured.
func SignupCmd() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to get module info: %v\n", err)
		os.Exit(1)
	}
	modulePath := moduleInfo.FullImportPath("")

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse shipq.ini: %v\n", err)
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

	// Derive dialect and test database URL
	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	testDatabaseURL := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
		if u, err := dburl.TestDatabaseURL(databaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	// Detect whether email has been configured
	emailEnabled := ini.Section("email") != nil

	// Detect enabled OAuth providers BEFORE generating signup files, so that
	// register.go includes RegisterOAuthRoutes when OAuth is configured.
	oauthProviders := enabledOAuthProvidersFromIni(ini)

	authCfg := authgen.AuthGenConfig{
		ModulePath:      modulePath,
		Dialect:         dialect,
		TestDatabaseURL: testDatabaseURL,
		EmailEnabled:    emailEnabled,
		OAuthProviders:  oauthProviders,
		SignupEnabled:   true,
	}

	fmt.Println("Generating signup handler...")
	fmt.Println("")

	signupFiles, err := authgen.GenerateSignupFiles(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate signup files: %v\n", err)
		os.Exit(1)
	}

	// Write signup files to api/auth/
	authDir := filepath.Join(roots.ShipqRoot, "api", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/auth directory: %v\n", err)
		os.Exit(1)
	}

	for filename, content := range signupFiles {
		filePath := filepath.Join(authDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if changed {
			relPath, _ := filepath.Rel(roots.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	// If OAuth is enabled, regenerate OAuth files with SignupEnabled: true
	// so that the auto-create account path is unlocked.
	if len(oauthProviders) > 0 {
		fmt.Println("")
		fmt.Println("OAuth is enabled — regenerating OAuth shared utilities with signup support...")

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
			relPath, _ := filepath.Rel(roots.ShipqRoot, sharedPath)
			fmt.Printf("  Updated: %s\n", relPath)
		}

		for _, providerName := range oauthProviders {
			provider := authgen.ProviderByName(providerName)
			if provider == nil {
				continue
			}
			providerCode, err := authgen.GenerateOAuthProvider(authCfg, *provider)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to generate oauth_%s.go: %v\n", providerName, err)
				os.Exit(1)
			}
			providerPath := filepath.Join(authDir, fmt.Sprintf("oauth_%s.go", providerName))
			changed, err := codegen.WriteFileIfChanged(providerPath, providerCode)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write oauth_%s.go: %v\n", providerName, err)
				os.Exit(1)
			}
			if changed {
				relPath, _ := filepath.Rel(roots.ShipqRoot, providerPath)
				fmt.Printf("  Updated: %s\n", relPath)
			}
		}
	}

	// Recompile queries and handler registry
	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to compile registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("")
	fmt.Println("Signup handler added successfully!")
	fmt.Println("")
	fmt.Println("New route:")
	fmt.Println("  POST /signup - Create a new account")
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
