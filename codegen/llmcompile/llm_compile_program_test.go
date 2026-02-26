package llmcompile

import (
	"strings"
	"testing"
)

func TestGenerateLLMCompileProgram_SinglePackage(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		ToolPkgs:   []string{"myapp/tools/weather"},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	// Should import the llm package.
	if !strings.Contains(code, `"myapp/shipq/lib/llm"`) {
		t.Error("expected import of myapp/shipq/lib/llm")
	}

	// Should import the tool package with an alias.
	if !strings.Contains(code, `"myapp/tools/weather"`) {
		t.Error("expected import of myapp/tools/weather")
	}

	// Should call the Register function.
	if !strings.Contains(code, ".Register(app)") {
		t.Error("expected call to Register(app)")
	}

	// Should create the app.
	if !strings.Contains(code, "llm.NewApp()") {
		t.Error("expected llm.NewApp() call")
	}

	// Should reference tool metadata fields.
	if !strings.Contains(code, "t.InputType") {
		t.Error("expected reference to t.InputType")
	}
	if !strings.Contains(code, "t.OutputType") {
		t.Error("expected reference to t.OutputType")
	}
	if !strings.Contains(code, "t.Package") {
		t.Error("expected reference to t.Package")
	}
	if !strings.Contains(code, "t.InputSchema") {
		t.Error("expected reference to t.InputSchema")
	}

	// Should be valid Go (format.Source would have failed otherwise).
	if !strings.Contains(code, "package main") {
		t.Error("expected package main declaration")
	}
}

func TestGenerateLLMCompileProgram_MultiplePackages(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		ToolPkgs:   []string{"myapp/tools/weather", "myapp/tools/calendar", "myapp/tools/search"},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	// All three packages should be imported.
	if !strings.Contains(code, `"myapp/tools/weather"`) {
		t.Error("expected import of myapp/tools/weather")
	}
	if !strings.Contains(code, `"myapp/tools/calendar"`) {
		t.Error("expected import of myapp/tools/calendar")
	}
	if !strings.Contains(code, `"myapp/tools/search"`) {
		t.Error("expected import of myapp/tools/search")
	}

	// Should have three Register calls (one per package).
	count := strings.Count(code, ".Register(app)")
	if count != 3 {
		t.Errorf("expected 3 Register(app) calls, got %d", count)
	}
}

func TestGenerateLLMCompileProgram_NoPackages(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		ToolPkgs:   []string{},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	// Should still generate valid Go with no Register calls.
	if !strings.Contains(code, "package main") {
		t.Error("expected package main declaration")
	}

	// No Register calls.
	if strings.Contains(code, ".Register(app)") {
		t.Error("expected no Register(app) calls for empty tool packages")
	}

	// Should still create app and serialize.
	if !strings.Contains(code, "llm.NewApp()") {
		t.Error("expected llm.NewApp() call even with no tool packages")
	}
}

func TestGenerateLLMCompileProgram_DeterministicOutput(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		ToolPkgs:   []string{"myapp/tools/b", "myapp/tools/a", "myapp/tools/c"},
	}

	src1, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("first GenerateLLMCompileProgram failed: %v", err)
	}

	src2, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("second GenerateLLMCompileProgram failed: %v", err)
	}

	if string(src1) != string(src2) {
		t.Error("expected deterministic output for same config")
	}
}

func TestGenerateLLMCompileProgram_SortedImports(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		// Deliberately out of order.
		ToolPkgs: []string{"myapp/tools/zeta", "myapp/tools/alpha"},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	// alpha should appear before zeta (sorted).
	alphaIdx := strings.Index(code, "myapp/tools/alpha")
	zetaIdx := strings.Index(code, "myapp/tools/zeta")

	if alphaIdx < 0 || zetaIdx < 0 {
		t.Fatal("expected both tool package imports to be present")
	}

	if alphaIdx > zetaIdx {
		t.Error("expected tool packages to be sorted alphabetically in imports")
	}
}

func TestGenerateLLMCompileProgram_MonorepoModulePath(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "github.com/company/monorepo/services/myservice",
		ToolPkgs:   []string{"github.com/company/monorepo/services/myservice/tools/weather"},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	// Should use the full module path for the llm import.
	if !strings.Contains(code, `"github.com/company/monorepo/services/myservice/shipq/lib/llm"`) {
		t.Error("expected full import path for llm package")
	}

	if !strings.Contains(code, `"github.com/company/monorepo/services/myservice/tools/weather"`) {
		t.Error("expected full import path for tool package")
	}
}

func TestGenerateLLMCompileProgram_ContainsJSONSerializer(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		ToolPkgs:   []string{"myapp/tools/weather"},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	// Should encode to stdout.
	if !strings.Contains(code, "json.NewEncoder(os.Stdout)") {
		t.Error("expected JSON encoder writing to stdout")
	}

	// Should have the toolMeta struct with all expected fields.
	if !strings.Contains(code, "toolMeta") {
		t.Error("expected toolMeta struct definition")
	}

	// Check JSON tags exist.
	for _, tag := range []string{`"name"`, `"description"`, `"input_schema"`, `"input_type"`, `"output_type"`, `"package"`} {
		if !strings.Contains(code, tag) {
			t.Errorf("expected JSON tag %s in generated code", tag)
		}
	}
}

func TestGenerateLLMCompileProgram_HasGeneratedHeader(t *testing.T) {
	cfg := LLMCompileProgramConfig{
		ModulePath: "myapp",
		ToolPkgs:   []string{},
	}

	src, err := GenerateLLMCompileProgram(cfg)
	if err != nil {
		t.Fatalf("GenerateLLMCompileProgram failed: %v", err)
	}

	code := string(src)

	if !strings.Contains(code, "Code generated by shipq") {
		t.Error("expected generated file header comment")
	}
}
