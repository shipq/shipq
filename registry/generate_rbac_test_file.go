package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/authgen"
	"github.com/shipq/shipq/dburl"
)

// generateRBACTests generates RBAC integration tests for the API.
// Only runs when auth handlers are detected in the registry.
func generateRBACTests(cfg CompileConfig) error {
	// Only generate if auth handlers exist
	if !hasAuthHandlersInConfig(cfg) {
		return nil
	}

	// Derive test database URL
	testDatabaseURL := ""
	if cfg.DatabaseURL != "" {
		if u, err := dburl.TestDatabaseURL(cfg.DatabaseURL); err == nil {
			testDatabaseURL = u
		}
	}

	// Determine RBAC scope column by checking if the roles migration
	// actually includes organization_id. This handles the case where
	// auth was generated without scope but scope was configured later
	// (e.g., tenancy-scoped pets scenario).
	rbacScopeColumn := ""
	if cfg.ScopeColumn != "" {
		rbacScopeColumn = detectRolesScopeColumn(cfg.ShipqRoot, cfg.ScopeColumn)
	}

	testCfg := authgen.RBACTestGenConfig{
		ModulePath:      cfg.ModulePath,
		OutputPkg:       cfg.OutputPkg,
		Dialect:         cfg.DBDialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     rbacScopeColumn,
		StripPrefix:     cfg.StripPrefix,
	}

	testCode, err := authgen.GenerateRBACTests(testCfg)
	if err != nil {
		return fmt.Errorf("failed to generate RBAC tests: %w", err)
	}

	// Write to api/zz_generated_rbac_test.go
	outputDir := filepath.Join(cfg.ShipqRoot, cfg.OutputDir)
	if err := codegen.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	testOutputPath := filepath.Join(outputDir, "zz_generated_rbac_test.go")
	written, err := codegen.WriteFileIfChanged(testOutputPath, testCode)
	if err != nil {
		return fmt.Errorf("failed to write RBAC test: %w", err)
	}

	if cfg.Verbose && written {
		fmt.Printf("Generated %s\n", testOutputPath)
	}

	return nil
}

// hasAuthHandlersInConfig checks if any handler in the config has an auth package path.
func hasAuthHandlersInConfig(cfg CompileConfig) bool {
	for _, h := range cfg.Handlers {
		if strings.HasSuffix(h.PackagePath, "/api/auth") {
			return true
		}
	}
	return false
}

// detectRolesScopeColumn checks whether the roles migration file actually
// includes the given scope column (e.g., organization_id). Returns the scope
// column if found, or empty string if not. This prevents generating scoped
// RBAC tests when the roles table was created before scope was configured.
func detectRolesScopeColumn(shipqRoot, scopeColumn string) string {
	migrationsDir := filepath.Join(shipqRoot, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_roles.go") {
			content, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
			if err != nil {
				return ""
			}
			if strings.Contains(string(content), scopeColumn) {
				return scopeColumn
			}
			return ""
		}
	}
	return ""
}
