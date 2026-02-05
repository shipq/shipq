package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateHTTPTestHarness_Basic(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should have package declaration
	if !strings.Contains(codeStr, "package api") {
		t.Error("missing package declaration")
	}

	// Should have TestServer struct
	if !strings.Contains(codeStr, "type TestServer struct") {
		t.Error("missing TestServer struct")
	}

	// Should embed httptest.Server
	if !strings.Contains(codeStr, "*httptest.Server") {
		t.Error("missing embedded httptest.Server")
	}

	// Should have transaction field
	if !strings.Contains(codeStr, "tx     *sql.Tx") {
		t.Error("missing transaction field")
	}

	// Should have NewUnauthenticatedTestServer function
	if !strings.Contains(codeStr, "func NewUnauthenticatedTestServer") {
		t.Error("missing NewUnauthenticatedTestServer function")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestHarness_TransactionManagement(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "postgres",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should begin transaction
	if !strings.Contains(codeStr, "db.BeginTx(ctx, nil)") {
		t.Error("missing BeginTx call")
	}

	// Should rollback transaction in cleanup
	if !strings.Contains(codeStr, "tx.Rollback()") {
		t.Error("missing Rollback call")
	}

	// Should use t.Cleanup
	if !strings.Contains(codeStr, "t.Cleanup(func()") {
		t.Error("missing t.Cleanup")
	}

	// Should close server in cleanup
	if !strings.Contains(codeStr, "ts.Close()") {
		t.Error("missing server Close in cleanup")
	}
}

func TestGenerateHTTPTestHarness_MuxCreation(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "sqlite",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should create mux with transaction
	if !strings.Contains(codeStr, "NewMux(tx)") {
		t.Error("missing NewMux call with transaction")
	}

	// Should create httptest.Server with mux
	if !strings.Contains(codeStr, "httptest.NewServer(mux)") {
		t.Error("missing httptest.NewServer call")
	}
}

func TestGenerateHTTPTestHarness_TxAccessor(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should have Tx() accessor method
	if !strings.Contains(codeStr, "func (ts *TestServer) Tx() *sql.Tx") {
		t.Error("missing Tx() accessor method")
	}

	// Should return the transaction
	if !strings.Contains(codeStr, "return ts.tx") {
		t.Error("missing return statement in Tx()")
	}
}

func TestGenerateHTTPTestHarness_HelperMethod(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should call t.Helper()
	if !strings.Contains(codeStr, "t.Helper()") {
		t.Error("missing t.Helper() call")
	}
}

func TestGenerateHTTPTestHarness_Imports(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should import required packages
	requiredImports := []string{
		`"context"`,
		`"database/sql"`,
		`"net/http/httptest"`,
		`"testing"`,
	}

	for _, imp := range requiredImports {
		if !strings.Contains(codeStr, imp) {
			t.Errorf("missing import %s", imp)
		}
	}
}

func TestGenerateHTTPTestHarness_DifferentPackageName(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "myapi",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should have custom package name
	if !strings.Contains(codeStr, "package myapi") {
		t.Error("missing custom package declaration")
	}

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateHTTPTestHarness_ContextCancellation(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should create cancellable context
	if !strings.Contains(codeStr, "context.WithCancel(context.Background())") {
		t.Error("missing context.WithCancel")
	}

	// Should have cancel field
	if !strings.Contains(codeStr, "cancel context.CancelFunc") {
		t.Error("missing cancel field in struct")
	}

	// Should call cancel in cleanup
	if !strings.Contains(codeStr, "cancel()") {
		t.Error("missing cancel() call in cleanup")
	}
}

func TestGenerateHTTPTestHarness_ErrorHandling(t *testing.T) {
	cfg := HTTPTestHarnessGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPTestHarness(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPTestHarness() error = %v", err)
	}

	codeStr := string(code)

	// Should handle BeginTx error with t.Fatalf
	if !strings.Contains(codeStr, "t.Fatalf") {
		t.Error("missing t.Fatalf for error handling")
	}

	// Should include error message about transaction
	if !strings.Contains(codeStr, "failed to begin transaction") {
		t.Error("missing transaction error message")
	}
}
