package generator

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/internal/commands/migrate/parser"
)

func TestGenerateMigration_WithScopeColumn(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "posts",
		Timestamp:     "20240115120000",
		Columns:       []parser.ColumnSpec{{Name: "title", Type: "string"}},
		ScopeColumn:   "organization_id",
		ScopeTable:    "organizations",
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check that scope column is injected
	if !strings.Contains(codeStr, `tb.Bigint("organization_id").References(organizationsRef)`) {
		t.Errorf("missing organization_id reference, got:\n%s", codeStr)
	}

	// Check that organizationsRef lookup is present
	if !strings.Contains(codeStr, `organizationsRef, err := plan.Table("organizations")`) {
		t.Errorf("missing organizations table lookup, got:\n%s", codeStr)
	}

	// Check for the auto-added comment
	if !strings.Contains(codeStr, "Auto-added: global scope") {
		t.Errorf("missing auto-added comment, got:\n%s", codeStr)
	}

	// Check that user column is also present
	if !strings.Contains(codeStr, `tb.String("title")`) {
		t.Errorf("missing title column, got:\n%s", codeStr)
	}
}

func TestGenerateMigration_GlobalFlag_NoScope(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "accounts",
		Timestamp:     "20240115120000",
		Columns:       []parser.ColumnSpec{{Name: "name", Type: "string"}},
		ScopeColumn:   "organization_id",
		ScopeTable:    "organizations",
		IsGlobal:      true, // --global flag
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check that scope column is NOT injected
	if strings.Contains(codeStr, "organization_id") {
		t.Errorf("global table should not have organization_id, got:\n%s", codeStr)
	}

	// Check that organizationsRef lookup is NOT present
	if strings.Contains(codeStr, "organizationsRef") {
		t.Errorf("global table should not have organizationsRef lookup, got:\n%s", codeStr)
	}

	// Check that user column is still present
	if !strings.Contains(codeStr, `tb.String("name")`) {
		t.Errorf("missing name column, got:\n%s", codeStr)
	}
}

func TestGenerateMigration_NoScopeConfigured(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "posts",
		Timestamp:     "20240115120000",
		Columns:       []parser.ColumnSpec{{Name: "title", Type: "string"}},
		ScopeColumn:   "", // No scope configured
		ScopeTable:    "",
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check that no scope column is injected
	if strings.Contains(codeStr, "organization_id") {
		t.Errorf("should not have organization_id without scope config, got:\n%s", codeStr)
	}

	// Check that user column is present
	if !strings.Contains(codeStr, `tb.String("title")`) {
		t.Errorf("missing title column, got:\n%s", codeStr)
	}
}

func TestGenerateMigration_ScopeColumnFirst(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "posts",
		Timestamp:     "20240115120000",
		Columns: []parser.ColumnSpec{
			{Name: "title", Type: "string"},
			{Name: "body", Type: "text"},
		},
		ScopeColumn: "organization_id",
		ScopeTable:  "organizations",
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Scope column should appear before title column
	scopeIdx := strings.Index(codeStr, "organization_id")
	titleIdx := strings.Index(codeStr, "title")

	if scopeIdx == -1 || titleIdx == -1 {
		t.Errorf("missing expected columns, got:\n%s", codeStr)
	}

	if scopeIdx >= titleIdx {
		t.Errorf("scope column should appear before user columns, scopeIdx=%d, titleIdx=%d", scopeIdx, titleIdx)
	}
}

func TestGenerateMigration_ScopeWithExistingReferences(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "posts",
		Timestamp:     "20240115120000",
		Columns: []parser.ColumnSpec{
			{Name: "title", Type: "string"},
			{Name: "user_id", Type: "references", References: "users"},
		},
		ScopeColumn: "organization_id",
		ScopeTable:  "organizations",
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Both table lookups should be present
	if !strings.Contains(codeStr, `organizationsRef, err := plan.Table("organizations")`) {
		t.Errorf("missing organizations table lookup, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `usersRef, err := plan.Table("users")`) {
		t.Errorf("missing users table lookup, got:\n%s", codeStr)
	}

	// Both references should be present
	if !strings.Contains(codeStr, `tb.Bigint("organization_id").References(organizationsRef)`) {
		t.Errorf("missing organization_id reference, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `tb.Bigint("user_id").References(usersRef)`) {
		t.Errorf("missing user_id reference, got:\n%s", codeStr)
	}
}

func TestGenerateTimestamp(t *testing.T) {
	ts := GenerateTimestamp()

	// Should be 14 digits
	if len(ts) != 14 {
		t.Errorf("timestamp length = %d, want 14", len(ts))
	}

	// Should be all digits
	for i, c := range ts {
		if c < '0' || c > '9' {
			t.Errorf("timestamp[%d] = %c, want digit", i, c)
		}
	}
}

func TestGenerateMigration_EmptyColumns(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "users",
		Timestamp:     "20260111170656",
		Columns:       []parser.ColumnSpec{},
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	if !strings.Contains(codeStr, "func Migrate_20260111170656_users") {
		t.Error("missing function declaration")
	}

	// Check AddTable call
	if !strings.Contains(codeStr, `plan.AddTable("users"`) {
		t.Error("missing AddTable call")
	}
}

func TestGenerateMigration_SimpleColumns(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "users",
		Timestamp:     "20260111170656",
		Columns: []parser.ColumnSpec{
			{Name: "name", Type: "string"},
			{Name: "email", Type: "string"},
			{Name: "age", Type: "int"},
		},
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check column definitions
	if !strings.Contains(codeStr, `tb.String("name")`) {
		t.Errorf("missing name column, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `tb.String("email")`) {
		t.Errorf("missing email column, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `tb.Integer("age")`) {
		t.Errorf("missing age column, got:\n%s", codeStr)
	}
}

func TestGenerateMigration_WithReferences(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "posts",
		Timestamp:     "20260111170656",
		Columns: []parser.ColumnSpec{
			{Name: "title", Type: "string"},
			{Name: "user_id", Type: "references", References: "users"},
		},
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check table reference lookup
	if !strings.Contains(codeStr, `usersRef, err := plan.Table("users")`) {
		t.Errorf("missing table reference lookup, got:\n%s", codeStr)
	}

	// Check error handling for reference
	if !strings.Contains(codeStr, "if err != nil") {
		t.Error("missing error handling")
	}

	// Check reference column
	if !strings.Contains(codeStr, `tb.Bigint("user_id").References(usersRef)`) {
		t.Errorf("missing reference column, got:\n%s", codeStr)
	}
}

func TestGenerateMigration_MultipleReferencesToSameTable(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "transfers",
		Timestamp:     "20260111170656",
		Columns: []parser.ColumnSpec{
			{Name: "from_account_id", Type: "references", References: "accounts"},
			{Name: "to_account_id", Type: "references", References: "accounts"},
		},
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Should only have one table lookup for accounts
	count := strings.Count(codeStr, `accountsRef, err := plan.Table("accounts")`)
	if count != 1 {
		t.Errorf("expected 1 accounts table lookup, got %d, code:\n%s", count, codeStr)
	}

	// Both columns should reference the same ref variable
	if !strings.Contains(codeStr, `tb.Bigint("from_account_id").References(accountsRef)`) {
		t.Errorf("missing from_account_id reference, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `tb.Bigint("to_account_id").References(accountsRef)`) {
		t.Errorf("missing to_account_id reference, got:\n%s", codeStr)
	}
}

func TestGenerateMigration_MultipleReferencesToDifferentTables(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "comments",
		Timestamp:     "20260111170656",
		Columns: []parser.ColumnSpec{
			{Name: "user_id", Type: "references", References: "users"},
			{Name: "post_id", Type: "references", References: "posts"},
		},
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check both table lookups exist
	if !strings.Contains(codeStr, `usersRef, err := plan.Table("users")`) {
		t.Errorf("missing users table lookup, got:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `postsRef, err := plan.Table("posts")`) {
		t.Errorf("missing posts table lookup, got:\n%s", codeStr)
	}

	// Check both references
	if !strings.Contains(codeStr, `tb.Bigint("user_id").References(usersRef)`) {
		t.Error("missing user_id reference")
	}
	if !strings.Contains(codeStr, `tb.Bigint("post_id").References(postsRef)`) {
		t.Error("missing post_id reference")
	}
}

func TestGenerateMigration_AllColumnTypes(t *testing.T) {
	cfg := MigrationConfig{
		PackageName:   "migrations",
		MigrationName: "all_types",
		Timestamp:     "20260111170656",
		Columns: []parser.ColumnSpec{
			{Name: "col_string", Type: "string"},
			{Name: "col_text", Type: "text"},
			{Name: "col_int", Type: "int"},
			{Name: "col_bigint", Type: "bigint"},
			{Name: "col_bool", Type: "bool"},
			{Name: "col_float", Type: "float"},
			{Name: "col_decimal", Type: "decimal"},
			{Name: "col_datetime", Type: "datetime"},
			{Name: "col_timestamp", Type: "timestamp"},
			{Name: "col_binary", Type: "binary"},
			{Name: "col_json", Type: "json"},
		},
	}

	code, err := GenerateMigration(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codeStr := string(code)

	// Check all column types are mapped correctly
	expectedMappings := map[string]string{
		"col_string":    `tb.String("col_string")`,
		"col_text":      `tb.Text("col_text")`,
		"col_int":       `tb.Integer("col_int")`,
		"col_bigint":    `tb.Bigint("col_bigint")`,
		"col_bool":      `tb.Bool("col_bool")`,
		"col_float":     `tb.Float("col_float")`,
		"col_decimal":   `tb.Decimal("col_decimal")`,
		"col_datetime":  `tb.Datetime("col_datetime")`,
		"col_timestamp": `tb.Timestamp("col_timestamp")`,
		"col_binary":    `tb.Binary("col_binary")`,
		"col_json":      `tb.JSON("col_json")`,
	}

	for colName, expected := range expectedMappings {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("missing %s mapping, expected %q in:\n%s", colName, expected, codeStr)
		}
	}
}

func TestGenerateMigration_ValidGoCode(t *testing.T) {
	// This test verifies that the generated code is valid Go by checking
	// that format.Source doesn't return an error

	testCases := []MigrationConfig{
		{
			PackageName:   "migrations",
			MigrationName: "empty",
			Timestamp:     "20260111170656",
			Columns:       []parser.ColumnSpec{},
		},
		{
			PackageName:   "migrations",
			MigrationName: "simple",
			Timestamp:     "20260111170656",
			Columns: []parser.ColumnSpec{
				{Name: "name", Type: "string"},
			},
		},
		{
			PackageName:   "migrations",
			MigrationName: "with_ref",
			Timestamp:     "20260111170656",
			Columns: []parser.ColumnSpec{
				{Name: "user_id", Type: "references", References: "users"},
			},
		},
	}

	for _, cfg := range testCases {
		t.Run(cfg.MigrationName, func(t *testing.T) {
			_, err := GenerateMigration(cfg)
			if err != nil {
				t.Errorf("generated invalid Go code: %v", err)
			}
		})
	}
}

func TestGenerateMigrationFileName(t *testing.T) {
	tests := []struct {
		timestamp string
		name      string
		want      string
	}{
		{"20260111170656", "users", "20260111170656_users.go"},
		{"20260101000000", "create_posts", "20260101000000_create_posts.go"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GenerateMigrationFileName(tt.timestamp, tt.name)
			if got != tt.want {
				t.Errorf("GenerateMigrationFileName(%q, %q) = %q, want %q", tt.timestamp, tt.name, got, tt.want)
			}
		})
	}
}

func TestColumnTypeToMethod(t *testing.T) {
	tests := []struct {
		colType string
		want    string
	}{
		{"string", "String"},
		{"text", "Text"},
		{"int", "Integer"},
		{"bigint", "Bigint"},
		{"bool", "Bool"},
		{"float", "Float"},
		{"decimal", "Decimal"},
		{"datetime", "Datetime"},
		{"timestamp", "Timestamp"},
		{"binary", "Binary"},
		{"json", "JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.colType, func(t *testing.T) {
			got := columnTypeToMethod(tt.colType)
			if got != tt.want {
				t.Errorf("columnTypeToMethod(%q) = %q, want %q", tt.colType, got, tt.want)
			}
		})
	}
}

func TestCollectReferencedTables(t *testing.T) {
	tests := []struct {
		name    string
		columns []parser.ColumnSpec
		want    []string
	}{
		{
			name:    "no references",
			columns: []parser.ColumnSpec{{Name: "name", Type: "string"}},
			want:    nil,
		},
		{
			name: "one reference",
			columns: []parser.ColumnSpec{
				{Name: "user_id", Type: "references", References: "users"},
			},
			want: []string{"users"},
		},
		{
			name: "multiple references same table",
			columns: []parser.ColumnSpec{
				{Name: "from_id", Type: "references", References: "accounts"},
				{Name: "to_id", Type: "references", References: "accounts"},
			},
			want: []string{"accounts"},
		},
		{
			name: "multiple references different tables",
			columns: []parser.ColumnSpec{
				{Name: "user_id", Type: "references", References: "users"},
				{Name: "post_id", Type: "references", References: "posts"},
			},
			want: []string{"users", "posts"},
		},
		{
			name: "preserves order",
			columns: []parser.ColumnSpec{
				{Name: "c_id", Type: "references", References: "c_table"},
				{Name: "a_id", Type: "references", References: "a_table"},
				{Name: "b_id", Type: "references", References: "b_table"},
			},
			want: []string{"c_table", "a_table", "b_table"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectReferencedTables(tt.columns)
			if len(got) != len(tt.want) {
				t.Errorf("collectReferencedTables() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("collectReferencedTables()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
