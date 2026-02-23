package start

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

// StartWorker implements "shipq start worker".
// It runs `go run ./cmd/worker` as a foreground child process, forwarding
// SIGINT/SIGTERM to it.
func StartWorker() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	// Validate that cmd/worker/main.go exists.
	workerMainPath := filepath.Join(roots.ShipqRoot, "cmd", "worker", "main.go")
	if !fileExists(workerMainPath) {
		cli.Fatal("cmd/worker/main.go not found -- run 'shipq workers' first")
	}

	fmt.Println("  Starting worker (go run ./cmd/worker)...")
	fmt.Println("")

	workerCmd := exec.Command("go", "run", "./cmd/worker")
	workerCmd.Dir = roots.ShipqRoot
	workerCmd.Stdout = newPrefixWriter(os.Stdout, "[worker] ")
	workerCmd.Stderr = newPrefixWriter(os.Stderr, "[worker] ")

	if err := workerCmd.Start(); err != nil {
		cli.FatalErr("failed to start worker", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down worker...", sig)
		terminateProcess(workerCmd)
	}()

	if err := workerCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("Worker stopped")
					return
				}
			}
		}
		cli.FatalErr("worker exited with error", err)
	}
}
