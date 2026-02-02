package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	tests := []struct {
		args []string
	}{
		{[]string{"help"}},
		{[]string{"--help"}},
		{[]string{"-h"}},
		{[]string{}}, // No args should also show help
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			cli := NewCLI(tt.args, "test-version")
			cli.WithOutput(stdout, stderr)

			code := cli.Execute()

			if code != ExitSuccess {
				t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
			}

			output := stdout.String()
			if !strings.Contains(output, "portsql") {
				t.Errorf("expected help output to contain 'portsql', got %q", output)
			}
			if !strings.Contains(output, "migrate") {
				t.Errorf("expected help output to contain 'migrate', got %q", output)
			}
			if !strings.Contains(output, "compile") {
				t.Errorf("expected help output to contain 'compile', got %q", output)
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	tests := []struct {
		args []string
	}{
		{[]string{"version"}},
		{[]string{"--version"}},
		{[]string{"-v"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			cli := NewCLI(tt.args, "1.2.3")
			cli.WithOutput(stdout, stderr)

			code := cli.Execute()

			if code != ExitSuccess {
				t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
			}

			output := stdout.String()
			if !strings.Contains(output, "1.2.3") {
				t.Errorf("expected version output to contain '1.2.3', got %q", output)
			}
		})
	}
}

func TestRunMigrateNewWithTempDir(t *testing.T) {
	// Create a temp directory to work in
	tmpDir := t.TempDir()

	// Change to temp directory for this test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create shipq.ini (required)
	if err := os.WriteFile("shipq.ini", []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"migrate", "new", "create_users"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	if code != ExitSuccess {
		t.Errorf("expected exit code %d, got %d; stderr: %s", ExitSuccess, code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Created:") {
		t.Errorf("expected output to contain 'Created:', got %q", output)
	}
	if !strings.Contains(output, "create_users") {
		t.Errorf("expected output to contain 'create_users', got %q", output)
	}

	// Verify the file was actually created
	entries, err := os.ReadDir(filepath.Join(tmpDir, "migrations"))
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 migration file, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Name(), "create_users") {
		t.Errorf("expected migration file to contain 'create_users', got %q", entries[0].Name())
	}
}

func TestRunMigrateNewNoName(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"migrate", "new"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "requires a migration name") {
		t.Errorf("expected error about missing name, got %q", errOutput)
	}
}

func TestRunMigrateUpNoDatabase(t *testing.T) {
	// Create a temp directory to work in
	tmpDir := t.TempDir()

	// Change to temp directory for this test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create shipq.ini (required)
	if err := os.WriteFile("shipq.ini", []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Clear DATABASE_URL if set
	originalURL := os.Getenv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	defer func() {
		if originalURL != "" {
			os.Setenv("DATABASE_URL", originalURL)
		}
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"migrate", "up"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	// Should fail because no database is configured
	if code != ExitError && code != ExitConfig {
		t.Errorf("expected exit code %d or %d, got %d", ExitError, ExitConfig, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "database URL") {
		t.Errorf("expected error about database URL, got %q", errOutput)
	}
}

func TestRunMigrateResetRefusesRemote(t *testing.T) {
	// Create a temp directory to work in
	tmpDir := t.TempDir()

	// Change to temp directory for this test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create shipq.ini (required)
	if err := os.WriteFile("shipq.ini", []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Set a remote DATABASE_URL
	originalURL := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://db.example.com/mydb")
	defer func() {
		if originalURL != "" {
			os.Setenv("DATABASE_URL", originalURL)
		} else {
			os.Unsetenv("DATABASE_URL")
		}
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"migrate", "reset"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	// Should fail because it's not localhost
	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "not allowed on remote") {
		t.Errorf("expected error about remote databases, got %q", errOutput)
	}
}

func TestRunCompileNoDatabase(t *testing.T) {
	// Create a temp directory to work in
	tmpDir := t.TempDir()

	// Change to temp directory for this test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create shipq.ini (required)
	if err := os.WriteFile("shipq.ini", []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Clear DATABASE_URL if set
	originalURL := os.Getenv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	defer func() {
		if originalURL != "" {
			os.Setenv("DATABASE_URL", originalURL)
		}
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"compile"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	// Should fail because no database is configured
	if code != ExitError && code != ExitConfig {
		t.Errorf("expected exit code %d or %d, got %d", ExitError, ExitConfig, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "database URL") {
		t.Errorf("expected error about database URL, got %q", errOutput)
	}
}

func TestRunCompileNoQueries(t *testing.T) {
	// Create a temp directory to work in
	tmpDir := t.TempDir()

	// Change to temp directory for this test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create shipq.ini (required)
	if err := os.WriteFile("shipq.ini", []byte("[db]\n"), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Set DATABASE_URL
	originalURL := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "sqlite:./test.db")
	defer func() {
		if originalURL != "" {
			os.Setenv("DATABASE_URL", originalURL)
		} else {
			os.Unsetenv("DATABASE_URL")
		}
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"compile"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	// Should fail because querydef directory doesn't exist
	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "queries directory") {
		t.Errorf("expected error about queries directory, got %q", errOutput)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"unknown"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "unknown command") {
		t.Errorf("expected error about unknown command, got %q", errOutput)
	}
}

func TestRunMigrateNoSubcommand(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"migrate"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "requires a subcommand") {
		t.Errorf("expected error about missing subcommand, got %q", errOutput)
	}
}

func TestRunMigrateUnknownSubcommand(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cli := NewCLI([]string{"migrate", "unknown"}, "test")
	cli.WithOutput(stdout, stderr)

	code := cli.Execute()

	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "unknown migrate subcommand") {
		t.Errorf("expected error about unknown subcommand, got %q", errOutput)
	}
}

func TestRunMigrateHelp(t *testing.T) {
	tests := []struct {
		args []string
	}{
		{[]string{"migrate", "help"}},
		{[]string{"migrate", "--help"}},
		{[]string{"migrate", "-h"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			cli := NewCLI(tt.args, "test")
			cli.WithOutput(stdout, stderr)

			code := cli.Execute()

			if code != ExitSuccess {
				t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
			}

			output := stdout.String()
			if !strings.Contains(output, "new <name>") {
				t.Errorf("expected migrate help to contain 'new <name>', got %q", output)
			}
			if !strings.Contains(output, "up") {
				t.Errorf("expected migrate help to contain 'up', got %q", output)
			}
			if !strings.Contains(output, "reset") {
				t.Errorf("expected migrate help to contain 'reset', got %q", output)
			}
		})
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantRemaining []string
		wantFlags     map[string]string
	}{
		{
			name:          "no flags",
			args:          []string{"arg1", "arg2"},
			wantRemaining: []string{"arg1", "arg2"},
			wantFlags:     map[string]string{},
		},
		{
			name:          "long flag with value",
			args:          []string{"--config", "path/to/config"},
			wantRemaining: []string{},
			wantFlags:     map[string]string{"config": "path/to/config"},
		},
		{
			name:          "long flag with equals",
			args:          []string{"--config=path/to/config"},
			wantRemaining: []string{},
			wantFlags:     map[string]string{"config": "path/to/config"},
		},
		{
			name:          "short flag with value",
			args:          []string{"-c", "path/to/config"},
			wantRemaining: []string{},
			wantFlags:     map[string]string{"c": "path/to/config"},
		},
		{
			name:          "boolean flag",
			args:          []string{"--verbose"},
			wantRemaining: []string{},
			wantFlags:     map[string]string{"verbose": "true"},
		},
		{
			name:          "mixed args and flags",
			args:          []string{"cmd", "--flag", "value", "arg"},
			wantRemaining: []string{"cmd", "arg"},
			wantFlags:     map[string]string{"flag": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining, flags := parseFlags(tt.args)

			if len(remaining) != len(tt.wantRemaining) {
				t.Errorf("remaining: got %v, want %v", remaining, tt.wantRemaining)
			} else {
				for i, arg := range remaining {
					if arg != tt.wantRemaining[i] {
						t.Errorf("remaining[%d]: got %q, want %q", i, arg, tt.wantRemaining[i])
					}
				}
			}

			if len(flags) != len(tt.wantFlags) {
				t.Errorf("flags: got %v, want %v", flags, tt.wantFlags)
			} else {
				for k, v := range tt.wantFlags {
					if flags[k] != v {
						t.Errorf("flags[%q]: got %q, want %q", k, flags[k], v)
					}
				}
			}
		})
	}
}
