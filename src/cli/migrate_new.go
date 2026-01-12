package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// MigrateNew creates a new timestamped migration file.
func MigrateNew(config *Config, name string) (string, error) {
	// Validate name - only allow alphanumeric and underscores
	if !isValidMigrationName(name) {
		return "", fmt.Errorf("invalid migration name %q: use only lowercase letters, numbers, and underscores", name)
	}

	// Generate timestamp
	timestamp := time.Now().Format("20060102150405")

	// Create filename
	filename := fmt.Sprintf("%s_%s.go", timestamp, name)
	migrationsDir := config.Paths.Migrations

	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Full path
	fullPath := filepath.Join(migrationsDir, filename)

	// Check if file already exists
	if _, err := os.Stat(fullPath); err == nil {
		return "", fmt.Errorf("migration file already exists: %s", fullPath)
	}

	// Generate the migration function name
	funcName := fmt.Sprintf("Migrate_%s_%s", timestamp, name)

	// Generate the stub content
	content := generateMigrationStub(funcName)

	// Write the file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write migration file: %w", err)
	}

	return fullPath, nil
}

// isValidMigrationName checks if a migration name is valid.
// Names should only contain lowercase letters, numbers, and underscores.
func isValidMigrationName(name string) bool {
	if name == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9_]*$`, name)
	return matched
}

// generateMigrationStub creates the content for a new migration file.
func generateMigrationStub(funcName string) string {
	return fmt.Sprintf(`package migrations

import "github.com/portsql/portsql/src/migrate"

func %s(plan *migrate.MigrationPlan) error {
	// TODO: Define your migration
	// Example:
	// plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
	//     tb.Bigint("id").PrimaryKey()
	//     tb.String("name")
	//     tb.String("email").Unique()
	//     return nil
	// })
	return nil
}
`, funcName)
}

// ParseMigrationFilename extracts the timestamp and name from a migration filename.
// Returns (timestamp, name, ok).
func ParseMigrationFilename(filename string) (string, string, bool) {
	// Remove .go extension
	if filepath.Ext(filename) != ".go" {
		return "", "", false
	}
	base := filename[:len(filename)-3]

	// Split on first underscore after timestamp
	if len(base) < 15 { // minimum: 14 digit timestamp + underscore + 1 char
		return "", "", false
	}

	timestamp := base[:14]
	if !isValidTimestamp(timestamp) {
		return "", "", false
	}

	if base[14] != '_' {
		return "", "", false
	}

	name := base[15:]
	if name == "" {
		return "", "", false
	}

	return timestamp, name, true
}

// isValidTimestamp checks if a string is a valid YYYYMMDDHHMMSS timestamp.
func isValidTimestamp(s string) bool {
	if len(s) != 14 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
