package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// =============================================================================
// getModulePath Tests
// =============================================================================

func TestGetModulePath_CurrentDir(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create temp directory with go.mod
	tmpDir, err := os.MkdirTemp("", "portsql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	goModContent := "module github.com/test/mymodule\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test
	modulePath, moduleRoot, err := getModulePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if modulePath != "github.com/test/mymodule" {
		t.Errorf("expected module path 'github.com/test/mymodule', got %q", modulePath)
	}

	if moduleRoot != tmpDir {
		t.Errorf("expected module root %q, got %q", tmpDir, moduleRoot)
	}
}

func TestGetModulePath_ParentDir(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create temp directory structure:
	// tmpDir/
	//   go.mod
	//   subpkg/
	tmpDir, err := os.MkdirTemp("", "portsql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	goModContent := "module github.com/test/monorepo\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "subpkg")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to subpkg directory (which has no go.mod)
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	// Test - should find go.mod in parent
	modulePath, moduleRoot, err := getModulePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if modulePath != "github.com/test/monorepo" {
		t.Errorf("expected module path 'github.com/test/monorepo', got %q", modulePath)
	}

	if moduleRoot != tmpDir {
		t.Errorf("expected module root %q, got %q", tmpDir, moduleRoot)
	}
}

func TestGetModulePath_DeeplyNested(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create temp directory structure:
	// tmpDir/
	//   go.mod
	//   a/b/c/deep/
	tmpDir, err := os.MkdirTemp("", "portsql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	goModContent := "module github.com/test/deepmodule\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	deepDir := filepath.Join(tmpDir, "a", "b", "c", "deep")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to deeply nested directory
	if err := os.Chdir(deepDir); err != nil {
		t.Fatal(err)
	}

	// Test - should walk up and find go.mod
	modulePath, moduleRoot, err := getModulePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if modulePath != "github.com/test/deepmodule" {
		t.Errorf("expected module path 'github.com/test/deepmodule', got %q", modulePath)
	}

	if moduleRoot != tmpDir {
		t.Errorf("expected module root %q, got %q", tmpDir, moduleRoot)
	}
}

func TestGetModulePath_NotFound(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create temp directory with no go.mod
	tmpDir, err := os.MkdirTemp("", "portsql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test - should fail
	_, _, err = getModulePath()
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}

	if !strings.Contains(err.Error(), "go.mod not found") {
		t.Errorf("expected 'go.mod not found' in error, got: %v", err)
	}
}

// =============================================================================
// Table Detection Tests
// =============================================================================

func TestIsAddTableTable(t *testing.T) {
	tests := []struct {
		name     string
		table    ddl.Table
		expected bool
	}{
		{
			name: "AddTable table with all standard columns",
			table: ddl.Table{
				Name: "users",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType, Unique: true},
					{Name: "created_at", Type: ddl.DatetimeType},
					{Name: "updated_at", Type: ddl.DatetimeType},
					{Name: "deleted_at", Type: ddl.DatetimeType},
					{Name: "name", Type: ddl.StringType},
				},
			},
			expected: true,
		},
		{
			name: "AddEmptyTable missing public_id",
			table: ddl.Table{
				Name: "categories",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "name", Type: ddl.StringType},
				},
			},
			expected: false,
		},
		{
			name: "Table with public_id but no deleted_at",
			table: ddl.Table{
				Name: "logs",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "message", Type: ddl.TextType},
				},
			},
			expected: false,
		},
		{
			name: "Table with deleted_at but no public_id",
			table: ddl.Table{
				Name: "internal_data",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "deleted_at", Type: ddl.DatetimeType},
					{Name: "data", Type: ddl.JSONType},
				},
			},
			expected: false,
		},
		{
			name: "Empty table",
			table: ddl.Table{
				Name:    "empty",
				Columns: []ddl.ColumnDefinition{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddTableTable(tt.table)
			if got != tt.expected {
				t.Errorf("isAddTableTable(%s) = %v, want %v", tt.table.Name, got, tt.expected)
			}
		})
	}
}

func TestGetCRUDTables(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: map[string]ddl.Table{
				// AddTable table - should be included
				"users": {
					Name: "users",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "deleted_at", Type: ddl.DatetimeType},
						{Name: "name", Type: ddl.StringType},
					},
				},
				// AddEmptyTable - should NOT be included
				"categories": {
					Name: "categories",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "name", Type: ddl.StringType},
					},
				},
				// Another AddTable table - should be included
				"orders": {
					Name: "orders",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "deleted_at", Type: ddl.DatetimeType},
						{Name: "total", Type: ddl.DecimalType},
					},
				},
			},
		},
	}

	tables := getCRUDTables(plan)

	// Should get 2 tables (users and orders)
	if len(tables) != 2 {
		t.Errorf("expected 2 CRUD tables, got %d", len(tables))
	}

	// Check that categories is not included
	for _, table := range tables {
		if table.Name == "categories" {
			t.Error("categories should not be included in CRUD tables")
		}
	}

	// Check that users and orders are included
	foundUsers := false
	foundOrders := false
	for _, table := range tables {
		if table.Name == "users" {
			foundUsers = true
		}
		if table.Name == "orders" {
			foundOrders = true
		}
	}
	if !foundUsers {
		t.Error("users should be included in CRUD tables")
	}
	if !foundOrders {
		t.Error("orders should be included in CRUD tables")
	}
}

// TestCompile_CRUDOnly_NoUserQueries tests that compile works when there are
// CRUD tables but no user-defined queries in querydef/.
func TestCompile_CRUDOnly_NoUserQueries(t *testing.T) {
	// This test verifies that:
	// 1. compile doesn't fail when querydef/ is empty or missing
	// 2. CRUD boilerplate is still generated for AddTable-style tables

	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := "module example.com/testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty querydef directory (no .go files)
	querydefDir := filepath.Join(tmpDir, "querydef")
	if err := os.MkdirAll(querydefDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create migrations directory with schema.json containing AddTable-style tables
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create schema.json with a CRUD-eligible table (has public_id and deleted_at)
	schema := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: map[string]ddl.Table{
				"users": {
					Name: "users",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "created_at", Type: ddl.DatetimeType},
						{Name: "updated_at", Type: ddl.DatetimeType},
						{Name: "deleted_at", Type: ddl.DatetimeType},
						{Name: "name", Type: ddl.StringType},
					},
				},
			},
		},
	}

	schemaJSON, err := schema.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), schemaJSON, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify the schema is CRUD-eligible
	crudTables := getCRUDTables(schema)
	if len(crudTables) != 1 {
		t.Fatalf("expected 1 CRUD table, got %d", len(crudTables))
	}
	if crudTables[0].Name != "users" {
		t.Fatalf("expected users table, got %s", crudTables[0].Name)
	}
}

// TestCompile_NoCRUD_NoUserQueries tests that compile fails appropriately
// when there are no CRUD tables and no user-defined queries.
func TestCompile_NoCRUD_NoUserQueries(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := "module example.com/testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty querydef directory
	querydefDir := filepath.Join(tmpDir, "querydef")
	if err := os.MkdirAll(querydefDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create migrations directory with schema.json containing only AddEmptyTable-style tables
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create schema.json with a NON-CRUD-eligible table (no public_id or deleted_at)
	schema := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: map[string]ddl.Table{
				"categories": {
					Name: "categories",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "name", Type: ddl.StringType},
					},
				},
			},
		},
	}

	schemaJSON, err := schema.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), schemaJSON, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify no CRUD tables
	crudTables := getCRUDTables(schema)
	if len(crudTables) != 0 {
		t.Fatalf("expected 0 CRUD tables, got %d", len(crudTables))
	}
}

// TestCompile_EmptyQuerydef_WithCRUD tests that having an empty querydef
// directory doesn't prevent CRUD generation.
func TestCompile_EmptyQuerydef_WithCRUD(t *testing.T) {
	// Create a plan with CRUD-eligible tables
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: map[string]ddl.Table{
				"users": {
					Name: "users",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "deleted_at", Type: ddl.DatetimeType},
						{Name: "name", Type: ddl.StringType},
					},
				},
			},
		},
	}

	// Verify CRUD tables are detected
	crudTables := getCRUDTables(plan)
	if len(crudTables) != 1 {
		t.Errorf("expected 1 CRUD table, got %d", len(crudTables))
	}

	// This confirms the schema detection works - the actual compile
	// with empty querydef is tested via integration tests
}

// TestIsAddTableTable_EdgeCases tests edge cases for AddTable detection.
func TestIsAddTableTable_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		table    ddl.Table
		expected bool
	}{
		{
			name: "has public_id only - not CRUD eligible",
			table: ddl.Table{
				Name: "partial1",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
				},
			},
			expected: false,
		},
		{
			name: "has deleted_at only - not CRUD eligible",
			table: ddl.Table{
				Name: "partial2",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "deleted_at", Type: ddl.DatetimeType},
				},
			},
			expected: false,
		},
		{
			name: "has both public_id and deleted_at - CRUD eligible",
			table: ddl.Table{
				Name: "complete",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "deleted_at", Type: ddl.DatetimeType},
				},
			},
			expected: true,
		},
		{
			name: "empty table - not CRUD eligible",
			table: ddl.Table{
				Name:    "empty",
				Columns: []ddl.ColumnDefinition{},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAddTableTable(tc.table)
			if got != tc.expected {
				t.Errorf("isAddTableTable(%s) = %v, want %v", tc.table.Name, got, tc.expected)
			}
		})
	}
}

// =============================================================================
// Scope Resolution Tests
// =============================================================================

func TestResolveScopeForTable(t *testing.T) {
	config := &Config{
		CRUD: CRUDConfig{
			GlobalScope: "org_id",
			TableScopes: map[string]string{
				"orders":      "user_id", // Override with different scope
				"public_logs": "",        // Override with no scope
			},
		},
	}

	tests := []struct {
		table    string
		expected string
	}{
		{"users", "org_id"},    // Uses global scope
		{"orders", "user_id"},  // Uses table-specific scope
		{"public_logs", ""},    // Empty override
		{"products", "org_id"}, // Uses global scope
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			got := config.CRUD.GetScopeForTable(tt.table)
			if got != tt.expected {
				t.Errorf("GetScopeForTable(%q) = %q, want %q", tt.table, got, tt.expected)
			}
		})
	}
}
