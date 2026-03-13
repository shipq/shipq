package crudquerydefs

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// accountsTable returns a representative accounts table for testing author JOINs.
func accountsTable() ddl.Table {
	return ddl.Table{
		Name: "accounts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "first_name", Type: ddl.StringType},
			{Name: "last_name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}
}

// postsTable returns a representative table for testing with FK, author_account_id,
// soft delete, timestamps, and a variety of column types.
func postsTable() ddl.Table {
	return ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "body", Type: ddl.TextType},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "author_account_id", Type: ddl.BigintType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}
}

func categoriesTable() ddl.Table {
	return ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}
}

func allTables() map[string]ddl.Table {
	return map[string]ddl.Table{
		"posts":      postsTable(),
		"categories": categoriesTable(),
		"accounts":   accountsTable(),
	}
}

func TestGenerateCRUDQueryDefs_ValidGo(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateCRUDQueryDefs() error = %v", err)
	}

	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", parseErr, string(code))
	}
}

func TestGenerateCRUDQueryDefs_PackageAndImports(t *testing.T) {
	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "posts",
		Table:      postsTable(),
		Schema:     allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("GenerateCRUDQueryDefs() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "package posts") {
		t.Error("missing package posts declaration")
	}

	if !strings.Contains(codeStr, `"example.com/myapp/shipq/db/schema"`) {
		t.Error("missing schema import")
	}

	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/db/portsql/query"`) {
		t.Error("missing query import")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `query.MustDefineOne("GetPostByPublicID"`) {
		t.Error("missing GetPostByPublicID query definition")
	}
	if !strings.Contains(codeStr, `schema.Posts.PublicId().Eq(query.Param[string]("publicId"))`) {
		t.Error("missing PublicId where clause in GetPost")
	}
	if !strings.Contains(codeStr, "schema.Posts.DeletedAt().IsNull()") {
		t.Error("missing DeletedAt().IsNull() in GetPost")
	}
	if !strings.Contains(codeStr, "schema.Posts.OrganizationId().Eq(") {
		t.Error("missing scope column in GetPost WHERE")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	// Posts table has created_at + id, so it should use MustDefinePaginated
	if !strings.Contains(codeStr, `query.MustDefinePaginated("ListPosts"`) {
		t.Error("missing ListPosts paginated query definition")
	}
	if !strings.Contains(codeStr, "schema.Posts.DeletedAt().IsNull()") {
		t.Error("missing DeletedAt().IsNull() in ListPosts")
	}
	if !strings.Contains(codeStr, "schema.Posts.OrganizationId().Eq(") {
		t.Error("missing scope column in ListPosts WHERE")
	}
	if !strings.Contains(codeStr, "schema.Posts.CreatedAt().Desc()") {
		t.Error("missing cursor column created_at with .Desc()")
	}
	if !strings.Contains(codeStr, "schema.Posts.PublicId().Desc()") {
		t.Error("missing cursor column public_id with .Desc()")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_NoCursorFallback(t *testing.T) {
	// Table without created_at should fall back to MustDefineMany
	table := ddl.Table{
		Name: "tags",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "tags",
		Table:      table,
		Schema:     map[string]ddl.Table{"tags": table},
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	// Should use MustDefineMany (not MustDefinePaginated) since no created_at
	if !strings.Contains(codeStr, `query.MustDefineMany("ListTags"`) {
		t.Error("missing ListTags many query definition (should not be paginated)")
	}
	if strings.Contains(codeStr, `query.MustDefinePaginated("ListTags"`) {
		t.Error("ListTags should not be paginated without created_at column")
	}
}

func TestGenerateCRUDQueryDefs_CreateQuery(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `query.MustDefineOne("CreatePost"`) {
		t.Error("missing CreatePost query definition")
	}
	if !strings.Contains(codeStr, "query.InsertInto(schema.Posts)") {
		t.Error("missing InsertInto call")
	}
	// FK resolution via subquery
	if !strings.Contains(codeStr, "query.Subquery(") {
		t.Error("missing Subquery for FK resolution")
	}
	if !strings.Contains(codeStr, "schema.Categories.Id()") {
		t.Error("missing FK subquery to categories")
	}
	// author_account_id should use a plain Param (not subquery)
	if !strings.Contains(codeStr, `query.Param[int64]("authorAccountId")`) {
		t.Error("missing authorAccountId param")
	}
	// Returning clause
	if !strings.Contains(codeStr, "Returning(") {
		t.Error("missing Returning clause")
	}
}

func TestGenerateCRUDQueryDefs_UpdateQuery(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `query.MustDefineExec("UpdatePostByPublicID"`) {
		t.Error("missing UpdatePostByPublicID query definition")
	}
	if !strings.Contains(codeStr, "query.Update(schema.Posts)") {
		t.Error("missing Update call")
	}
	// updated_at = NOW()
	if !strings.Contains(codeStr, "query.Now()") {
		t.Error("missing NOW() for updated_at")
	}
	// Scope column should NOT be in SET, only in WHERE
	// Count occurrences: should appear in WHERE but not in Set
	if strings.Contains(codeStr, "Set(schema.Posts.OrganizationId()") {
		t.Error("scope column should not be in SET clause")
	}
	// author_account_id should NOT be updatable
	if strings.Contains(codeStr, "Set(schema.Posts.AuthorAccountId()") {
		t.Error("author_account_id should not be updatable")
	}
}

func TestGenerateCRUDQueryDefs_DeleteQuery_SoftDelete(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `query.MustDefineExec("SoftDeletePostByPublicID"`) {
		t.Error("missing SoftDeletePostByPublicID query definition")
	}
	// Soft delete should UPDATE ... SET deleted_at = NOW()
	if !strings.Contains(codeStr, "Set(schema.Posts.DeletedAt(), query.Now())") {
		t.Error("soft delete should SET deleted_at = NOW()")
	}
}

func TestGenerateCRUDQueryDefs_DeleteQuery_HardDelete(t *testing.T) {
	// Table without deleted_at => hard delete
	table := ddl.Table{
		Name: "tags",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "tags",
		Table:      table,
		Schema:     map[string]ddl.Table{"tags": table},
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `query.MustDefineExec("DeleteTag"`) {
		t.Error("missing DeleteTag query definition")
	}
	// Hard delete uses query.Delete
	if !strings.Contains(codeStr, "query.Delete(schema.Tags)") {
		t.Error("hard delete should use query.Delete")
	}
}

func TestGenerateCRUDQueryDefs_NoScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "categories",
		Table:      table,
		Schema:     map[string]ddl.Table{"categories": table},
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	// Valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", parseErr, codeStr)
	}

	// Should have all 5 CRUD queries
	if !strings.Contains(codeStr, `"GetCategoryByPublicID"`) {
		t.Error("missing GetCategoryByPublicID")
	}
	if !strings.Contains(codeStr, `"ListCategories"`) {
		t.Error("missing ListCategories")
	}
	if !strings.Contains(codeStr, `"CreateCategory"`) {
		t.Error("missing CreateCategory")
	}
	if !strings.Contains(codeStr, `"UpdateCategoryByPublicID"`) {
		t.Error("missing UpdateCategoryByPublicID")
	}
	if !strings.Contains(codeStr, `"DeleteCategory"`) {
		t.Error("missing DeleteCategory")
	}
}

func TestGenerateCRUDQueryDefs_TimestampImport(t *testing.T) {
	// Sessions table has expires_at (non-nullable timestamp) as a user column,
	// which requires "time" import for query.Param[time.Time].
	table := ddl.Table{
		Name: "sessions",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "expires_at", Type: ddl.TimestampType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}

	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "sessions",
		Table:      table,
		Schema:     map[string]ddl.Table{"sessions": table, "accounts": categoriesTable()},
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	// Must be valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", parseErr, codeStr)
	}

	// Must import "time" since expires_at is time.Time
	if !strings.Contains(codeStr, `"time"`) {
		t.Error("missing time import for timestamp column")
	}

	// Must use time.Time param
	if !strings.Contains(codeStr, `query.Param[time.Time]("expiresAt")`) {
		t.Error("missing time.Time param for expires_at")
	}
}

func TestGenerateCRUDQueryDefs_FKSubqueryInCreateAndUpdate(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	codeStr := string(code)

	// In CREATE: FK column should use subquery for resolution
	if !strings.Contains(codeStr, "query.Subquery(") {
		t.Error("missing Subquery for FK resolution in CREATE")
	}

	// The subquery should reference the categories table
	if !strings.Contains(codeStr, "query.From(schema.Categories)") {
		t.Error("missing FK subquery From(schema.Categories)")
	}

	// The subquery should select the id column
	if !strings.Contains(codeStr, "schema.Categories.Id()") {
		t.Error("missing FK subquery Select(schema.Categories.Id())")
	}

	// The subquery should filter by PublicId
	if !strings.Contains(codeStr, "schema.Categories.PublicId()") {
		t.Error("missing FK subquery Where(schema.Categories.PublicId())")
	}
}

// =============================================================================
// §3.1 — FK Resolution, Author JOIN, Scope Exclusion, CREATE Returning Tests
// =============================================================================

func TestGenerateCRUDQueryDefs_GetQuery_JoinsForFKColumns(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Must JOIN categories for category_id FK
	if !strings.Contains(codeStr, "Join(schema.Categories).On(schema.Categories.Id().Eq(schema.Posts.CategoryId()))") {
		t.Error("GET query missing Join on Categories for category_id FK")
	}

	// Must use SelectAs to resolve category_id to categories.public_id
	if !strings.Contains(codeStr, `SelectAs(schema.Categories.PublicId(), "category_id")`) {
		t.Error("GET query missing SelectAs for category_id FK resolution")
	}

	// Must NOT contain the raw FK column in the Select(...) block.
	// (It IS expected in the JOIN ON clause, so check the Select block specifically.)
	getIdx := strings.Index(codeStr, `"GetPostByPublicID"`)
	if getIdx == -1 {
		t.Fatal("missing GetPostByPublicID query in generated code")
	}
	getSection := codeStr[getIdx:]
	selectIdx := strings.Index(getSection, "Select(")
	if selectIdx == -1 {
		t.Fatal("GET query missing Select() call")
	}
	selectEnd := strings.Index(getSection[selectIdx:], ").")
	if selectEnd == -1 {
		t.Fatal("GET query Select() block not properly closed")
	}
	selectBlock := getSection[selectIdx : selectIdx+selectEnd]
	if strings.Contains(selectBlock, "schema.Posts.CategoryId()") {
		t.Error("GET query must NOT select raw schema.Posts.CategoryId() in Select(); should use SelectAs instead")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_JoinsForFKColumns(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Must JOIN categories for category_id FK
	if !strings.Contains(codeStr, "Join(schema.Categories).On(schema.Categories.Id().Eq(schema.Posts.CategoryId()))") {
		t.Error("LIST query missing Join on Categories for category_id FK")
	}

	// Must use SelectAs to resolve category_id to categories.public_id
	if !strings.Contains(codeStr, `SelectAs(schema.Categories.PublicId(), "category_id")`) {
		t.Error("LIST query missing SelectAs for category_id FK resolution")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery_NullableFKUsesLeftJoin(t *testing.T) {
	table := ddl.Table{
		Name: "articles",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "category_id", Type: ddl.BigintType, References: "categories", Nullable: true},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "articles",
		Table:      table,
		Schema: map[string]ddl.Table{
			"articles":   table,
			"categories": categoriesTable(),
		},
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", parseErr, codeStr)
	}

	// Nullable FK should use LeftJoin, not Join
	if !strings.Contains(codeStr, "LeftJoin(schema.Categories)") {
		t.Error("nullable FK column should use LeftJoin, not Join")
	}
	if strings.Contains(codeStr, "Join(schema.Categories)") && !strings.Contains(codeStr, "LeftJoin(schema.Categories)") {
		t.Error("nullable FK column should use LeftJoin, not plain Join")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery_AuthorJoin(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Must LEFT JOIN accounts for author_account_id (LEFT because the author
	// may be absent for public resources or if the account was deleted).
	if !strings.Contains(codeStr, "LeftJoin(schema.Accounts).On(schema.Accounts.Id().Eq(schema.Posts.AuthorAccountId()))") {
		t.Error("GET query missing LeftJoin on Accounts for author_account_id")
	}

	// Must have SelectAs for author fields
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.PublicId(), "author_id")`) {
		t.Error("GET query missing SelectAs for author_id")
	}
	if strings.Contains(codeStr, `SelectAs(schema.Accounts.Email(), "author_email")`) {
		t.Error("GET query must NOT contain SelectAs for author_email when ExposeEmail is false")
	}
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.FirstName(), "author_first_name")`) {
		t.Error("GET query missing SelectAs for author_first_name")
	}
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.LastName(), "author_last_name")`) {
		t.Error("GET query missing SelectAs for author_last_name")
	}

	// Must NOT contain raw author_account_id in Select() block.
	// (It IS expected in the JOIN ON clause, so check the Select block specifically.)
	getIdx := strings.Index(codeStr, `"GetPostByPublicID"`)
	if getIdx == -1 {
		t.Fatal("missing GetPostByPublicID query in generated code")
	}
	getSection := codeStr[getIdx:]
	selectIdx := strings.Index(getSection, "Select(")
	if selectIdx == -1 {
		t.Fatal("GET query missing Select() call")
	}
	selectEnd := strings.Index(getSection[selectIdx:], ").")
	if selectEnd == -1 {
		t.Fatal("GET query Select() block not properly closed")
	}
	selectBlock := getSection[selectIdx : selectIdx+selectEnd]
	if strings.Contains(selectBlock, "schema.Posts.AuthorAccountId()") {
		t.Error("GET query must NOT select raw schema.Posts.AuthorAccountId() in Select()")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_AuthorJoin(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// The LIST section (after MustDefinePaginated) should also have the author join
	listIdx := strings.Index(codeStr, "MustDefinePaginated")
	if listIdx == -1 {
		t.Fatal("missing MustDefinePaginated in generated code")
	}
	listSection := codeStr[listIdx:]

	if !strings.Contains(listSection, "LeftJoin(schema.Accounts).On(schema.Accounts.Id().Eq(schema.Posts.AuthorAccountId()))") {
		t.Error("LIST query missing LeftJoin on Accounts for author_account_id")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.PublicId(), "author_id")`) {
		t.Error("LIST query missing SelectAs for author_id")
	}
	if strings.Contains(listSection, `SelectAs(schema.Accounts.Email(), "author_email")`) {
		t.Error("LIST query must NOT contain SelectAs for author_email when ExposeEmail is false")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.FirstName(), "author_first_name")`) {
		t.Error("LIST query missing SelectAs for author_first_name")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.LastName(), "author_last_name")`) {
		t.Error("LIST query missing SelectAs for author_last_name")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery_AuthorJoin_ExposeEmail(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
		ExposeEmail: true,
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Must LEFT JOIN accounts for author_account_id
	if !strings.Contains(codeStr, "LeftJoin(schema.Accounts).On(schema.Accounts.Id().Eq(schema.Posts.AuthorAccountId()))") {
		t.Error("GET query missing LeftJoin on Accounts for author_account_id")
	}

	// Must have SelectAs for all author fields including email
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.PublicId(), "author_id")`) {
		t.Error("GET query missing SelectAs for author_id")
	}
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.Email(), "author_email")`) {
		t.Error("GET query missing SelectAs for author_email when ExposeEmail is true")
	}
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.FirstName(), "author_first_name")`) {
		t.Error("GET query missing SelectAs for author_first_name")
	}
	if !strings.Contains(codeStr, `SelectAs(schema.Accounts.LastName(), "author_last_name")`) {
		t.Error("GET query missing SelectAs for author_last_name")
	}

	// Must NOT contain raw author_account_id in Select() block.
	getIdx := strings.Index(codeStr, `"GetPostByPublicID"`)
	if getIdx == -1 {
		t.Fatal("missing GetPostByPublicID query in generated code")
	}
	getSection := codeStr[getIdx:]
	selectIdx := strings.Index(getSection, "Select(")
	if selectIdx == -1 {
		t.Fatal("GET query missing Select() call")
	}
	selectEnd := strings.Index(getSection[selectIdx:], ").")
	if selectEnd == -1 {
		t.Fatal("GET query Select() block not properly closed")
	}
	selectBlock := getSection[selectIdx : selectIdx+selectEnd]
	if strings.Contains(selectBlock, "schema.Posts.AuthorAccountId()") {
		t.Error("GET query must NOT select raw schema.Posts.AuthorAccountId() in Select()")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_AuthorJoin_ExposeEmail(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
		ExposeEmail: true,
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// The LIST section (after MustDefinePaginated) should also have the author join
	listIdx := strings.Index(codeStr, "MustDefinePaginated")
	if listIdx == -1 {
		t.Fatal("missing MustDefinePaginated in generated code")
	}
	listSection := codeStr[listIdx:]

	if !strings.Contains(listSection, "LeftJoin(schema.Accounts).On(schema.Accounts.Id().Eq(schema.Posts.AuthorAccountId()))") {
		t.Error("LIST query missing LeftJoin on Accounts for author_account_id")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.PublicId(), "author_id")`) {
		t.Error("LIST query missing SelectAs for author_id")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.Email(), "author_email")`) {
		t.Error("LIST query missing SelectAs for author_email when ExposeEmail is true")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.FirstName(), "author_first_name")`) {
		t.Error("LIST query missing SelectAs for author_first_name")
	}
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.LastName(), "author_last_name")`) {
		t.Error("LIST query missing SelectAs for author_last_name")
	}
}

func TestGenerateCRUDQueryDefs_ScopeColumnNotJoined(t *testing.T) {
	// organization_id is a scope column — it should NOT be joined even though
	// it could theoretically reference another table. Scope columns stay as
	// raw params in the WHERE clause.
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType, References: "organizations"},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	orgsTable := ddl.Table{
		Name: "organizations",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
		},
	}

	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       table,
		ScopeColumn: "organization_id",
		Schema: map[string]ddl.Table{
			"posts":         table,
			"organizations": orgsTable,
		},
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "queries.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", parseErr, codeStr)
	}

	// Scope column should NOT be joined
	if strings.Contains(codeStr, "Join(schema.Organizations)") || strings.Contains(codeStr, "LeftJoin(schema.Organizations)") {
		t.Error("scope column organization_id should NOT be joined — it stays as a raw param in the WHERE clause")
	}

	// Scope column should remain in the Select() list (it's a plain column when scoped)
	if !strings.Contains(codeStr, "schema.Posts.OrganizationId()") {
		t.Error("scope column organization_id should still appear in Select() as a plain column")
	}
}

func TestGenerateCRUDQueryDefs_CreateQuery_ReturningIdAndPublicId(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Find the CREATE query section
	createIdx := strings.Index(codeStr, `"CreatePost"`)
	if createIdx == -1 {
		t.Fatal("missing CreatePost query in generated code")
	}
	// Find the next query definition to bound the CREATE section
	nextQueryIdx := strings.Index(codeStr[createIdx+1:], "query.MustDefine")
	var createSection string
	if nextQueryIdx == -1 {
		createSection = codeStr[createIdx:]
	} else {
		createSection = codeStr[createIdx : createIdx+1+nextQueryIdx]
	}

	// Find the Returning(...) block within the CREATE section
	retIdx := strings.Index(createSection, "Returning(")
	if retIdx == -1 {
		t.Fatal("CREATE query missing Returning() clause")
	}
	retEnd := strings.Index(createSection[retIdx:], ").")
	if retEnd == -1 {
		t.Fatal("CREATE query Returning() block not properly closed")
	}
	retBlock := createSection[retIdx : retIdx+retEnd]

	// Returning clause must contain internal id (needed by fixtures for scope-column FK resolution)
	if !strings.Contains(retBlock, "schema.Posts.Id()") {
		t.Error("CREATE query Returning should contain schema.Posts.Id()")
	}

	// Returning clause must also contain public_id
	if !strings.Contains(retBlock, "schema.Posts.PublicId()") {
		t.Error("CREATE query Returning should contain schema.Posts.PublicId()")
	}

	// Must NOT contain raw FK columns in Returning
	if strings.Contains(retBlock, "schema.Posts.CategoryId()") {
		t.Error("CREATE query Returning must NOT contain schema.Posts.CategoryId()")
	}

	// Must NOT contain author_account_id in Returning
	if strings.Contains(retBlock, "schema.Posts.AuthorAccountId()") {
		t.Error("CREATE query Returning must NOT contain schema.Posts.AuthorAccountId()")
	}

	// CREATE query should NOT contain any JOIN clauses — FK resolution happens
	// in the handler's re-fetch, not in the INSERT.
	if strings.Contains(createSection, "Join(schema.") {
		t.Error("CREATE query must NOT contain JOIN clauses")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery_NeverSelectsInternalId(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Find the GET query section
	getIdx := strings.Index(codeStr, `"GetPostByPublicID"`)
	if getIdx == -1 {
		t.Fatal("missing GetPostByPublicID query in generated code")
	}
	// Bound to next query
	nextIdx := strings.Index(codeStr[getIdx+1:], "query.MustDefine")
	var getSection string
	if nextIdx == -1 {
		getSection = codeStr[getIdx:]
	} else {
		getSection = codeStr[getIdx : getIdx+1+nextIdx]
	}

	if strings.Contains(getSection, "schema.Posts.Id()") {
		t.Error("GET query must NOT contain schema.Posts.Id() in its Select list")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery_NeverSelectsAuthorAccountId(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Find the GET query section
	getIdx := strings.Index(codeStr, `"GetPostByPublicID"`)
	if getIdx == -1 {
		t.Fatal("missing GetPostByPublicID query in generated code")
	}
	nextIdx := strings.Index(codeStr[getIdx+1:], "query.MustDefine")
	var getSection string
	if nextIdx == -1 {
		getSection = codeStr[getIdx:]
	} else {
		getSection = codeStr[getIdx : getIdx+1+nextIdx]
	}

	// schema.Posts.AuthorAccountId() must not appear in Select list.
	// It IS referenced in the JOIN ON clause — that's fine. We check that
	// it does NOT appear inside the Select(...) block.
	selectIdx := strings.Index(getSection, "Select(")
	if selectIdx == -1 {
		t.Fatal("GET query missing Select() call")
	}
	selectEnd := strings.Index(getSection[selectIdx:], ").")
	if selectEnd == -1 {
		t.Fatal("GET query Select() block not properly closed")
	}
	selectBlock := getSection[selectIdx : selectIdx+selectEnd]

	if strings.Contains(selectBlock, "schema.Posts.AuthorAccountId()") {
		t.Error("GET query must NOT contain schema.Posts.AuthorAccountId() in its Select() list")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_NeverSelectsInternalId(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Find the LIST query section
	listIdx := strings.Index(codeStr, `"ListPosts"`)
	if listIdx == -1 {
		t.Fatal("missing ListPosts query in generated code")
	}
	nextIdx := strings.Index(codeStr[listIdx+1:], "query.MustDefine")
	var listSection string
	if nextIdx == -1 {
		listSection = codeStr[listIdx:]
	} else {
		listSection = codeStr[listIdx : listIdx+1+nextIdx]
	}

	// Check inside the Select() block specifically
	selectIdx := strings.Index(listSection, "Select(")
	if selectIdx == -1 {
		t.Fatal("LIST query missing Select() call")
	}
	selectEnd := strings.Index(listSection[selectIdx:], ").")
	if selectEnd == -1 {
		t.Fatal("LIST query Select() block not properly closed")
	}
	selectBlock := listSection[selectIdx : selectIdx+selectEnd]

	if strings.Contains(selectBlock, "schema.Posts.Id()") {
		t.Error("LIST query must NOT contain schema.Posts.Id() in its Select() list")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_NeverSelectsAuthorAccountId(t *testing.T) {
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "posts",
		Table:       postsTable(),
		ScopeColumn: "organization_id",
		Schema:      allTables(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Find the LIST query section
	listIdx := strings.Index(codeStr, `"ListPosts"`)
	if listIdx == -1 {
		t.Fatal("missing ListPosts query in generated code")
	}
	nextIdx := strings.Index(codeStr[listIdx+1:], "query.MustDefine")
	var listSection string
	if nextIdx == -1 {
		listSection = codeStr[listIdx:]
	} else {
		listSection = codeStr[listIdx : listIdx+1+nextIdx]
	}

	selectIdx := strings.Index(listSection, "Select(")
	if selectIdx == -1 {
		t.Fatal("LIST query missing Select() call")
	}
	selectEnd := strings.Index(listSection[selectIdx:], ").")
	if selectEnd == -1 {
		t.Fatal("LIST query Select() block not properly closed")
	}
	selectBlock := listSection[selectIdx : selectIdx+selectEnd]

	if strings.Contains(selectBlock, "schema.Posts.AuthorAccountId()") {
		t.Error("LIST query must NOT contain schema.Posts.AuthorAccountId() in its Select() list")
	}
}

// messagesTable returns a table where account_id:references:accounts AND
// author_account_id both point at the accounts table — the duplicate-table
// scenario that triggers Bug 3.
func messagesTable() ddl.Table {
	return ddl.Table{
		Name: "messages",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "body", Type: ddl.TextType},
			{Name: "account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "author_account_id", Type: ddl.BigintType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}
}

func allTablesWithMessages() map[string]ddl.Table {
	t := allTables()
	t["messages"] = messagesTable()
	return t
}

func TestGenerateCRUDQueryDefs_GetQuery_DuplicateTableAlias(t *testing.T) {
	// Regression test for Bug 3: when a table has both account_id:references:accounts
	// and author_account_id (→ accounts), the GET query must alias each JOIN so that
	// MySQL/Postgres don't reject the SQL with "Not unique table/alias".
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "messages",
		Table:       messagesTable(),
		ScopeColumn: "organization_id",
		Schema:      allTablesWithMessages(),
		ExposeEmail: true,
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Verify valid Go (unused imports / syntax errors would fail here)
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, codeStr)
	}

	// Find the GET query section
	getIdx := strings.Index(codeStr, `"GetMessageByPublicID"`)
	if getIdx == -1 {
		t.Fatal("missing GetMessageByPublicID query in generated code")
	}
	nextIdx := strings.Index(codeStr[getIdx+1:], "query.MustDefine")
	var getSection string
	if nextIdx == -1 {
		getSection = codeStr[getIdx:]
	} else {
		getSection = codeStr[getIdx : getIdx+1+nextIdx]
	}

	// Both JOINs on Accounts must use .As(...) aliases
	if !strings.Contains(getSection, `.As("account")`) {
		t.Error("GET query missing .As(\"account\") alias for account_id FK join")
	}
	if !strings.Contains(getSection, `.As("author")`) {
		t.Error("GET query missing .As(\"author\") alias for author join")
	}

	// JOIN ON clauses must use .WithTable(alias)
	if !strings.Contains(getSection, `.WithTable("account")`) {
		t.Error("GET query missing .WithTable(\"account\") in JOIN ON or SelectAs")
	}
	if !strings.Contains(getSection, `.WithTable("author")`) {
		t.Error("GET query missing .WithTable(\"author\") in JOIN ON or SelectAs")
	}

	// SelectAs for the FK column must use .WithTable
	if !strings.Contains(getSection, `SelectAs(schema.Accounts.PublicId().WithTable("account"), "account_id")`) {
		t.Error("GET query missing aliased SelectAs for account_id FK resolution")
	}

	// SelectAs for author fields must use .WithTable
	if !strings.Contains(getSection, `SelectAs(schema.Accounts.PublicId().WithTable("author"), "author_id")`) {
		t.Error("GET query missing aliased SelectAs for author_id")
	}
	if !strings.Contains(getSection, `SelectAs(schema.Accounts.Email().WithTable("author"), "author_email")`) {
		t.Error("GET query missing aliased SelectAs for author_email")
	}
}

func TestGenerateCRUDQueryDefs_ListQuery_DuplicateTableAlias(t *testing.T) {
	// Same setup as GetQuery test, but verifies the LIST query.
	cfg := Config{
		ModulePath:  "example.com/myapp",
		TableName:   "messages",
		Table:       messagesTable(),
		ScopeColumn: "organization_id",
		Schema:      allTablesWithMessages(),
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Find the LIST query section
	listIdx := strings.Index(codeStr, `"ListMessages"`)
	if listIdx == -1 {
		t.Fatal("missing ListMessages query in generated code")
	}
	nextIdx := strings.Index(codeStr[listIdx+1:], "query.MustDefine")
	var listSection string
	if nextIdx == -1 {
		listSection = codeStr[listIdx:]
	} else {
		listSection = codeStr[listIdx : listIdx+1+nextIdx]
	}

	// Both JOINs on Accounts must use .As(...) aliases
	if !strings.Contains(listSection, `.As("account")`) {
		t.Error("LIST query missing .As(\"account\") alias for account_id FK join")
	}
	if !strings.Contains(listSection, `.As("author")`) {
		t.Error("LIST query missing .As(\"author\") alias for author join")
	}

	// SelectAs for the FK column must use .WithTable
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.PublicId().WithTable("account"), "account_id")`) {
		t.Error("LIST query missing aliased SelectAs for account_id FK resolution")
	}

	// SelectAs for author fields must use .WithTable
	if !strings.Contains(listSection, `SelectAs(schema.Accounts.PublicId().WithTable("author"), "author_id")`) {
		t.Error("LIST query missing aliased SelectAs for author_id")
	}
}

func TestGenerateCRUDQueryDefs_GetQuery_SingleFKNoAlias(t *testing.T) {
	// When a table is only referenced once (e.g., category_id:references:categories
	// with no author_account_id), no .As(...) should be emitted.
	// This ensures the alias change is backwards-compatible for the common case.
	table := ddl.Table{
		Name: "articles",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}
	schema := map[string]ddl.Table{
		"articles":   table,
		"categories": categoriesTable(),
	}

	cfg := Config{
		ModulePath: "example.com/myapp",
		TableName:  "articles",
		Table:      table,
		Schema:     schema,
	}

	code, err := GenerateCRUDQueryDefs(cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	codeStr := string(code)

	// Verify valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, codeStr)
	}

	// Single-FK join must NOT use .As(...)
	if strings.Contains(codeStr, ".As(") {
		t.Error("single FK join must NOT use .As() alias — only needed when table appears more than once")
	}

	// Must still have the unaliased JOIN
	if !strings.Contains(codeStr, "Join(schema.Categories).On(schema.Categories.Id().Eq(schema.Articles.CategoryId()))") {
		t.Error("GET query missing unaliased Join on Categories for category_id FK")
	}

	// Must still have the unaliased SelectAs
	if !strings.Contains(codeStr, `SelectAs(schema.Categories.PublicId(), "category_id")`) {
		t.Error("GET query missing unaliased SelectAs for category_id FK resolution")
	}
}
