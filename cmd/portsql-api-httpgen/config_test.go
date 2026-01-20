package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("missing section", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("package = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "[httpgen]") {
			t.Errorf("expected error to contain '[httpgen]', got %q", err.Error())
		}
	})

	t.Run("missing package key", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "package") {
			t.Errorf("expected error to contain 'package', got %q", err.Error())
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage =   ./api  \n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("ignores other sections", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[other]\nfoo = bar\n[httpgen]\npackage = ./myapi\n[another]\nbaz = qux\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./myapi" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./myapi")
		}
	})

	t.Run("ignores comments", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("# comment\n[httpgen]\n; another comment\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("error if file not found", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/path/config.ini")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestFindConfig(t *testing.T) {
	t.Run("uses env var if set", func(t *testing.T) {
		// Create temp file
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "custom.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Set env var
		t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", cfgPath)

		path, err := FindConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != cfgPath {
			t.Errorf("got path %q, want %q", path, cfgPath)
		}
	})

	t.Run("falls back to ./portsql-api-httpgen.ini", func(t *testing.T) {
		// Create temp dir with ini file
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "portsql-api-httpgen.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Change to temp dir
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		defer os.Chdir(origDir)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Clear env var
		t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", "")

		path, err := FindConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "./portsql-api-httpgen.ini" {
			t.Errorf("got path %q, want %q", path, "./portsql-api-httpgen.ini")
		}
	})

	t.Run("error if no config found", func(t *testing.T) {
		// Empty temp dir
		tmpDir := t.TempDir()

		// Change to temp dir
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		defer os.Chdir(origDir)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Clear env var
		t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", "")

		_, err = FindConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "config not found") {
			t.Errorf("expected error to contain 'config not found', got %q", err.Error())
		}
	})
}

func TestLoadConfig_MiddlewarePackage(t *testing.T) {
	t.Run("missing middleware_package is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MiddlewarePackage != "" {
			t.Errorf("got MiddlewarePackage %q, want empty string", cfg.MiddlewarePackage)
		}
	})

	t.Run("present middleware_package is parsed", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		cfgContent := "[httpgen]\npackage = ./api\nmiddleware_package = ./middleware\n"
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MiddlewarePackage != "./middleware" {
			t.Errorf("got MiddlewarePackage %q, want %q", cfg.MiddlewarePackage, "./middleware")
		}
	})

	t.Run("middleware_package trims whitespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		cfgContent := "[httpgen]\npackage = ./api\nmiddleware_package =   ./mw  \n"
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MiddlewarePackage != "./mw" {
			t.Errorf("got MiddlewarePackage %q, want %q", cfg.MiddlewarePackage, "./mw")
		}
	})
}

// writeTestConfig is a helper that writes an INI file and returns its path.
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test.ini")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return cfgPath
}

// Test 1: Backwards compatibility (no new keys)
func TestLoadConfig_BackwardsCompatibility(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
middleware_package = ./middleware
`)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Existing fields
	if cfg.Package != "./api" {
		t.Errorf("got Package %q, want %q", cfg.Package, "./api")
	}
	if cfg.MiddlewarePackage != "./middleware" {
		t.Errorf("got MiddlewarePackage %q, want %q", cfg.MiddlewarePackage, "./middleware")
	}

	// New fields should have defaults
	if cfg.OpenAPIEnabled != false {
		t.Errorf("got OpenAPIEnabled %v, want false", cfg.OpenAPIEnabled)
	}
	if cfg.DocsUIEnabled != false {
		t.Errorf("got DocsUIEnabled %v, want false", cfg.DocsUIEnabled)
	}
	if cfg.OpenAPIOutput != "openapi.json" {
		t.Errorf("got OpenAPIOutput %q, want %q", cfg.OpenAPIOutput, "openapi.json")
	}
	if cfg.OpenAPIVersion != "0.0.0" {
		t.Errorf("got OpenAPIVersion %q, want %q", cfg.OpenAPIVersion, "0.0.0")
	}
	if cfg.OpenAPITitle != "api" {
		t.Errorf("got OpenAPITitle %q, want %q", cfg.OpenAPITitle, "api")
	}
	if cfg.OpenAPIJSONPath != "" {
		t.Errorf("got OpenAPIJSONPath %q, want empty", cfg.OpenAPIJSONPath)
	}
	if cfg.DocsPath != "" {
		t.Errorf("got DocsPath %q, want empty", cfg.DocsPath)
	}
}

// Test 2: Enable OpenAPI only
func TestLoadConfig_OpenAPIEnabled(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
`)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OpenAPIEnabled != true {
		t.Errorf("got OpenAPIEnabled %v, want true", cfg.OpenAPIEnabled)
	}
	if cfg.OpenAPIOutput != "openapi.json" {
		t.Errorf("got OpenAPIOutput %q, want %q", cfg.OpenAPIOutput, "openapi.json")
	}
	if cfg.DocsUIEnabled != false {
		t.Errorf("got DocsUIEnabled %v, want false", cfg.DocsUIEnabled)
	}
}

// Test 3: Override openapi_output filename
func TestLoadConfig_OpenAPIOutput(t *testing.T) {
	t.Run("custom filename", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_output = my-openapi.json
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIOutput != "my-openapi.json" {
			t.Errorf("got OpenAPIOutput %q, want %q", cfg.OpenAPIOutput, "my-openapi.json")
		}
	})

	t.Run("rejects absolute path", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_output = /tmp/openapi.json
`)

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be relative") {
			t.Errorf("expected error to contain 'must be relative', got %q", err.Error())
		}
	})
}

// Test 4: Enable docs_ui implies OpenAPIEnabled
func TestLoadConfig_DocsUIImpliesOpenAPI(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /docs
`)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DocsUIEnabled != true {
		t.Errorf("got DocsUIEnabled %v, want true", cfg.DocsUIEnabled)
	}
	if cfg.DocsPath != "/docs" {
		t.Errorf("got DocsPath %q, want %q", cfg.DocsPath, "/docs")
	}
	if cfg.OpenAPIEnabled != true {
		t.Errorf("got OpenAPIEnabled %v, want true (implied by docs_ui)", cfg.OpenAPIEnabled)
	}
	if cfg.OpenAPIJSONPath != "/openapi.json" {
		t.Errorf("got OpenAPIJSONPath %q, want %q", cfg.OpenAPIJSONPath, "/openapi.json")
	}
}

// Test 5: docs_path normalization removes trailing slash
func TestLoadConfig_DocsPathNormalization(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /docs/
`)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DocsPath != "/docs" {
		t.Errorf("got DocsPath %q, want %q (trailing slash removed)", cfg.DocsPath, "/docs")
	}
}

// Test 6: docs_path validation rejects empty/missing
func TestLoadConfig_DocsPathRequired(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
`)

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "docs_path") {
		t.Errorf("expected error to mention 'docs_path', got %q", err.Error())
	}
}

// Test 7: docs_path validation rejects non-absolute path
func TestLoadConfig_DocsPathMustBeAbsolute(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = docs
`)

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must start with /") {
		t.Errorf("expected error to contain 'must start with /', got %q", err.Error())
	}
}

// Test 8: reject docs_path="/"
func TestLoadConfig_DocsPathRejectsRoot(t *testing.T) {
	cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /
`)

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "docs_path") {
		t.Errorf("expected error to mention 'docs_path', got %q", err.Error())
	}
}

// Test 9: openapi_json_path defaulting and validation
func TestLoadConfig_OpenAPIJSONPath(t *testing.T) {
	t.Run("defaults when docs enabled", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /docs
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIJSONPath != "/openapi.json" {
			t.Errorf("got OpenAPIJSONPath %q, want %q", cfg.OpenAPIJSONPath, "/openapi.json")
		}
	})

	t.Run("override path", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = /internal/openapi.json
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIJSONPath != "/internal/openapi.json" {
			t.Errorf("got OpenAPIJSONPath %q, want %q", cfg.OpenAPIJSONPath, "/internal/openapi.json")
		}
	})

	t.Run("rejects no leading slash", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = openapi.json
`)

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must start with /") {
			t.Errorf("expected error to contain 'must start with /', got %q", err.Error())
		}
	})

	t.Run("rejects trailing slash", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = /openapi/
`)

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "trailing slash") {
			t.Errorf("expected error to contain 'trailing slash', got %q", err.Error())
		}
	})
}

// Test 10: openapi_servers parsing
func TestLoadConfig_OpenAPIServers(t *testing.T) {
	t.Run("parses comma-separated list", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_servers = http://localhost:8080, https://api.example.com
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{"http://localhost:8080", "https://api.example.com"}
		if !reflect.DeepEqual(cfg.OpenAPIServers, want) {
			t.Errorf("got OpenAPIServers %v, want %v", cfg.OpenAPIServers, want)
		}
	})

	t.Run("empty entries are dropped", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_servers = , ,
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cfg.OpenAPIServers) != 0 {
			t.Errorf("got OpenAPIServers %v, want empty slice", cfg.OpenAPIServers)
		}
	})

	t.Run("allows relative server URL", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_servers = /
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{"/"}
		if !reflect.DeepEqual(cfg.OpenAPIServers, want) {
			t.Errorf("got OpenAPIServers %v, want %v", cfg.OpenAPIServers, want)
		}
	})
}

// Test: OpenAPI title derivation
func TestLoadConfig_OpenAPITitle(t *testing.T) {
	t.Run("derives from package path", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPITitle != "api" {
			t.Errorf("got OpenAPITitle %q, want %q", cfg.OpenAPITitle, "api")
		}
	})

	t.Run("derives from longer path", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = github.com/example/myproject/internal/api
openapi = true
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPITitle != "api" {
			t.Errorf("got OpenAPITitle %q, want %q", cfg.OpenAPITitle, "api")
		}
	})

	t.Run("explicit title overrides derived", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_title = My Custom API
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPITitle != "My Custom API" {
			t.Errorf("got OpenAPITitle %q, want %q", cfg.OpenAPITitle, "My Custom API")
		}
	})
}

// Test: OpenAPI version
func TestLoadConfig_OpenAPIVersion(t *testing.T) {
	t.Run("defaults to 0.0.0", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIVersion != "0.0.0" {
			t.Errorf("got OpenAPIVersion %q, want %q", cfg.OpenAPIVersion, "0.0.0")
		}
	})

	t.Run("explicit version", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_version = 1.2.3
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIVersion != "1.2.3" {
			t.Errorf("got OpenAPIVersion %q, want %q", cfg.OpenAPIVersion, "1.2.3")
		}
	})
}

// Test: OpenAPI description
func TestLoadConfig_OpenAPIDescription(t *testing.T) {
	t.Run("defaults to empty", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIDescription != "" {
			t.Errorf("got OpenAPIDescription %q, want empty", cfg.OpenAPIDescription)
		}
	})

	t.Run("explicit description", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
openapi_description = This is my API description
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIDescription != "This is my API description" {
			t.Errorf("got OpenAPIDescription %q, want %q", cfg.OpenAPIDescription, "This is my API description")
		}
	})
}

// Test: Boolean parsing accepts true/false
func TestLoadConfig_BooleanParsing(t *testing.T) {
	t.Run("accepts true", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIEnabled != true {
			t.Errorf("got OpenAPIEnabled %v, want true", cfg.OpenAPIEnabled)
		}
	})

	t.Run("accepts false", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = false
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIEnabled != false {
			t.Errorf("got OpenAPIEnabled %v, want false", cfg.OpenAPIEnabled)
		}
	})

	t.Run("accepts 1", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = 1
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIEnabled != true {
			t.Errorf("got OpenAPIEnabled %v, want true", cfg.OpenAPIEnabled)
		}
	})

	t.Run("accepts 0", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = 0
`)

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIEnabled != false {
			t.Errorf("got OpenAPIEnabled %v, want false", cfg.OpenAPIEnabled)
		}
	})

	t.Run("rejects invalid boolean", func(t *testing.T) {
		cfgPath := writeTestConfig(t, `[httpgen]
package = ./api
openapi = yes
`)

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid boolean") {
			t.Errorf("expected error to contain 'invalid boolean', got %q", err.Error())
		}
	})
}
