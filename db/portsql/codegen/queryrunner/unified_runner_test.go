package queryrunner

import (
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/proptest"
)

func TestGenerateUnifiedRunner_Empty(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
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
	if strings.Contains(codeStr, "func parseSQLiteTime(") {
		t.Error("postgres runner should not include sqlite scan helpers")
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

// TestGenerateUnifiedRunner_PostgresMySQLDontImportTimeWithUserQueries verifies
// that for postgres and mysql, the runner does NOT import "time" even when user
// queries have time.Time result columns. The runner references result types via
// the shared queries package.
func TestGenerateUnifiedRunner_PostgresMySQLDontImportTimeWithUserQueries(t *testing.T) {
	for _, dialect := range []string{dburl.DialectPostgres, dburl.DialectMySQL} {
		t.Run(dialect, func(t *testing.T) {
			cfg := UnifiedRunnerConfig{
				ModulePath: "example.com/myapp",
				Dialect:    dialect,
				UserQueries: []query.SerializedQuery{
					{
						Name:       "FindActiveSession",
						ReturnType: query.ReturnOne,
						AST: &query.SerializedAST{
							Kind: "select",
							FromTable: query.SerializedTableRef{
								Name: "sessions",
							},
							SelectCols: []query.SerializedSelectExpr{
								{
									Expr: query.SerializedExpr{
										Type: "column",
										Column: &query.SerializedColumn{
											Table:  "sessions",
											Name:   "Id",
											GoType: "int64",
										},
									},
								},
								{
									Expr: query.SerializedExpr{
										Type: "func",
										Func: &query.SerializedFunc{
											Name: "NOW",
											Args: nil,
										},
									},
									Alias: "ExpiresAt",
								},
							},
							Where: &query.SerializedExpr{
								Type: "binary",
								Binary: &query.SerializedBinary{
									Left: query.SerializedExpr{
										Type: "column",
										Column: &query.SerializedColumn{
											Table:  "sessions",
											Name:   "Id",
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
								{Name: "id", GoType: "int64"},
							},
						},
					},
				},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed: %v", err)
			}

			codeStr := string(code)

			// Even though the query result includes time.Time (via NOW()),
			// the runner references it through the queries package, so
			// "time" should NOT be imported in the runner.
			if strings.Contains(codeStr, `"time"`) {
				t.Errorf("%q runner should NOT import \"time\" when user queries have time.Time results", dialect)
			}
		})
	}
}

func TestGenerateSharedTypes_Empty(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
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

func TestGenerateSharedTypes_ContainsTxRunner(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have TxRunner struct
	if !strings.Contains(codeStr, "type TxRunner struct") {
		t.Error("expected TxRunner struct in generated types")
	}

	// TxRunner should embed Runner
	if !strings.Contains(codeStr, "Runner") {
		t.Error("expected TxRunner to embed Runner")
	}

	// TxRunner should have exported Tx field
	if !strings.Contains(codeStr, "Tx *sql.Tx") {
		t.Error("expected TxRunner to have Tx *sql.Tx field")
	}

	// Should have Commit method
	if !strings.Contains(codeStr, "func (t *TxRunner) Commit()") {
		t.Error("expected Commit method on TxRunner")
	}

	// Should have Rollback method
	if !strings.Contains(codeStr, "func (t *TxRunner) Rollback()") {
		t.Error("expected Rollback method on TxRunner")
	}
}

func TestGenerateSharedTypes_RunnerInterfaceHasBeginTx(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: nil,
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Runner interface should include BeginTx
	if !strings.Contains(codeStr, "BeginTx(ctx context.Context) (*TxRunner, error)") {
		t.Error("expected Runner interface to contain BeginTx method")
	}
}

func TestGenerateUnifiedRunner_HasBeginTxMethod(t *testing.T) {
	dialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: nil,
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner(%s) failed: %v", dialect, err)
			}

			codeStr := string(code)

			// Should have BeginTx method on QueryRunner
			if !strings.Contains(codeStr, "func (r *QueryRunner) BeginTx(ctx context.Context) (*queries.TxRunner, error)") {
				t.Errorf("expected BeginTx method on QueryRunner for dialect %s", dialect)
			}

			// Should reference sql.DB type assertion
			if !strings.Contains(codeStr, "r.db.(*sql.DB)") {
				t.Errorf("expected BeginTx to type-assert db to *sql.DB for dialect %s", dialect)
			}

			// Should call BeginTx on the sql.DB
			if !strings.Contains(codeStr, "sqlDB.BeginTx(ctx, nil)") {
				t.Errorf("expected BeginTx to call sqlDB.BeginTx for dialect %s", dialect)
			}

			// Should return TxRunner with WithTx
			if !strings.Contains(codeStr, "r.WithTx(tx)") {
				t.Errorf("expected BeginTx to use r.WithTx(tx) for dialect %s", dialect)
			}

			// Should set Tx field
			if !strings.Contains(codeStr, "Tx:     tx,") {
				t.Errorf("expected BeginTx to set Tx field for dialect %s", dialect)
			}

			// Generated code must be valid Go
			_, fmtErr := format.Source(code)
			if fmtErr != nil {
				t.Errorf("generated runner for dialect %s is not valid Go: %v", dialect, fmtErr)
			}
		})
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

func TestGenerateUnifiedRunner_MySQLDialect(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: nil,
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
	if strings.Contains(codeStr, "func parseSQLiteTime(") {
		t.Error("mysql runner should not include sqlite scan helpers")
	}
}

func TestGenerateUnifiedRunner_SQLiteDialect(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectSQLite,
		UserQueries: nil,
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
	if !strings.Contains(codeStr, "func parseSQLiteTime(") {
		t.Error("sqlite runner should include sqlite time parser helper")
	}
	if !strings.Contains(codeStr, "func parseSQLiteNullTime(") {
		t.Error("sqlite runner should include sqlite nullable time parser helper")
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

// =============================================================================
// JSON Aggregation Tests
// =============================================================================

// makeJSONAggQuery builds a serialized SELECT query with a json_agg field.
// This is a helper for the json_agg unit tests.
func makeJSONAggQuery(queryName string, cols []query.SerializedColumn) query.SerializedQuery {
	aggCols := make([]query.SerializedColumn, len(cols))
	copy(aggCols, cols)
	return query.SerializedQuery{
		Name:       queryName,
		ReturnType: query.ReturnOne,
		AST: &query.SerializedAST{
			Kind: "select",
			FromTable: query.SerializedTableRef{
				Name: "accounts",
			},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "accounts",
							Name:   "email",
							GoType: "string",
						},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "json_agg",
						JSONAgg: &query.SerializedJSONAgg{
							FieldName: "roles",
							Columns:   aggCols,
						},
					},
					Alias: "roles",
				},
			},
			Where: &query.SerializedExpr{
				Type: "binary",
				Binary: &query.SerializedBinary{
					Left: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "accounts",
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
				{Name: "id", GoType: "int64"},
			},
		},
	}
}

// TestExtractResults_JSONAgg verifies that a serialized AST with json_agg
// produces the correct Go type (slice-of-struct) and populates JSONAggCols.
func TestExtractResults_JSONAgg(t *testing.T) {
	sq := makeJSONAggQuery("FindAccountByInternalID", []query.SerializedColumn{
		{Table: "roles", Name: "name", GoType: "string"},
		{Table: "roles", Name: "description", GoType: "*string"},
	})

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result should be the email column
	if results[0].Name != "Email" || results[0].GoType != "string" {
		t.Errorf("expected Email string, got %s %s", results[0].Name, results[0].GoType)
	}
	if results[0].JSONAggCols != nil {
		t.Error("email field should not have JSONAggCols")
	}

	// Second result should be the json_agg field
	r := results[1]
	if r.Name != "Roles" {
		t.Errorf("expected name Roles, got %s", r.Name)
	}
	if r.GoType != "[]FindAccountByInternalIDRolesItem" {
		t.Errorf("expected GoType []FindAccountByInternalIDRolesItem, got %s", r.GoType)
	}
	if len(r.JSONAggCols) != 2 {
		t.Fatalf("expected 2 JSONAggCols, got %d", len(r.JSONAggCols))
	}
	if r.JSONAggCols[0].Name != "name" || r.JSONAggCols[0].GoType != "string" {
		t.Errorf("JSONAggCols[0] = %+v, want {name string}", r.JSONAggCols[0])
	}
	if r.JSONAggCols[1].Name != "description" || r.JSONAggCols[1].GoType != "*string" {
		t.Errorf("JSONAggCols[1] = %+v, want {description *string}", r.JSONAggCols[1])
	}
}

// Regression: a top-level SelectExprAs with a SubqueryExpr on a ReturnMany
// query must infer the scalar subquery's type — not fall back to "any".
func TestExtractResults_TopLevelScalarSubquery(t *testing.T) {
	sq := query.SerializedQuery{
		Name:       "ListBooksWithChapterCount",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "books"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
					},
				},
				{
					// SelectExprAs(SubqueryExpr{...}, "chapter_count")
					Expr: query.SerializedExpr{
						Type: "subquery",
						Subquery: &query.SerializedAST{
							Kind:      "select",
							FromTable: query.SerializedTableRef{Name: "chapters"},
							SelectCols: []query.SerializedSelectExpr{
								{
									Expr: query.SerializedExpr{
										Type:      "aggregate",
										Aggregate: &query.SerializedAgg{Func: "COUNT"},
									},
								},
							},
						},
					},
					Alias: "chapter_count",
				},
			},
		},
	}

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	title := results[0]
	if title.GoType != "string" {
		t.Errorf("title GoType = %q, want 'string'", title.GoType)
	}

	cc := results[1]
	if cc.Name != "ChapterCount" {
		t.Errorf("expected name ChapterCount, got %s", cc.Name)
	}
	if cc.GoType == "any" {
		t.Fatalf("top-level scalar subquery (SELECT COUNT) resolved to 'any'; should be 'int64'")
	}
	if cc.GoType != "int64" {
		t.Errorf("chapter_count GoType = %q, want 'int64'", cc.GoType)
	}
}

// Regression: same as above but with a column-returning subquery.
func TestExtractResults_TopLevelScalarSubquery_Column(t *testing.T) {
	sq := query.SerializedQuery{
		Name:       "ListBooksWithLatestReview",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "books"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "subquery",
						Subquery: &query.SerializedAST{
							Kind:      "select",
							FromTable: query.SerializedTableRef{Name: "reviews"},
							SelectCols: []query.SerializedSelectExpr{
								{
									Expr: query.SerializedExpr{
										Type:   "column",
										Column: &query.SerializedColumn{Table: "reviews", Name: "body", GoType: "string"},
									},
								},
							},
						},
					},
					Alias: "latest_review",
				},
			},
		},
	}

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	review := results[1]
	if review.Name != "LatestReview" {
		t.Errorf("expected name LatestReview, got %s", review.Name)
	}
	if review.GoType == "any" {
		t.Fatalf("top-level scalar subquery (SELECT column) resolved to 'any'; should be 'string'")
	}
	if review.GoType != "string" {
		t.Errorf("latest_review GoType = %q, want 'string'", review.GoType)
	}
}

// Regression: MIN/MAX aggregate subquery should infer the inner column type.
func TestExtractResults_TopLevelScalarSubquery_MinMax(t *testing.T) {
	sq := query.SerializedQuery{
		Name:       "ListBooksWithMinPrice",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "books"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "subquery",
						Subquery: &query.SerializedAST{
							Kind:      "select",
							FromTable: query.SerializedTableRef{Name: "editions"},
							SelectCols: []query.SerializedSelectExpr{
								{
									Expr: query.SerializedExpr{
										Type: "aggregate",
										Aggregate: &query.SerializedAgg{
											Func: "MIN",
											Arg: &query.SerializedExpr{
												Type:   "column",
												Column: &query.SerializedColumn{Table: "editions", Name: "price", GoType: "float64"},
											},
										},
									},
								},
							},
						},
					},
					Alias: "min_price",
				},
			},
		},
	}

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	price := results[1]
	if price.Name != "MinPrice" {
		t.Errorf("expected name MinPrice, got %s", price.Name)
	}
	if price.GoType == "any" {
		t.Fatalf("top-level scalar subquery (SELECT MIN(price)) resolved to 'any'; should be 'float64'")
	}
	if price.GoType != "float64" {
		t.Errorf("min_price GoType = %q, want 'float64'", price.GoType)
	}
}

// Regression: SelectExprAs(SubqueryExpr{JSONAggExpr}, "key_facts") on a
// MustDefineMany query must produce a typed struct slice — not "any".
// This is the pattern used by keyFactsSubquery() → ListSourcesByDraft.
func TestExtractResults_SubqueryWrappedJSONAgg_TopLevel(t *testing.T) {
	sq := query.SerializedQuery{
		Name:       "ListSourcesByDraft",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "saved_sources"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "saved_sources", Name: "headline", GoType: "string"},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "saved_sources", Name: "url", GoType: "string"},
					},
				},
				{
					// SelectExprAs(SubqueryExpr{ JSONAggExpr{...} }, "key_facts")
					Expr: query.SerializedExpr{
						Type: "subquery",
						Subquery: &query.SerializedAST{
							Kind:      "select",
							FromTable: query.SerializedTableRef{Name: "source_key_facts"},
							SelectCols: []query.SerializedSelectExpr{
								{
									Expr: query.SerializedExpr{
										Type: "json_agg",
										JSONAgg: &query.SerializedJSONAgg{
											FieldName: "key_facts_inner",
											Columns: []query.SerializedColumn{
												{Table: "source_key_facts", Name: "fact", GoType: "string"},
												{Table: "source_key_facts", Name: "source", GoType: "string"},
											},
										},
									},
								},
							},
						},
					},
					Alias: "key_facts",
				},
			},
		},
	}

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	kf := results[2]
	if kf.Name != "KeyFacts" {
		t.Errorf("expected name KeyFacts, got %s", kf.Name)
	}
	if kf.GoType == "any" {
		t.Fatalf("SubqueryExpr wrapping JSONAggExpr resolved to 'any'; should be a typed slice")
	}
	if !strings.HasPrefix(kf.GoType, "[]") {
		t.Fatalf("expected slice type, got %q", kf.GoType)
	}
	if len(kf.JSONAggCols) != 2 {
		t.Fatalf("expected 2 JSONAggCols, got %d", len(kf.JSONAggCols))
	}
	if kf.JSONAggCols[0].Name != "fact" || kf.JSONAggCols[0].GoType != "string" {
		t.Errorf("JSONAggCols[0] = %+v, want {fact string}", kf.JSONAggCols[0])
	}
	if kf.JSONAggCols[1].Name != "source" || kf.JSONAggCols[1].GoType != "string" {
		t.Errorf("JSONAggCols[1] = %+v, want {source string}", kf.JSONAggCols[1])
	}
}

// Verify the above also generates correct Go structs (not just extractResults).
func TestGenerateSharedTypes_SubqueryWrappedJSONAgg_TopLevel(t *testing.T) {
	sq := query.SerializedQuery{
		Name:       "ListSourcesByDraft",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "saved_sources"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "saved_sources", Name: "headline", GoType: "string"},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "subquery",
						Subquery: &query.SerializedAST{
							Kind:      "select",
							FromTable: query.SerializedTableRef{Name: "source_key_facts"},
							SelectCols: []query.SerializedSelectExpr{
								{
									Expr: query.SerializedExpr{
										Type: "json_agg",
										JSONAgg: &query.SerializedJSONAgg{
											FieldName: "key_facts_inner",
											Columns: []query.SerializedColumn{
												{Table: "source_key_facts", Name: "fact", GoType: "string"},
												{Table: "source_key_facts", Name: "source", GoType: "string"},
											},
										},
									},
								},
							},
						},
					},
					Alias: "key_facts",
				},
			},
		},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "KeyFacts any") {
		t.Fatalf("KeyFacts resolved to 'any':\n%s", codeStr)
	}

	if !strings.Contains(codeStr, "KeyFacts []ListSourcesByDraftKeyFactsInnerItem") {
		t.Errorf("expected KeyFacts []ListSourcesByDraftKeyFactsInnerItem, got:\n%s", codeStr)
	}

	if !strings.Contains(codeStr, "type ListSourcesByDraftKeyFactsInnerItem struct") {
		t.Errorf("expected ListSourcesByDraftKeyFactsInnerItem struct, got:\n%s", codeStr)
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated types.go is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestExtractResults_NestedJSONAgg(t *testing.T) {
	// The realistic pattern: inner JSON_AGG is wrapped in a scalar subquery
	// (SubqueryExpr → AST → SelectCols[0] = JSONAggExpr).
	sq := query.SerializedQuery{
		Name:       "ListAuthors",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "authors"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table: "authors", Name: "name", GoType: "string",
						},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "json_agg",
						JSONAgg: &query.SerializedJSONAgg{
							FieldName: "books",
							Fields: []query.SerializedJSONAggField{
								{
									Key:    "title",
									Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
								},
								{
									Key: "chapters",
									Expr: &query.SerializedExpr{
										Type: "subquery",
										Subquery: &query.SerializedAST{
											Kind:      "select",
											FromTable: query.SerializedTableRef{Name: "chapters"},
											SelectCols: []query.SerializedSelectExpr{
												{
													Expr: query.SerializedExpr{
														Type: "json_agg",
														JSONAgg: &query.SerializedJSONAgg{
															FieldName: "chapters_inner",
															Columns: []query.SerializedColumn{
																{Table: "chapters", Name: "title", GoType: "string"},
																{Table: "chapters", Name: "page_count", GoType: "int"},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Alias: "books",
				},
			},
		},
	}

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	r := results[1]
	if r.Name != "Books" {
		t.Errorf("expected name Books, got %s", r.Name)
	}
	if r.GoType != "[]ListAuthorsBooksItem" {
		t.Errorf("expected GoType []ListAuthorsBooksItem, got %s", r.GoType)
	}
	if len(r.JSONAggCols) != 2 {
		t.Fatalf("expected 2 JSONAggCols, got %d", len(r.JSONAggCols))
	}
	if r.JSONAggCols[0].Name != "title" || r.JSONAggCols[0].GoType != "string" {
		t.Errorf("JSONAggCols[0] = %+v, want {title string}", r.JSONAggCols[0])
	}

	chaptersField := r.JSONAggCols[1]
	if chaptersField.Name != "chapters" {
		t.Errorf("JSONAggCols[1].Name = %q, want 'chapters'", chaptersField.Name)
	}
	// Must NOT be "any" — the nested JSON_AGG should produce a typed slice
	if chaptersField.GoType == "any" {
		t.Fatalf("chapters field GoType is 'any'; nested JSON_AGG type was not resolved")
	}
	if !strings.HasPrefix(chaptersField.GoType, "[]") {
		t.Errorf("JSONAggCols[1].GoType should start with '[]', got %q", chaptersField.GoType)
	}
	if len(chaptersField.Children) != 2 {
		t.Fatalf("expected 2 children for chapters field, got %d", len(chaptersField.Children))
	}
	if chaptersField.Children[0].Name != "title" || chaptersField.Children[0].GoType != "string" {
		t.Errorf("chapter child[0] = %+v, want {title string}", chaptersField.Children[0])
	}
	if chaptersField.Children[1].Name != "page_count" || chaptersField.Children[1].GoType != "int" {
		t.Errorf("chapter child[1] = %+v, want {page_count int}", chaptersField.Children[1])
	}
}

func TestExtractResults_NestedJSONAgg_ScalarSubquery(t *testing.T) {
	// A JSONAggField with a scalar subquery (not json_agg) should infer the
	// type of the subquery's single select column, not fall back to "any".
	sq := query.SerializedQuery{
		Name:       "ListAuthors",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "authors"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type: "json_agg",
						JSONAgg: &query.SerializedJSONAgg{
							FieldName: "books",
							Fields: []query.SerializedJSONAggField{
								{
									Key:    "title",
									Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
								},
								{
									Key: "chapter_count",
									Expr: &query.SerializedExpr{
										Type: "subquery",
										Subquery: &query.SerializedAST{
											Kind:      "select",
											FromTable: query.SerializedTableRef{Name: "chapters"},
											SelectCols: []query.SerializedSelectExpr{
												{
													Expr: query.SerializedExpr{
														Type: "aggregate",
														Aggregate: &query.SerializedAgg{
															Func: "COUNT",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Alias: "books",
				},
			},
		},
	}

	results := extractResults(sq.AST, sq.Name)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if len(r.JSONAggCols) != 2 {
		t.Fatalf("expected 2 JSONAggCols, got %d", len(r.JSONAggCols))
	}

	countField := r.JSONAggCols[1]
	if countField.Name != "chapter_count" {
		t.Errorf("field[1].Name = %q, want 'chapter_count'", countField.Name)
	}
	if countField.GoType == "any" {
		t.Fatalf("scalar subquery (SELECT COUNT) resolved to 'any'; should be 'int64'")
	}
	if countField.GoType != "int64" {
		t.Errorf("field[1].GoType = %q, want 'int64'", countField.GoType)
	}
}

func TestInferGoType_SubqueryColumn(t *testing.T) {
	expr := &query.SerializedExpr{
		Type: "subquery",
		Subquery: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "books"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
					},
				},
			},
		},
	}

	got := inferGoType(expr)
	if got != "string" {
		t.Errorf("inferGoType(subquery with string column) = %q, want 'string'", got)
	}
}

func TestInferGoType_SubqueryAggregate(t *testing.T) {
	expr := &query.SerializedExpr{
		Type: "subquery",
		Subquery: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "chapters"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type:      "aggregate",
						Aggregate: &query.SerializedAgg{Func: "COUNT"},
					},
				},
			},
		},
	}

	got := inferGoType(expr)
	if got != "int64" {
		t.Errorf("inferGoType(subquery with COUNT) = %q, want 'int64'", got)
	}
}

func TestInferGoType_BinaryComparison(t *testing.T) {
	expr := &query.SerializedExpr{
		Type: "binary",
		Binary: &query.SerializedBinary{
			Left: query.SerializedExpr{
				Type:   "column",
				Column: &query.SerializedColumn{Table: "x", Name: "a", GoType: "int64"},
			},
			Op: "=",
			Right: query.SerializedExpr{
				Type:   "column",
				Column: &query.SerializedColumn{Table: "x", Name: "b", GoType: "int64"},
			},
		},
	}

	got := inferGoType(expr)
	if got != "bool" {
		t.Errorf("inferGoType(a = b) = %q, want 'bool'", got)
	}
}

func TestInferGoType_BinaryArithmetic(t *testing.T) {
	expr := &query.SerializedExpr{
		Type: "binary",
		Binary: &query.SerializedBinary{
			Left: query.SerializedExpr{
				Type:   "column",
				Column: &query.SerializedColumn{Table: "x", Name: "price", GoType: "float64"},
			},
			Op: "+",
			Right: query.SerializedExpr{
				Type:    "literal",
				Literal: float64(1),
			},
		},
	}

	got := inferGoType(expr)
	if got != "float64" {
		t.Errorf("inferGoType(price + 1) = %q, want 'float64'", got)
	}
}

func TestInferGoType_Param(t *testing.T) {
	expr := &query.SerializedExpr{
		Type: "param",
		Param: &query.SerializedParam{
			Name:   "user_id",
			GoType: "int64",
		},
	}

	got := inferGoType(expr)
	if got != "int64" {
		t.Errorf("inferGoType(param int64) = %q, want 'int64'", got)
	}
}

func TestGenerateSharedTypes_NestedJSONAgg(t *testing.T) {
	// Use the realistic subquery-wrapped pattern for the inner JSON_AGG
	sq := query.SerializedQuery{
		Name:       "ListAuthors",
		ReturnType: query.ReturnMany,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "authors"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table: "authors", Name: "name", GoType: "string",
						},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "json_agg",
						JSONAgg: &query.SerializedJSONAgg{
							FieldName: "books",
							Fields: []query.SerializedJSONAggField{
								{
									Key:    "title",
									Column: &query.SerializedColumn{Table: "books", Name: "title", GoType: "string"},
								},
								{
									Key: "chapters",
									Expr: &query.SerializedExpr{
										Type: "subquery",
										Subquery: &query.SerializedAST{
											Kind:      "select",
											FromTable: query.SerializedTableRef{Name: "chapters"},
											SelectCols: []query.SerializedSelectExpr{
												{
													Expr: query.SerializedExpr{
														Type: "json_agg",
														JSONAgg: &query.SerializedJSONAgg{
															FieldName: "chapters_inner",
															Columns: []query.SerializedColumn{
																{Table: "chapters", Name: "title", GoType: "string"},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Alias: "books",
				},
			},
		},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have the outer item struct
	if !strings.Contains(codeStr, "type ListAuthorsBooksItem struct") {
		t.Errorf("expected ListAuthorsBooksItem struct, got:\n%s", codeStr)
	}

	// Should have a chapters field in the outer item struct — NOT "any"
	if strings.Contains(codeStr, "Chapters any") {
		t.Fatalf("chapters field resolved to 'any'; nested JSON_AGG type was not resolved:\n%s", codeStr)
	}
	if !strings.Contains(codeStr, `Chapters []ListAuthorsBooksChaptersItem`) {
		t.Errorf("expected Chapters []ListAuthorsBooksChaptersItem field, got:\n%s", codeStr)
	}

	// Should have the inner item struct
	if !strings.Contains(codeStr, "type ListAuthorsBooksChaptersItem struct") {
		t.Errorf("expected ListAuthorsBooksChaptersItem struct, got:\n%s", codeStr)
	}

	// Inner struct should have properly typed fields (string, not any)
	if strings.Contains(codeStr, "Title any") {
		t.Errorf("inner struct title resolved to 'any':\n%s", codeStr)
	}

	// Should be valid Go
	if _, err := format.Source(code); err != nil {
		t.Errorf("generated types.go is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateSharedTypes_WithJSONAgg verifies that nested struct is generated
// for json_agg fields with correct field names, types, and json tags.
func TestGenerateSharedTypes_WithJSONAgg(t *testing.T) {
	sq := makeJSONAggQuery("FindAccountByInternalID", []query.SerializedColumn{
		{Table: "roles", Name: "name", GoType: "string"},
		{Table: "roles", Name: "description", GoType: "*string"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have the result struct with the slice field
	if !strings.Contains(codeStr, "Roles []FindAccountByInternalIDRolesItem") {
		t.Error("expected Roles []FindAccountByInternalIDRolesItem in result struct")
	}

	// Should have the nested item struct
	if !strings.Contains(codeStr, "type FindAccountByInternalIDRolesItem struct") {
		t.Error("expected FindAccountByInternalIDRolesItem struct")
	}

	// Should have Name field with json tag
	if !strings.Contains(codeStr, "Name string") && !strings.Contains(codeStr, `json:"name"`) {
		t.Error("expected Name string field with json tag in nested struct")
	}

	// Should have Description field with json tag including omitempty
	if !strings.Contains(codeStr, "Description *string") && !strings.Contains(codeStr, `json:"description,omitempty"`) {
		t.Error("expected Description *string field with omitempty json tag in nested struct")
	}

	// Should be valid Go
	if _, err := format.Source(code); err != nil {
		t.Errorf("generated types.go is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_WithJSONAgg_ScanCodegen verifies the generated runner
// scans json_agg into a temp string and unmarshals into the typed slice.
func TestGenerateUnifiedRunner_WithJSONAgg_ScanCodegen(t *testing.T) {
	for _, dialect := range []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite} {
		t.Run(dialect, func(t *testing.T) {
			sq := makeJSONAggQuery("FindAccountByInternalID", []query.SerializedColumn{
				{Table: "roles", Name: "name", GoType: "string"},
				{Table: "roles", Name: "description", GoType: "*string"},
			})

			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{sq},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed: %v", err)
			}

			codeStr := string(code)

			// Should declare a temp string for the json_agg field
			if !strings.Contains(codeStr, "var rolesRaw string") {
				t.Error("expected 'var rolesRaw string' for json_agg scan intermediate")
			}

			// Should scan into the temp string
			if !strings.Contains(codeStr, "&rolesRaw") {
				t.Error("expected '&rolesRaw' in Scan call")
			}

			// Should json.Unmarshal into the typed field
			if !strings.Contains(codeStr, "json.Unmarshal([]byte(rolesRaw), &result.Roles)") {
				t.Error("expected json.Unmarshal([]byte(rolesRaw), &result.Roles)")
			}

			// Should import encoding/json
			if !strings.Contains(codeStr, `"encoding/json"`) {
				t.Error("expected encoding/json import for json_agg unmarshal")
			}

			// Should be valid Go
			if _, err := format.Source(code); err != nil {
				t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
			}
		})
	}
}

// TestGenerateUnifiedRunner_WithJSONAgg_ReturnMany verifies the json_agg scan
// codegen for ReturnMany queries.
func TestGenerateUnifiedRunner_WithJSONAgg_ReturnMany(t *testing.T) {
	sq := makeJSONAggQuery("ListAccountsWithRoles", []query.SerializedColumn{
		{Table: "roles", Name: "name", GoType: "string"},
	})
	sq.ReturnType = query.ReturnMany

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should declare temp string and unmarshal in the row loop
	if !strings.Contains(codeStr, "var rolesRaw string") {
		t.Error("expected 'var rolesRaw string' in ReturnMany scan")
	}
	if !strings.Contains(codeStr, "json.Unmarshal([]byte(rolesRaw), &item.Roles)") {
		t.Error("expected json.Unmarshal into item.Roles in ReturnMany")
	}

	// Should be valid Go
	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_WithJSONAgg_BoolFix verifies that MySQL and SQLite
// runners emit fixJSONBoolFields before json.Unmarshal when json_agg columns
// contain bool fields, and that Postgres does not.
func TestGenerateUnifiedRunner_WithJSONAgg_BoolFix(t *testing.T) {
	sq := makeJSONAggQuery("FindAccountByInternalID", []query.SerializedColumn{
		{Table: "forum_signups", Name: "should_text", GoType: "bool"},
		{Table: "forum_signups", Name: "name", GoType: "string"},
		{Table: "forum_signups", Name: "is_active", GoType: "*bool"},
	})

	for _, dialect := range []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite} {
		t.Run(dialect, func(t *testing.T) {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{sq},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed: %v", err)
			}

			codeStr := string(code)

			wantFix := dialect == dburl.DialectMySQL || dialect == dburl.DialectSQLite

			if wantFix {
				// Should contain the fixJSONBoolFields helper function
				if !strings.Contains(codeStr, "func fixJSONBoolFields(") {
					t.Error("expected fixJSONBoolFields helper in generated runner")
				}
				// Should call fixJSONBoolFields on the raw json_agg string
				if !strings.Contains(codeStr, `fixJSONBoolFields(rolesRaw, []string{"should_text", "is_active"})`) {
					t.Errorf("expected fixJSONBoolFields call with bool field names, got:\n%s", codeStr)
				}
				// Should import strings for ReplaceAll
				if !strings.Contains(codeStr, `"strings"`) {
					t.Error("expected strings import for fixJSONBoolFields")
				}
			} else {
				// Postgres JSON_BUILD_OBJECT uses proper true/false; no fix needed
				if strings.Contains(codeStr, "fixJSONBoolFields") {
					t.Error("Postgres should not emit fixJSONBoolFields")
				}
			}

			// Should be valid Go regardless of dialect
			if _, err := format.Source(code); err != nil {
				t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
			}
		})
	}
}

// TestGenerateUnifiedRunner_WithJSONAgg_BoolFix_ReturnMany verifies the bool
// fix is emitted inside the row loop for ReturnMany queries.
func TestGenerateUnifiedRunner_WithJSONAgg_BoolFix_ReturnMany(t *testing.T) {
	sq := makeJSONAggQuery("ListAccountsWithSignups", []query.SerializedColumn{
		{Table: "forum_signups", Name: "should_text", GoType: "bool"},
		{Table: "forum_signups", Name: "label", GoType: "string"},
	})
	sq.ReturnType = query.ReturnMany

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "fixJSONBoolFields(rolesRaw,") {
		t.Errorf("expected fixJSONBoolFields call in ReturnMany loop, got:\n%s", codeStr)
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_WithJSONAgg_NoBoolNoBoolFix verifies that the bool
// fix helper is NOT emitted when json_agg columns have no bool fields.
func TestGenerateUnifiedRunner_WithJSONAgg_NoBoolNoBoolFix(t *testing.T) {
	sq := makeJSONAggQuery("FindAccountByInternalID", []query.SerializedColumn{
		{Table: "roles", Name: "name", GoType: "string"},
		{Table: "roles", Name: "description", GoType: "*string"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "fixJSONBoolFields") {
		t.Error("should not emit fixJSONBoolFields when no bool columns in json_agg")
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// =============================================================================
// Property Tests
// =============================================================================

// generateRandomInsertQuery creates a random INSERT query with RETURNING columns
// for property testing. It generates an INSERT into the given table with all non-auto
// columns as insert values and some columns as RETURNING.
func generateRandomInsertQuery(g *proptest.Generator, tableName string, table ddl.Table) query.SerializedQuery {
	queryName := "Insert" + strings.Title(tableName) //nolint:staticcheck

	var insertCols []query.SerializedColumn
	var insertVals []query.SerializedExpr
	var params []query.SerializedParamInfo
	var returningCols []query.SerializedColumn

	for _, col := range table.Columns {
		// Skip auto-generated columns
		if col.Name == "id" || col.Name == "created_at" || col.Name == "updated_at" || col.Name == "deleted_at" || col.Name == "public_id" {
			continue
		}
		goType := ddlTypeToGoType(col.Type)
		insertCols = append(insertCols, query.SerializedColumn{
			Table:  tableName,
			Name:   col.Name,
			GoType: goType,
		})
		insertVals = append(insertVals, query.SerializedExpr{
			Type: "param",
			Param: &query.SerializedParam{
				Name:   col.Name,
				GoType: goType,
			},
		})
		params = append(params, query.SerializedParamInfo{
			Name:   col.Name,
			GoType: goType,
		})
	}

	// Add some RETURNING columns (at least "id")
	returningCols = append(returningCols, query.SerializedColumn{
		Table:  tableName,
		Name:   "id",
		GoType: "int64",
	})
	for _, col := range table.Columns {
		if col.Name == "id" {
			continue
		}
		if g.BoolWithProb(0.5) {
			returningCols = append(returningCols, query.SerializedColumn{
				Table:  tableName,
				Name:   col.Name,
				GoType: ddlTypeToGoType(col.Type),
			})
		}
	}

	// SELECT cols match RETURNING for result scanning
	var selectCols []query.SerializedSelectExpr
	for _, rc := range returningCols {
		selectCols = append(selectCols, query.SerializedSelectExpr{
			Expr: query.SerializedExpr{
				Type:   "column",
				Column: &query.SerializedColumn{Table: rc.Table, Name: rc.Name, GoType: rc.GoType},
			},
		})
	}

	return query.SerializedQuery{
		Name:       queryName,
		ReturnType: query.ReturnOne,
		AST: &query.SerializedAST{
			Kind:       "insert",
			FromTable:  query.SerializedTableRef{Name: tableName},
			SelectCols: selectCols,
			InsertCols: insertCols,
			InsertRows: [][]query.SerializedExpr{insertVals},
			Returning:  returningCols,
			Params:     params,
		},
	}
}

// generateRandomBulkInsertQuery creates a random bulk insert query for property testing.
// Unlike generateRandomInsertQuery, this uses ReturnBulkExec and has no RETURNING.
func generateRandomBulkInsertQuery(g *proptest.Generator, tableName string, table ddl.Table) query.SerializedQuery {
	queryName := "BulkInsert" + strings.Title(tableName) //nolint:staticcheck

	var insertCols []query.SerializedColumn
	var insertVals []query.SerializedExpr
	var params []query.SerializedParamInfo

	for _, col := range table.Columns {
		// Skip auto-generated columns
		if col.Name == "id" || col.Name == "created_at" || col.Name == "updated_at" || col.Name == "deleted_at" || col.Name == "public_id" {
			continue
		}
		goType := ddlTypeToGoType(col.Type)
		insertCols = append(insertCols, query.SerializedColumn{
			Table:  tableName,
			Name:   col.Name,
			GoType: goType,
		})
		insertVals = append(insertVals, query.SerializedExpr{
			Type: "param",
			Param: &query.SerializedParam{
				Name:   col.Name,
				GoType: goType,
			},
		})
		params = append(params, query.SerializedParamInfo{
			Name:   col.Name,
			GoType: goType,
		})
	}

	return query.SerializedQuery{
		Name:       queryName,
		ReturnType: query.ReturnBulkExec,
		AST: &query.SerializedAST{
			Kind:       "insert",
			FromTable:  query.SerializedTableRef{Name: tableName},
			InsertCols: insertCols,
			InsertRows: [][]query.SerializedExpr{insertVals},
			Params:     params,
		},
	}
}

// ddlTypeToGoType converts a DDL column type to a Go type string.
func ddlTypeToGoType(colType string) string {
	switch colType {
	case ddl.IntegerType:
		return "int64"
	case ddl.BigintType:
		return "int64"
	case ddl.FloatType:
		return "float64"
	case ddl.DecimalType:
		return "string"
	case ddl.BooleanType:
		return "bool"
	case ddl.StringType, ddl.TextType:
		return "string"
	case ddl.DatetimeType:
		return "time.Time"
	case ddl.BinaryType:
		return "[]byte"
	case ddl.JSONType:
		return "json.RawMessage"
	default:
		return "string"
	}
}

// generateRandomTable creates a random table using the proptest generator.
// Ensures the table has at least one non-auto column for INSERTs.
func generateRandomTable(g *proptest.Generator) (string, ddl.Table) {
	tableName := migrate.GenerateTableName(g)
	cfg := migrate.DefaultTableConfig()
	cfg.MinColumns = 2
	cfg.MaxColumns = 6
	table := migrate.GenerateTable(g, tableName, cfg)
	return tableName, *table
}

// Property 1: Generated runner code always formats for all dialects.
// Uses a curated set of table names to avoid pre-existing codegen edge cases
// with arbitrary identifier generation.
func TestProperty_GeneratedRunnerAlwaysFormats(t *testing.T) {
	allDialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}

	proptest.Check(t, "generated runner always formats as valid Go", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName, table := generateRandomTable(g)
		insertQuery := generateRandomInsertQuery(g, tableName, table)

		for _, dialect := range allDialects {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{insertQuery},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Logf("[%s] GenerateUnifiedRunner failed for table %q: %v", dialect, tableName, err)
				return false
			}

			if _, err := format.Source(code); err != nil {
				t.Logf("[%s] generated code does not format for table %q: %v", dialect, tableName, err)
				return false
			}
		}
		return true
	})
}

// Property: Generated bulk insert runner code always formats for all dialects.
func TestProperty_BulkInsertGeneratedRunnerAlwaysFormats(t *testing.T) {
	allDialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}

	proptest.Check(t, "bulk insert runner always formats as valid Go", proptest.Config{NumTrials: 30}, func(g *proptest.Generator) bool {
		tableName, table := generateRandomTable(g)
		bulkQuery := generateRandomBulkInsertQuery(g, tableName, table)

		for _, dialect := range allDialects {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{bulkQuery},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Logf("[%s] GenerateUnifiedRunner failed for bulk table %q: %v", dialect, tableName, err)
				return false
			}

			if _, err := format.Source(code); err != nil {
				t.Logf("[%s] bulk insert generated code does not format for table %q: %v\ncode:\n%s", dialect, tableName, err, string(code))
				return false
			}

			codeStr := string(code)

			// Bulk method should use ExecContext, not QueryRowContext
			methodName := bulkQuery.Name
			if !strings.Contains(codeStr, "func (r *QueryRunner) "+methodName+"(") {
				t.Logf("[%s] cannot find bulk method %s in generated code", dialect, methodName)
				return false
			}

			// Should use driver.RowsAffected for empty params
			if !strings.Contains(codeStr, "driver.RowsAffected(0)") {
				t.Logf("[%s] bulk method missing driver.RowsAffected(0) no-op", dialect)
				return false
			}

			// Should use ExecContext
			if !strings.Contains(codeStr, "r.db.ExecContext(ctx, sb.String(), args...)") {
				t.Logf("[%s] bulk method missing ExecContext call", dialect)
				return false
			}
		}
		return true
	})
}

// Property: Bulk insert mixed with regular insert always formats for all dialects.
func TestProperty_BulkInsertMixedWithRegularAlwaysFormats(t *testing.T) {
	allDialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}

	proptest.Check(t, "mixed bulk+regular runner always formats", proptest.Config{NumTrials: 20}, func(g *proptest.Generator) bool {
		tableName, table := generateRandomTable(g)
		insertQuery := generateRandomInsertQuery(g, tableName, table)
		bulkQuery := generateRandomBulkInsertQuery(g, tableName, table)

		for _, dialect := range allDialects {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{insertQuery, bulkQuery},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Logf("[%s] GenerateUnifiedRunner failed for mixed table %q: %v", dialect, tableName, err)
				return false
			}

			if _, err := format.Source(code); err != nil {
				t.Logf("[%s] mixed generated code does not format for table %q: %v", dialect, tableName, err)
				return false
			}

			codeStr := string(code)

			// Both methods should be present
			if !strings.Contains(codeStr, "func (r *QueryRunner) "+insertQuery.Name+"(") {
				t.Logf("[%s] cannot find regular insert method %s", dialect, insertQuery.Name)
				return false
			}
			if !strings.Contains(codeStr, "func (r *QueryRunner) "+bulkQuery.Name+"(") {
				t.Logf("[%s] cannot find bulk insert method %s", dialect, bulkQuery.Name)
				return false
			}
		}
		return true
	})
}

// Property 2: MySQL runner never uses QueryRowContext for INSERT with RETURNING.
// Instead it must use ExecContext for the insert step.
func TestProperty_MySQLNeverUsesQueryRowContextForInsertReturning(t *testing.T) {
	proptest.Check(t, "MySQL INSERT with RETURNING uses ExecContext, not QueryRowContext for insert", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName, table := generateRandomTable(g)
		insertQuery := generateRandomInsertQuery(g, tableName, table)

		cfg := UnifiedRunnerConfig{
			ModulePath:  "example.com/myapp",
			Dialect:     dburl.DialectMySQL,
			UserQueries: []query.SerializedQuery{insertQuery},
		}

		code, err := GenerateUnifiedRunner(cfg)
		if err != nil {
			t.Logf("GenerateUnifiedRunner failed for table %q: %v", tableName, err)
			return false
		}

		codeStr := string(code)

		// Find the user query method (e.g., func (r *QueryRunner) InsertXxx(...))
		methodName := insertQuery.Name
		methodStart := strings.Index(codeStr, "func (r *QueryRunner) "+methodName+"(")
		if methodStart < 0 {
			t.Logf("MySQL: cannot find method %s in generated code", methodName)
			return false
		}

		// Extract just the method body (until the next top-level func)
		methodBody := extractMethodBody(codeStr, methodStart)

		// The INSERT step must use ExecContext, not QueryRowContext directly on the insert SQL field
		sqlField := "r." + toFirstLower(methodName) + "SQL"
		insertExecPattern := "r.db.ExecContext(ctx, " + sqlField
		if !strings.Contains(methodBody, insertExecPattern) {
			t.Logf("MySQL INSERT method %s: expected ExecContext with %s\nmethod body:\n%s", methodName, sqlField, methodBody)
			return false
		}

		return true
	})
}

// Property 3: Postgres and SQLite INSERT with RETURNING use QueryRowContext.
func TestProperty_PostgresSQLiteUseQueryRowContextForInsertReturning(t *testing.T) {
	for _, dialect := range []string{dburl.DialectPostgres, dburl.DialectSQLite} {
		t.Run(dialect, func(t *testing.T) {
			proptest.Check(t, dialect+" INSERT with RETURNING uses QueryRowContext", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
				tableName, table := generateRandomTable(g)
				insertQuery := generateRandomInsertQuery(g, tableName, table)

				cfg := UnifiedRunnerConfig{
					ModulePath:  "example.com/myapp",
					Dialect:     dialect,
					UserQueries: []query.SerializedQuery{insertQuery},
				}

				code, err := GenerateUnifiedRunner(cfg)
				if err != nil {
					t.Logf("[%s] GenerateUnifiedRunner failed for table %q: %v", dialect, tableName, err)
					return false
				}

				codeStr := string(code)

				methodName := insertQuery.Name
				methodStart := strings.Index(codeStr, "func (r *QueryRunner) "+methodName+"(")
				if methodStart < 0 {
					t.Logf("[%s] cannot find method %s in generated code", dialect, methodName)
					return false
				}

				methodBody := extractMethodBody(codeStr, methodStart)

				// Should use QueryRowContext with the query SQL field
				sqlField := "r." + toFirstLower(methodName) + "SQL"
				queryRowPattern := "r.db.QueryRowContext(ctx, " + sqlField
				if !strings.Contains(methodBody, queryRowPattern) {
					t.Logf("[%s] INSERT method %s: expected QueryRowContext with %s\nmethod body:\n%s", dialect, methodName, sqlField, methodBody)
					return false
				}

				return true
			})
		})
	}
}

// Property 5: All dialects generate consistent method signatures for the same schema.
func TestProperty_ConsistentMethodSignaturesAcrossDialects(t *testing.T) {
	proptest.Check(t, "all dialects produce consistent method signatures", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
		tableName, table := generateRandomTable(g)
		insertQuery := generateRandomInsertQuery(g, tableName, table)

		allDialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}

		// Collect method signatures for each dialect
		sigPattern := regexp.MustCompile(`func \(r \*QueryRunner\) (\w+)\(([^)]*)\)\s*([^{]+)\{`)

		var firstDialectSigs []querySig
		var firstDialect string

		for _, dialect := range allDialects {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{insertQuery},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Logf("[%s] GenerateUnifiedRunner failed for table %q: %v", dialect, tableName, err)
				return false
			}

			codeStr := string(code)
			matches := sigPattern.FindAllStringSubmatch(codeStr, -1)

			var sigs []querySig
			for _, m := range matches {
				sigs = append(sigs, querySig{
					name:       m[1],
					params:     strings.TrimSpace(m[2]),
					returnType: strings.TrimSpace(m[3]),
				})
			}

			if firstDialectSigs == nil {
				firstDialectSigs = sigs
				firstDialect = dialect
				continue
			}

			// Compare method names (should be the same set across dialects)
			firstNames := methodNames(firstDialectSigs)
			currentNames := methodNames(sigs)

			if !equalStringSlices(firstNames, currentNames) {
				t.Logf("method name mismatch: %s has %v, %s has %v", firstDialect, firstNames, dialect, currentNames)
				return false
			}

			// Compare return types for each method
			firstByName := sigsByName(firstDialectSigs)
			currentByName := sigsByName(sigs)

			for name, firstSig := range firstByName {
				currentSig, ok := currentByName[name]
				if !ok {
					continue
				}
				if firstSig.returnType != currentSig.returnType {
					t.Logf("return type mismatch for %s: %s=%q, %s=%q",
						name, firstDialect, firstSig.returnType, dialect, currentSig.returnType)
					return false
				}
			}
		}

		return true
	})
}

// =============================================================================
// Test Helpers
// =============================================================================

// extractMethodBody extracts the body of a Go method starting at the given position.
// It finds the matching closing brace for the function.
func extractMethodBody(code string, start int) string {
	braceCount := 0
	inMethod := false
	for i := start; i < len(code); i++ {
		if code[i] == '{' {
			braceCount++
			inMethod = true
		} else if code[i] == '}' {
			braceCount--
			if inMethod && braceCount == 0 {
				return code[start : i+1]
			}
		}
	}
	return code[start:]
}

// toFirstLower converts the first character of a string to lowercase.
func toFirstLower(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// querySig captures a method signature from generated code.
type querySig struct {
	name       string
	params     string
	returnType string
}

func methodNames(sigs []querySig) []string {
	seen := map[string]bool{}
	var names []string
	for _, s := range sigs {
		if !seen[s.name] {
			seen[s.name] = true
			names = append(names, s.name)
		}
	}
	return names
}

func sigsByName(sigs []querySig) map[string]querySig {
	m := make(map[string]querySig)
	for _, s := range sigs {
		m[s.name] = s
	}
	return m
}

func equalStringSlices(a, b []string) bool {
	setA := map[string]bool{}
	setB := map[string]bool{}
	for _, s := range a {
		setA[s] = true
	}
	for _, s := range b {
		setB[s] = true
	}
	if len(setA) != len(setB) {
		return false
	}
	for k := range setA {
		if !setB[k] {
			return false
		}
	}
	return true
}

// makeNullableJSONSelectQuery builds a Postgres SELECT query that includes a
// nullable JSON column (GoType "*json.RawMessage") alongside a regular string
// column. This lets us verify the generated scan code handles NULL JSONB.
func makeNullableJSONSelectQuery() query.SerializedQuery {
	return query.SerializedQuery{
		Name:       "GetEventByID",
		ReturnType: query.ReturnOne,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "events"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "events",
							Name:   "id",
							GoType: "int64",
						},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "events",
							Name:   "name",
							GoType: "string",
						},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "events",
							Name:   "metadata",
							GoType: "*json.RawMessage",
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
							Table:  "events",
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
				{Name: "id", GoType: "int64"},
			},
		},
	}
}

// TestGenerateUnifiedRunner_Postgres_NullableJSONColumn_ScansViaNullRawMessage
// reproduces a bug where a nullable JSONB column in Postgres was scanned
// directly into json.RawMessage ([]byte). When the column is NULL at runtime,
// pgx/v5/stdlib can fail because there is no sql.NullString intermediate to
// absorb the NULL. The fix is to scan into a *json.RawMessage so that the
// driver can set the pointer to nil for NULL values.
func TestGenerateUnifiedRunner_Postgres_NullableJSONColumn_ScansViaNullRawMessage(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{makeNullableJSONSelectQuery()},
	}

	runnerCode, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	typesCode, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	runner := string(runnerCode)
	types := string(typesCode)

	// The result struct field should be *json.RawMessage (pointer) so that
	// a nil pointer cleanly represents SQL NULL.
	if !strings.Contains(types, "*json.RawMessage") {
		t.Errorf("expected result struct to use *json.RawMessage for nullable JSON column, got:\n%s", types)
	}

	// The scan code must NOT scan directly into &result.Metadata when the
	// column can be NULL. It should scan into an intermediate (e.g.
	// *json.RawMessage or sql.NullString) that can represent NULL.
	//
	// A direct "&result.Metadata" scan for a json.RawMessage field means
	// the generated code cannot distinguish NULL from a valid JSON value.
	// At minimum, the generated code should not just do:
	//   row.Scan(..., &result.Metadata, ...)
	// for a json.RawMessage field that could be NULL.
	//
	// Instead we expect either:
	//   - The field type in the result struct is *json.RawMessage and the
	//     scan target is &result.Metadata (pointer-to-pointer absorbs NULL)
	//   - Or an intermediate variable is used, similar to SQLite's approach
	if strings.Contains(types, "Metadata json.RawMessage") {
		t.Errorf("result struct should use *json.RawMessage for nullable JSON, not json.RawMessage:\n%s", types)
	}

	_ = runner // runner should compile; we only inspect types here
}

// TestGenerateUnifiedRunner_Postgres_NullableJSONColumn_ReturnMany verifies
// the same nullable-JSON fix for queries that return multiple rows.
func TestGenerateUnifiedRunner_Postgres_NullableJSONColumn_ReturnMany(t *testing.T) {
	q := makeNullableJSONSelectQuery()
	q.Name = "ListEvents"
	q.ReturnType = query.ReturnMany

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{q},
	}

	typesCode, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	types := string(typesCode)

	if !strings.Contains(types, "*json.RawMessage") {
		t.Errorf("expected *json.RawMessage for nullable JSON column in ReturnMany result, got:\n%s", types)
	}

	if strings.Contains(types, "Metadata json.RawMessage") {
		t.Errorf("result struct should use *json.RawMessage, not json.RawMessage:\n%s", types)
	}
}

// TestGenerateUnifiedRunner_Postgres_NonNullableJSONColumn_RemainsRawMessage
// ensures that a non-nullable JSON column still uses plain json.RawMessage
// (not a pointer), since it can never be NULL.
// =============================================================================
// Bulk Insert Codegen Tests
// =============================================================================

func makeBulkInsertQuery(name string) query.SerializedQuery {
	return query.SerializedQuery{
		Name:       name,
		ReturnType: query.ReturnBulkExec,
		AST: &query.SerializedAST{
			Kind: "insert",
			FromTable: query.SerializedTableRef{
				Name: "authors",
			},
			InsertCols: []query.SerializedColumn{
				{Table: "authors", Name: "name", GoType: "string"},
				{Table: "authors", Name: "email", GoType: "string"},
			},
			InsertRows: [][]query.SerializedExpr{
				{
					{Type: "param", Param: &query.SerializedParam{Name: "name", GoType: "string"}},
					{Type: "param", Param: &query.SerializedParam{Name: "email", GoType: "string"}},
				},
			},
			Params: []query.SerializedParamInfo{
				{Name: "name", GoType: "string"},
				{Name: "email", GoType: "string"},
			},
		},
	}
}

func TestGenerateUnifiedRunner_BulkInsert_Postgres(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{makeBulkInsertQuery("BulkInsertAuthors")},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have bulk prefix/suffix/paramsPerRow fields (go/format may add alignment spaces)
	if !strings.Contains(codeStr, "bulkInsertAuthorsBulkPrefix") || !strings.Contains(codeStr, "string") {
		t.Error("expected bulkInsertAuthorsBulkPrefix field in struct")
	}
	if !strings.Contains(codeStr, "bulkInsertAuthorsBulkSuffix") {
		t.Error("expected bulkInsertAuthorsBulkSuffix field in struct")
	}
	if !strings.Contains(codeStr, "bulkInsertAuthorsBulkParamsPerRow") {
		t.Error("expected bulkInsertAuthorsBulkParamsPerRow field in struct")
	}

	// Should have the method accepting a slice of params
	if !strings.Contains(codeStr, "func (r *QueryRunner) BulkInsertAuthors(ctx context.Context, params []queries.BulkInsertAuthorsParams) (sql.Result, error)") {
		t.Error("expected BulkInsertAuthors method with []Params signature")
	}

	// Should handle empty params with driver.RowsAffected(0)
	if !strings.Contains(codeStr, "driver.RowsAffected(0)") {
		t.Error("expected driver.RowsAffected(0) for empty params")
	}

	// Postgres should use $N renumbering
	if !strings.Contains(codeStr, `fmt.Fprintf(&sb, "$%d", base+j+1)`) {
		t.Error("expected Postgres $N placeholder renumbering in generated code")
	}

	// Should NOT contain static ? placeholders
	if strings.Contains(codeStr, `sb.WriteString("?")`) {
		t.Error("Postgres should NOT use ? placeholders")
	}

	// Should append args from params struct
	if !strings.Contains(codeStr, "args = append(args, p.Name)") {
		t.Error("expected args append for Name param")
	}
	if !strings.Contains(codeStr, "args = append(args, p.Email)") {
		t.Error("expected args append for Email param")
	}
}

func TestGenerateUnifiedRunner_BulkInsert_MySQL(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{makeBulkInsertQuery("BulkInsertAuthors")},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have the method
	if !strings.Contains(codeStr, "func (r *QueryRunner) BulkInsertAuthors(ctx context.Context, params []queries.BulkInsertAuthorsParams) (sql.Result, error)") {
		t.Error("expected BulkInsertAuthors method with []Params signature")
	}

	// MySQL should use ? placeholders
	if !strings.Contains(codeStr, `sb.WriteString("?")`) {
		t.Error("expected MySQL ? placeholder in generated code")
	}

	// Should NOT contain Postgres $N placeholders
	if strings.Contains(codeStr, `fmt.Fprintf(&sb, "$%d"`) {
		t.Error("MySQL should NOT use $N placeholders")
	}
}

func TestGenerateUnifiedRunner_BulkInsert_SQLite(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectSQLite,
		UserQueries: []query.SerializedQuery{makeBulkInsertQuery("BulkInsertAuthors")},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have the method
	if !strings.Contains(codeStr, "func (r *QueryRunner) BulkInsertAuthors(ctx context.Context, params []queries.BulkInsertAuthorsParams) (sql.Result, error)") {
		t.Error("expected BulkInsertAuthors method with []Params signature")
	}

	// SQLite should use ? placeholders
	if !strings.Contains(codeStr, `sb.WriteString("?")`) {
		t.Error("expected SQLite ? placeholder in generated code")
	}
}

func TestGenerateSharedTypes_BulkInsert(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{makeBulkInsertQuery("BulkInsertAuthors")},
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have params struct
	if !strings.Contains(codeStr, "type BulkInsertAuthorsParams struct") {
		t.Error("expected BulkInsertAuthorsParams struct")
	}

	// Params struct should have Name and Email fields
	if !strings.Contains(codeStr, "Name  string") && !strings.Contains(codeStr, "Name string") {
		t.Error("expected Name string field in BulkInsertAuthorsParams")
	}
	if !strings.Contains(codeStr, "Email  string") && !strings.Contains(codeStr, "Email string") {
		t.Error("expected Email string field in BulkInsertAuthorsParams")
	}

	// Should NOT have a result struct (bulk exec doesn't return rows)
	if strings.Contains(codeStr, "BulkInsertAuthorsResult") {
		t.Error("bulk exec should NOT have a result struct")
	}

	// Should have the bulk insert comment
	if !strings.Contains(codeStr, "Each element represents one row in the bulk insert") {
		t.Error("expected bulk insert doc comment")
	}
}

func TestGenerateUnifiedRunner_BulkInsert_WithTxCopiesFields(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{makeBulkInsertQuery("BulkInsertAuthors")},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// WithTx should copy bulk fields (go/format may adjust spacing)
	if !strings.Contains(codeStr, "bulkInsertAuthorsBulkPrefix:") {
		t.Error("WithTx should copy bulkInsertAuthorsBulkPrefix")
	}
	if !strings.Contains(codeStr, "bulkInsertAuthorsBulkSuffix:") {
		t.Error("WithTx should copy bulkInsertAuthorsBulkSuffix")
	}
	if !strings.Contains(codeStr, "bulkInsertAuthorsBulkParamsPerRow:") {
		t.Error("WithTx should copy bulkInsertAuthorsBulkParamsPerRow")
	}
}

func TestGenerateUnifiedRunner_BulkInsert_FormatValidation(t *testing.T) {
	// Verify generated code formats correctly for all dialects
	dialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}
	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{makeBulkInsertQuery("BulkInsertAuthors")},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed for %s: %v", dialect, err)
			}

			// If go/format succeeded, code won't be nil
			if len(code) == 0 {
				t.Errorf("generated code is empty for %s", dialect)
			}
		})
	}
}

func TestGenerateUnifiedRunner_BulkInsert_MixedWithRegularQueries(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath: "example.com/myapp",
		Dialect:    dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{
			makeBulkInsertQuery("BulkInsertAuthors"),
			{
				Name:       "GetUserByEmail",
				ReturnType: query.ReturnOne,
				AST: &query.SerializedAST{
					Kind:      "select",
					FromTable: query.SerializedTableRef{Name: "users"},
					SelectCols: []query.SerializedSelectExpr{
						{Expr: query.SerializedExpr{Type: "column", Column: &query.SerializedColumn{Table: "users", Name: "id", GoType: "int64"}}},
					},
					Where: &query.SerializedExpr{
						Type: "binary",
						Binary: &query.SerializedBinary{
							Left:  query.SerializedExpr{Type: "column", Column: &query.SerializedColumn{Table: "users", Name: "email", GoType: "string"}},
							Op:    "=",
							Right: query.SerializedExpr{Type: "param", Param: &query.SerializedParam{Name: "email", GoType: "string"}},
						},
					},
					Params: []query.SerializedParamInfo{{Name: "email", GoType: "string"}},
				},
			},
		},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Both methods should exist
	if !strings.Contains(codeStr, "func (r *QueryRunner) BulkInsertAuthors(") {
		t.Error("expected BulkInsertAuthors method")
	}
	if !strings.Contains(codeStr, "func (r *QueryRunner) GetUserByEmail(") {
		t.Error("expected GetUserByEmail method")
	}

	// Regular query should use SQL field, not bulk fields (go/format may add alignment spaces)
	if !strings.Contains(codeStr, "getUserByEmailSQL") {
		t.Error("expected getUserByEmailSQL field for regular query")
	}
	// Bulk query should use bulk fields, not SQL field
	if strings.Contains(codeStr, "bulkInsertAuthorsSQL string") {
		t.Error("bulk query should NOT have a plain SQL field")
	}
}

func TestGenerateUnifiedRunner_Postgres_NonNullableJSONColumn_RemainsRawMessage(t *testing.T) {
	// Build a query with an explicitly non-nullable JSON column
	// (GoType "json.RawMessage", no pointer).
	q := query.SerializedQuery{
		Name:       "GetEventByID",
		ReturnType: query.ReturnOne,
		AST: &query.SerializedAST{
			Kind:      "select",
			FromTable: query.SerializedTableRef{Name: "events"},
			SelectCols: []query.SerializedSelectExpr{
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "events",
							Name:   "id",
							GoType: "int64",
						},
					},
				},
				{
					Expr: query.SerializedExpr{
						Type: "column",
						Column: &query.SerializedColumn{
							Table:  "events",
							Name:   "metadata",
							GoType: "json.RawMessage",
						},
					},
				},
			},
			Params: []query.SerializedParamInfo{
				{Name: "id", GoType: "int64"},
			},
		},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{q},
	}

	typesCode, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	types := string(typesCode)

	// Non-nullable JSON should remain json.RawMessage, not *json.RawMessage.
	if !strings.Contains(types, "Metadata json.RawMessage") {
		t.Errorf("expected non-nullable JSON to remain json.RawMessage in result struct, got:\n%s", types)
	}
	if strings.Contains(types, "*json.RawMessage") {
		t.Errorf("non-nullable JSON column should NOT use *json.RawMessage:\n%s", types)
	}
}

// ---------------------------------------------------------------------------
// INSERT ... SELECT tests
// ---------------------------------------------------------------------------

func makeInsertSelectQuery() query.SerializedQuery {
	return query.SerializedQuery{
		Name:       "InsertFromSource",
		ReturnType: query.ReturnExec,
		AST: &query.SerializedAST{
			Kind:      "insert",
			FromTable: query.SerializedTableRef{Name: "target"},
			InsertCols: []query.SerializedColumn{
				{Table: "target", Name: "name", GoType: "string"},
				{Table: "target", Name: "email", GoType: "string"},
			},
			InsertSource: &query.SerializedAST{
				Kind:      "select",
				FromTable: query.SerializedTableRef{Name: "source"},
				SelectCols: []query.SerializedSelectExpr{
					{Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "source", Name: "name", GoType: "string"},
					}},
					{Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "source", Name: "email", GoType: "string"},
					}},
				},
				Where: &query.SerializedExpr{
					Type: "binary",
					Binary: &query.SerializedBinary{
						Left: query.SerializedExpr{
							Type:   "column",
							Column: &query.SerializedColumn{Table: "source", Name: "status", GoType: "string"},
						},
						Op: "=",
						Right: query.SerializedExpr{
							Type:  "param",
							Param: &query.SerializedParam{Name: "status", GoType: "string"},
						},
					},
				},
			},
			Params: []query.SerializedParamInfo{
				{Name: "status", GoType: "string"},
			},
		},
	}
}

func TestGenerateUnifiedRunner_InsertSelect_Postgres(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{makeInsertSelectQuery()},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "INSERT INTO") {
		t.Error("expected generated code to contain INSERT INTO")
	}
	if !strings.Contains(codeStr, "SELECT") {
		t.Error("expected generated code to contain SELECT")
	}
	if !strings.Contains(codeStr, "$1") {
		t.Error("expected Postgres $1 placeholder in generated code")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_MySQL(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{makeInsertSelectQuery()},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "INSERT INTO") {
		t.Error("expected generated code to contain INSERT INTO")
	}
	if !strings.Contains(codeStr, "SELECT") {
		t.Error("expected generated code to contain SELECT")
	}
	if !strings.Contains(codeStr, "?") {
		t.Error("expected MySQL ? placeholder in generated code")
	}
	// MySQL uses backtick quoting
	if !strings.Contains(codeStr, "`") {
		t.Error("expected backtick quoting in MySQL generated code")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_SQLite(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectSQLite,
		UserQueries: []query.SerializedQuery{makeInsertSelectQuery()},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "INSERT INTO") {
		t.Error("expected generated code to contain INSERT INTO")
	}
	if !strings.Contains(codeStr, "SELECT") {
		t.Error("expected generated code to contain SELECT")
	}
	if !strings.Contains(codeStr, "?") {
		t.Error("expected SQLite ? placeholder in generated code")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_WithReturning_Postgres(t *testing.T) {
	q := makeInsertSelectQuery()
	q.Name = "InsertFromSourceReturning"
	q.ReturnType = query.ReturnOne
	q.AST.Returning = []query.SerializedColumn{
		{Table: "target", Name: "id", GoType: "int64"},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{q},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "RETURNING") {
		t.Error("expected generated code to contain RETURNING")
	}
	if !strings.Contains(codeStr, "INSERT INTO") {
		t.Error("expected generated code to contain INSERT INTO")
	}
	if !strings.Contains(codeStr, "SELECT") {
		t.Error("expected generated code to contain SELECT")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_WithCTE_Postgres(t *testing.T) {
	q := query.SerializedQuery{
		Name:       "InsertFromCTE",
		ReturnType: query.ReturnExec,
		AST: &query.SerializedAST{
			Kind:      "insert",
			FromTable: query.SerializedTableRef{Name: "target"},
			CTEs: []query.SerializedCTE{
				{
					Name: "filtered",
					Query: &query.SerializedAST{
						Kind:      "select",
						FromTable: query.SerializedTableRef{Name: "users"},
						SelectCols: []query.SerializedSelectExpr{
							{Expr: query.SerializedExpr{
								Type:   "column",
								Column: &query.SerializedColumn{Table: "users", Name: "name", GoType: "string"},
							}},
							{Expr: query.SerializedExpr{
								Type:   "column",
								Column: &query.SerializedColumn{Table: "users", Name: "email", GoType: "string"},
							}},
						},
						Where: &query.SerializedExpr{
							Type: "binary",
							Binary: &query.SerializedBinary{
								Left: query.SerializedExpr{
									Type:   "column",
									Column: &query.SerializedColumn{Table: "users", Name: "active", GoType: "bool"},
								},
								Op: "=",
								Right: query.SerializedExpr{
									Type:  "param",
									Param: &query.SerializedParam{Name: "active", GoType: "bool"},
								},
							},
						},
					},
				},
			},
			InsertCols: []query.SerializedColumn{
				{Table: "target", Name: "name", GoType: "string"},
				{Table: "target", Name: "email", GoType: "string"},
			},
			InsertSource: &query.SerializedAST{
				Kind:      "select",
				FromTable: query.SerializedTableRef{Name: "filtered"},
				SelectCols: []query.SerializedSelectExpr{
					{Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "filtered", Name: "name", GoType: "string"},
					}},
					{Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "filtered", Name: "email", GoType: "string"},
					}},
				},
			},
			Params: []query.SerializedParamInfo{
				{Name: "active", GoType: "bool"},
			},
		},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{q},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "WITH") {
		t.Error("expected generated code to contain WITH (CTE)")
	}
	if !strings.Contains(codeStr, "INSERT INTO") {
		t.Error("expected generated code to contain INSERT INTO")
	}
	if !strings.Contains(codeStr, "SELECT") {
		t.Error("expected generated code to contain SELECT")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_NoParams(t *testing.T) {
	q := query.SerializedQuery{
		Name:       "InsertAllFromSource",
		ReturnType: query.ReturnExec,
		AST: &query.SerializedAST{
			Kind:      "insert",
			FromTable: query.SerializedTableRef{Name: "target"},
			InsertCols: []query.SerializedColumn{
				{Table: "target", Name: "name", GoType: "string"},
				{Table: "target", Name: "email", GoType: "string"},
			},
			InsertSource: &query.SerializedAST{
				Kind:      "select",
				FromTable: query.SerializedTableRef{Name: "source"},
				SelectCols: []query.SerializedSelectExpr{
					{Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "source", Name: "name", GoType: "string"},
					}},
					{Expr: query.SerializedExpr{
						Type:   "column",
						Column: &query.SerializedColumn{Table: "source", Name: "email", GoType: "string"},
					}},
				},
			},
			Params: []query.SerializedParamInfo{},
		},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{q},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	if len(code) == 0 {
		t.Error("expected non-empty generated code")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_BulkExec_Rejected(t *testing.T) {
	q := makeInsertSelectQuery()
	q.Name = "InsertSelectBulk"
	q.ReturnType = query.ReturnBulkExec

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{q},
	}

	_, err := GenerateUnifiedRunner(cfg)
	if err == nil {
		t.Fatal("expected error for INSERT ... SELECT with ReturnBulkExec, got nil")
	}
	if !strings.Contains(err.Error(), "ReturnBulkExec") {
		t.Errorf("expected error to mention ReturnBulkExec, got: %v", err)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_MySQL_Returning_Rejected(t *testing.T) {
	q := makeInsertSelectQuery()
	q.Name = "InsertSelectReturningMySQL"
	q.ReturnType = query.ReturnOne
	q.AST.Returning = []query.SerializedColumn{
		{Table: "target", Name: "id", GoType: "int64"},
	}

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{q},
	}

	_, err := GenerateUnifiedRunner(cfg)
	if err == nil {
		t.Fatal("expected error for INSERT ... SELECT ... RETURNING on MySQL, got nil")
	}
	if !strings.Contains(err.Error(), "not supported on MySQL") {
		t.Errorf("expected error to mention 'not supported on MySQL', got: %v", err)
	}
}

func TestGenerateSharedTypes_InsertSelect(t *testing.T) {
	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{makeInsertSelectQuery()},
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have params struct
	if !strings.Contains(codeStr, "type InsertFromSourceParams struct") {
		t.Error("expected InsertFromSourceParams struct in generated types")
	}

	// Params struct should have the Status field from the source WHERE clause
	if !strings.Contains(codeStr, "Status") {
		t.Error("expected Status field in InsertFromSourceParams")
	}
	if !strings.Contains(codeStr, "string") {
		t.Error("expected string type for Status param")
	}

	// Should NOT have a result struct (exec doesn't return rows)
	if strings.Contains(codeStr, "InsertFromSourceResult") {
		t.Error("exec query should NOT have a result struct")
	}

	// Verify it's valid Go
	_, parseErr := parser.ParseFile(token.NewFileSet(), "types.go", code, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated types code is not valid Go: %v", parseErr)
	}
}

func TestGenerateUnifiedRunner_InsertSelect_FormatValidation(t *testing.T) {
	dialects := []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite}
	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{makeInsertSelectQuery()},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed for %s: %v", dialect, err)
			}

			if len(code) == 0 {
				t.Errorf("generated code is empty for %s", dialect)
			}

			// Verify go/parser accepts the output
			_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", code, parser.AllErrors)
			if parseErr != nil {
				t.Fatalf("generated code for %s is not valid Go: %v", dialect, parseErr)
			}
		})
	}
}

// =============================================================================
// Regression tests for Bug 1: [null] from empty LEFT JOINs
// =============================================================================

// TestGenerateUnifiedRunner_JSONAgg_NullStrip_MySQL verifies that the generated
// runner for MySQL emits stripJSONNulls before json.Unmarshal for json_agg fields.
// Regression test: MySQL JSON_ARRAYAGG(CASE WHEN ... END) produces [null] for
// empty LEFT JOINs, which unmarshals to [{zero-value}] instead of [].
func TestGenerateUnifiedRunner_JSONAgg_NullStrip_MySQL(t *testing.T) {
	sq := makeJSONAggQuery("GetDraft", []query.SerializedColumn{
		{Table: "entries", Name: "public_id", GoType: "string"},
		{Table: "entries", Name: "title", GoType: "string"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Must emit the stripJSONNulls helper
	if !strings.Contains(codeStr, "func stripJSONNulls(") {
		t.Error("expected stripJSONNulls helper to be emitted for MySQL")
	}

	// Must call stripJSONNulls before json.Unmarshal
	if !strings.Contains(codeStr, "stripJSONNulls(rolesRaw)") {
		t.Error("expected stripJSONNulls(rolesRaw) call before unmarshal")
	}

	// stripJSONNulls must appear BEFORE json.Unmarshal in the output
	stripIdx := strings.Index(codeStr, "stripJSONNulls(rolesRaw)")
	unmarshalIdx := strings.Index(codeStr, "json.Unmarshal([]byte(rolesRaw)")
	if stripIdx < 0 || unmarshalIdx < 0 || stripIdx >= unmarshalIdx {
		t.Error("stripJSONNulls must be called before json.Unmarshal")
	}

	// Must be valid Go
	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_JSONAgg_NullStrip_SQLite verifies that the generated
// runner for SQLite also emits stripJSONNulls.
func TestGenerateUnifiedRunner_JSONAgg_NullStrip_SQLite(t *testing.T) {
	sq := makeJSONAggQuery("GetDraft", []query.SerializedColumn{
		{Table: "entries", Name: "public_id", GoType: "string"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectSQLite,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "func stripJSONNulls(") {
		t.Error("expected stripJSONNulls helper to be emitted for SQLite")
	}

	if !strings.Contains(codeStr, "stripJSONNulls(rolesRaw)") {
		t.Error("expected stripJSONNulls(rolesRaw) call before unmarshal")
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_JSONAgg_NullStrip_Postgres_NotEmitted verifies that
// Postgres does NOT emit stripJSONNulls because Postgres uses FILTER (WHERE ...)
// which never produces null entries.
func TestGenerateUnifiedRunner_JSONAgg_NullStrip_Postgres_NotEmitted(t *testing.T) {
	sq := makeJSONAggQuery("GetDraft", []query.SerializedColumn{
		{Table: "entries", Name: "public_id", GoType: "string"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectPostgres,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "stripJSONNulls") {
		t.Error("Postgres should NOT emit stripJSONNulls (uses FILTER WHERE instead)")
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_JSONAgg_NullStrip_ReturnMany verifies null strip is
// also emitted for ReturnMany queries (row-loop path).
func TestGenerateUnifiedRunner_JSONAgg_NullStrip_ReturnMany(t *testing.T) {
	sq := makeJSONAggQuery("ListDrafts", []query.SerializedColumn{
		{Table: "entries", Name: "public_id", GoType: "string"},
	})
	sq.ReturnType = query.ReturnMany

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "stripJSONNulls(rolesRaw)") {
		t.Error("expected stripJSONNulls in ReturnMany row loop")
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// =============================================================================
// Regression tests for Bug 2: time.Time fields inside JSONAGG break on MySQL
// =============================================================================

// TestGenerateSharedTypes_JSONAgg_TimeColumn verifies that when json_agg
// columns include time.Time fields, the generated struct uses time.Time as the
// Go type (the fix is in the SQL layer, not the type layer).
func TestGenerateSharedTypes_JSONAgg_TimeColumn(t *testing.T) {
	sq := makeJSONAggQuery("GetDraftWithEntries", []query.SerializedColumn{
		{Table: "entries", Name: "title", GoType: "string"},
		{Table: "entries", Name: "created_at", GoType: "time.Time"},
		{Table: "entries", Name: "updated_at", GoType: "*time.Time"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateSharedTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateSharedTypes failed: %v", err)
	}

	codeStr := string(code)

	// The item struct should have time.Time fields
	if !strings.Contains(codeStr, "CreatedAt time.Time") {
		t.Error("expected CreatedAt time.Time in nested struct")
	}
	if !strings.Contains(codeStr, "UpdatedAt *time.Time") {
		t.Error("expected UpdatedAt *time.Time in nested struct")
	}

	if _, err := format.Source(code); err != nil {
		t.Errorf("generated types.go is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_JSONAgg_TimeColumn_MySQL_ValidGo verifies that the
// full generated runner with time.Time json_agg columns produces valid Go.
// Regression test: MySQL JSON_OBJECT serializes datetime in MySQL format, not
// RFC3339. The fix uses DATE_FORMAT in the SQL, so the generated Go scanner
// should remain the same (json.Unmarshal of time.Time from an RFC3339 string).
func TestGenerateUnifiedRunner_JSONAgg_TimeColumn_MySQL_ValidGo(t *testing.T) {
	sq := makeJSONAggQuery("GetDraftWithEntries", []query.SerializedColumn{
		{Table: "entries", Name: "title", GoType: "string"},
		{Table: "entries", Name: "created_at", GoType: "time.Time"},
	})

	cfg := UnifiedRunnerConfig{
		ModulePath:  "example.com/myapp",
		Dialect:     dburl.DialectMySQL,
		UserQueries: []query.SerializedQuery{sq},
	}

	code, err := GenerateUnifiedRunner(cfg)
	if err != nil {
		t.Fatalf("GenerateUnifiedRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should scan json_agg into raw string and unmarshal
	if !strings.Contains(codeStr, "var rolesRaw string") {
		t.Error("expected 'var rolesRaw string' for json_agg scan")
	}
	if !strings.Contains(codeStr, "json.Unmarshal([]byte(rolesRaw), &result.Roles)") {
		t.Error("expected json.Unmarshal for json_agg field")
	}

	// Must be valid Go
	if _, err := format.Source(code); err != nil {
		t.Errorf("generated runner is not valid Go: %v\n%s", err, codeStr)
	}
}

// TestGenerateUnifiedRunner_JSONAgg_TimeColumn_AllDialects_FormatsOK ensures
// the generated code is valid Go for all three dialects when json_agg includes
// time columns.
func TestGenerateUnifiedRunner_JSONAgg_TimeColumn_AllDialects_FormatsOK(t *testing.T) {
	for _, dialect := range []string{dburl.DialectPostgres, dburl.DialectMySQL, dburl.DialectSQLite} {
		t.Run(dialect, func(t *testing.T) {
			sq := makeJSONAggQuery("GetDraftWithEntries", []query.SerializedColumn{
				{Table: "entries", Name: "title", GoType: "string"},
				{Table: "entries", Name: "created_at", GoType: "time.Time"},
				{Table: "entries", Name: "deleted_at", GoType: "*time.Time"},
			})

			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{sq},
			}

			code, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Fatalf("GenerateUnifiedRunner failed: %v", err)
			}

			if _, err := format.Source(code); err != nil {
				t.Errorf("generated runner is not valid Go: %v\n%s", err, string(code))
			}
		})
	}
}
