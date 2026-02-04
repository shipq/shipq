package codegen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/proptest"
)

// Property: DiscoverMigrations always returns migrations sorted by timestamp
func TestProperty_DiscoverMigrations_SortedByTimestamp(t *testing.T) {
	proptest.QuickCheck(t, "migrations sorted by timestamp", func(g *proptest.Generator) bool {
		// Generate random migration files
		numMigrations := g.IntRange(0, 20)
		migrations := generateRandomMigrationFiles(g, numMigrations)

		// Create temp directory with migration files
		tmpDir, err := os.MkdirTemp("", "proptest-migrations-*")
		if err != nil {
			t.Logf("failed to create temp dir: %v", err)
			return false
		}
		defer os.RemoveAll(tmpDir)

		for _, m := range migrations {
			path := filepath.Join(tmpDir, m.filename)
			if err := os.WriteFile(path, []byte(m.content), 0644); err != nil {
				t.Logf("failed to write file: %v", err)
				return false
			}
		}

		// Discover migrations
		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Logf("DiscoverMigrations failed: %v", err)
			return false
		}

		// Verify sorted by timestamp
		for i := 1; i < len(discovered); i++ {
			if discovered[i].Timestamp < discovered[i-1].Timestamp {
				t.Logf("not sorted: %s < %s", discovered[i].Timestamp, discovered[i-1].Timestamp)
				return false
			}
		}

		return true
	})
}

// Property: Generated DB file is valid Go code for all dialects
func TestProperty_GenerateDBFile_ValidGoCode(t *testing.T) {
	dialects := []string{"postgres", "mysql", "sqlite"}

	for _, dialect := range dialects {
		dialect := dialect // capture for closure
		t.Run(dialect, func(t *testing.T) {
			proptest.QuickCheck(t, "valid go code", func(g *proptest.Generator) bool {
				// Generate random but valid database URL
				dbURL := generateRandomDBURL(g, dialect)

				cfg := &codegen.DBPackageConfig{
					ProjectRoot: "/fake/root",
					ModulePath:  "example.com/myapp",
					DatabaseURL: dbURL,
					Dialect:     dialect,
				}

				content, err := codegen.GenerateDBFile(cfg)
				if err != nil {
					t.Logf("GenerateDBFile failed: %v", err)
					return false
				}

				// Verify it's non-empty and contains expected markers
				if len(content) == 0 {
					return false
				}
				if !strings.Contains(string(content), "package db") {
					return false
				}
				if !strings.Contains(string(content), "func DB()") {
					return false
				}

				return true
			})
		})
	}
}

// Property: Migration function names are correctly parsed from filenames
func TestProperty_MigrationFuncName_RoundTrip(t *testing.T) {
	proptest.QuickCheck(t, "funcname roundtrip", func(g *proptest.Generator) bool {
		// Generate valid timestamp (14 digits)
		timestamp := generateTimestamp(g)

		// Generate valid identifier for name
		name := g.IdentifierLower(20)
		if name == "" {
			name = "migration"
		}

		// Expected function name
		expectedFuncName := "Migrate_" + timestamp + "_" + name

		// Create filename
		filename := timestamp + "_" + name + ".go"

		// Parse it back (simulating what DiscoverMigrations does)
		baseName := strings.TrimSuffix(filename, ".go")
		if len(baseName) < 16 {
			return false
		}

		parsedTimestamp := baseName[:14]
		parsedName := baseName[15:]
		parsedFuncName := "Migrate_" + parsedTimestamp + "_" + parsedName

		return parsedFuncName == expectedFuncName
	})
}

// Property: Test database URL is always derived correctly from dev URL
func TestProperty_TestDatabaseURL_Convention(t *testing.T) {
	proptest.QuickCheck(t, "test url convention", func(g *proptest.Generator) bool {
		dialect := proptest.OneOf(g, "postgres", "mysql", "sqlite")
		dbName := g.IdentifierLower(20)
		if dbName == "" {
			dbName = "mydb"
		}

		var devURL string
		switch dialect {
		case "postgres":
			devURL = "postgres://user@localhost:5432/" + dbName
		case "mysql":
			devURL = "mysql://user@localhost:3306/" + dbName
		case "sqlite":
			devURL = "sqlite:///path/to/" + dbName + ".db"
		}

		testURL, err := buildTestDatabaseURL(devURL, dialect)
		if err != nil {
			t.Logf("buildTestDatabaseURL failed: %v", err)
			return false
		}

		// Verify test URL contains _test suffix
		if dialect == "sqlite" {
			return strings.Contains(testURL, dbName+"_test.db")
		}
		return strings.Contains(testURL, dbName+"_test")
	})
}

// Property: Orphaned migration detection finds all non-matching migrations
func TestProperty_OrphanedMigrations_Complete(t *testing.T) {
	proptest.QuickCheck(t, "orphaned detection complete", func(g *proptest.Generator) bool {
		// Generate some migrations that are "in plan"
		numInPlan := g.IntRange(0, 10)
		inPlan := make([]string, numInPlan)
		for i := 0; i < numInPlan; i++ {
			inPlan[i] = generateTimestamp(g) + "_" + g.IdentifierLower(10)
		}

		// Generate some migrations that are "applied but not in plan" (orphaned)
		numOrphaned := g.IntRange(0, 5)
		orphaned := make([]string, numOrphaned)
		for i := 0; i < numOrphaned; i++ {
			orphaned[i] = generateTimestamp(g) + "_orphan_" + g.IdentifierLower(5)
		}

		// Combined "applied" list
		applied := append(append([]string{}, inPlan...), orphaned...)

		// Build plan set
		planSet := make(map[string]bool)
		for _, m := range inPlan {
			planSet[m] = true
		}

		// Find orphaned (simulating checkOrphanedMigrations logic)
		var foundOrphaned []string
		for _, name := range applied {
			if !planSet[name] {
				foundOrphaned = append(foundOrphaned, name)
			}
		}

		// Sort both for comparison
		sort.Strings(orphaned)
		sort.Strings(foundOrphaned)

		if len(orphaned) != len(foundOrphaned) {
			return false
		}
		for i := range orphaned {
			if orphaned[i] != foundOrphaned[i] {
				return false
			}
		}

		return true
	})
}

// Property: DiscoverMigrations discovers exactly the valid migration files
func TestProperty_DiscoverMigrations_Correctness(t *testing.T) {
	proptest.QuickCheck(t, "discovers correct files", func(g *proptest.Generator) bool {
		tmpDir, err := os.MkdirTemp("", "proptest-migrations-*")
		if err != nil {
			t.Logf("failed to create temp dir: %v", err)
			return false
		}
		defer os.RemoveAll(tmpDir)

		// Generate valid migration files
		numValid := g.IntRange(0, 10)
		validFiles := generateRandomMigrationFiles(g, numValid)

		// Generate invalid files that should be ignored
		invalidFiles := []string{
			"readme.md",
			"config.json",
			"helper.go",              // no timestamp
			"20260115_short.go",      // timestamp too short
			"20260115120000users.go", // missing underscore
		}

		// Add some test files that should be ignored
		numTestFiles := g.IntRange(0, 3)
		for i := 0; i < numTestFiles; i++ {
			ts := generateTimestamp(g)
			invalidFiles = append(invalidFiles, ts+"_test_"+g.IdentifierLower(5)+"_test.go")
		}

		// Write all files
		for _, m := range validFiles {
			path := filepath.Join(tmpDir, m.filename)
			if err := os.WriteFile(path, []byte(m.content), 0644); err != nil {
				t.Logf("failed to write valid file: %v", err)
				return false
			}
		}

		for _, f := range invalidFiles {
			path := filepath.Join(tmpDir, f)
			if err := os.WriteFile(path, []byte("invalid content"), 0644); err != nil {
				t.Logf("failed to write invalid file: %v", err)
				return false
			}
		}

		// Discover migrations
		discovered, err := codegen.DiscoverMigrations(tmpDir)
		if err != nil {
			t.Logf("DiscoverMigrations failed: %v", err)
			return false
		}

		// Should discover exactly the valid files
		if len(discovered) != len(validFiles) {
			t.Logf("discovered %d, expected %d", len(discovered), len(validFiles))
			return false
		}

		return true
	})
}

// Property: GenerateMigrateRunner produces valid Go code for any module path
func TestProperty_GenerateMigrateRunner_ValidCode(t *testing.T) {
	proptest.QuickCheck(t, "runner is valid go", func(g *proptest.Generator) bool {
		// Generate random module paths
		parts := g.IntRange(1, 4)
		moduleParts := make([]string, parts)
		for i := 0; i < parts; i++ {
			moduleParts[i] = g.IdentifierLower(10)
		}
		modulePath := strings.Join(moduleParts, "/")
		if modulePath == "" {
			modulePath = "example"
		}

		content, err := codegen.GenerateMigrateRunner(modulePath)
		if err != nil {
			t.Logf("GenerateMigrateRunner failed: %v", err)
			return false
		}

		contentStr := string(content)

		// Verify essential parts
		if !strings.Contains(contentStr, "package migrate") {
			return false
		}
		if !strings.Contains(contentStr, "//go:embed schema.json") {
			return false
		}
		if !strings.Contains(contentStr, "func Plan()") {
			return false
		}
		if !strings.Contains(contentStr, "func Run(") {
			return false
		}
		if !strings.Contains(contentStr, "func RunWithDB(") {
			return false
		}

		// Verify module path is correctly embedded in import
		expectedImport := modulePath + "/shipq/db"
		if !strings.Contains(contentStr, expectedImport) {
			t.Logf("missing expected import: %s", expectedImport)
			return false
		}

		return true
	})
}

// Property: WriteFileIfChanged is idempotent
func TestProperty_WriteFileIfChanged_Idempotent(t *testing.T) {
	proptest.QuickCheck(t, "write is idempotent", func(g *proptest.Generator) bool {
		tmpDir, err := os.MkdirTemp("", "proptest-write-*")
		if err != nil {
			t.Logf("failed to create temp dir: %v", err)
			return false
		}
		defer os.RemoveAll(tmpDir)

		// Generate random content
		content := []byte(g.String(1000))
		filePath := filepath.Join(tmpDir, "test.txt")

		// First write should return true
		written1, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			t.Logf("first write failed: %v", err)
			return false
		}
		if !written1 {
			t.Log("first write should return true")
			return false
		}

		// Second write with same content should return false
		written2, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			t.Logf("second write failed: %v", err)
			return false
		}
		if written2 {
			t.Log("second write should return false")
			return false
		}

		// Verify content is still correct
		readContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Logf("read failed: %v", err)
			return false
		}

		return string(readContent) == string(content)
	})
}

// =============================================================================
// Helper functions
// =============================================================================

// Helper: Generate a valid 14-digit timestamp
func generateTimestamp(g *proptest.Generator) string {
	year := g.IntRange(2020, 2030)
	month := g.IntRange(1, 12)
	day := g.IntRange(1, 28) // Safe for all months
	hour := g.IntRange(0, 23)
	minute := g.IntRange(0, 59)
	second := g.IntRange(0, 59)
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d", year, month, day, hour, minute, second)
}

// Helper: Generate random migration file info
type migrationFileInfo struct {
	filename string
	content  string
}

func generateRandomMigrationFiles(g *proptest.Generator, count int) []migrationFileInfo {
	result := make([]migrationFileInfo, count)
	for i := 0; i < count; i++ {
		timestamp := generateTimestamp(g)
		name := g.IdentifierLower(15)
		if name == "" {
			name = "migration"
		}
		filename := timestamp + "_" + name + ".go"
		funcName := "Migrate_" + timestamp + "_" + name

		content := fmt.Sprintf(`package migrations

import "github.com/shipq/shipq/db/portsql/migrate"

func %s(plan *migrate.MigrationPlan) error {
	return nil
}
`, funcName)

		result[i] = migrationFileInfo{
			filename: filename,
			content:  content,
		}
	}
	return result
}

// Helper: Generate random database URL for dialect
func generateRandomDBURL(g *proptest.Generator, dialect string) string {
	user := g.IdentifierLower(8)
	if user == "" {
		user = "user"
	}
	dbName := g.IdentifierLower(15)
	if dbName == "" {
		dbName = "mydb"
	}
	port := g.IntRange(1024, 65535)

	switch dialect {
	case "postgres":
		return fmt.Sprintf("postgres://%s@localhost:%d/%s", user, port, dbName)
	case "mysql":
		return fmt.Sprintf("mysql://%s@localhost:%d/%s", user, port, dbName)
	case "sqlite":
		return fmt.Sprintf("sqlite:///tmp/%s.db", dbName)
	default:
		return "postgres://user@localhost:5432/db"
	}
}

// buildTestDatabaseURL creates the test database URL from the dev URL.
// This mirrors the logic in migrate_up.go
func buildTestDatabaseURL(devURL, dialect string) (string, error) {
	// Extract database name
	var dbName string
	if dialect == "sqlite" {
		// For sqlite, extract from path
		if strings.HasPrefix(devURL, "sqlite://") {
			path := devURL[9:]
			dbName = filepath.Base(path)
			if strings.HasSuffix(dbName, ".db") {
				dbName = strings.TrimSuffix(dbName, ".db")
			}
		}
	} else {
		// For postgres/mysql, extract from URL path
		parts := strings.Split(devURL, "/")
		if len(parts) > 0 {
			dbName = parts[len(parts)-1]
		}
	}

	if dbName == "" {
		return "", fmt.Errorf("could not parse database name from URL")
	}

	testDBName := dbName + "_test"

	// Build new URL
	if dialect == "sqlite" {
		if strings.HasPrefix(devURL, "sqlite://") {
			path := devURL[9:]
			dir := filepath.Dir(path)
			return "sqlite://" + filepath.Join(dir, testDBName+".db"), nil
		}
	}

	// For postgres/mysql, replace the last path component
	lastSlash := strings.LastIndex(devURL, "/")
	if lastSlash == -1 {
		return "", fmt.Errorf("invalid URL format")
	}
	return devURL[:lastSlash+1] + testDBName, nil
}
