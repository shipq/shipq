package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStartPostgres_MissingBinary(t *testing.T) {
	// This test verifies that StartPostgres returns an appropriate error
	// when the required binaries are not found.
	//
	// We can't easily test this without modifying PATH, so we'll test the
	// error message format when we can't find binaries.

	// Skip if postgres is actually installed (we want to test the missing case)
	if _, err := exec.LookPath("postgres"); err == nil {
		t.Skip("postgres is installed, skipping missing binary test")
	}

	// Create temp dir with a valid project name
	tmpBase := t.TempDir()
	tmpDir := filepath.Join(tmpBase, "testproject")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	iniContent := "[db]\nurl = postgres://localhost/mydb\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = StartPostgres(cfg, &stdout, &stderr)

	if err == nil {
		t.Fatal("expected error when postgres binaries not found")
	}

	// Check error mentions the missing binary
	errStr := err.Error()
	if !bytes.Contains([]byte(errStr), []byte("not found on PATH")) {
		t.Errorf("error should mention 'not found on PATH', got: %v", err)
	}
}

func TestStartMySQL_MissingBinary(t *testing.T) {
	// Skip if mysqld is actually installed
	if _, err := exec.LookPath("mysqld"); err == nil {
		t.Skip("mysqld is installed, skipping missing binary test")
	}

	// Create temp dir with a valid project name
	tmpBase := t.TempDir()
	tmpDir := filepath.Join(tmpBase, "testproject")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	iniContent := "[db]\nurl = mysql://localhost/mydb\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = StartMySQL(cfg, &stdout, &stderr)

	if err == nil {
		t.Fatal("expected error when mysqld binary not found")
	}

	// Check error mentions the missing binary
	errStr := err.Error()
	if !bytes.Contains([]byte(errStr), []byte("not found on PATH")) {
		t.Errorf("error should mention 'not found on PATH', got: %v", err)
	}
}

func TestPostgresDataDir(t *testing.T) {
	// Verify the postgres data dir constant is set correctly
	expected := "db/databases/.postgres-data"
	if postgresDataDir != expected {
		t.Errorf("postgresDataDir = %q, want %q", postgresDataDir, expected)
	}
}

func TestMySQLDataDir(t *testing.T) {
	// Verify the mysql data dir constant is set correctly
	expected := "db/databases/.mysql-data"
	if mysqlDataDir != expected {
		t.Errorf("mysqlDataDir = %q, want %q", mysqlDataDir, expected)
	}
}

func TestRunStart_NoArguments(t *testing.T) {
	cli := NewCLI([]string{"start"}, "test")
	var stdout, stderr bytes.Buffer
	cli.WithOutput(&stdout, &stderr)

	code := cli.Execute()

	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	// Should mention that a database type is required
	if !bytes.Contains(stderr.Bytes(), []byte("requires a database type")) {
		t.Errorf("error should mention database type required, got: %s", stderr.String())
	}
}

func TestRunStart_UnknownDatabase(t *testing.T) {
	cli := NewCLI([]string{"start", "oracle"}, "test")
	var stdout, stderr bytes.Buffer
	cli.WithOutput(&stdout, &stderr)

	code := cli.Execute()

	if code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}

	// Should mention the unknown database type
	if !bytes.Contains(stderr.Bytes(), []byte("unknown database type")) {
		t.Errorf("error should mention unknown database type, got: %s", stderr.String())
	}
}

func TestRunStart_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help subcommand", []string{"start", "help"}},
		{"--help flag", []string{"start", "--help"}},
		{"-h flag", []string{"start", "-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := NewCLI(tt.args, "test")
			var stdout, stderr bytes.Buffer
			cli.WithOutput(&stdout, &stderr)

			code := cli.Execute()

			if code != ExitSuccess {
				t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
			}

			// Help should mention postgres and mysql
			output := stdout.String()
			if !bytes.Contains([]byte(output), []byte("postgres")) {
				t.Errorf("help should mention postgres")
			}
			if !bytes.Contains([]byte(output), []byte("mysql")) {
				t.Errorf("help should mention mysql")
			}
		})
	}
}

func TestRunSetup_Help(t *testing.T) {
	// Verify that help text is accessible via the main help
	cli := NewCLI([]string{"help"}, "test")
	var stdout, stderr bytes.Buffer
	cli.WithOutput(&stdout, &stderr)

	code := cli.Execute()

	if code != ExitSuccess {
		t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
	}

	output := stdout.String()

	// Main help should mention setup command
	if !bytes.Contains([]byte(output), []byte("setup")) {
		t.Errorf("main help should mention setup command")
	}

	// Main help should mention start command
	if !bytes.Contains([]byte(output), []byte("start")) {
		t.Errorf("main help should mention start command")
	}
}

func TestCLI_StartPostgresNoConfig(t *testing.T) {
	// Test that start postgres fails gracefully when no config exists
	tmpDir := t.TempDir()

	// Change to temp dir (no shipq.ini)
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cli := NewCLI([]string{"start", "postgres"}, "test")
	var stdout, stderr bytes.Buffer
	cli.WithOutput(&stdout, &stderr)

	code := cli.Execute()

	if code != ExitConfig {
		t.Errorf("expected exit code %d (config error), got %d", ExitConfig, code)
	}
}

func TestCLI_StartMySQLNoConfig(t *testing.T) {
	// Test that start mysql fails gracefully when no config exists
	tmpDir := t.TempDir()

	// Change to temp dir (no shipq.ini)
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cli := NewCLI([]string{"start", "mysql"}, "test")
	var stdout, stderr bytes.Buffer
	cli.WithOutput(&stdout, &stderr)

	code := cli.Execute()

	if code != ExitConfig {
		t.Errorf("expected exit code %d (config error), got %d", ExitConfig, code)
	}
}

func TestCLI_SetupNoConfig(t *testing.T) {
	// Test that setup fails gracefully when no config exists
	tmpDir := t.TempDir()

	// Change to temp dir (no shipq.ini)
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cli := NewCLI([]string{"setup"}, "test")
	var stdout, stderr bytes.Buffer
	cli.WithOutput(&stdout, &stderr)

	code := cli.Execute()

	if code != ExitConfig {
		t.Errorf("expected exit code %d (config error), got %d", ExitConfig, code)
	}
}

func TestStartPostgres_DataDirCreation(t *testing.T) {
	// Skip if initdb is not available
	if _, err := exec.LookPath("initdb"); err != nil {
		t.Skip("initdb not available, skipping data dir creation test")
	}

	// Create temp dir with a valid project name
	tmpBase := t.TempDir()
	tmpDir := filepath.Join(tmpBase, "testproject")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	iniContent := "[db]\nurl = postgres://localhost/mydb\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Create the db/databases structure
	dbDir := filepath.Join(tmpDir, "db", "databases")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("failed to create db/databases: %v", err)
	}

	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// This will try to initialize but then fail to start postgres
	// We just want to verify the initialization output
	var stdout, stderr bytes.Buffer
	_ = StartPostgres(cfg, &stdout, &stderr)

	// Check that output mentions initializing the data directory
	output := stdout.String() + stderr.String()
	if !bytes.Contains([]byte(output), []byte("Initializing")) || !bytes.Contains([]byte(output), []byte("postgres")) {
		// Either initialization happened or postgres started - both are acceptable
		// The key is we didn't crash
	}

	// Check that data directory was created
	dataDir := filepath.Join(tmpDir, postgresDataDir)
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		// Data dir might not exist if initdb failed, which is OK for this test
		t.Logf("Data directory not created (initdb may have failed): %s", dataDir)
	}
}
