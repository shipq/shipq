package registry

import (
	"github.com/shipq/shipq/codegen"
)

// CompileConfig holds all configuration needed for registry compilation.
type CompileConfig struct {
	ProjectRoot string
	ModulePath  string
	Handlers    []codegen.SerializedHandlerInfo
	// OutputPkg is the package name for generated HTTP server code (e.g., "api").
	// Defaults to "api" if empty.
	OutputPkg string
	// OutputDir is the directory for generated HTTP server code relative to ProjectRoot.
	// Defaults to "api" if empty.
	OutputDir string
	// DBDialect is the database dialect for main.go generation ("mysql", "postgres", "sqlite").
	// Defaults to "mysql" if empty.
	DBDialect string
	// Port is the server port for main.go. Defaults to "8080" if empty.
	Port string
	// GenerateResourceTests enables generation of CRUD tests for full resources.
	// A "full resource" is a package that implements all 5 CRUD operations.
	GenerateResourceTests bool
	// Verbose enables additional logging.
	Verbose bool
}

// CompileRegistry is the central hook for all codegen that depends on the
// handler registry. This function will grow to include:
//
//   - generateHTTPServer() ✓
//   - generateHTTPMain() ✓
//   - generateHTTPTestClient() ✓
//   - generateHTTPTestHarness() ✓
//   - generateResourceTests() ✓
//   - generateOpenAPISpec()
//   - generateTypeScriptClient()
func CompileRegistry(cfg CompileConfig) error {
	setDefaults(&cfg)

	if cfg.Verbose {
		if err := printDebugRegistry(cfg.Handlers); err != nil {
			return err
		}
	}

	if err := generateHTTPServer(cfg); err != nil {
		return err
	}

	if err := generateHTTPMain(cfg); err != nil {
		return err
	}

	// Generate test infrastructure
	if err := generateHTTPTestClient(cfg); err != nil {
		return err
	}

	if err := generateHTTPTestHarness(cfg); err != nil {
		return err
	}

	// Generate resource tests if enabled
	if cfg.GenerateResourceTests {
		if err := generateResourceTests(cfg); err != nil {
			return err
		}
	}

	return nil
}
