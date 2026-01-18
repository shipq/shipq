package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// Helper to wrap a table in a migration plan for testing
func tableToMigrationPlan(table ddl.Table) *migrate.MigrationPlan {
	return &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				table.Name: table,
			},
		},
	}
}

func TestGenerateSharedTypes_CRUD_WithPublicID(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// GetAuthorParams should use PublicID
	if !strings.Contains(codeStr, "type GetAuthorParams struct") {
		t.Error("generated code should contain GetAuthorParams struct")
	}
	if !strings.Contains(codeStr, "PublicID string") {
		t.Error("generated code should contain 'PublicID string'")
	}

	// InsertAuthorParams should NOT include public_id, created_at, updated_at
	if !strings.Contains(codeStr, "type InsertAuthorParams struct") {
		t.Error("generated code should contain InsertAuthorParams struct")
	}

	// HardDelete should be generated for tables with deleted_at
	if !strings.Contains(codeStr, "type HardDeleteAuthorParams struct") {
		t.Error("generated code should contain HardDeleteAuthorParams for tables with deleted_at")
	}
}

func TestGenerateSharedTypes_CRUD_WithoutPublicID(t *testing.T) {
	table := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.TextType},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// GetSettingParams should use ID (int64) when no public_id
	if !strings.Contains(codeStr, "type GetSettingParams struct") {
		t.Error("generated code should contain GetSettingParams struct")
	}
	if !strings.Contains(codeStr, "ID int64") {
		t.Error("generated code should contain 'ID int64' when no public_id")
	}

	// HardDelete should NOT be generated (no deleted_at)
	if strings.Contains(codeStr, "HardDeleteSettingParams") {
		t.Error("HardDeleteSettingParams should not be generated when no deleted_at column")
	}
}

func TestGenerateSharedTypes_CRUD_Compiles(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "bio", Type: ddl.TextType, Nullable: true},
			{Name: "score", Type: ddl.IntegerType, Nullable: true},
			{Name: "rating", Type: ddl.FloatType},
			{Name: "active", Type: ddl.BooleanType},
			{Name: "metadata", Type: ddl.JSONType, Nullable: true},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	// Use go/parser to verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "crud.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestGenerateSharedTypes_CRUD_InsertParamsExcludesAutoFilled(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Find InsertAuthorParams section
	insertIdx := strings.Index(codeStr, "type InsertAuthorParams struct")
	if insertIdx == -1 {
		t.Fatal("InsertAuthorParams not found")
	}

	// Find the closing brace of InsertAuthorParams
	endIdx := strings.Index(codeStr[insertIdx:], "}\n")
	if endIdx == -1 {
		t.Fatal("InsertAuthorParams closing brace not found")
	}
	insertSection := codeStr[insertIdx : insertIdx+endIdx]

	// Should contain Name and Email
	if !strings.Contains(insertSection, "Name") {
		t.Error("InsertAuthorParams should contain Name field")
	}
	if !strings.Contains(insertSection, "Email") {
		t.Error("InsertAuthorParams should contain Email field")
	}

	// Should NOT contain auto-filled columns
	if strings.Contains(insertSection, "Id ") || strings.Contains(insertSection, "ID ") {
		t.Error("InsertAuthorParams should not contain ID field")
	}
	if strings.Contains(insertSection, "CreatedAt") {
		t.Error("InsertAuthorParams should not contain CreatedAt field")
	}
	if strings.Contains(insertSection, "UpdatedAt") {
		t.Error("InsertAuthorParams should not contain UpdatedAt field")
	}
	if strings.Contains(insertSection, "DeletedAt") {
		t.Error("InsertAuthorParams should not contain DeletedAt field")
	}
}

func TestGenerateSharedTypes_CRUD_ResultColumnsExcludeInternalID(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Find GetAuthorResult section
	resultIdx := strings.Index(codeStr, "type GetAuthorResult struct")
	if resultIdx == -1 {
		t.Fatal("GetAuthorResult not found")
	}

	endIdx := strings.Index(codeStr[resultIdx:], "}\n")
	if endIdx == -1 {
		t.Fatal("GetAuthorResult closing brace not found")
	}
	resultSection := codeStr[resultIdx : resultIdx+endIdx]

	// Should contain PublicID and Name
	if !strings.Contains(resultSection, "PublicId") {
		t.Error("GetAuthorResult should contain PublicId field")
	}
	if !strings.Contains(resultSection, "Name") {
		t.Error("GetAuthorResult should contain Name field")
	}

	// Should NOT contain internal id or deleted_at
	// Note: Check for "Id " with space to avoid matching "PublicId"
	lines := strings.Split(resultSection, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Id ") || trimmed == "Id" {
			t.Error("GetAuthorResult should not contain internal Id field")
		}
		if strings.Contains(trimmed, "DeletedAt") {
			t.Error("GetAuthorResult should not contain DeletedAt field")
		}
	}
}

func TestGenerateSharedTypes_CRUD_MultipleTables(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "name", Type: ddl.StringType},
						{Name: "created_at", Type: ddl.DatetimeType},
						{Name: "updated_at", Type: ddl.DatetimeType},
						{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
					},
				},
				"posts": {
					Name: "posts",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "title", Type: ddl.StringType},
						{Name: "author_id", Type: ddl.BigintType},
					},
				},
			},
		},
	}

	code, err := GenerateSharedTypes(nil, plan, "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Both tables should be generated
	if !strings.Contains(codeStr, "GetAuthorParams") {
		t.Error("generated code should contain GetAuthorParams")
	}
	if !strings.Contains(codeStr, "GetPostParams") {
		t.Error("generated code should contain GetPostParams")
	}

	// Authors has deleted_at, should have HardDelete
	if !strings.Contains(codeStr, "HardDeleteAuthorParams") {
		t.Error("generated code should contain HardDeleteAuthorParams")
	}

	// Posts doesn't have deleted_at, should NOT have HardDelete
	if strings.Contains(codeStr, "HardDeletePostParams") {
		t.Error("generated code should not contain HardDeletePostParams")
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "crud.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestGenerateSharedTypes_CRUD_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}

	tableOpts := map[string]CRUDOptions{
		"authors": {ScopeColumn: "organization_id"},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", tableOpts)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// GetAuthorParams should have OrganizationId
	getIdx := strings.Index(codeStr, "type GetAuthorParams struct")
	if getIdx == -1 {
		t.Fatal("GetAuthorParams not found")
	}
	endIdx := strings.Index(codeStr[getIdx:], "}\n")
	getSection := codeStr[getIdx : getIdx+endIdx]
	if !strings.Contains(getSection, "OrganizationId") {
		t.Error("GetAuthorParams should contain OrganizationId when scope is configured")
	}

	// ListAuthorsParams should have OrganizationId
	listIdx := strings.Index(codeStr, "type ListAuthorsParams struct")
	if listIdx == -1 {
		t.Fatal("ListAuthorsParams not found")
	}
	endIdx = strings.Index(codeStr[listIdx:], "}\n")
	listSection := codeStr[listIdx : listIdx+endIdx]
	if !strings.Contains(listSection, "OrganizationId") {
		t.Error("ListAuthorsParams should contain OrganizationId when scope is configured")
	}

	// InsertAuthorParams should have OrganizationId
	insertIdx := strings.Index(codeStr, "type InsertAuthorParams struct")
	if insertIdx == -1 {
		t.Fatal("InsertAuthorParams not found")
	}
	endIdx = strings.Index(codeStr[insertIdx:], "}\n")
	insertSection := codeStr[insertIdx : insertIdx+endIdx]
	if !strings.Contains(insertSection, "OrganizationId") {
		t.Error("InsertAuthorParams should contain OrganizationId when scope is configured")
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "crud.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestToSingular(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"authors", "author"},
		{"posts", "post"},
		{"categories", "category"},
		{"addresses", "address"}, // "es" suffix removed
		{"users", "user"},
		{"data", "data"}, // No change for words not ending in s/es/ies
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSingular(tt.input)
			if got != tt.want {
				t.Errorf("toSingular(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateSharedTypes_CRUD_NullableTypesInParams(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
			{Name: "bio", Type: ddl.TextType, Nullable: true},
			{Name: "age", Type: ddl.IntegerType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// InsertAuthorParams should have pointer types for nullable columns
	insertIdx := strings.Index(codeStr, "type InsertAuthorParams struct")
	if insertIdx == -1 {
		t.Fatal("InsertAuthorParams not found")
	}
	endIdx := strings.Index(codeStr[insertIdx:], "}\n")
	insertSection := codeStr[insertIdx : insertIdx+endIdx]

	// Bio should be *string (go/format may add whitespace, so just check the types are present)
	if !strings.Contains(insertSection, "Bio") || !strings.Contains(insertSection, "*string") {
		t.Errorf("InsertAuthorParams.Bio should be *string for nullable text. Section:\n%s", insertSection)
	}

	// Age should be *int32
	if !strings.Contains(insertSection, "Age") || !strings.Contains(insertSection, "*int32") {
		t.Errorf("InsertAuthorParams.Age should be *int32 for nullable integer. Section:\n%s", insertSection)
	}

	// Name should be string (verify Name exists and check the line doesn't have pointer)
	// Find the Name line specifically
	lines := strings.Split(insertSection, "\n")
	foundName := false
	for _, line := range lines {
		if strings.Contains(line, "Name") {
			foundName = true
			if strings.Contains(line, "*string") {
				t.Error("InsertAuthorParams.Name should be string, not *string")
			}
		}
	}
	if !foundName {
		t.Error("InsertAuthorParams should contain Name field")
	}
}

// =============================================================================
// Cursor-Based Pagination Tests
// =============================================================================

func TestGenerateSharedTypes_CRUD_ListCursorStruct(t *testing.T) {
	// Tables with both created_at and public_id should generate a cursor struct
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should generate ListUsersCursor struct
	if !strings.Contains(codeStr, "type ListUsersCursor struct") {
		t.Error("generated code should contain ListUsersCursor struct")
	}

	// Find the cursor struct and verify its fields
	cursorIdx := strings.Index(codeStr, "type ListUsersCursor struct")
	if cursorIdx == -1 {
		t.Fatal("ListUsersCursor not found")
	}
	endIdx := strings.Index(codeStr[cursorIdx:], "}\n")
	cursorSection := codeStr[cursorIdx : cursorIdx+endIdx]

	// Should have CreatedAt time.Time
	if !strings.Contains(cursorSection, "CreatedAt") || !strings.Contains(cursorSection, "time.Time") {
		t.Errorf("ListUsersCursor should contain CreatedAt time.Time. Section:\n%s", cursorSection)
	}

	// Should have PublicID string
	if !strings.Contains(cursorSection, "PublicID") || !strings.Contains(cursorSection, "string") {
		t.Errorf("ListUsersCursor should contain PublicID string. Section:\n%s", cursorSection)
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "crud.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestGenerateSharedTypes_CRUD_ListParamsWithCursor(t *testing.T) {
	// Tables with created_at and public_id should have cursor-based params
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Find ListUsersParams
	listIdx := strings.Index(codeStr, "type ListUsersParams struct")
	if listIdx == -1 {
		t.Fatal("ListUsersParams not found")
	}
	endIdx := strings.Index(codeStr[listIdx:], "}\n")
	listSection := codeStr[listIdx : listIdx+endIdx]

	// Should have Limit int
	if !strings.Contains(listSection, "Limit") {
		t.Errorf("ListUsersParams should contain Limit. Section:\n%s", listSection)
	}

	// Should have Cursor *ListUsersCursor (pointer for optional)
	if !strings.Contains(listSection, "Cursor") || !strings.Contains(listSection, "*ListUsersCursor") {
		t.Errorf("ListUsersParams should contain Cursor *ListUsersCursor. Section:\n%s", listSection)
	}

	// Should have CreatedAfter *time.Time
	if !strings.Contains(listSection, "CreatedAfter") || !strings.Contains(listSection, "*time.Time") {
		t.Errorf("ListUsersParams should contain CreatedAfter *time.Time. Section:\n%s", listSection)
	}

	// Should have CreatedBefore *time.Time
	if !strings.Contains(listSection, "CreatedBefore") {
		t.Errorf("ListUsersParams should contain CreatedBefore. Section:\n%s", listSection)
	}

	// Should NOT have Offset (old pagination style)
	if strings.Contains(listSection, "Offset") {
		t.Errorf("ListUsersParams should NOT contain Offset (using cursor pagination). Section:\n%s", listSection)
	}
}

func TestGenerateSharedTypes_CRUD_ListResultWithNextCursor(t *testing.T) {
	// Tables with created_at and public_id should wrap result with Items and NextCursor
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should generate ListUsersResult as wrapper struct
	if !strings.Contains(codeStr, "type ListUsersResult struct") {
		t.Error("generated code should contain ListUsersResult struct")
	}

	// Find ListUsersResult
	resultIdx := strings.Index(codeStr, "type ListUsersResult struct")
	if resultIdx == -1 {
		t.Fatal("ListUsersResult not found")
	}
	endIdx := strings.Index(codeStr[resultIdx:], "}\n")
	resultSection := codeStr[resultIdx : resultIdx+endIdx]

	// Should have Items []ListUsersItem
	if !strings.Contains(resultSection, "Items") || !strings.Contains(resultSection, "[]ListUsersItem") {
		t.Errorf("ListUsersResult should contain Items []ListUsersItem. Section:\n%s", resultSection)
	}

	// Should have NextCursor *ListUsersCursor
	if !strings.Contains(resultSection, "NextCursor") || !strings.Contains(resultSection, "*ListUsersCursor") {
		t.Errorf("ListUsersResult should contain NextCursor *ListUsersCursor. Section:\n%s", resultSection)
	}

	// Should generate ListUsersItem struct with actual data fields
	if !strings.Contains(codeStr, "type ListUsersItem struct") {
		t.Error("generated code should contain ListUsersItem struct")
	}

	// Find ListUsersItem and verify it has actual columns
	itemIdx := strings.Index(codeStr, "type ListUsersItem struct")
	if itemIdx == -1 {
		t.Fatal("ListUsersItem not found")
	}
	itemEndIdx := strings.Index(codeStr[itemIdx:], "}\n")
	itemSection := codeStr[itemIdx : itemIdx+itemEndIdx]

	// Should have PublicId and Name fields
	if !strings.Contains(itemSection, "PublicId") {
		t.Errorf("ListUsersItem should contain PublicId. Section:\n%s", itemSection)
	}
	if !strings.Contains(itemSection, "Name") {
		t.Errorf("ListUsersItem should contain Name. Section:\n%s", itemSection)
	}
	if !strings.Contains(itemSection, "CreatedAt") {
		t.Errorf("ListUsersItem should contain CreatedAt. Section:\n%s", itemSection)
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "crud.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestGenerateSharedTypes_CRUD_NoCursorWithoutCreatedAtOrPublicID(t *testing.T) {
	// Tables without created_at should fall back to offset pagination
	table := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.TextType},
		},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should NOT generate cursor struct (no created_at or public_id)
	if strings.Contains(codeStr, "ListSettingsCursor") {
		t.Error("ListSettingsCursor should not be generated for table without created_at")
	}

	// ListSettingsParams should have Offset (fallback pagination)
	listIdx := strings.Index(codeStr, "type ListSettingsParams struct")
	if listIdx == -1 {
		t.Fatal("ListSettingsParams not found")
	}
	endIdx := strings.Index(codeStr[listIdx:], "}\n")
	listSection := codeStr[listIdx : listIdx+endIdx]

	if !strings.Contains(listSection, "Offset") {
		t.Errorf("ListSettingsParams should contain Offset for tables without cursor support. Section:\n%s", listSection)
	}
}

func TestGenerateSharedTypes_CRUD_CursorWithScopeColumn(t *testing.T) {
	// Cursor-based params should include scope column when configured
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "org_id", Type: ddl.BigintType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}

	tableOpts := map[string]CRUDOptions{
		"users": {ScopeColumn: "org_id"},
	}

	code, err := GenerateSharedTypes(nil, tableToMigrationPlan(table), "crud", tableOpts)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Find ListUsersParams
	listIdx := strings.Index(codeStr, "type ListUsersParams struct")
	if listIdx == -1 {
		t.Fatal("ListUsersParams not found")
	}
	endIdx := strings.Index(codeStr[listIdx:], "}\n")
	listSection := codeStr[listIdx : listIdx+endIdx]

	// Should have OrgId (scope column)
	if !strings.Contains(listSection, "OrgId") {
		t.Errorf("ListUsersParams should contain OrgId when scope is configured. Section:\n%s", listSection)
	}

	// Should also have cursor fields
	if !strings.Contains(listSection, "Cursor") {
		t.Errorf("ListUsersParams should contain Cursor even with scope. Section:\n%s", listSection)
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "crud.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}
