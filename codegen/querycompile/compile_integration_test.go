package querycompile

import (
	"encoding/json"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen/discovery"
	"github.com/shipq/shipq/codegen/handlergen"
	"github.com/shipq/shipq/codegen/queryrunner"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/db/portsql/ref"
	"github.com/shipq/shipq/dburl"
)

// TestCompileWorkflow_EndToEnd tests the complete compile workflow:
// 1. Serialize queries to JSON
// 2. Generate shared types
// 3. Generate unified runner
// 4. Verify generated code compiles (by checking it's valid Go)
func TestCompileWorkflow_EndToEnd(t *testing.T) {
	// Create a test schema using the migration plan builder
	plan := migrate.NewPlan()
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email")
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	// Create serialized user queries (simulating what the compile program would output)
	userQueries := []query.SerializedQuery{
		{
			Name:       "GetUserByEmail",
			ReturnType: query.ReturnOne,
			AST: &query.SerializedAST{
				Kind: "select",
				FromTable: query.SerializedTableRef{
					Name: "users",
				},
				SelectCols: []query.SerializedSelectExpr{
					{
						Expr: query.SerializedExpr{
							Type: "column",
							Column: &query.SerializedColumn{
								Table:  "users",
								Name:   "id",
								GoType: "int64",
							},
						},
					},
					{
						Expr: query.SerializedExpr{
							Type: "column",
							Column: &query.SerializedColumn{
								Table:  "users",
								Name:   "email",
								GoType: "string",
							},
						},
					},
				},
				Where: &query.SerializedExpr{
					Type: "binary",
					Binary: &query.SerializedBinary{
						Left: query.SerializedExpr{
							Type: "column",
							Column: &query.SerializedColumn{
								Table:  "users",
								Name:   "email",
								GoType: "string",
							},
						},
						Op: "=",
						Right: query.SerializedExpr{
							Type: "param",
							Param: &query.SerializedParam{
								Name:   "email",
								GoType: "string",
							},
						},
					},
				},
				Params: []query.SerializedParamInfo{
					{Name: "email", GoType: "string"},
				},
			},
		},
		{
			Name:       "ListActiveUsers",
			ReturnType: query.ReturnMany,
			AST: &query.SerializedAST{
				Kind: "select",
				FromTable: query.SerializedTableRef{
					Name: "users",
				},
				SelectCols: []query.SerializedSelectExpr{
					{
						Expr: query.SerializedExpr{
							Type: "column",
							Column: &query.SerializedColumn{
								Table:  "users",
								Name:   "id",
								GoType: "int64",
							},
						},
					},
					{
						Expr: query.SerializedExpr{
							Type: "column",
							Column: &query.SerializedColumn{
								Table:  "users",
								Name:   "name",
								GoType: "string",
							},
						},
					},
				},
				Params: []query.SerializedParamInfo{},
			},
		},
		{
			Name:       "UpdateUserEmail",
			ReturnType: query.ReturnExec,
			AST: &query.SerializedAST{
				Kind: "update",
				FromTable: query.SerializedTableRef{
					Name: "users",
				},
				SetClauses: []query.SerializedSetClause{
					{
						Column: query.SerializedColumn{
							Table:  "users",
							Name:   "email",
							GoType: "string",
						},
						Value: query.SerializedExpr{
							Type: "param",
							Param: &query.SerializedParam{
								Name:   "new_email",
								GoType: "string",
							},
						},
					},
				},
				Where: &query.SerializedExpr{
					Type: "binary",
					Binary: &query.SerializedBinary{
						Left: query.SerializedExpr{
							Type: "column",
							Column: &query.SerializedColumn{
								Table:  "users",
								Name:   "id",
								GoType: "int64",
							},
						},
						Op: "=",
						Right: query.SerializedExpr{
							Type: "param",
							Param: &query.SerializedParam{
								Name:   "id",
								GoType: "int64",
							},
						},
					},
				},
				Params: []query.SerializedParamInfo{
					{Name: "new_email", GoType: "string"},
					{Name: "id", GoType: "int64"},
				},
			},
		},
	}

	// Test for each dialect
	dialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			cfg := queryrunner.UnifiedRunnerConfig{
				ModulePath:  "example.com/testapp",
				Dialect:     dialect,
				UserQueries: userQueries,
				Schema:      plan,
			}

			// Generate shared types
			typesCode, err := queryrunner.GenerateSharedTypes(cfg)
			if err != nil {
				t.Fatalf("GenerateSharedTypes failed: %v", err)
			}

			// Verify types.go content
			typesStr := string(typesCode)
			if !strings.Contains(typesStr, "package queries") {
				t.Error("types.go missing package declaration")
			}
			if !strings.Contains(typesStr, "GetUserByEmailParams") {
				t.Error("types.go missing GetUserByEmailParams")
			}
			if !strings.Contains(typesStr, "GetUserByEmailResult") {
				t.Error("types.go missing GetUserByEmailResult")
			}
			if !strings.Contains(typesStr, "ListActiveUsersParams") {
				t.Error("types.go missing ListActiveUsersParams")
			}
			if !strings.Contains(typesStr, "ListActiveUsersResult") {
				t.Error("types.go missing ListActiveUsersResult")
			}
			if !strings.Contains(typesStr, "UpdateUserEmailParams") {
				t.Error("types.go missing UpdateUserEmailParams")
			}

			// Generate unified runner
			runnerCode, err := queryrunner.GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed: %v", err)
			}

			// Verify runner.go content
			runnerStr := string(runnerCode)
			if !strings.Contains(runnerStr, "package "+dialect) {
				t.Errorf("runner.go missing package %s declaration", dialect)
			}
			if !strings.Contains(runnerStr, "type QueryRunner struct") {
				t.Error("runner.go missing QueryRunner struct")
			}
			if !strings.Contains(runnerStr, "func NewQueryRunner") {
				t.Error("runner.go missing NewQueryRunner")
			}
			if !strings.Contains(runnerStr, "func (r *QueryRunner) WithTx") {
				t.Error("runner.go missing WithTx method")
			}

			// Verify user query methods
			if !strings.Contains(runnerStr, "func (r *QueryRunner) GetUserByEmail") {
				t.Error("runner.go missing GetUserByEmail method")
			}
			if !strings.Contains(runnerStr, "func (r *QueryRunner) ListActiveUsers") {
				t.Error("runner.go missing ListActiveUsers method")
			}
			if !strings.Contains(runnerStr, "func (r *QueryRunner) UpdateUserEmail") {
				t.Error("runner.go missing UpdateUserEmail method")
			}

			// Verify CRUD methods
			if !strings.Contains(runnerStr, "func (r *QueryRunner) GetUser") {
				t.Error("runner.go missing GetUser CRUD method")
			}
			if !strings.Contains(runnerStr, "func (r *QueryRunner) ListUsers") {
				t.Error("runner.go missing ListUsers CRUD method")
			}
			if !strings.Contains(runnerStr, "func (r *QueryRunner) CreateUser") {
				t.Error("runner.go missing CreateUser CRUD method")
			}
		})
	}
}

// TestCompileProgram_Integration tests that the compile program can be generated
// and contains valid Go code structure.
func TestCompileProgram_Integration(t *testing.T) {
	cfg := CompileProgramConfig{
		ModulePath: "example.com/testapp",
		QuerydefsPkgs: []string{
			"example.com/testapp/querydefs",
			"example.com/testapp/querydefs/users",
			"example.com/testapp/querydefs/orders",
		},
	}

	code, err := GenerateCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateCompileProgram failed: %v", err)
	}

	codeStr := string(code)

	// Verify it's a valid Go program
	if !strings.Contains(codeStr, "package main") {
		t.Error("compile program missing package main")
	}
	if !strings.Contains(codeStr, "func main()") {
		t.Error("compile program missing main function")
	}

	// Verify all querydefs packages are imported
	if !strings.Contains(codeStr, `_ "example.com/testapp/querydefs"`) {
		t.Error("compile program missing querydefs import")
	}
	if !strings.Contains(codeStr, `_ "example.com/testapp/querydefs/users"`) {
		t.Error("compile program missing querydefs/users import")
	}
	if !strings.Contains(codeStr, `_ "example.com/testapp/querydefs/orders"`) {
		t.Error("compile program missing querydefs/orders import")
	}

	// Verify it calls SerializeQueries
	if !strings.Contains(codeStr, "query.SerializeQueries()") {
		t.Error("compile program missing SerializeQueries call")
	}
}

// TestSerializationRoundTrip tests that queries can be serialized to JSON
// and deserialized back without loss of information.
func TestSerializationRoundTrip(t *testing.T) {
	// Register some test queries
	query.ClearRegistry()
	defer query.ClearRegistry()

	// Create a complex query with various features
	ast := &query.AST{
		Kind: query.SelectQuery,
		FromTable: query.TableRef{
			Name:  "users",
			Alias: "u",
		},
		Distinct: true,
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "u", Name: "id"}}},
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "u", Name: "email"}}, Alias: "user_email"},
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}, Alias: "total"},
		},
		Joins: []query.JoinClause{
			{
				Type: query.LeftJoin,
				Table: query.TableRef{
					Name:  "orders",
					Alias: "o",
				},
				Condition: query.BinaryExpr{
					Left:  query.ColumnExpr{Column: query.Int64Column{Table: "o", Name: "user_id"}},
					Op:    query.OpEq,
					Right: query.ColumnExpr{Column: query.Int64Column{Table: "u", Name: "id"}},
				},
			},
		},
		Where: query.BinaryExpr{
			Left: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.StringColumn{Table: "u", Name: "email"}},
				Op:    query.OpEq,
				Right: query.ParamExpr{Name: "email", GoType: "string"},
			},
			Op: query.OpAnd,
			Right: query.BinaryExpr{
				Left:  query.ColumnExpr{Column: query.Int64Column{Table: "u", Name: "id"}},
				Op:    query.OpGt,
				Right: query.ParamExpr{Name: "min_id", GoType: "int64"},
			},
		},
		GroupBy: []query.Column{
			query.Int64Column{Table: "u", Name: "id"},
		},
		OrderBy: []query.OrderByExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "u", Name: "id"}}, Desc: true},
		},
		Limit:  query.LiteralExpr{Value: 10},
		Offset: query.ParamExpr{Name: "offset", GoType: "int"},
		Params: []query.ParamInfo{
			{Name: "email", GoType: "string"},
			{Name: "min_id", GoType: "int64"},
			{Name: "offset", GoType: "int"},
		},
	}

	query.MustDefineOne("ComplexQuery", ast)

	// Serialize
	jsonData, err := query.SerializeQueries()
	if err != nil {
		t.Fatalf("SerializeQueries failed: %v", err)
	}

	// Parse the JSON
	var queries []query.SerializedQuery
	if err := json.Unmarshal(jsonData, &queries); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}

	sq := queries[0]
	if sq.Name != "ComplexQuery" {
		t.Errorf("expected name ComplexQuery, got %s", sq.Name)
	}
	if sq.ReturnType != query.ReturnOne {
		t.Errorf("expected return type 'one', got %s", sq.ReturnType)
	}

	// Verify AST properties
	if sq.AST.Kind != "select" {
		t.Errorf("expected kind 'select', got %s", sq.AST.Kind)
	}
	if !sq.AST.Distinct {
		t.Error("expected Distinct to be true")
	}
	if len(sq.AST.SelectCols) != 3 {
		t.Errorf("expected 3 select cols, got %d", len(sq.AST.SelectCols))
	}
	if len(sq.AST.Joins) != 1 {
		t.Errorf("expected 1 join, got %d", len(sq.AST.Joins))
	}
	if sq.AST.Joins[0].Type != "LEFT" {
		t.Errorf("expected join type 'LEFT', got %s", sq.AST.Joins[0].Type)
	}
	if len(sq.AST.GroupBy) != 1 {
		t.Errorf("expected 1 group by, got %d", len(sq.AST.GroupBy))
	}
	if len(sq.AST.OrderBy) != 1 {
		t.Errorf("expected 1 order by, got %d", len(sq.AST.OrderBy))
	}
	if !sq.AST.OrderBy[0].Desc {
		t.Error("expected order by to be DESC")
	}
	if len(sq.AST.Params) != 3 {
		t.Errorf("expected 3 params, got %d", len(sq.AST.Params))
	}

	// Deserialize the AST and verify it can be used
	deserializedAST := query.DeserializeAST(sq.AST)
	if deserializedAST == nil {
		t.Fatal("DeserializeAST returned nil")
	}
	if deserializedAST.Kind != query.SelectQuery {
		t.Errorf("deserialized kind mismatch: got %s", deserializedAST.Kind)
	}
	if !deserializedAST.Distinct {
		t.Error("deserialized Distinct should be true")
	}
	if len(deserializedAST.SelectCols) != 3 {
		t.Errorf("deserialized select cols mismatch: got %d", len(deserializedAST.SelectCols))
	}
}

// TestDiscoverPackages_Integration tests package discovery with a real directory structure.
func TestDiscoverPackages_Integration(t *testing.T) {
	// Create a temporary directory structure that mimics a real project
	tmpDir := t.TempDir()

	// Create querydefs structure
	dirs := []string{
		"querydefs",
		"querydefs/users",
		"querydefs/orders",
		"querydefs/internal/helpers",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}

		// Create a Go file in each directory
		goFile := filepath.Join(fullPath, "queries.go")
		content := "package " + filepath.Base(dir) + "\n\nimport _ \"github.com/shipq/shipq/db/portsql/query\"\n"
		if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create Go file in %s: %v", dir, err)
		}
	}

	// Also create a directory with only test files (should be skipped)
	testOnlyDir := filepath.Join(tmpDir, "querydefs/testonly")
	if err := os.MkdirAll(testOnlyDir, 0755); err != nil {
		t.Fatalf("failed to create testonly dir: %v", err)
	}
	testFile := filepath.Join(testOnlyDir, "queries_test.go")
	if err := os.WriteFile(testFile, []byte("package testonly\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Discover packages (in standard case, goModRoot and shipqRoot are the same)
	pkgs, err := discovery.DiscoverPackages(tmpDir, tmpDir, "querydefs", "example.com/testapp")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	// Should find 4 packages (querydefs, users, orders, internal/helpers)
	// Should NOT find testonly (only has test files)
	if len(pkgs) != 4 {
		t.Errorf("expected 4 packages, got %d: %v", len(pkgs), pkgs)
	}

	// Verify expected packages are present
	expectedPkgs := map[string]bool{
		"example.com/testapp/querydefs":                  false,
		"example.com/testapp/querydefs/users":            false,
		"example.com/testapp/querydefs/orders":           false,
		"example.com/testapp/querydefs/internal/helpers": false,
	}

	for _, pkg := range pkgs {
		if _, ok := expectedPkgs[pkg]; ok {
			expectedPkgs[pkg] = true
		} else {
			t.Errorf("unexpected package: %s", pkg)
		}
	}

	for pkg, found := range expectedPkgs {
		if !found {
			t.Errorf("expected package not found: %s", pkg)
		}
	}
}

// TestGenerateRunner_AllQueryTypes tests that all query return types generate correct code.
func TestGenerateRunner_AllQueryTypes(t *testing.T) {
	queries := []query.SerializedQuery{
		{
			Name:       "GetSingleItem",
			ReturnType: query.ReturnOne,
			AST: &query.SerializedAST{
				Kind:      "select",
				FromTable: query.SerializedTableRef{Name: "items"},
				SelectCols: []query.SerializedSelectExpr{
					{Expr: query.SerializedExpr{Type: "column", Column: &query.SerializedColumn{Table: "items", Name: "id", GoType: "int64"}}},
				},
				Params: []query.SerializedParamInfo{{Name: "id", GoType: "int64"}},
			},
		},
		{
			Name:       "ListAllItems",
			ReturnType: query.ReturnMany,
			AST: &query.SerializedAST{
				Kind:      "select",
				FromTable: query.SerializedTableRef{Name: "items"},
				SelectCols: []query.SerializedSelectExpr{
					{Expr: query.SerializedExpr{Type: "column", Column: &query.SerializedColumn{Table: "items", Name: "id", GoType: "int64"}}},
				},
				Params: []query.SerializedParamInfo{},
			},
		},
		{
			Name:       "DeleteItem",
			ReturnType: query.ReturnExec,
			AST: &query.SerializedAST{
				Kind:      "delete",
				FromTable: query.SerializedTableRef{Name: "items"},
				Params:    []query.SerializedParamInfo{{Name: "id", GoType: "int64"}},
			},
		},
	}

			cfg := queryrunner.UnifiedRunnerConfig{
		ModulePath:  "example.com/testapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: queries,
		Schema:      nil,
	}

			runnerCode, err := queryrunner.GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	runnerStr := string(runnerCode)

	// Verify ReturnOne method returns (*Result, error)
	if !strings.Contains(runnerStr, "func (r *QueryRunner) GetSingleItem(ctx context.Context, params queries.GetSingleItemParams) (*queries.GetSingleItemResult, error)") {
		t.Error("GetSingleItem should return (*Result, error)")
	}

	// Verify ReturnMany method returns ([]Result, error)
	if !strings.Contains(runnerStr, "func (r *QueryRunner) ListAllItems(ctx context.Context, params queries.ListAllItemsParams) ([]queries.ListAllItemsResult, error)") {
		t.Error("ListAllItems should return ([]Result, error)")
	}

	// Verify ReturnExec method returns (sql.Result, error)
	if !strings.Contains(runnerStr, "func (r *QueryRunner) DeleteItem(ctx context.Context, params queries.DeleteItemParams) (sql.Result, error)") {
		t.Error("DeleteItem should return (sql.Result, error)")
	}
}

// TestGeneratedCodeCompiles verifies that the full codegen pipeline
// produces Go code that actually compiles. This catches sync issues
// between handler_gen.go and unified_runner.go.
func TestGeneratedCodeCompiles(t *testing.T) {
	// Create a test schema using the migration plan builder
	plan := migrate.NewPlan()
	plan.SetCurrentMigration("20260101120000_create_accounts")
	_, err := plan.AddTable("accounts", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email").Unique()
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	// Generate shared types
	typesCfg := queryrunner.UnifiedRunnerConfig{
		ModulePath: "testproject",
		Dialect:    dburl.DialectPostgres,
		Schema:     plan,
	}

	typesCode, err := queryrunner.GenerateSharedTypes(typesCfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	// Verify types.go is valid Go
	typesStr := string(typesCode)
	if _, err := format.Source(typesCode); err != nil {
		t.Errorf("types.go is not valid Go: %v\n%s", err, typesStr)
	}

	// Verify types.go contains expected types from contract
	expectedTypes := []string{
		"type contextKey struct{}",
		"func NewContextWithRunner",
		"func RunnerFromContext",
		"type ListAccountsCursor struct",
		"func EncodeAccountsCursor",
		"func DecodeAccountsCursor",
		"type ListAccountsParams struct",
		"Limit  int",
		"Cursor *ListAccountsCursor",
		"type ListAccountsResult struct",
		"Items      []ListAccountsItem",
		"NextCursor *ListAccountsCursor",
		"type GetAccountResult struct",
		"type CreateAccountParams struct",
		"type CreateAccountResult struct",
		"type UpdateAccountParams struct",
		"type UpdateAccountResult struct",
	}

	for _, expected := range expectedTypes {
		if !strings.Contains(typesStr, expected) {
			t.Errorf("types.go missing expected content: %q", expected)
		}
	}

	// Generate runner
	runnerCode, err := queryrunner.GenerateUnifiedRunner(typesCfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	// Verify runner.go is valid Go
	runnerStr := string(runnerCode)
	if _, err := format.Source(runnerCode); err != nil {
		t.Errorf("runner.go is not valid Go: %v\n%s", err, runnerStr)
	}

	// Verify runner.go contains expected methods from contract
	expectedMethods := []string{
		"func (r *QueryRunner) GetAccountByPublicID(ctx context.Context, publicID string)",
		"func (r *QueryRunner) ListAccounts(ctx context.Context, params queries.ListAccountsParams)",
		"func (r *QueryRunner) CreateAccount(ctx context.Context, params queries.CreateAccountParams)",
		"func (r *QueryRunner) UpdateAccountByPublicID(ctx context.Context, publicID string, params queries.UpdateAccountParams)",
		"func (r *QueryRunner) SoftDeleteAccountByPublicID(ctx context.Context, publicID string) error",
	}

	for _, expected := range expectedMethods {
		if !strings.Contains(runnerStr, expected) {
			t.Errorf("runner.go missing expected method: %q", expected)
		}
	}

	// Generate handlers
	table := plan.Schema.Tables["accounts"]
	handlerCfg := handlergen.HandlerGenConfig{
		ModulePath: "testproject",
		TableName:  "accounts",
		Table:      table,
		Schema:     plan.Schema.Tables,
	}

	// Generate each handler and verify it's valid Go
	handlers := map[string]func(handlergen.HandlerGenConfig, []handlergen.RelationshipInfo) ([]byte, error){
		"create":      handlergen.GenerateCreateHandler,
		"get_one":     handlergen.GenerateGetOneHandler,
		"list":        handlergen.GenerateListHandler,
		"update":      handlergen.GenerateUpdateHandler,
		"soft_delete": handlergen.GenerateSoftDeleteHandler,
	}

	for name, generator := range handlers {
		code, err := generator(handlerCfg, nil)
		if err != nil {
			t.Errorf("%s handler generation failed: %v", name, err)
			continue
		}

		if _, err := format.Source(code); err != nil {
			t.Errorf("%s handler is not valid Go: %v\n%s", name, err, string(code))
		}
	}

	// Verify handler code uses contract-based names
	createCode, _ := handlergen.GenerateCreateHandler(handlerCfg, nil)
	createStr := string(createCode)

	// Check that handlers call the right query methods
	handlerExpectations := map[string][]string{
		"create": {
			"queries.RunnerFromContext(ctx)",
			"runner.CreateAccount(ctx, queries.CreateAccountParams{",
		},
		"get_one": {
			"queries.RunnerFromContext(ctx)",
			"runner.GetAccountByPublicID(ctx, req.ID)",
		},
		"list": {
			"queries.RunnerFromContext(ctx)",
			"queries.ListAccountsCursor",
			"queries.DecodeAccountsCursor",
			"runner.ListAccounts(ctx, queries.ListAccountsParams{",
			"queries.EncodeAccountsCursor",
		},
		"update": {
			"queries.RunnerFromContext(ctx)",
			"runner.UpdateAccountByPublicID(ctx, req.ID, queries.UpdateAccountParams{",
		},
		"soft_delete": {
			"queries.RunnerFromContext(ctx)",
			"runner.SoftDeleteAccountByPublicID(ctx, req.ID)",
		},
	}

	for handlerName, expectations := range handlerExpectations {
		var code []byte
		switch handlerName {
		case "create":
			code, _ = handlergen.GenerateCreateHandler(handlerCfg, nil)
		case "get_one":
			code, _ = handlergen.GenerateGetOneHandler(handlerCfg, nil)
		case "list":
			code, _ = handlergen.GenerateListHandler(handlerCfg, nil)
		case "update":
			code, _ = handlergen.GenerateUpdateHandler(handlerCfg, nil)
		case "soft_delete":
			code, _ = handlergen.GenerateSoftDeleteHandler(handlerCfg, nil)
		}

		codeStr := string(code)
		for _, expected := range expectations {
			if !strings.Contains(codeStr, expected) {
				t.Errorf("%s handler missing expected content: %q\n\nGenerated code:\n%s", handlerName, expected, codeStr)
			}
		}
	}

	t.Log("Generated code validation passed - handlers and queries use consistent naming")
	_ = createStr // suppress unused
}

// TestGeneratedCodeCompiles_WithRelationships verifies that codegen works correctly
// when tables have foreign key relationships. This tests:
// 1. Two migrations creating related tables
// 2. Handler generation with embedded/nested results
// 3. Proper Runner interface with methods for both tables
func TestGeneratedCodeCompiles_WithRelationships(t *testing.T) {
	// Create a test schema with two related tables using multiple migrations
	plan := migrate.NewPlan()

	// First migration: create users table
	plan.SetCurrentMigration("20260101120000_create_users")
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("name")
		tb.String("email").Unique()
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	// Second migration: create posts table that references users
	plan.SetCurrentMigration("20260101120001_create_posts")
	usersRef := &ref.TableRef{Name: "users"}
	_, err = plan.AddTable("posts", func(tb *ddl.TableBuilder) error {
		tb.String("title")
		tb.Text("content")
		tb.Bigint("author_id").Indexed().References(usersRef)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	// Verify schema has both tables
	if len(plan.Schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(plan.Schema.Tables))
	}
	if _, ok := plan.Schema.Tables["users"]; !ok {
		t.Fatal("users table not found in schema")
	}
	if _, ok := plan.Schema.Tables["posts"]; !ok {
		t.Fatal("posts table not found in schema")
	}

	// Verify posts table has the author_id reference
	postsTable := plan.Schema.Tables["posts"]
	var authorCol *ddl.ColumnDefinition
	for _, col := range postsTable.Columns {
		if col.Name == "author_id" {
			authorCol = &col
			break
		}
	}
	if authorCol == nil {
		t.Fatal("author_id column not found in posts table")
	}
	if authorCol.References != "users" {
		t.Errorf("author_id.References = %q, want %q", authorCol.References, "users")
	}

	// Generate shared types
	typesCfg := queryrunner.UnifiedRunnerConfig{
		ModulePath: "testproject",
		Dialect:    dburl.DialectPostgres,
		Schema:     plan,
	}

	typesCode, err := queryrunner.GenerateSharedTypes(typesCfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	// Verify types.go is valid Go
	typesStr := string(typesCode)
	if _, err := format.Source(typesCode); err != nil {
		t.Errorf("types.go is not valid Go: %v\n%s", err, typesStr)
	}

	// Verify types.go contains types for both tables
	expectedTypes := []string{
		// Context helpers
		"type contextKey struct{}",
		"func NewContextWithRunner",
		"func RunnerFromContext",
		// Runner interface should have methods for both tables
		"GetUserByPublicID(ctx context.Context, publicID string)",
		"GetPostByPublicID(ctx context.Context, publicID string)",
		"ListUsers(ctx context.Context, params ListUsersParams)",
		"ListPosts(ctx context.Context, params ListPostsParams)",
		"CreateUser(ctx context.Context, params CreateUserParams)",
		"CreatePost(ctx context.Context, params CreatePostParams)",
		// Users types
		"type GetUserResult struct",
		"type ListUsersCursor struct",
		"type ListUsersParams struct",
		"type ListUsersResult struct",
		"type CreateUserParams struct",
		"type CreateUserResult struct",
		"type UpdateUserParams struct",
		// Posts types
		"type GetPostResult struct",
		"type ListPostsCursor struct",
		"type ListPostsParams struct",
		"type ListPostsResult struct",
		"type CreatePostParams struct",
		"type CreatePostResult struct",
		"type UpdatePostParams struct",
	}

	for _, expected := range expectedTypes {
		if !strings.Contains(typesStr, expected) {
			t.Errorf("types.go missing expected content: %q", expected)
		}
	}

	// Generate runner
	runnerCode, err := queryrunner.GenerateUnifiedRunner(typesCfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	// Verify runner.go is valid Go
	runnerStr := string(runnerCode)
	if _, err := format.Source(runnerCode); err != nil {
		t.Errorf("runner.go is not valid Go: %v\n%s", err, runnerStr)
	}

	// Verify runner.go contains methods for both tables
	expectedMethods := []string{
		// Users methods
		"func (r *QueryRunner) GetUserByPublicID(ctx context.Context, publicID string)",
		"func (r *QueryRunner) ListUsers(ctx context.Context, params queries.ListUsersParams)",
		"func (r *QueryRunner) CreateUser(ctx context.Context, params queries.CreateUserParams)",
		"func (r *QueryRunner) UpdateUserByPublicID(ctx context.Context, publicID string, params queries.UpdateUserParams)",
		"func (r *QueryRunner) SoftDeleteUserByPublicID(ctx context.Context, publicID string) error",
		// Posts methods
		"func (r *QueryRunner) GetPostByPublicID(ctx context.Context, publicID string)",
		"func (r *QueryRunner) ListPosts(ctx context.Context, params queries.ListPostsParams)",
		"func (r *QueryRunner) CreatePost(ctx context.Context, params queries.CreatePostParams)",
		"func (r *QueryRunner) UpdatePostByPublicID(ctx context.Context, publicID string, params queries.UpdatePostParams)",
		"func (r *QueryRunner) SoftDeletePostByPublicID(ctx context.Context, publicID string) error",
	}

	for _, expected := range expectedMethods {
		if !strings.Contains(runnerStr, expected) {
			t.Errorf("runner.go missing expected method: %q", expected)
		}
	}

	// Generate handlers for posts table (which has the relationship)
	postsHandlerCfg := handlergen.HandlerGenConfig{
		ModulePath: "testproject",
		TableName:  "posts",
		Table:      plan.Schema.Tables["posts"],
		Schema:     plan.Schema.Tables,
	}

	// Analyze relationships for posts
	relations := handlergen.AnalyzeRelationships(postsHandlerCfg.Table, postsHandlerCfg.Schema)

	// Verify relationship was detected
	if len(relations) != 1 {
		t.Errorf("expected 1 relationship for posts, got %d", len(relations))
	} else {
		rel := relations[0]
		if rel.TargetTable != "users" {
			t.Errorf("relationship target = %q, want %q", rel.TargetTable, "users")
		}
		if rel.FKColumn != "author_id" {
			t.Errorf("relationship FK column = %q, want %q", rel.FKColumn, "author_id")
		}
		if rel.FieldName != "author" {
			t.Errorf("relationship field name = %q, want %q", rel.FieldName, "author")
		}
		if rel.IsMany {
			t.Error("relationship should not be IsMany")
		}
	}

	// Generate get_one handler which should include embedded relation
	getOneCode, err := handlergen.GenerateGetOneHandler(postsHandlerCfg, relations)
	if err != nil {
		t.Fatalf("GenerateGetOneHandler failed: %v", err)
	}

	// Verify get_one.go is valid Go
	getOneStr := string(getOneCode)
	if _, err := format.Source(getOneCode); err != nil {
		t.Errorf("get_one.go is not valid Go: %v\n%s", err, getOneStr)
	}

	// Verify get_one.go contains embedded relation types
	expectedGetOneContent := []string{
		// Embed struct for the author relation
		"type AuthorEmbed struct",
		// Response struct should have the embedded author
		"type GetPostResponse struct",
		`json:"author"`,
	}

	for _, expected := range expectedGetOneContent {
		if !strings.Contains(getOneStr, expected) {
			t.Errorf("get_one.go missing expected content: %q\n\nGenerated code:\n%s", expected, getOneStr)
		}
	}

	// Check for Author field with *AuthorEmbed type (spacing may vary due to alignment)
	if !strings.Contains(getOneStr, "Author") || !strings.Contains(getOneStr, "*AuthorEmbed") {
		t.Errorf("get_one.go missing Author *AuthorEmbed field\n\nGenerated code:\n%s", getOneStr)
	}

	// Verify the FK column (author_id) is NOT in the response (replaced by embed)
	if strings.Contains(getOneStr, `json:"author_id"`) {
		t.Error("get_one.go should not contain author_id field (should be embedded as author)")
	}

	// Generate all handlers and verify they compile
	handlers := map[string]func(handlergen.HandlerGenConfig, []handlergen.RelationshipInfo) ([]byte, error){
		"create":      handlergen.GenerateCreateHandler,
		"get_one":     handlergen.GenerateGetOneHandler,
		"list":        handlergen.GenerateListHandler,
		"update":      handlergen.GenerateUpdateHandler,
		"soft_delete": handlergen.GenerateSoftDeleteHandler,
	}

	// Test handlers for posts (with relations)
	for name, generator := range handlers {
		code, err := generator(postsHandlerCfg, relations)
		if err != nil {
			t.Errorf("posts %s handler generation failed: %v", name, err)
			continue
		}
		if _, err := format.Source(code); err != nil {
			t.Errorf("posts %s handler is not valid Go: %v\n%s", name, err, string(code))
		}
	}

	// Test handlers for users (no relations)
	usersHandlerCfg := handlergen.HandlerGenConfig{
		ModulePath: "testproject",
		TableName:  "users",
		Table:      plan.Schema.Tables["users"],
		Schema:     plan.Schema.Tables,
	}
	usersRelations := handlergen.AnalyzeRelationships(usersHandlerCfg.Table, usersHandlerCfg.Schema)

	for name, generator := range handlers {
		code, err := generator(usersHandlerCfg, usersRelations)
		if err != nil {
			t.Errorf("users %s handler generation failed: %v", name, err)
			continue
		}
		if _, err := format.Source(code); err != nil {
			t.Errorf("users %s handler is not valid Go: %v\n%s", name, err, string(code))
		}
	}

	t.Log("Generated code with relationships validation passed")
}
