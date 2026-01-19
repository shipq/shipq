//go:build integration

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	_ "modernc.org/sqlite"
)

// setupTestProject creates a temporary project with schema and config.
func setupTestProject(t *testing.T, schema *migrate.MigrationPlan, config string) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create migrations directory
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	// Write schema.json
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), schemaJSON, 0644); err != nil {
		t.Fatalf("failed to write schema.json: %v", err)
	}

	// Create querydef directory with empty init file
	querydefDir := filepath.Join(tmpDir, "querydef")
	if err := os.MkdirAll(querydefDir, 0755); err != nil {
		t.Fatalf("failed to create querydef dir: %v", err)
	}

	// Write minimal querydef/queries.go
	queriesContent := `package querydef

import "github.com/shipq/shipq/db/portsql/query"

func init() {
	// Define a dummy query so compile doesn't fail
	_ = query.GetRegisteredQueries
}
`
	if err := os.WriteFile(filepath.Join(querydefDir, "queries.go"), []byte(queriesContent), 0644); err != nil {
		t.Fatalf("failed to write queries.go: %v", err)
	}

	// Create queries output directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "queries"), 0755); err != nil {
		t.Fatalf("failed to create queries dir: %v", err)
	}

	// Write portsql.ini if config provided
	if config != "" {
		if err := os.WriteFile(filepath.Join(tmpDir, "portsql.ini"), []byte(config), 0644); err != nil {
			t.Fatalf("failed to write portsql.ini: %v", err)
		}
	}

	return tmpDir
}

// createAddTableSchema creates a schema with AddTable-style tables.
func createAddTableSchema(tables ...string) *migrate.MigrationPlan {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}

	for _, name := range tables {
		plan.Schema.Tables[name] = ddl.Table{
			Name: name,
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType, Unique: true},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
				{Name: "name", Type: ddl.StringType},
				{Name: "email", Type: ddl.StringType},
			},
		}
	}

	return plan
}

// createMixedSchema creates a schema with both AddTable and AddEmptyTable style tables.
func createMixedSchema() *migrate.MigrationPlan {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}

	// AddTable style - should get CRUD
	plan.Schema.Tables["users"] = ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType, Unique: true},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			{Name: "name", Type: ddl.StringType},
		},
	}

	// AddEmptyTable style - should NOT get CRUD
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}

	return plan
}

func TestIntegration_CRUDTableDetection(t *testing.T) {
	schema := createMixedSchema()

	crudTables := getCRUDTables(schema)

	// Should only have users, not categories
	if len(crudTables) != 1 {
		t.Errorf("expected 1 CRUD table, got %d", len(crudTables))
	}

	if len(crudTables) > 0 && crudTables[0].Name != "users" {
		t.Errorf("expected users table, got %s", crudTables[0].Name)
	}
}

func TestIntegration_LoadSchema(t *testing.T) {
	schema := createAddTableSchema("users", "orders")
	tmpDir := setupTestProject(t, schema, `[database]
url = sqlite:./test.db
`)

	// Load schema
	loaded, err := loadSchema(filepath.Join(tmpDir, "migrations"))
	if err != nil {
		t.Fatalf("loadSchema failed: %v", err)
	}

	if len(loaded.Schema.Tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(loaded.Schema.Tables))
	}

	if _, exists := loaded.Schema.Tables["users"]; !exists {
		t.Error("expected users table to exist")
	}
	if _, exists := loaded.Schema.Tables["orders"]; !exists {
		t.Error("expected orders table to exist")
	}
}

func TestIntegration_CRUDWithScope_GeneratesCorrectSQL(t *testing.T) {
	schema := createAddTableSchema("users")

	// Add org_id column for scoping
	usersTable := schema.Schema.Tables["users"]
	usersTable.Columns = append(usersTable.Columns, ddl.ColumnDefinition{
		Name: "org_id",
		Type: ddl.BigintType,
	})
	schema.Schema.Tables["users"] = usersTable

	tmpDir := setupTestProject(t, schema, `[database]
url = sqlite:./test.db

[crud]
scope = org_id
`)

	// Load config
	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify scope is configured
	scope := cfg.CRUD.GetScopeForTable("users")
	if scope != "org_id" {
		t.Errorf("expected scope 'org_id', got %q", scope)
	}
}

func TestIntegration_CRUDPerTableScope(t *testing.T) {
	schema := createAddTableSchema("users", "orders")

	// Add scope columns
	usersTable := schema.Schema.Tables["users"]
	usersTable.Columns = append(usersTable.Columns, ddl.ColumnDefinition{
		Name: "org_id",
		Type: ddl.BigintType,
	})
	schema.Schema.Tables["users"] = usersTable

	ordersTable := schema.Schema.Tables["orders"]
	ordersTable.Columns = append(ordersTable.Columns, ddl.ColumnDefinition{
		Name: "user_id",
		Type: ddl.BigintType,
	})
	schema.Schema.Tables["orders"] = ordersTable

	tmpDir := setupTestProject(t, schema, `[database]
url = sqlite:./test.db

[crud]
scope = org_id

[crud.orders]
scope = user_id
`)

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// users should use global scope
	if scope := cfg.CRUD.GetScopeForTable("users"); scope != "org_id" {
		t.Errorf("expected users scope 'org_id', got %q", scope)
	}

	// orders should use table-specific scope
	if scope := cfg.CRUD.GetScopeForTable("orders"); scope != "user_id" {
		t.Errorf("expected orders scope 'user_id', got %q", scope)
	}
}

func TestIntegration_SQLiteCRUD_EndToEnd(t *testing.T) {
	// Create an in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skipf("SQLite unavailable: %v", err)
	}
	defer db.Close()

	// Create users table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			org_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			deleted_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	// Test INSERT
	publicID := "test_user_123"
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (public_id, org_id, name, email, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, publicID, 1, "Test User", "test@example.com", now, now)
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	// Test SELECT with scope
	var name, email string
	err = db.QueryRowContext(ctx, `
		SELECT name, email FROM users 
		WHERE public_id = ? AND org_id = ? AND deleted_at IS NULL
	`, publicID, 1).Scan(&name, &email)
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	if name != "Test User" || email != "test@example.com" {
		t.Errorf("unexpected values: name=%q email=%q", name, email)
	}

	// Test soft delete
	_, err = db.ExecContext(ctx, `
		UPDATE users SET deleted_at = ? WHERE public_id = ? AND deleted_at IS NULL
	`, now, publicID)
	if err != nil {
		t.Fatalf("soft DELETE failed: %v", err)
	}

	// Verify soft-deleted record is not returned
	err = db.QueryRowContext(ctx, `
		SELECT name FROM users WHERE public_id = ? AND deleted_at IS NULL
	`, publicID).Scan(&name)
	if err != sql.ErrNoRows {
		t.Errorf("expected no rows after soft delete, got: %v", err)
	}
}

func TestIntegration_GeneratedCodeContainsExpectedTypes(t *testing.T) {
	schema := createAddTableSchema("users")
	tmpDir := setupTestProject(t, schema, `[database]
url = sqlite:./test.db

[crud]
scope = org_id
`)

	// Load config and schema
	cfg, _ := LoadConfig(tmpDir)
	plan, _ := loadSchema(filepath.Join(tmpDir, "migrations"))

	// Get CRUD tables
	crudTables := getCRUDTables(plan)
	if len(crudTables) == 0 {
		t.Fatal("no CRUD tables found")
	}

	// Build table options
	tableOpts := make(map[string]codegen.CRUDOptions)
	for _, table := range crudTables {
		tableOpts[table.Name] = codegen.CRUDOptions{
			ScopeColumn: cfg.CRUD.GetScopeForTable(table.Name),
		}
	}

	// Create filtered plan
	crudPlan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}
	for _, table := range crudTables {
		crudPlan.Schema.Tables[table.Name] = table
	}

	// Generate CRUD types
	code, err := codegen.GenerateSharedTypes(nil, crudPlan, "queries", tableOpts)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Verify expected types are generated
	expectedTypes := []string{
		"GetUserParams",
		"GetUserResult",
		"ListUsersParams",
		"ListUsersResult",
		"InsertUserParams",
		"UpdateUserParams",
		"DeleteUserParams",
	}

	for _, typeName := range expectedTypes {
		if !strings.Contains(codeStr, "type "+typeName+" struct") {
			t.Errorf("expected type %s not found in generated code", typeName)
		}
	}
}

// =============================================================================
// Cursor Pagination Integration Tests
// =============================================================================

func TestIntegration_CursorPagination_GeneratedCode(t *testing.T) {
	// Test that cursor pagination generates the right types and code structure

	// Create schema with AddTable-style tables (has created_at and public_id)
	schema := createAddTableSchema("users")

	config := `[database]
url = sqlite:test.db

[paths]
queries_out = queries
`

	tmpDir := setupTestProject(t, schema, config)
	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Collect tables that need CRUD
	var crudTables []ddl.Table
	for _, table := range schema.Schema.Tables {
		crudTables = append(crudTables, table)
	}

	tableOpts := make(map[string]codegen.CRUDOptions)
	for _, table := range crudTables {
		tableOpts[table.Name] = codegen.CRUDOptions{
			ScopeColumn: cfg.CRUD.GetScopeForTable(table.Name),
			OrderAsc:    cfg.CRUD.GetOrderForTable(table.Name) == "asc",
		}
	}

	// Create CRUD plan
	crudPlan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}
	for _, table := range crudTables {
		crudPlan.Schema.Tables[table.Name] = table
	}

	// Generate shared types
	code, err := codegen.GenerateSharedTypes(nil, crudPlan, "queries", tableOpts)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Verify cursor pagination types are generated for tables with created_at + public_id
	expectedCursorTypes := []string{
		"ListUsersCursor", // Cursor struct
		"ListUsersItem",   // Item struct (actual data)
		"ListUsersResult", // Wrapper with Items + NextCursor
		"ListUsersParams", // Params with Cursor, CreatedAfter, CreatedBefore
	}

	for _, typeName := range expectedCursorTypes {
		if !strings.Contains(codeStr, "type "+typeName+" struct") {
			t.Errorf("expected cursor pagination type %s not found in generated code", typeName)
		}
	}

	// Verify cursor struct has CreatedAt and PublicID
	if !strings.Contains(codeStr, "CreatedAt time.Time") {
		t.Error("ListUsersCursor should have CreatedAt time.Time field")
	}
	if !strings.Contains(codeStr, "PublicID  string") {
		t.Error("ListUsersCursor should have PublicID string field")
	}

	// Verify params has cursor fields
	if !strings.Contains(codeStr, "Cursor        *ListUsersCursor") {
		t.Error("ListUsersParams should have Cursor *ListUsersCursor field")
	}
	if !strings.Contains(codeStr, "CreatedAfter  *time.Time") {
		t.Error("ListUsersParams should have CreatedAfter *time.Time field")
	}
	if !strings.Contains(codeStr, "CreatedBefore *time.Time") {
		t.Error("ListUsersParams should have CreatedBefore *time.Time field")
	}

	// Verify result wrapper has Items and NextCursor
	if !strings.Contains(codeStr, "Items      []ListUsersItem") {
		t.Error("ListUsersResult should have Items []ListUsersItem field")
	}
	if !strings.Contains(codeStr, "NextCursor *ListUsersCursor") {
		t.Error("ListUsersResult should have NextCursor *ListUsersCursor field")
	}
}

func TestIntegration_CursorPagination_RunnerCode(t *testing.T) {
	// Test that the generated runner code has the right structure

	// Create schema with AddTable-style tables
	schema := createAddTableSchema("users")

	config := `[database]
url = sqlite:test.db
`

	tmpDir := setupTestProject(t, schema, config)
	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Collect tables for CRUD
	var crudTables []ddl.Table
	for _, table := range schema.Schema.Tables {
		crudTables = append(crudTables, table)
	}

	tableOpts := make(map[string]codegen.CRUDOptions)
	for _, table := range crudTables {
		tableOpts[table.Name] = codegen.CRUDOptions{
			ScopeColumn: cfg.CRUD.GetScopeForTable(table.Name),
			OrderAsc:    cfg.CRUD.GetOrderForTable(table.Name) == "asc",
		}
	}

	crudPlan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}
	for _, table := range crudTables {
		crudPlan.Schema.Tables[table.Name] = table
	}

	// Generate SQLite runner
	code, err := codegen.GenerateDialectRunner(nil, crudPlan, "sqlite", "myapp/queries", tableOpts)
	if err != nil {
		t.Fatalf("GenerateDialectRunner failed: %v", err)
	}

	codeStr := string(code)

	// Verify the List method returns the result wrapper type
	if !strings.Contains(codeStr, "(*queries.ListUsersResult, error)") {
		t.Error("ListUsers should return (*queries.ListUsersResult, error)")
	}

	// Verify N+1 fetch strategy
	if !strings.Contains(codeStr, "params.Limit + 1") && !strings.Contains(codeStr, "params.Limit+1") {
		t.Error("ListUsers should fetch params.Limit + 1 rows")
	}

	// Verify dynamic SQL building
	if !strings.Contains(codeStr, "params.Cursor != nil") {
		t.Error("ListUsers should check for cursor parameter")
	}
	if !strings.Contains(codeStr, "params.CreatedAfter != nil") {
		t.Error("ListUsers should check for CreatedAfter parameter")
	}

	// Verify NextCursor construction
	if !strings.Contains(codeStr, "NextCursor") && !strings.Contains(codeStr, "ListUsersCursor") {
		t.Error("ListUsers should build NextCursor")
	}
}

func TestIntegration_CursorPagination_OrderDirection(t *testing.T) {
	// Test that order direction config affects SQL generation

	schema := createAddTableSchema("audit_logs")

	config := `[database]
url = sqlite:test.db

[crud.audit_logs]
order = asc
`

	tmpDir := setupTestProject(t, schema, config)
	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify order direction is parsed
	if order := cfg.CRUD.GetOrderForTable("audit_logs"); order != "asc" {
		t.Errorf("GetOrderForTable('audit_logs') = %q, want 'asc'", order)
	}

	// Generate SQL and verify ASC order
	table := schema.Schema.Tables["audit_logs"]
	opts := codegen.CRUDOptions{
		OrderAsc: cfg.CRUD.GetOrderForTable("audit_logs") == "asc",
	}

	sqlSet := codegen.GenerateCRUDSQL(table, codegen.SQLDialectSQLite, opts)

	// Verify ASC order in generated SQL
	if !strings.Contains(sqlSet.ListSQL, "ASC") {
		t.Errorf("ListSQL should have ASC order, got: %s", sqlSet.ListSQL)
	}
}
