package start

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

// StartCentrifugo implements "shipq start centrifugo".
// It starts a standalone foreground Centrifugo process, waits for readiness,
// then blocks until the process exits or a signal is received.
func StartCentrifugo() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	// Check centrifugo binary is on $PATH.
	centrifugoPath, err := exec.LookPath("centrifugo")
	if err != nil {
		cli.Fatal("centrifugo not found on $PATH -- add it to your shell.nix")
	}
	fmt.Printf("  Checking centrifugo... found at %s\n", centrifugoPath)

	// centrifugo.json must exist.
	centrifugoConfigPath := filepath.Join(roots.ShipqRoot, "centrifugo.json")
	if _, err := os.Stat(centrifugoConfigPath); os.IsNotExist(err) {
		cli.Fatal("centrifugo.json not found -- run 'shipq workers' first")
	}

	// Read shipq.ini for the API URL and key (needed for the readiness check).
	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	centrifugoAPIURL := ini.Get("workers", "centrifugo_api_url")
	if centrifugoAPIURL == "" {
		centrifugoAPIURL = "http://localhost:8000/api"
	}
	centrifugoAPIKey := ini.Get("workers", "centrifugo_api_key")

	centrifugoWS := ini.Get("workers", "centrifugo_ws_url")
	if centrifugoWS == "" {
		centrifugoWS = "ws://localhost:8000/connection/websocket"
	}

	// Start Centrifugo.
	fmt.Printf("  Starting Centrifugo (config: centrifugo.json, port 8000)...\n")

	centrifugoCmd := exec.Command("centrifugo", "--config", centrifugoConfigPath, "-p", "8000")
	centrifugoCmd.Dir = roots.ShipqRoot
	centrifugoCmd.Stdout = newPrefixWriter(os.Stdout, "[centrifugo] ")
	centrifugoCmd.Stderr = newPrefixWriter(os.Stderr, "[centrifugo] ")

	if err := centrifugoCmd.Start(); err != nil {
		cli.FatalErr("failed to start Centrifugo", err)
	}

	// Wait for Centrifugo to be ready.
	fmt.Println("  Waiting for Centrifugo to be ready...")

	if err := waitForCentrifugo(centrifugoAPIURL, centrifugoAPIKey, 10*time.Second); err != nil {
		_ = centrifugoCmd.Process.Signal(syscall.SIGTERM)
		_ = centrifugoCmd.Wait()
		cli.FatalErr("Centrifugo failed readiness check", err)
	}

	cli.Success("Centrifugo is ready")
	fmt.Printf("  WebSocket endpoint: %s\n", centrifugoWS)
	fmt.Println("")
	fmt.Println("  Press Ctrl-C to stop.")
	fmt.Println("")

	// Forward SIGINT/SIGTERM to Centrifugo.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down Centrifugo...", sig)
		terminateProcess(centrifugoCmd)
	}()

	if err := centrifugoCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("Centrifugo stopped")
					return
				}
			}
		}
		cli.FatalErr("Centrifugo exited with error", err)
	}
}
