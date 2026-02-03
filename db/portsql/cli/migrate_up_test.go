package cli

import (
	"testing"
)

func TestConvertMySQLURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple URL without query params",
			input:    "root:password@localhost:3306/mydb",
			expected: "root:password@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL without password",
			input:    "root@localhost:3306/mydb",
			expected: "root@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL with existing query params",
			input:    "root@localhost:3306/mydb?parseTime=true",
			expected: "root@tcp(localhost:3306)/mydb?parseTime=true&multiStatements=true",
		},
		{
			name:     "URL already has multiStatements=true",
			input:    "root@localhost:3306/mydb?multiStatements=true",
			expected: "root@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL with multiple existing query params",
			input:    "root@localhost:3306/mydb?parseTime=true&charset=utf8mb4",
			expected: "root@tcp(localhost:3306)/mydb?parseTime=true&charset=utf8mb4&multiStatements=true",
		},
		{
			name:     "URL without port",
			input:    "root@localhost/mydb",
			expected: "root@tcp(localhost)/mydb?multiStatements=true",
		},
		{
			name:     "URL with complex password",
			input:    "admin:p@ssw0rd@localhost:3306/mydb",
			expected: "admin:p@ssw0rd@tcp(localhost:3306)/mydb?multiStatements=true",
		},
		{
			name:     "URL without @ returns unchanged (invalid format)",
			input:    "localhost:3306/mydb",
			expected: "localhost:3306/mydb",
		},
		{
			name:     "URL without slash returns unchanged (invalid format)",
			input:    "root@localhost:3306",
			expected: "root@localhost:3306",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := convertMySQLURL(tc.input)
			if result != tc.expected {
				t.Errorf("convertMySQLURL(%q)\n  got:  %q\n  want: %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestConvertMySQLURL_MultiStatements_Required(t *testing.T) {
	// This test documents WHY multiStatements=true is required:
	// Migrations can generate SQL like:
	//   CREATE TABLE `users` (...);
	//   CREATE UNIQUE INDEX `idx_users_email` ON `users` (`email`)
	//
	// MySQL's default behavior rejects multiple statements in a single Exec() call
	// unless multiStatements=true is set in the DSN.

	// Verify that convertMySQLURL always adds multiStatements=true
	testCases := []string{
		"root@localhost:3306/mydb",
		"root:pass@localhost:3306/mydb",
		"root@localhost:3306/mydb?parseTime=true",
	}

	for _, tc := range testCases {
		result := convertMySQLURL(tc)
		if result == "" {
			continue // Invalid input, skip
		}
		if !containsMultiStatements(result) {
			t.Errorf("convertMySQLURL(%q) = %q, missing multiStatements=true", tc, result)
		}
	}
}

// containsMultiStatements checks if a DSN contains multiStatements=true
func containsMultiStatements(dsn string) bool {
	return len(dsn) > 0 && (indexOf(dsn, "multiStatements=true") >= 0)
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
