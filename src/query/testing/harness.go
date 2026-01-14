//go:build integration

package testing

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/portsql/portsql/src/query"
	"github.com/portsql/portsql/src/query/compile"
	_ "modernc.org/sqlite"
)

// TestDBs holds connections to all three database types.
type TestDBs struct {
	Postgres *pgx.Conn
	MySQL    *sql.DB
	SQLite   *sql.DB
}

// Dialect represents a database dialect
type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	DialectMySQL    Dialect = "mysql"
	DialectSQLite   Dialect = "sqlite"
)

// CompileFor compiles an AST for the specified dialect
func CompileFor(ast *query.AST, dialect Dialect) (string, []string, error) {
	switch dialect {
	case DialectPostgres:
		return compile.NewCompiler(compile.Postgres).Compile(ast)
	case DialectMySQL:
		return compile.NewCompiler(compile.MySQL).Compile(ast)
	case DialectSQLite:
		return compile.NewCompiler(compile.SQLite).Compile(ast)
	default:
		return "", nil, fmt.Errorf("unknown dialect: %s", dialect)
	}
}

// AllDialects returns all supported dialects
func AllDialects() []Dialect {
	return []Dialect{DialectPostgres, DialectMySQL, DialectSQLite}
}

// SetupTestDBs creates test databases with identical schemas.
// Returns nil for any database that is unavailable, allowing tests to skip.
func SetupTestDBs(t *testing.T) (*TestDBs, func()) {
	t.Helper()

	pgConn := setupPostgres(t)
	myDB := setupMySQL(t)
	sqDB := setupSQLite(t)

	// All databases must be available for cross-db tests
	if pgConn == nil || myDB == nil || sqDB == nil {
		if pgConn != nil {
			pgConn.Close(context.Background())
		}
		if myDB != nil {
			myDB.Close()
		}
		if sqDB != nil {
			sqDB.Close()
		}
		t.Skip("Not all databases available for cross-database testing")
		return nil, func() {}
	}

	dbs := &TestDBs{
		Postgres: pgConn,
		MySQL:    myDB,
		SQLite:   sqDB,
	}

	// Create identical schemas on all databases
	createTestSchema(t, dbs)

	cleanup := func() {
		pgConn.Close(context.Background())
		myDB.Close()
		sqDB.Close()
	}

	return dbs, cleanup
}

// setupPostgres connects to Postgres
func setupPostgres(t *testing.T) *pgx.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connString := "host=/tmp user=postgres database=postgres"
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		t.Logf("PostgreSQL unavailable: %v", err)
		return nil
	}

	return conn
}

// setupMySQL connects to MySQL
func setupMySQL(t *testing.T) *sql.DB {
	t.Helper()

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Logf("MySQL unavailable: cannot determine working directory: %v", err)
			return nil
		}
		// We're in orm/query/testing, so go up 3 levels
		projectRoot = filepath.Join(cwd, "..", "..", "..")
	}

	socketPath := filepath.Join(projectRoot, "databases", ".mysql-data", "mysql.sock")

	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Logf("MySQL unavailable: socket not found at %s", socketPath)
		return nil
	}

	// First connect without database to create it if needed
	dsnNoDb := "root@unix(" + socketPath + ")/?multiStatements=true"
	tempDb, err := sql.Open("mysql", dsnNoDb)
	if err != nil {
		t.Logf("MySQL unavailable: %v", err)
		return nil
	}
	tempDb.Exec("CREATE DATABASE IF NOT EXISTS test")
	tempDb.Close()

	// Now connect to the test database
	dsn := "root@unix(" + socketPath + ")/test?multiStatements=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Logf("MySQL unavailable: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		db.Close()
		t.Logf("MySQL unavailable: %v", err)
		return nil
	}

	return db
}

// setupSQLite creates an in-memory SQLite database
func setupSQLite(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Logf("SQLite unavailable: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		db.Close()
		t.Logf("SQLite unavailable: %v", err)
		return nil
	}

	return db
}

// createTestSchema creates identical schemas on all databases
func createTestSchema(t *testing.T, dbs *TestDBs) {
	t.Helper()

	ctx := context.Background()

	// Drop existing tables first
	// Postgres
	dbs.Postgres.Exec(ctx, "DROP TABLE IF EXISTS test_books CASCADE")
	dbs.Postgres.Exec(ctx, "DROP TABLE IF EXISTS test_authors CASCADE")

	// MySQL
	dbs.MySQL.Exec("SET FOREIGN_KEY_CHECKS = 0")
	dbs.MySQL.Exec("DROP TABLE IF EXISTS test_books")
	dbs.MySQL.Exec("DROP TABLE IF EXISTS test_authors")
	dbs.MySQL.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// SQLite
	dbs.SQLite.Exec("DROP TABLE IF EXISTS test_books")
	dbs.SQLite.Exec("DROP TABLE IF EXISTS test_authors")

	// Create authors table
	pgAuthorsSQL := `
		CREATE TABLE test_authors (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			bio TEXT,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP
		)
	`

	myAuthorsSQL := `
		CREATE TABLE test_authors (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			bio TEXT,
			active TINYINT(1) NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT NOW(),
			updated_at DATETIME NOT NULL DEFAULT NOW(),
			deleted_at DATETIME
		)
	`

	sqAuthorsSQL := `
		CREATE TABLE test_authors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			bio TEXT,
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			deleted_at TEXT
		)
	`

	if _, err := dbs.Postgres.Exec(ctx, pgAuthorsSQL); err != nil {
		t.Fatalf("postgres create authors failed: %v", err)
	}
	if _, err := dbs.MySQL.Exec(myAuthorsSQL); err != nil {
		t.Fatalf("mysql create authors failed: %v", err)
	}
	if _, err := dbs.SQLite.Exec(sqAuthorsSQL); err != nil {
		t.Fatalf("sqlite create authors failed: %v", err)
	}

	// Create books table
	pgBooksSQL := `
		CREATE TABLE test_books (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			author_id BIGINT NOT NULL REFERENCES test_authors(id),
			title VARCHAR(255) NOT NULL,
			price DECIMAL(10,2),
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`

	myBooksSQL := `
		CREATE TABLE test_books (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			author_id BIGINT NOT NULL,
			title VARCHAR(255) NOT NULL,
			price DECIMAL(10,2),
			created_at DATETIME NOT NULL DEFAULT NOW(),
			FOREIGN KEY (author_id) REFERENCES test_authors(id)
		)
	`

	sqBooksSQL := `
		CREATE TABLE test_books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			author_id INTEGER NOT NULL REFERENCES test_authors(id),
			title TEXT NOT NULL,
			price TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`

	if _, err := dbs.Postgres.Exec(ctx, pgBooksSQL); err != nil {
		t.Fatalf("postgres create books failed: %v", err)
	}
	if _, err := dbs.MySQL.Exec(myBooksSQL); err != nil {
		t.Fatalf("mysql create books failed: %v", err)
	}
	if _, err := dbs.SQLite.Exec(sqBooksSQL); err != nil {
		t.Fatalf("sqlite create books failed: %v", err)
	}

	// Verify tables exist in MySQL (most problematic)
	var tableName string
	row := dbs.MySQL.QueryRow("SHOW TABLES LIKE 'test_books'")
	if err := row.Scan(&tableName); err != nil {
		t.Fatalf("mysql test_books table verification failed: %v", err)
	}
	row = dbs.MySQL.QueryRow("SHOW TABLES LIKE 'test_authors'")
	if err := row.Scan(&tableName); err != nil {
		t.Fatalf("mysql test_authors table verification failed: %v", err)
	}
}

// mockTable implements query.Table for testing
type mockTable struct {
	name string
}

func (m mockTable) TableName() string { return m.name }

// MockTable returns a mock table for testing
func MockTable(name string) mockTable {
	return mockTable{name: name}
}
