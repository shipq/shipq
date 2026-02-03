package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/config"
)

func TestRun_NoArgs(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput(nil, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq") {
		t.Errorf("expected help to contain 'shipq', got %q", output)
	}
	if !strings.Contains(output, "init") {
		t.Errorf("expected help to contain 'init', got %q", output)
	}
	if !strings.Contains(output, "db") {
		t.Errorf("expected help to contain 'db', got %q", output)
	}
	if !strings.Contains(output, "api") {
		t.Errorf("expected help to contain 'api', got %q", output)
	}
}

func TestRun_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help", []string{"help"}},
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			code := runWithOutput(tt.args, stdout, stderr)

			if code != 0 {
				t.Errorf("expected exit code 0, got %d", code)
			}

			output := stdout.String()
			if !strings.Contains(output, "shipq") {
				t.Errorf("expected help to contain 'shipq', got %q", output)
			}
		})
	}
}

func TestRun_Version(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"version", []string{"version"}},
		{"--version", []string{"--version"}},
		{"-v", []string{"-v"}},
	}

	// Set a known version for testing
	oldVersion := Version
	Version = "1.2.3-test"
	defer func() { Version = oldVersion }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			code := runWithOutput(tt.args, stdout, stderr)

			if code != 0 {
				t.Errorf("expected exit code 0, got %d", code)
			}

			output := stdout.String()
			if !strings.Contains(output, "shipq version") {
				t.Errorf("expected version output to contain 'shipq version', got %q", output)
			}
			if !strings.Contains(output, "1.2.3-test") {
				t.Errorf("expected version output to contain '1.2.3-test', got %q", output)
			}
		})
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"unknown"}, stdout, stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "unknown command") {
		t.Errorf("expected error to mention 'unknown command', got %q", errOutput)
	}
	if !strings.Contains(errOutput, "unknown") {
		t.Errorf("expected error to contain the command name 'unknown', got %q", errOutput)
	}
}

func TestRun_DB_Help(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Note: 'shipq db --help' should pass through to shipq db CLI
	// which will print its own help
	code := runWithOutput([]string{"db", "--help"}, stdout, stderr)

	// PortSQL's --help returns 0
	if code != 0 {
		t.Errorf("expected exit code 0 for db --help, got %d", code)
	}

	// The output should contain shipq db help content
	output := stdout.String()
	if !strings.Contains(output, "shipq db") {
		t.Errorf("expected db --help to show shipq db help, got %q", output)
	}
	if !strings.Contains(output, "migrate") {
		t.Errorf("expected shipq db help to contain 'migrate', got %q", output)
	}
}

func TestRun_DB_NoArgs(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// 'shipq db' with no args should show shipq db help
	code := runWithOutput([]string{"db"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for db with no args, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq db") {
		t.Errorf("expected db with no args to show shipq db help, got %q", output)
	}
}

func TestRun_Init_HappyPath(t *testing.T) {
	// Change to a temp directory for this test
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"init"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Created shipq.ini") {
		t.Errorf("expected success message, got %q", output)
	}
}

func TestRun_Init_Help(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"init", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for init --help, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq init") {
		t.Errorf("expected help output to contain 'shipq init', got %q", output)
	}
	if !strings.Contains(output, "--database") {
		t.Errorf("expected help output to mention --database flag, got %q", output)
	}
}

func TestRun_API_Help(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"api", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for api --help, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq api") {
		t.Errorf("expected api --help to show shipq api help, got %q", output)
	}
}

func TestRun_API_Version(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Set a known version for testing
	oldVersion := Version
	Version = "test-api-version"
	defer func() { Version = oldVersion }()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"api", "version"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for api version, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq api version") {
		t.Errorf("expected output to contain 'shipq api version', got %q", output)
	}
	if !strings.Contains(output, "test-api-version") {
		t.Errorf("expected output to contain version string, got %q", output)
	}
}

func TestRun_API_ResourceHelp(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"api", "resource", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for api resource --help, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq api resource") {
		t.Errorf("expected api resource --help to show resource help, got %q", output)
	}
	if !strings.Contains(output, "table") {
		t.Errorf("expected api resource help to mention 'table', got %q", output)
	}
}

func TestRun_API_ResourceNoArgs(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"api", "resource"}, stdout, stderr)

	// Should fail because no table name provided
	if code == 0 {
		t.Errorf("expected non-zero exit code for api resource without table name")
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "resource requires a table name") {
		t.Errorf("expected error about table name, got %q", errOutput)
	}
}

func TestRun_DB_PassesArguments(t *testing.T) {
	// Create a project directory for this test
	dir := t.TempDir()
	writeConfigFile(t, dir)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Test that arguments after 'db' are passed through correctly
	// We use 'version' because it doesn't require any config files
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"db", "version"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq db version") {
		t.Errorf("expected output to contain 'shipq db version', got %q", output)
	}
}

func TestPrintHelp_ContainsAllCommands(t *testing.T) {
	buf := &bytes.Buffer{}
	printHelp(buf)

	output := buf.String()

	commands := []string{"init", "db", "api", "help", "version"}
	for _, cmd := range commands {
		if !strings.Contains(output, cmd) {
			t.Errorf("expected help to contain command %q", cmd)
		}
	}
}

func TestPrintHelp_ContainsExamples(t *testing.T) {
	buf := &bytes.Buffer{}
	printHelp(buf)

	output := buf.String()

	if !strings.Contains(output, "Examples:") {
		t.Error("expected help to contain 'Examples:'")
	}
	if !strings.Contains(output, "shipq db migrate") {
		t.Error("expected help to contain 'shipq db migrate' example")
	}
}

func TestPrintVersion(t *testing.T) {
	oldVersion := Version
	Version = "test-version"
	defer func() { Version = oldVersion }()

	buf := &bytes.Buffer{}
	printVersion(buf)

	output := buf.String()
	if !strings.Contains(output, "shipq version test-version") {
		t.Errorf("expected 'shipq version test-version', got %q", output)
	}
}

func TestRun_ProjectFlag(t *testing.T) {
	// Create a project in a temp directory
	projectDir := t.TempDir()
	writeConfigFile(t, projectDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run from a different directory but specify --project
	code := runWithOutput([]string{"--project", projectDir, "db", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}
}

func TestRun_ProjectFlagWithEquals(t *testing.T) {
	// Create a project in a temp directory
	projectDir := t.TempDir()
	writeConfigFile(t, projectDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Test --project=path syntax
	code := runWithOutput([]string{"--project=" + projectDir, "db", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}
}

func TestRun_ProjectFlag_MissingValue(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"--project"}, stdout, stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "--project requires") {
		t.Errorf("expected error about missing value, got: %s", errOutput)
	}
}

func TestRun_ProjectFlag_InvalidPath(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"--project", "/nonexistent/path", "db", "--help"}, stdout, stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "does not exist") {
		t.Errorf("expected error about nonexistent path, got: %s", errOutput)
	}
}

func TestRun_ProjectFlag_NoConfig(t *testing.T) {
	// Create a temp directory without shipq.ini
	emptyDir := t.TempDir()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"--project", emptyDir, "db", "--help"}, stdout, stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, config.ConfigFilename) {
		t.Errorf("expected error to mention %s, got: %s", config.ConfigFilename, errOutput)
	}
}

func TestRun_DB_FromSubdirectory(t *testing.T) {
	// Create a project with a subdirectory
	projectDir := t.TempDir()
	writeConfigFile(t, projectDir)

	subDir := filepath.Join(projectDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to subdirectory
	oldDir, _ := os.Getwd()
	os.Chdir(subDir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Should find project root automatically
	code := runWithOutput([]string{"db", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq db") {
		t.Errorf("expected shipq db help output, got: %s", output)
	}
}

func TestRun_API_FromSubdirectory(t *testing.T) {
	// Create a project with a subdirectory
	projectDir := t.TempDir()
	writeConfigFile(t, projectDir)

	subDir := filepath.Join(projectDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to subdirectory
	oldDir, _ := os.Getwd()
	os.Chdir(subDir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Should find project root automatically
	code := runWithOutput([]string{"api", "--help"}, stdout, stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "shipq api") {
		t.Errorf("expected shipq api help output, got: %s", output)
	}
}

func TestRun_NoProjectFound(t *testing.T) {
	// Run from a temp directory with no shipq.ini
	emptyDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(emptyDir)
	defer os.Chdir(oldDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := runWithOutput([]string{"db", "--help"}, stdout, stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "shipq.ini") {
		t.Errorf("expected error to mention shipq.ini, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "--project") {
		t.Errorf("expected hint about --project, got: %s", errOutput)
	}
}

func TestPrintHelp_ContainsProjectFlag(t *testing.T) {
	buf := &bytes.Buffer{}
	printHelp(buf)

	output := buf.String()
	if !strings.Contains(output, "--project") {
		t.Error("expected help to mention --project flag")
	}
}

// Helper function to create a minimal shipq.ini
func writeConfigFile(t *testing.T, dir string) {
	t.Helper()
	content := `[db]
dialects = mysql
migrations = migrations
schematypes = schematypes
queries_in = querydef
queries_out = queries

[api]
package = ./api
`
	path := filepath.Join(dir, config.ConfigFilename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
}
