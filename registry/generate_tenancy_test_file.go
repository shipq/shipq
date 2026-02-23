package registry

import (
	"fmt"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/resourcegen"
	"github.com/shipq/shipq/dburl"
)

// generateTenancyTests generates tenancy isolation tests for resources with scope columns.
func generateTenancyTests(cfg CompileConfig) error {
	// Detect full resources
	resources := resourcegen.DetectFullResources(cfg.Handlers)
	fullResources := resourcegen.FilterFullResources(resources)

	if len(fullResources) == 0 {
		return nil
	}

	// Derive test database URL
	testDatabaseURL := ""
	if cfg.DatabaseURL != "" {
		if u, err := dburl.TestDatabaseURL(cfg.DatabaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	for _, resource := range fullResources {
		// Only generate tenancy tests for scoped resources
		scopeColumn, hasScope := cfg.TableScopes[resource.PackageName]
		if !hasScope || scopeColumn == "" {
			continue
		}

		testCfg := resourcegen.TenancyTestGenConfig{
			ModulePath:      cfg.ModulePath,
			OutputPkg:       cfg.OutputPkg,
			Dialect:         cfg.DBDialect,
			TestDatabaseURL: testDatabaseURL,
			ScopeColumn:     scopeColumn,
		}

		testCode, err := resourcegen.GenerateTenancyTest(testCfg, resource)
		if err != nil {
			return fmt.Errorf("failed to generate tenancy test for %s: %w", resource.PackageName, err)
		}

		// Place the test next to the resource's other tests
		resourceDir := extractResourceDir(cfg.GoModRoot, cfg.ModulePath, resource.PackagePath)
		testDir := filepath.Join(resourceDir, "spec")

		if err := codegen.EnsureDir(testDir); err != nil {
			return fmt.Errorf("failed to create test directory %s: %w", testDir, err)
		}

		testOutputPath := filepath.Join(testDir, "zz_generated_tenancy_test.go")
		written, err := codegen.WriteFileIfChanged(testOutputPath, testCode)
		if err != nil {
			return fmt.Errorf("failed to write tenancy test: %w", err)
		}

		if cfg.Verbose && written {
			fmt.Printf("Generated %s\n", testOutputPath)
		}
	}

	return nil
}
