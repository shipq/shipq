package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ── Test input/output types ───────────────────────────────────────────────────

type weatherInput struct {
	City    string `json:"city"    desc:"The city to get weather for"`
	Country string `json:"country" desc:"ISO country code, e.g. US"`
}

type weatherOutput struct {
	TempC       float64 `json:"temp_c"`
	Description string  `json:"description"`
}

type parseDateInput struct {
	DateStr string `json:"date_str" desc:"The date string to parse"`
}

type parseDateOutput struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type nestedAddress struct {
	Street string `json:"street" desc:"Street address"`
	City   string `json:"city"   desc:"City name"`
}

type nestedInput struct {
	Name    string        `json:"name"    desc:"Person name"`
	Address nestedAddress `json:"address" desc:"Mailing address"`
}

type nestedOutput struct {
	OK bool `json:"ok"`
}

type optionalFieldsInput struct {
	Required string  `json:"required" desc:"A required field"`
	Optional *string `json:"optional" desc:"An optional field"`
	Count    *int    `json:"count"    desc:"An optional count"`
}

type optionalFieldsOutput struct {
	Result string `json:"result"`
}

type emptyInput struct{}

type emptyOutput struct{}

// ── Sample tool functions ─────────────────────────────────────────────────────

func getWeather(ctx context.Context, input *weatherInput) (*weatherOutput, error) {
	return &weatherOutput{TempC: 22.5, Description: "Sunny in " + input.City}, nil
}

func parseDate(_ *parseDateInput) (*parseDateOutput, error) {
	return &parseDateOutput{Year: 2025, Month: 7, Day: 4}, nil
}

func failingWeatherTool(ctx context.Context, input *weatherInput) (*weatherOutput, error) {
	return nil, errors.New("tool failed: city not found")
}

func nestedTool(_ *nestedInput) (*nestedOutput, error) {
	return &nestedOutput{OK: true}, nil
}

func optionalFieldsTool(_ *optionalFieldsInput) (*optionalFieldsOutput, error) {
	return &optionalFieldsOutput{Result: "done"}, nil
}

func emptyTool(_ *emptyInput) (*emptyOutput, error) {
	return &emptyOutput{}, nil
}

// ── Valid signature tests ─────────────────────────────────────────────────────

func TestTool_ValidSignatureWithContext(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get the current weather for a city", getWeather)

	if len(app.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(app.Tools))
	}

	td := app.Tools[0]
	if td.Name != "get_weather" {
		t.Errorf("name: got %q, want %q", td.Name, "get_weather")
	}
	if td.Description != "Get the current weather for a city" {
		t.Errorf("description: got %q", td.Description)
	}
	if td.Func == nil {
		t.Fatal("Func is nil")
	}
	if td.InputSchema == nil {
		t.Fatal("InputSchema is nil")
	}

	// Verify the schema looks correct at a high level.
	var schema map[string]any
	if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("schema type: got %v, want \"object\"", schema["type"])
	}
}

func TestTool_ValidSignatureWithoutContext(t *testing.T) {
	app := NewApp()
	app.Tool("parse_date", "Parse a date string", parseDate)

	if len(app.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(app.Tools))
	}

	td := app.Tools[0]
	if td.Name != "parse_date" {
		t.Errorf("name: got %q, want %q", td.Name, "parse_date")
	}
	if td.Func == nil {
		t.Fatal("Func is nil")
	}
}

// ── Panic tests ───────────────────────────────────────────────────────────────

func assertPanics(t *testing.T, name string, wantSubstring string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic, but did not panic", name)
		}
		msg := fmt.Sprintf("%v", r)
		if !strings.Contains(msg, wantSubstring) {
			t.Errorf("%s: panic message %q does not contain %q", name, msg, wantSubstring)
		}
	}()
	fn()
}

func TestTool_PanicOnBadArgCount_Zero(t *testing.T) {
	assertPanics(t, "zero args", "must take 1 or 2 arguments, got 0", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func() (*emptyOutput, error) {
			return nil, nil
		})
	})
}

func TestTool_PanicOnBadArgCount_Three(t *testing.T) {
	assertPanics(t, "three args", "must take 1 or 2 arguments, got 3", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(ctx context.Context, a *weatherInput, b *weatherInput) (*weatherOutput, error) {
			return nil, nil
		})
	})
}

func TestTool_PanicOnNonPointerInput(t *testing.T) {
	assertPanics(t, "non-pointer input", "last argument must be a pointer to a struct", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(ctx context.Context, input weatherInput) (*weatherOutput, error) {
			return nil, nil
		})
	})
}

func TestTool_PanicOnNonStructInput(t *testing.T) {
	assertPanics(t, "non-struct pointer input", "last argument must be a pointer to a struct", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(ctx context.Context, input *string) (*weatherOutput, error) {
			return nil, nil
		})
	})
}

func TestTool_PanicOnBadReturnCount_Zero(t *testing.T) {
	assertPanics(t, "zero returns", "must return exactly 2 values, got 0", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(input *weatherInput) {
		})
	})
}

func TestTool_PanicOnBadReturnCount_One(t *testing.T) {
	assertPanics(t, "one return", "must return exactly 2 values, got 1", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(input *weatherInput) error {
			return nil
		})
	})
}

func TestTool_PanicOnBadReturnCount_Three(t *testing.T) {
	assertPanics(t, "three returns", "must return exactly 2 values, got 3", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(input *weatherInput) (*weatherOutput, int, error) {
			return nil, 0, nil
		})
	})
}

func TestTool_PanicOnNonPointerOutput(t *testing.T) {
	assertPanics(t, "non-pointer output", "first return value must be a pointer to a struct", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(input *weatherInput) (weatherOutput, error) {
			return weatherOutput{}, nil
		})
	})
}

func TestTool_PanicOnNonStructOutput(t *testing.T) {
	assertPanics(t, "non-struct pointer output", "first return value must be a pointer to a struct", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(input *weatherInput) (*string, error) {
			return nil, nil
		})
	})
}

func TestTool_PanicOnNonErrorReturn(t *testing.T) {
	assertPanics(t, "non-error second return", "second return value must implement error", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(input *weatherInput) (*weatherOutput, string) {
			return nil, ""
		})
	})
}

func TestTool_PanicOnNotAFunction(t *testing.T) {
	assertPanics(t, "not a function", "expected a function", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", "not a function")
	})
}

func TestTool_PanicOnTwoArgsFirstNotContext(t *testing.T) {
	assertPanics(t, "first arg not context", "first must implement context.Context", func() {
		app := NewApp()
		app.Tool("bad", "bad tool", func(a string, input *weatherInput) (*weatherOutput, error) {
			return nil, nil
		})
	})
}

// ── Schema generation tests ──────────────────────────────────────────────────

func TestTool_SchemaGeneration(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get the current weather", getWeather)

	td := app.Tools[0]
	var schema map[string]any
	if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	// Type should be "object".
	if schema["type"] != "object" {
		t.Errorf("type: got %v, want \"object\"", schema["type"])
	}

	// additionalProperties should be false.
	if ap, ok := schema["additionalProperties"].(bool); !ok || ap != false {
		t.Errorf("additionalProperties: got %v, want false", schema["additionalProperties"])
	}

	// Properties should contain "city" and "country".
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties: expected map[string]any, got %T", schema["properties"])
	}

	// Check "city" property.
	cityProp, ok := props["city"].(map[string]any)
	if !ok {
		t.Fatalf("property 'city' not found or not a map")
	}
	if cityProp["type"] != "string" {
		t.Errorf("city type: got %v, want \"string\"", cityProp["type"])
	}
	if cityProp["description"] != "The city to get weather for" {
		t.Errorf("city description: got %v", cityProp["description"])
	}

	// Check "country" property.
	countryProp, ok := props["country"].(map[string]any)
	if !ok {
		t.Fatalf("property 'country' not found or not a map")
	}
	if countryProp["type"] != "string" {
		t.Errorf("country type: got %v, want \"string\"", countryProp["type"])
	}
	if countryProp["description"] != "ISO country code, e.g. US" {
		t.Errorf("country description: got %v", countryProp["description"])
	}

	// Required should contain both fields.
	reqRaw, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("required: expected []any, got %T", schema["required"])
	}
	reqSet := make(map[string]bool)
	for _, v := range reqRaw {
		reqSet[v.(string)] = true
	}
	if !reqSet["city"] {
		t.Error("\"city\" not in required")
	}
	if !reqSet["country"] {
		t.Error("\"country\" not in required")
	}
}

// ── Dispatch tests ───────────────────────────────────────────────────────────

func TestTool_Dispatch(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get weather", getWeather)

	td := app.Tools[0]
	argsJSON := []byte(`{"city":"London","country":"GB"}`)
	result, err := td.Func(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	var output weatherOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.TempC != 22.5 {
		t.Errorf("temp_c: got %f, want 22.5", output.TempC)
	}
	if output.Description != "Sunny in London" {
		t.Errorf("description: got %q, want %q", output.Description, "Sunny in London")
	}
}

func TestTool_DispatchError(t *testing.T) {
	app := NewApp()
	app.Tool("failing", "A tool that fails", failingWeatherTool)

	td := app.Tools[0]
	argsJSON := []byte(`{"city":"Atlantis","country":"XX"}`)
	result, err := td.Func(context.Background(), argsJSON)
	if err == nil {
		t.Fatal("expected error from failing tool")
	}
	if !strings.Contains(err.Error(), "city not found") {
		t.Errorf("error message: got %q, want to contain \"city not found\"", err.Error())
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %s", string(result))
	}
}

func TestTool_DispatchBadJSON(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get weather", getWeather)

	td := app.Tools[0]
	_, err := td.Func(context.Background(), []byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error from bad JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error message: got %q, want to contain \"unmarshal\"", err.Error())
	}
}

func TestTool_DispatchWithoutContext(t *testing.T) {
	// Register a no-context function, dispatch with a context — context is NOT
	// passed to the function, but dispatch still works.
	app := NewApp()
	app.Tool("parse_date", "Parse a date", parseDate)

	td := app.Tools[0]
	argsJSON := []byte(`{"date_str":"2025-07-04"}`)
	result, err := td.Func(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	var output parseDateOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.Year != 2025 {
		t.Errorf("year: got %d, want 2025", output.Year)
	}
	if output.Month != 7 {
		t.Errorf("month: got %d, want 7", output.Month)
	}
	if output.Day != 4 {
		t.Errorf("day: got %d, want 4", output.Day)
	}
}

func TestTool_DispatchContextPassedThrough(t *testing.T) {
	// Verify that the context passed to ToolFunc is forwarded to the tool function.
	type ctxKey struct{}

	toolFn := func(ctx context.Context, input *emptyInput) (*emptyOutput, error) {
		val, ok := ctx.Value(ctxKey{}).(string)
		if !ok || val != "test-value" {
			return nil, fmt.Errorf("expected context value \"test-value\", got %q (ok=%v)", val, ok)
		}
		return &emptyOutput{}, nil
	}

	app := NewApp()
	app.Tool("ctx_test", "Test context passing", toolFn)

	ctx := context.WithValue(context.Background(), ctxKey{}, "test-value")
	_, err := app.Tools[0].Func(ctx, []byte(`{}`))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
}

// ── Nested struct tests ──────────────────────────────────────────────────────

func TestTool_NestedStruct(t *testing.T) {
	app := NewApp()
	app.Tool("nested", "A tool with nested input", nestedTool)

	td := app.Tools[0]
	var schema map[string]any
	if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties: expected map, got %T", schema["properties"])
	}

	addrProp, ok := props["address"].(map[string]any)
	if !ok {
		t.Fatalf("address property not found or not a map")
	}
	if addrProp["type"] != "object" {
		t.Errorf("address type: got %v, want \"object\"", addrProp["type"])
	}

	// Check nested properties.
	addrProps, ok := addrProp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("address.properties: expected map, got %T", addrProp["properties"])
	}
	if _, ok := addrProps["street"]; !ok {
		t.Error("address.properties missing \"street\"")
	}
	if _, ok := addrProps["city"]; !ok {
		t.Error("address.properties missing \"city\"")
	}

	// Verify dispatch works with nested JSON.
	argsJSON := []byte(`{"name":"Alice","address":{"street":"123 Main St","city":"Springfield"}}`)
	result, err := td.Func(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	var out nestedOutput
	json.Unmarshal(result, &out)
	if !out.OK {
		t.Error("expected OK=true")
	}
}

// ── Optional (pointer) fields tests ──────────────────────────────────────────

func TestTool_OptionalFields(t *testing.T) {
	app := NewApp()
	app.Tool("optional", "A tool with optional fields", optionalFieldsTool)

	td := app.Tools[0]
	var schema map[string]any
	if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	props := schema["properties"].(map[string]any)

	// "required" field should be a plain string type.
	reqProp := props["required"].(map[string]any)
	if reqProp["type"] != "string" {
		t.Errorf("required.type: got %v, want \"string\"", reqProp["type"])
	}

	// "optional" field should be a nullable string (["string", "null"]).
	optProp := props["optional"].(map[string]any)
	optTypes, ok := optProp["type"].([]any)
	if !ok {
		t.Fatalf("optional.type: expected []any, got %T %v", optProp["type"], optProp["type"])
	}
	if len(optTypes) != 2 {
		t.Fatalf("optional.type: expected 2 elements, got %d", len(optTypes))
	}
	if optTypes[0] != "string" || optTypes[1] != "null" {
		t.Errorf("optional.type: got %v, want [\"string\", \"null\"]", optTypes)
	}

	// "count" field should be a nullable integer (["integer", "null"]).
	countProp := props["count"].(map[string]any)
	countTypes, ok := countProp["type"].([]any)
	if !ok {
		t.Fatalf("count.type: expected []any, got %T %v", countProp["type"], countProp["type"])
	}
	if countTypes[0] != "integer" || countTypes[1] != "null" {
		t.Errorf("count.type: got %v, want [\"integer\", \"null\"]", countTypes)
	}

	// All fields (including pointer fields) should be in "required".
	reqRaw := schema["required"].([]any)
	reqSet := make(map[string]bool)
	for _, v := range reqRaw {
		reqSet[v.(string)] = true
	}
	if !reqSet["required"] {
		t.Error("\"required\" not in required array")
	}
	if !reqSet["optional"] {
		t.Error("\"optional\" not in required array")
	}
	if !reqSet["count"] {
		t.Error("\"count\" not in required array")
	}
}

// ── Multiple tools tests ─────────────────────────────────────────────────────

func TestTool_MultipleTools(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get weather", getWeather)
	app.Tool("parse_date", "Parse date", parseDate)

	if len(app.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(app.Tools))
	}
	if app.Tools[0].Name != "get_weather" {
		t.Errorf("tools[0].Name: got %q, want \"get_weather\"", app.Tools[0].Name)
	}
	if app.Tools[1].Name != "parse_date" {
		t.Errorf("tools[1].Name: got %q, want \"parse_date\"", app.Tools[1].Name)
	}

	// Registry should contain both.
	reg := app.Registry()
	if len(reg.Tools) != 2 {
		t.Fatalf("registry: expected 2 tools, got %d", len(reg.Tools))
	}

	// FindTool should work on the registry.
	if reg.FindTool("get_weather") == nil {
		t.Error("registry.FindTool(\"get_weather\") returned nil")
	}
	if reg.FindTool("parse_date") == nil {
		t.Error("registry.FindTool(\"parse_date\") returned nil")
	}
	if reg.FindTool("nonexistent") != nil {
		t.Error("registry.FindTool(\"nonexistent\") should return nil")
	}
}

// ── Duplicate name tests ─────────────────────────────────────────────────────

func TestTool_DuplicateName(t *testing.T) {
	assertPanics(t, "duplicate name", "duplicate tool name", func() {
		app := NewApp()
		app.Tool("get_weather", "Get weather", getWeather)
		app.Tool("get_weather", "Get weather again", getWeather)
	})
}

// ── Chaining tests ───────────────────────────────────────────────────────────

func TestTool_Chaining(t *testing.T) {
	app := NewApp()
	result := app.
		Tool("get_weather", "Get weather", getWeather).
		Tool("parse_date", "Parse date", parseDate)

	if result != app {
		t.Error("Tool() should return the same *App for chaining")
	}
	if len(app.Tools) != 2 {
		t.Fatalf("expected 2 tools after chaining, got %d", len(app.Tools))
	}
}

// ── Registry convenience method tests ────────────────────────────────────────

func TestApp_Registry_Empty(t *testing.T) {
	app := NewApp()
	reg := app.Registry()
	if reg == nil {
		t.Fatal("Registry() returned nil")
	}
	if len(reg.Tools) != 0 {
		t.Errorf("expected 0 tools in empty registry, got %d", len(reg.Tools))
	}
}

func TestApp_Registry_ToolsMatch(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get weather", getWeather)

	reg := app.Registry()
	if len(reg.Tools) != 1 {
		t.Fatalf("expected 1 tool in registry, got %d", len(reg.Tools))
	}
	if reg.Tools[0].Name != "get_weather" {
		t.Errorf("registry tool name: got %q, want \"get_weather\"", reg.Tools[0].Name)
	}
	if reg.Tools[0].Description != "Get weather" {
		t.Errorf("registry tool description: got %q", reg.Tools[0].Description)
	}
}

// ── Schema for without-context function ──────────────────────────────────────

func TestTool_SchemaGeneration_WithoutContext(t *testing.T) {
	app := NewApp()
	app.Tool("parse_date", "Parse a date string", parseDate)

	td := app.Tools[0]
	var schema map[string]any
	if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("type: got %v, want \"object\"", schema["type"])
	}

	props := schema["properties"].(map[string]any)
	dateProp, ok := props["date_str"].(map[string]any)
	if !ok {
		t.Fatal("property 'date_str' not found")
	}
	if dateProp["type"] != "string" {
		t.Errorf("date_str type: got %v, want \"string\"", dateProp["type"])
	}
	if dateProp["description"] != "The date string to parse" {
		t.Errorf("date_str description: got %v", dateProp["description"])
	}
}

// ── Dispatch with empty input ────────────────────────────────────────────────

func TestTool_DispatchEmptyInput(t *testing.T) {
	app := NewApp()
	app.Tool("empty", "An empty tool", emptyTool)

	td := app.Tools[0]
	result, err := td.Func(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	var output emptyOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
}

// ── Dispatch with extra JSON fields (should not error — Go ignores unknown fields by default) ──

func TestTool_DispatchExtraJSONFields(t *testing.T) {
	app := NewApp()
	app.Tool("get_weather", "Get weather", getWeather)

	td := app.Tools[0]
	argsJSON := []byte(`{"city":"Berlin","country":"DE","extra_field":"should be ignored"}`)
	result, err := td.Func(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	var output weatherOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.Description != "Sunny in Berlin" {
		t.Errorf("description: got %q", output.Description)
	}
}

// ── Register convention test ─────────────────────────────────────────────────

func TestRegisterConvention(t *testing.T) {
	// Simulate the Register(app *App) convention used by tool packages.
	register := func(app *App) {
		app.Tool("get_weather", "Get weather for a city", getWeather)
	}

	app := NewApp()
	register(app)

	if len(app.Tools) != 1 {
		t.Fatalf("expected 1 tool after Register, got %d", len(app.Tools))
	}
	if app.Tools[0].Name != "get_weather" {
		t.Errorf("tool name: got %q", app.Tools[0].Name)
	}

	// Use the registry with FindTool.
	reg := app.Registry()
	td := reg.FindTool("get_weather")
	if td == nil {
		t.Fatal("FindTool returned nil")
	}

	result, err := td.Func(context.Background(), []byte(`{"city":"Tokyo","country":"JP"}`))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	var output weatherOutput
	json.Unmarshal(result, &output)
	if output.Description != "Sunny in Tokyo" {
		t.Errorf("description: got %q", output.Description)
	}
}
