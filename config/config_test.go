package config

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/inifile"
)

func parseINI(t *testing.T, content string) *inifile.File {
	t.Helper()
	f, err := inifile.Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to parse INI: %v", err)
	}
	return f
}

func TestParseLLMConfig_SinglePackage(t *testing.T) {
	ini := parseINI(t, `
[llm]
tool_pkgs = myapp/tools/weather
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 1 {
		t.Fatalf("expected 1 tool package, got %d", len(cfg.ToolPkgs))
	}
	if cfg.ToolPkgs[0] != "myapp/tools/weather" {
		t.Errorf("expected 'myapp/tools/weather', got %q", cfg.ToolPkgs[0])
	}
}

func TestParseLLMConfig_MultiplePackages(t *testing.T) {
	ini := parseINI(t, `
[llm]
tool_pkgs = myapp/tools/weather, myapp/tools/calendar
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 2 {
		t.Fatalf("expected 2 tool packages, got %d", len(cfg.ToolPkgs))
	}
	if cfg.ToolPkgs[0] != "myapp/tools/weather" {
		t.Errorf("expected 'myapp/tools/weather', got %q", cfg.ToolPkgs[0])
	}
	if cfg.ToolPkgs[1] != "myapp/tools/calendar" {
		t.Errorf("expected 'myapp/tools/calendar', got %q", cfg.ToolPkgs[1])
	}
}

func TestParseLLMConfig_MultiplePackagesExtraWhitespace(t *testing.T) {
	ini := parseINI(t, `
[llm]
tool_pkgs =   myapp/tools/a ,  myapp/tools/b  ,myapp/tools/c
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 3 {
		t.Fatalf("expected 3 tool packages, got %d", len(cfg.ToolPkgs))
	}

	expected := []string{"myapp/tools/a", "myapp/tools/b", "myapp/tools/c"}
	for i, want := range expected {
		if cfg.ToolPkgs[i] != want {
			t.Errorf("tool_pkgs[%d]: expected %q, got %q", i, want, cfg.ToolPkgs[i])
		}
	}
}

func TestParseLLMConfig_EmptyToolPkgs(t *testing.T) {
	ini := parseINI(t, `
[llm]
tool_pkgs =
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 0 {
		t.Errorf("expected empty tool packages, got %d: %v", len(cfg.ToolPkgs), cfg.ToolPkgs)
	}
}

func TestParseLLMConfig_SectionWithoutToolPkgs(t *testing.T) {
	ini := parseINI(t, `
[llm]
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 0 {
		t.Errorf("expected empty tool packages, got %d: %v", len(cfg.ToolPkgs), cfg.ToolPkgs)
	}
}

func TestParseLLMConfig_MissingSection(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = sqlite:test.db
`)

	cfg := ParseLLMConfig(ini)
	if cfg != nil {
		t.Errorf("expected nil LLMConfig when [llm] section is missing, got %+v", cfg)
	}
}

func TestParseLLMConfig_EmptyINI(t *testing.T) {
	ini := parseINI(t, ``)

	cfg := ParseLLMConfig(ini)
	if cfg != nil {
		t.Errorf("expected nil LLMConfig for empty INI, got %+v", cfg)
	}
}

func TestParseLLMConfig_TrailingCommas(t *testing.T) {
	ini := parseINI(t, `
[llm]
tool_pkgs = myapp/tools/weather,
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	// Trailing comma should not produce an empty string entry
	if len(cfg.ToolPkgs) != 1 {
		t.Fatalf("expected 1 tool package (trailing comma ignored), got %d: %v", len(cfg.ToolPkgs), cfg.ToolPkgs)
	}
	if cfg.ToolPkgs[0] != "myapp/tools/weather" {
		t.Errorf("expected 'myapp/tools/weather', got %q", cfg.ToolPkgs[0])
	}
}

func TestParseLLMConfig_OnlyCommas(t *testing.T) {
	ini := parseINI(t, `
[llm]
tool_pkgs = , , ,
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 0 {
		t.Errorf("expected empty tool packages for comma-only value, got %d: %v", len(cfg.ToolPkgs), cfg.ToolPkgs)
	}
}

func TestParseLLMConfig_OtherSectionsPresent(t *testing.T) {
	ini := parseINI(t, `
[db]
database_url = sqlite:test.db

[workers]
redis_url = redis://localhost:6379

[llm]
tool_pkgs = myapp/tools/search

[typescript]
framework = react
`)

	cfg := ParseLLMConfig(ini)
	if cfg == nil {
		t.Fatal("expected non-nil LLMConfig")
	}

	if len(cfg.ToolPkgs) != 1 {
		t.Fatalf("expected 1 tool package, got %d", len(cfg.ToolPkgs))
	}
	if cfg.ToolPkgs[0] != "myapp/tools/search" {
		t.Errorf("expected 'myapp/tools/search', got %q", cfg.ToolPkgs[0])
	}
}
