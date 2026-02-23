package start

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/project"
)

// StartMySQL implements "shipq start mysql".
// It initialises the data directory on first run, then starts a foreground
// mysqld process and forwards SIGINT/SIGTERM to it.
func StartMySQL() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	dataDir := filepath.Join(roots.ShipqRoot, ".shipq", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		cli.FatalErr("failed to create data directory", err)
	}

	mysqlDataDir := filepath.Join(dataDir, ".mysql-data")

	// Initialise if needed.
	if !dirExists(mysqlDataDir) {
		cli.Info("Initializing MySQL data directory...")
		initCmd := exec.Command("mysqld", "--initialize-insecure", "--datadir="+mysqlDataDir)
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		if err := initCmd.Run(); err != nil {
			cli.FatalErr("failed to initialize MySQL", err)
		}
		removeUndoFiles(mysqlDataDir)
		cli.Success("MySQL initialized")
	}

	// Remove stale undo tablespace files that prevent InnoDB from starting.
	// MySQL leaves these behind after unclean shutdowns and refuses to boot
	// if they already exist during initialisation.
	removeUndoFiles(mysqlDataDir)

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down MySQL...", sig)
		if mysqlCmd.Process != nil {
			_ = mysqlCmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := mysqlCmd.Wait(); err != nil {
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
