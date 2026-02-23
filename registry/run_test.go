package registry

import (
	"strings"
	"testing"

	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	"github.com/shipq/shipq/inifile"
)

// ── ParseFrameworks tests ────────────────────────────────────────────────────

func TestParseFrameworks_Default(t *testing.T) {
	got := ParseFrameworks("")
	if len(got) != 1 || got[0] != "react" {
		t.Errorf("ParseFrameworks(\"\") = %v, want [react]", got)
	}
}

func TestParseFrameworks_React(t *testing.T) {
	got := ParseFrameworks("react")
	if len(got) != 1 || got[0] != "react" {
		t.Errorf("ParseFrameworks(\"react\") = %v, want [react]", got)
	}
}

func TestParseFrameworks_Svelte(t *testing.T) {
	got := ParseFrameworks("svelte")
	if len(got) != 1 || got[0] != "svelte" {
		t.Errorf("ParseFrameworks(\"svelte\") = %v, want [svelte]", got)
	}
}

func TestParseFrameworks_Both(t *testing.T) {
	got := ParseFrameworks("react,svelte")
	if len(got) != 2 || got[0] != "react" || got[1] != "svelte" {
		t.Errorf("ParseFrameworks(\"react,svelte\") = %v, want [react svelte]", got)
	}
}

func TestParseFrameworks_BothReversed(t *testing.T) {
	got := ParseFrameworks("svelte,react")
	if len(got) != 2 || got[0] != "svelte" || got[1] != "react" {
		t.Errorf("ParseFrameworks(\"svelte,react\") = %v, want [svelte react]", got)
	}
}

func TestParseFrameworks_WhitespaceHandling(t *testing.T) {
	got := ParseFrameworks("  react , svelte  ")
	if len(got) != 2 || got[0] != "react" || got[1] != "svelte" {
		t.Errorf("ParseFrameworks(\"  react , svelte  \") = %v, want [react svelte]", got)
	}
}

func TestParseFrameworks_CaseInsensitive(t *testing.T) {
	got := ParseFrameworks("React,SVELTE")
	if len(got) != 2 || got[0] != "react" || got[1] != "svelte" {
		t.Errorf("ParseFrameworks(\"React,SVELTE\") = %v, want [react svelte]", got)
	}
}

func TestParseFrameworks_UnknownDropped(t *testing.T) {
	got := ParseFrameworks("react,vue,svelte")
	if len(got) != 2 || got[0] != "react" || got[1] != "svelte" {
		t.Errorf("ParseFrameworks(\"react,vue,svelte\") = %v, want [react svelte]", got)
	}
}

func TestParseFrameworks_AllUnknownFallsBackToReact(t *testing.T) {
	got := ParseFrameworks("vue,angular")
	if len(got) != 1 || got[0] != "react" {
		t.Errorf("ParseFrameworks(\"vue,angular\") = %v, want [react]", got)
	}
}

// ── HasFramework tests ───────────────────────────────────────────────────────

func TestHasFramework_Present(t *testing.T) {
	if !HasFramework([]string{"react", "svelte"}, "react") {
		t.Error("HasFramework([react svelte], react) should be true")
	}
	if !HasFramework([]string{"react", "svelte"}, "svelte") {
		t.Error("HasFramework([react svelte], svelte) should be true")
	}
}

func TestHasFramework_Absent(t *testing.T) {
	if HasFramework([]string{"react"}, "svelte") {
		t.Error("HasFramework([react], svelte) should be false")
	}
}

func TestHasFramework_Empty(t *testing.T) {
	if HasFramework(nil, "react") {
		t.Error("HasFramework(nil, react) should be false")
	}
}

// ── INI integration: [typescript] section parsed correctly ───────────────────

func TestParseFrameworks_FromIni(t *testing.T) {
	input := "[typescript]\nframework = svelte\nhttp_output = frontend/src/lib\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	got := ParseFrameworks(ini.Get("typescript", "framework"))
	if len(got) != 1 || got[0] != "svelte" {
		t.Errorf("expected [svelte], got %v", got)
	}
}

func TestParseFrameworks_FromIni_CommaSeparated(t *testing.T) {
	input := "[typescript]\nframework = react,svelte\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	got := ParseFrameworks(ini.Get("typescript", "framework"))
	if len(got) != 2 || got[0] != "react" || got[1] != "svelte" {
		t.Errorf("expected [react svelte], got %v", got)
	}
}

func TestParseFrameworks_FromIni_MissingSection(t *testing.T) {
	input := "[db]\ndatabase_url = sqlite:///tmp/test.db\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	// ini.Get returns "" when the section/key doesn't exist
	got := ParseFrameworks(ini.Get("typescript", "framework"))
	if len(got) != 1 || got[0] != "react" {
		t.Errorf("expected default [react] when section missing, got %v", got)
	}
}

func TestParseCustomEnvVars_NilWhenNoEnvSection(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[db]\ndatabase_url = sqlite:///tmp/test.db\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if vars != nil {
		t.Errorf("expected nil when no [env] section, got %v", vars)
	}
}

func TestParseCustomEnvVars_EmptySection(t *testing.T) {
	ini, err := inifile.Parse(strings.NewReader("[env]\n"))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 0 {
		t.Errorf("expected empty slice for empty [env] section, got %d entries", len(vars))
	}
}

func TestParseCustomEnvVars_RequiredAndOptional(t *testing.T) {
	input := "[env]\nOPENAI_API_KEY = required\nANTHROPIC_API_KEY = optional\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(vars))
	}

	expected := []configpkg.CustomEnvVar{
		{Name: "OPENAI_API_KEY", Required: true},
		{Name: "ANTHROPIC_API_KEY", Required: false},
	}

	for i, ev := range expected {
		if vars[i].Name != ev.Name {
			t.Errorf("vars[%d].Name = %q, want %q", i, vars[i].Name, ev.Name)
		}
		if vars[i].Required != ev.Required {
			t.Errorf("vars[%d].Required = %v, want %v", i, vars[i].Required, ev.Required)
		}
	}
}

func TestParseCustomEnvVars_KeysAreUppercased(t *testing.T) {
	// The ini parser lowercases keys, so "openai_api_key" is stored lowercase.
	// ParseCustomEnvVars should uppercase them back to env var convention.
	input := "[env]\nopenai_api_key = required\nmy_setting = optional\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(vars))
	}

	if vars[0].Name != "OPENAI_API_KEY" {
		t.Errorf("vars[0].Name = %q, want %q", vars[0].Name, "OPENAI_API_KEY")
	}
	if vars[1].Name != "MY_SETTING" {
		t.Errorf("vars[1].Name = %q, want %q", vars[1].Name, "MY_SETTING")
	}
}

func TestParseCustomEnvVars_UnknownValuesTreatedAsOptional(t *testing.T) {
	// Any value other than "required" should be treated as optional.
	input := "[env]\nAPI_KEY = required\nWEBHOOK_URL = something_else\nDEBUG_MODE = yes\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(vars))
	}

	if !vars[0].Required {
		t.Error("API_KEY should be required")
	}
	if vars[1].Required {
		t.Error("WEBHOOK_URL with value 'something_else' should be treated as optional")
	}
	if vars[2].Required {
		t.Error("DEBUG_MODE with value 'yes' should be treated as optional")
	}
}

func TestParseCustomEnvVars_RequiredIsCaseInsensitive(t *testing.T) {
	input := "[env]\nKEY_A = Required\nKEY_B = REQUIRED\nKEY_C = required\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(vars))
	}

	for i, v := range vars {
		if !v.Required {
			t.Errorf("vars[%d] (%s) should be required (value was case-insensitive 'required')", i, v.Name)
		}
	}
}

func TestParseCustomEnvVars_PreservesOrder(t *testing.T) {
	input := "[env]\nZZZ_LAST = required\nAAA_FIRST = optional\nMMM_MIDDLE = required\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(vars))
	}

	expectedNames := []string{"ZZZ_LAST", "AAA_FIRST", "MMM_MIDDLE"}
	for i, name := range expectedNames {
		if vars[i].Name != name {
			t.Errorf("vars[%d].Name = %q, want %q (order should match ini file)", i, vars[i].Name, name)
		}
	}
}

func TestParseCustomEnvVars_CoexistsWithOtherSections(t *testing.T) {
	input := "[db]\ndatabase_url = sqlite:///tmp/test.db\n\n[auth]\nprotect_by_default = true\n\n[env]\nSECRET_KEY = required\n\n[workers]\nredis_url = redis://localhost:6379\n"
	ini, err := inifile.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	vars := ParseCustomEnvVars(ini)
	if len(vars) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(vars))
	}
	if vars[0].Name != "SECRET_KEY" {
		t.Errorf("vars[0].Name = %q, want %q", vars[0].Name, "SECRET_KEY")
	}
	if !vars[0].Required {
		t.Error("SECRET_KEY should be required")
	}
}
