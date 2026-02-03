// Package cli provides the PortSQL command-line interface.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

// Database server data directory paths (relative to project root)
const (
	postgresDataDir = "db/databases/.postgres-data"
	mysqlDataDir    = "db/databases/.mysql-data"
)

// StartPostgres starts a local Postgres server.
// It initializes the data directory if needed, then runs postgres in the foreground.
func StartPostgres(cfg *Config, stdout, stderr io.Writer) error {
	// Verify required binaries are on PATH
	if _, err := exec.LookPath("initdb"); err != nil {
		return fmt.Errorf("initdb not found on PATH\n  Install PostgreSQL (e.g., brew install postgresql on macOS)")
	}
	if _, err := exec.LookPath("postgres"); err != nil {
		return fmt.Errorf("postgres not found on PATH\n  Install PostgreSQL (e.g., brew install postgresql on macOS)")
	}

	// Get project root (we're already chdir'd to project root by ShipQ)
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	dataDir := filepath.Join(projectRoot, postgresDataDir)

	// Initialize data directory if it doesn't exist
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Fprintf(stdout, "Initializing Postgres data directory: %s\n", dataDir)

		// Create parent directories
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Run initdb
		initCmd := exec.Command("initdb", "-D", dataDir, "--username=postgres")
		initCmd.Stdout = stdout
		initCmd.Stderr = stderr
		if err := initCmd.Run(); err != nil {
			return fmt.Errorf("initdb failed: %w\n  Data directory: %s", err, dataDir)
		}
		fmt.Fprintf(stdout, "Postgres data directory initialized.\n\n")
	}

	// Print startup info
	fmt.Fprintf(stdout, "Starting Postgres server...\n")
	fmt.Fprintf(stdout, "  Data directory: %s\n", dataDir)
	fmt.Fprintf(stdout, "  Connect with: psql -h localhost -U postgres\n")
	fmt.Fprintf(stdout, "  Press Ctrl+C to stop.\n\n")

	// Start postgres in foreground
	pgCmd := exec.Command("postgres", "-D", dataDir)
	pgCmd.Stdout = stdout
	pgCmd.Stderr = stderr

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if err := pgCmd.Start(); err != nil {
		return fmt.Errorf("failed to start postgres: %w", err)
	}

	// Wait for either the process to exit or a signal
	done := make(chan error, 1)
	go func() {
		done <- pgCmd.Wait()
	}()

	select {
	case err := <-done:
		// Process exited on its own
		if err != nil {
			return fmt.Errorf("postgres exited with error: %w", err)
		}
		return nil
	case sig := <-sigChan:
		// Received shutdown signal
		fmt.Fprintf(stdout, "\nReceived %v, shutting down Postgres...\n", sig)

		// Forward signal to child process
		if pgCmd.Process != nil {
			pgCmd.Process.Signal(sig)
		}

		// Wait for process to exit
		<-done
		fmt.Fprintf(stdout, "Postgres stopped.\n")
		return nil
	}
}

// StartMySQL starts a local MySQL server.
// It initializes the data directory if needed, then runs mysqld in the foreground.
func StartMySQL(cfg *Config, stdout, stderr io.Writer) error {
	// Verify required binary is on PATH
	if _, err := exec.LookPath("mysqld"); err != nil {
		return fmt.Errorf("mysqld not found on PATH\n  Install MySQL (e.g., brew install mysql on macOS)")
	}

	// Get project root (we're already chdir'd to project root by ShipQ)
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	dataDir := filepath.Join(projectRoot, mysqlDataDir)
	socketPath := filepath.Join(dataDir, "mysql.sock")
	mysqlxSocketPath := filepath.Join(dataDir, "mysqlx.sock")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize if mysql subdirectory doesn't exist
	mysqlSubDir := filepath.Join(dataDir, "mysql")
	if _, err := os.Stat(mysqlSubDir); os.IsNotExist(err) {
		fmt.Fprintf(stdout, "Initializing MySQL data directory: %s\n", dataDir)

		initCmd := exec.Command("mysqld", "--initialize-insecure", "--datadir="+dataDir)
		initCmd.Stdout = stdout
		initCmd.Stderr = stderr
		if err := initCmd.Run(); err != nil {
			return fmt.Errorf("mysqld --initialize-insecure failed: %w\n  Data directory: %s", err, dataDir)
		}
		fmt.Fprintf(stdout, "MySQL data directory initialized.\n\n")
	}

	// Remove undo transaction files (matches shell script behavior)
	undoFiles, _ := filepath.Glob(filepath.Join(dataDir, "undo_*"))
	for _, f := range undoFiles {
		os.Remove(f)
	}

	// Print startup info
	fmt.Fprintf(stdout, "Starting MySQL server...\n")
	fmt.Fprintf(stdout, "  Data directory: %s\n", dataDir)
	fmt.Fprintf(stdout, "  Socket: %s\n", socketPath)
	fmt.Fprintf(stdout, "  Connect with: mysql -u root --socket=%s\n", socketPath)
	fmt.Fprintf(stdout, "  Press Ctrl+C to stop.\n\n")

	// Start mysqld
	mysqlCmd := exec.Command("mysqld",
		"--datadir="+dataDir,
		"--socket="+socketPath,
		"--mysqlx-socket="+mysqlxSocketPath,
		"--console",
	)
	mysqlCmd.Stdout = stdout
	mysqlCmd.Stderr = stderr

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if err := mysqlCmd.Start(); err != nil {
		return fmt.Errorf("failed to start mysqld: %w", err)
	}

	// Wait for either the process to exit or a signal
	done := make(chan error, 1)
	go func() {
		done <- mysqlCmd.Wait()
	}()

	select {
	case err := <-done:
		// Process exited on its own
		if err != nil {
			return fmt.Errorf("mysqld exited with error: %w", err)
		}
		return nil
	case sig := <-sigChan:
		// Received shutdown signal
		fmt.Fprintf(stdout, "\nReceived %v, shutting down MySQL...\n", sig)

		// MySQL requires SIGTERM for graceful shutdown (SIGINT is ignored)
		if mysqlCmd.Process != nil {
			mysqlCmd.Process.Signal(syscall.SIGTERM)
		}

		// Wait for process to exit
		<-done
		fmt.Fprintf(stdout, "MySQL stopped.\n")
		return nil
	}
}
