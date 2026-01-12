package cli

import (
	"net/url"
	"strings"
)

// IsLocalhostURL checks if a database URL points to localhost.
// SQLite URLs are always considered localhost.
// For postgres:// and mysql://, checks if host is "localhost" or "127.0.0.1".
func IsLocalhostURL(databaseURL string) bool {
	if databaseURL == "" {
		return false
	}

	// SQLite is always considered localhost (it's a local file)
	if strings.HasPrefix(databaseURL, "sqlite://") || strings.HasPrefix(databaseURL, "sqlite:") {
		return true
	}

	// Parse the URL
	u, err := url.Parse(databaseURL)
	if err != nil {
		return false
	}

	// Get the hostname (without port)
	host := u.Hostname()

	// Check if it's localhost
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// ParseDialect extracts the database dialect from a URL.
// Returns "postgres", "mysql", or "sqlite".
func ParseDialect(databaseURL string) string {
	if databaseURL == "" {
		return ""
	}

	if strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://") {
		return "postgres"
	}
	if strings.HasPrefix(databaseURL, "mysql://") {
		return "mysql"
	}
	if strings.HasPrefix(databaseURL, "sqlite://") || strings.HasPrefix(databaseURL, "sqlite:") {
		return "sqlite"
	}

	return ""
}

// ParseSQLitePath extracts the file path from a SQLite URL.
// Handles both sqlite:///path and sqlite:/path formats.
func ParseSQLitePath(databaseURL string) string {
	if strings.HasPrefix(databaseURL, "sqlite:///") {
		return databaseURL[len("sqlite://"):]
	}
	if strings.HasPrefix(databaseURL, "sqlite://") {
		return databaseURL[len("sqlite://"):]
	}
	if strings.HasPrefix(databaseURL, "sqlite:") {
		return databaseURL[len("sqlite:"):]
	}
	return databaseURL
}
