package seed

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/internal/dbops"
	"github.com/shipq/shipq/project"
)

// SeedFile represents a discovered seed file.
type SeedFile struct {
	Path     string // Full path to the file
	Name     string // Name after "Seed_" prefix (e.g., "dev")
	FuncName string // Full function name (e.g., "Seed_dev")
}

// SeedCmd handles "shipq seed" - discovers and runs seed files.
func SeedCmd() {
	// Step 1: Find project roots
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project", err)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdSeed, roots.ShipqRoot) {
		os.Exit(1)
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		cli.FatalErr("failed to get module info", err)
	}
	// Seed runner builds import paths via filepath.Rel(goModRoot, seedsPath) and
	// uses require/replace directives in a temp go.mod — both need the raw module path.
	modulePath := moduleInfo.ModulePath

	// Step 2: Load config
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

	// Step 3: Discover seed files
	seedsPath := filepath.Join(roots.ShipqRoot, "seeds")
	seeds, err := DiscoverSeeds(seedsPath)
	if err != nil {
		cli.FatalErr("failed to discover seeds", err)
	}

	if len(seeds) == 0 {
		cli.Info("No seed files found in seeds/")
		cli.Info("Run 'shipq auth' to generate auth seed files")
		return
	}

	cli.Infof("Found %d seed file(s)", len(seeds))

	// Step 4: Build and run the seed runner
	cli.Info("Running seeds...")
	if err := RunSeeds(roots.GoModRoot, modulePath, seedsPath, databaseURL, dialect, seeds); err != nil {
		cli.FatalErr("failed to run seeds", err)
	}

	cli.Success("Seeds applied successfully!")
}

// DiscoverSeeds finds all Go files in the seeds/ directory containing Seed_* functions.
// Returns them sorted by name (alphabetical).
func DiscoverSeeds(seedsPath string) ([]SeedFile, error) {
	entries, err := os.ReadDir(seedsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read seeds directory: %w", err)
	}

	var seeds []SeedFile
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

		// Read the file to find Seed_ functions
		data, err := os.ReadFile(filepath.Join(seedsPath, name))
		if err != nil {
			continue
		}

		content := string(data)

		// Find all Seed_ functions in this file
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "func Seed_") {
				continue
			}

			// Extract function name: "func Seed_dev(db *sql.DB) error {"
			funcName := line[len("func "):]
			if idx := strings.Index(funcName, "("); idx > 0 {
				funcName = funcName[:idx]
			}

			seedName := funcName[len("Seed_"):]
			if seedName == "" {
				continue
			}

			seeds = append(seeds, SeedFile{
				Path:     filepath.Join(seedsPath, name),
				Name:     seedName,
				FuncName: funcName,
			})
		}
	}

	// Sort by name for deterministic execution
	sort.Slice(seeds, func(i, j int) bool {
		return seeds[i].Name < seeds[j].Name
	})

	return seeds, nil
}

// RunSeeds creates a temp program that imports the seeds package and runs all seed functions.
func RunSeeds(goModRoot, modulePath, seedsPath, databaseURL, dialect string, seeds []SeedFile) error {
	if len(seeds) == 0 {
		return nil
	}

	// Create temporary directory for the runner
	tmpDir, err := os.MkdirTemp("", "shipq-seed-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine seeds package import path
	relSeedsPath, err := filepath.Rel(goModRoot, seedsPath)
	if err != nil {
		return fmt.Errorf("failed to get relative seeds path: %w", err)
	}
	seedsImportPath := modulePath + "/" + filepath.ToSlash(relSeedsPath)

	// Determine the driver import
	var driverImport string
	switch dialect {
	case dburl.DialectPostgres:
		driverImport = `_ "github.com/jackc/pgx/v5/stdlib"`
	case dburl.DialectMySQL:
		driverImport = `_ "github.com/go-sql-driver/mysql"`
	case dburl.DialectSQLite:
		driverImport = `_ "modernc.org/sqlite"`
	}

	// Convert the database URL to a DSN
	dsn, driverName, err := urlToDSNWithDriver(databaseURL, dialect)
	if err != nil {
		return fmt.Errorf("failed to convert database URL: %w", err)
	}

	// Generate the runner main.go
	runnerCode := generateSeedRunner(seedsImportPath, driverImport, driverName, dsn, seeds)
	runnerPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return fmt.Errorf("failed to write runner: %w", err)
	}

	// Generate go.mod
	goModContent := fmt.Sprintf(`module shipq-seed-runner

go 1.21

require %s v0.0.0

replace %s => %s
`, modulePath, modulePath, goModRoot)

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Run the seed runner
	runCmd := exec.Command("go", "run", ".")
	runCmd.Dir = tmpDir
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("seed runner failed: %w", err)
	}

	return nil
}

// generateSeedRunner generates Go code that runs all seed functions.
func generateSeedRunner(seedsImportPath, driverImport, driverName, dsn string, seeds []SeedFile) string {
	var buf strings.Builder

	buf.WriteString(`package main

import (
	"database/sql"
	"fmt"
	"os"

	`)
	buf.WriteString(driverImport)
	buf.WriteString(`
	seedpkg "`)
	buf.WriteString(seedsImportPath)
	buf.WriteString(`"
)

func main() {
	db, err := sql.Open("`)
	buf.WriteString(driverName)
	buf.WriteString(`", "`)
	buf.WriteString(dsn)
	buf.WriteString(`")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}

`)

	for _, s := range seeds {
		buf.WriteString(fmt.Sprintf(`	fmt.Println("Running seed: %s...")
	if err := seedpkg.%s(db); err != nil {
		fmt.Fprintf(os.Stderr, "seed %s failed: %%v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ %s")

`, s.FuncName, s.FuncName, s.FuncName, s.FuncName))
	}

	buf.WriteString(`	fmt.Println("")
	fmt.Println("All seeds applied successfully!")
}
`)

	return buf.String()
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

// openDatabase opens a database connection.
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
