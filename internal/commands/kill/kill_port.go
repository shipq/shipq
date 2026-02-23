package kill

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shipq/shipq/cli"
)

const TermGracePeriod = 3 * time.Second
const KillPollInterval = 200 * time.Millisecond

// killPort finds the process(es) bound to the given TCP port and terminates
// them.  It sends SIGTERM first; if a process is still alive after
// TermGracePeriod it sends SIGKILL.
//
// Returns killed=true when at least one PID was acted upon, or
// killed=false, err=nil when nothing was listening on the port.
func killPort(port int) (killed bool, err error) {
	lsofPath, err := exec.LookPath("lsof")
	if err != nil {
		return false, fmt.Errorf(
			"'lsof' not found on $PATH – please install it " +
				"(on NixOS add 'lsof' to shell.nix buildInputs)",
		)
	}

	// -t  = terse output (PIDs only)
	// -i tcp:<port> = filter to the given TCP port
	cmd := exec.Command(lsofPath, "-t", "-i", fmt.Sprintf("tcp:%d", port))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// lsof exits non-zero when there are no matching processes.
		if stdout.Len() == 0 {
			return false, nil
		}
		// Non-empty stdout with non-zero exit is still useful – fall through
		// and try to parse the PIDs.
	}

	pids, err := parsePIDs(stdout.String())
	if err != nil {
		return false, fmt.Errorf("parsing lsof output: %w", err)
	}
	if len(pids) == 0 {
		return false, nil
	}

	for _, pid := range pids {
		if termErr := signalAndWait(pid); termErr != nil {
			cli.Warnf("could not kill PID %d: %v", pid, termErr)
		}
	}

	return true, nil
}

// parsePIDs reads lsof terse output (one PID per line) and returns a
// deduplicated slice of integers.  Lines that fail to parse are warned about
// and skipped.
func parsePIDs(output string) ([]int, error) {
	seen := make(map[int]bool)
	var pids []int

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			cli.Warnf("skipping non-integer PID line %q: %v", line, err)
			continue
		}
		if !seen[pid] {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}

	return pids, scanner.Err()
}

// signalAndWait sends SIGTERM to pid and polls until the process exits or
// TermGracePeriod elapses, at which point it sends SIGKILL.
func signalAndWait(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		// ESRCH means the process is already gone – treat as success.
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("SIGTERM to PID %d: %w", pid, err)
	}

	deadline := time.Now().Add(TermGracePeriod)
	for time.Now().Before(deadline) {
		time.Sleep(KillPollInterval)
		// kill -0 checks whether the process exists without sending a signal.
		if err := syscall.Kill(pid, 0); err != nil {
			// ESRCH = no such process → it exited.
			if err == syscall.ESRCH {
				return nil
			}
		}
	}

	// Grace period expired; force-kill.
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("SIGKILL to PID %d: %w", pid, err)
	}

	return nil
}
