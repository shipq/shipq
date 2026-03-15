package dbops

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
)

// MySQLURLToDSN converts a mysql:// URL to a go-sql-driver/mysql DSN.
// Format: user:password@tcp(host:port)/dbname?params
//
// Query parameters from the input URL are preserved. If not explicitly set,
// parseTime=true and loc=Local are added as defaults — parseTime so the driver
// scans DATETIME columns into time.Time, and loc so timestamps use the
// server's local timezone rather than UTC.
func MySQLURLToDSN(mysqlURL string) (string, error) {
	u, err := url.Parse(mysqlURL)
	if err != nil {
		return "", fmt.Errorf("invalid MySQL URL: %w", err)
	}

	if u.Scheme != "mysql" {
		return "", fmt.Errorf("invalid MySQL URL: unexpected scheme %q", u.Scheme)
	}

	user := u.User.String()
	host := u.Host

	dbName := ""
	if len(u.Path) > 1 {
		dbName = u.Path[1:]
	}

	params := u.Query()
	if params.Get("parseTime") == "" {
		params.Set("parseTime", "true")
	}
	if params.Get("loc") == "" {
		params.Set("loc", "Local")
	}

	return fmt.Sprintf("%s@tcp(%s)/%s?%s", user, host, dbName, params.Encode()), nil
}

// OpenMaintenanceDB opens a connection to the maintenance database.
// For Postgres, connects to "postgres" database.
// For MySQL, connects without selecting a database.
// For SQLite, returns nil (no maintenance DB needed).
//
// The caller is responsible for registering the appropriate database drivers.
func OpenMaintenanceDB(databaseURL, dialect string) (*sql.DB, error) {
	switch dialect {
	case "postgres":
		// Connect to the "postgres" database for maintenance operations
		maintenanceURL, err := replacePostgresDBName(databaseURL, "postgres")
		if err != nil {
			return nil, fmt.Errorf("failed to build maintenance URL: %w", err)
		}
		db, err := sql.Open("pgx", maintenanceURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open PostgreSQL maintenance connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}
		return db, nil

	case "mysql":
		// Connect without specifying a database
		dsn, err := MySQLURLToDSN(databaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MySQL URL: %w", err)
		}
		// Remove database name from DSN for maintenance operations
		dsn = removeMySQLDBName(dsn)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open MySQL maintenance connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to ping MySQL: %w", err)
		}
		return db, nil

	case "sqlite":
		// SQLite doesn't need a maintenance database
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

// replacePostgresDBName replaces the database name in a PostgreSQL URL.
func replacePostgresDBName(pgURL, newDBName string) (string, error) {
	// Find the path component and replace it
	// Format: postgres://user@host:port/dbname
	lastSlash := strings.LastIndex(pgURL, "/")
	if lastSlash == -1 {
		return "", fmt.Errorf("invalid PostgreSQL URL: no path separator")
	}

	// Check if there's a query string
	queryIdx := strings.Index(pgURL[lastSlash:], "?")
	if queryIdx != -1 {
		return pgURL[:lastSlash+1] + newDBName + pgURL[lastSlash+queryIdx:], nil
	}

	return pgURL[:lastSlash+1] + newDBName, nil
}

// removeMySQLDBName removes the database name from a MySQL DSN.
// Input:  user@tcp(host:port)/dbname
// Output: user@tcp(host:port)/
func removeMySQLDBName(dsn string) string {
	slashIdx := strings.LastIndex(dsn, "/")
	if slashIdx == -1 {
		return dsn
	}
	return dsn[:slashIdx+1]
}

// SQLiteURLToPath extracts the file path from a SQLite URL.
func SQLiteURLToPath(sqliteURL string) string {
	if len(sqliteURL) > 9 && sqliteURL[:9] == "sqlite://" {
		return sqliteURL[9:]
	}
	if len(sqliteURL) > 7 && sqliteURL[:7] == "sqlite:" {
		return sqliteURL[7:]
	}
	return sqliteURL
}
