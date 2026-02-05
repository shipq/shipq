package crud

import (
	"os"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/inifile"
)

func parseINI(t *testing.T, content string) *inifile.File {
	t.Helper()
	ini, err := inifile.Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to parse INI: %v", err)
	}
	return ini
}

func TestLoadCRUDConfig_GlobalDefaults(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = postgres://localhost:5432/myapp
scope = organization_id
order = desc
`)
	tables := []string{"users", "posts"}
	cfg, err := LoadCRUDConfig(ini, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check global values
	if cfg.GlobalScope != "organization_id" {
		t.Errorf("GlobalScope = %q, want %q", cfg.GlobalScope, "organization_id")
	}
	if cfg.GlobalScopeTable != "organizations" {
		t.Errorf("GlobalScopeTable = %q, want %q", cfg.GlobalScopeTable, "organizations")
	}
	if cfg.GlobalOrderAsc != false {
		t.Errorf("GlobalOrderAsc = %v, want false", cfg.GlobalOrderAsc)
	}

	// Check that all tables inherit global defaults
	for _, tableName := range tables {
		opts := cfg.TableOpts[tableName]
		if opts.ScopeColumn != "organization_id" {
			t.Errorf("%s: ScopeColumn = %q, want %q", tableName, opts.ScopeColumn, "organization_id")
		}
		if opts.OrderAsc != false {
			t.Errorf("%s: OrderAsc = %v, want false", tableName, opts.OrderAsc)
		}
	}
}

func TestLoadCRUDConfig_GlobalOrderAsc(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = postgres://localhost:5432/myapp
order = asc
`)
	tables := []string{"events"}
	cfg, err := LoadCRUDConfig(ini, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GlobalOrderAsc != true {
		t.Errorf("GlobalOrderAsc = %v, want true", cfg.GlobalOrderAsc)
	}
	if cfg.TableOpts["events"].OrderAsc != true {
		t.Errorf("events.OrderAsc = %v, want true", cfg.TableOpts["events"].OrderAsc)
	}
}

func TestLoadCRUDConfig_PerTableOverride(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = postgres://localhost:5432/myapp
scope = organization_id
order = desc

[crud.events]
order = asc

[crud.audit_logs]
scope = tenant_id
order = asc
`)
	tables := []string{"users", "events", "audit_logs"}
	cfg, err := LoadCRUDConfig(ini, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// users: inherits global defaults
	if cfg.TableOpts["users"].ScopeColumn != "organization_id" {
		t.Errorf("users.ScopeColumn = %q, want %q", cfg.TableOpts["users"].ScopeColumn, "organization_id")
	}
	if cfg.TableOpts["users"].OrderAsc != false {
		t.Errorf("users.OrderAsc = %v, want false", cfg.TableOpts["users"].OrderAsc)
	}

	// events: overrides order only
	if cfg.TableOpts["events"].ScopeColumn != "organization_id" {
		t.Errorf("events.ScopeColumn = %q, want %q", cfg.TableOpts["events"].ScopeColumn, "organization_id")
	}
	if cfg.TableOpts["events"].OrderAsc != true {
		t.Errorf("events.OrderAsc = %v, want true", cfg.TableOpts["events"].OrderAsc)
	}

	// audit_logs: overrides both scope and order
	if cfg.TableOpts["audit_logs"].ScopeColumn != "tenant_id" {
		t.Errorf("audit_logs.ScopeColumn = %q, want %q", cfg.TableOpts["audit_logs"].ScopeColumn, "tenant_id")
	}
	if cfg.TableOpts["audit_logs"].OrderAsc != true {
		t.Errorf("audit_logs.OrderAsc = %v, want true", cfg.TableOpts["audit_logs"].OrderAsc)
	}
}

func TestLoadCRUDConfig_ExplicitScopeTable(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = postgres://localhost:5432/myapp
scope = org_id
scope_table = organizations
`)
	tables := []string{"users"}
	cfg, err := LoadCRUDConfig(ini, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GlobalScope != "org_id" {
		t.Errorf("GlobalScope = %q, want %q", cfg.GlobalScope, "org_id")
	}
	if cfg.GlobalScopeTable != "organizations" {
		t.Errorf("GlobalScopeTable = %q, want %q (should use explicit value, not inferred)", cfg.GlobalScopeTable, "organizations")
	}
}

func TestLoadCRUDConfig_NoScopeConfigured(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = postgres://localhost:5432/myapp
`)
	tables := []string{"users"}
	cfg, err := LoadCRUDConfig(ini, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GlobalScope != "" {
		t.Errorf("GlobalScope = %q, want empty", cfg.GlobalScope)
	}
	if cfg.TableOpts["users"].ScopeColumn != "" {
		t.Errorf("users.ScopeColumn = %q, want empty", cfg.TableOpts["users"].ScopeColumn)
	}
}

func TestInferScopeTable(t *testing.T) {
	tests := []struct {
		column   string
		expected string
	}{
		{"organization_id", "organizations"},
		{"tenant_id", "tenants"},
		{"user_id", "users"},
		{"company_id", "companies"},
		{"category_id", "categories"},
		{"post_id", "posts"},
		{"address_id", "addresses"},
		{"box_id", "boxes"},
		{"brush_id", "brushes"},
		{"church_id", "churches"},
		// Edge case: column without _id suffix
		{"org", "orgs"},
	}

	for _, tt := range tests {
		got := InferScopeTable(tt.column)
		if got != tt.expected {
			t.Errorf("InferScopeTable(%q) = %q, want %q", tt.column, got, tt.expected)
		}
	}
}

func TestToPlural(t *testing.T) {
	tests := []struct {
		singular string
		expected string
	}{
		{"organization", "organizations"},
		{"user", "users"},
		{"post", "posts"},
		{"company", "companies"},
		{"category", "categories"},
		{"box", "boxes"},
		{"brush", "brushes"},
		{"church", "churches"},
		{"dish", "dishes"},
		{"buzz", "buzzes"},
		{"person", "people"},
		{"child", "children"},
		{"day", "days"},
		{"key", "keys"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ToPlural(tt.singular)
		if got != tt.expected {
			t.Errorf("ToPlural(%q) = %q, want %q", tt.singular, got, tt.expected)
		}
	}
}

func TestValidateScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	// Valid: scope column exists
	if err := ValidateScopeColumn(table, "organization_id"); err != nil {
		t.Errorf("expected nil error for existing column, got: %v", err)
	}

	// Valid: no scope configured
	if err := ValidateScopeColumn(table, ""); err != nil {
		t.Errorf("expected nil error for empty scope, got: %v", err)
	}

	// Invalid: scope column doesn't exist
	if err := ValidateScopeColumn(table, "tenant_id"); err == nil {
		t.Error("expected error for missing column, got nil")
	}
}

func TestFilterScopeForTable(t *testing.T) {
	usersTable := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	organizationsTable := ddl.Table{
		Name: "organizations",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	// Table with scope column -> returns scope
	if got := FilterScopeForTable(usersTable, "organization_id"); got != "organization_id" {
		t.Errorf("FilterScopeForTable(users, organization_id) = %q, want %q", got, "organization_id")
	}

	// Table without scope column -> returns empty (global table)
	if got := FilterScopeForTable(organizationsTable, "organization_id"); got != "" {
		t.Errorf("FilterScopeForTable(organizations, organization_id) = %q, want empty", got)
	}

	// No scope configured -> returns empty
	if got := FilterScopeForTable(usersTable, ""); got != "" {
		t.Errorf("FilterScopeForTable(users, \"\") = %q, want empty", got)
	}
}

func TestApplyScopeFiltering(t *testing.T) {
	tables := map[string]ddl.Table{
		"users": {
			Name: "users",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "organization_id", Type: ddl.BigintType},
				{Name: "name", Type: ddl.StringType},
			},
		},
		"organizations": {
			Name: "organizations",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "name", Type: ddl.StringType},
			},
		},
		"posts": {
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "organization_id", Type: ddl.BigintType},
				{Name: "title", Type: ddl.StringType},
			},
		},
	}

	ini := parseINI(t, `
[db]
scope = organization_id
`)
	cfg, err := LoadCRUDConfig(ini, []string{"users", "organizations", "posts"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Before filtering, all tables have the scope column set
	if cfg.TableOpts["organizations"].ScopeColumn != "organization_id" {
		t.Errorf("before filtering: organizations.ScopeColumn = %q, want %q", cfg.TableOpts["organizations"].ScopeColumn, "organization_id")
	}

	// Apply scope filtering
	ApplyScopeFiltering(cfg, tables)

	// After filtering:
	// - users has organization_id column -> scope applied
	if cfg.TableOpts["users"].ScopeColumn != "organization_id" {
		t.Errorf("after filtering: users.ScopeColumn = %q, want %q", cfg.TableOpts["users"].ScopeColumn, "organization_id")
	}

	// - organizations doesn't have organization_id column -> no scope (global table)
	if cfg.TableOpts["organizations"].ScopeColumn != "" {
		t.Errorf("after filtering: organizations.ScopeColumn = %q, want empty (global table)", cfg.TableOpts["organizations"].ScopeColumn)
	}

	// - posts has organization_id column -> scope applied
	if cfg.TableOpts["posts"].ScopeColumn != "organization_id" {
		t.Errorf("after filtering: posts.ScopeColumn = %q, want %q", cfg.TableOpts["posts"].ScopeColumn, "organization_id")
	}
}

func TestLoadCRUDConfigWithTables(t *testing.T) {
	// Create a temporary directory with a shipq.ini file
	tempDir := t.TempDir()

	// Write a test shipq.ini
	iniContent := `[db]
database_url = postgres://localhost:5432/testdb
scope = organization_id
order = desc

[crud.events]
order = asc
`
	iniPath := tempDir + "/shipq.ini"
	if err := os.WriteFile(iniPath, []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write test shipq.ini: %v", err)
	}

	// Create test tables
	tables := map[string]ddl.Table{
		"users": {
			Name: "users",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "organization_id", Type: ddl.BigintType},
				{Name: "name", Type: ddl.StringType},
			},
		},
		"organizations": {
			Name: "organizations",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "name", Type: ddl.StringType},
			},
		},
		"events": {
			Name: "events",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "organization_id", Type: ddl.BigintType},
				{Name: "type", Type: ddl.StringType},
			},
		},
	}

	tableNames := []string{"users", "organizations", "events"}

	cfg, err := LoadCRUDConfigWithTables(tempDir, tableNames, tables)
	if err != nil {
		t.Fatalf("LoadCRUDConfigWithTables failed: %v", err)
	}

	// users has organization_id -> scope applied
	if cfg.TableOpts["users"].ScopeColumn != "organization_id" {
		t.Errorf("users.ScopeColumn = %q, want %q", cfg.TableOpts["users"].ScopeColumn, "organization_id")
	}
	if cfg.TableOpts["users"].OrderAsc != false {
		t.Errorf("users.OrderAsc = %v, want false", cfg.TableOpts["users"].OrderAsc)
	}

	// organizations doesn't have organization_id -> no scope (global table)
	if cfg.TableOpts["organizations"].ScopeColumn != "" {
		t.Errorf("organizations.ScopeColumn = %q, want empty (global table)", cfg.TableOpts["organizations"].ScopeColumn)
	}

	// events has organization_id and overrides order
	if cfg.TableOpts["events"].ScopeColumn != "organization_id" {
		t.Errorf("events.ScopeColumn = %q, want %q", cfg.TableOpts["events"].ScopeColumn, "organization_id")
	}
	if cfg.TableOpts["events"].OrderAsc != true {
		t.Errorf("events.OrderAsc = %v, want true (overridden)", cfg.TableOpts["events"].OrderAsc)
	}
}
