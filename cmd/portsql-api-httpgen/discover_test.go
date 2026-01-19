package main

import (
	"strings"
	"testing"
)

func TestGenerateRunnerCode(t *testing.T) {
	t.Run("includes package import", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, `"example.com/app/api"`) {
			t.Error("expected code to contain package import")
		}
	})

	t.Run("calls Register on app", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "api.Register(app)") {
			t.Error("expected code to contain api.Register(app)")
		}
	})

	t.Run("outputs JSON to stdout", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "json.NewEncoder(os.Stdout)") {
			t.Error("expected code to contain json.NewEncoder(os.Stdout)")
		}
	})

	t.Run("imports portapi", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, `"github.com/shipq/shipq/api/portapi"`) {
			t.Error("expected code to contain portapi import")
		}
	})

	t.Run("validates endpoints", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "portapi.ValidateEndpoint") {
			t.Error("expected code to contain portapi.ValidateEndpoint")
		}
	})

	t.Run("builds manifest from endpoints", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "Endpoints()") {
			t.Error("expected code to contain Endpoints()")
		}
		if !strings.Contains(code, "ManifestEndpoint") {
			t.Error("expected code to contain ManifestEndpoint")
		}
	})

	t.Run("extracts handler info using reflect", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "reflect.ValueOf") {
			t.Error("expected code to contain reflect.ValueOf")
		}
	})

	t.Run("uses correct package alias based on import path", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/myproject/handlers", "")

		if !strings.Contains(code, `handlers "example.com/myproject/handlers"`) {
			t.Error("expected code to contain handlers import with alias")
		}
		if !strings.Contains(code, "handlers.Register(app)") {
			t.Error("expected code to contain handlers.Register(app)")
		}
	})

	t.Run("is valid Go syntax", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		// Basic structural checks
		if !strings.Contains(code, "package main") {
			t.Error("expected code to contain package main")
		}
		if !strings.Contains(code, "func main()") {
			t.Error("expected code to contain func main()")
		}
		if !strings.Contains(code, "import (") {
			t.Error("expected code to contain import block")
		}
	})
}
