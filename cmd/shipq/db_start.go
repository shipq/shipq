package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/project"
)

const (
	dbTypePostgres = "postgres"
	dbTypeMySQL    = "mysql"
	dbTypeSQLite   = "sqlite"
)

// dbStartCmd implements the "shipq db start <type>" command.
// It starts a local database server for development.
func dbStartCmd(dbType string) {
	// Validate database type
	switch dbType {
	case dbTypePostgres, dbTypeMySQL, dbTypeSQLite:
		// Valid type
	default:
		cli.Fatal(fmt.Sprintf("unknown database type: %s (must be postgres, mysql, or sqlite)", dbType))
	}

	// Find and validate project root
	projectRoot, err := project.FindProjectRoot()
	if err != nil {
		cli.FatalErr("failed to find project root", err)
	}

	if err := project.ValidateProjectRoot(projectRoot); err != nil {
		cli.FatalErr("invalid project", err)
	}

	// Create data directory
	dataDir := filepath.Join(projectRoot, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		cli.FatalErr("failed to create data directory", err)
	}

	switch dbType {
	case dbTypePostgres:
		startPostgres(dataDir)
	case dbTypeMySQL:
		startMySQL(dataDir)
	case dbTypeSQLite:
		startSQLite(dataDir)
	}
}

// startPostgres starts a PostgreSQL server.
func startPostgres(dataDir string) {
	pgDataDir := filepath.Join(dataDir, ".postgres-data")

	// Initialize if needed
	if !dirExists(pgDataDir) {
		cli.Info("Initializing PostgreSQL data directory...")
		initCmd := exec.Command("initdb", "-D", pgDataDir, "--username=postgres")
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		if err := initCmd.Run(); err != nil {
			cli.FatalErr("failed to initialize PostgreSQL", err)
		}
		cli.Success("PostgreSQL initialized")
	}

	// Start PostgreSQL
	cli.Info("Starting PostgreSQL server...")
	cli.Infof("Data directory: %s", pgDataDir)
	cli.Info("Connect with: postgres://postgres@localhost:5432/<dbname>")
	cli.Info("")

	pgCmd := exec.Command("postgres", "-D", pgDataDir)
	pgCmd.Stdout = os.Stdout
	pgCmd.Stderr = os.Stderr

	if err := pgCmd.Start(); err != nil {
		cli.FatalErr("failed to start PostgreSQL", err)
	}

	// Wait for process to exit
	if err := pgCmd.Wait(); err != nil {
		// Check if it was killed by a signal (normal shutdown)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("PostgreSQL stopped")
					return
				}
			}
		}
		cli.FatalErr("PostgreSQL exited with error", err)
	}
}

// startMySQL starts a MySQL server.
func startMySQL(dataDir string) {
	mysqlDataDir := filepath.Join(dataDir, ".mysql-data")

	// Initialize if needed
	if !dirExists(mysqlDataDir) {
		cli.Info("Initializing MySQL data directory...")
		initCmd := exec.Command("mysqld", "--initialize-insecure", "--datadir="+mysqlDataDir)
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		if err := initCmd.Run(); err != nil {
			cli.FatalErr("failed to initialize MySQL", err)
		}

		// Remove undo_* files that can cause issues
		removeUndoFiles(mysqlDataDir)
		cli.Success("MySQL initialized")
	}

	// Start MySQL
	cli.Info("Starting MySQL server...")
	cli.Infof("Data directory: %s", mysqlDataDir)
	cli.Info("Connect with: mysql://root@localhost:3306/<dbname>")
	cli.Info("")

	socketPath := filepath.Join(mysqlDataDir, "mysql.sock")
	mysqlxSocketPath := filepath.Join(mysqlDataDir, "mysqlx.sock")

	mysqlCmd := exec.Command("mysqld",
		"--datadir="+mysqlDataDir,
		"--socket="+socketPath,
		"--mysqlx-socket="+mysqlxSocketPath,
		"--console",
	)
	mysqlCmd.Stdout = os.Stdout
	mysqlCmd.Stderr = os.Stderr

	if err := mysqlCmd.Start(); err != nil {
		cli.FatalErr("failed to start MySQL", err)
	}

	// Set up signal handler to gracefully shutdown MySQL
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cli.Info("Shutting down MySQL...")
		mysqlCmd.Process.Signal(syscall.SIGTERM)
	}()

	// Wait for process to exit
	if err := mysqlCmd.Wait(); err != nil {
		// Check if it was killed by a signal (normal shutdown)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("MySQL stopped")
					return
				}
			}
		}
		cli.FatalErr("MySQL exited with error", err)
	}
}

// startSQLite sets up SQLite database file.
func startSQLite(dataDir string) {
	sqliteDBPath := filepath.Join(dataDir, ".sqlite-db")

	// Touch the file if it doesn't exist
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

// dirExists returns true if the directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// fileExists returns true if the file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// removeUndoFiles removes undo_* files from the MySQL data directory.
func removeUndoFiles(dataDir string) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[:5] == "undo_" {
			os.Remove(filepath.Join(dataDir, entry.Name()))
		}
	}
}
