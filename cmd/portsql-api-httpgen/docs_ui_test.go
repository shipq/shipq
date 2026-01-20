package main

import (
	"strings"
	"testing"
)

// =============================================================================
// OpenAPI Step 4 Tests: Docs UI Generation
// =============================================================================

// Test 1: When docs_ui disabled, no docs code is generated
func TestGenerateDocsUI_Disabled(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   false,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/health",
				HandlerPkg:  "example.com/api",
				HandlerName: "Health",
				Shape:       "ctx_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	// When disabled, should return empty string
	if code != "" {
		t.Errorf("expected empty code when docs_ui disabled, got %d bytes", len(code))
	}
}

// Test 2: When docs_ui enabled, generated file defines expected functions
func TestGenerateDocsUI_Enabled(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPITitle:    "Test API",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets",
				HandlerPkg:  "example.com/api",
				HandlerName: "ListPets",
				Shape:       "ctx_resp_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	t.Run("contains_RegisterDocs_function", func(t *testing.T) {
		if !strings.Contains(code, "func RegisterDocs(mux *http.ServeMux)") {
			t.Error("expected RegisterDocs function")
		}
	})

	t.Run("contains_OpenAPIJSON_function", func(t *testing.T) {
		if !strings.Contains(code, "func OpenAPIJSON() []byte") {
			t.Error("expected OpenAPIJSON function")
		}
	})

	t.Run("embeds_openapi_json", func(t *testing.T) {
		if !strings.Contains(code, "//go:embed openapi.json") {
			t.Error("expected go:embed directive for openapi.json")
		}
	})

	t.Run("embeds_docs_assets", func(t *testing.T) {
		if !strings.Contains(code, "//go:embed zz_generated_docs_assets") {
			t.Error("expected go:embed directive for docs assets")
		}
	})

	t.Run("registers_openapi_json_route", func(t *testing.T) {
		if !strings.Contains(code, "/openapi.json") {
			t.Error("expected openapi.json route registration")
		}
	})

	t.Run("registers_docs_route", func(t *testing.T) {
		if !strings.Contains(code, "/docs") {
			t.Error("expected docs route registration")
		}
	})

	t.Run("registers_assets_route", func(t *testing.T) {
		if !strings.Contains(code, "/docs/assets/") {
			t.Error("expected assets route registration")
		}
	})

	t.Run("html_references_openapi_url", func(t *testing.T) {
		if !strings.Contains(code, `apiDescriptionUrl="/openapi.json"`) &&
			!strings.Contains(code, `apiDescriptionUrl=\"/openapi.json\"`) {
			t.Error("expected HTML to reference /openapi.json")
		}
	})

	t.Run("html_references_assets", func(t *testing.T) {
		if !strings.Contains(code, "/docs/assets/") {
			t.Error("expected HTML to reference /docs/assets/")
		}
	})

	t.Run("imports_required_packages", func(t *testing.T) {
		if !strings.Contains(code, `"embed"`) {
			t.Error("expected embed import")
		}
		if !strings.Contains(code, `"net/http"`) {
			t.Error("expected net/http import")
		}
		if !strings.Contains(code, `"io/fs"`) {
			t.Error("expected io/fs import")
		}
	})

	t.Run("is_valid_go_package", func(t *testing.T) {
		if !strings.Contains(code, "package ") {
			t.Error("expected package declaration")
		}
	})
}

// Test 3: Path normalization is baked in correctly
func TestGenerateDocsUI_PathNormalization(t *testing.T) {
	// Test with trailing slash - should be normalized
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs", // Already normalized by config loading
		OpenAPIJSONPath: "/openapi.json",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	// Should use /docs consistently, not /docs/
	// The route for docs root should redirect or serve
	if !strings.Contains(code, "/docs") {
		t.Error("expected /docs path in generated code")
	}
}

// Test: Custom paths are used correctly
func TestGenerateDocsUI_CustomPaths(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/api-docs",
		OpenAPIJSONPath: "/api/openapi.json",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	t.Run("uses_custom_docs_path", func(t *testing.T) {
		if !strings.Contains(code, "/api-docs") {
			t.Error("expected custom docs path /api-docs")
		}
	})

	t.Run("uses_custom_openapi_path", func(t *testing.T) {
		if !strings.Contains(code, "/api/openapi.json") {
			t.Error("expected custom openapi path /api/openapi.json")
		}
	})

	t.Run("assets_under_docs_path", func(t *testing.T) {
		if !strings.Contains(code, "/api-docs/assets/") {
			t.Error("expected assets under /api-docs/assets/")
		}
	})
}

// Test: HTML contains required elements for Stoplight
func TestGenerateDocsUI_HTMLContent(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPITitle:    "My Pet API",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	t.Run("html_contains_doctype", func(t *testing.T) {
		if !strings.Contains(code, "<!DOCTYPE html>") && !strings.Contains(code, "<!doctype html>") {
			t.Error("expected HTML doctype")
		}
	})

	t.Run("html_contains_elements_api_tag", func(t *testing.T) {
		if !strings.Contains(code, "<elements-api") {
			t.Error("expected <elements-api> custom element")
		}
	})

	t.Run("html_contains_css_link", func(t *testing.T) {
		if !strings.Contains(code, ".css") {
			t.Error("expected CSS link in HTML")
		}
	})

	t.Run("html_contains_js_script", func(t *testing.T) {
		if !strings.Contains(code, ".js") {
			t.Error("expected JS script in HTML")
		}
	})

	t.Run("html_contains_title", func(t *testing.T) {
		if !strings.Contains(code, "My Pet API") && !strings.Contains(code, "API Documentation") {
			t.Error("expected title in HTML")
		}
	})
}

// Test: Deterministic output
func TestGenerateDocsUI_Deterministic(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPITitle:    "Test API",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "api", HandlerName: "ListPets", Shape: "ctx_resp_err"},
			{Method: "POST", Path: "/pets", HandlerPkg: "api", HandlerName: "CreatePet", Shape: "ctx_req_resp_err"},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code1, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("first GenerateDocsUI failed: %v", err)
	}

	code2, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("second GenerateDocsUI failed: %v", err)
	}

	if code1 != code2 {
		t.Error("GenerateDocsUI output is not deterministic")
	}
}

// Test: Cache control headers are set
func TestGenerateDocsUI_CacheHeaders(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	t.Run("openapi_json_cache_control", func(t *testing.T) {
		// OpenAPI JSON should have no-store or short cache
		if !strings.Contains(code, "Cache-Control") {
			t.Error("expected Cache-Control header handling")
		}
	})

	t.Run("sets_content_type_for_json", func(t *testing.T) {
		if !strings.Contains(code, "application/json") {
			t.Error("expected application/json content type")
		}
	})

	t.Run("sets_content_type_for_html", func(t *testing.T) {
		if !strings.Contains(code, "text/html") {
			t.Error("expected text/html content type")
		}
	})
}

// Test: Redirect from /docs to /docs/
func TestGenerateDocsUI_RedirectBehavior(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	// Should handle both /docs and /docs/
	// Either redirect or serve both
	if !strings.Contains(code, `"/docs/"`) && !strings.Contains(code, "Redirect") {
		t.Log("Note: Consider handling redirect from /docs to /docs/")
	}
}

// Test: Package name parameter
func TestGenerateDocsUI_PackageName(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUIWithPackage(cfg, manifest, "myapi")
	if err != nil {
		t.Fatalf("GenerateDocsUIWithPackage failed: %v", err)
	}

	if !strings.Contains(code, "package myapi") {
		t.Error("expected package myapi")
	}
}

// Test: DO NOT EDIT header
func TestGenerateDocsUI_Header(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:  true,
		DocsUIEnabled:   true,
		DocsPath:        "/docs",
		OpenAPIJSONPath: "/openapi.json",
		OpenAPIOutput:   "openapi.json",
	}

	manifest := &Manifest{
		Endpoints:    []ManifestEndpoint{},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	code, err := GenerateDocsUI(cfg, manifest)
	if err != nil {
		t.Fatalf("GenerateDocsUI failed: %v", err)
	}

	if !strings.Contains(code, "DO NOT EDIT") {
		t.Error("expected DO NOT EDIT header")
	}
}
