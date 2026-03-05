package start

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/shipq/shipq/cli"
	killcmd "github.com/shipq/shipq/internal/commands/kill"
	"github.com/shipq/shipq/project"
)

const defaultServerPort = 8080

// serverPort returns the port the server will listen on.
// It checks the PORT environment variable first, falling back to 8080.
func serverPort() int {
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 65535 {
			return n
		}
	}
	return defaultServerPort
}

// killStaleServer attempts to kill any process occupying the server port.
// It prints a message when a stale process is found and killed.
// Errors are logged as warnings but do not prevent startup.
func killStaleServer(port int) {
	killed, err := killcmd.KillPort(port)
	if err != nil {
		cli.Warnf("could not check port %d for stale processes: %v", port, err)
		return
	}
	if killed {
		cli.Infof("Killed stale process on port %d", port)
	}
}

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

	// Kill any stale process hogging the server port from a previous session.
	killStaleServer(serverPort())

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
	// Place the child in its own process group so we can kill `go run`
	// AND the server binary it spawns, preventing orphaned zombies.
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := serverCmd.Start(); err != nil {
		cli.FatalErr("failed to start server", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down server...", sig)
		if serverCmd.Process != nil {
			// Kill the entire process group (negative PID) so that
			// both `go run` and the spawned server binary receive
			// the signal.
			_ = syscall.Kill(-serverCmd.Process.Pid, syscall.SIGTERM)
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
