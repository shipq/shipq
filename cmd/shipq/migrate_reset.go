package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

// migrateResetCmd implements the "shipq migrate reset" command.
// It drops and recreates dev/test databases, then re-runs all migrations.
func migrateResetCmd() {
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

	// Step 3: Safety check - must be localhost
	if !dburl.IsLocalhost(databaseURL) {
		cli.Fatal("migrate reset only works on localhost databases for safety")
	}

	// Step 4: Generate/update shipq/db package (in shipq root)
	cli.Info("Generating shipq/db package...")
	if err := dbpkg.EnsureDBPackage(roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to generate db package", err)
	}
	cli.Success("Generated shipq/db/db.go")

	// Step 5: Get database names
	projectName := project.GetProjectName(roots.ShipqRoot)
	devDBName := dburl.ParseDatabaseName(databaseURL)
	if devDBName == "" {
		devDBName = projectName
	}
	testDBName := buildTestDBName(devDBName, dialect)

	// Step 6: Drop databases
	cli.Info("Dropping databases...")
	if err := dropDatabases(databaseURL, dialect, devDBName, testDBName, roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to drop databases", err)
	}
	cli.Successf("Dropped databases: %s, %s", devDBName, testDBName)

	// Step 7: Recreate databases
	cli.Info("Creating databases...")
	if err := createDatabases(databaseURL, dialect, devDBName, testDBName, roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to create databases", err)
	}
	cli.Successf("Created databases: %s, %s", devDBName, testDBName)

	// Step 8: Discover and load migrations (from shipq root)
	migrationsPath := getMigrationsPath(ini, roots.ShipqRoot)
	migrations, err := codegenMigrate.DiscoverMigrations(migrationsPath)
	if err != nil {
		cli.FatalErr("failed to discover migrations", err)
	}

	if len(migrations) == 0 {
		cli.Info("No migrations found in " + migrationsPath)
		cli.Info("Create a migration with: shipq migrate new <name>")
		cli.Success("migrate reset complete (no migrations to run)")
		return
	}

	cli.Infof("Found %d migration(s)", len(migrations))

	// Step 9: Build migration plan (use GoModRoot for replace directive)
	cli.Info("Building migration plan...")
	planJSON, err := codegenMigrate.BuildMigrationPlan(roots.GoModRoot, modulePath, migrationsPath, migrations)
	if err != nil {
		cli.FatalErr("failed to build migration plan", err)
	}

	// Step 10: Write schema.json (in shipq root)
	migratePkgPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate")
	if err := codegen.EnsureDir(migratePkgPath); err != nil {
		cli.FatalErr("failed to create migrate directory", err)
	}

	schemaJSONPath := filepath.Join(migratePkgPath, "schema.json")
	if _, err := codegen.WriteFileIfChanged(schemaJSONPath, planJSON); err != nil {
		cli.FatalErr("failed to write schema.json", err)
	}
	cli.Success("Generated shipq/db/migrate/schema.json")

	// Step 11: Generate runner.go
	runnerContent, err := codegenMigrate.GenerateMigrateRunner(modulePath)
	if err != nil {
		cli.FatalErr("failed to generate runner", err)
	}

	runnerPath := filepath.Join(migratePkgPath, "runner.go")
	if _, err := codegen.WriteFileIfChanged(runnerPath, runnerContent); err != nil {
		cli.FatalErr("failed to write runner.go", err)
	}
	cli.Success("Generated shipq/db/migrate/runner.go")

	// Step 12: Run migrations against dev database
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
	cli.Success("Dev database migrated")

	// Step 13: Run migrations against test database
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

	// Step 14: Generate query runner (in shipq root)
	cli.Info("Generating shipq/queries package...")
	if err := generateQueryRunnerForReset(roots.ShipqRoot, modulePath, plan, dialect); err != nil {
		cli.FatalErr("failed to generate query runner", err)
	}
	cli.Successf("Generated shipq/queries/%s/runner.go", dialect)

	cli.Success("migrate reset complete")
}

// generateQueryRunnerForReset generates the shipq/queries package with the unified query runner.
func generateQueryRunnerForReset(shipqRoot, modulePath string, plan *migrate.MigrationPlan, dialect string) error {
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
		UserQueries: nil, // No user queries from migrate reset - use db compile for that
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

// buildTestDBName returns the test database name for a given dev database name.
func buildTestDBName(devDBName, dialect string) string {
	if dialect == dburl.DialectSQLite {
		// For SQLite file paths, handle the .db extension
		if len(devDBName) > 3 && devDBName[len(devDBName)-3:] == ".db" {
			return devDBName[:len(devDBName)-3] + "_test.db"
		}
	}
	return devDBName + "_test"
}

// dropDatabases drops both dev and test databases.
func dropDatabases(databaseURL, dialect, devDBName, testDBName, projectRoot string) error {
	ctx := context.Background()

	switch dialect {
	case dburl.DialectPostgres:
		// Open maintenance connection
		db, err := dbops.OpenMaintenanceDB(databaseURL, dialect)
		if err != nil {
			return err
		}
		defer db.Close()

		// Drop dev database
		if err := dbops.DropPostgresDB(ctx, db, devDBName); err != nil {
			return err
		}

		// Drop test database
		if err := dbops.DropPostgresDB(ctx, db, testDBName); err != nil {
			return err
		}

	case dburl.DialectMySQL:
		// Open maintenance connection
		db, err := dbops.OpenMaintenanceDB(databaseURL, dialect)
		if err != nil {
			return err
		}
		defer db.Close()

		// Drop dev database
		if err := dbops.DropMySQLDB(ctx, db, devDBName); err != nil {
			return err
		}

		// Drop test database
		if err := dbops.DropMySQLDB(ctx, db, testDBName); err != nil {
			return err
		}

	case dburl.DialectSQLite:
		// For SQLite, delete the database files
		dataDir := filepath.Join(projectRoot, ".shipq", "data")

		devPath := filepath.Join(dataDir, devDBName)
		if !filepath.IsAbs(devDBName) && devDBName[0] != '.' {
			devPath = filepath.Join(dataDir, devDBName)
		} else {
			devPath = dbops.SQLiteURLToPath(databaseURL)
		}

		testPath := filepath.Join(dataDir, testDBName)
		if filepath.IsAbs(devPath) || devPath[0] == '.' {
			// If dev path is absolute or relative, derive test path from it
			if len(devPath) > 3 && devPath[len(devPath)-3:] == ".db" {
				testPath = devPath[:len(devPath)-3] + "_test.db"
			} else {
				testPath = devPath + "_test"
			}
		}

		if err := dbops.DropSQLiteDB(devPath); err != nil {
			return err
		}

		if err := dbops.DropSQLiteDB(testPath); err != nil {
			return err
		}
	}

	return nil
}

// createDatabases creates both dev and test databases.
func createDatabases(databaseURL, dialect, devDBName, testDBName, projectRoot string) error {
	ctx := context.Background()

	switch dialect {
	case dburl.DialectPostgres:
		// Open maintenance connection
		db, err := dbops.OpenMaintenanceDB(databaseURL, dialect)
		if err != nil {
			return err
		}
		defer db.Close()

		// Create dev database
		if err := dbops.CreatePostgresDB(ctx, db, devDBName); err != nil {
			return err
		}

		// Create test database
		if err := dbops.CreatePostgresDB(ctx, db, testDBName); err != nil {
			return err
		}

	case dburl.DialectMySQL:
		// Open maintenance connection
		db, err := dbops.OpenMaintenanceDB(databaseURL, dialect)
		if err != nil {
			return err
		}
		defer db.Close()

		// Create dev database
		if err := dbops.CreateMySQLDB(ctx, db, devDBName); err != nil {
			return err
		}

		// Create test database
		if err := dbops.CreateMySQLDB(ctx, db, testDBName); err != nil {
			return err
		}

	case dburl.DialectSQLite:
		// For SQLite, create the database files
		dataDir := filepath.Join(projectRoot, ".shipq", "data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return err
		}

		devPath := filepath.Join(dataDir, devDBName)
		if !filepath.IsAbs(devDBName) && devDBName[0] != '.' {
			devPath = filepath.Join(dataDir, devDBName)
		} else {
			devPath = dbops.SQLiteURLToPath(databaseURL)
		}

		testPath := filepath.Join(dataDir, testDBName)
		if filepath.IsAbs(devPath) || devPath[0] == '.' {
			// If dev path is absolute or relative, derive test path from it
			if len(devPath) > 3 && devPath[len(devPath)-3:] == ".db" {
				testPath = devPath[:len(devPath)-3] + "_test.db"
			} else {
				testPath = devPath + "_test"
			}
		}

		if err := dbops.CreateSQLiteDB(devPath); err != nil {
			return err
		}

		if err := dbops.CreateSQLiteDB(testPath); err != nil {
			return err
		}
	}

	return nil
}
