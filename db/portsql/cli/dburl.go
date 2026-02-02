// Package cli provides the PortSQL command-line interface.
package cli

import (
	"fmt"
	"net/url"
	"strings"
)

// ParsedDBURL represents a parsed database URL with its components.
type ParsedDBURL struct {
	// Original is the original URL string
	Original string

	// Dialect is the database type: "postgres", "mysql", or "sqlite"
	Dialect string

	// Host is the hostname (without port)
	Host string

	// Port is the port number (empty string if not specified)
	Port string

	// User is the username
	User string

	// Password is the password (empty string if not specified)
	Password string

	// Database is the database name (or file path for SQLite)
	Database string

	// Query contains any query parameters
	Query string
}

// ParseDBURL parses a database URL into its components.
// Supports postgres://, postgresql://, mysql://, and sqlite:// URLs.
func ParseDBURL(databaseURL string) (*ParsedDBURL, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is empty")
	}

	// Handle SQLite specially
	if strings.HasPrefix(databaseURL, "sqlite:") {
		return parseSQLiteURL(databaseURL)
	}

	// Parse as a standard URL
	u, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	parsed := &ParsedDBURL{
		Original: databaseURL,
		Host:     u.Hostname(),
		Port:     u.Port(),
		Query:    u.RawQuery,
	}

	// Determine dialect
	switch u.Scheme {
	case "postgres", "postgresql":
		parsed.Dialect = "postgres"
	case "mysql":
		parsed.Dialect = "mysql"
	default:
		return nil, fmt.Errorf("unsupported database URL scheme: %q (supported: postgres, postgresql, mysql, sqlite)", u.Scheme)
	}

	// Extract user/password
	if u.User != nil {
		parsed.User = u.User.Username()
		parsed.Password, _ = u.User.Password()
	}

	// Extract database name (path without leading /)
	parsed.Database = strings.TrimPrefix(u.Path, "/")

	return parsed, nil
}

// parseSQLiteURL parses a SQLite URL.
func parseSQLiteURL(databaseURL string) (*ParsedDBURL, error) {
	parsed := &ParsedDBURL{
		Original: databaseURL,
		Dialect:  "sqlite",
	}

	// Handle various SQLite URL formats:
	// sqlite:///path/to/file.db (absolute path)
	// sqlite://path/to/file.db (relative path)
	// sqlite:path/to/file.db (simple format)
	path := databaseURL
	if strings.HasPrefix(path, "sqlite:///") {
		path = path[len("sqlite://"):] // Keep leading / for absolute path
	} else if strings.HasPrefix(path, "sqlite://") {
		path = path[len("sqlite://"):]
	} else if strings.HasPrefix(path, "sqlite:") {
		path = path[len("sqlite:"):]
	}

	// Check for query params
	if idx := strings.Index(path, "?"); idx != -1 {
		parsed.Query = path[idx+1:]
		path = path[:idx]
	}

	parsed.Database = path
	return parsed, nil
}

// WithDatabase returns a new URL string with the database name replaced.
// All other components (user, password, host, port, query params) are preserved.
func (p *ParsedDBURL) WithDatabase(dbName string) string {
	if p.Dialect == "sqlite" {
		return p.withDatabaseSQLite(dbName)
	}

	var sb strings.Builder

	// Scheme
	if p.Dialect == "postgres" {
		sb.WriteString("postgres://")
	} else {
		sb.WriteString("mysql://")
	}

	// User info
	if p.User != "" {
		sb.WriteString(url.PathEscape(p.User))
		if p.Password != "" {
			sb.WriteString(":")
			sb.WriteString(url.PathEscape(p.Password))
		}
		sb.WriteString("@")
	}

	// Host and port
	sb.WriteString(p.Host)
	if p.Port != "" {
		sb.WriteString(":")
		sb.WriteString(p.Port)
	}

	// Database name
	sb.WriteString("/")
	sb.WriteString(dbName)

	// Query params
	if p.Query != "" {
		sb.WriteString("?")
		sb.WriteString(p.Query)
	}

	return sb.String()
}

// withDatabaseSQLite handles database name replacement for SQLite.
// For SQLite, the "database" is a file path, so we replace the filename
// while keeping the directory.
func (p *ParsedDBURL) withDatabaseSQLite(dbName string) string {
	// For SQLite, if the dbName doesn't end in .db, add it
	if !strings.HasSuffix(dbName, ".db") && !strings.HasSuffix(dbName, ".sqlite") {
		dbName = dbName + ".db"
	}

	var sb strings.Builder
	sb.WriteString("sqlite://")
	sb.WriteString(dbName)

	if p.Query != "" {
		sb.WriteString("?")
		sb.WriteString(p.Query)
	}

	return sb.String()
}

// IsLocalhost returns true if the URL points to a local database.
// SQLite is always considered local.
// For postgres/mysql, checks if host is localhost, 127.0.0.1, or ::1.
func (p *ParsedDBURL) IsLocalhost() bool {
	if p.Dialect == "sqlite" {
		return true
	}

	return p.Host == "localhost" || p.Host == "127.0.0.1" || p.Host == "::1"
}

// MaintenanceURL returns a URL suitable for administrative operations.
// For Postgres: connects to the "postgres" database
// For MySQL: connects without specifying a database
// For SQLite: returns empty (not applicable)
func (p *ParsedDBURL) MaintenanceURL() string {
	switch p.Dialect {
	case "postgres":
		return p.WithDatabase("postgres")
	case "mysql":
		// For MySQL, we can connect without specifying a database
		return p.WithDatabase("")
	default:
		return ""
	}
}

// String returns the original URL string.
func (p *ParsedDBURL) String() string {
	return p.Original
}

// DeriveDBNames derives the dev and test database names based on config and project folder.
// Returns (devName, testName).
//
// Precedence:
// 1. Explicit dev_name/test_name if set
// 2. Derived from base name (name config) if set
// 3. Derived from project folder name
func DeriveDBNames(baseName, devNameOverride, testNameOverride, projectFolder string) (devName, testName string) {
	// Determine the base name to use
	base := baseName
	if base == "" {
		base = projectFolder
	}

	// Sanitize the base name (only allow alphanumeric and underscores)
	base = sanitizeDBName(base)

	// Apply overrides or derive from base
	if devNameOverride != "" {
		devName = sanitizeDBName(devNameOverride)
	} else {
		devName = base
	}

	if testNameOverride != "" {
		testName = sanitizeDBName(testNameOverride)
	} else {
		testName = base + "_test"
	}

	return devName, testName
}

// sanitizeDBName sanitizes a database name to be safe for use.
// Replaces hyphens with underscores and removes invalid characters.
func sanitizeDBName(name string) string {
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")

	// Only keep alphanumeric and underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// ValidateDBName checks if a database name is valid.
// Returns an error if the name is invalid.
func ValidateDBName(name string) error {
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	// Check that it starts with a letter or underscore
	first := rune(name[0])
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return fmt.Errorf("database name must start with a letter or underscore: %q", name)
	}

	// Check all characters are valid
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("database name contains invalid character %q: %q", string(r), name)
		}
	}

	// Check length (most databases have limits around 63-64 characters)
	if len(name) > 63 {
		return fmt.Errorf("database name too long (max 63 characters): %q", name)
	}

	return nil
}
