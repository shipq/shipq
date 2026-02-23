package workers

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/inifile"
)

func TestGenerateRandomKey(t *testing.T) {
	key, err := generateRandomKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be a 64-character hex string (32 bytes = 64 hex chars)
	if len(key) != 64 {
		t.Errorf("expected key length 64, got %d", len(key))
	}

	// Should be valid hex
	_, err = hex.DecodeString(key)
	if err != nil {
		t.Errorf("expected valid hex string, got error: %v", err)
	}

	// Two calls should produce different keys
	key2, err := generateRandomKey()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if key == key2 {
		t.Error("expected two calls to generate different keys")
	}
}

func TestWorkersIniUpdate_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	iniPath := filepath.Join(tmpDir, "shipq.ini")

	// Write an initial ini file with [db] and [auth] sections
	initial := "[db]\ndatabase_url = mysql://root@localhost:3306/testdb\n\n[auth]\nprotect_by_default = true\n"
	if err := os.WriteFile(iniPath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial ini: %v", err)
	}

	// Simulate first run: add [workers] section
	ini1, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse ini: %v", err)
	}

	if ini1.Section("workers") == nil {
		ini1.Set("workers", "redis_url", "redis://localhost:6379")
		ini1.Set("workers", "centrifugo_api_url", "http://localhost:8000/api")
		ini1.Set("workers", "centrifugo_api_key", "first-run-key")
		ini1.Set("workers", "centrifugo_hmac_secret", "first-run-secret")
		ini1.Set("workers", "centrifugo_ws_url", "ws://localhost:8000/connection/websocket")
		if err := ini1.WriteFile(iniPath); err != nil {
			t.Fatalf("failed to write ini (first run): %v", err)
		}
	}

	// Capture values after first run
	ini1After, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to re-read ini after first run: %v", err)
	}
	firstKey := ini1After.Get("workers", "centrifugo_api_key")
	firstSecret := ini1After.Get("workers", "centrifugo_hmac_secret")

	if firstKey != "first-run-key" {
		t.Errorf("expected api key 'first-run-key', got %q", firstKey)
	}

	// Simulate second run: should NOT overwrite
	ini2, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to parse ini (second run): %v", err)
	}

	if ini2.Section("workers") == nil {
		// This should NOT execute because the section already exists
		ini2.Set("workers", "centrifugo_api_key", "second-run-key")
		ini2.Set("workers", "centrifugo_hmac_secret", "second-run-secret")
		if err := ini2.WriteFile(iniPath); err != nil {
			t.Fatalf("failed to write ini (second run): %v", err)
		}
	}

	// Re-read and verify values are unchanged
	ini2After, err := inifile.ParseFile(iniPath)
	if err != nil {
		t.Fatalf("failed to re-read ini after second run: %v", err)
	}

	secondKey := ini2After.Get("workers", "centrifugo_api_key")
	secondSecret := ini2After.Get("workers", "centrifugo_hmac_secret")

	if secondKey != firstKey {
		t.Errorf("api key changed on second run: %q -> %q", firstKey, secondKey)
	}
	if secondSecret != firstSecret {
		t.Errorf("hmac secret changed on second run: %q -> %q", firstSecret, secondSecret)
	}

	// Verify no duplicate [workers] sections
	content, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("failed to read ini file: %v", err)
	}

	count := 0
	lines := string(content)
	for _, line := range splitLines(lines) {
		if line == "[workers]" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 [workers] section, found %d", count)
	}
}

func TestExtractRedisAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard redis URL",
			input:    "redis://localhost:6379",
			expected: "localhost:6379",
		},
		{
			name:     "custom host and port",
			input:    "redis://myredis:6380",
			expected: "myredis:6380",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "localhost:6379",
		},
		{
			name:     "just host:port without scheme",
			input:    "localhost:6379",
			expected: "localhost:6379",
		},
		{
			name:     "redis URL with path",
			input:    "redis://localhost:6379/0",
			expected: "localhost:6379/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRedisAddr(tt.input)
			if got != tt.expected {
				t.Errorf("extractRedisAddr(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestJobResultsMigrationExists(t *testing.T) {
	t.Run("returns false for empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if jobResultsMigrationExists(tmpDir) {
			t.Error("expected false for empty directory")
		}
	})

	t.Run("returns false for non-existing directory", func(t *testing.T) {
		if jobResultsMigrationExists("/non/existing/path") {
			t.Error("expected false for non-existing directory")
		}
	})

	t.Run("returns true when migration exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		migrationFile := filepath.Join(tmpDir, "20250101120000_job_results.go")
		if err := os.WriteFile(migrationFile, []byte("package migrations"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		if !jobResultsMigrationExists(tmpDir) {
			t.Error("expected true when job_results migration exists")
		}
	})

	t.Run("returns false for unrelated migrations", func(t *testing.T) {
		tmpDir := t.TempDir()
		otherFile := filepath.Join(tmpDir, "20250101120000_accounts.go")
		if err := os.WriteFile(otherFile, []byte("package migrations"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		if jobResultsMigrationExists(tmpDir) {
			t.Error("expected false when only unrelated migrations exist")
		}
	})

	t.Run("ignores directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "20250101120000_job_results.go")
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		if jobResultsMigrationExists(tmpDir) {
			t.Error("expected false when matching name is a directory")
		}
	})
}

func TestGenerateExampleChannel(t *testing.T) {
	code := generateExampleChannel("myapp")

	// Check package declaration
	if !containsStr(code, "package example") {
		t.Error("expected 'package example' in generated code")
	}

	// Check import of channel library
	if !containsStr(code, `"myapp/shipq/lib/channel"`) {
		t.Error("expected channel library import")
	}

	// Check struct definitions
	if !containsStr(code, "type EchoRequest struct") {
		t.Error("expected EchoRequest struct")
	}
	if !containsStr(code, "type EchoResponse struct") {
		t.Error("expected EchoResponse struct")
	}

	// Check Register function
	if !containsStr(code, "func Register(app *channel.App)") {
		t.Error("expected Register function")
	}

	// Check channel name matches
	if !containsStr(code, `"example"`) {
		t.Error("expected channel name 'example'")
	}

	// Handler must NOT be in register.go — it references TypedChannelFromContext
	// which is generated later. Including it here causes a compile failure during
	// channel registry compilation (the regression this split fixes).
	if containsStr(code, "HandleEchoRequest") {
		t.Error("register.go must not contain HandleEchoRequest (it references generated code)")
	}
	if containsStr(code, "TypedChannelFromContext") {
		t.Error("register.go must not reference TypedChannelFromContext")
	}
}

func TestGenerateExampleHandler(t *testing.T) {
	code := generateExampleHandler()

	// Check package declaration
	if !containsStr(code, "package example") {
		t.Error("expected 'package example' in generated code")
	}

	// Check context import
	if !containsStr(code, `"context"`) {
		t.Error("expected context import")
	}

	// Check handler function
	if !containsStr(code, "func HandleEchoRequest(ctx context.Context, req *EchoRequest) error") {
		t.Error("expected HandleEchoRequest function")
	}

	// Check it uses TypedChannelFromContext
	if !containsStr(code, "TypedChannelFromContext(ctx)") {
		t.Error("expected TypedChannelFromContext call")
	}

	// Check it sends the echo response
	if !containsStr(code, "SendEchoResponse") {
		t.Error("expected SendEchoResponse call")
	}
}

// splitLines splits a string into lines for searching.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
