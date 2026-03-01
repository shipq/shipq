package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/authgen"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	codegenMigrate "github.com/shipq/shipq/codegen/migrate"
	"github.com/shipq/shipq/codegen/seedgen"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

const (
	// DefaultMigrationsDir is the default directory for migration files.
	DefaultMigrationsDir = "migrations"
)

// ProjectConfig holds the loaded project configuration.
type ProjectConfig struct {
	GoModRoot      string
	ShipqRoot      string
	ModulePath     string
	MigrationsPath string
	DatabaseURL    string // from shipq.ini [db] database_url
	Dialect        string // inferred from DatabaseURL
	ScopeColumn    string // from shipq.ini [db] scope (e.g., "organization_id")
}

// loadProjectConfig finds project roots and loads configuration.
func loadProjectConfig() (*ProjectConfig, error) {
	roots, err := project.FindProjectRoots()
	if err != nil {
		return nil, err
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		return nil, err
	}
	modulePath := moduleInfo.FullImportPath("")

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return nil, err
	}

	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	migrationsPath := filepath.Join(roots.ShipqRoot, migrationsDir)

	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
	}

	scopeColumn := ini.Get("db", "scope")

	return &ProjectConfig{
		GoModRoot:      roots.GoModRoot,
		ShipqRoot:      roots.ShipqRoot,
		ModulePath:     modulePath,
		MigrationsPath: migrationsPath,
		DatabaseURL:    databaseURL,
		Dialect:        dialect,
		ScopeColumn:    scopeColumn,
	}, nil
}

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

// authMigrationsExist checks if all 4 auth migration files already exist in the
// migrations directory. This prevents duplicate migration generation when running
// `shipq auth` multiple times.
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

// AuthCmd handles "shipq auth" - generates auth tables and crypto utilities.
func AuthCmd() {
	cfg, err := loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdAuth, cfg.ShipqRoot) {
		os.Exit(1)
	}

	// Create migrations directory if needed
	if err := os.MkdirAll(cfg.MigrationsPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create migrations directory: %v\n", err)
		os.Exit(1)
	}

	if authMigrationsExist(cfg.MigrationsPath) {
		fmt.Println("Auth migrations already exist, skipping migration generation...")
		fmt.Println("")
		fmt.Println("Running migrations (in case they haven't been applied)...")
		up.MigrateUpCmd()
	} else {
		fmt.Println("Generating auth migrations...")
		fmt.Println("")

		// Generate timestamps with 1 second increments to ensure correct ordering.
		// Use NextMigrationBaseTime to avoid collisions with existing migrations.
		baseTime := codegenMigrate.NextMigrationBaseTime(cfg.MigrationsPath)
		timestamps := make([]string, 7)
		for i := range timestamps {
			timestamps[i] = baseTime.Add(time.Duration(i) * time.Second).Format("20060102150405")
		}

		// Generate the 7 auth migrations
		type migrationDef struct {
			name     string
			generate func(timestamp, modulePath string) []byte
		}
		migrations := []migrationDef{
			{"organizations", generateOrganizationsMigration},
			{"accounts", generateAccountsMigration},
			{"organization_users", generateOrganizationUsersMigration},
			{"sessions", generateSessionsMigration},
			{"roles", func(ts, mod string) []byte { return generateRolesMigration(ts, mod, cfg.ScopeColumn) }},
			{"account_roles", generateAccountRolesMigration},
			{"role_actions", generateRoleActionsMigration},
		}

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

		fmt.Println("")
		fmt.Println("Running migrations...")
		up.MigrateUpCmd()
	}

	// Set protect_by_default = true in shipq.ini so generated routes require auth
	fmt.Println("")
	fmt.Println("Updating shipq.ini with auth config...")
	shipqIniPath := filepath.Join(cfg.ShipqRoot, project.ShipqIniFile)
	ini, iniErr := inifile.ParseFile(shipqIniPath)
	if iniErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse shipq.ini: %v\n", iniErr)
		os.Exit(1)
	}
	ini.Set("auth", "protect_by_default", "true")

	// Generate and store a dev cookie_secret if one doesn't already exist.
	// This value is baked into the generated config as a compile-time dev default,
	// so the user doesn't need to set COOKIE_SECRET as an env var locally.
	if ini.Get("auth", "cookie_secret") == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to generate cookie secret: %v\n", err)
			os.Exit(1)
		}
		ini.Set("auth", "cookie_secret", hex.EncodeToString(b))
	}

	if writeErr := ini.WriteFile(shipqIniPath); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write shipq.ini: %v\n", writeErr)
		os.Exit(1)
	}
	fmt.Println("  Set [auth] protect_by_default = true")
	fmt.Println("  Set [auth] cookie_secret (dev default)")

	// STEP 1: Generate config package FIRST
	// Auth handlers import config for COOKIE_SECRET, so config must exist before handler compilation
	fmt.Println("")
	fmt.Println("Generating config package...")
	// Re-read ini to pick up the cookie_secret we just wrote
	ini, iniErr = inifile.ParseFile(shipqIniPath)
	if iniErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to re-read shipq.ini: %v\n", iniErr)
		os.Exit(1)
	}
	filesEnabled := ini.Section("files") != nil
	workersEnabled := ini.Section("workers") != nil
	if err := registry.GenerateConfigEarlyWithFullOptions(registry.ConfigEarlyOptions{
		ShipqRoot:      cfg.ShipqRoot,
		GoModRoot:      cfg.GoModRoot,
		Dialect:        cfg.Dialect,
		FilesEnabled:   filesEnabled,
		WorkersEnabled: workersEnabled,
		DevDefaults: configpkg.DevDefaults{
			DatabaseURL:  cfg.DatabaseURL,
			Port:         "8080",
			CookieSecret: ini.Get("auth", "cookie_secret"),
		},
		CustomEnvVars: registry.ParseCustomEnvVars(ini),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Created: config/config.go")

	// STEP 2: Generate auth handlers
	fmt.Println("")
	fmt.Println("Generating auth handlers...")
	fmt.Println("")

	// Derive test database URL from the dev URL in shipq.ini
	testDatabaseURL := ""
	if cfg.DatabaseURL != "" {
		if u, err := dburl.TestDatabaseURL(cfg.DatabaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	authCfg := authgen.AuthGenConfig{
		ModulePath:      cfg.ModulePath,
		Dialect:         cfg.Dialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     cfg.ScopeColumn,
	}

	handlerFiles, err := authgen.GenerateAuthHandlerFiles(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate auth handlers: %v\n", err)
		os.Exit(1)
	}

	// Create api/auth directory
	authDir := filepath.Join(cfg.ShipqRoot, "api", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/auth directory: %v\n", err)
		os.Exit(1)
	}

	// Write handler files
	for filename, content := range handlerFiles {
		filePath := filepath.Join(authDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if changed {
			relPath, _ := filepath.Rel(cfg.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	// Write .shipq-no-regen marker so "shipq resource up" won't overwrite
	// the auth handlers with generic CRUD handlers.
	markerPath := filepath.Join(authDir, ".shipq-no-regen")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		markerContent := "# This file prevents shipq from regenerating handlers in this directory.\n# Auth handlers are custom and should not be overwritten by CRUD generation.\n# Delete this file if you want shipq to regenerate the handlers.\n"
		if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", markerPath, err)
			os.Exit(1)
		}
	}

	// STEP 2.5: Generate auth query definitions (querydefs/auth/queries.go)
	// Auth handlers use custom queries (FindAccountByEmail, FindActiveSession, etc.)
	// that must be compiled into the query runner before handlers can compile.
	fmt.Println("")
	fmt.Println("Generating auth query definitions...")
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
	fmt.Println("  Created: querydefs/auth/queries.go")

	// STEP 2.6: Run go mod tidy so the generated code's imports resolve
	// (querydefs need github.com/shipq/shipq/db/portsql/query,
	//  handlers need golang.org/x/crypto/argon2, etc.)
	fmt.Println("")
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = cfg.GoModRoot
	if tidyOut, tidyErr := tidyCmd.CombinedOutput(); tidyErr != nil {
		fmt.Fprintf(os.Stderr, "error: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
		os.Exit(1)
	}
	fmt.Println("  go mod tidy done")

	// STEP 2.7: Run db compile to regenerate the query runner with auth query methods
	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// STEP 2.8: Generate organizations fixture
	// Organizations are auth-managed (no public CRUD routes), so the fixture
	// creates orgs directly via the query runner. This must run after db compile
	// so the queries package exists.
	fmt.Println("")
	fmt.Println("Generating organizations fixture...")
	orgFixtureCode, err := authgen.GenerateOrganizationFixture(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate organizations fixture: %v\n", err)
		os.Exit(1)
	}

	orgFixtureDir := filepath.Join(cfg.ShipqRoot, "api", "organizations", "fixture")
	if err := os.MkdirAll(orgFixtureDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/organizations/fixture directory: %v\n", err)
		os.Exit(1)
	}

	orgFixturePath := filepath.Join(orgFixtureDir, "fixture.go")
	written, writeErr := codegen.WriteFileIfChanged(orgFixturePath, orgFixtureCode)
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write organizations fixture: %v\n", writeErr)
		os.Exit(1)
	}
	if written {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, orgFixturePath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// STEP 2.8b: Generate accounts fixture
	// Accounts are auth-managed (no public CRUD routes), so the fixture
	// creates accounts directly via the query runner. It depends on the
	// organization fixture because accounts have a FK to organizations.
	fmt.Println("")
	fmt.Println("Generating accounts fixture...")
	acctFixtureCode, err := authgen.GenerateAccountFixture(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate accounts fixture: %v\n", err)
		os.Exit(1)
	}

	acctFixtureDir := filepath.Join(cfg.ShipqRoot, "api", "accounts", "fixture")
	if err := os.MkdirAll(acctFixtureDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/accounts/fixture directory: %v\n", err)
		os.Exit(1)
	}

	acctFixturePath := filepath.Join(acctFixtureDir, "fixture.go")
	acctWritten, acctWriteErr := codegen.WriteFileIfChanged(acctFixturePath, acctFixtureCode)
	if acctWriteErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write accounts fixture: %v\n", acctWriteErr)
		os.Exit(1)
	}
	if acctWritten {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, acctFixturePath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// STEP 2.9: Generate dev auth seed file
	fmt.Println("")
	fmt.Println("Generating dev auth seed...")
	seedCfg := seedgen.SeedGenConfig{
		ModulePath:  cfg.ModulePath,
		Dialect:     cfg.Dialect,
		ScopeColumn: cfg.ScopeColumn,
	}
	seedCode, seedErr := seedgen.GenerateDevAuthSeed(seedCfg)
	if seedErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate dev auth seed: %v\n", seedErr)
		os.Exit(1)
	}

	seedsDir := filepath.Join(cfg.ShipqRoot, "seeds")
	if err := os.MkdirAll(seedsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create seeds directory: %v\n", err)
		os.Exit(1)
	}

	seedPath := filepath.Join(seedsDir, "dev_auth_seed.go")
	seedWritten, seedWriteErr := codegen.WriteFileIfChanged(seedPath, seedCode)
	if seedWriteErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write dev auth seed: %v\n", seedWriteErr)
		os.Exit(1)
	}
	if seedWritten {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, seedPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// STEP 3: Compile handler registry (generates api/server.go, test client, etc.)
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(cfg.ShipqRoot, cfg.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to compile registry: %v\n", err)
		os.Exit(1)
	}

	// STEP 4: Generate auth tests AFTER api package exists
	// Auth tests import the api package (for test client), so they must be generated after registry.Run
	fmt.Println("")
	fmt.Println("Generating auth tests...")
	testFiles, err := authgen.GenerateAuthTestFiles(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate auth tests: %v\n", err)
		os.Exit(1)
	}

	// Create api/auth/spec directory
	authTestDir := filepath.Join(cfg.ShipqRoot, "api", "auth", "spec")
	if err := os.MkdirAll(authTestDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/auth/spec directory: %v\n", err)
		os.Exit(1)
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(authTestDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if changed {
			relPath, _ := filepath.Rel(cfg.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	fmt.Println("")
	fmt.Println("Auth system created successfully!")
	fmt.Println("")
	fmt.Println("Generated routes:")
	fmt.Println("  POST   /login   - Log in with email/password")
	fmt.Println("  GET    /me      - Get current user info")
	fmt.Println("  DELETE /logout  - Log out and clear session")
	fmt.Println("")
	fmt.Println("To add signup, run: shipq signup")
	fmt.Println("")
	fmt.Println("Environment variable required:")
	fmt.Println("  COOKIE_SECRET - Secret key for signing session cookies")
	fmt.Println("")
	fmt.Println("To run tests:")
	fmt.Println("  go test ./api/auth/spec/...")
}

func generateOrganizationsMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_organizations(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("organizations", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.Text("description").Nullable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateAccountsMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_accounts(plan *migrate.MigrationPlan) error {
	organizationsRef, err := plan.Table("organizations")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("accounts", func(tb *ddl.TableBuilder) error {
		tb.String("first_name")
		tb.String("last_name")
		tb.String("email").Unique()
		tb.Binary("password_hash").Nullable()
		tb.Bigint("default_organization_id").References(organizationsRef).Nullable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateOrganizationUsersMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_organization_users(plan *migrate.MigrationPlan) error {
	organizationsRef, err := plan.Table("organizations")
	if err != nil {
		return err
	}

	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("organization_users", func(tb *ddl.TableBuilder) error {
		orgIDCol := tb.Bigint("organization_id").References(organizationsRef).Col()
		accountIDCol := tb.Bigint("account_id").References(accountsRef).Col()
		tb.AddUniqueIndex(orgIDCol, accountIDCol)
		tb.JunctionTable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateSessionsMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_sessions(plan *migrate.MigrationPlan) error {
	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("sessions", func(tb *ddl.TableBuilder) error {
		tb.Bigint("account_id").References(accountsRef)
		tb.Datetime("expires_at")
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateRolesMigration(timestamp, modulePath, scopeColumn string) []byte {
	if scopeColumn != "" {
		// Scoped variant: roles have a nullable organization_id for per-org roles.
		// System-level roles (e.g., GLOBAL_OWNER) use organization_id = NULL.
		return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_roles(plan *migrate.MigrationPlan) error {
	organizationsRef, err := plan.Table("organizations")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("roles", func(tb *ddl.TableBuilder) error {
		orgIDCol := tb.Bigint("organization_id").References(organizationsRef).Nullable().Col()
		nameCol := tb.String("name").Col()
		tb.Text("description").Nullable()
		tb.AddUniqueIndex(orgIDCol, nameCol)
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
	}

	// Unscoped variant: role names are globally unique.
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_roles(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("roles", func(tb *ddl.TableBuilder) error {
		tb.String("name").Unique()
		tb.Text("description").Nullable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateAccountRolesMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_account_roles(plan *migrate.MigrationPlan) error {
	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	rolesRef, err := plan.Table("roles")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("account_roles", func(tb *ddl.TableBuilder) error {
		accountIDCol := tb.Bigint("account_id").References(accountsRef).Col()
		roleIDCol := tb.Bigint("role_id").References(rolesRef).Col()
		tb.AddUniqueIndex(accountIDCol, roleIDCol)
		tb.JunctionTable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateRoleActionsMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_role_actions(plan *migrate.MigrationPlan) error {
	rolesRef, err := plan.Table("roles")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("role_actions", func(tb *ddl.TableBuilder) error {
		roleIDCol := tb.Bigint("role_id").References(rolesRef).Col()
		routePathCol := tb.String("route_path").Col()
		methodCol := tb.String("method").Col()
		tb.AddUniqueIndex(roleIDCol, routePathCol, methodCol)
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}
