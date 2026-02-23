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

// StartRedis implements "shipq start redis".
// It starts a foreground redis-server process and forwards SIGINT/SIGTERM to it.
func StartRedis() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	// Check redis-server is on $PATH.
	if _, err := exec.LookPath("redis-server"); err != nil {
		cli.Fatal("redis-server not found on $PATH -- add it to your shell.nix")
	}

	dataDir := filepath.Join(roots.ShipqRoot, ".shipq", "data")
	redisDataDir := filepath.Join(dataDir, ".redis-data")

	if err := os.MkdirAll(redisDataDir, 0755); err != nil {
		cli.FatalErr("failed to create Redis data directory", err)
	}

	cli.Info("Starting Redis server...")
	cli.Infof("Data directory: %s", redisDataDir)
	cli.Info("Connect with: redis://localhost:6379")
	cli.Info("")

	redisCmd := exec.Command("redis-server",
		"--dir", redisDataDir,
		"--daemonize", "no",
		"--port", "6379",
	)
	redisCmd.Stdout = os.Stdout
	redisCmd.Stderr = os.Stderr

	if err := redisCmd.Start(); err != nil {
		cli.FatalErr("failed to start Redis", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down Redis...", sig)
		if redisCmd.Process != nil {
			_ = redisCmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := redisCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("Redis stopped")
					return
				}
			}
		}
		cli.FatalErr("Redis exited with error", err)
	}
}
