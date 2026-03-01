package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shipq/shipq/db/portsql/migrate"
)

// MigrationFile represents a discovered migration file.
type MigrationFile struct {
	Path      string // Full path to the file
	Timestamp string // 14-digit timestamp
	Name      string // Name after timestamp (e.g., "users")
	FuncName  string // Full function name (e.g., "Migrate_20260115120000_users")
}

// NextMigrationBaseTime returns a base time that is guaranteed to produce
// timestamps strictly after all existing migration files in migrationsPath.
// It scans the directory for the latest timestamp, parses it, and returns
// whichever is later: now or (latestExistingTimestamp + 1 second).
//
// This prevents timestamp collisions when multiple shipq commands (e.g.
// auth, files, email) generate migrations in rapid succession.
func NextMigrationBaseTime(migrationsPath string) time.Time {
	now := time.Now().UTC()

	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return now
	}

	var latest string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		baseName := strings.TrimSuffix(name, ".go")
		if len(baseName) < 16 || baseName[14] != '_' {
			continue
		}
		ts := baseName[:14]
		valid := true
		for _, c := range ts {
			if c < '0' || c > '9' {
				valid = false
				break
			}
		}
		if valid && ts > latest {
			latest = ts
		}
	}

	if latest == "" {
		return now
	}

	parsed, err := time.Parse("20060102150405", latest)
	if err != nil {
		return now
	}

	// Ensure we start at least 1 second after the latest existing migration
	minBase := parsed.Add(1 * time.Second)
	if now.After(minBase) {
		return now
	}
	return minBase
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
//
// Parameters:
//   - goModRoot: directory containing go.mod
//   - goModModule: raw module path from go.mod (used for require/replace in temp go.mod)
//   - importPrefix: effective import prefix for generated import statements
//     (in a monorepo this includes the subpath, e.g. "com.company/apps/backend")
//   - migrationsPath: absolute path to migrations directory (within shipqRoot)
//   - migrations: list of migration files to execute
func BuildMigrationPlan(goModRoot, goModModule, importPrefix, migrationsPath string, migrations []MigrationFile) ([]byte, error) {
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

	// Determine migrations package path - must be relative to goModRoot for correct Go imports
	relMigrationsPath, err := filepath.Rel(goModRoot, migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative migrations path: %w", err)
	}
	migrationsImportPath := goModModule + "/" + filepath.ToSlash(relMigrationsPath)

	// Generate the runner main.go
	runnerCode := generateMigrationRunner(importPrefix, migrationsImportPath, migrations)
	runnerPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write runner: %w", err)
	}

	// Generate go.mod that requires the user's module
	// The replace directive must point to goModRoot where the actual go.mod lives
	// Uses the raw module path (goModModule) for require/replace directives.
	goModContent := fmt.Sprintf(`module shipq-migrate-runner

go 1.21

require %s v0.0.0

replace %s => %s
`, goModModule, goModModule, goModRoot)

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
// importPrefix is the effective import prefix for generated import statements.
// migrationsImportPath is the full import path to the migrations package.
func generateMigrationRunner(importPrefix, migrationsImportPath string, migrations []MigrationFile) string {
	var buf strings.Builder

	buf.WriteString(`package main

import (
	"encoding/json"
	"fmt"
	"os"

	"`)
	buf.WriteString(importPrefix)
	buf.WriteString(`/shipq/lib/db/portsql/migrate"
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
	return generateMigrationRunner("example.com/test", "example.com/test/migrations", migrations)
}

// LoadMigrationPlan loads the migration plan from schema.json.
// The shipqRoot is the directory containing shipq.ini (where schema.json lives).
// Returns nil if schema.json doesn't exist.
func LoadMigrationPlan(shipqRoot string) (*migrate.MigrationPlan, error) {
	schemaPath := filepath.Join(shipqRoot, "shipq", "db", "migrate", "schema.json")

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
