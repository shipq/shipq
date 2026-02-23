package openapigen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateOpenAPITest_ValidGo(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath:      "example.com/app",
		OutputPkg:       "api",
		DBDialect:       "mysql",
		TestDatabaseURL: "mysql://root@localhost/testdb",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	// Should be valid Go
	_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n%s", err, codeStr)
	}
}

func TestGenerateOpenAPITest_PackageName(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "myapi",
		DBDialect:  "postgres",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "package myapi") {
		t.Error("missing custom package declaration")
	}
}

func TestGenerateOpenAPITest_ContainsTestFunctions(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	requiredFuncs := []string{
		"func TestOpenAPISpec(t *testing.T)",
		"func TestOpenAPIDocs(t *testing.T)",
		"func TestOpenAPIAssets(t *testing.T)",
		"func newOpenAPITestServer(t *testing.T)",
	}

	for _, fn := range requiredFuncs {
		if !strings.Contains(codeStr, fn) {
			t.Errorf("missing function: %s", fn)
		}
	}
}

func TestGenerateOpenAPITest_ContainsAssertions(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	// Should check for OpenAPI version
	if !strings.Contains(codeStr, `"3.1"`) {
		t.Error("missing OpenAPI version check")
	}

	// Should check for elements-api component
	if !strings.Contains(codeStr, "elements-api") {
		t.Error("missing elements-api check")
	}

	// Should check content types
	if !strings.Contains(codeStr, "application/json") {
		t.Error("missing application/json content type check")
	}
	if !strings.Contains(codeStr, "text/html") {
		t.Error("missing text/html content type check")
	}
	if !strings.Contains(codeStr, "application/javascript") {
		t.Error("missing application/javascript content type check")
	}
	if !strings.Contains(codeStr, "text/css") {
		t.Error("missing text/css content type check")
	}
}

func TestGenerateOpenAPITest_DevModeSetup(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	// Should mention the supported modes
	if !strings.Contains(codeStr, `"development"`) && !strings.Contains(codeStr, `"test"`) {
		t.Error("missing documentation of supported GO_ENV modes")
	}
}

func TestGenerateOpenAPITest_TestDatabaseURL(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath:      "example.com/app",
		OutputPkg:       "api",
		DBDialect:       "postgres",
		TestDatabaseURL: "postgres://localhost/myapp_test",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "postgres://localhost/myapp_test") {
		t.Error("missing test database URL fallback")
	}
}

func TestGenerateOpenAPITest_Imports(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	requiredImports := []string{
		`"database/sql"`,
		`"encoding/json"`,
		`"io"`,
		`"net/http"`,
		`"os"`,
		`"strings"`,
		`"testing"`,
		`"example.com/app/config"`,
	}

	for _, imp := range requiredImports {
		if !strings.Contains(codeStr, imp) {
			t.Errorf("missing import %s", imp)
		}
	}
}

func TestGenerateOpenAPITest_UsesTestHarness(t *testing.T) {
	cfg := OpenAPITestGenConfig{
		ModulePath: "example.com/app",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateOpenAPITest(cfg)
	if err != nil {
		t.Fatalf("GenerateOpenAPITest() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "NewUnauthenticatedTestServer") {
		t.Error("generated test should use NewUnauthenticatedTestServer from the test harness")
	}
}
