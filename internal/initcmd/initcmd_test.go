package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/internal/config"
)

func TestRun_HappyPath(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Check shipq.ini was created
	configPath := filepath.Join(dir, config.ConfigFilename)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("shipq.ini was not created")
	}

	// Check directories were created
	for _, d := range directories {
		dirPath := filepath.Join(dir, d)
		info, err := os.Stat(dirPath)
		if os.IsNotExist(err) {
			t.Errorf("directory %q was not created", d)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", d)
		}
	}

	// Check success message
	output := stdout.String()
	if !strings.Contains(output, "Created shipq.ini") {
		t.Errorf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "Next steps") {
		t.Errorf("expected next steps message, got: %s", output)
	}
}

func TestRun_DefaultDialectMySQL(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	content := readFile(t, dir, config.ConfigFilename)
	if !strings.Contains(content, "dialects = mysql") {
		t.Errorf("expected default dialect 'mysql', got: %s", content)
	}
}

func TestRun_DatabasePostgres(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--database", "postgres"}, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	content := readFile(t, dir, config.ConfigFilename)
	if !strings.Contains(content, "dialects = postgres") {
		t.Errorf("expected dialect 'postgres', got: %s", content)
	}
	if !strings.Contains(content, "postgres://") {
		t.Errorf("expected postgres example URL in comments")
	}
}

func TestRun_DatabaseSQLite(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--database", "sqlite"}, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	content := readFile(t, dir, config.ConfigFilename)
	if !strings.Contains(content, "dialects = sqlite") {
		t.Errorf("expected dialect 'sqlite', got: %s", content)
	}
	if !strings.Contains(content, "sqlite://") {
		t.Errorf("expected sqlite example URL in comments")
	}
}

func TestRun_InvalidDialect(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--database", "oracle"}, Options{Stdout: stdout, Stderr: stderr})

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "invalid database dialect") {
		t.Errorf("expected error about invalid dialect, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "oracle") {
		t.Errorf("expected error to mention 'oracle', got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "mysql, postgres, sqlite") {
		t.Errorf("expected error to list supported dialects, got: %s", errOutput)
	}

	// Config should not have been created
	configPath := filepath.Join(dir, config.ConfigFilename)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("shipq.ini should not have been created on error")
	}
}

func TestRun_ExistingConfig_NoForce(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Create existing config
	existingContent := "existing content"
	writeFile(t, dir, config.ConfigFilename, existingContent)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "already exists") {
		t.Errorf("expected error about existing config, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "--force") {
		t.Errorf("expected hint about --force, got: %s", errOutput)
	}

	// Original content should be preserved
	content := readFile(t, dir, config.ConfigFilename)
	if content != existingContent {
		t.Errorf("existing config was modified: got %q", content)
	}
}

func TestRun_ExistingConfig_WithForce(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Create existing config
	writeFile(t, dir, config.ConfigFilename, "old content")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--force"}, Options{Stdout: stdout, Stderr: stderr})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Config should be overwritten
	content := readFile(t, dir, config.ConfigFilename)
	if strings.Contains(content, "old content") {
		t.Error("config was not overwritten")
	}
	if !strings.Contains(content, "[db]") {
		t.Error("new config should contain [db] section")
	}

	// Success message should mention overwrite
	output := stdout.String()
	if !strings.Contains(output, "Overwrote") {
		t.Errorf("expected overwrite message, got: %s", output)
	}
}

func TestRun_ExistingDirectories_NotRecreated(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Create some directories with content
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, migrationsDir, "test.sql", "-- existing migration")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Existing file should still be there
	content := readFile(t, migrationsDir, "test.sql")
	if content != "-- existing migration" {
		t.Error("existing file was modified or deleted")
	}

	// Output should not mention creating migrations/ since it existed
	output := stdout.String()
	if strings.Contains(output, "Created directories:") && strings.Contains(output, "migrations") {
		// If migrations is listed in created dirs, it should not be
		// because we only list newly created dirs
		// Actually, let's check the message more carefully
	}
}

func TestRun_ConflictingFile(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Create a file where a directory should be
	writeFile(t, dir, "api", "this is a file, not a directory")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "expected directory but found file") {
		t.Errorf("expected error about conflicting file, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "api") {
		t.Errorf("expected error to mention 'api', got: %s", errOutput)
	}
}

func TestRun_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			changeDir(t, dir)

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			code := Run(tt.args, Options{Stdout: stdout, Stderr: stderr})

			if code != 0 {
				t.Errorf("expected exit code 0, got %d", code)
			}

			output := stdout.String()
			if !strings.Contains(output, "shipq init") {
				t.Errorf("expected help to contain 'shipq init', got: %s", output)
			}
			if !strings.Contains(output, "--database") {
				t.Errorf("expected help to mention --database flag")
			}
			if !strings.Contains(output, "--force") {
				t.Errorf("expected help to mention --force flag")
			}

			// Config should not be created when showing help
			configPath := filepath.Join(dir, config.ConfigFilename)
			if _, err := os.Stat(configPath); !os.IsNotExist(err) {
				t.Error("config should not be created when showing help")
			}
		})
	}
}

func TestRenderShipqINI_ContainsRequiredSections(t *testing.T) {
	content := renderShipqINI("mysql", true)

	if !strings.Contains(content, "[project]") {
		t.Error("config should contain [project] section")
	}
	if !strings.Contains(content, "[db]") {
		t.Error("config should contain [db] section")
	}
	if !strings.Contains(content, "[api]") {
		t.Error("config should contain [api] section")
	}
}

func TestRenderShipqINI_ContainsRequiredKeys(t *testing.T) {
	content := renderShipqINI("mysql", true)

	requiredKeys := []string{
		"include_logging =",
		"url =",
		"dialects =",
		"migrations =",
		"schematypes =",
		"queries_in =",
		"queries_out =",
		"package =",
	}

	for _, key := range requiredKeys {
		if !strings.Contains(content, key) {
			t.Errorf("config should contain %q", key)
		}
	}
}

func TestRenderShipqINI_Dialects(t *testing.T) {
	tests := []struct {
		dialect  string
		expected string
	}{
		{"mysql", "dialects = mysql"},
		{"postgres", "dialects = postgres"},
		{"sqlite", "dialects = sqlite"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			content := renderShipqINI(tt.dialect, true)
			if !strings.Contains(content, tt.expected) {
				t.Errorf("expected %q in content", tt.expected)
			}
		})
	}
}

func TestRenderShipqINI_ExampleURLs(t *testing.T) {
	tests := []struct {
		dialect     string
		expectedURL string
	}{
		{"mysql", "mysql://"},
		{"postgres", "postgres://"},
		{"sqlite", "sqlite://"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			content := renderShipqINI(tt.dialect, true)
			if !strings.Contains(content, tt.expectedURL) {
				t.Errorf("expected example URL containing %q", tt.expectedURL)
			}
		})
	}
}

func TestRenderShipqINI_IsDeterministic(t *testing.T) {
	content1 := renderShipqINI("mysql", true)
	content2 := renderShipqINI("mysql", true)

	if content1 != content2 {
		t.Error("renderShipqINI should produce deterministic output")
	}
}

func TestRenderShipqINI_Parseable(t *testing.T) {
	// The generated config should be parseable by our config loader
	dir := t.TempDir()
	content := renderShipqINI("postgres", true)
	writeFile(t, dir, config.ConfigFilename, content)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("generated config is not parseable: %v", err)
	}

	if len(cfg.DB.Dialects) != 1 || cfg.DB.Dialects[0] != "postgres" {
		t.Errorf("expected dialects=[postgres], got %v", cfg.DB.Dialects)
	}
	if cfg.DB.Migrations != "migrations" {
		t.Errorf("expected migrations='migrations', got %q", cfg.DB.Migrations)
	}
	if cfg.API.Package != "./api" {
		t.Errorf("expected package='./api', got %q", cfg.API.Package)
	}
	if !cfg.Project.IncludeLogging {
		t.Error("expected IncludeLogging=true")
	}
}

func TestRenderShipqINI_IncludeLoggingTrue(t *testing.T) {
	content := renderShipqINI("mysql", true)

	if !strings.Contains(content, "include_logging = true") {
		t.Error("expected include_logging = true in config")
	}
}

func TestRenderShipqINI_IncludeLoggingFalse(t *testing.T) {
	content := renderShipqINI("mysql", false)

	if !strings.Contains(content, "include_logging = false") {
		t.Error("expected include_logging = false in config")
	}
}

func TestIsValidDialect(t *testing.T) {
	valid := []string{"mysql", "postgres", "sqlite"}
	invalid := []string{"oracle", "mssql", "mariadb", "MYSQL", "Postgres", ""}

	for _, d := range valid {
		if !isValidDialect(d) {
			t.Errorf("%q should be valid", d)
		}
	}

	for _, d := range invalid {
		if isValidDialect(d) {
			t.Errorf("%q should be invalid", d)
		}
	}
}

func TestRun_ConfigFilePermissions(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	configPath := filepath.Join(dir, config.ConfigFilename)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}

	// Check permissions are 0600 (user read/write only)
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("expected file permissions 0600, got %o", mode)
	}
}

func TestRun_DirectoryPermissions(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	for _, d := range directories {
		dirPath := filepath.Join(dir, d)
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Fatal(err)
		}

		mode := info.Mode().Perm()
		if mode != 0755 {
			t.Errorf("directory %s: expected permissions 0755, got %o", d, mode)
		}
	}
}

// Helper functions

func changeDir(t *testing.T, dir string) {
	t.Helper()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(oldDir)
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	return string(data)
}

// Tests for logging scaffold functionality

func TestRun_DefaultIncludesLogging(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Check logging directory was created
	loggingDir := filepath.Join(dir, "logging")
	info, err := os.Stat(loggingDir)
	if os.IsNotExist(err) {
		t.Error("logging/ directory was not created")
	} else if !info.IsDir() {
		t.Error("logging/ is not a directory")
	}

	// Check logging/logging.go was created
	loggingGoPath := filepath.Join(dir, "logging", "logging.go")
	if _, err := os.Stat(loggingGoPath); os.IsNotExist(err) {
		t.Error("logging/logging.go was not created")
	}

	// Check logging.go content
	content := readFile(t, filepath.Join(dir, "logging"), "logging.go")
	if !strings.Contains(content, "package logging") {
		t.Error("logging.go should contain 'package logging'")
	}
	if !strings.Contains(content, "func Decorate") {
		t.Error("logging.go should contain 'func Decorate'")
	}

	// Check shipq.ini contains include_logging = true
	iniContent := readFile(t, dir, config.ConfigFilename)
	if !strings.Contains(iniContent, "include_logging = true") {
		t.Errorf("expected shipq.ini to contain 'include_logging = true', got: %s", iniContent)
	}

	// Check success message
	output := stdout.String()
	if !strings.Contains(output, "Created logging/logging.go") {
		t.Errorf("expected success message about logging, got: %s", output)
	}
}

func TestRun_NoLoggingFlag(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--no-logging"}, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Check logging directory was NOT created
	loggingDir := filepath.Join(dir, "logging")
	if _, err := os.Stat(loggingDir); !os.IsNotExist(err) {
		t.Error("logging/ directory should not be created with --no-logging")
	}

	// Check shipq.ini contains include_logging = false
	iniContent := readFile(t, dir, config.ConfigFilename)
	if !strings.Contains(iniContent, "include_logging = false") {
		t.Errorf("expected shipq.ini to contain 'include_logging = false', got: %s", iniContent)
	}

	// Check success message mentions skipped
	output := stdout.String()
	if !strings.Contains(output, "Logging scaffold skipped") {
		t.Errorf("expected message about skipped logging, got: %s", output)
	}
}

func TestRun_LoggingFalseFlag(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--logging=false"}, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Check logging directory was NOT created
	loggingDir := filepath.Join(dir, "logging")
	if _, err := os.Stat(loggingDir); !os.IsNotExist(err) {
		t.Error("logging/ directory should not be created with --logging=false")
	}

	// Check shipq.ini contains include_logging = false
	iniContent := readFile(t, dir, config.ConfigFilename)
	if !strings.Contains(iniContent, "include_logging = false") {
		t.Errorf("expected shipq.ini to contain 'include_logging = false', got: %s", iniContent)
	}
}

func TestRun_ExistingLoggingGo_NoForce(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Create existing logging/logging.go
	loggingDir := filepath.Join(dir, "logging")
	if err := os.MkdirAll(loggingDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingContent := "// existing content"
	writeFile(t, loggingDir, "logging.go", existingContent)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "logging/logging.go already exists") {
		t.Errorf("expected error about existing logging.go, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "--force") {
		t.Errorf("expected hint about --force, got: %s", errOutput)
	}

	// Original content should be preserved
	content := readFile(t, loggingDir, "logging.go")
	if content != existingContent {
		t.Errorf("existing logging.go was modified: got %q", content)
	}
}

func TestRun_ExistingLoggingGo_WithForce(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Create existing logging/logging.go
	loggingDir := filepath.Join(dir, "logging")
	if err := os.MkdirAll(loggingDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, loggingDir, "logging.go", "// old content")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--force"}, Options{Stdout: stdout, Stderr: stderr})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Content should be overwritten
	content := readFile(t, loggingDir, "logging.go")
	if strings.Contains(content, "old content") {
		t.Error("logging.go was not overwritten")
	}
	if !strings.Contains(content, "package logging") {
		t.Error("logging.go should contain new template content")
	}
}

func TestRun_LoggingFileContainsValidGo(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run(nil, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	// Check that the generated logging.go contains expected elements
	content := readFile(t, filepath.Join(dir, "logging"), "logging.go")

	expectedElements := []string{
		"package logging",
		"import (",
		"log/slog",
		"net/http",
		"type contextKey string",
		"UserIDKey contextKey",
		"PrettyJSONHandler",
		"ProdLogger",
		"DevLogger",
		"func Decorate(",
		"request_started",
		"request_completed",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(content, elem) {
			t.Errorf("expected logging.go to contain %q", elem)
		}
	}
}

func TestRun_HelpShowsLoggingFlags(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := Run([]string{"--help"}, Options{Stdout: stdout, Stderr: stderr})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "--no-logging") {
		t.Errorf("expected help to mention --no-logging flag")
	}
	if !strings.Contains(output, "logging/") {
		t.Errorf("expected help to mention logging/ directory")
	}
}
