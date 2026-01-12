package cli

import "testing"

func TestIsLocalhostURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Localhost cases
		{"postgres localhost", "postgres://localhost/mydb", true},
		{"postgres localhost with port", "postgres://localhost:5432/mydb", true},
		{"postgres localhost with user", "postgres://user:pass@localhost:5432/mydb", true},
		{"postgres 127.0.0.1", "postgres://127.0.0.1/mydb", true},
		{"postgres 127.0.0.1 with port", "postgres://127.0.0.1:5432/mydb", true},
		{"mysql localhost", "mysql://localhost/mydb", true},
		{"mysql 127.0.0.1", "mysql://127.0.0.1:3306/mydb", true},
		{"sqlite triple slash", "sqlite:///path/to/db.sqlite", true},
		{"sqlite single colon", "sqlite:/path/to/db.sqlite", true},
		{"sqlite relative path", "sqlite:./test.db", true},

		// Non-localhost cases
		{"postgres remote", "postgres://db.example.com/mydb", false},
		{"postgres remote ip", "postgres://192.168.1.100:5432/mydb", false},
		{"mysql remote", "mysql://db.example.com/mydb", false},
		{"empty url", "", false},

		// Edge cases
		{"postgres ipv6 localhost", "postgres://[::1]:5432/mydb", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLocalhostURL(tt.url)
			if got != tt.want {
				t.Errorf("IsLocalhostURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseDialect(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"postgres", "postgres://localhost/mydb", "postgres"},
		{"postgresql", "postgresql://localhost/mydb", "postgres"},
		{"mysql", "mysql://localhost/mydb", "mysql"},
		{"sqlite triple slash", "sqlite:///path/to/db.sqlite", "sqlite"},
		{"sqlite colon", "sqlite:/path/to/db.sqlite", "sqlite"},
		{"empty", "", ""},
		{"unknown", "http://example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDialect(tt.url)
			if got != tt.want {
				t.Errorf("ParseDialect(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseSQLitePath(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"triple slash", "sqlite:///path/to/db.sqlite", "/path/to/db.sqlite"},
		{"double slash", "sqlite://path/to/db.sqlite", "path/to/db.sqlite"},
		{"single colon", "sqlite:/path/to/db.sqlite", "/path/to/db.sqlite"},
		{"relative path", "sqlite:./test.db", "./test.db"},
		{"just path", "/path/to/db.sqlite", "/path/to/db.sqlite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSQLitePath(tt.url)
			if got != tt.want {
				t.Errorf("ParseSQLitePath(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
