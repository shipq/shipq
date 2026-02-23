package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateOrganizationsMigration(t *testing.T) {
	timestamp := "20260205120000"
	modulePath := "github.com/example/myproject"
	code := generateOrganizationsMigration(timestamp, modulePath)
	codeStr := string(code)

	// Check package declaration
	if !strings.Contains(codeStr, "package migrations") {
		t.Error("missing package declaration")
	}

	// Check imports use embedded lib path, NOT github.com/shipq/shipq
	if !strings.Contains(codeStr, `"github.com/example/myproject/shipq/lib/db/portsql/ddl"`) {
		t.Error("missing embedded ddl import")
	}
	if !strings.Contains(codeStr, `"github.com/example/myproject/shipq/lib/db/portsql/migrate"`) {
		t.Error("missing embedded migrate import")
	}
	if strings.Contains(codeStr, `"github.com/shipq/shipq/`) {
		t.Error("generated code must NOT import from github.com/shipq/shipq")
	}

	// Check function name
	expectedFunc := "Migrate_20260205120000_organizations"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}

	// Check table name
	if !strings.Contains(codeStr, `AddTable("organizations"`) {
		t.Error("missing AddTable call for organizations")
	}

	// Check columns
	if !strings.Contains(codeStr, `tb.String("name")`) {
		t.Error("missing name column")
	}
	if !strings.Contains(codeStr, `tb.Text("description").Nullable()`) {
		t.Error("missing description column")
	}
}

func TestGenerateAccountsMigration(t *testing.T) {
	timestamp := "20260205120001"
	modulePath := "github.com/example/myproject"
	code := generateAccountsMigration(timestamp, modulePath)
	codeStr := string(code)

	// Check function name
	expectedFunc := "Migrate_20260205120001_accounts"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}

	// Check table reference lookup
	if !strings.Contains(codeStr, `plan.Table("organizations")`) {
		t.Error("missing organizations table reference lookup")
	}

	// Check table name
	if !strings.Contains(codeStr, `AddTable("accounts"`) {
		t.Error("missing AddTable call for accounts")
	}

	// Check columns
	if !strings.Contains(codeStr, `tb.String("first_name")`) {
		t.Error("missing first_name column")
	}
	if !strings.Contains(codeStr, `tb.String("last_name")`) {
		t.Error("missing last_name column")
	}
	if !strings.Contains(codeStr, `tb.String("email").Unique()`) {
		t.Error("missing email column with unique constraint")
	}
	if !strings.Contains(codeStr, `tb.Binary("password_hash")`) {
		t.Error("missing password_hash column")
	}
	if !strings.Contains(codeStr, `tb.Bigint("default_organization_id").References(organizationsRef).Nullable()`) {
		t.Error("missing default_organization_id column with reference")
	}
}

func TestGenerateOrganizationUsersMigration(t *testing.T) {
	timestamp := "20260205120002"
	modulePath := "github.com/example/myproject"
	code := generateOrganizationUsersMigration(timestamp, modulePath)
	codeStr := string(code)

	// Check function name
	expectedFunc := "Migrate_20260205120002_organization_users"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}

	// Check table reference lookups
	if !strings.Contains(codeStr, `plan.Table("organizations")`) {
		t.Error("missing organizations table reference lookup")
	}
	if !strings.Contains(codeStr, `plan.Table("accounts")`) {
		t.Error("missing accounts table reference lookup")
	}

	// Check AddTable (junction tables get standard columns like any resource)
	if !strings.Contains(codeStr, `AddTable("organization_users"`) {
		t.Error("missing AddTable call for organization_users")
	}

	// Check columns with references
	if !strings.Contains(codeStr, `tb.Bigint("organization_id").References(organizationsRef)`) {
		t.Error("missing organization_id column")
	}
	if !strings.Contains(codeStr, `tb.Bigint("account_id").References(accountsRef)`) {
		t.Error("missing account_id column")
	}

	// Check unique index
	if !strings.Contains(codeStr, `tb.AddUniqueIndex(orgIDCol, accountIDCol)`) {
		t.Error("missing unique index")
	}

	// Check junction table flag
	if !strings.Contains(codeStr, `tb.JunctionTable()`) {
		t.Error("missing JunctionTable() call")
	}
}

func TestGenerateSessionsMigration(t *testing.T) {
	timestamp := "20260205120003"
	modulePath := "github.com/example/myproject"
	code := generateSessionsMigration(timestamp, modulePath)
	codeStr := string(code)

	// Check function name
	expectedFunc := "Migrate_20260205120003_sessions"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}

	// Check table reference lookup
	if !strings.Contains(codeStr, `plan.Table("accounts")`) {
		t.Error("missing accounts table reference lookup")
	}

	// Check table name
	if !strings.Contains(codeStr, `AddTable("sessions"`) {
		t.Error("missing AddTable call for sessions")
	}

	// Check columns
	if !strings.Contains(codeStr, `tb.Bigint("account_id").References(accountsRef)`) {
		t.Error("missing account_id column with reference")
	}
	if !strings.Contains(codeStr, `tb.Datetime("expires_at")`) {
		t.Error("missing expires_at column")
	}

	// expires_at should NOT be nullable
	if strings.Contains(codeStr, `Datetime("expires_at").Nullable()`) {
		t.Error("expires_at should NOT be nullable")
	}
}

func TestGenerateRolesMigration_Unscoped(t *testing.T) {
	timestamp := "20260205120004"
	modulePath := "github.com/example/myproject"
	code := generateRolesMigration(timestamp, modulePath, "")
	codeStr := string(code)

	expectedFunc := "Migrate_20260205120004_roles"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}
	if !strings.Contains(codeStr, `AddTable("roles"`) {
		t.Error("missing AddTable call for roles")
	}
	if !strings.Contains(codeStr, `tb.String("name").Unique()`) {
		t.Error("missing name column with unique constraint")
	}
	if !strings.Contains(codeStr, `tb.Text("description").Nullable()`) {
		t.Error("missing description column")
	}
	// Unscoped should NOT have organization_id
	if strings.Contains(codeStr, "organization_id") {
		t.Error("unscoped roles should NOT have organization_id")
	}
}

func TestGenerateRolesMigration_Scoped(t *testing.T) {
	timestamp := "20260205120004"
	modulePath := "github.com/example/myproject"
	code := generateRolesMigration(timestamp, modulePath, "organization_id")
	codeStr := string(code)

	expectedFunc := "Migrate_20260205120004_roles"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}
	if !strings.Contains(codeStr, `AddTable("roles"`) {
		t.Error("missing AddTable call for roles")
	}
	if !strings.Contains(codeStr, `plan.Table("organizations")`) {
		t.Error("missing organizations table reference lookup")
	}
	if !strings.Contains(codeStr, `tb.Bigint("organization_id").References(organizationsRef).Nullable()`) {
		t.Error("missing nullable organization_id column")
	}
	if !strings.Contains(codeStr, `tb.String("name")`) {
		t.Error("missing name column")
	}
	if !strings.Contains(codeStr, `tb.Text("description").Nullable()`) {
		t.Error("missing description column")
	}
	if !strings.Contains(codeStr, `tb.AddUniqueIndex(orgIDCol, nameCol)`) {
		t.Error("missing unique index on (organization_id, name)")
	}
}

func TestGenerateAccountRolesMigration(t *testing.T) {
	timestamp := "20260205120005"
	modulePath := "github.com/example/myproject"
	code := generateAccountRolesMigration(timestamp, modulePath)
	codeStr := string(code)

	expectedFunc := "Migrate_20260205120005_account_roles"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}
	if !strings.Contains(codeStr, `plan.Table("accounts")`) {
		t.Error("missing accounts table reference lookup")
	}
	if !strings.Contains(codeStr, `plan.Table("roles")`) {
		t.Error("missing roles table reference lookup")
	}
	if !strings.Contains(codeStr, `AddTable("account_roles"`) {
		t.Error("missing AddTable call for account_roles")
	}
	if !strings.Contains(codeStr, `tb.Bigint("account_id").References(accountsRef)`) {
		t.Error("missing account_id column")
	}
	if !strings.Contains(codeStr, `tb.Bigint("role_id").References(rolesRef)`) {
		t.Error("missing role_id column")
	}
	if !strings.Contains(codeStr, `tb.AddUniqueIndex(accountIDCol, roleIDCol)`) {
		t.Error("missing unique index")
	}
	if !strings.Contains(codeStr, `tb.JunctionTable()`) {
		t.Error("missing JunctionTable() call")
	}
}

func TestGenerateRoleActionsMigration(t *testing.T) {
	timestamp := "20260205120006"
	modulePath := "github.com/example/myproject"
	code := generateRoleActionsMigration(timestamp, modulePath)
	codeStr := string(code)

	expectedFunc := "Migrate_20260205120006_role_actions"
	if !strings.Contains(codeStr, expectedFunc) {
		t.Errorf("missing function %s", expectedFunc)
	}
	if !strings.Contains(codeStr, `plan.Table("roles")`) {
		t.Error("missing roles table reference lookup")
	}
	if !strings.Contains(codeStr, `AddTable("role_actions"`) {
		t.Error("missing AddTable call for role_actions")
	}
	if !strings.Contains(codeStr, `tb.Bigint("role_id").References(rolesRef)`) {
		t.Error("missing role_id column")
	}
	if !strings.Contains(codeStr, `tb.String("route_path")`) {
		t.Error("missing route_path column")
	}
	if !strings.Contains(codeStr, `tb.String("method")`) {
		t.Error("missing method column")
	}
	if !strings.Contains(codeStr, `tb.AddUniqueIndex(roleIDCol, routePathCol, methodCol)`) {
		t.Error("missing unique index on (role_id, route_path, method)")
	}
	// role_actions should NOT have organization_id (scoping flows through roles)
	if strings.Contains(codeStr, "organization_id") {
		t.Error("role_actions should NOT have organization_id")
	}
}

func TestMigrationTimestampOrder(t *testing.T) {
	modulePath := "github.com/example/myproject"

	type migrationDef struct {
		name     string
		generate func(timestamp, modulePath string) []byte
	}
	migrations := []migrationDef{
		{"organizations", generateOrganizationsMigration},
		{"accounts", generateAccountsMigration},
		{"organization_users", generateOrganizationUsersMigration},
		{"sessions", generateSessionsMigration},
		{"roles", func(ts, mod string) []byte { return generateRolesMigration(ts, mod, "") }},
		{"account_roles", generateAccountRolesMigration},
		{"role_actions", generateRoleActionsMigration},
	}

	timestamps := []string{
		"20260205120000",
		"20260205120001",
		"20260205120002",
		"20260205120003",
		"20260205120004",
		"20260205120005",
		"20260205120006",
	}

	for i, m := range migrations {
		code := m.generate(timestamps[i], modulePath)
		if len(code) == 0 {
			t.Errorf("%s migration generated empty code", m.name)
		}
	}
}

func TestMigrationCodeCompiles(t *testing.T) {
	modulePath := "github.com/example/myproject"

	type migrationDef struct {
		name     string
		generate func(timestamp, modulePath string) []byte
	}
	migrations := []migrationDef{
		{"organizations", generateOrganizationsMigration},
		{"accounts", generateAccountsMigration},
		{"organization_users", generateOrganizationUsersMigration},
		{"sessions", generateSessionsMigration},
		{"roles_unscoped", func(ts, mod string) []byte { return generateRolesMigration(ts, mod, "") }},
		{"roles_scoped", func(ts, mod string) []byte { return generateRolesMigration(ts, mod, "organization_id") }},
		{"account_roles", generateAccountRolesMigration},
		{"role_actions", generateRoleActionsMigration},
	}

	timestamp := "20260205120000"
	for _, m := range migrations {
		code := m.generate(timestamp, modulePath)
		codeStr := string(code)

		if !strings.HasPrefix(codeStr, "package migrations") {
			t.Errorf("%s: missing package declaration", m.name)
		}

		openBraces := strings.Count(codeStr, "{")
		closeBraces := strings.Count(codeStr, "}")
		if openBraces != closeBraces {
			t.Errorf("%s: unbalanced braces (open: %d, close: %d)", m.name, openBraces, closeBraces)
		}

		openParens := strings.Count(codeStr, "(")
		closeParens := strings.Count(codeStr, ")")
		if openParens != closeParens {
			t.Errorf("%s: unbalanced parentheses (open: %d, close: %d)", m.name, openParens, closeParens)
		}
	}
}

func TestAuthMigrationsExist(t *testing.T) {
	t.Run("returns false for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if authMigrationsExist(dir) {
			t.Error("expected false for empty directory")
		}
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		if authMigrationsExist("/nonexistent/path/that/does/not/exist") {
			t.Error("expected false for non-existent directory")
		}
	})

	t.Run("returns false when only some migrations exist", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260205120000_organizations.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120001_accounts.go"), []byte("package m"), 0644)
		if authMigrationsExist(dir) {
			t.Error("expected false when only 2 of 7 migrations exist")
		}
	})

	t.Run("returns true when all seven migrations exist", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20260205120000_organizations.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120001_accounts.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120002_organization_users.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120003_sessions.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120004_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120005_account_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120006_role_actions.go"), []byte("package m"), 0644)
		if !authMigrationsExist(dir) {
			t.Error("expected true when all 7 migrations exist")
		}
	})

	t.Run("returns true with different timestamps", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "20250101000000_organizations.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20250601000000_accounts.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260101000000_organization_users.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120003_sessions.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120004_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120005_account_roles.go"), []byte("package m"), 0644)
		os.WriteFile(filepath.Join(dir, "20260205120006_role_actions.go"), []byte("package m"), 0644)
		if !authMigrationsExist(dir) {
			t.Error("expected true regardless of timestamps")
		}
	})
}
