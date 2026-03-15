package tsutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGoTypeStringToTS(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"int", "number"},
		{"int8", "number"},
		{"int16", "number"},
		{"int32", "number"},
		{"int64", "number"},
		{"uint", "number"},
		{"uint8", "number"},
		{"uint16", "number"},
		{"uint32", "number"},
		{"uint64", "number"},
		{"float32", "number"},
		{"float64", "number"},
		{"bool", "boolean"},
		{"any", "any"},
		{"interface{}", "any"},
		{"[]string", "string[]"},
		{"[]int", "number[]"},
		{"[]bool", "boolean[]"},
		{"[][]string", "string[][]"},
		{"*string", "string"},
		{"*int", "number"},
		{"*bool", "boolean"},
		{"map[string]any", "Record<string, any>"},
		{"map[string]string", "Record<string, string>"},
		{"map[string]int", "Record<string, number>"},
		{"map[string][]string", "Record<string, string[]>"},
		{"json.RawMessage", "any"},
		{"*json.RawMessage", "any"},
		{"time.Time", "any"},
		{"SomeCustomType", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := GoTypeStringToTS(tt.goType)
			if result != tt.expected {
				t.Errorf("GoTypeStringToTS(%q) = %q, want %q", tt.goType, result, tt.expected)
			}
		})
	}
}

func TestGoTypeToTS_SimpleField(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name: "Title",
		Type: "string",
	}
	result := GoTypeToTS(field)
	if result != "string" {
		t.Errorf("GoTypeToTS(string field) = %q, want %q", result, "string")
	}
}

func TestGoTypeToTS_NestedStruct(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name: "Author",
		Type: "Author",
		StructFields: &codegen.SerializedStructInfo{
			Name: "Author",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Name", Type: "string", JSONName: "name", Required: true},
				{Name: "Email", Type: "string", JSONName: "email", Required: false},
			},
		},
	}
	result := GoTypeToTS(field)
	if !strings.Contains(result, "name: string;") {
		t.Errorf("GoTypeToTS nested struct should contain 'name: string;', got %q", result)
	}
	if !strings.Contains(result, "email?: string;") {
		t.Errorf("GoTypeToTS nested struct should contain 'email?: string;', got %q", result)
	}
}

func TestGoStructToInlineTS(t *testing.T) {
	structInfo := &codegen.SerializedStructInfo{
		Name: "Address",
		Fields: []codegen.SerializedFieldInfo{
			{Name: "Street", Type: "string", JSONName: "street", Required: true},
			{Name: "City", Type: "string", JSONName: "city", Required: true},
			{Name: "Zip", Type: "string", JSONName: "zip", Required: false},
		},
	}
	result := GoStructToInlineTS(structInfo)
	if !strings.HasPrefix(result, "{ ") {
		t.Errorf("GoStructToInlineTS should start with '{ ', got %q", result)
	}
	if !strings.HasSuffix(result, " }") {
		t.Errorf("GoStructToInlineTS should end with ' }', got %q", result)
	}
	if !strings.Contains(result, "street: string;") {
		t.Errorf("GoStructToInlineTS should contain 'street: string;', got %q", result)
	}
	if !strings.Contains(result, "zip?: string;") {
		t.Errorf("GoStructToInlineTS should contain 'zip?: string;', got %q", result)
	}
}

func TestGoStructToInlineTS_SkipsOmittedFields(t *testing.T) {
	structInfo := &codegen.SerializedStructInfo{
		Name: "Data",
		Fields: []codegen.SerializedFieldInfo{
			{Name: "Visible", Type: "string", JSONName: "visible", Required: true},
			{Name: "Hidden", Type: "string", JSONName: "", JSONOmit: true},
		},
	}
	result := GoStructToInlineTS(structInfo)
	if strings.Contains(result, "Hidden") {
		t.Errorf("GoStructToInlineTS should skip omitted fields, got %q", result)
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello_world", "HelloWorld"},
		{"chatbot", "Chatbot"},
		{"email_notification", "EmailNotification"},
		{"my-component", "MyComponent"},
		{"already.dotted.name", "AlreadyDottedName"},
		{"a_b_c", "ABC"},
		{"", ""},
		{"single", "Single"},
		{"ALLCAPS", "ALLCAPS"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CreatePost", "createPost"},
		{"hello_world", "helloWorld"},
		{"GetOne", "getOne"},
		{"ListPosts", "listPosts"},
		{"SoftDeletePost", "softDeletePost"},
		{"A", "a"},
		{"", ""},
		{"already_camel", "alreadyCamel"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToCamelCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateTSInterface(t *testing.T) {
	var buf bytes.Buffer
	fields := []codegen.SerializedFieldInfo{
		{Name: "ID", Type: "string", JSONName: "id", Required: true},
		{Name: "Title", Type: "string", JSONName: "title", Required: true},
		{Name: "Body", Type: "string", JSONName: "body", Required: false},
		{Name: "Views", Type: "int", JSONName: "views", Required: false},
	}

	GenerateTSInterface(&buf, "CreatePostRequest", fields)
	result := buf.String()

	if !strings.Contains(result, "export interface CreatePostRequest {") {
		t.Errorf("should contain interface declaration, got %q", result)
	}
	if !strings.Contains(result, "  id: string;") {
		t.Errorf("should contain 'id: string;', got %q", result)
	}
	if !strings.Contains(result, "  title: string;") {
		t.Errorf("should contain 'title: string;', got %q", result)
	}
	if !strings.Contains(result, "  body?: string;") {
		t.Errorf("should contain 'body?: string;', got %q", result)
	}
	if !strings.Contains(result, "  views?: number;") {
		t.Errorf("should contain 'views?: number;', got %q", result)
	}
}

func TestGenerateTSInterface_SkipsHiddenFields(t *testing.T) {
	var buf bytes.Buffer
	fields := []codegen.SerializedFieldInfo{
		{Name: "ID", Type: "string", JSONName: "id", Required: true},
		{Name: "Internal", Type: "string", JSONName: "", JSONOmit: true},
		{Name: "OptionalField", Type: "string", JSONName: "optional_field", JSONOmit: true, Required: false},
	}

	GenerateTSInterface(&buf, "TestInterface", fields)
	result := buf.String()

	if strings.Contains(result, "Internal") {
		t.Errorf("should skip fields with JSONOmit=true and empty JSONName, got %q", result)
	}
	// Fields with JSONOmit=true but non-empty JSONName are optional, not hidden
	if !strings.Contains(result, "optional_field") {
		t.Errorf("should include fields with JSONOmit=true but non-empty JSONName, got %q", result)
	}
}

func TestGenerateTSInterface_UsesGoNameWhenNoJSONName(t *testing.T) {
	var buf bytes.Buffer
	fields := []codegen.SerializedFieldInfo{
		{Name: "MyField", Type: "string", JSONName: "", Required: true},
	}

	GenerateTSInterface(&buf, "TestInterface", fields)
	result := buf.String()

	if !strings.Contains(result, "  MyField: string;") {
		t.Errorf("should use Go field name when JSONName is empty, got %q", result)
	}
}

func TestGoTypeToTS_SliceField(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name: "Tags",
		Type: "[]string",
	}
	result := GoTypeToTS(field)
	if result != "string[]" {
		t.Errorf("GoTypeToTS([]string) = %q, want %q", result, "string[]")
	}
}

func TestGoTypeToTS_MapField(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name: "Metadata",
		Type: "map[string]any",
	}
	result := GoTypeToTS(field)
	if result != "Record<string, any>" {
		t.Errorf("GoTypeToTS(map[string]any) = %q, want %q", result, "Record<string, any>")
	}
}

func TestGoTypeToTS_PointerField(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name: "Description",
		Type: "*string",
	}
	result := GoTypeToTS(field)
	if result != "string" {
		t.Errorf("GoTypeToTS(*string) = %q, want %q", result, "string")
	}
}

func TestGoTypeToTS_NestedStructSlice(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name: "Items",
		Type: "[]Post",
		StructFields: &codegen.SerializedStructInfo{
			Name: "Post",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "ID", Type: "string", JSONName: "id", Required: true},
				{Name: "Title", Type: "string", JSONName: "title", Required: true},
			},
		},
	}
	result := GoTypeToTS(field)
	if !strings.HasSuffix(result, "[]") {
		t.Errorf("GoTypeToTS([]Post with StructFields) should end with '[]', got %q", result)
	}
	if !strings.Contains(result, "id: string;") {
		t.Errorf("GoTypeToTS([]Post with StructFields) should contain 'id: string;', got %q", result)
	}
	if !strings.Contains(result, "title: string;") {
		t.Errorf("GoTypeToTS([]Post with StructFields) should contain 'title: string;', got %q", result)
	}
	expected := "{ id: string; title: string; }[]"
	if result != expected {
		t.Errorf("GoTypeToTS([]Post with StructFields) = %q, want %q", result, expected)
	}
}

func TestGoTypeStringToTS_NestedMap(t *testing.T) {
	result := GoTypeStringToTS("map[string]map[string]int")
	if result != "Record<string, Record<string, number>>" {
		t.Errorf("GoTypeStringToTS nested map = %q, want %q", result, "Record<string, Record<string, number>>")
	}
}
