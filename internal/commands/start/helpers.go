package start

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shipq/shipq/cli"
)

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// removeUndoFiles removes undo_* files from the given directory.
// These files can cause issues with MySQL initialization.
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

// waitForCentrifugo polls POST <apiURL>/info with the X-API-Key header until
// it gets HTTP 200, with exponential backoff and a timeout.
// Centrifugo v6 has no /health endpoint; use POST /api/info instead.
func waitForCentrifugo(apiURL, apiKey string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 100 * time.Millisecond
	maxInterval := 2 * time.Second

	client := &http.Client{Timeout: 2 * time.Second}

	infoURL := strings.TrimSuffix(apiURL, "/") + "/info"

	for time.Now().Before(deadline) {
		req, err := http.NewRequest("POST", infoURL, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(interval)
		interval *= 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}

	return fmt.Errorf("timed out after %s waiting for Centrifugo at %s", timeout, infoURL)
}

// terminateProcess sends SIGTERM to a process and waits for it to exit.
// If the process does not exit within 5 seconds it sends SIGKILL.
func terminateProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited cleanly.
	case <-time.After(5 * time.Second):
		forceKillProcess(cmd)
	}
}

// forceKillProcess sends SIGKILL to a process.
func forceKillProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGKILL)
}

// prefixWriter wraps an io.Writer and prepends a fixed prefix to every line.
type prefixWriter struct {
	w      io.Writer
	prefix string
	buf    []byte
}

// newPrefixWriter returns a new prefixWriter that writes to w.
func newPrefixWriter(w io.Writer, prefix string) *prefixWriter {
	return &prefixWriter{w: w, prefix: prefix}
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.buf = append(pw.buf, p...)
	totalWritten := len(p)

	for {
		idx := -1
		for i, b := range pw.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}

		line := pw.buf[:idx+1]
		pw.buf = pw.buf[idx+1:]

		if _, err := fmt.Fprintf(pw.w, "%s%s", pw.prefix, line); err != nil {
			return totalWritten, err
		}
	}

	return totalWritten, nil
}

// runProcess starts cmd, registers a SIGINT/SIGTERM forwarding goroutine,
// waits for the process to exit, and prints a "stopped" message on clean
// signal-induced termination.
func runProcess(cmd *exec.Cmd, name string) {
	if err := cmd.Start(); err != nil {
		cli.FatalErr("failed to start "+name, err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down %s...", sig, name)
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Infof("%s stopped", name)
					return
				}
			}
		}
		cli.FatalErr(name+" exited with error", err)
	}
}
