package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

func TestGenerateResourceCode_WithDBIntegration(t *testing.T) {
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "email", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	opts := ResourceOptions{
		TableName:     "users",
		Prefix:        "",
		OutDir:        "api/resources",
		QueriesOut:    "queries",
		QueriesImport: "myapp/queries",
	}

	analysis := analyzeResourceTable(table)

	code, err := generateResourceCode(opts, table, analysis)
	if err != nil {
		t.Fatalf("generateResourceCode() error = %v", err)
	}

	codeStr := string(code)

	// Check for DB-backed handler elements using the global Querier pattern
	expectedElements := []string{
		"package users",
		"func GetUser(ctx context.Context, req GetUserRequest)",
		"func ListUsers(ctx context.Context, req ListUsersRequest)",
		"func CreateUser(ctx context.Context, req CreateUserRequest)",
		"func UpdateUser(ctx context.Context, req UpdateUserRequest)",
		"func DeleteUser(ctx context.Context, req DeleteUserRequest)",
		"func Register(app *portapi.App)",
		".Querier.GetUser(ctx, queries.GetUserParams",
		".Querier.InsertUser(ctx, queries.InsertUserParams",
		".Querier.UpdateUser(ctx, queries.UpdateUserParams",
		".Querier.DeleteUser(ctx, queries.DeleteUserParams",
		`"myapp/queries"`,
	}

	for _, expected := range expectedElements {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code missing %q", expected)
		}
	}

	// Should NOT have stub handler TODOs
	unexpectedElements := []string{
		"// TODO: Implement this handler",
	}

	for _, unexpected := range unexpectedElements {
		if strings.Contains(codeStr, unexpected) {
			t.Errorf("generated code should not contain %q in DB-backed mode", unexpected)
		}
	}
}

func TestGenerateResourceCode_AutoDerivesDBIntegration(t *testing.T) {
	// This test verifies that even without explicit QueriesOut or QueriesImport,
	// the generator automatically derives DB integration from the module path.
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "title", Type: ddl.StringType},
		},
	}

	// No explicit QueriesOut or QueriesImport - should still get DB integration
	// because we derive from module path
	opts := ResourceOptions{
		TableName: "posts",
		Prefix:    "",
		OutDir:    "api/resources",
	}

	analysis := analyzeResourceTable(table)

	code, err := generateResourceCode(opts, table, analysis)
	if err != nil {
		t.Fatalf("generateResourceCode() error = %v", err)
	}

	codeStr := string(code)

	// Should have DB-backed handler elements (auto-derived from module path)
	expectedElements := []string{
		"package posts",
		"func GetPost(ctx context.Context, req GetPostRequest)",
		"func ListPosts(ctx context.Context, req ListPostsRequest)",
		"func CreatePost(ctx context.Context, req CreatePostRequest)",
		"func UpdatePost(ctx context.Context, req UpdatePostRequest)",
		"func DeletePost(ctx context.Context, req DeletePostRequest)",
		"func Register(app *portapi.App)",
		".Querier.GetPost(ctx,",
		".Querier.InsertPost(ctx,",
		"/queries\"", // Should import a queries package
	}

	for _, expected := range expectedElements {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code missing %q", expected)
		}
	}

	// Should NOT have stub handler TODOs
	unexpectedElements := []string{
		"// TODO: Implement this handler",
	}

	for _, unexpected := range unexpectedElements {
		if strings.Contains(codeStr, unexpected) {
			t.Errorf("generated code should not contain %q when DB integration is auto-derived", unexpected)
		}
	}
}

func TestGenerateResourceCode_DBIntegrationMappers(t *testing.T) {
	table := ddl.Table{
		Name: "orders",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "total", Type: ddl.DecimalType},
			{Name: "status", Type: ddl.StringType},
		},
	}

	opts := ResourceOptions{
		TableName:     "orders",
		Prefix:        "/api/v1",
		OutDir:        "api/resources",
		QueriesOut:    "queries",
		QueriesImport: "myapp/queries",
	}

	analysis := analyzeResourceTable(table)

	code, err := generateResourceCode(opts, table, analysis)
	if err != nil {
		t.Fatalf("generateResourceCode() error = %v", err)
	}

	codeStr := string(code)

	// Check for mapper functions (note: GetOrderResult uses pointer type)
	expectedElements := []string{
		"func mapOrderToResponse(r *queries.GetOrderResult) OrderResponse",
		"func mapOrderItemToResponse(r queries.ListOrdersItem) OrderResponse",
		"parseCursorForOrders",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code missing %q", expected)
		}
	}

	// Check that paths include the prefix
	expectedPaths := []string{
		`app.Get("/api/v1/orders/{public_id}"`,
		`app.Get("/api/v1/orders"`,
		`app.Post("/api/v1/orders"`,
	}

	for _, expected := range expectedPaths {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code missing path %q", expected)
		}
	}
}

func TestToSingular(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"posts", "post"},
		{"categories", "category"},
		{"statuses", "status"},
		{"boxes", "box"},
		{"person", "person"},
		{"data", "data"},
		{"purchase_orders", "purchase_order"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSingular(tt.input)
			if result != tt.expected {
				t.Errorf("toSingular(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user", "User"},
		{"users", "Users"},
		{"public_id", "PublicId"},
		{"created_at", "CreatedAt"},
		{"purchase_order", "PurchaseOrder"},
		{"", "X"},
		{"123", "X123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPathWithPrefix(t *testing.T) {
	tests := []struct {
		prefix    string
		tableName string
		expected  string
	}{
		{"", "users", "/users"},
		{"/api", "users", "/api/users"},
		{"/api/v1", "users", "/api/v1/users"},
		{"/api/v1/", "users", "/api/v1/users"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+"/"+tt.tableName, func(t *testing.T) {
			result := pathWithPrefix(tt.prefix, tt.tableName)
			if result != tt.expected {
				t.Errorf("pathWithPrefix(%q, %q) = %q, want %q", tt.prefix, tt.tableName, result, tt.expected)
			}
		})
	}
}

func TestAnalyzeResourceTable(t *testing.T) {
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "email", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	analysis := analyzeResourceTable(table)

	if analysis.TableName != "users" {
		t.Errorf("TableName = %q, want %q", analysis.TableName, "users")
	}
	if analysis.SingularName != "user" {
		t.Errorf("SingularName = %q, want %q", analysis.SingularName, "user")
	}
	if analysis.PluralName != "users" {
		t.Errorf("PluralName = %q, want %q", analysis.PluralName, "users")
	}
	if analysis.SingularPascal != "User" {
		t.Errorf("SingularPascal = %q, want %q", analysis.SingularPascal, "User")
	}
	if analysis.PluralPascal != "Users" {
		t.Errorf("PluralPascal = %q, want %q", analysis.PluralPascal, "Users")
	}
	if !analysis.SupportsCursor {
		t.Error("SupportsCursor should be true for table with created_at and public_id")
	}

	// User columns should only include email and name
	if len(analysis.UserColumns) != 2 {
		t.Errorf("UserColumns length = %d, want 2", len(analysis.UserColumns))
	}

	// Result columns should include public_id, created_at, updated_at, email, name
	if len(analysis.ResultColumns) != 5 {
		t.Errorf("ResultColumns length = %d, want 5", len(analysis.ResultColumns))
	}
}

func TestAnalyzeResourceTable_NoCursor(t *testing.T) {
	// Table without created_at should not support cursor pagination
	table := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.StringType},
		},
	}

	analysis := analyzeResourceTable(table)

	if analysis.SupportsCursor {
		t.Error("SupportsCursor should be false for table without created_at")
	}
}

func TestMapColumnGoType(t *testing.T) {
	tests := []struct {
		col      ddl.ColumnDefinition
		expected string
	}{
		{ddl.ColumnDefinition{Type: ddl.IntegerType}, "int32"},
		{ddl.ColumnDefinition{Type: ddl.IntegerType, Nullable: true}, "*int32"},
		{ddl.ColumnDefinition{Type: ddl.BigintType}, "int64"},
		{ddl.ColumnDefinition{Type: ddl.BigintType, Nullable: true}, "*int64"},
		{ddl.ColumnDefinition{Type: ddl.StringType}, "string"},
		{ddl.ColumnDefinition{Type: ddl.StringType, Nullable: true}, "*string"},
		{ddl.ColumnDefinition{Type: ddl.BooleanType}, "bool"},
		{ddl.ColumnDefinition{Type: ddl.BooleanType, Nullable: true}, "*bool"},
		{ddl.ColumnDefinition{Type: ddl.FloatType}, "float64"},
		{ddl.ColumnDefinition{Type: ddl.FloatType, Nullable: true}, "*float64"},
		{ddl.ColumnDefinition{Type: ddl.TimestampType}, "time.Time"},
		{ddl.ColumnDefinition{Type: ddl.TimestampType, Nullable: true}, "*time.Time"},
		{ddl.ColumnDefinition{Type: ddl.JSONType}, "json.RawMessage"},
		{ddl.ColumnDefinition{Type: ddl.BinaryType}, "[]byte"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := mapColumnGoType(tt.col)
			if result != tt.expected {
				t.Errorf("mapColumnGoType() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGoTypeImport(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", ""},
		{"int64", ""},
		{"time.Time", "time"},
		{"*time.Time", "time"},
		{"json.RawMessage", "encoding/json"},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := goTypeImport(tt.goType)
			if result != tt.expected {
				t.Errorf("goTypeImport(%q) = %q, want %q", tt.goType, result, tt.expected)
			}
		})
	}
}

func TestGenerateResourceCode(t *testing.T) {
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "email", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	opts := ResourceOptions{
		TableName: "users",
		Prefix:    "",
		OutDir:    "api/resources",
	}

	analysis := analyzeResourceTable(table)

	code, err := generateResourceCode(opts, table, analysis)
	if err != nil {
		t.Fatalf("generateResourceCode() error = %v", err)
	}

	codeStr := string(code)

	// Check that the generated code contains expected elements
	expectedElements := []string{
		"package users",
		"type GetUserRequest struct",
		"type ListUsersRequest struct",
		"type CreateUserRequest struct",
		"type UpdateUserRequest struct",
		"type DeleteUserRequest struct",
		"type UserResponse struct",
		"type ListUsersResponse struct",
		"func GetUser(ctx context.Context, req GetUserRequest)",
		"func ListUsers(ctx context.Context, req ListUsersRequest)",
		"func CreateUser(ctx context.Context, req CreateUserRequest)",
		"func UpdateUser(ctx context.Context, req UpdateUserRequest)",
		"func DeleteUser(ctx context.Context, req DeleteUserRequest)",
		"func Register(app *portapi.App)",
		`app.Get("/users/{public_id}"`,
		`app.Get("/users"`,
		`app.Post("/users"`,
		`app.Put("/users/{public_id}"`,
		`app.Delete("/users/{public_id}"`,
	}

	for _, expected := range expectedElements {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code missing %q", expected)
		}
	}

	// Check that HardDelete is NOT in the generated code
	if strings.Contains(codeStr, "HardDelete") {
		t.Error("generated code should not contain HardDelete")
	}
}

func TestGenerateResourceCode_WithPrefix(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			{Name: "title", Type: ddl.StringType},
		},
	}

	opts := ResourceOptions{
		TableName: "posts",
		Prefix:    "/api/v1",
		OutDir:    "api/resources",
	}

	analysis := analyzeResourceTable(table)

	code, err := generateResourceCode(opts, table, analysis)
	if err != nil {
		t.Fatalf("generateResourceCode() error = %v", err)
	}

	codeStr := string(code)

	// Check that paths include the prefix
	expectedPaths := []string{
		`app.Get("/api/v1/posts/{public_id}"`,
		`app.Get("/api/v1/posts"`,
		`app.Post("/api/v1/posts"`,
		`app.Put("/api/v1/posts/{public_id}"`,
		`app.Delete("/api/v1/posts/{public_id}"`,
	}

	for _, expected := range expectedPaths {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code missing %q", expected)
		}
	}
}

func TestRunResource_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help", []string{"help"}},
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			opts := Options{
				Stdout:  &stdout,
				Stderr:  &stderr,
				Version: "test",
			}

			exitCode := runResource(tt.args, opts)

			if exitCode != ExitSuccess {
				t.Errorf("runResource() exit code = %d, want %d", exitCode, ExitSuccess)
			}

			output := stdout.String()
			if !strings.Contains(output, "shipq api resource") {
				t.Errorf("expected help to contain 'shipq api resource', got %q", output)
			}
		})
	}
}

func TestRunResource_NoTableName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{}, opts)

	if exitCode != ExitError {
		t.Errorf("runResource() exit code = %d, want %d", exitCode, ExitError)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "resource requires a table name") {
		t.Errorf("expected error about table name, got %q", errOutput)
	}
}

func TestRunResource_TableNotFound(t *testing.T) {
	// Create a temporary directory with a schema.json that doesn't have our table
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to the temp directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"users"}, opts)

	if exitCode != ExitError {
		t.Errorf("runResource() exit code = %d, want %d", exitCode, ExitError)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "not found") {
		t.Errorf("expected error about table not found, got %q", errOutput)
	}
}

func TestRunResource_NotEligible(t *testing.T) {
	// Create a temporary directory with a schema.json that has a table without public_id
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Table without public_id and deleted_at (AddEmptyTable style)
	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {
				"settings": {
					"name": "settings",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "key", "type": "string"},
						{"name": "value", "type": "string"}
					]
				}
			}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to the temp directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"settings"}, opts)

	if exitCode != ExitError {
		t.Errorf("runResource() exit code = %d, want %d", exitCode, ExitError)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "not eligible") {
		t.Errorf("expected error about not eligible, got %q", errOutput)
	}
	if !strings.Contains(errOutput, "plan.AddTable()") {
		t.Errorf("expected hint about AddTable, got %q", errOutput)
	}
}

func TestRunResource_Success(t *testing.T) {
	// Create a temporary directory with a valid schema.json
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Table with public_id and deleted_at (AddTable style)
	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {
				"users": {
					"name": "users",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "public_id", "type": "string"},
						{"name": "created_at", "type": "timestamp"},
						{"name": "updated_at", "type": "timestamp"},
						{"name": "deleted_at", "type": "timestamp", "nullable": true},
						{"name": "email", "type": "string"},
						{"name": "name", "type": "string"}
					]
				}
			}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to the temp directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"users", "--no-runtime"}, opts)

	if exitCode != ExitSuccess {
		t.Errorf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Generated:") {
		t.Errorf("expected output to contain 'Generated:', got %q", output)
	}

	// Check that the file was created
	handlersPath := filepath.Join(tmpDir, "api", "resources", "users", "handlers.go")
	if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
		t.Errorf("expected handlers.go to be created at %s", handlersPath)
	}

	// Check the generated file content
	content, err := os.ReadFile(handlersPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	contentStr := string(content)
	expectedElements := []string{
		"package users",
		"type GetUserRequest struct",
		"func Register(app *portapi.App)",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("generated file missing %q", expected)
		}
	}
}

func TestColumnToResourceColumn(t *testing.T) {
	col := ddl.ColumnDefinition{
		Name:     "email_address",
		Type:     ddl.StringType,
		Nullable: false,
	}

	rc := columnToResourceColumn(col)

	if rc.Name != "email_address" {
		t.Errorf("Name = %q, want %q", rc.Name, "email_address")
	}
	if rc.FieldName != "EmailAddress" {
		t.Errorf("FieldName = %q, want %q", rc.FieldName, "EmailAddress")
	}
	if rc.GoType != "string" {
		t.Errorf("GoType = %q, want %q", rc.GoType, "string")
	}
	if rc.JSONTag != "email_address" {
		t.Errorf("JSONTag = %q, want %q", rc.JSONTag, "email_address")
	}
	if rc.IsNullable != false {
		t.Errorf("IsNullable = %v, want %v", rc.IsNullable, false)
	}
}
