package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/db/portsql/query"
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
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/testapp",
				Dialect:     dialect,
				UserQueries: userQueries,
				Schema:      plan,
			}

			// Generate shared types
			typesCode, err := GenerateSharedTypes(cfg)
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
			runnerCode, err := GenerateUnifiedRunner(cfg)
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

	// Discover packages
	pkgs, err := DiscoverPackages(tmpDir, "querydefs", "example.com/testapp")
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

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/testapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: queries,
		Schema:      nil,
	}

	runnerCode, err := GenerateUnifiedRunner(cfg)
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
