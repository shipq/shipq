package llmgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen/llmcompile"
)

// TestGenerateToolRegistries_MonorepoLayout verifies that in a monorepo
// layout (go.mod at a parent directory, shipq.ini in a subdirectory), the
// generated zz_generated_registry.go files are written relative to shipqRoot
// (the directory containing shipq.ini), NOT relative to goModRoot (the
// directory containing go.mod).
//
// This is a regression test for a bug where the generated files were placed
// at the go.mod root instead of under the shipq project subdirectory.
func TestGenerateToolRegistries_MonorepoLayout(t *testing.T) {
	// Set up a monorepo-like directory structure:
	//   <tmp>/                     ← goModRoot (contains go.mod)
	//   <tmp>/services/myservice/  ← shipqRoot (contains shipq.ini)
	//   tools live under shipqRoot: services/myservice/tools/weather/
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "services", "myservice")
	if err := os.MkdirAll(shipqRoot, 0755); err != nil {
		t.Fatalf("failed to create shipqRoot: %v", err)
	}

	// The tool package directory must exist for EnsureDir to work.
	toolDir := filepath.Join(shipqRoot, "tools", "weather")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("failed to create tool dir: %v", err)
	}

	modulePath := "github.com/company/monorepo"
	// importPrefix includes the subpath: github.com/company/monorepo/services/myservice
	importPrefix := modulePath + "/services/myservice"

	toolPkg := llmcompile.SerializedToolPackage{
		PackagePath: importPrefix + "/tools/weather",
		PackageName: "weather",
		Tools: []llmcompile.SerializedToolInfo{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
				FuncName:    "GetWeather",
				PackagePath: importPrefix + "/tools/weather",
				PackageName: "weather",
				InputType:   "WeatherInput",
				OutputType:  "WeatherOutput",
			},
		},
	}

	cfg := GenerateAllLLMConfig{
		ToolPackages: []llmcompile.SerializedToolPackage{toolPkg},
		ModulePath:   importPrefix,
		GoModRoot:    goModRoot,
		ShipqRoot:    shipqRoot,
		DBDialect:    "sqlite",
		HasTenancy:   false,
		HasAuth:      false,
	}

	// Run only the tool registries generator (the part with the path bug).
	err := generateToolRegistries(cfg)
	if err != nil {
		t.Fatalf("generateToolRegistries failed: %v", err)
	}

	// The generated file MUST be under shipqRoot, NOT goModRoot.
	correctPath := filepath.Join(shipqRoot, "tools", "weather", "zz_generated_registry.go")
	if _, err := os.Stat(correctPath); os.IsNotExist(err) {
		t.Errorf("expected generated file at %s but it does not exist", correctPath)
	}

	// The file must NOT exist at the wrong location (goModRoot/tools/weather/).
	wrongPath := filepath.Join(goModRoot, "tools", "weather", "zz_generated_registry.go")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("generated file was placed at WRONG path %s (go.mod root) instead of under shipq root", wrongPath)
	}
}

// TestGenerateToolRegistries_StandardLayout verifies that in a standard
// (non-monorepo) layout where goModRoot == shipqRoot, files are written
// correctly.
func TestGenerateToolRegistries_StandardLayout(t *testing.T) {
	projectRoot := t.TempDir()

	toolDir := filepath.Join(projectRoot, "tools", "search")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("failed to create tool dir: %v", err)
	}

	modulePath := "myapp"

	toolPkg := llmcompile.SerializedToolPackage{
		PackagePath: modulePath + "/tools/search",
		PackageName: "search",
		Tools: []llmcompile.SerializedToolInfo{
			{
				Name:        "search_docs",
				Description: "Search documents",
				FuncName:    "SearchDocs",
				PackagePath: modulePath + "/tools/search",
				PackageName: "search",
				InputType:   "SearchInput",
				OutputType:  "SearchOutput",
			},
		},
	}

	cfg := GenerateAllLLMConfig{
		ToolPackages: []llmcompile.SerializedToolPackage{toolPkg},
		ModulePath:   modulePath,
		GoModRoot:    projectRoot,
		ShipqRoot:    projectRoot,
		DBDialect:    "sqlite",
		HasTenancy:   false,
		HasAuth:      false,
	}

	err := generateToolRegistries(cfg)
	if err != nil {
		t.Fatalf("generateToolRegistries failed: %v", err)
	}

	expectedPath := filepath.Join(projectRoot, "tools", "search", "zz_generated_registry.go")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected generated file at %s but it does not exist", expectedPath)
	}
}

// TestGenerateToolRegistries_MonorepoMultiplePackages verifies correct
// placement of generated files when multiple tool packages exist in a monorepo.
func TestGenerateToolRegistries_MonorepoMultiplePackages(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "apps", "backend")
	modulePath := "github.com/org/repo"
	importPrefix := modulePath + "/apps/backend"

	toolNames := []string{"weather", "calendar", "email"}

	for _, name := range toolNames {
		dir := filepath.Join(shipqRoot, "tools", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create tool dir for %s: %v", name, err)
		}
	}

	var toolPkgs []llmcompile.SerializedToolPackage
	for _, name := range toolNames {
		toolPkgs = append(toolPkgs, llmcompile.SerializedToolPackage{
			PackagePath: importPrefix + "/tools/" + name,
			PackageName: name,
			Tools: []llmcompile.SerializedToolInfo{
				{
					Name:        "do_" + name,
					Description: "Do " + name,
					FuncName:    "Do",
					PackagePath: importPrefix + "/tools/" + name,
					PackageName: name,
					InputType:   "Input",
					OutputType:  "Output",
				},
			},
		})
	}

	cfg := GenerateAllLLMConfig{
		ToolPackages: toolPkgs,
		ModulePath:   importPrefix,
		GoModRoot:    goModRoot,
		ShipqRoot:    shipqRoot,
		DBDialect:    "sqlite",
		HasTenancy:   false,
		HasAuth:      false,
	}

	err := generateToolRegistries(cfg)
	if err != nil {
		t.Fatalf("generateToolRegistries failed: %v", err)
	}

	for _, name := range toolNames {
		correctPath := filepath.Join(shipqRoot, "tools", name, "zz_generated_registry.go")
		if _, err := os.Stat(correctPath); os.IsNotExist(err) {
			t.Errorf("tool %q: expected generated file at %s but it does not exist", name, correctPath)
		}

		wrongPath := filepath.Join(goModRoot, "tools", name, "zz_generated_registry.go")
		if _, err := os.Stat(wrongPath); err == nil {
			t.Errorf("tool %q: generated file at WRONG path %s (go.mod root)", name, wrongPath)
		}
	}
}

// TestGenerateToolRegistries_MonorepoDeeplyNested verifies correct behavior
// when the shipq root is several levels deep inside the monorepo.
func TestGenerateToolRegistries_MonorepoDeeplyNested(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "packages", "services", "api", "backend")
	modulePath := "github.com/bigcorp/platform"
	importPrefix := modulePath + "/packages/services/api/backend"

	toolDir := filepath.Join(shipqRoot, "tools", "analyze")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("failed to create tool dir: %v", err)
	}

	toolPkg := llmcompile.SerializedToolPackage{
		PackagePath: importPrefix + "/tools/analyze",
		PackageName: "analyze",
		Tools: []llmcompile.SerializedToolInfo{
			{
				Name:        "run_analysis",
				Description: "Run analysis",
				FuncName:    "RunAnalysis",
				PackagePath: importPrefix + "/tools/analyze",
				PackageName: "analyze",
				InputType:   "AnalysisInput",
				OutputType:  "AnalysisOutput",
			},
		},
	}

	cfg := GenerateAllLLMConfig{
		ToolPackages: []llmcompile.SerializedToolPackage{toolPkg},
		ModulePath:   importPrefix,
		GoModRoot:    goModRoot,
		ShipqRoot:    shipqRoot,
		DBDialect:    "sqlite",
		HasTenancy:   false,
		HasAuth:      false,
	}

	err := generateToolRegistries(cfg)
	if err != nil {
		t.Fatalf("generateToolRegistries failed: %v", err)
	}

	correctPath := filepath.Join(shipqRoot, "tools", "analyze", "zz_generated_registry.go")
	if _, err := os.Stat(correctPath); os.IsNotExist(err) {
		t.Errorf("expected generated file at %s but it does not exist", correctPath)
	}

	// Ensure no file was created relative to goModRoot
	wrongPath := filepath.Join(goModRoot, "tools", "analyze", "zz_generated_registry.go")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("generated file at WRONG path %s", wrongPath)
	}
}

// TestDetectLLMChannels_MonorepoLayout verifies that DetectLLMChannels
// correctly locates channel source files in a monorepo layout where
// goModRoot != shipqRoot.
//
// This is a regression test: the function used to strip the full import
// prefix (including subpath) from the package path, then join the remainder
// with goModRoot, producing an incorrect filesystem path.
func TestDetectLLMChannels_MonorepoLayout(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "services", "myservice")

	modulePath := "github.com/company/monorepo"
	importPrefix := modulePath + "/services/myservice"

	// Create the channel package directory under shipqRoot.
	channelDir := filepath.Join(shipqRoot, "channels", "assistant")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channel dir: %v", err)
	}

	// Write a Go file that imports and uses llm.WithClient.
	channelCode := `package assistant

import (
	"context"

	"` + importPrefix + `/shipq/lib/llm"
)

func Setup(ctx context.Context) context.Context {
	return llm.WithClient(ctx, nil)
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "setup.go"), []byte(channelCode), 0644); err != nil {
		t.Fatalf("failed to write channel file: %v", err)
	}

	channelPkg := importPrefix + "/channels/assistant"

	// DetectLLMChannels should find the channel even when goModRoot != shipqRoot.
	// The function receives goModRoot and modulePath; it must correctly resolve
	// the filesystem path.
	result, err := DetectLLMChannels(shipqRoot, importPrefix, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 LLM channel, got %d", len(result))
	}

	if result[0] != channelPkg {
		t.Errorf("expected %q, got %q", channelPkg, result[0])
	}
}

// TestDetectLLMChannels_MonorepoNoLLM verifies that a channel without LLM
// usage is not detected as LLM-enabled in a monorepo layout.
func TestDetectLLMChannels_MonorepoNoLLM(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "services", "myservice")

	modulePath := "github.com/company/monorepo"
	importPrefix := modulePath + "/services/myservice"

	channelDir := filepath.Join(shipqRoot, "channels", "notifications")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channel dir: %v", err)
	}

	// Write a Go file that does NOT use LLM.
	channelCode := `package notifications

type Alert struct {
	Message string
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "types.go"), []byte(channelCode), 0644); err != nil {
		t.Fatalf("failed to write channel file: %v", err)
	}

	channelPkg := importPrefix + "/channels/notifications"

	result, err := DetectLLMChannels(shipqRoot, importPrefix, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels, got %d: %v", len(result), result)
	}
}
