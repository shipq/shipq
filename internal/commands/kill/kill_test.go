package kill

import (
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parsePort – validation table
// ---------------------------------------------------------------------------

func TestParsePort(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
		wantVal int
	}{
		{input: "8080", wantErr: false, wantVal: 8080},
		{input: "1", wantErr: false, wantVal: 1},
		{input: "65535", wantErr: false, wantVal: 65535},
		{input: "0", wantErr: true},
		{input: "65536", wantErr: true},
		{input: "-1", wantErr: true},
		{input: "abc", wantErr: true},
		{input: "", wantErr: true},
		{input: " 8080 ", wantErr: false, wantVal: 8080},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got, err := parsePort(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parsePort(%q): expected error, got nil (value %d)", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parsePort(%q): unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.wantVal {
				t.Errorf("parsePort(%q): got %d, want %d", tc.input, got, tc.wantVal)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// killPort – no-op path
// ---------------------------------------------------------------------------

// freePort returns a TCP port that is not currently in use by binding to :0,
// recording the chosen port, and immediately closing the listener.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestKillPort_NoProcess(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not found on $PATH; skipping")
	}

	port := freePort(t)

	killed, err := KillPort(port)
	if err != nil {
		t.Fatalf("killPort(%d): unexpected error: %v", port, err)
	}
	if killed {
		t.Errorf("killPort(%d): expected killed=false for an unoccupied port, got true", port)
	}
}

// ---------------------------------------------------------------------------
// killPort – live-process path
// ---------------------------------------------------------------------------

// TestKillPort_LiveProcess spawns an external subprocess (nc) that holds a
// TCP port open, then verifies that killPort terminates it.  We must use an
// external process because lsof would find the test binary itself if we used
// an in-process net.Listener, causing killPort to SIGTERM the test runner.
func TestKillPort_LiveProcess(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not found on $PATH; skipping")
	}
	if _, err := exec.LookPath("nc"); err != nil {
		t.Skip("nc not found on $PATH; skipping")
	}

	// Grab a free port by binding :0, then close immediately so nc can bind it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Start nc as an external listener so lsof reports nc's PID, not ours.
	cmd := exec.Command("nc", "-l", strconv.Itoa(port))
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start nc: %v", err)
	}
	// Ensure nc is cleaned up even if the test fails early.
	defer cmd.Process.Kill() //nolint:errcheck

	// Give nc time to bind the port.
	time.Sleep(150 * time.Millisecond)

	killed, err := KillPort(port)
	if err != nil {
		t.Fatalf("killPort(%d): unexpected error: %v", port, err)
	}
	if !killed {
		t.Fatalf("killPort(%d): expected killed=true, got false", port)
	}

	// Wait for nc to actually exit (killPort already waited, but be sure).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		// nc exited – success.
	case <-time.After(5 * time.Second):
		t.Error("nc did not exit within 5 s after killPort returned")
	}
}

// ---------------------------------------------------------------------------
// defaultPorts – smoke / shape test
// ---------------------------------------------------------------------------

func TestDefaultPorts_Shape(t *testing.T) {
	if len(defaultPorts) == 0 {
		t.Fatal("defaultPorts is empty")
	}

	for _, svc := range defaultPorts {
		if svc.name == "" {
			t.Errorf("defaultPorts entry with empty name: %+v", svc)
		}
		if len(svc.ports) == 0 {
			t.Errorf("service %q has no ports", svc.name)
		}
		for _, p := range svc.ports {
			if p < 1 || p > 65535 {
				t.Errorf("service %q has out-of-range port %d", svc.name, p)
			}
		}
	}
}
