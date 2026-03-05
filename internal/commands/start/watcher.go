package start

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/shipq/shipq/cli"
)

// WatchConfig holds all configuration needed by the watch-and-restart loop.
type WatchConfig struct {
	// ProjectRoot is the directory to watch recursively.
	ProjectRoot string

	// BuildCmd is the build command, e.g. []string{"go", "build", "-o", binPath, target}.
	BuildCmd []string

	// BinPath is the absolute path to the compiled binary.
	BinPath string

	// RunArgs are extra arguments passed to the binary.
	RunArgs []string

	// Stdout / Stderr for the child process.
	Stdout io.Writer
	Stderr io.Writer

	// Name is a human-readable label ("server" or "worker").
	Name string
}

// skippedDirs is the set of directory base-names that should never be watched.
var skippedDirs = map[string]bool{
	".shipq":       true,
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"test_results": true,
}

// shouldSkipDir returns true if a directory with the given base name should be
// excluded from watching.
func shouldSkipDir(name string) bool {
	if skippedDirs[name] {
		return true
	}
	// Skip hidden directories (names starting with '.') that aren't already
	// covered by the explicit map.
	if len(name) > 0 && name[0] == '.' {
		return true
	}
	return false
}

// isGoFile returns true if the file path ends with ".go".
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}

// RunWithWatch implements the hot-reload loop: it watches the project tree for
// .go file changes, rebuilds the binary, and restarts the child process.
func RunWithWatch(cfg WatchConfig) {
	// 1. Ensure the output directory exists.
	if err := os.MkdirAll(filepath.Dir(cfg.BinPath), 0755); err != nil {
		cli.FatalErr("failed to create build output directory", err)
	}

	// 2. Set up fsnotify watcher.
	w, err := fsnotify.NewWatcher()
	if err != nil {
		cli.FatalErr("failed to create file watcher", err)
	}
	defer w.Close()

	// 3. Recursively add directories to the watcher.
	_ = filepath.WalkDir(cfg.ProjectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			if addErr := w.Add(path); addErr != nil {
				cli.Warnf("could not watch %s: %v", path, addErr)
			}
		}
		return nil
	})

	// 4. Set up OS signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 5. Initial build + run.
	var cmd *exec.Cmd
	var childDone chan struct{}
	if build(cfg) {
		cmd, childDone = startChild(cfg)
	} else {
		cli.Warn("Initial build failed. Watching for changes...")
	}

	// 6. Debounce timer (stopped initially).
	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	var pendingPath string

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}

			// New directory? Watch it so we pick up new packages.
			if ev.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					if !shouldSkipDir(filepath.Base(ev.Name)) {
						_ = w.Add(ev.Name)
					}
					continue
				}
			}

			// Only care about .go files.
			if !isGoFile(ev.Name) {
				continue
			}

			// Reset debounce timer.
			pendingPath = ev.Name
			debounce.Reset(500 * time.Millisecond)

		case <-debounce.C:
			rel, relErr := filepath.Rel(cfg.ProjectRoot, pendingPath)
			if relErr != nil {
				rel = pendingPath
			}
			cli.Infof("File changed: %s — restarting %s...", rel, cfg.Name)

			// Stop old child.
			if cmd != nil && cmd.Process != nil {
				stopChild(cmd, childDone)
			}

			// Rebuild.
			if build(cfg) {
				cmd, childDone = startChild(cfg)
			} else {
				cmd = nil
				childDone = nil
				cli.Warn("Build failed. Watching for more changes...")
			}

		case wErr, ok := <-w.Errors:
			if !ok {
				return
			}
			cli.Warnf("watcher error: %v", wErr)

		case sig := <-sigCh:
			cli.Infof("Received %s, shutting down %s...", sig, cfg.Name)
			if cmd != nil && cmd.Process != nil {
				stopChild(cmd, childDone)
			}
			_ = os.Remove(cfg.BinPath)
			return
		}
	}
}

// build runs the build command and returns true if it succeeds.
func build(cfg WatchConfig) bool {
	cmd := exec.Command(cfg.BuildCmd[0], cfg.BuildCmd[1:]...)
	cmd.Dir = cfg.ProjectRoot
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	return cmd.Run() == nil
}

// startChild starts the compiled binary and returns the exec.Cmd and a channel
// that is closed when the child exits.
func startChild(cfg WatchConfig) (*exec.Cmd, chan struct{}) {
	cmd := exec.Command(cfg.BinPath, cfg.RunArgs...)
	cmd.Dir = cfg.ProjectRoot
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	// Place the child in its own process group so we can kill the entire
	// tree (the binary and any children it spawns) when restarting.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		cli.Warnf("failed to start %s: %v", cfg.Name, err)
		return nil, nil
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	return cmd, done
}

// stopChild sends SIGTERM to the child's process group, waits up to 5
// seconds, then SIGKILLs the group.  Targeting the process group (negative
// PID) ensures that any grandchild processes spawned by the binary are also
// terminated, preventing orphaned zombies that hog ports across restarts.
func stopChild(cmd *exec.Cmd, done chan struct{}) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	// Check if the child has already exited.
	if done != nil {
		select {
		case <-done:
			return
		default:
		}
	}

	// Kill the entire process group (negative PID).
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

	if done != nil {
		select {
		case <-done:
			return
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
	} else {
		// Fallback: no done channel, use terminateProcess from helpers.
		terminateProcess(cmd)
	}
}
