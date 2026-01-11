//go:build integration

package compile

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/portsql/portsql/src/query"
)

// connectMySQL attempts to connect to MySQL and returns a database connection.
// Returns nil and skips the test if MySQL is unavailable.
func connectMySQL(t *testing.T) *sql.DB {
	t.Helper()

	// Find the MySQL socket path
	// The socket is at $PROJECT_ROOT/databases/.mysql-data/mysql.sock
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// Try to find it relative to the test file
		// We're in orm/query/compile, so go up 3 levels
		cwd, err := os.Getwd()
		if err != nil {
			t.Skipf("MySQL unavailable: cannot determine working directory: %v", err)
			return nil
		}
		projectRoot = filepath.Join(cwd, "..", "..", "..")
	}

	socketPath := filepath.Join(projectRoot, "databases", ".mysql-data", "mysql.sock")

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skipf("MySQL unavailable: socket not found at %s. Please see the README for instructions about how to start all databases.", socketPath)
		return nil
	}

	// Connect via Unix socket
	// DSN format: user:password@unix(/path/to/socket)/database
	dsn := "root@unix(" + socketPath + ")/test?multiStatements=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("MySQL unavailable: %v. Please see the README for instructions about how to start all databases.", err)
		return nil
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: %v. Please see the README for instructions about how to start all databases.", err)
		return nil
	}

	// Create test database if it doesn't exist and use it
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS test")
	if err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: failed to create test database: %v", err)
		return nil
	}

	_, err = db.Exec("USE test")
	if err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: failed to use test database: %v", err)
		return nil
	}

	return db
}

func TestMySQLIntegration_SelectExecutes(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		SET FOREIGN_KEY_CHECKS = 0;
		DROP TABLE IF EXISTS compile_authors;
		SET FOREIGN_KEY_CHECKS = 1;
		CREATE TABLE compile_authors (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS compile_authors`)

	// Insert test data
	_, err = db.Exec("INSERT INTO compile_authors (public_id, name) VALUES ('abc123', 'Alice')")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build and compile query using the builder
	publicID := query.StringColumn{Table: "compile_authors", Name: "public_id"}
	name := query.StringColumn{Table: "compile_authors", Name: "name"}

	ast := query.From(mockTable{name: "compile_authors"}).
		Select(publicID, name).
		Where(publicID.Eq(query.Param[string]("public_id"))).
		Build()

	sql, params, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute the query
	row := db.QueryRow(sql, "abc123")

	var gotPublicID, gotName string
	err = row.Scan(&gotPublicID, &gotName)
	if err != nil {
		t.Fatalf("failed to scan row: %v", err)
	}

	if gotPublicID != "abc123" {
		t.Errorf("expected public_id = %q, got %q", "abc123", gotPublicID)
	}
	if gotName != "Alice" {
		t.Errorf("expected name = %q, got %q", "Alice", gotName)
	}
}

func TestMySQLIntegration_SelectWithOrderByLimitOffset(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_users;
		CREATE TABLE test_users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_users`)

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO test_users (name) VALUES ('Alice'), ('Bob'), ('Charlie'), ('David')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build query with ORDER BY, LIMIT, OFFSET
	nameCol := query.StringColumn{Table: "test_users", Name: "name"}

	ast := query.From(mockTable{name: "test_users"}).
		Select(nameCol).
		OrderBy(nameCol.Asc()).
		Limit(query.Param[int]("limit")).
		Offset(query.Param[int]("offset")).
		Build()

	sql, params, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute: get 2 users starting from offset 1
	rows, err := db.Query(sql, 2, 1)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		names = append(names, name)
	}

	// With ORDER BY name ASC, LIMIT 2, OFFSET 1:
	// Full order: Alice, Bob, Charlie, David
	// After offset 1: Bob, Charlie, David
	// Limit 2: Bob, Charlie
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "Bob" || names[1] != "Charlie" {
		t.Errorf("expected [Bob, Charlie], got %v", names)
	}
}

func TestMySQLIntegration_InsertWithLastInsertId(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table with AUTO_INCREMENT
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_posts;
		CREATE TABLE test_posts (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			title VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_posts`)

	// Build INSERT (no RETURNING for MySQL)
	publicID := query.StringColumn{Table: "test_posts", Name: "public_id"}
	title := query.StringColumn{Table: "test_posts", Name: "title"}

	ast := query.InsertInto(mockTable{name: "test_posts"}).
		Columns(publicID, title).
		Values(query.Param[string]("public_id"), query.Param[string]("title")).
		Build()

	sql, params, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute and get LastInsertId
	result, err := db.Exec(sql, "xyz789", "Test Post")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get LastInsertId: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected LastInsertId > 0, got %d", id)
	}

	// Verify the row exists
	var gotTitle string
	err = db.QueryRow("SELECT title FROM test_posts WHERE id = ?", id).Scan(&gotTitle)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if gotTitle != "Test Post" {
		t.Errorf("expected title = %q, got %q", "Test Post", gotTitle)
	}
}

func TestMySQLIntegration_InsertWithNow(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_events;
		CREATE TABLE test_events (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_events`)

	// Build INSERT with NOW()
	nameCol := query.StringColumn{Table: "test_events", Name: "name"}
	createdAtCol := query.TimeColumn{Table: "test_events", Name: "created_at"}

	ast := query.InsertInto(mockTable{name: "test_events"}).
		Columns(nameCol, createdAtCol).
		Values(query.Param[string]("name"), query.Now()).
		Build()

	sql, params, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	_, err = db.Exec(sql, "Test Event")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Verify the timestamp was set
	var createdAt string
	err = db.QueryRow("SELECT created_at FROM test_events WHERE name = 'Test Event'").Scan(&createdAt)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	// Verify it's not empty
	if createdAt == "" {
		t.Error("created_at should not be empty")
	}
	t.Logf("created_at was set to: %s", createdAt)
}

func TestMySQLIntegration_Update(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_profiles;
		CREATE TABLE test_profiles (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_profiles`)

	// Insert test data
	_, err = db.Exec("INSERT INTO test_profiles (public_id, name, email) VALUES ('pid123', 'Alice', 'alice@example.com')")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build UPDATE
	nameCol := query.StringColumn{Table: "test_profiles", Name: "name"}
	emailCol := query.StringColumn{Table: "test_profiles", Name: "email"}
	publicIDCol := query.StringColumn{Table: "test_profiles", Name: "public_id"}

	ast := query.Update(mockTable{name: "test_profiles"}).
		Set(nameCol, query.Param[string]("name")).
		Set(emailCol, query.Param[string]("email")).
		Where(publicIDCol.Eq(query.Param[string]("public_id"))).
		Build()

	sql, params, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	_, err = db.Exec(sql, "Bob", "bob@example.com", "pid123")
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// Verify the update
	var name, email string
	err = db.QueryRow("SELECT name, email FROM test_profiles WHERE public_id = 'pid123'").Scan(&name, &email)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if name != "Bob" {
		t.Errorf("expected name = %q, got %q", "Bob", name)
	}
	if email != "bob@example.com" {
		t.Errorf("expected email = %q, got %q", "bob@example.com", email)
	}
}

func TestMySQLIntegration_Delete(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_items;
		CREATE TABLE test_items (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_items`)

	// Insert test data
	_, err = db.Exec("INSERT INTO test_items (public_id, name) VALUES ('item1', 'Item 1'), ('item2', 'Item 2')")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build DELETE
	publicIDCol := query.StringColumn{Table: "test_items", Name: "public_id"}

	ast := query.Delete(mockTable{name: "test_items"}).
		Where(publicIDCol.Eq(query.Param[string]("public_id"))).
		Build()

	sql, params, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	_, err = db.Exec(sql, "item1")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify item1 is deleted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_items WHERE public_id = 'item1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 items with public_id 'item1', got %d", count)
	}

	// Verify item2 still exists
	err = db.QueryRow("SELECT COUNT(*) FROM test_items WHERE public_id = 'item2'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 item with public_id 'item2', got %d", count)
	}
}

func TestMySQLIntegration_BooleanValues(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create table with boolean (TINYINT in MySQL)
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_flags;
		CREATE TABLE test_flags (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			active TINYINT(1) NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_flags`)

	// Insert with boolean
	_, err = db.Exec("INSERT INTO test_flags (active) VALUES (1)")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Query with boolean literal
	active := query.BoolColumn{Table: "test_flags", Name: "active"}

	ast := query.From(mockTable{name: "test_flags"}).
		Select(active).
		Where(active.Eq(query.Literal(true))).
		Build()

	sql, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	var gotActive bool
	err = db.QueryRow(sql).Scan(&gotActive)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if !gotActive {
		t.Error("expected active = true")
	}
}

func TestMySQLIntegration_SelectWithIn(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_orders;
		CREATE TABLE test_orders (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			status VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_orders`)

	// Insert test data
	_, err = db.Exec("INSERT INTO test_orders (status) VALUES ('pending'), ('processing'), ('shipped'), ('delivered')")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build query with IN
	statusCol := query.StringColumn{Table: "test_orders", Name: "status"}

	ast := query.From(mockTable{name: "test_orders"}).
		Select(statusCol).
		Where(statusCol.In("pending", "processing")).
		Build()

	sql, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute
	rows, err := db.Query(sql)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	var statuses []string
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		statuses = append(statuses, status)
	}

	if len(statuses) != 2 {
		t.Errorf("expected 2 orders, got %d: %v", len(statuses), statuses)
	}
}

func TestMySQLIntegration_JSONAggregation(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create tables
	_, err := db.Exec(`
		DROP TABLE IF EXISTS compile_books;
		DROP TABLE IF EXISTS compile_authors2;
		CREATE TABLE compile_authors2 (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
		CREATE TABLE compile_books (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			author_id BIGINT,
			title VARCHAR(255) NOT NULL,
			FOREIGN KEY (author_id) REFERENCES compile_authors2(id)
		);
		INSERT INTO compile_authors2 (id, name) VALUES (1, 'Alice');
		INSERT INTO compile_books (author_id, title) VALUES (1, 'Book 1'), (1, 'Book 2');
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS compile_books; DROP TABLE IF EXISTS compile_authors2`)

	// Build query with JSON aggregation
	authorID := query.Int64Column{Table: "compile_authors2", Name: "id"}
	authorName := query.StringColumn{Table: "compile_authors2", Name: "name"}
	bookID := query.Int64Column{Table: "compile_books", Name: "id"}
	bookTitle := query.StringColumn{Table: "compile_books", Name: "title"}
	bookAuthorID := query.Int64Column{Table: "compile_books", Name: "author_id"}

	ast := query.From(mockTable{name: "compile_authors2"}).
		Select(authorName).
		SelectJSONAgg("books", bookID, bookTitle).
		LeftJoin(mockTable{name: "compile_books"}).On(authorID.Eq(bookAuthorID)).
		GroupBy(authorName).
		Build()

	sql, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute and verify JSON is valid
	var name string
	var booksJSON []byte
	err = db.QueryRow(sql).Scan(&name, &booksJSON)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if name != "Alice" {
		t.Errorf("expected name = %q, got %q", "Alice", name)
	}

	var books []map[string]any
	err = json.Unmarshal(booksJSON, &books)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if len(books) != 2 {
		t.Errorf("expected 2 books, got %d: %v", len(books), books)
	}

	// Verify book structure
	for _, book := range books {
		if _, ok := book["id"]; !ok {
			t.Error("book should have 'id' key")
		}
		if _, ok := book["title"]; !ok {
			t.Error("book should have 'title' key")
		}
	}
}

func TestMySQLIntegration_ILike(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_people;
		CREATE TABLE test_people (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS test_people`)

	// Insert test data
	_, err = db.Exec("INSERT INTO test_people (name) VALUES ('John Smith'), ('JOHNNY DOE'), ('Jane Doe')")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build query with ILIKE (translated to LOWER() LIKE LOWER() for MySQL)
	nameCol := query.StringColumn{Table: "test_people", Name: "name"}

	ast := query.From(mockTable{name: "test_people"}).
		Select(nameCol).
		Where(nameCol.ILike("%john%")).
		Build()

	sql, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute
	rows, err := db.Query(sql)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		names = append(names, name)
	}

	// Should match "John Smith" and "JOHNNY DOE" (case-insensitive)
	if len(names) != 2 {
		t.Errorf("expected 2 matches for ILIKE %%john%%, got %d: %v", len(names), names)
	}
}

// =============================================================================
// Phase 7: Advanced SQL Features Integration Tests
// =============================================================================

func TestMySQLIntegration_CountAggregate(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table and insert data
	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_users;
		CREATE TABLE phase7_users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(255) NOT NULL,
			country VARCHAR(100) NOT NULL
		);
		INSERT INTO phase7_users (name, country) VALUES
			('Alice', 'USA'),
			('Bob', 'USA'),
			('Charlie', 'UK'),
			('David', 'UK'),
			('Eve', 'USA');
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS phase7_users`)

	// Test COUNT(*)
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "phase7_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}},
		},
	}

	sqlStr, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}
	t.Logf("COUNT SQL: %s", sqlStr)

	var count int
	err = db.QueryRow(sqlStr).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count = 5, got %d", count)
	}
}

func TestMySQLIntegration_SelectDistinct(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_users;
		CREATE TABLE phase7_users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(255) NOT NULL,
			country VARCHAR(100) NOT NULL
		);
		INSERT INTO phase7_users (name, country) VALUES
			('Alice', 'USA'),
			('Bob', 'USA'),
			('Charlie', 'UK'),
			('David', 'UK'),
			('Eve', 'USA');
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS phase7_users`)

	countryCol := query.StringColumn{Table: "phase7_users", Name: "country"}

	ast := &query.AST{
		Kind:      query.SelectQuery,
		Distinct:  true,
		FromTable: query.TableRef{Name: "phase7_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
		OrderBy: []query.OrderByExpr{
			{Expr: query.ColumnExpr{Column: countryCol}},
		},
	}

	sqlStr, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}
	t.Logf("DISTINCT SQL: %s", sqlStr)

	rows, err := db.Query(sqlStr)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	var countries []string
	for rows.Next() {
		var country string
		if err := rows.Scan(&country); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		countries = append(countries, country)
	}

	if len(countries) != 2 {
		t.Errorf("expected 2 distinct countries, got %d: %v", len(countries), countries)
	}
}

func TestMySQLIntegration_Union(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_active_users;
		DROP TABLE IF EXISTS phase7_archived_users;
		CREATE TABLE phase7_active_users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			email VARCHAR(255) NOT NULL
		);
		CREATE TABLE phase7_archived_users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			email VARCHAR(255) NOT NULL
		);
		INSERT INTO phase7_active_users (email) VALUES ('alice@test.com'), ('bob@test.com');
		INSERT INTO phase7_archived_users (email) VALUES ('charlie@test.com'), ('alice@test.com');
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS phase7_active_users; DROP TABLE IF EXISTS phase7_archived_users`)

	left := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "phase7_active_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "phase7_active_users", Name: "email"}}},
		},
	}
	right := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "phase7_archived_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.StringColumn{Table: "phase7_archived_users", Name: "email"}}},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		SetOp: &query.SetOperation{
			Left:  left,
			Op:    query.SetOpUnion,
			Right: right,
		},
	}

	sqlStr, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}
	t.Logf("UNION SQL: %s", sqlStr)

	rows, err := db.Query(sqlStr)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		emails = append(emails, email)
	}

	if len(emails) != 3 {
		t.Errorf("expected 3 unique emails (UNION deduplicates), got %d: %v", len(emails), emails)
	}
}

func TestMySQLIntegration_CTE(t *testing.T) {
	db := connectMySQL(t)
	if db == nil {
		return
	}
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_orders;
		CREATE TABLE phase7_orders (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			status VARCHAR(50) NOT NULL,
			amount DECIMAL(10,2) NOT NULL
		);
		INSERT INTO phase7_orders (status, amount) VALUES
			('pending', 100.00),
			('pending', 200.00),
			('completed', 150.00),
			('cancelled', 50.00);
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS phase7_orders`)

	cteQuery := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "phase7_orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.ColumnExpr{Column: query.Int64Column{Table: "phase7_orders", Name: "id"}}},
			{Expr: query.ColumnExpr{Column: query.DecimalColumn{Table: "phase7_orders", Name: "amount"}}},
		},
		Where: query.BinaryExpr{
			Left:  query.ColumnExpr{Column: query.StringColumn{Table: "phase7_orders", Name: "status"}},
			Op:    query.OpEq,
			Right: query.LiteralExpr{Value: "pending"},
		},
	}

	ast := &query.AST{
		Kind: query.SelectQuery,
		CTEs: []query.CTE{
			{Name: "pending_orders", Query: cteQuery},
		},
		FromTable: query.TableRef{Name: "pending_orders"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}, Alias: "cnt"},
			{Expr: query.AggregateExpr{Func: query.AggSum, Arg: query.ColumnExpr{Column: query.DecimalColumn{Table: "pending_orders", Name: "amount"}}}, Alias: "total"},
		},
	}

	sqlStr, _, err := CompileMySQL(ast)
	if err != nil {
		t.Fatalf("CompileMySQL failed: %v", err)
	}
	t.Logf("CTE SQL: %s", sqlStr)

	var cnt int
	var total float64
	err = db.QueryRow(sqlStr).Scan(&cnt, &total)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if cnt != 2 {
		t.Errorf("expected 2 pending orders, got %d", cnt)
	}
	if total != 300.00 {
		t.Errorf("expected total = 300.00, got %.2f", total)
	}
}
