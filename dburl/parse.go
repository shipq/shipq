package dburl

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Supported database dialects
const (
	DialectPostgres = "postgres"
	DialectMySQL    = "mysql"
	DialectSQLite   = "sqlite"
)

var (
	ErrUnknownDialect = errors.New("unknown database dialect")
	ErrInvalidURL     = errors.New("invalid database URL")
)

// InferDialectFromDBUrl returns the dialect ("postgres", "mysql", or "sqlite")
// based on the URL scheme.
func InferDialectFromDBUrl(dbURL string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "postgres", "postgresql":
		return DialectPostgres, nil
	case "mysql":
		return DialectMySQL, nil
	case "sqlite", "sqlite3":
		return DialectSQLite, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownDialect, scheme)
	}
}

// IsLocalhost returns true if the URL points to localhost (127.0.0.1, localhost, or ::1).
// For SQLite URLs, this always returns true since SQLite is file-based.
func IsLocalhost(dbURL string) bool {
	u, err := url.Parse(dbURL)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)

	// SQLite is always local
	if scheme == "sqlite" || scheme == "sqlite3" {
		return true
	}

	host := u.Hostname()
	host = strings.ToLower(host)

	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// BuildPostgresURL constructs a PostgreSQL connection URL.
// Format: postgres://user@host:port/dbname
func BuildPostgresURL(dbname, user, host string, port int) string {
	return fmt.Sprintf("postgres://%s@%s:%d/%s", user, host, port, dbname)
}

// BuildMySQLURL constructs a MySQL connection URL (TCP, no socket).
// Format: mysql://user@host:port/dbname
func BuildMySQLURL(dbname, user, host string, port int) string {
	return fmt.Sprintf("mysql://%s@%s:%d/%s", user, host, port, dbname)
}

// BuildSQLiteURL constructs a SQLite connection URL.
// Format: sqlite:///path/to/file.db
func BuildSQLiteURL(filepath string) string {
	// Ensure we have three slashes for absolute paths
	if strings.HasPrefix(filepath, "/") {
		return fmt.Sprintf("sqlite://%s", filepath)
	}
	// For relative paths, use sqlite:./path or sqlite:path
	return fmt.Sprintf("sqlite:%s", filepath)
}

// ParseDatabaseName extracts the database name from a URL.
// Returns an empty string if no database name is present.
func ParseDatabaseName(dbURL string) string {
	u, err := url.Parse(dbURL)
	if err != nil {
		return ""
	}

	// Remove leading slash from path
	path := strings.TrimPrefix(u.Path, "/")
	return path
}

// WithDatabaseName returns a new URL with the database name replaced.
func WithDatabaseName(dbURL, dbname string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	u.Path = "/" + dbname
	return u.String(), nil
}

// TestDatabaseURL returns the test database URL for a given dev URL.
// Convention: test database is named {dev_db}_test
// For SQLite: foo.db -> foo_test.db
func TestDatabaseURL(devURL string) (string, error) {
	devDBName := ParseDatabaseName(devURL)
	if devDBName == "" {
		return "", fmt.Errorf("could not parse database name from URL")
	}

	dialect, err := InferDialectFromDBUrl(devURL)
	if err != nil {
		return "", err
	}

	var testDBName string
	if dialect == DialectSQLite {
		// For SQLite, insert _test before the .db extension
		// path/to/foo.db -> path/to/foo_test.db
		if strings.HasSuffix(devDBName, ".db") {
			testDBName = strings.TrimSuffix(devDBName, ".db") + "_test.db"
		} else {
			testDBName = devDBName + "_test"
		}
	} else {
		testDBName = devDBName + "_test"
	}

	return WithDatabaseName(devURL, testDBName)
}
