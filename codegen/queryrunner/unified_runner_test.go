package queryrunner

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/dburl"
)

func TestGenerateUnifiedRunner_Empty(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
		Schema:      nil,
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have package declaration
	if !strings.Contains(codeStr, "package postgres") {
		t.Error("expected 'package postgres' in generated code")
	}

	// Should have Querier interface
	if !strings.Contains(codeStr, "type Querier interface") {
		t.Error("expected Querier interface in generated code")
	}

	// Should have QueryRunner struct
	if !strings.Contains(codeStr, "type QueryRunner struct") {
		t.Error("expected QueryRunner struct in generated code")
	}

	// Should have NewQueryRunner function
	if !strings.Contains(codeStr, "func NewQueryRunner(db Querier) *QueryRunner") {
		t.Error("expected NewQueryRunner function in generated code")
	}

	// Should have WithTx method
	if !strings.Contains(codeStr, "func (r *QueryRunner) WithTx(tx *sql.Tx) *QueryRunner") {
		t.Error("expected WithTx method in generated code")
	}

	// Should have WithDB method
	if !strings.Contains(codeStr, "func (r *QueryRunner) WithDB(db Querier) *QueryRunner") {
		t.Error("expected WithDB method in generated code")
	}
}

func TestGenerateUnifiedRunner_WithUserQueries(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath: "example.com/myapp",
		Dialect:    dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{
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
		},
		Schema: nil,
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have SQL field for the query
	if !strings.Contains(codeStr, "getUserByEmailSQL string") {
		t.Error("expected getUserByEmailSQL field in QueryRunner struct")
	}

	// Should have method for the query
	if !strings.Contains(codeStr, "func (r *QueryRunner) GetUserByEmail(ctx context.Context") {
		t.Error("expected GetUserByEmail method in generated code")
	}

	// Should use queries package for types
	if !strings.Contains(codeStr, "queries.GetUserByEmailParams") {
		t.Error("expected queries.GetUserByEmailParams in generated code")
	}

	if !strings.Contains(codeStr, "queries.GetUserByEmailResult") {
		t.Error("expected queries.GetUserByEmailResult in generated code")
	}
}

func TestGenerateUnifiedRunner_WithSchema(t *testing.T) {
	plan := migrate.NewPlan()
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email")
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("failed to add table: %v", err)
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
		Schema:      plan,
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have CRUD SQL fields (go fmt uses tabs, so check without leading whitespace)
	if !strings.Contains(codeStr, "getUserSQL") || !strings.Contains(codeStr, "string") {
		t.Error("expected getUserSQL field")
	}
	if !strings.Contains(codeStr, "listUsersSQL") {
		t.Error("expected listUsersSQL field")
	}
	if !strings.Contains(codeStr, "createUserSQL") {
		t.Error("expected createUserSQL field")
	}
	if !strings.Contains(codeStr, "updateUserSQL") {
		t.Error("expected updateUserSQL field")
	}
	if !strings.Contains(codeStr, "deleteUserSQL") {
		t.Error("expected deleteUserSQL field")
	}

	// Should have CRUD methods with contract-based naming
	if !strings.Contains(codeStr, "func (r *QueryRunner) GetUserByPublicID(ctx context.Context") {
		t.Error("expected GetUserByPublicID method")
	}
	if !strings.Contains(codeStr, "func (r *QueryRunner) ListUsers(ctx context.Context") {
		t.Error("expected ListUsers method")
	}
	if !strings.Contains(codeStr, "func (r *QueryRunner) CreateUser(ctx context.Context") {
		t.Error("expected CreateUser method")
	}
	if !strings.Contains(codeStr, "func (r *QueryRunner) UpdateUserByPublicID(ctx context.Context") {
		t.Error("expected UpdateUserByPublicID method")
	}
	if !strings.Contains(codeStr, "func (r *QueryRunner) SoftDeleteUserByPublicID(ctx context.Context") {
		t.Error("expected SoftDeleteUserByPublicID method")
	}
}

func TestGenerateSharedTypes_Empty(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
		Schema:      nil,
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have package declaration
	if !strings.Contains(codeStr, "package queries") {
		t.Error("expected 'package queries' in generated code")
	}
}

func TestGenerateSharedTypes_WithUserQueries(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath: "example.com/myapp",
		Dialect:    dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{
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
					Params: []query.SerializedParamInfo{
						{Name: "email", GoType: "string"},
					},
				},
			},
		},
		Schema: nil,
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have params struct
	if !strings.Contains(codeStr, "type GetUserByEmailParams struct") {
		t.Error("expected GetUserByEmailParams struct")
	}

	// Should have result struct
	if !strings.Contains(codeStr, "type GetUserByEmailResult struct") {
		t.Error("expected GetUserByEmailResult struct")
	}

	// Should have Email field in params (go fmt uses tabs between field name and type)
	if !strings.Contains(codeStr, "Email") || !strings.Contains(codeStr, "string") {
		t.Error("expected Email field in params struct")
	}

	// Should have Id field in result (go fmt uses tabs between field name and type)
	if !strings.Contains(codeStr, "Id") || !strings.Contains(codeStr, "int64") {
		t.Error("expected Id field in result struct")
	}
}

func TestGenerateSharedTypes_WithSchema(t *testing.T) {
	plan := migrate.NewPlan()
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("email")
		tb.String("name")
		return nil
	})
	if err != nil {
		t.Fatalf("failed to add table: %v", err)
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
		Schema:      plan,
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have CRUD types
	// Check for CRUD types with contract-based naming
	// No GetUserParams - Get takes publicID directly
	if !strings.Contains(codeStr, "type GetUserResult struct") {
		t.Error("expected GetUserResult struct")
	}
	if !strings.Contains(codeStr, "type ListUsersParams struct") {
		t.Error("expected ListUsersParams struct")
	}
	if !strings.Contains(codeStr, "type ListUsersItem struct") {
		t.Error("expected ListUsersItem struct")
	}
	if !strings.Contains(codeStr, "type ListUsersResult struct") {
		t.Error("expected ListUsersResult struct")
	}
	if !strings.Contains(codeStr, "type ListUsersCursor struct") {
		t.Error("expected ListUsersCursor struct")
	}
	if !strings.Contains(codeStr, "type CreateUserParams struct") {
		t.Error("expected CreateUserParams struct")
	}
	if !strings.Contains(codeStr, "type CreateUserResult struct") {
		t.Error("expected CreateUserResult struct")
	}
	if !strings.Contains(codeStr, "type UpdateUserParams struct") {
		t.Error("expected UpdateUserParams struct")
	}
	if !strings.Contains(codeStr, "type UpdateUserResult struct") {
		t.Error("expected UpdateUserResult struct")
	}
	// No DeleteUserParams - SoftDelete takes publicID directly
}

func TestGenerateUnifiedRunner_MySQLDialect(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: nil,
		Schema:      nil,
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have mysql package
	if !strings.Contains(codeStr, "package mysql") {
		t.Error("expected 'package mysql' in generated code")
	}
}

func TestGenerateUnifiedRunner_SQLiteDialect(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectSQLite,
		UserQueries: nil,
		Schema:      nil,
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have sqlite package
	if !strings.Contains(codeStr, "package sqlite") {
		t.Error("expected 'package sqlite' in generated code")
	}
}

func TestGetCompiler(t *testing.T) {
	tests := []struct {
		dialect string
		wantErr bool
	}{
		{dburl.DialectPostgres, false},
		{dburl.DialectMySQL, false},
		{dburl.DialectSQLite, false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			compiler, err := getCompiler(tt.dialect)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unknown dialect")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if compiler == nil {
					t.Error("expected non-nil compiler")
				}
			}
		})
	}
}

func TestIsStdLib(t *testing.T) {
	tests := []struct {
		pkg      string
		expected bool
	}{
		{"context", true},
		{"database/sql", true},
		{"encoding/json", true},
		{"time", true},
		{"github.com/shipq/shipq/query", false},
		{"example.com/myapp/queries", false},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			result := isStdLib(tt.pkg)
			if result != tt.expected {
				t.Errorf("isStdLib(%q) = %v, want %v", tt.pkg, result, tt.expected)
			}
		})
	}
}

func TestNeedsTimeImport(t *testing.T) {
	tests := []struct {
		goType   string
		expected bool
	}{
		{"time.Time", true},
		{"*time.Time", true},
		{"string", false},
		{"int64", false},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := needsTimeImport(tt.goType)
			if result != tt.expected {
				t.Errorf("needsTimeImport(%q) = %v, want %v", tt.goType, result, tt.expected)
			}
		})
	}
}

func TestNeedsJSONImport(t *testing.T) {
	tests := []struct {
		goType   string
		expected bool
	}{
		{"json.RawMessage", true},
		{"string", false},
		{"int64", false},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := needsJSONImport(tt.goType)
			if result != tt.expected {
				t.Errorf("needsJSONImport(%q) = %v, want %v", tt.goType, result, tt.expected)
			}
		})
	}
}
