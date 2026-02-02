package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSetup_NonLocalhostURLRejected(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "remote postgres host",
			url:  "postgres://db.example.com/mydb",
		},
		{
			name: "remote mysql host",
			url:  "mysql://db.example.com/mydb",
		},
		{
			name: "private IP 192.168.x.x",
			url:  "postgres://192.168.1.100/mydb",
		},
		{
			name: "private IP 10.x.x.x",
			url:  "postgres://10.0.0.50/mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir with shipq.ini
			tmpDir := t.TempDir()
			iniContent := "[db]\nurl = " + tt.url + "\n"
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
			err = Setup(cfg, &stdout, &stderr)

			if err == nil {
				t.Error("expected error for non-localhost URL, got nil")
			}

			// Check error message mentions localhost
			if err != nil && !bytes.Contains([]byte(err.Error()), []byte("localhost")) {
				t.Errorf("error should mention localhost, got: %v", err)
			}
		})
	}
}

func TestSetup_LocalhostURLsAccepted(t *testing.T) {
	// These tests verify that localhost URLs pass the safety check
	// They will fail to connect (no server running) but that's expected
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "localhost postgres",
			url:  "postgres://localhost/mydb",
		},
		{
			name: "127.0.0.1 postgres",
			url:  "postgres://127.0.0.1/mydb",
		},
		{
			name: "localhost mysql",
			url:  "mysql://localhost/mydb",
		},
		{
			name: "sqlite always local",
			url:  "sqlite://test.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir with a valid project name (not starting with number)
			tmpBase := t.TempDir()
			tmpDir := filepath.Join(tmpBase, "testproject")
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}
			iniContent := "[db]\nurl = " + tt.url + "\n"
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
			err = Setup(cfg, &stdout, &stderr)

			// For postgres/mysql, we expect a connection error (server not running)
			// For sqlite, we expect success or a different kind of error
			// The key thing is we should NOT get a "not localhost" error

			if err != nil {
				errStr := err.Error()
				if bytes.Contains([]byte(errStr), []byte("only supports localhost")) {
					t.Errorf("localhost URL should pass safety check, got: %v", err)
				}
			}

			// For SQLite, setup should succeed
			if tt.url == "sqlite://test.db" && err != nil {
				t.Errorf("SQLite setup should succeed, got: %v", err)
			}
		})
	}
}

func TestSetup_NoDatabaseURL(t *testing.T) {
	// Create temp dir with shipq.ini but no URL
	tmpBase := t.TempDir()
	tmpDir := filepath.Join(tmpBase, "testproject")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	iniContent := "[db]\nmigrations = migrations\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatalf("failed to write shipq.ini: %v", err)
	}

	// Unset DATABASE_URL if set
	oldEnv := os.Getenv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldEnv != "" {
			os.Setenv("DATABASE_URL", oldEnv)
		}
	}()

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
	err = Setup(cfg, &stdout, &stderr)

	if err == nil {
		t.Error("expected error when no database URL configured")
	}

	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("no database URL")) {
		t.Errorf("error should mention missing database URL, got: %v", err)
	}
}

func TestSetup_SQLiteNoOp(t *testing.T) {
	// Create temp dir with a valid project name
	tmpBase := t.TempDir()
	tmpDir := filepath.Join(tmpBase, "testproject")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	iniContent := "[db]\nurl = sqlite://test.db\n"
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
	err = Setup(cfg, &stdout, &stderr)

	if err != nil {
		t.Errorf("SQLite setup should succeed: %v", err)
	}

	// Check output mentions SQLite databases are created automatically
	if !bytes.Contains(stdout.Bytes(), []byte("SQLite")) {
		t.Error("output should mention SQLite")
	}
}

func TestSetup_DatabaseNamesFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		iniContent  string
		expectDev   string
		expectTest  string
		projectName string
	}{
		{
			name:        "default from project folder",
			iniContent:  "[db]\nurl = sqlite://test.db\n",
			expectDev:   "", // Will be derived from folder name
			expectTest:  "", // Will be derived from folder name
			projectName: "testproject",
		},
		{
			name:        "explicit base name",
			iniContent:  "[db]\nurl = sqlite://test.db\nname = myapp\n",
			expectDev:   "myapp",
			expectTest:  "myapp_test",
			projectName: "ignored",
		},
		{
			name:        "explicit dev name",
			iniContent:  "[db]\nurl = sqlite://test.db\ndev_name = custom_dev\n",
			expectDev:   "custom_dev",
			expectTest:  "", // Will be derived
			projectName: "testproject",
		},
		{
			name:        "explicit test name",
			iniContent:  "[db]\nurl = sqlite://test.db\ntest_name = custom_test\n",
			expectDev:   "", // Will be derived
			expectTest:  "custom_test",
			projectName: "testproject",
		},
		{
			name:        "both explicit",
			iniContent:  "[db]\nurl = sqlite://test.db\ndev_name = mydev\ntest_name = mytest\n",
			expectDev:   "mydev",
			expectTest:  "mytest",
			projectName: "ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir with specific name for project folder derivation
			tmpBase := t.TempDir()
			tmpDir := filepath.Join(tmpBase, tt.projectName)
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(tt.iniContent), 0644); err != nil {
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
			err = Setup(cfg, &stdout, &stderr)

			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			output := stdout.String()

			// Check expected names appear in output
			if tt.expectDev != "" && !bytes.Contains([]byte(output), []byte(tt.expectDev)) {
				t.Errorf("expected dev name %q in output, got: %s", tt.expectDev, output)
			}
			if tt.expectTest != "" && !bytes.Contains([]byte(output), []byte(tt.expectTest)) {
				t.Errorf("expected test name %q in output, got: %s", tt.expectTest, output)
			}
		})
	}
}

func TestSetup_ConnectionErrorSuggestsStart(t *testing.T) {
	// Create temp dir with a valid project name
	tmpBase := t.TempDir()
	tmpDir := filepath.Join(tmpBase, "testproject")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	iniContent := "[db]\nurl = postgres://localhost:59999/mydb\n" // Use unlikely port
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
	err = Setup(cfg, &stdout, &stderr)

	if err == nil {
		t.Error("expected connection error")
	}

	// Error message should suggest starting the server
	if err != nil {
		errStr := err.Error()
		if !bytes.Contains([]byte(errStr), []byte("shipq db start")) {
			t.Errorf("error should suggest 'shipq db start', got: %v", err)
		}
	}
}

func TestMysqlURLToConnStr(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple localhost",
			url:      "mysql://localhost/mydb",
			expected: "tcp(localhost)/mydb",
		},
		{
			name:     "with port",
			url:      "mysql://localhost:3306/mydb",
			expected: "tcp(localhost:3306)/mydb",
		},
		{
			name:     "with user",
			url:      "mysql://root@localhost/mydb",
			expected: "root@tcp(localhost)/mydb",
		},
		{
			name:     "with user and password",
			url:      "mysql://root:pass@localhost:3306/mydb",
			expected: "root:pass@tcp(localhost:3306)/mydb",
		},
		{
			name:     "with query params",
			url:      "mysql://root@localhost/mydb?parseTime=true",
			expected: "root@tcp(localhost)/mydb?parseTime=true",
		},
		{
			name:     "empty database for maintenance",
			url:      "mysql://root@localhost/",
			expected: "root@tcp(localhost)/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mysqlURLToConnStr(tt.url)
			if result != tt.expected {
				t.Errorf("mysqlURLToConnStr() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPostgresURLToConnStr(t *testing.T) {
	// lib/pq and pgx can accept postgres:// URLs directly
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "simple URL",
			url:  "postgres://localhost/mydb",
		},
		{
			name: "full URL",
			url:  "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := postgresURLToConnStr(tt.url)
			// The function just returns the URL as-is since pgx accepts it
			if result != tt.url {
				t.Errorf("postgresURLToConnStr() = %q, want %q", result, tt.url)
			}
		})
	}
}

func TestSetup_InferURLFromDialect(t *testing.T) {
	tests := []struct {
		name       string
		iniContent string
		expectURL  bool // true if we expect a URL to be inferred
	}{
		{
			name: "postgres dialect configured",
			iniContent: `[db]
dialects = postgres
`,
			expectURL: true,
		},
		{
			name: "mysql dialect configured",
			iniContent: `[db]
dialects = mysql
`,
			expectURL: true,
		},
		{
			name: "sqlite dialect - no URL inferred",
			iniContent: `[db]
dialects = sqlite
`,
			expectURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir with a valid project name
			tmpBase := t.TempDir()
			tmpDir := filepath.Join(tmpBase, "testproject")
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(tt.iniContent), 0644); err != nil {
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
			err = Setup(cfg, &stdout, &stderr)

			if tt.expectURL {
				// Should attempt to connect (and fail since no server running)
				// but should NOT say "no database URL configured"
				if err != nil && bytes.Contains([]byte(err.Error()), []byte("no database URL configured")) {
					t.Errorf("should have inferred URL from dialect, got: %v", err)
				}
				// Should mention using default URL
				if !bytes.Contains(stderr.Bytes(), []byte("No db.url configured")) {
					t.Errorf("should mention using default URL, stderr: %s", stderr.String())
				}
			} else {
				// For sqlite, should get no URL error
				if err == nil || !bytes.Contains([]byte(err.Error()), []byte("no database URL configured")) {
					t.Errorf("sqlite should fail with no URL error, got: %v", err)
				}
			}
		})
	}
}

func TestSetup_InferURLFromDataDir(t *testing.T) {
	// Test that setup can infer the URL by detecting existing data directories
	tests := []struct {
		name          string
		createDataDir string
		expectDialect string
	}{
		{
			name:          "postgres data dir exists",
			createDataDir: "db/databases/.postgres-data",
			expectDialect: "postgres",
		},
		{
			name:          "mysql data dir exists",
			createDataDir: "db/databases/.mysql-data",
			expectDialect: "mysql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir with a valid project name
			tmpBase := t.TempDir()
			tmpDir := filepath.Join(tmpBase, "testproject")
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}

			// Write minimal shipq.ini (no URL, no dialects)
			iniContent := "[db]\nmigrations = migrations\n"
			if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(iniContent), 0644); err != nil {
				t.Fatalf("failed to write shipq.ini: %v", err)
			}

			// Create the data directory to trigger detection
			dataDir := filepath.Join(tmpDir, tt.createDataDir)
			if err := os.MkdirAll(dataDir, 0755); err != nil {
				t.Fatalf("failed to create data dir: %v", err)
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
			err = Setup(cfg, &stdout, &stderr)

			// Should NOT get "no database URL configured" error
			if err != nil && bytes.Contains([]byte(err.Error()), []byte("no database URL configured")) {
				t.Errorf("should have inferred URL from data dir, got: %v", err)
			}

			// Should mention found data directory
			stderrStr := stderr.String()
			if !bytes.Contains([]byte(stderrStr), []byte("Found")) {
				t.Errorf("should mention found data directory, stderr: %s", stderrStr)
			}
		})
	}
}
