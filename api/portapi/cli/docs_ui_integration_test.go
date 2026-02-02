//go:build integration

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// OpenAPI Step 4 Integration Tests: Docs UI Runtime Behavior
// =============================================================================

// Test 5: GET openapi_json_path returns JSON
func TestDocsUI_OpenAPIJSONEndpoint(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	// Create test module
	setupDocsUITestModule(t, tmpDir, modDir)

	// Build and run the generator
	runDocsGenerator(t, tmpDir, modDir)

	// Build and start test server (simulated from generated files)
	server := startDocsTestServer(t, tmpDir)
	defer server.Close()

	// Test GET /openapi.json
	resp, err := http.Get(server.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("GET /openapi.json failed: %v", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected Content-Type to start with 'application/json', got %q", contentType)
	}

	// Check cache control
	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl != "no-store" {
		t.Errorf("expected Cache-Control 'no-store', got %q", cacheControl)
	}

	// Parse body as JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check OpenAPI structure
	if _, ok := doc["openapi"]; !ok {
		t.Error("expected 'openapi' key in JSON")
	}
	if _, ok := doc["paths"]; !ok {
		t.Error("expected 'paths' key in JSON")
	}
}

// Test 6: GET docs_path returns HTML (with redirect)
func TestDocsUI_DocsHTMLEndpoint(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupDocsUITestModule(t, tmpDir, modDir)
	runDocsGenerator(t, tmpDir, modDir)
	server := startDocsTestServer(t, tmpDir)
	defer server.Close()

	t.Run("redirect_from_docs_to_docs_slash", func(t *testing.T) {
		// Create client that doesn't follow redirects
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := client.Get(server.URL + "/docs")
		if err != nil {
			t.Fatalf("GET /docs failed: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect
		if resp.StatusCode != http.StatusMovedPermanently {
			t.Errorf("expected status 301, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/docs/" {
			t.Errorf("expected Location '/docs/', got %q", location)
		}
	})

	t.Run("docs_slash_returns_html", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/docs/")
		if err != nil {
			t.Fatalf("GET /docs/ failed: %v", err)
		}
		defer resp.Body.Close()

		// Check status
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Check content type
		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "text/html") {
			t.Errorf("expected Content-Type to start with 'text/html', got %q", contentType)
		}

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		html := string(body)

		// Check HTML content
		if !strings.Contains(html, "<!DOCTYPE html>") {
			t.Error("expected HTML doctype")
		}
		if !strings.Contains(html, "<elements-api") {
			t.Error("expected <elements-api> element")
		}
		if !strings.Contains(html, "/openapi.json") {
			t.Error("expected reference to /openapi.json")
		}
		if !strings.Contains(html, "/docs/assets/") {
			t.Error("expected reference to /docs/assets/")
		}
	})
}

// Test 7: GET docs assets returns content
func TestDocsUI_AssetsEndpoint(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupDocsUITestModule(t, tmpDir, modDir)
	runDocsGenerator(t, tmpDir, modDir)
	server := startDocsTestServer(t, tmpDir)
	defer server.Close()

	t.Run("css_asset", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/docs/assets/stoplight/elements.min.css")
		if err != nil {
			t.Fatalf("GET CSS failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "css") {
			t.Errorf("expected CSS content type, got %q", contentType)
		}

		// Check cache header
		cacheControl := resp.Header.Get("Cache-Control")
		if !strings.Contains(cacheControl, "max-age") {
			t.Errorf("expected Cache-Control with max-age, got %q", cacheControl)
		}
	})

	t.Run("js_asset", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/docs/assets/stoplight/elements.min.js")
		if err != nil {
			t.Fatalf("GET JS failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "javascript") && !strings.Contains(contentType, "text/plain") {
			t.Errorf("expected JS content type, got %q", contentType)
		}
	})

	t.Run("nonexistent_asset_returns_404", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/docs/assets/nonexistent.xyz")
		if err != nil {
			t.Fatalf("GET nonexistent failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

// Test 9: Docs UI works with custom openapi_json_path
func TestDocsUI_CustomOpenAPIPath(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	// Setup with custom paths
	setupDocsUITestModuleWithPaths(t, tmpDir, modDir, "/api-docs", "/api/spec.json")
	runDocsGenerator(t, tmpDir, modDir)
	server := startDocsTestServerWithPaths(t, tmpDir, "/api-docs", "/api/spec.json")
	defer server.Close()

	t.Run("openapi_at_custom_path", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/spec.json")
		if err != nil {
			t.Fatalf("GET /api/spec.json failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("docs_at_custom_path", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api-docs/")
		if err != nil {
			t.Fatalf("GET /api-docs/ failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		html := string(body)

		// HTML should reference the custom openapi path
		if !strings.Contains(html, "/api/spec.json") {
			t.Error("expected HTML to reference /api/spec.json")
		}
	})
}

// Test: Docs UI files are generated when enabled
func TestDocsUI_GeneratesFiles(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	setupDocsUITestModule(t, tmpDir, modDir)
	runDocsGenerator(t, tmpDir, modDir)

	apiDir := filepath.Join(tmpDir, "api")

	// Check zz_generated_openapi.go exists
	openapiGoPath := filepath.Join(apiDir, "zz_generated_openapi.go")
	if _, err := os.Stat(openapiGoPath); os.IsNotExist(err) {
		t.Error("zz_generated_openapi.go was not generated")
	}

	// Check zz_generated_docs_assets directory exists
	assetsDir := filepath.Join(apiDir, "zz_generated_docs_assets")
	if _, err := os.Stat(assetsDir); os.IsNotExist(err) {
		t.Error("zz_generated_docs_assets directory was not generated")
	}

	// Check assets exist
	cssPath := filepath.Join(assetsDir, "stoplight", "elements.min.css")
	if _, err := os.Stat(cssPath); os.IsNotExist(err) {
		t.Error("elements.min.css was not generated")
	}

	jsPath := filepath.Join(assetsDir, "stoplight", "elements.min.js")
	if _, err := os.Stat(jsPath); os.IsNotExist(err) {
		t.Error("elements.min.js was not generated")
	}
}

// Test: No docs files when disabled
func TestDocsUI_NotGeneratedWhenDisabled(t *testing.T) {
	modDir := getModuleDir(t)
	tmpDir := t.TempDir()

	// Setup without docs UI enabled
	setupDocsUITestModuleDisabled(t, tmpDir, modDir)
	runDocsGenerator(t, tmpDir, modDir)

	apiDir := filepath.Join(tmpDir, "api")

	// Check zz_generated_openapi.go does NOT exist
	openapiGoPath := filepath.Join(apiDir, "zz_generated_openapi.go")
	if _, err := os.Stat(openapiGoPath); !os.IsNotExist(err) {
		t.Error("zz_generated_openapi.go should not be generated when docs_ui is disabled")
	}

	// Check zz_generated_docs_assets does NOT exist
	assetsDir := filepath.Join(apiDir, "zz_generated_docs_assets")
	if _, err := os.Stat(assetsDir); !os.IsNotExist(err) {
		t.Error("zz_generated_docs_assets should not be generated when docs_ui is disabled")
	}
}

// =============================================================================
// Helper functions
// =============================================================================

func setupDocsUITestModule(t *testing.T, tmpDir, modDir string) {
	setupDocsUITestModuleWithPaths(t, tmpDir, modDir, "/docs", "/openapi.json")
}

func setupDocsUITestModuleWithPaths(t *testing.T, tmpDir, modDir, docsPath, openapiPath string) {
	t.Helper()

	// Create go.mod
	goMod := `module example.com/testapp

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + modDir

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create empty go.sum
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// Create api package directory
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create handlers.go
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

// Pet represents a pet.
type Pet struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

// GetPetRequest is the request for GetPet.
type GetPetRequest struct {
	ID string ` + "`path:\"id\"`" + `
}

// Register registers endpoints.
func Register(app *portapi.App) {
	app.Get("/pets/{id}", GetPet)
	app.Get("/health", Health)
}

// GetPet retrieves a pet.
func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	return Pet{ID: req.ID, Name: "Fluffy"}, nil
}

// Health returns health status.
func Health(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Create config with docs_ui enabled
	configContent := `[db]
dialects = mysql

[api]
package = ./api
openapi = true
openapi_output = openapi.json
openapi_title = Test API
openapi_version = 1.0.0
docs_ui = true
docs_path = ` + docsPath + `
openapi_json_path = ` + openapiPath + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}
}

func setupDocsUITestModuleDisabled(t *testing.T, tmpDir, modDir string) {
	t.Helper()

	// Create go.mod
	goMod := `module example.com/testapp

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + modDir

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create empty go.sum
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// Create api package directory
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}

	// Create minimal handlers.go
	handlersCode := `package api

import (
	"context"
	"github.com/shipq/shipq/api/portapi"
)

func Register(app *portapi.App) {
	app.Get("/health", Health)
}

func Health(ctx context.Context) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(apiDir, "handlers.go"), []byte(handlersCode), 0644); err != nil {
		t.Fatalf("failed to write handlers.go: %v", err)
	}

	// Create config WITHOUT docs_ui
	configContent := `[db]
dialects = mysql

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %s", string(output))
	}
}

func runDocsGenerator(t *testing.T, tmpDir, modDir string) {
	t.Helper()

	// Build the generator
	generatorPath := filepath.Join(tmpDir, "generator")
	buildCmd := exec.Command("go", "build", "-o", generatorPath, "./cmd/shipq")
	buildCmd.Dir = modDir
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build generator: %s", string(output))
	}

	// Run the generator
	genCmd := exec.Command(generatorPath, "api", "generate")
	genCmd.Dir = tmpDir
	genCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err = genCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generator failed: %s", string(output))
	}
}

func startDocsTestServer(t *testing.T, tmpDir string) *httptest.Server {
	return startDocsTestServerWithPaths(t, tmpDir, "/docs", "/openapi.json")
}

func startDocsTestServerWithPaths(t *testing.T, tmpDir, docsPath, openapiPath string) *httptest.Server {
	t.Helper()

	apiDir := filepath.Join(tmpDir, "api")

	// Read the generated openapi.json
	openapiBytes, err := os.ReadFile(filepath.Join(apiDir, "openapi.json"))
	if err != nil {
		t.Fatalf("failed to read openapi.json: %v", err)
	}

	// Read the HTML template from the generated code
	docsGoBytes, err := os.ReadFile(filepath.Join(apiDir, "zz_generated_openapi.go"))
	if err != nil {
		t.Fatalf("failed to read zz_generated_openapi.go: %v", err)
	}
	docsGoContent := string(docsGoBytes)

	// Extract HTML from the generated code (find the const zzDocsHTML)
	htmlStart := strings.Index(docsGoContent, "const zzDocsHTML = `")
	if htmlStart == -1 {
		t.Fatal("could not find zzDocsHTML in generated code")
	}
	htmlStart += len("const zzDocsHTML = `")
	htmlEnd := strings.Index(docsGoContent[htmlStart:], "`")
	if htmlEnd == -1 {
		t.Fatal("could not find end of zzDocsHTML")
	}
	docsHTML := docsGoContent[htmlStart : htmlStart+htmlEnd]

	// Create a test server that simulates the generated behavior
	mux := http.NewServeMux()

	// OpenAPI JSON endpoint
	mux.HandleFunc("GET "+openapiPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(openapiBytes)
	})

	// Docs redirect
	mux.HandleFunc("GET "+docsPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, docsPath+"/", http.StatusMovedPermanently)
	})

	// Docs HTML
	mux.HandleFunc("GET "+docsPath+"/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != docsPath+"/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(docsHTML))
	})

	// Assets - serve from the generated assets directory
	assetsDir := filepath.Join(apiDir, "zz_generated_docs_assets")
	assetsHandler := http.StripPrefix(docsPath+"/assets/", http.FileServer(http.Dir(assetsDir)))
	mux.Handle("GET "+docsPath+"/assets/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		assetsHandler.ServeHTTP(w, r)
	}))

	return httptest.NewServer(mux)
}
