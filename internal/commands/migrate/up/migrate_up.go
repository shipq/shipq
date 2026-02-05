package up

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/dbpkg"
	codegenMigrate "github.com/shipq/shipq/codegen/migrate"
	"github.com/shipq/shipq/codegen/queryrunner"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/dbops"
	"github.com/shipq/shipq/project"
)

// MigrateUpCmd implements the "shipq migrate up" command.
func MigrateUpCmd() {
	// Step 1: Find and validate project roots (supports monorepo setup)
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project", err)
	}

	// Step 2: Load configuration
	modulePath, err := codegen.GetModulePath(roots.GoModRoot)
	if err != nil {
		cli.FatalErr("failed to get module path", err)
	}

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	databaseURL := ini.Get("db", "database_url")
	if databaseURL == "" {
		cli.Fatal("db.database_url not configured in shipq.ini\n  Run 'shipq db setup' first")
	}

	dialect, err := dburl.InferDialectFromDBUrl(databaseURL)
	if err != nil {
		cli.FatalErr("failed to determine database dialect", err)
	}

	// Step 3: Generate/update shipq/db package (in shipq root)
	cli.Info("Generating shipq/db package...")
	if err := dbpkg.EnsureDBPackage(roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to generate db package", err)
	}
	cli.Success("Generated shipq/db/db.go")

	// Step 4: Discover and load migrations (from shipq root)
	migrationsPath := getMigrationsPath(ini, roots.ShipqRoot)
	migrations, err := codegenMigrate.DiscoverMigrations(migrationsPath)
	if err != nil {
		cli.FatalErr("failed to discover migrations", err)
	}

	if len(migrations) == 0 {
		cli.Info("No migrations found in " + migrationsPath)
		cli.Info("Create a migration with: shipq migrate new <name>")
		return
	}

	cli.Infof("Found %d migration(s)", len(migrations))

	// Step 5: Build migration plan by executing migration functions
	// Use GoModRoot for the replace directive in temp go.mod
	cli.Info("Building migration plan...")
	planJSON, err := codegenMigrate.BuildMigrationPlan(roots.GoModRoot, modulePath, migrationsPath, migrations)
	if err != nil {
		cli.FatalErr("failed to build migration plan", err)
	}

	// Step 6: Write schema.json (in shipq root)
	migratePkgPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate")
	if err := codegen.EnsureDir(migratePkgPath); err != nil {
		cli.FatalErr("failed to create migrate directory", err)
	}

	schemaJSONPath := filepath.Join(migratePkgPath, "schema.json")
	if _, err := codegen.WriteFileIfChanged(schemaJSONPath, planJSON); err != nil {
		cli.FatalErr("failed to write schema.json", err)
	}
	cli.Success("Generated shipq/db/migrate/schema.json")

	// Step 7: Generate runner.go
	runnerContent, err := codegenMigrate.GenerateMigrateRunner(modulePath)
	if err != nil {
		cli.FatalErr("failed to generate runner", err)
	}

	runnerPath := filepath.Join(migratePkgPath, "runner.go")
	if _, err := codegen.WriteFileIfChanged(runnerPath, runnerContent); err != nil {
		cli.FatalErr("failed to write runner.go", err)
	}
	cli.Success("Generated shipq/db/migrate/runner.go")

	// Step 8: Run migrations against dev database
	plan, err := migrate.PlanFromJSON(planJSON)
	if err != nil {
		cli.FatalErr("failed to parse migration plan", err)
	}

	cli.Info("Running migrations against dev database...")
	devDB, err := openDatabase(databaseURL, dialect)
	if err != nil {
		cli.FatalErr("failed to connect to dev database", err)
	}
	defer devDB.Close()

	if err := migrate.Run(context.Background(), devDB, plan, dialect); err != nil {
		cli.FatalErr("failed to migrate dev database", err)
	}

	// Check for orphaned migrations in dev database
	checkOrphanedMigrations(context.Background(), devDB, plan)

	cli.Success("Dev database migrated")

	// Step 9: Run migrations against test database
	testURL, err := buildTestDatabaseURL(databaseURL, dialect)
	if err != nil {
		cli.FatalErr("failed to build test database URL", err)
	}

	cli.Info("Running migrations against test database...")
	testDB, err := openDatabase(testURL, dialect)
	if err != nil {
		cli.FatalErr("failed to connect to test database", err)
	}
	defer testDB.Close()

	if err := migrate.Run(context.Background(), testDB, plan, dialect); err != nil {
		cli.FatalErr("failed to migrate test database", err)
	}
	cli.Success("Test database migrated")

	// Step 10: Generate query runner (in shipq root)
	cli.Info("Generating shipq/queries package...")
	if err := generateQueryRunner(roots.ShipqRoot, modulePath, plan, dialect); err != nil {
		cli.FatalErr("failed to generate query runner", err)
	}
	cli.Successf("Generated shipq/queries/%s/runner.go", dialect)

	cli.Success("migrate up complete")
}

// generateQueryRunner generates the shipq/queries package with the unified query runner.
func generateQueryRunner(shipqRoot, modulePath string, plan *migrate.MigrationPlan, dialect string) error {
	// Create output directories (in shipq root)
	queriesDir := filepath.Join(shipqRoot, "shipq", "queries")
	if err := codegen.EnsureDir(queriesDir); err != nil {
		return fmt.Errorf("failed to create queries directory: %w", err)
	}

	dialectDir := filepath.Join(queriesDir, dialect)
	if err := codegen.EnsureDir(dialectDir); err != nil {
		return fmt.Errorf("failed to create dialect directory: %w", err)
	}

	// Build config for the runner generator
	runnerCfg := queryrunner.UnifiedRunnerConfig{
		ModulePath:  modulePath,
		Dialect:     dialect,
		UserQueries: nil, // No user queries from migrate up - use db compile for that
		Schema:      plan,
	}

	// Generate and write types.go
	typesCode, err := queryrunner.GenerateSharedTypes(runnerCfg)
	if err != nil {
		return fmt.Errorf("failed to generate types.go: %w", err)
	}

	typesPath := filepath.Join(queriesDir, "types.go")
	if _, err := codegen.WriteFileIfChanged(typesPath, typesCode); err != nil {
		return fmt.Errorf("failed to write types.go: %w", err)
	}

	// Generate and write runner.go
	runnerCode, err := queryrunner.GenerateUnifiedRunner(runnerCfg)
	if err != nil {
		return fmt.Errorf("failed to generate runner.go: %w", err)
	}

	runnerPath := filepath.Join(dialectDir, "runner.go")
	if _, err := codegen.WriteFileIfChanged(runnerPath, runnerCode); err != nil {
		return fmt.Errorf("failed to write runner.go: %w", err)
	}

	return nil
}

// getMigrationsPath returns the migrations directory path.
func getMigrationsPath(ini *inifile.File, projectRoot string) string {
	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	return filepath.Join(projectRoot, migrationsDir)
}

// buildTestDatabaseURL creates the test database URL from the dev URL.
// Convention: test database is named {dev_db}_test
// For SQLite: foo.db -> foo_test.db
func buildTestDatabaseURL(devURL, dialect string) (string, error) {
	devDBName := dburl.ParseDatabaseName(devURL)
	if devDBName == "" {
		return "", fmt.Errorf("could not parse database name from URL")
	}

	var testDBName string
	if dialect == dburl.DialectSQLite {
		// For SQLite, insert _test before the .db extension
		// path/to/foo.db -> path/to/foo_test.db
		if strings.HasSuffix(devDBName, ".db") {
			testDBName = strings.TrimSuffix(devDBName, ".db") + "_test.db"
		} else {
			testDBName = devDBName + "_test"
		}
	} else {
		testDBName = devDBName + "_test"
	}

	return dburl.WithDatabaseName(devURL, testDBName)
}

// openDatabase opens a database connection using the appropriate driver.
func openDatabase(dbURL, dialect string) (*sql.DB, error) {
	dsn, driverName, err := urlToDSNWithDriver(dbURL, dialect)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// urlToDSNWithDriver converts a URL to a driver-specific DSN and returns the driver name.
func urlToDSNWithDriver(dbURL, dialect string) (dsn string, driver string, err error) {
	switch dialect {
	case dburl.DialectPostgres:
		return dbURL, "pgx", nil
	case dburl.DialectMySQL:
		dsn, err = dbops.MySQLURLToDSN(dbURL)
		return dsn, "mysql", err
	case dburl.DialectSQLite:
		return dbops.SQLiteURLToPath(dbURL), "sqlite", nil
	default:
		return "", "", fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

// checkOrphanedMigrations compares applied migrations against the plan and warns
// about any that no longer exist in the codebase.
func checkOrphanedMigrations(ctx context.Context, db *sql.DB, plan *migrate.MigrationPlan) {
	applied, err := migrate.GetAppliedMigrations(ctx, db)
	if err != nil {
		return // Silently skip if we can't read applied migrations
	}

	// Build set of migration names in the current plan
	planMigrations := make(map[string]bool)
	for _, m := range plan.Migrations {
		planMigrations[m.Name] = true
	}

	// Find orphaned migrations
	var orphaned []string
	for _, appliedName := range applied {
		if !planMigrations[appliedName] {
			orphaned = append(orphaned, appliedName)
		}
	}

	if len(orphaned) > 0 {
		cli.Warn("The following migrations were applied but no longer exist in your migrations directory:")
		for _, name := range orphaned {
			cli.Warnf("  - %s", name)
		}
		cli.Warn("This may indicate deleted migration files. The database schema may differ from your code.")
		cli.Warn("Consider creating a new migration to reconcile the schema, or manually remove")
		cli.Warn("these entries from the _portsql_migrations table if they are no longer needed.")
	}
}
