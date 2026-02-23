package kill

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shipq/shipq/cli"
)

// KillPortCmd is the entry point for "shipq kill-port <port>".
// It validates rawPort, delegates to killPort, and prints the result.
func KillPortCmd(rawPort string) {
	port, err := parsePort(rawPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Usage: shipq kill-port <port>")
		os.Exit(1)
	}

	killed, err := killPort(port)
	if err != nil {
		cli.FatalErr("kill-port failed", err)
	}

	if !killed {
		fmt.Printf("Nothing found on port %d, nothing to do.\n", port)
	} else {
		fmt.Printf("Killed process(es) on port %d.\n", port)
	}
}

// KillDefaultsCmd is the entry point for "shipq kill-defaults".
// It iterates defaultPorts and calls killPort for each entry.
func KillDefaultsCmd() {
	for _, svc := range defaultPorts {
		for _, port := range svc.ports {
			if len(svc.ports) == 1 {
				fmt.Printf("  %s (%d)\n", svc.name, port)
			} else {
				fmt.Printf("  %s (%d)\n", svc.name, port)
			}

			killed, err := killPort(port)
			if err != nil {
				cli.Warnf("    could not kill port %d: %v", port, err)
				continue
			}
			if killed {
				fmt.Println("    ✓ killed")
			} else {
				fmt.Println("    - nothing running")
			}
		}
	}

	fmt.Println("Done.")
}

// parsePort trims whitespace from rawPort, parses it as an integer, and
// validates that it is in the range [1, 65535].
func parsePort(rawPort string) (int, error) {
	trimmed := strings.TrimSpace(rawPort)
	if trimmed == "" {
		return 0, fmt.Errorf("port must not be empty")
	}

	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: not an integer", trimmed)
	}

	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("invalid port %d: must be in the range 1–65535", n)
	}

	return n, nil
}
