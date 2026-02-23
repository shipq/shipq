package start

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/project"
)

// StartPostgres implements "shipq start postgres".
// It initialises the data directory on first run, then starts a foreground
// postgres process and forwards SIGINT/SIGTERM to it.
func StartPostgres() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	dataDir := filepath.Join(roots.ShipqRoot, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		cli.FatalErr("failed to create data directory", err)
	}

	pgDataDir := filepath.Join(dataDir, ".postgres-data")

	// Initialise if needed.
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

	// Wait for the process, handling clean signal-induced shutdown.
	if err := pgCmd.Wait(); err != nil {
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
