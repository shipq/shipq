package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestHarnessGenerator_Generate(t *testing.T) {
	tests := []struct {
		name           string
		gen            *TestHarnessGenerator
		wantErr        bool
		wantContain    []string
		wantNotContain []string
	}{
		{
			name: "basic generation without NewMux",
			gen: &TestHarnessGenerator{
				PackageName: "testpkg",
				HasNewMux:   false,
			},
			wantContain: []string{
				"package testpkg",
				"func NewTestClient(ts *httptest.Server) *Client",
				"func NewTestServer(t *testing.T, handler http.Handler) *httptest.Server",
				"ts.Client()",
				"ts.URL",
				"t.Cleanup(ts.Close)",
				`panic("NewTestClient: nil httptest.Server")`,
			},
			wantNotContain: []string{
				"func NewMux()",
			},
		},
		{
			name: "generation with NewMux (no handler impl)",
			gen: &TestHarnessGenerator{
				PackageName: "api",
				HasNewMux:   true,
			},
			wantContain: []string{
				"package api",
				"func NewTestClient(ts *httptest.Server) *Client",
				"func NewTestServer(t *testing.T) *httptest.Server",
				"mux := NewMux()",
				"ts := httptest.NewServer(mux)",
				"t.Cleanup(ts.Close)",
			},
			wantNotContain: []string{
				"handler http.Handler",
			},
		},
		{
			name: "generation with NewMux and handler impl",
			gen: &TestHarnessGenerator{
				PackageName:      "api",
				HasNewMux:        true,
				NeedsHandlerImpl: true,
				HandlerImplType:  "APIHandler",
			},
			wantContain: []string{
				"package api",
				"func NewTestClient(ts *httptest.Server) *Client",
				"func NewTestServer(t *testing.T, impl APIHandler) *httptest.Server",
				"mux := NewMux(impl)",
				"t.Cleanup(ts.Close)",
			},
		},
		{
			name: "missing package name",
			gen: &TestHarnessGenerator{
				PackageName: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.gen.Generate()

			if tt.wantErr {
				if err == nil {
					t.Error("Generate() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			outputStr := string(output)

			for _, want := range tt.wantContain {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output should contain %q, but doesn't.\n\nOutput:\n%s", want, outputStr)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(outputStr, notWant) {
					t.Errorf("Output should NOT contain %q, but does.\n\nOutput:\n%s", notWant, outputStr)
				}
			}
		})
	}
}

func TestTestHarnessGenerator_Determinism(t *testing.T) {
	gen := &TestHarnessGenerator{
		PackageName: "testpkg",
		HasNewMux:   true,
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

func TestTestHarnessGenerator_ImportsAreSorted(t *testing.T) {
	gen := &TestHarnessGenerator{
		PackageName: "testpkg",
		HasNewMux:   true,
	}

	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	outputStr := string(output)

	// Find the import block
	importStart := strings.Index(outputStr, "import (")
	if importStart == -1 {
		t.Fatal("Could not find import block")
	}

	importEnd := strings.Index(outputStr[importStart:], ")")
	if importEnd == -1 {
		t.Fatal("Could not find end of import block")
	}

	importBlock := outputStr[importStart : importStart+importEnd+1]

	// Verify imports are present and sorted
	// net/http should come before net/http/httptest which should come before testing
	netHTTPIdx := strings.Index(importBlock, `"net/http"`)
	httptestIdx := strings.Index(importBlock, `"net/http/httptest"`)
	testingIdx := strings.Index(importBlock, `"testing"`)

	if netHTTPIdx == -1 || httptestIdx == -1 || testingIdx == -1 {
		t.Errorf("Missing expected imports in import block:\n%s", importBlock)
		return
	}

	if !(netHTTPIdx < httptestIdx && httptestIdx < testingIdx) {
		t.Errorf("Imports are not sorted alphabetically:\n%s", importBlock)
	}
}

func TestTestHarnessGenerator_CompilesValidGo(t *testing.T) {
	// The Generate function uses go/format.Source which will fail if the output
	// is not valid Go code. This test verifies that all configurations produce
	// valid, formattable Go code.

	configs := []struct {
		name string
		gen  *TestHarnessGenerator
	}{
		{
			name: "without NewMux",
			gen: &TestHarnessGenerator{
				PackageName: "api",
				HasNewMux:   false,
			},
		},
		{
			name: "with NewMux",
			gen: &TestHarnessGenerator{
				PackageName: "api",
				HasNewMux:   true,
			},
		},
		{
			name: "with NewMux and handler impl",
			gen: &TestHarnessGenerator{
				PackageName:      "api",
				HasNewMux:        true,
				NeedsHandlerImpl: true,
				HandlerImplType:  "HandlerImpl",
			},
		},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			output, err := cfg.gen.Generate()
			if err != nil {
				t.Errorf("Generate() failed to produce valid Go code: %v", err)
				return
			}

			if len(output) == 0 {
				t.Error("Generate() produced empty output")
			}
		})
	}
}

func TestTestHarnessGenerator_DocumentationComments(t *testing.T) {
	gen := &TestHarnessGenerator{
		PackageName: "api",
		HasNewMux:   true,
	}

	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	outputStr := string(output)

	// Verify documentation comments are present
	expectedComments := []string{
		"// NewTestClient creates a test client configured for the given httptest.Server.",
		"// NewTestServer creates an httptest.Server",
		"// Example usage:",
		"// Code generated by portapi testharness generator. DO NOT EDIT.",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("Output should contain documentation comment %q", comment)
		}
	}
}

func TestTestHarnessGenerator_THelperCalled(t *testing.T) {
	// NewTestServer should call t.Helper() so that test failures
	// report the correct line in the actual test file

	gen := &TestHarnessGenerator{
		PackageName: "api",
		HasNewMux:   true,
	}

	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	outputStr := string(output)

	if !strings.Contains(outputStr, "t.Helper()") {
		t.Error("NewTestServer should call t.Helper()")
	}
}

func TestTestHarnessGenerator_GoldenFile(t *testing.T) {
	gen := &TestHarnessGenerator{
		PackageName: "gen",
		HasNewMux:   false,
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
	goldenPath := filepath.Join("testdata", "testharness_golden.go")

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
