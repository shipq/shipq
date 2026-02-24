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

// StartServer implements "shipq start server".
// When watch is true it uses go build + fsnotify for hot reload.
// When watch is false it falls back to the original go run behaviour.
func StartServer(watch bool) {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	// Validate that cmd/server/main.go exists.
	serverMainPath := filepath.Join(roots.ShipqRoot, "cmd", "server", "main.go")
	if !fileExists(serverMainPath) {
		cli.Fatal("cmd/server/main.go not found -- run 'shipq init' or scaffold your server first")
	}

	if watch {
		fmt.Println("  Starting server with hot reload (go build + watch)...")
		fmt.Println("  Watching for .go file changes.  Use --no-watch to disable.")
		fmt.Println("")
		RunWithWatch(WatchConfig{
			ProjectRoot: roots.ShipqRoot,
			BuildCmd:    []string{"go", "build", "-o", filepath.Join(roots.ShipqRoot, ".shipq", "bin", "server"), "./cmd/server"},
			BinPath:     filepath.Join(roots.ShipqRoot, ".shipq", "bin", "server"),
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
			Name:        "server",
		})
		return
	}

	// --no-watch: original go run flow.
	fmt.Println("  Starting server (go run ./cmd/server)...")
	fmt.Println("")

	serverCmd := exec.Command("go", "run", "./cmd/server")
	serverCmd.Dir = roots.ShipqRoot
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	if err := serverCmd.Start(); err != nil {
		cli.FatalErr("failed to start server", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down server...", sig)
		if serverCmd.Process != nil {
			_ = serverCmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := serverCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("Server stopped")
					return
				}
			}
		}
		cli.FatalErr("server exited with error", err)
	}
}
