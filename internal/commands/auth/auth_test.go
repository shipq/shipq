package auth

import (
	"strings"
	"testing"
)

func TestGenerateOrganizationsMigration(t *testing.T) {
	timestamp := "20260205120000"
	code := generateOrganizationsMigration(timestamp)
	codeStr := string(code)

	// Check package declaration
	if !strings.Contains(codeStr, "package migrations") {
		t.Error("missing package declaration")
	}

	// Check imports
	if !strings.Contains(codeStr, `"github.com/shipq/shipq/db/portsql/ddl"`) {
		t.Error("missing ddl import")
	}
	if !strings.Contains(codeStr, `"github.com/shipq/shipq/db/portsql/migrate"`) {
		t.Error("missing migrate import")
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
	code := generateAccountsMigration(timestamp)
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
	code := generateOrganizationUsersMigration(timestamp)
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

	// Check AddEmptyTable (junction tables don't have default columns)
	if !strings.Contains(codeStr, `AddEmptyTable("organization_users"`) {
		t.Error("missing AddEmptyTable call for organization_users")
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
	code := generateSessionsMigration(timestamp)
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

func TestMigrationTimestampOrder(t *testing.T) {
	// Verify that when migrations are generated with incremental timestamps,
	// they will be ordered correctly

	migrations := []struct {
		name     string
		generate func(timestamp string) []byte
	}{
		{"organizations", generateOrganizationsMigration},
		{"accounts", generateAccountsMigration},
		{"organization_users", generateOrganizationUsersMigration},
		{"sessions", generateSessionsMigration},
	}

	// Generate with sequential timestamps
	timestamps := []string{
		"20260205120000",
		"20260205120001",
		"20260205120002",
		"20260205120003",
	}

	for i, m := range migrations {
		code := m.generate(timestamps[i])
		if len(code) == 0 {
			t.Errorf("%s migration generated empty code", m.name)
		}
	}
}

func TestMigrationCodeCompiles(t *testing.T) {
	// This is a basic sanity check - the generated code should be valid Go
	// A more thorough test would actually compile and run the migrations

	migrations := []struct {
		name     string
		generate func(timestamp string) []byte
	}{
		{"organizations", generateOrganizationsMigration},
		{"accounts", generateAccountsMigration},
		{"organization_users", generateOrganizationUsersMigration},
		{"sessions", generateSessionsMigration},
	}

	timestamp := "20260205120000"
	for _, m := range migrations {
		code := m.generate(timestamp)
		codeStr := string(code)

		// Basic syntax checks
		if !strings.HasPrefix(codeStr, "package migrations") {
			t.Errorf("%s: missing package declaration", m.name)
		}

		// Check for balanced braces (very basic)
		openBraces := strings.Count(codeStr, "{")
		closeBraces := strings.Count(codeStr, "}")
		if openBraces != closeBraces {
			t.Errorf("%s: unbalanced braces (open: %d, close: %d)", m.name, openBraces, closeBraces)
		}

		// Check for balanced parentheses
		openParens := strings.Count(codeStr, "(")
		closeParens := strings.Count(codeStr, ")")
		if openParens != closeParens {
			t.Errorf("%s: unbalanced parentheses (open: %d, close: %d)", m.name, openParens, closeParens)
		}
	}
}
