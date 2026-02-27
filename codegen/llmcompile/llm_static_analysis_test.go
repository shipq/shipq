package llmcompile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindToolRegistrations_SingleTool(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	pkgName := "weather"
	importPath := modulePath + "/tools/" + pkgName

	// Create the package directory structure.
	pkgDir := filepath.Join(tmpDir, "tools", pkgName)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package weather

import "myapp/shipq/lib/llm"

type WeatherInput struct {
	City string
}

type WeatherOutput struct {
	Temp float64
}

func GetWeather(input *WeatherInput) (*WeatherOutput, error) {
	return nil, nil
}

func Register(app *llm.App) {
	app.Tool("get_weather", "Get the current weather for a city", GetWeather)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", tool.Name)
	}
	if tool.Description != "Get the current weather for a city" {
		t.Errorf("expected description 'Get the current weather for a city', got %q", tool.Description)
	}
	if tool.FuncName != "GetWeather" {
		t.Errorf("expected FuncName 'GetWeather', got %q", tool.FuncName)
	}
	if tool.Line == 0 {
		t.Error("expected non-zero line number")
	}
}

func TestFindToolRegistrations_MultipleTools(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	pkgName := "tools"
	importPath := modulePath + "/" + pkgName

	pkgDir := filepath.Join(tmpDir, pkgName)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package tools

import "myapp/shipq/lib/llm"

func GetWeather(input *WeatherInput) (*WeatherOutput, error) { return nil, nil }
func SearchWeb(input *SearchInput) (*SearchOutput, error) { return nil, nil }
func SendEmail(input *EmailInput) (*EmailOutput, error) { return nil, nil }

func Register(app *llm.App) {
	app.Tool("get_weather", "Get the current weather", GetWeather)
	app.Tool("search_web", "Search the web", SearchWeb)
	app.Tool("send_email", "Send an email", SendEmail)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	expectedNames := []string{"get_weather", "search_web", "send_email"}
	expectedDescs := []string{"Get the current weather", "Search the web", "Send an email"}
	expectedFuncs := []string{"GetWeather", "SearchWeb", "SendEmail"}

	for i, tool := range tools {
		if tool.Name != expectedNames[i] {
			t.Errorf("tool %d: expected name %q, got %q", i, expectedNames[i], tool.Name)
		}
		if tool.Description != expectedDescs[i] {
			t.Errorf("tool %d: expected description %q, got %q", i, expectedDescs[i], tool.Description)
		}
		if tool.FuncName != expectedFuncs[i] {
			t.Errorf("tool %d: expected FuncName %q, got %q", i, expectedFuncs[i], tool.FuncName)
		}
	}
}

func TestFindToolRegistrations_NoRegisterFunc(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/empty"

	pkgDir := filepath.Join(tmpDir, "tools", "empty")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package empty

func DoSomething() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "empty.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools when no Register func, got %d", len(tools))
	}
}

func TestFindToolRegistrations_SkipsTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/weather"

	pkgDir := filepath.Join(tmpDir, "tools", "weather")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// Put the Register function in a _test.go file — should be skipped.
	testContent := `package weather

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
	app.Tool("test_tool", "A tool in a test file", TestFunc)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register_test.go"), []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Also write a non-test file with no Register func.
	normalContent := `package weather

func SomeHelper() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(normalContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools (test files should be skipped), got %d", len(tools))
	}
}

func TestFindToolRegistrations_SkipsGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/weather"

	pkgDir := filepath.Join(tmpDir, "tools", "weather")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// Put the Register function in a zz_generated_ file — should be skipped.
	genContent := `package weather

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
	app.Tool("gen_tool", "A tool in a generated file", GenFunc)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "zz_generated_registry.go"), []byte(genContent), 0644); err != nil {
		t.Fatalf("failed to write generated file: %v", err)
	}

	// Also write a non-generated file with no Register func.
	normalContent := `package weather

func SomeHelper() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(normalContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools (generated files should be skipped), got %d", len(tools))
	}
}

func TestFindToolRegistrations_SelectorFunc(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/multi"

	pkgDir := filepath.Join(tmpDir, "tools", "multi")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package multi

import (
	"myapp/shipq/lib/llm"
	"myapp/otherpkg"
)

func Register(app *llm.App) {
	app.Tool("remote_tool", "A tool from another package", otherpkg.RemoteFunc)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].FuncName != "otherpkg.RemoteFunc" {
		t.Errorf("expected FuncName 'otherpkg.RemoteFunc', got %q", tools[0].FuncName)
	}
}

func TestFindToolRegistrations_ChainedCalls(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/chained"

	pkgDir := filepath.Join(tmpDir, "tools", "chained")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package chained

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
	app.Tool("tool_a", "First tool", FuncA).Tool("tool_b", "Second tool", FuncB)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools from chained calls, got %d", len(tools))
	}

	// ast.Inspect visits the outer call first (tool_b), then the inner (tool_a),
	// because chaining means app.Tool("tool_a",...).Tool("tool_b",...) — the
	// outer CallExpr is the .Tool("tool_b",...) call whose receiver is the
	// inner .Tool("tool_a",...) call.
	// We just verify both tools are present regardless of order.
	names := map[string]string{}
	for _, tool := range tools {
		names[tool.Name] = tool.FuncName
	}
	if fn, ok := names["tool_a"]; !ok {
		t.Error("expected to find tool 'tool_a'")
	} else if fn != "FuncA" {
		t.Errorf("expected tool_a FuncName 'FuncA', got %q", fn)
	}
	if fn, ok := names["tool_b"]; !ok {
		t.Error("expected to find tool 'tool_b'")
	} else if fn != "FuncB" {
		t.Errorf("expected tool_b FuncName 'FuncB', got %q", fn)
	}
}

func TestFindToolRegistrations_MethodReceiver(t *testing.T) {
	// A method named Register on a struct should NOT be picked up.
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/methodrecv"

	pkgDir := filepath.Join(tmpDir, "tools", "methodrecv")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package methodrecv

import "myapp/shipq/lib/llm"

type MyStruct struct{}

func (s *MyStruct) Register(app *llm.App) {
	app.Tool("should_not_find", "not a top-level Register", Foo)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools for method receiver Register, got %d", len(tools))
	}
}

func TestFindToolRegistrations_WrongParamType(t *testing.T) {
	// Register with a parameter that's not *llm.App should be skipped.
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/wrongparam"

	pkgDir := filepath.Join(tmpDir, "tools", "wrongparam")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package wrongparam

import "myapp/shipq/lib/channel"

func Register(app *channel.App) {
	// This is a channel Register, not an LLM Register.
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools for non-llm Register, got %d", len(tools))
	}
}

func TestFindToolRegistrations_AliasedImport(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/aliased"

	pkgDir := filepath.Join(tmpDir, "tools", "aliased")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package aliased

import myllm "myapp/shipq/lib/llm"

func Register(app *myllm.App) {
	app.Tool("aliased_tool", "Tool with aliased import", DoStuff)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool with aliased import, got %d", len(tools))
	}

	if tools[0].Name != "aliased_tool" {
		t.Errorf("expected tool name 'aliased_tool', got %q", tools[0].Name)
	}
	if tools[0].FuncName != "DoStuff" {
		t.Errorf("expected FuncName 'DoStuff', got %q", tools[0].FuncName)
	}
}

func TestFindToolRegistrations_NonStringLiteralName(t *testing.T) {
	// app.Tool(variable, ...) where name is not a string literal → tool is skipped.
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/nonliteral"

	pkgDir := filepath.Join(tmpDir, "tools", "nonliteral")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package nonliteral

import "myapp/shipq/lib/llm"

var toolName = "dynamic_tool"

func Register(app *llm.App) {
	app.Tool(toolName, "dynamic name", SomeFunc)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	// Non-literal names are silently skipped (the compile program will catch real errors).
	if len(tools) != 0 {
		t.Errorf("expected 0 tools for non-literal name, got %d", len(tools))
	}
}

func TestFindToolRegistrations_MultipleFilesInPackage(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/multifile"

	pkgDir := filepath.Join(tmpDir, "tools", "multifile")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// Register in one file.
	registerContent := `package multifile

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
	app.Tool("tool_one", "First tool", FuncOne)
	app.Tool("tool_two", "Second tool", FuncTwo)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(registerContent), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	// Non-Go file should be ignored.
	if err := os.WriteFile(filepath.Join(pkgDir, "README.md"), []byte("# Tools"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	// Another Go file with helpers (no Register).
	helperContent := `package multifile

func FuncOne(input *Input) (*Output, error) { return nil, nil }
func FuncTwo(input *Input) (*Output, error) { return nil, nil }
`
	if err := os.WriteFile(filepath.Join(pkgDir, "funcs.go"), []byte(helperContent), 0644); err != nil {
		t.Fatalf("failed to write funcs.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools across multiple files, got %d", len(tools))
	}

	if tools[0].Name != "tool_one" {
		t.Errorf("tool 0: expected name 'tool_one', got %q", tools[0].Name)
	}
	if tools[1].Name != "tool_two" {
		t.Errorf("tool 1: expected name 'tool_two', got %q", tools[1].Name)
	}
}

func TestFindToolRegistrations_DifferentParamName(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/diffparam"

	pkgDir := filepath.Join(tmpDir, "tools", "diffparam")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package diffparam

import "myapp/shipq/lib/llm"

func Register(registry *llm.App) {
	registry.Tool("my_tool", "A tool", MyFunc)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool with different param name, got %d", len(tools))
	}

	if tools[0].Name != "my_tool" {
		t.Errorf("expected tool name 'my_tool', got %q", tools[0].Name)
	}
}

func TestFindToolRegistrations_BacktickStrings(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/backtick"

	pkgDir := filepath.Join(tmpDir, "tools", "backtick")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := "package backtick\n\nimport \"myapp/shipq/lib/llm\"\n\nfunc Register(app *llm.App) {\n\tapp.Tool(`raw_tool`, `A raw string description`, RawFunc)\n}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool with backtick strings, got %d", len(tools))
	}

	if tools[0].Name != "raw_tool" {
		t.Errorf("expected tool name 'raw_tool', got %q", tools[0].Name)
	}
	if tools[0].Description != "A raw string description" {
		t.Errorf("expected description 'A raw string description', got %q", tools[0].Description)
	}
}

func TestFindToolRegistrations_DirectoryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/nonexistent"

	_, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestFindToolRegistrations_TwoParamRegister(t *testing.T) {
	// Register with two parameters should be skipped.
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/twoparam"

	pkgDir := filepath.Join(tmpDir, "tools", "twoparam")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package twoparam

import "myapp/shipq/lib/llm"

func Register(app *llm.App, extra string) {
	app.Tool("should_skip", "not valid signature", Func)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools for two-param Register, got %d", len(tools))
	}
}

func TestFindToolRegistrations_TooFewArgs(t *testing.T) {
	// app.Tool() with fewer than 3 args should be silently skipped.
	tmpDir := t.TempDir()
	modulePath := "myapp"
	importPath := modulePath + "/tools/fewargs"

	pkgDir := filepath.Join(tmpDir, "tools", "fewargs")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package fewargs

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
	app.Tool("only_name")
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools for too-few args, got %d", len(tools))
	}
}

// TestFindToolRegistrations_MonorepoLayout verifies that FindToolRegistrations
// correctly locates tool source files in a monorepo layout where the first
// argument (rootDir) is the shipq project root and modulePath is the full
// import prefix (including the subpath from go.mod root to shipq root).
//
// This is a regression test for a bug where the function would strip the full
// import prefix from the package path, then join the remainder with goModRoot,
// producing an incorrect filesystem path. The fix is to pass shipqRoot (not
// goModRoot) as the first argument when modulePath is the full import prefix.
func TestFindToolRegistrations_MonorepoLayout(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "services", "myservice")

	rawModulePath := "github.com/company/monorepo"
	importPrefix := rawModulePath + "/services/myservice"
	importPath := importPrefix + "/tools/weather"

	// Create the package directory under shipqRoot (where the files actually live).
	pkgDir := filepath.Join(shipqRoot, "tools", "weather")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package weather

import "` + importPrefix + `/shipq/lib/llm"

type WeatherInput struct {
	City string
}

type WeatherOutput struct {
	Temp float64
}

func GetWeather(input *WeatherInput) (*WeatherOutput, error) {
	return nil, nil
}

func Register(app *llm.App) {
	app.Tool("get_weather", "Get the current weather for a city", GetWeather)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	// When modulePath is the full import prefix, rootDir must be shipqRoot
	// so that TrimPrefix + Join produces the correct filesystem path.
	tools, err := FindToolRegistrations(shipqRoot, importPrefix, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", tools[0].Name)
	}
	if tools[0].FuncName != "GetWeather" {
		t.Errorf("expected func name 'GetWeather', got %q", tools[0].FuncName)
	}
}

// TestFindToolRegistrations_MonorepoDeeplyNested verifies correct behaviour
// when the shipq root is several levels deep inside the monorepo.
func TestFindToolRegistrations_MonorepoDeeplyNested(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "packages", "services", "api", "backend")

	rawModulePath := "github.com/bigcorp/platform"
	importPrefix := rawModulePath + "/packages/services/api/backend"
	importPath := importPrefix + "/tools/analyze"

	pkgDir := filepath.Join(shipqRoot, "tools", "analyze")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package analyze

import "` + importPrefix + `/shipq/lib/llm"

type AnalysisInput struct {
	Query string
}

type AnalysisOutput struct {
	Result string
}

func RunAnalysis(input *AnalysisInput) (*AnalysisOutput, error) {
	return nil, nil
}

func Register(app *llm.App) {
	app.Tool("run_analysis", "Run analysis", RunAnalysis)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	tools, err := FindToolRegistrations(shipqRoot, importPrefix, importPath)
	if err != nil {
		t.Fatalf("FindToolRegistrations failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "run_analysis" {
		t.Errorf("expected tool name 'run_analysis', got %q", tools[0].Name)
	}
}

// TestFindToolRegistrations_MonorepoMultiplePackages verifies that multiple
// tool packages are correctly resolved in a monorepo layout.
func TestFindToolRegistrations_MonorepoMultiplePackages(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "apps", "backend")

	rawModulePath := "github.com/org/repo"
	importPrefix := rawModulePath + "/apps/backend"

	toolNames := []string{"weather", "calendar"}
	for _, name := range toolNames {
		pkgDir := filepath.Join(shipqRoot, "tools", name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatalf("failed to create package dir for %s: %v", name, err)
		}

		content := `package ` + name + `

import "` + importPrefix + `/shipq/lib/llm"

type Input struct{ Data string }
type Output struct{ Result string }

func Do(input *Input) (*Output, error) { return nil, nil }

func Register(app *llm.App) {
	app.Tool("do_` + name + `", "Do ` + name + `", Do)
}
`
		if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write register.go for %s: %v", name, err)
		}
	}

	for _, name := range toolNames {
		importPath := importPrefix + "/tools/" + name
		tools, err := FindToolRegistrations(shipqRoot, importPrefix, importPath)
		if err != nil {
			t.Fatalf("FindToolRegistrations for %s failed: %v", name, err)
		}

		if len(tools) != 1 {
			t.Fatalf("tool %s: expected 1 tool, got %d", name, len(tools))
		}

		expectedName := "do_" + name
		if tools[0].Name != expectedName {
			t.Errorf("tool %s: expected name %q, got %q", name, expectedName, tools[0].Name)
		}
	}
}
