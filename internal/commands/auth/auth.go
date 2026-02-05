package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	"github.com/shipq/shipq/project"
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
	fmt.Println("Auth tables created successfully!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  1. Use crypto.HashPassword() to hash passwords before storing")
	fmt.Println("  2. Use crypto.VerifyPassword() to verify passwords on login")
	fmt.Println("  3. Use crypto.SignCookie() to sign session cookies")
	fmt.Println("  4. Use crypto.VerifyCookie() to verify session cookies")
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
