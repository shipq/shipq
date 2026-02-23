package start

import (
	"os"
	"path/filepath"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/project"
)

// StartSQLite implements "shipq start sqlite".
// SQLite does not require a running server process. This command ensures the
// database file exists and prints the connection string.
func StartSQLite() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	dataDir := filepath.Join(roots.ShipqRoot, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		cli.FatalErr("failed to create data directory", err)
	}

	sqliteDBPath := filepath.Join(dataDir, ".sqlite-db")

	if !fileExists(sqliteDBPath) {
		file, err := os.Create(sqliteDBPath)
		if err != nil {
			cli.FatalErr("failed to create SQLite database file", err)
		}
		file.Close()
		cli.Success("Created SQLite database file")
	}

	cli.Infof("SQLite database path: %s", sqliteDBPath)
	cli.Infof("Connect with: sqlite://%s", sqliteDBPath)
	cli.Info("")
	cli.Info("SQLite doesn't require a running server. Use the path above in your connection string.")
}
