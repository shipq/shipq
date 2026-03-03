package init

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

// InitCmd implements the "shipq init" command.
// It initializes a new shipq project by creating go.mod (if needed) and shipq.ini.
// In a monorepo setup, if a go.mod exists in a parent directory, it will be used
// instead of creating a new one.
//
// Flags:
//
//	--postgres   Use PostgreSQL as the database dialect
//	--mysql      Use MySQL as the database dialect
//	--sqlite     Use SQLite as the database dialect (default)
func InitCmd() {
	cwd, err := os.Getwd()
	if err != nil {
		cli.FatalErr("failed to get current directory", err)
	}

	projectName := project.GetProjectName(cwd)
	createdGoMod := false
	createdShipqIni := false
	updatedGitignore := false
	existingGoModRoot := ""

	createdHealth := false

	// Parse dialect flag from os.Args
	dialect := parseDialectFlag()

	// Check if a go.mod exists anywhere up the directory tree (monorepo support)
	goModRoot, err := project.FindGoModRootFrom(cwd)
	if err == project.ErrNotInProject {
		// No go.mod found anywhere - create one in current directory
		if err := createGoMod(cwd, projectName); err != nil {
			cli.FatalErr("failed to create go.mod", err)
		}
		createdGoMod = true
	} else if err != nil {
		cli.FatalErr("failed to check for go.mod", err)
	} else {
		// Found existing go.mod - don't create a new one
		existingGoModRoot = goModRoot
	}

	// Create shipq.ini if it doesn't exist in current directory
	if !project.HasShipqIni(cwd) {
		if err := createShipqIni(cwd, projectName, dialect); err != nil {
			cli.FatalErr("failed to create shipq.ini", err)
		}
		createdShipqIni = true
	}

	// Create or update .gitignore to include .shipq/
	updated, err := ensureGitignore(cwd)
	if err != nil {
		cli.FatalErr("failed to update .gitignore", err)
	}
	updatedGitignore = updated

	// Scaffold api/health/ endpoint (idempotent — skips if register.go already exists)
	goModDir := cwd
	if existingGoModRoot != "" {
		goModDir = existingGoModRoot
	}
	moduleInfo, err := codegen.GetModuleInfo(goModDir, cwd)
	if err != nil {
		cli.FatalErr("failed to read module info", err)
	}
	modulePath := moduleInfo.FullImportPath("")
	created, err := createHealthEndpoint(cwd, modulePath)
	if err != nil {
		cli.FatalErr("failed to create health endpoint", err)
	}
	createdHealth = created

	// Print results
	if createdGoMod && createdShipqIni {
		cli.Success("Initialized new shipq project")
		cli.Infof("  Created go.mod (module: com.%s)", projectName)
		cli.Infof("  Created shipq.ini (dialect: %s)", dialect)
		if createdHealth {
			cli.Info("  Created api/health/ (healthcheck endpoint)")
		}
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if createdGoMod {
		cli.Success("Created go.mod")
		cli.Infof("  Module: com.%s", projectName)
		if createdHealth {
			cli.Info("  Created api/health/ (healthcheck endpoint)")
		}
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if createdShipqIni {
		cli.Success("Created shipq.ini")
		cli.Infof("  Dialect: %s", dialect)
		if existingGoModRoot != "" && existingGoModRoot != cwd {
			cli.Infof("  Using existing go.mod from %s", existingGoModRoot)
		}
		if createdHealth {
			cli.Info("  Created api/health/ (healthcheck endpoint)")
		}
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if createdHealth {
		cli.Success("Created api/health/ (healthcheck endpoint)")
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if updatedGitignore {
		cli.Success("Updated .gitignore")
	} else {
		cli.Info("Project already initialized (go.mod and shipq.ini exist)")
	}
}

// parseDialectFlag inspects os.Args for --postgres, --mysql, or --sqlite.
// Defaults to "sqlite" when no flag is provided.
func parseDialectFlag() string {
	dialect := "sqlite"
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--postgres":
			dialect = "postgres"
		case "--mysql":
			dialect = "mysql"
		case "--sqlite":
			dialect = "sqlite"
		}
	}
	return dialect
}

// defaultDatabaseURL builds a default database URL for the given dialect.
func defaultDatabaseURL(dialect, projectName, dir string) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf("postgres://postgres@localhost:5432/%s", projectName)
	case "mysql":
		return fmt.Sprintf("mysql://root@localhost:3306/%s", projectName)
	default: // "sqlite"
		dataDir := filepath.Join(dir, ".shipq", "data")
		dbPath := filepath.Join(dataDir, projectName+".db")
		return dburl.BuildSQLiteURL(dbPath)
	}
}

// createGoMod creates a new go.mod file with the module name "com.<projectName>"
func createGoMod(dir, projectName string) error {
	goVersion := getGoVersion()
	moduleName := fmt.Sprintf("com.%s", projectName)

	content := fmt.Sprintf("module %s\n\ngo %s\n", moduleName, goVersion)

	goModPath := filepath.Join(dir, project.GoModFile)
	return os.WriteFile(goModPath, []byte(content), 0644)
}

// createShipqIni creates a new shipq.ini file with a [db] section containing
// a default database_url for the chosen dialect, and a [typescript] section
// with default framework settings.
func createShipqIni(dir, projectName, dialect string) error {
	f := &inifile.File{}

	dbURL := defaultDatabaseURL(dialect, projectName, dir)

	f.Sections = append(f.Sections, inifile.Section{
		Name: "db",
		Values: []inifile.KeyValue{
			{Key: "database_url", Value: dbURL},
		},
	})

	// Add [typescript] section with default framework
	f.Sections = append(f.Sections, inifile.Section{
		Name: "typescript",
		Values: []inifile.KeyValue{
			{Key: "framework", Value: "react"},
			{Key: "http_output", Value: "."},
		},
	})

	shipqIniPath := filepath.Join(dir, project.ShipqIniFile)
	return f.WriteFile(shipqIniPath)
}

// ensureGitignore creates or updates .gitignore to include .shipq/
// Returns true if the file was created or modified, false if already up-to-date.
func ensureGitignore(dir string) (bool, error) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	shipqEntry := ".shipq/"

	// Read existing .gitignore if it exists
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	// Check if .shipq/ is already in the file
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Check for exact match or common variations
			if trimmed == shipqEntry || trimmed == ".shipq" || trimmed == "/.shipq/" || trimmed == "/.shipq" {
				// Already present
				return false, nil
			}
		}
	}

	// Append .shipq/ to the file
	var newContent string
	if len(content) == 0 {
		// New file - add header and entry
		newContent = "# shipq generated files\n" + shipqEntry + "\n"
	} else {
		// Existing file - append with appropriate newline handling
		existingContent := string(content)
		// Ensure there's a newline before our entry
		if !strings.HasSuffix(existingContent, "\n") {
			existingContent += "\n"
		}
		newContent = existingContent + "\n# shipq generated files\n" + shipqEntry + "\n"
	}

	if err := os.WriteFile(gitignorePath, []byte(newContent), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// createHealthEndpoint scaffolds api/health/register.go and api/health/health_check.go
// using the provided module path (e.g. "com.myproject"). It is idempotent: if
// api/health/register.go already exists the function returns (false, nil).
func createHealthEndpoint(dir, modulePath string) (bool, error) {
	healthDir := filepath.Join(dir, "api", "health")
	registerPath := filepath.Join(healthDir, "register.go")

	// Idempotency: skip if register.go already exists
	if _, err := os.Stat(registerPath); err == nil {
		return false, nil
	}

	if err := os.MkdirAll(healthDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create api/health directory: %w", err)
	}

	registerContent := fmt.Sprintf(`package health

import (
	"%s/shipq/lib/handler"
)

func Register(app *handler.App) {
	app.Get("/health", HealthCheck)
}
`, modulePath)

	if err := os.WriteFile(registerPath, []byte(registerContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write register.go: %w", err)
	}

	healthCheckContent := `package health

import "context"

type HealthCheckRequest struct{}

type HealthCheckResponse struct {
	Healthy bool ` + "`" + `json:"healthy"` + "`" + `
}

func HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	// The database ping is handled by the generated HTTP server layer;
	// this handler always reports healthy if it is reached.
	return &HealthCheckResponse{Healthy: true}, nil
}
`

	healthCheckPath := filepath.Join(healthDir, "health_check.go")
	if err := os.WriteFile(healthCheckPath, []byte(healthCheckContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write health_check.go: %w", err)
	}

	return true, nil
}

// getGoVersion returns the current Go version in "X.Y" format
func getGoVersion() string {
	version := runtime.Version()
	// runtime.Version() returns something like "go1.21.5"
	// We want "1.21"
	if len(version) > 2 && version[:2] == "go" {
		version = version[2:]
	}

	// Extract major.minor
	dotCount := 0
	for i, c := range version {
		if c == '.' {
			dotCount++
			if dotCount == 2 {
				return version[:i]
			}
		}
	}
	return version
}
