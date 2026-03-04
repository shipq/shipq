package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

// ValidDialects lists the accepted dialect names for `shipq db set <dialect>`.
var ValidDialects = []string{"sqlite", "postgres", "mysql"}

// DBSetCmd implements the "shipq db set <dialect>" command.
// It updates shipq.ini with the default localhost database URL for the chosen
// dialect. This is a lightweight alternative to `db setup` — it does NOT create
// the database or run any connections; it only writes the URL into the config.
func DBSetCmd(dialect string) {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project", err)
	}

	projectName := project.GetProjectName(roots.ShipqRoot)
	dbURL := DefaultDatabaseURL(dialect, projectName, roots.ShipqRoot)

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	iniFile, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	iniFile.Set("db", "database_url", dbURL)

	if err := iniFile.WriteFile(shipqIniPath); err != nil {
		cli.FatalErr("failed to write shipq.ini", err)
	}

	cli.Success(fmt.Sprintf("Set database dialect to %s", dialect))
	cli.Infof("  database_url = %s", dbURL)
	fmt.Println("")
	fmt.Println("Next step:")
	fmt.Println("  shipq db setup")
}

// DefaultDatabaseURL builds the default localhost database URL for a dialect.
// Exported so that init and tests can reuse the same logic.
func DefaultDatabaseURL(dialect, projectName, projectRoot string) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf("postgres://postgres@localhost:5432/%s?sslmode=disable", projectName)
	case "mysql":
		return fmt.Sprintf("mysql://root@localhost:3306/%s", projectName)
	default: // "sqlite"
		dataDir := filepath.Join(projectRoot, ".shipq", "data")
		dbPath := filepath.Join(dataDir, projectName+".db")
		return dburl.BuildSQLiteURL(dbPath)
	}
}

// IsValidDialect returns true if dialect is one of the accepted values.
func IsValidDialect(dialect string) bool {
	for _, d := range ValidDialects {
		if dialect == d {
			return true
		}
	}
	return false
}

// DBSetUsage prints help text for `shipq db set` to stderr and exits.
func DBSetUsage() {
	fmt.Fprintln(os.Stderr, "shipq db set - Set the database dialect")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: shipq db set <dialect>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Dialects:")
	fmt.Fprintln(os.Stderr, "  sqlite     SQLite (file-based, no server needed)")
	fmt.Fprintln(os.Stderr, "  postgres   PostgreSQL (localhost:5432)")
	fmt.Fprintln(os.Stderr, "  mysql      MySQL (localhost:3306)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "This updates shipq.ini with the default localhost URL for the chosen")
	fmt.Fprintln(os.Stderr, "dialect. It does NOT create the database — run 'shipq db setup' after.")
}
