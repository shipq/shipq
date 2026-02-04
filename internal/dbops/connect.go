package dbops

import (
	"database/sql"
	"fmt"
	"strings"
)

// MySQLURLToDSN converts a mysql:// URL to a MySQL driver DSN.
// Format: user:password@tcp(host:port)/dbname
func MySQLURLToDSN(mysqlURL string) (string, error) {
	// Strip mysql:// prefix
	importURL := mysqlURL
	if len(importURL) > 8 && importURL[:8] == "mysql://" {
		importURL = importURL[8:]
	}

	// Find @ separator
	atIdx := -1
	for i, c := range importURL {
		if c == '@' {
			atIdx = i
			break
		}
	}

	if atIdx == -1 {
		return "", fmt.Errorf("invalid MySQL URL: missing @ separator")
	}

	user := importURL[:atIdx]
	rest := importURL[atIdx+1:]

	// Find / separator for database
	slashIdx := -1
	for i, c := range rest {
		if c == '/' {
			slashIdx = i
			break
		}
	}

	var hostPort, dbName string
	if slashIdx == -1 {
		hostPort = rest
		dbName = ""
	} else {
		hostPort = rest[:slashIdx]
		dbName = rest[slashIdx+1:]
	}

	// Build DSN: user@tcp(host:port)/dbname
	return fmt.Sprintf("%s@tcp(%s)/%s", user, hostPort, dbName), nil
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
