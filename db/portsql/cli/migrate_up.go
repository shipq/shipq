package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/shipq/shipq/db/portsql/codegen"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// MigrateUp runs all pending migrations and generates output files.
func MigrateUp(ctx context.Context, config *Config) error {
	// Validate config
	if config.Database.URL == "" {
		return fmt.Errorf("database URL not configured (set DATABASE_URL or add to portsql.ini)")
	}

	// Parse dialect
	dialect := ParseDialect(config.Database.URL)
	if dialect == "" {
		return fmt.Errorf("unsupported database URL scheme: %s", config.Database.URL)
	}

	// Open database connection
	db, err := openDatabase(config.Database.URL, dialect)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Ensure tracking table exists
	if err := migrate.EnsureTrackingTable(ctx, db, dialect); err != nil {
		return fmt.Errorf("failed to create tracking table: %w", err)
	}

	// Get applied migrations
	applied, err := migrate.GetAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}
	appliedSet := make(map[string]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Scan migrations directory
	migrationFiles, err := scanMigrationFiles(config.Paths.Migrations)
	if err != nil {
		return fmt.Errorf("failed to scan migrations: %w", err)
	}

	// Filter to unapplied migrations
	var pendingFiles []migrationFile
	for _, mf := range migrationFiles {
		if !appliedSet[mf.Name] {
			pendingFiles = append(pendingFiles, mf)
		}
	}

	if len(pendingFiles) == 0 {
		fmt.Println("No pending migrations.")
		return nil
	}

	// Sort by version (timestamp)
	sort.Slice(pendingFiles, func(i, j int) bool {
		return pendingFiles[i].Version < pendingFiles[j].Version
	})

	// Execute each pending migration
	accumulatedPlan := migrate.NewPlan()

	// First, load existing schema if it exists
	schemaPath := filepath.Join(config.Paths.Migrations, "schema.json")
	if data, err := os.ReadFile(schemaPath); err == nil {
		if existingPlan, err := migrate.PlanFromJSON(data); err == nil {
			accumulatedPlan = existingPlan
		}
	}

	for _, mf := range pendingFiles {
		fmt.Printf("Running migration: %s\n", mf.Name)

		// Generate and run temp program to get migration plan
		// Pass accumulated plan so UpdateTable/DropTable can see existing tables
		plan, err := executeMigrationFile(ctx, config.Paths.Migrations, mf, accumulatedPlan)
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", mf.Name, err)
		}

		// Start a transaction for this migration
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start transaction for %s: %w", mf.Name, err)
		}

		// Execute SQL for this migration within the transaction
		for _, m := range plan.Migrations {
			var sqlStmt string
			switch dialect {
			case "postgres":
				sqlStmt = m.Instructions.Postgres
			case "mysql":
				sqlStmt = m.Instructions.MySQL
			case "sqlite":
				sqlStmt = m.Instructions.Sqlite
			}

			if sqlStmt == "" {
				continue
			}

			if _, err := tx.ExecContext(ctx, sqlStmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute SQL for %s: %w\nSQL: %s", m.Name, err, sqlStmt)
			}
		}

		// Record migration within the same transaction
		if err := migrate.RecordMigrationTx(ctx, tx, dialect, mf.Version, mf.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", mf.Name, err)
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", mf.Name, err)
		}

		// Merge into accumulated plan with timestamped names
		for name, table := range plan.Schema.Tables {
			accumulatedPlan.Schema.Tables[name] = table
		}
		// Prefix migration names with timestamp from the file
		for i := range plan.Migrations {
			plan.Migrations[i].Name = fmt.Sprintf("%s_%s", mf.Version, plan.Migrations[i].Name)
		}
		accumulatedPlan.Migrations = append(accumulatedPlan.Migrations, plan.Migrations...)

		fmt.Printf("  ✓ Applied %s\n", mf.Name)
	}

	// Write schema.json
	schemaJSON, err := accumulatedPlan.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize schema: %w", err)
	}
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write schema.json: %w", err)
	}
	fmt.Printf("Updated: %s\n", schemaPath)

	// Generate runner.go
	runnerCode, err := codegen.GenerateRunner("migrations")
	if err != nil {
		return fmt.Errorf("failed to generate runner.go: %w", err)
	}
	runnerPath := filepath.Join(config.Paths.Migrations, "runner.go")
	if err := os.WriteFile(runnerPath, runnerCode, 0644); err != nil {
		return fmt.Errorf("failed to write runner.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", runnerPath)

	// Generate schematypes
	if err := os.MkdirAll(config.Paths.Schematypes, 0755); err != nil {
		return fmt.Errorf("failed to create schematypes directory: %w", err)
	}

	schemaCode, err := codegen.GenerateSchemaPackage(accumulatedPlan, "github.com/shipq/shipq/db/portsql/query")
	if err != nil {
		return fmt.Errorf("failed to generate schematypes: %w", err)
	}
	schemaTypesPath := filepath.Join(config.Paths.Schematypes, "tables.go")
	if err := os.WriteFile(schemaTypesPath, schemaCode, 0644); err != nil {
		return fmt.Errorf("failed to write tables.go: %w", err)
	}
	fmt.Printf("Generated: %s\n", schemaTypesPath)

	fmt.Printf("\nSuccessfully applied %d migration(s).\n", len(pendingFiles))
	return nil
}

// migrationFile represents a discovered migration file.
type migrationFile struct {
	Path    string // Full path to the file
	Name    string // Filename without extension (e.g., "20260111153000_create_users")
	Version string // Timestamp portion (e.g., "20260111153000")
}

// scanMigrationFiles finds all migration .go files in a directory.
func scanMigrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No migrations directory yet
		}
		return nil, err
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip non-Go files and generated files
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if name == "runner.go" {
			continue
		}

		// Parse the filename
		version, _, ok := ParseMigrationFilename(name)
		if !ok {
			continue
		}

		files = append(files, migrationFile{
			Path:    filepath.Join(dir, name),
			Name:    strings.TrimSuffix(name, ".go"),
			Version: version,
		})
	}

	return files, nil
}

// executeMigrationFile generates a temp program to execute a migration and capture its plan.
// The existingPlan parameter provides the current schema so UpdateTable/DropTable can work.
func executeMigrationFile(ctx context.Context, migrationsDir string, mf migrationFile, existingPlan *migrate.MigrationPlan) (*migrate.MigrationPlan, error) {
	// Get the module path for imports
	modulePath, err := getModulePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get module path: %w", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "portsql-migrate-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write existing schema to temp file so the migration can use it
	schemaPath := filepath.Join(tmpDir, "schema.json")
	schemaJSON, err := existingPlan.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize schema: %w", err)
	}
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp schema: %w", err)
	}

	// Get absolute path to migrations directory
	absMigrationsDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Calculate the import path for migrations
	// This assumes migrations are in a subdirectory of the module
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(cwd, absMigrationsDir)
	if err != nil {
		return nil, err
	}

	// Convert to import path
	migrationsImport := modulePath + "/" + filepath.ToSlash(relPath)

	// Generate function name from migration file
	funcName := "Migrate_" + mf.Name

	// Generate the temp main.go that loads the existing schema
	mainGo := fmt.Sprintf(`package main

import (
	"encoding/json"
	"os"

	migrations %q
	"github.com/shipq/shipq/db/portsql/migrate"
)

func main() {
	// Load existing schema from environment variable
	schemaPath := os.Getenv("PORTSQL_SCHEMA_PATH")
	var plan *migrate.MigrationPlan
	if schemaPath != "" {
		data, err := os.ReadFile(schemaPath)
		if err != nil {
			os.Stderr.WriteString("failed to read schema: " + err.Error() + "\n")
			os.Exit(1)
		}
		plan, err = migrate.PlanFromJSON(data)
		if err != nil {
			os.Stderr.WriteString("failed to parse schema: " + err.Error() + "\n")
			os.Exit(1)
		}
		// Clear migrations - we only want the schema, not old migrations
		plan.Migrations = nil
	} else {
		plan = migrate.NewPlan()
	}

	if err := migrations.%s(plan); err != nil {
		os.Stderr.WriteString("migration error: " + err.Error() + "\n")
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(plan)
}
`, migrationsImport, funcName)

	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainGo), 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp main.go: %w", err)
	}

	// Run go run with schema path in environment
	cmd := exec.CommandContext(ctx, "go", "run", mainPath)
	cmd.Dir = cwd // Run from original directory for proper module resolution
	cmd.Env = append(os.Environ(), "PORTSQL_SCHEMA_PATH="+schemaPath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("migration failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run migration: %w", err)
	}

	// Parse the JSON output
	var plan migrate.MigrationPlan
	if err := json.Unmarshal(output, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse migration output: %w", err)
	}

	return &plan, nil
}

// getModulePath reads the module path from go.mod.
func getModulePath() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// openDatabase opens a database connection based on the URL and dialect.
func openDatabase(databaseURL string, dialect string) (*sql.DB, error) {
	var driverName, dsn string

	switch dialect {
	case "postgres":
		driverName = "pgx"
		dsn = databaseURL
	case "mysql":
		// MySQL driver expects DSN without the mysql:// prefix
		dsn = strings.TrimPrefix(databaseURL, "mysql://")
		// Convert URL format to DSN format: user:pass@tcp(host:port)/dbname
		dsn = convertMySQLURL(dsn)
		driverName = "mysql"
	case "sqlite":
		driverName = "sqlite"
		dsn = ParseSQLitePath(databaseURL)
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// convertMySQLURL converts a URL-style connection string to MySQL DSN format.
// From: user:pass@host:port/dbname
// To:   user:pass@tcp(host:port)/dbname
func convertMySQLURL(urlStr string) string {
	// Handle user:pass@host:port/dbname format
	atIdx := strings.LastIndex(urlStr, "@")
	if atIdx == -1 {
		return urlStr
	}

	userPass := urlStr[:atIdx]
	hostDbname := urlStr[atIdx+1:]

	slashIdx := strings.Index(hostDbname, "/")
	if slashIdx == -1 {
		return urlStr
	}

	hostPort := hostDbname[:slashIdx]
	dbname := hostDbname[slashIdx:]

	return fmt.Sprintf("%s@tcp(%s)%s", userPass, hostPort, dbname)
}
