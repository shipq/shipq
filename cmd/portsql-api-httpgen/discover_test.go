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

// =============================================================================
// OpenAPI Step 2 Tests: Runner code generation for Types and Docs
// =============================================================================

func TestGenerateRunnerCode_TypeGraph(t *testing.T) {
	t.Run("includes ManifestType struct", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "type ManifestType struct") {
			t.Error("expected code to contain ManifestType struct")
		}
	})

	t.Run("includes ManifestField struct", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "type ManifestField struct") {
			t.Error("expected code to contain ManifestField struct")
		}
	})

	t.Run("includes ManifestDoc struct", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "type ManifestDoc struct") {
			t.Error("expected code to contain ManifestDoc struct")
		}
	})

	t.Run("manifest has Types field", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "Types") && !strings.Contains(code, `"types"`) {
			t.Error("expected Manifest to have Types field")
		}
	})

	t.Run("manifest has EndpointDocs field", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "EndpointDocs") && !strings.Contains(code, `"endpoint_docs"`) {
			t.Error("expected Manifest to have EndpointDocs field")
		}
	})

	t.Run("includes type collection function", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "collectTypes") {
			t.Error("expected code to contain collectTypes function")
		}
	})

	t.Run("includes doc extraction function", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "extractDocs") {
			t.Error("expected code to contain extractDocs function")
		}
	})

	t.Run("imports go/packages for doc extraction", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, `"golang.org/x/tools/go/packages"`) {
			t.Error("expected code to import go/packages")
		}
	})
}

func TestGenerateRunnerCode_TypeID(t *testing.T) {
	t.Run("includes getTypeID function", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		if !strings.Contains(code, "getTypeID") {
			t.Error("expected code to contain getTypeID helper function")
		}
	})
}

func TestGenerateRunnerCode_ValidationForConflictingBindings(t *testing.T) {
	t.Run("includes binding conflict validation", func(t *testing.T) {
		code := GenerateRunnerCode("example.com/app/api", "")

		// Should validate that fields don't have multiple binding sources
		if !strings.Contains(code, "validateBindingConflicts") && !strings.Contains(code, "multiple binding") {
			t.Error("expected code to validate binding conflicts")
		}
	})
}
