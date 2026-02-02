package gen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/api/portapi/testdata/exampleapi"
)

func TestTestClientGenerator_Generate(t *testing.T) {
	// Create an App and register endpoints
	app := &portapi.App{}
	exampleapi.Register(app)

	gen := &TestClientGenerator{
		PackageName:  "gen",
		Endpoints:    app.Endpoints(),
		TypesPackage: "github.com/shipq/shipq/api/portapi/testdata/exampleapi",
	}

	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Verify output is non-empty
	if len(output) == 0 {
		t.Fatal("Generate() produced empty output")
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "testclient_golden.go")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		// Update golden file
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("Failed to create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0644); err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Compare with golden file
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file (run with UPDATE_GOLDEN=1 to create): %v", err)
	}

	if string(output) != string(golden) {
		t.Errorf("Generated output does not match golden file.\n\nGot:\n%s\n\nWant:\n%s", output, golden)
	}
}

func TestTestClientGenerator_Determinism(t *testing.T) {
	// Create an App and register endpoints
	app := &portapi.App{}
	exampleapi.Register(app)

	gen := &TestClientGenerator{
		PackageName: "gen",
		Endpoints:   app.Endpoints(),
	}

	// Generate twice
	output1, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() first call error: %v", err)
	}

	output2, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() second call error: %v", err)
	}

	// Outputs should be identical
	if string(output1) != string(output2) {
		t.Error("Generate() is not deterministic - outputs differ between calls")
	}
}

func TestTestClientGenerator_EmptyEndpoints(t *testing.T) {
	gen := &TestClientGenerator{
		PackageName: "test",
		Endpoints:   nil,
	}

	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Should still produce valid code with Client struct
	if len(output) == 0 {
		t.Error("Generate() with empty endpoints should still produce output")
	}

	// Should contain the Client struct
	if !contains(output, "type Client struct") {
		t.Error("Output should contain Client struct definition")
	}
}

func TestTestClientGenerator_RequiresPackageName(t *testing.T) {
	gen := &TestClientGenerator{
		PackageName: "",
		Endpoints:   nil,
	}

	_, err := gen.Generate()
	if err == nil {
		t.Error("Generate() should error when PackageName is empty")
	}
}

func TestTestClientGenerator_MethodNaming(t *testing.T) {
	// Test that method names are derived correctly
	tests := []struct {
		method      string
		path        string
		handlerName string
		wantName    string
	}{
		{"GET", "/pets", "ListPets", "ListPets"},
		{"POST", "/pets", "CreatePet", "CreatePet"},
		{"GET", "/pets/{id}", "GetPet", "GetPet"},
		{"DELETE", "/pets/{id}", "DeletePet", "DeletePet"},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			gen := &TestClientGenerator{PackageName: "test"}
			ep := portapi.Endpoint{
				Method:      tt.method,
				Path:        tt.path,
				HandlerName: tt.handlerName,
			}
			name := gen.deriveMethodName(ep)
			if name != tt.wantName {
				t.Errorf("deriveMethodName() = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestTestClientGenerator_UniqueMethodNames(t *testing.T) {
	gen := &TestClientGenerator{PackageName: "test"}
	usedNames := make(map[string]int)

	// First use should return the base name
	name1 := gen.uniqueMethodName("GetPet", usedNames)
	if name1 != "GetPet" {
		t.Errorf("First use should return base name, got %q", name1)
	}

	// Second use should return suffixed name
	name2 := gen.uniqueMethodName("GetPet", usedNames)
	if name2 != "GetPet_2" {
		t.Errorf("Second use should return suffixed name, got %q", name2)
	}

	// Third use should return next suffix
	name3 := gen.uniqueMethodName("GetPet", usedNames)
	if name3 != "GetPet_3" {
		t.Errorf("Third use should return next suffix, got %q", name3)
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"hello_world", "HelloWorld"},
		{"hello-world", "HelloWorld"},
		{"hello world", "HelloWorld"},
		{"helloWorld", "HelloWorld"},
		{"", ""},
		{"a", "A"},
		{"ID", "ID"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Helper to check if output contains a string
func contains(output []byte, substr string) bool {
	return len(output) > 0 && len(substr) > 0 && string(output) != "" && string(output) != substr && containsString(string(output), substr)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

// Compile-time check that generated code uses correct types
var _ = func() bool {
	// This ensures the exampleapi package is imported and handlers exist
	_ = exampleapi.CreatePet
	_ = exampleapi.GetPet
	_ = exampleapi.ListPets
	_ = exampleapi.DeletePet
	_ = exampleapi.SearchPets
	_ = exampleapi.UpdatePet
	_ = exampleapi.HealthCheck
	return true
}()

// Ensure context is used (for import)
var _ context.Context
