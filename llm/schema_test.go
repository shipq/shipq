package llm

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func mustSchema(t *testing.T, typ reflect.Type) map[string]any {
	t.Helper()
	raw, err := SchemaFromType(typ)
	if err != nil {
		t.Fatalf("SchemaFromType(%s): %v", typ, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return out
}

func schemaOf[T any](t *testing.T) map[string]any {
	t.Helper()
	var zero T
	return mustSchema(t, reflect.TypeOf(zero))
}

func assertType(t *testing.T, schema map[string]any, want string) {
	t.Helper()
	got, ok := schema["type"]
	if !ok {
		t.Errorf("schema missing \"type\" field; schema = %v", schema)
		return
	}
	if s, ok := got.(string); ok {
		if s != want {
			t.Errorf("type: got %q, want %q", s, want)
		}
		return
	}
	t.Errorf("type: expected string %q, got %T %v", want, got, got)
}

func assertTypeUnion(t *testing.T, schema map[string]any, want ...string) {
	t.Helper()
	raw, ok := schema["type"]
	if !ok {
		t.Errorf("schema missing \"type\" field; schema = %v", schema)
		return
	}
	slice, ok := raw.([]any)
	if !ok {
		t.Errorf("type: expected []any, got %T %v", raw, raw)
		return
	}
	got := make([]string, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			t.Errorf("type[%d]: expected string, got %T %v", i, v, v)
			return
		}
		got[i] = s
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("type union: got %v, want %v", got, want)
	}
}

func assertAdditionalPropertiesFalse(t *testing.T, schema map[string]any) {
	t.Helper()
	ap, ok := schema["additionalProperties"]
	if !ok {
		t.Errorf("schema missing \"additionalProperties\"")
		return
	}
	if b, ok := ap.(bool); !ok || b != false {
		t.Errorf("additionalProperties: got %v, want false", ap)
	}
}

func assertProperty(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema[\"properties\"] is not map[string]any: %T", schema["properties"])
	}
	prop, ok := props[name]
	if !ok {
		t.Fatalf("property %q not found in schema; keys = %v", name, keys(props))
	}
	m, ok := prop.(map[string]any)
	if !ok {
		t.Fatalf("property %q is not map[string]any: %T", name, prop)
	}
	return m
}

func assertNoProperty(t *testing.T, schema map[string]any, name string) {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	if _, found := props[name]; found {
		t.Errorf("property %q should not be present in schema", name)
	}
}

func assertRequired(t *testing.T, schema map[string]any, fields ...string) {
	t.Helper()
	raw, ok := schema["required"]
	if !ok {
		t.Errorf("schema missing \"required\" field")
		return
	}
	slice, ok := raw.([]any)
	if !ok {
		t.Errorf("\"required\" is not []any: %T", raw)
		return
	}
	got := make(map[string]bool, len(slice))
	for _, v := range slice {
		s, ok := v.(string)
		if !ok {
			t.Errorf("required entry is not string: %T %v", v, v)
			continue
		}
		got[s] = true
	}
	for _, f := range fields {
		if !got[f] {
			t.Errorf("field %q not in required; got %v", f, slice)
		}
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ── primitive types ───────────────────────────────────────────────────────────

func TestSchemaString(t *testing.T) {
	s := schemaOf[string](t)
	assertType(t, s, "string")
}

func TestSchemaInt(t *testing.T) {
	s := schemaOf[int](t)
	assertType(t, s, "integer")
}

func TestSchemaInt32(t *testing.T) {
	s := schemaOf[int32](t)
	assertType(t, s, "integer")
}

func TestSchemaInt64(t *testing.T) {
	s := schemaOf[int64](t)
	assertType(t, s, "integer")
}

func TestSchemaFloat32(t *testing.T) {
	s := schemaOf[float32](t)
	assertType(t, s, "number")
}

func TestSchemaFloat64(t *testing.T) {
	s := schemaOf[float64](t)
	assertType(t, s, "number")
}

func TestSchemaBool(t *testing.T) {
	s := schemaOf[bool](t)
	assertType(t, s, "boolean")
}

// ── time.Time ─────────────────────────────────────────────────────────────────

func TestSchemaTimeTime(t *testing.T) {
	s := schemaOf[time.Time](t)
	assertType(t, s, "string")
	if fmt, ok := s["format"].(string); !ok || fmt != "date-time" {
		t.Errorf("format: got %v, want \"date-time\"", s["format"])
	}
}

// ── slices ────────────────────────────────────────────────────────────────────

func TestSchemaSliceString(t *testing.T) {
	s := schemaOf[[]string](t)
	assertType(t, s, "array")
	items, ok := s["items"].(map[string]any)
	if !ok {
		t.Fatalf("items: expected map[string]any, got %T", s["items"])
	}
	assertType(t, items, "string")
}

func TestSchemaSliceInt(t *testing.T) {
	s := schemaOf[[]int](t)
	assertType(t, s, "array")
	items := s["items"].(map[string]any)
	assertType(t, items, "integer")
}

func TestSchemaSliceStruct(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}
	s := mustSchema(t, reflect.TypeOf([]Item{}))
	assertType(t, s, "array")
	items, ok := s["items"].(map[string]any)
	if !ok {
		t.Fatalf("items: expected map[string]any, got %T", s["items"])
	}
	assertType(t, items, "object")
	assertAdditionalPropertiesFalse(t, items)
}

// ── maps ──────────────────────────────────────────────────────────────────────

func TestSchemaMapStringString(t *testing.T) {
	s := schemaOf[map[string]string](t)
	assertType(t, s, "object")
	ap, ok := s["additionalProperties"].(map[string]any)
	if !ok {
		t.Fatalf("additionalProperties: expected map[string]any, got %T %v", s["additionalProperties"], s["additionalProperties"])
	}
	assertType(t, ap, "string")
}

func TestSchemaMapStringInt(t *testing.T) {
	s := schemaOf[map[string]int](t)
	ap := s["additionalProperties"].(map[string]any)
	assertType(t, ap, "integer")
}

// ── pointer fields → nullable type union ─────────────────────────────────────

func TestSchemaNullablePointerField(t *testing.T) {
	type Req struct {
		Required string  `json:"required"`
		Optional *string `json:"optional"`
	}
	s := schemaOf[Req](t)
	assertType(t, s, "object")

	req := assertProperty(t, s, "required")
	assertType(t, req, "string")

	opt := assertProperty(t, s, "optional")
	assertTypeUnion(t, opt, "string", "null")
}

func TestSchemaNullablePointerInt(t *testing.T) {
	type Req struct {
		Count *int `json:"count"`
	}
	s := schemaOf[Req](t)
	p := assertProperty(t, s, "count")
	assertTypeUnion(t, p, "integer", "null")
}

func TestSchemaNullablePointerTime(t *testing.T) {
	type Req struct {
		Deadline *time.Time `json:"deadline"`
	}
	s := schemaOf[Req](t)
	p := assertProperty(t, s, "deadline")
	assertTypeUnion(t, p, "string", "null")
	if fmt, ok := p["format"].(string); !ok || fmt != "date-time" {
		t.Errorf("format: got %v, want \"date-time\"", p["format"])
	}
}

// ── json tag handling ─────────────────────────────────────────────────────────

func TestSchemaJSONTagRename(t *testing.T) {
	type Req struct {
		MyField string `json:"my_field"`
	}
	s := schemaOf[Req](t)
	assertProperty(t, s, "my_field")
	assertNoProperty(t, s, "MyField")
}

func TestSchemaJSONTagDash(t *testing.T) {
	type Req struct {
		Exported string `json:"exported"`
		Hidden   string `json:"-"`
	}
	s := schemaOf[Req](t)
	assertProperty(t, s, "exported")
	assertNoProperty(t, s, "Hidden")
	assertNoProperty(t, s, "-")
}

func TestSchemaJSONTagOmitempty(t *testing.T) {
	// omitempty should NOT exclude the field from the schema — only json:"-" does.
	type Req struct {
		Name string `json:"name,omitempty"`
	}
	s := schemaOf[Req](t)
	assertProperty(t, s, "name")
}

// ── desc tag → description field ─────────────────────────────────────────────

func TestSchemaDescTag(t *testing.T) {
	type Req struct {
		City string `json:"city" desc:"The city name"`
	}
	s := schemaOf[Req](t)
	p := assertProperty(t, s, "city")
	desc, ok := p["description"].(string)
	if !ok {
		t.Fatalf("description: expected string, got %T %v", p["description"], p["description"])
	}
	if desc != "The city name" {
		t.Errorf("description: got %q, want %q", desc, "The city name")
	}
}

// ── struct schemas ─────────────────────────────────────────────────────────────

func TestSchemaStructBasic(t *testing.T) {
	type WeatherInput struct {
		City    string `json:"city" desc:"City to get weather for"`
		Country string `json:"country"`
	}
	s := schemaOf[WeatherInput](t)
	assertType(t, s, "object")
	assertAdditionalPropertiesFalse(t, s)
	assertRequired(t, s, "city", "country")

	city := assertProperty(t, s, "city")
	assertType(t, city, "string")
	if d := city["description"].(string); d != "City to get weather for" {
		t.Errorf("description: got %q", d)
	}
}

func TestSchemaStructAllFieldsRequired(t *testing.T) {
	type Req struct {
		A string  `json:"a"`
		B int     `json:"b"`
		C *string `json:"c"`
	}
	s := schemaOf[Req](t)
	// All fields — including pointer fields — must appear in required.
	assertRequired(t, s, "a", "b", "c")
}

func TestSchemaStructAdditionalPropertiesFalse(t *testing.T) {
	type Req struct {
		X string `json:"x"`
	}
	s := schemaOf[Req](t)
	assertAdditionalPropertiesFalse(t, s)
}

func TestSchemaStructUnexportedFieldSkipped(t *testing.T) {
	type Req struct {
		Exported string `json:"exported"`
		hidden   string //nolint:unused
	}
	s := schemaOf[Req](t)
	assertProperty(t, s, "exported")
	if props, ok := s["properties"].(map[string]any); ok {
		if _, found := props["hidden"]; found {
			t.Error("unexported field \"hidden\" should not appear in schema")
		}
	}
}

// ── nested structs ────────────────────────────────────────────────────────────

func TestSchemaNestedStruct(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	type Person struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}
	s := schemaOf[Person](t)
	assertType(t, s, "object")
	assertAdditionalPropertiesFalse(t, s)
	assertRequired(t, s, "name", "address")

	addr := assertProperty(t, s, "address")
	assertType(t, addr, "object")
	assertAdditionalPropertiesFalse(t, addr)
	assertRequired(t, addr, "street", "city")
	assertProperty(t, addr, "street")
	assertProperty(t, addr, "city")
}

func TestSchemaNestedStructPointer(t *testing.T) {
	type Inner struct {
		Value int `json:"value"`
	}
	type Outer struct {
		Inner *Inner `json:"inner"`
	}
	s := schemaOf[Outer](t)
	inner := assertProperty(t, s, "inner")
	// Pointer to struct → object type with null in union.
	assertTypeUnion(t, inner, "object", "null")
	assertAdditionalPropertiesFalse(t, inner)
}

// ── map of struct ──────────────────────────────────────────────────────────────

func TestSchemaMapStringStruct(t *testing.T) {
	type Val struct {
		X int `json:"x"`
	}
	s := mustSchema(t, reflect.TypeOf(map[string]Val{}))
	assertType(t, s, "object")
	ap, ok := s["additionalProperties"].(map[string]any)
	if !ok {
		t.Fatalf("additionalProperties: expected map[string]any, got %T", s["additionalProperties"])
	}
	assertType(t, ap, "object")
	assertAdditionalPropertiesFalse(t, ap)
}
