package resourcegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

func TestGoBaseTypeForFixture_JSONType(t *testing.T) {
	got := goBaseTypeForFixture(ddl.JSONType)
	if got != "json.RawMessage" {
		t.Errorf("goBaseTypeForFixture(%q) = %q, want %q", ddl.JSONType, got, "json.RawMessage")
	}
}

func TestGetSampleValue_JSONRawMessage(t *testing.T) {
	got := getSampleValue("json.RawMessage", "metadata")
	want := `json.RawMessage("{}")`
	if got != want {
		t.Errorf("getSampleValue(\"json.RawMessage\", \"metadata\") = %q, want %q", got, want)
	}
}

func TestGenerateFixture_JSONColumn_ImportsEncodingJSON(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "events",
		Table: ddl.Table{
			Name: "events",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "metadata", Type: ddl.JSONType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Must import encoding/json
	if !strings.Contains(code, `"encoding/json"`) {
		t.Error("expected \"encoding/json\" import for JSON column")
	}

	// Must use json.RawMessage sample value
	if !strings.Contains(code, `json.RawMessage("{}")`) {
		t.Error("expected json.RawMessage(\"{}\") sample value for JSON column")
	}
}

func TestGenerateFixture_NoJSONColumn_OmitsEncodingJSON(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "users",
		Table: ddl.Table{
			Name: "users",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "age", Type: ddl.IntegerType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	if strings.Contains(code, `"encoding/json"`) {
		t.Error("should NOT import \"encoding/json\" when there are no JSON columns")
	}
}

func TestGenerateFixture_JSONColumn_ValidGo(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "events",
		Table: ddl.Table{
			Name: "events",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "metadata", Type: ddl.JSONType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	// Verify the generated code is syntactically valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated fixture with JSON column is not valid Go: %v\n%s", err, string(result))
	}
}

func TestGenerateFixture_NullableJSONColumn_OmitsEncodingJSON(t *testing.T) {
	// A nullable JSON column is skipped in the fixture (gets nil),
	// so no encoding/json import should be emitted.
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "events",
		Table: ddl.Table{
			Name: "events",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "metadata", Type: ddl.JSONType, Nullable: true},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	if strings.Contains(code, `"encoding/json"`) {
		t.Error("should NOT import \"encoding/json\" when JSON column is nullable (skipped in fixture)")
	}
}

func TestGenerateFixture_ValidGo(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "pets",
		Table: ddl.Table{
			Name: "pets",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "species", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Verify fixture package
	if !strings.Contains(code, "package fixture") {
		t.Error("expected package fixture")
	}

	// Verify uses *sql.Tx, not *api.Client
	if !strings.Contains(code, "tx *sql.Tx") {
		t.Error("expected tx *sql.Tx parameter")
	}
	if strings.Contains(code, "*api.Client") {
		t.Error("should not reference *api.Client")
	}

	// Verify calls query runner
	if !strings.Contains(code, "dbrunner.NewQueryRunner(tx)") {
		t.Error("expected dbrunner.NewQueryRunner(tx)")
	}

	// Verify returns runner result type
	if !strings.Contains(code, "*queries.CreatePetResult") {
		t.Error("expected return type *queries.CreatePetResult")
	}

	// Verify calls runner.CreatePet
	if !strings.Contains(code, "runner.CreatePet(ctx") {
		t.Error("expected runner.CreatePet(ctx, ...) call")
	}

	// Verify sample values for required columns
	if !strings.Contains(code, `Name:`) {
		t.Error("expected Name field in params")
	}
	if !strings.Contains(code, `Species:`) {
		t.Error("expected Species field in params")
	}
}

func TestGenerateFixture_WithFKDependency(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "books",
		Table: ddl.Table{
			Name: "books",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "author_id", Type: ddl.BigintType, References: "authors"},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "postgres",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Verify dependency fixture import
	if !strings.Contains(code, `authorfixture "myapp/api/authors/fixture"`) {
		t.Error("expected authorfixture import")
	}

	// Verify dependency is created via tx
	if !strings.Contains(code, "authorfixture.Create(t, ctx, tx)") {
		t.Error("expected authorfixture.Create(t, ctx, tx)")
	}

	// Verify FK field uses dependency's PublicId
	if !strings.Contains(code, "author.PublicId") {
		t.Error("expected AuthorId: author.PublicId")
	}
}

func TestGenerateFixture_NullableFKSkipped(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "category_id", Type: ddl.BigintType, References: "categories", Nullable: true},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Nullable FK should not generate a dependency fixture import
	if strings.Contains(code, "categoryfixture") {
		t.Error("nullable FK should not import dependency fixture")
	}
}

func TestGenerateFixture_ScopeColumnUsesInternalId(t *testing.T) {
	// Regression test for Bug 2: when a scope column (e.g., organization_id)
	// references another table, the fixture must use the internal integer Id
	// (not PublicId) because the INSERT uses a direct placeholder for scope columns.
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "accounts",
		Table: ddl.Table{
			Name: "accounts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "email", Type: ddl.StringType},
				{Name: "first_name", Type: ddl.StringType},
				{Name: "last_name", Type: ddl.StringType},
				{Name: "organization_id", Type: ddl.BigintType, References: "organizations"},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
			},
		},
		Schema: map[string]ddl.Table{
			"organizations": {
				Name: "organizations",
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
					{Name: "public_id", Type: ddl.StringType},
					{Name: "name", Type: ddl.StringType},
					{Name: "created_at", Type: ddl.DatetimeType},
					{Name: "updated_at", Type: ddl.DatetimeType},
				},
			},
		},
		Dialect:     "postgres",
		ScopeColumn: "organization_id",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Verify valid Go — this would fail if CreateAccountResult lacks an Id field
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Scope column must reference the internal Id, not PublicId
	if !strings.Contains(code, "organization.Id") {
		t.Error("scope column should reference organization.Id (internal integer ID)")
	}

	// Must NOT use PublicId for the scope column
	if strings.Contains(code, "OrganizationId: organization.PublicId") {
		t.Error("scope column must NOT use organization.PublicId — it needs the raw integer ID")
	}

	// Verify the organization dependency fixture is imported and created
	if !strings.Contains(code, "organizationfixture") {
		t.Error("expected organizationfixture import for scope column dependency")
	}
	if !strings.Contains(code, "organizationfixture.Create(t, ctx, tx)") {
		t.Error("expected organizationfixture.Create call for scope column dependency")
	}
}

func TestGenerateFixture_SetsAuthorAccountId(t *testing.T) {
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "body", Type: ddl.TextType},
				{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "postgres",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, code)
	}

	// Verify accounts fixture import
	if !strings.Contains(code, `accountfixture "myapp/api/accounts/fixture"`) {
		t.Error("expected accountfixture import for author_account_id dependency")
	}

	// Verify account dependency is created via tx
	if !strings.Contains(code, "accountfixture.Create(t, ctx, tx)") {
		t.Error("expected accountfixture.Create(t, ctx, tx) call")
	}

	// Verify AuthorAccountId is set using account.Id (internal integer FK)
	if !strings.Contains(code, "AuthorAccountId: account.Id") {
		t.Error("expected AuthorAccountId: account.Id in create params")
	}

	// AuthorAccountId should NOT use PublicId (it's a raw integer FK)
	if strings.Contains(code, "AuthorAccountId: account.PublicId") {
		t.Error("AuthorAccountId must NOT use account.PublicId — it needs the raw integer ID")
	}
}

func TestGenerateFixture_NoDuplicateAccountVar_WhenAccountIdAndAuthorAccountId(t *testing.T) {
	// Regression test for Bug 11: when a table has both account_id:references:accounts
	// AND the auto-injected author_account_id column, the generator must emit only ONE
	// `account := accountfixture.Create(t, ctx, tx)` declaration, not two.
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "messages",
		Table: ddl.Table{
			Name: "messages",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "body", Type: ddl.TextType},
				{Name: "channel_id", Type: ddl.BigintType, References: "channels"},
				{Name: "account_id", Type: ddl.BigintType, References: "accounts"},
				{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "postgres",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// The generated code must be valid Go — a duplicate short-var declaration
	// would cause a parse error ("no new variables on left side of :=").
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go (likely duplicate variable declaration): %v\n%s", err, code)
	}

	// There must be exactly ONE `account := accountfixture.Create(t, ctx, tx)` line.
	occurrences := strings.Count(code, "account := accountfixture.Create(t, ctx, tx)")
	if occurrences != 1 {
		t.Errorf("expected exactly 1 account := accountfixture.Create(...) declaration, got %d\n%s", occurrences, code)
	}

	// The accountfixture import must appear exactly once.
	importOccurrences := strings.Count(code, `accountfixture "myapp/api/accounts/fixture"`)
	if importOccurrences != 1 {
		t.Errorf("expected exactly 1 accountfixture import, got %d\n%s", importOccurrences, code)
	}

	// Both FK fields should still be set in the create params.
	if !strings.Contains(code, "AccountId:") {
		t.Error("expected AccountId field in create params")
	}
	if !strings.Contains(code, "AuthorAccountId: account.Id") {
		t.Error("expected AuthorAccountId: account.Id in create params")
	}

	// Channel dependency should still be present.
	if !strings.Contains(code, "channelfixture.Create(t, ctx, tx)") {
		t.Error("expected channelfixture.Create(t, ctx, tx) for channel_id FK")
	}
}

func TestGenerateFixture_NoDuplicateAccountVar_ChannelMembers(t *testing.T) {
	// Regression test for Bug 11 (channel_members variant): same duplicate-var
	// issue when channel_members has account_id and author_account_id.
	cfg := FixtureGenConfig{
		ModulePath: "myapp",
		TableName:  "channel_members",
		Table: ddl.Table{
			Name: "channel_members",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "channel_id", Type: ddl.BigintType, References: "channels"},
				{Name: "account_id", Type: ddl.BigintType, References: "accounts"},
				{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
				{Name: "created_at", Type: ddl.DatetimeType},
				{Name: "updated_at", Type: ddl.DatetimeType},
				{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
			},
		},
		Schema:  map[string]ddl.Table{},
		Dialect: "sqlite",
	}

	result, err := GenerateFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	code := string(result)

	// Must be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", result, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go (likely duplicate variable declaration): %v\n%s", err, code)
	}

	// Exactly one account variable declaration
	occurrences := strings.Count(code, "account := accountfixture.Create(t, ctx, tx)")
	if occurrences != 1 {
		t.Errorf("expected exactly 1 account := accountfixture.Create(...) declaration, got %d\n%s", occurrences, code)
	}
}
