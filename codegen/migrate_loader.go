package codegen

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shipq/shipq/db/portsql/migrate"
)

// MigrationFile represents a discovered migration file.
type MigrationFile struct {
	Path      string // Full path to the file
	Timestamp string // 14-digit timestamp
	Name      string // Name after timestamp (e.g., "users")
	FuncName  string // Full function name (e.g., "Migrate_20260115120000_users")
}

// DiscoverMigrations finds all migration files in the migrations directory.
// Returns them sorted by timestamp (ascending).
func DiscoverMigrations(migrationsPath string) ([]MigrationFile, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No migrations directory = no migrations
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []MigrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		// Expected format: TIMESTAMP_name.go (e.g., 20260115120000_users.go)
		baseName := strings.TrimSuffix(name, ".go")
		if len(baseName) < 16 { // 14 digits + underscore + at least 1 char
			continue
		}

		timestamp := baseName[:14]
		// Validate timestamp is all digits
		valid := true
		for _, c := range timestamp {
			if c < '0' || c > '9' {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}

		if baseName[14] != '_' {
			continue
		}

		migrationName := baseName[15:]
		funcName := fmt.Sprintf("Migrate_%s_%s", timestamp, migrationName)

		migrations = append(migrations, MigrationFile{
			Path:      filepath.Join(migrationsPath, name),
			Timestamp: timestamp,
			Name:      migrationName,
			FuncName:  funcName,
		})
	}

	// Sort by timestamp (lexicographic = chronological for this format)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Timestamp < migrations[j].Timestamp
	})

	return migrations, nil
}

// BuildMigrationPlan executes all migration functions and returns the JSON plan.
// This creates a temporary Go program that imports the migrations package and
// calls each migration function in order.
func BuildMigrationPlan(projectRoot, modulePath, migrationsPath string, migrations []MigrationFile) ([]byte, error) {
	if len(migrations) == 0 {
		// Return empty plan
		return []byte(`{"schema":{"name":"","tables":{}},"migrations":[]}`), nil
	}

	// Create temporary directory for the runner
	tmpDir, err := os.MkdirTemp("", "shipq-migrate-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine migrations package path
	relMigrationsPath, err := filepath.Rel(projectRoot, migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative migrations path: %w", err)
	}
	migrationsImportPath := modulePath + "/" + filepath.ToSlash(relMigrationsPath)

	// Generate the runner main.go
	runnerCode := generateMigrationRunner(migrationsImportPath, migrations)
	runnerPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write runner: %w", err)
	}

	// Generate go.mod that requires the user's module
	goModContent := fmt.Sprintf(`module shipq-migrate-runner

go 1.21

require %s v0.0.0

replace %s => %s
`, modulePath, modulePath, projectRoot)

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Run go mod tidy to resolve dependencies
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return nil, fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Run the migration runner
	runCmd := exec.Command("go", "run", ".")
	runCmd.Dir = tmpDir
	output, err := runCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("migration runner failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("migration runner failed: %w", err)
	}

	return output, nil
}

// generateMigrationRunner generates Go code that executes all migrations.
func generateMigrationRunner(migrationsImportPath string, migrations []MigrationFile) string {
	var buf strings.Builder

	buf.WriteString(`package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/shipq/shipq/db/portsql/migrate"
	migrations "`)
	buf.WriteString(migrationsImportPath)
	buf.WriteString(`"
)

func main() {
	plan := migrate.NewPlan()

	var err error
`)

	for _, m := range migrations {
		// Build the migration name from the file: TIMESTAMP_name
		migrationName := m.Timestamp + "_" + m.Name
		buf.WriteString(fmt.Sprintf(`
	// Set the migration name before calling the migration function
	// This ensures the migration name matches the filename and is stable across rebuilds
	plan.SetCurrentMigration(%q)
	err = migrations.%s(plan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration %s failed: %%v\n", err)
		os.Exit(1)
	}
`, migrationName, m.FuncName, m.FuncName))
	}

	buf.WriteString(`
	// Output the plan as JSON
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to serialize plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(data))
}
`)

	return buf.String()
}

// GenerateMigrationRunnerForTest is an exported version of generateMigrationRunner for testing.
// It generates the Go code that executes migrations, allowing tests to verify the generated code
// includes proper SetCurrentMigration calls.
func GenerateMigrationRunnerForTest(migrations []MigrationFile) string {
	return generateMigrationRunner("example.com/test/migrations", migrations)
}

// LoadMigrationPlan loads the migration plan from schema.json.
// Returns nil if schema.json doesn't exist.
func LoadMigrationPlan(projectRoot string) (*migrate.MigrationPlan, error) {
	schemaPath := filepath.Join(projectRoot, "shipq", "db", "migrate", "schema.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("schema.json not found - run 'shipq migrate up' first")
		}
		return nil, fmt.Errorf("failed to read schema.json: %w", err)
	}

	var plan migrate.MigrationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse schema.json: %w", err)
	}

	return &plan, nil
}
