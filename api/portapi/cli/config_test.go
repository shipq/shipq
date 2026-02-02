package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeShipqConfig is a helper that writes a shipq.ini file and returns the directory path.
func writeShipqConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "shipq.ini")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return tmpDir
}

func TestLoadConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		dir := writeShipqConfig(t, "[api]\npackage = ./api\n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("empty package is allowed at config level", func(t *testing.T) {
		dir := writeShipqConfig(t, "[api]\n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Empty package should be handled by caller (main.go validates this)
		if cfg.Package != "" {
			t.Errorf("got Package %q, want empty string", cfg.Package)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		dir := writeShipqConfig(t, "[api]\npackage =   ./api  \n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("ignores other sections", func(t *testing.T) {
		dir := writeShipqConfig(t, "[db]\nurl = postgres://localhost/mydb\n[api]\npackage = ./myapi\n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./myapi" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./myapi")
		}
	})

	t.Run("ignores comments", func(t *testing.T) {
		dir := writeShipqConfig(t, "# comment\n[api]\n; another comment\npackage = ./api\n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("error if shipq.ini not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := LoadConfig(tmpDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "shipq.ini") {
			t.Errorf("expected error to contain 'shipq.ini', got %q", err.Error())
		}
	})
}

func TestFindConfig(t *testing.T) {
	t.Run("finds shipq.ini in current directory", func(t *testing.T) {
		// Create temp dir with shipq.ini
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "shipq.ini")
		if err := os.WriteFile(cfgPath, []byte("[api]\npackage = ./api\n"), 0644); err != nil {
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

		_, err = FindConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error if no shipq.ini found", func(t *testing.T) {
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

		_, err = FindConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "shipq.ini") {
			t.Errorf("expected error to contain 'shipq.ini', got %q", err.Error())
		}
	})
}

func TestLoadConfig_MiddlewarePackage(t *testing.T) {
	t.Run("missing middleware_package is valid", func(t *testing.T) {
		dir := writeShipqConfig(t, "[api]\npackage = ./api\n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MiddlewarePackage != "" {
			t.Errorf("got MiddlewarePackage %q, want empty string", cfg.MiddlewarePackage)
		}
	})

	t.Run("present middleware_package is parsed", func(t *testing.T) {
		dir := writeShipqConfig(t, "[api]\npackage = ./api\nmiddleware_package = ./middleware\n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MiddlewarePackage != "./middleware" {
			t.Errorf("got MiddlewarePackage %q, want %q", cfg.MiddlewarePackage, "./middleware")
		}
	})

	t.Run("middleware_package trims whitespace", func(t *testing.T) {
		dir := writeShipqConfig(t, "[api]\npackage = ./api\nmiddleware_package =   ./mw  \n")

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MiddlewarePackage != "./mw" {
			t.Errorf("got MiddlewarePackage %q, want %q", cfg.MiddlewarePackage, "./mw")
		}
	})
}

// Test defaults when no optional keys are provided
func TestLoadConfig_Defaults(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
middleware_package = ./middleware
`)

	cfg, err := LoadConfig(dir)
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
	// When docs_ui is not enabled, openapi_json_path keeps its default from unified config
	if cfg.OpenAPIJSONPath != "/openapi.json" {
		t.Errorf("got OpenAPIJSONPath %q, want %q", cfg.OpenAPIJSONPath, "/openapi.json")
	}
	if cfg.DocsPath != "" {
		t.Errorf("got DocsPath %q, want empty", cfg.DocsPath)
	}
}

func TestLoadConfig_OpenAPIEnabled(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.OpenAPIEnabled {
		t.Error("expected OpenAPIEnabled to be true")
	}
}

func TestLoadConfig_OpenAPIOutput(t *testing.T) {
	t.Run("custom output", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_output = custom.json
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIOutput != "custom.json" {
			t.Errorf("got OpenAPIOutput %q, want %q", cfg.OpenAPIOutput, "custom.json")
		}
	})

	t.Run("default output", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIOutput != "openapi.json" {
			t.Errorf("got OpenAPIOutput %q, want %q", cfg.OpenAPIOutput, "openapi.json")
		}
	})

	t.Run("absolute path rejected", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_output = /absolute/path.json
`)

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for absolute path, got nil")
		}
		if !strings.Contains(err.Error(), "relative") {
			t.Errorf("expected error about relative path, got %q", err.Error())
		}
	})
}

func TestLoadConfig_DocsUIImpliesOpenAPI(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /docs
`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.OpenAPIEnabled {
		t.Error("docs_ui=true should imply openapi=true")
	}
	if !cfg.DocsUIEnabled {
		t.Error("DocsUIEnabled should be true")
	}
}

func TestLoadConfig_DocsPathNormalization(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /docs/
`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Trailing slash should be removed
	if cfg.DocsPath != "/docs" {
		t.Errorf("got DocsPath %q, want %q", cfg.DocsPath, "/docs")
	}
}

func TestLoadConfig_DocsPathRequired(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
`)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error when docs_ui is enabled without docs_path")
	}
	if !strings.Contains(err.Error(), "docs_path") {
		t.Errorf("expected error about docs_path, got %q", err.Error())
	}
}

func TestLoadConfig_DocsPathMustBeAbsolute(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = docs
`)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error when docs_path doesn't start with /")
	}
	if !strings.Contains(err.Error(), "start with /") {
		t.Errorf("expected error about leading slash, got %q", err.Error())
	}
}

func TestLoadConfig_DocsPathRejectsRoot(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /
`)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error when docs_path is /")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Errorf("expected error about root path, got %q", err.Error())
	}
}

func TestLoadConfig_OpenAPIJSONPath(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /docs
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIJSONPath != "/openapi.json" {
			t.Errorf("got OpenAPIJSONPath %q, want %q", cfg.OpenAPIJSONPath, "/openapi.json")
		}
	})

	t.Run("custom value", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = /api/spec.json
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIJSONPath != "/api/spec.json" {
			t.Errorf("got OpenAPIJSONPath %q, want %q", cfg.OpenAPIJSONPath, "/api/spec.json")
		}
	})

	t.Run("must start with slash", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = openapi.json
`)

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error when openapi_json_path doesn't start with /")
		}
		if !strings.Contains(err.Error(), "start with /") {
			t.Errorf("expected error about leading slash, got %q", err.Error())
		}
	})

	t.Run("must not end with slash", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
docs_ui = true
docs_path = /docs
openapi_json_path = /openapi/
`)

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error when openapi_json_path ends with /")
		}
		if !strings.Contains(err.Error(), "trailing slash") {
			t.Errorf("expected error about trailing slash, got %q", err.Error())
		}
	})
}

func TestLoadConfig_OpenAPIServers(t *testing.T) {
	t.Run("single server", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_servers = http://localhost:8080
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{"http://localhost:8080"}
		if !reflect.DeepEqual(cfg.OpenAPIServers, expected) {
			t.Errorf("got OpenAPIServers %v, want %v", cfg.OpenAPIServers, expected)
		}
	})

	t.Run("multiple servers", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_servers = http://localhost:8080, https://api.example.com
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{"http://localhost:8080", "https://api.example.com"}
		if !reflect.DeepEqual(cfg.OpenAPIServers, expected) {
			t.Errorf("got OpenAPIServers %v, want %v", cfg.OpenAPIServers, expected)
		}
	})

	t.Run("empty value", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cfg.OpenAPIServers) != 0 {
			t.Errorf("got OpenAPIServers %v, want empty", cfg.OpenAPIServers)
		}
	})
}

func TestLoadConfig_OpenAPITitle(t *testing.T) {
	t.Run("derived from package", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./internal/api
openapi = true
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPITitle != "api" {
			t.Errorf("got OpenAPITitle %q, want %q", cfg.OpenAPITitle, "api")
		}
	})

	t.Run("explicit title", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_title = My Cool API
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPITitle != "My Cool API" {
			t.Errorf("got OpenAPITitle %q, want %q", cfg.OpenAPITitle, "My Cool API")
		}
	})
}

func TestLoadConfig_OpenAPIVersion(t *testing.T) {
	t.Run("default version", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIVersion != "0.0.0" {
			t.Errorf("got OpenAPIVersion %q, want %q", cfg.OpenAPIVersion, "0.0.0")
		}
	})

	t.Run("explicit version", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_version = 1.2.3
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.OpenAPIVersion != "1.2.3" {
			t.Errorf("got OpenAPIVersion %q, want %q", cfg.OpenAPIVersion, "1.2.3")
		}
	})
}

func TestLoadConfig_OpenAPIDescription(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
openapi = true
openapi_description = This is my API description
`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OpenAPIDescription != "This is my API description" {
		t.Errorf("got OpenAPIDescription %q, want %q", cfg.OpenAPIDescription, "This is my API description")
	}
}

func TestLoadConfig_BooleanParsing(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true lowercase", "true", true},
		{"TRUE uppercase", "TRUE", true},
		{"True mixed case", "True", true},
		{"1", "1", true},
		{"false lowercase", "false", false},
		{"FALSE uppercase", "FALSE", false},
		{"False mixed case", "False", false},
		{"0", "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeShipqConfig(t, `[api]
package = ./api
test_client = `+tt.value+`
`)

			cfg, err := LoadConfig(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.TestClientEnabled != tt.want {
				t.Errorf("got TestClientEnabled %v, want %v", cfg.TestClientEnabled, tt.want)
			}
		})
	}
}

func TestLoadConfig_InvalidBoolean(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
test_client = yes
`)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid boolean, got nil")
	}
	if !strings.Contains(err.Error(), "test_client") {
		t.Errorf("expected error to mention test_client, got %q", err.Error())
	}
}

func TestLoadConfig_TestClientFilename(t *testing.T) {
	t.Run("default filename", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
test_client = true
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.TestClientFilename != "zz_generated_testclient_test.go" {
			t.Errorf("got TestClientFilename %q, want %q", cfg.TestClientFilename, "zz_generated_testclient_test.go")
		}
	})

	t.Run("custom filename", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
test_client = true
test_client_filename = my_client_test.go
`)

		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.TestClientFilename != "my_client_test.go" {
			t.Errorf("got TestClientFilename %q, want %q", cfg.TestClientFilename, "my_client_test.go")
		}
	})

	t.Run("invalid filename without _test.go suffix", func(t *testing.T) {
		dir := writeShipqConfig(t, `[api]
package = ./api
test_client = true
test_client_filename = client.go
`)

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for invalid filename, got nil")
		}
		if !strings.Contains(err.Error(), "_test.go") {
			t.Errorf("expected error to mention '_test.go', got %q", err.Error())
		}
	})
}

func TestLoadConfig_TestClientEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		want    bool
		wantErr bool
	}{
		{
			name: "enabled",
			config: `[api]
package = ./api
test_client = true
`,
			want: true,
		},
		{
			name: "disabled",
			config: `[api]
package = ./api
test_client = false
`,
			want: false,
		},
		{
			name: "not specified",
			config: `[api]
package = ./api
`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeShipqConfig(t, tt.config)

			cfg, err := LoadConfig(dir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.TestClientEnabled != tt.want {
				t.Errorf("got TestClientEnabled %v, want %v", cfg.TestClientEnabled, tt.want)
			}
		})
	}
}

// Test that error messages reference shipq.ini
func TestLoadConfig_ErrorMessages(t *testing.T) {
	t.Run("invalid dialect error mentions shipq.ini", func(t *testing.T) {
		dir := writeShipqConfig(t, `[db]
dialects = invalid_dialect
[api]
package = ./api
`)

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "shipq.ini") {
			t.Errorf("expected error to mention 'shipq.ini', got %q", err.Error())
		}
	})
}

// Test deriveTitle function
func TestDeriveTitle(t *testing.T) {
	tests := []struct {
		pkg  string
		want string
	}{
		{"./api", "api"},
		{"./internal/api", "api"},
		{"api", "api"},
		{"github.com/example/project/api", "api"},
		{"./", ""},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			got := deriveTitle(tt.pkg)
			if got != tt.want {
				t.Errorf("deriveTitle(%q) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

// Test full config with all options
func TestLoadConfig_AllOptions(t *testing.T) {
	dir := writeShipqConfig(t, `[api]
package = ./api
middleware_package = ./middleware
openapi = true
openapi_output = custom_api.json
openapi_title = My API
openapi_version = 2.0.0
openapi_description = A custom API description
openapi_servers = http://localhost:8080, https://prod.example.com
docs_ui = true
docs_path = /api/docs
openapi_json_path = /api/spec.json
test_client = true
test_client_filename = api_client_test.go
`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields
	if cfg.Package != "./api" {
		t.Errorf("Package = %q, want %q", cfg.Package, "./api")
	}
	if cfg.MiddlewarePackage != "./middleware" {
		t.Errorf("MiddlewarePackage = %q, want %q", cfg.MiddlewarePackage, "./middleware")
	}
	if !cfg.OpenAPIEnabled {
		t.Error("OpenAPIEnabled should be true")
	}
	if cfg.OpenAPIOutput != "custom_api.json" {
		t.Errorf("OpenAPIOutput = %q, want %q", cfg.OpenAPIOutput, "custom_api.json")
	}
	if cfg.OpenAPITitle != "My API" {
		t.Errorf("OpenAPITitle = %q, want %q", cfg.OpenAPITitle, "My API")
	}
	if cfg.OpenAPIVersion != "2.0.0" {
		t.Errorf("OpenAPIVersion = %q, want %q", cfg.OpenAPIVersion, "2.0.0")
	}
	if cfg.OpenAPIDescription != "A custom API description" {
		t.Errorf("OpenAPIDescription = %q, want %q", cfg.OpenAPIDescription, "A custom API description")
	}
	expectedServers := []string{"http://localhost:8080", "https://prod.example.com"}
	if !reflect.DeepEqual(cfg.OpenAPIServers, expectedServers) {
		t.Errorf("OpenAPIServers = %v, want %v", cfg.OpenAPIServers, expectedServers)
	}
	if !cfg.DocsUIEnabled {
		t.Error("DocsUIEnabled should be true")
	}
	if cfg.DocsPath != "/api/docs" {
		t.Errorf("DocsPath = %q, want %q", cfg.DocsPath, "/api/docs")
	}
	if cfg.OpenAPIJSONPath != "/api/spec.json" {
		t.Errorf("OpenAPIJSONPath = %q, want %q", cfg.OpenAPIJSONPath, "/api/spec.json")
	}
	if !cfg.TestClientEnabled {
		t.Error("TestClientEnabled should be true")
	}
	if cfg.TestClientFilename != "api_client_test.go" {
		t.Errorf("TestClientFilename = %q, want %q", cfg.TestClientFilename, "api_client_test.go")
	}
}
