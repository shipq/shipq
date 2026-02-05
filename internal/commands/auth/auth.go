package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/authgen"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/migrate/up"
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
}

// loadProjectConfig finds project roots and loads configuration.
func loadProjectConfig() (*ProjectConfig, error) {
	roots, err := project.FindProjectRoots()
	if err != nil {
		return nil, err
	}

	modulePath, err := codegen.GetModulePath(roots.GoModRoot)
	if err != nil {
		return nil, err
	}

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

	return &ProjectConfig{
		GoModRoot:      roots.GoModRoot,
		ShipqRoot:      roots.ShipqRoot,
		ModulePath:     modulePath,
		MigrationsPath: migrationsPath,
	}, nil
}

// AuthCmd handles "shipq auth" - generates auth tables and crypto utilities.
func AuthCmd() {
	cfg, err := loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// Create migrations directory if needed
	if err := os.MkdirAll(cfg.MigrationsPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create migrations directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating auth migrations...")
	fmt.Println("")

	// Generate timestamps with 1 second increments to ensure correct ordering
	baseTime := time.Now().UTC()
	timestamps := []string{
		baseTime.Format("20060102150405"),
		baseTime.Add(1 * time.Second).Format("20060102150405"),
		baseTime.Add(2 * time.Second).Format("20060102150405"),
		baseTime.Add(3 * time.Second).Format("20060102150405"),
	}

	// Generate the 4 auth migrations
	migrations := []struct {
		name     string
		generate func(timestamp string) []byte
	}{
		{"organizations", generateOrganizationsMigration},
		{"accounts", generateAccountsMigration},
		{"organization_users", generateOrganizationUsersMigration},
		{"sessions", generateSessionsMigration},
	}

	for i, m := range migrations {
		code := m.generate(timestamps[i])
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

	fmt.Println("")
	fmt.Println("Generating auth handlers...")
	fmt.Println("")

	// Generate auth handlers
	authCfg := authgen.AuthGenConfig{
		ModulePath: cfg.ModulePath,
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

	// Generate auth tests
	testFiles, err := authgen.GenerateAuthTestFiles(authCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate auth tests: %v\n", err)
		os.Exit(1)
	}

	// Create api/auth_test directory
	authTestDir := filepath.Join(cfg.ShipqRoot, "api", "auth_test")
	if err := os.MkdirAll(authTestDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/auth_test directory: %v\n", err)
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
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(cfg.ShipqRoot, cfg.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to compile registry: %v\n", err)
		// Don't exit - handler generation succeeded
	}

	fmt.Println("")
	fmt.Println("Auth system created successfully!")
	fmt.Println("")
	fmt.Println("Generated routes:")
	fmt.Println("  POST   /login   - Log in with email/password")
	fmt.Println("  POST   /signup  - Create a new account")
	fmt.Println("  GET    /me      - Get current user info")
	fmt.Println("  DELETE /logout  - Log out and clear session")
	fmt.Println("")
	fmt.Println("Environment variable required:")
	fmt.Println("  COOKIE_SECRET - Secret key for signing session cookies")
	fmt.Println("")
	fmt.Println("To run tests:")
	fmt.Println("  go test ./api/auth_test/...")
}

func generateOrganizationsMigration(timestamp string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_%s_organizations(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("organizations", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.Text("description").Nullable()
		return nil
	})
	return err
}
`, timestamp))
}

func generateAccountsMigration(timestamp string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
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
		tb.Binary("password_hash")
		tb.Bigint("default_organization_id").References(organizationsRef).Nullable()
		return nil
	})
	return err
}
`, timestamp))
}

func generateOrganizationUsersMigration(timestamp string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
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

	_, err = plan.AddEmptyTable("organization_users", func(tb *ddl.TableBuilder) error {
		orgIDCol := tb.Bigint("organization_id").References(organizationsRef).Col()
		accountIDCol := tb.Bigint("account_id").References(accountsRef).Col()
		tb.AddUniqueIndex(orgIDCol, accountIDCol)
		tb.JunctionTable()
		return nil
	})
	return err
}
`, timestamp))
}

func generateSessionsMigration(timestamp string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
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
`, timestamp))
}
