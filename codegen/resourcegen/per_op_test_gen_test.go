package resourcegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// ─── List test gen tests (Step 7i) ───

func TestGenerateListTest_UsesLimitAndCursor(t *testing.T) {
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     false,
		Dialect:         "sqlite",
		TestDatabaseURL: "file::memory:?cache=shared",
	}

	result, err := GenerateListTest(cfg)
	if err != nil {
		t.Fatalf("GenerateListTest failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Assert that the generated test code exercises query params via Limit: 2
	if !strings.Contains(code, "Limit: 2") {
		t.Error("expected Limit: 2 in generated list test to exercise query param pagination")
	}

	// Assert that the generated test code passes the cursor for page 2
	if !strings.Contains(code, "Cursor: page1.NextCursor") {
		t.Error("expected Cursor: page1.NextCursor in generated list test to exercise cursor query param")
	}
}

func TestGenerateListTest_PaginationIsFunctional(t *testing.T) {
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "items",
		Table: ddl.Table{
			Name: "items",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     false,
		Dialect:         "sqlite",
		TestDatabaseURL: "file::memory:?cache=shared",
	}

	result, err := GenerateListTest(cfg)
	if err != nil {
		t.Fatalf("GenerateListTest failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Assert that the generated test checks page 2 has at least 1 item
	if !strings.Contains(code, "len(page2.Items) < 1") {
		t.Error("expected page 2 check for at least 1 item in generated pagination test")
	}

	// Assert that the generated test checks for no overlap between pages
	if !strings.Contains(code, "page1IDs") {
		t.Error("expected page overlap check (page1IDs) in generated pagination test")
	}
	if !strings.Contains(code, "appears on both page 1 and page 2") {
		t.Error("expected overlap error message in generated pagination test")
	}
}

func TestGenerateCreateTest_JSONColumn_ImportsEncodingJSON(t *testing.T) {
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "events",
		Table: ddl.Table{
			Name: "events",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "metadata", Type: ddl.JSONType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     false,
		Dialect:         "sqlite",
		TestDatabaseURL: "file::memory:?cache=shared",
	}

	result, err := GenerateCreateTest(cfg)
	if err != nil {
		t.Fatalf("GenerateCreateTest failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Must import encoding/json
	if !strings.Contains(code, `"encoding/json"`) {
		t.Error("expected \"encoding/json\" import for JSON column in create test")
	}

	// Must use json.RawMessage sample value
	if !strings.Contains(code, `json.RawMessage`) {
		t.Error("expected json.RawMessage usage for JSON column in create test")
	}
}

func TestGenerateCreateTest_NoJSONColumn_OmitsEncodingJSON(t *testing.T) {
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "pets",
		Table: ddl.Table{
			Name: "pets",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "species", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     false,
		Dialect:         "sqlite",
		TestDatabaseURL: "file::memory:?cache=shared",
	}

	result, err := GenerateCreateTest(cfg)
	if err != nil {
		t.Fatalf("GenerateCreateTest failed: %v", err)
	}

	code := string(result)

	if strings.Contains(code, `"encoding/json"`) {
		t.Error("should NOT import \"encoding/json\" when there are no JSON columns")
	}
}

func TestGenerateUpdateTest_JSONColumn_ImportsEncodingJSON(t *testing.T) {
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "events",
		Table: ddl.Table{
			Name: "events",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "metadata", Type: ddl.JSONType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     false,
		Dialect:         "sqlite",
		TestDatabaseURL: "file::memory:?cache=shared",
	}

	result, err := GenerateUpdateTest(cfg)
	if err != nil {
		t.Fatalf("GenerateUpdateTest failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Must import encoding/json
	if !strings.Contains(code, `"encoding/json"`) {
		t.Error("expected \"encoding/json\" import for JSON column in update test")
	}

	// Must use json.RawMessage sample value
	if !strings.Contains(code, `json.RawMessage`) {
		t.Error("expected json.RawMessage usage for JSON column in update test")
	}
}

func TestGenerateUpdateTest_NoCreateResultFieldReference(t *testing.T) {
	// Regression test for Bug 3: the update test must NOT reference
	// created.<Field> for non-ID fields, because CreateResult only
	// contains Id and PublicId. Instead it should use sample-value
	// literals (keep<Field> variables).
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "channels",
		Table: ddl.Table{
			Name: "channels",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "description", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     true,
		Dialect:         "postgres",
		TestDatabaseURL: "postgres://localhost:5432/myapp_test?sslmode=disable",
	}

	result, err := GenerateUpdateTest(cfg)
	if err != nil {
		t.Fatalf("GenerateUpdateTest failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// The output must NOT contain &created.Description or &created.Name
	// (CreateResult only has Id/PublicId)
	if strings.Contains(code, "&created.Description") {
		t.Error("output should NOT contain &created.Description — CreateResult only has Id/PublicId")
	}
	if strings.Contains(code, "&created.Name") {
		t.Error("output should NOT contain &created.Name — CreateResult only has Id/PublicId")
	}

	// It SHOULD contain a keep variable for the non-target field.
	// "name" is the first string column so it becomes updateField;
	// "description" is the second string column and should get a keepDescription variable.
	if !strings.Contains(code, "keepDescription") {
		t.Error("expected keepDescription sample-value variable for carried-over field")
	}
	if !strings.Contains(code, `"test_description"`) {
		t.Error("expected sample value \"test_description\" for carried-over description field")
	}

	// Verify the target field uses updatedVal
	if !strings.Contains(code, "updatedVal") {
		t.Error("expected updatedVal for the update target field")
	}
}

func TestGenerateUpdateTest_WithFK_NoCreateResultReference(t *testing.T) {
	// Ensure FK columns use fixture-created dependencies, not created.<Field>
	cfg := PerOpTestGenConfig{
		ModulePath: "myapp",
		TableName:  "books",
		Table: ddl.Table{
			Name: "books",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "summary", Type: ddl.StringType},
				{Name: "author_id", Type: ddl.BigintType, References: "authors"},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     true,
		Dialect:         "postgres",
		TestDatabaseURL: "postgres://localhost:5432/myapp_test?sslmode=disable",
	}

	result, err := GenerateUpdateTest(cfg)
	if err != nil {
		t.Fatalf("GenerateUpdateTest failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// FK column should reference fixture, not created.<Field>
	if !strings.Contains(code, "authorForUpdate") {
		t.Error("expected authorForUpdate fixture dependency for FK column")
	}

	// Non-target, non-FK column should use keep variable
	if !strings.Contains(code, "keepSummary") {
		t.Error("expected keepSummary sample-value variable")
	}

	// Must not reference created.Summary or created.AuthorId
	if strings.Contains(code, "&created.Summary") {
		t.Error("should NOT reference &created.Summary")
	}
	if strings.Contains(code, "&created.AuthorId") {
		t.Error("should NOT reference &created.AuthorId")
	}
}

func TestGenerateTestHelpers_UniqueEmail(t *testing.T) {
	// Regression test for Bug 4: generated test helpers must NOT use
	// a hardcoded email, to avoid cross-package lock contention on the
	// accounts unique email constraint.
	cfg := PerOpTestGenConfig{
		ModulePath:      "myapp",
		TableName:       "posts",
		Table:           ddl.Table{Name: "posts"},
		Schema:          map[string]ddl.Table{},
		RequireAuth:     true,
		Dialect:         "postgres",
		TestDatabaseURL: "postgres://localhost:5432/myapp_test?sslmode=disable",
	}

	result, err := GenerateTestHelpers(cfg)
	if err != nil {
		t.Fatalf("GenerateTestHelpers failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Must NOT contain the hardcoded email
	if strings.Contains(code, `"test@example.com"`) {
		t.Error("output should NOT contain hardcoded \"test@example.com\"")
	}

	// Must use nanoid for unique email generation
	if !strings.Contains(code, "nanoid.New()") {
		t.Error("expected nanoid.New() usage for unique email generation")
	}

	// The org name should also be randomized
	if strings.Contains(code, `firstName + "'s Organization"`) {
		t.Error("organization name should include a unique suffix via nanoid")
	}
}
