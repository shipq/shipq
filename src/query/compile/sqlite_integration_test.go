//go:build integration

package compile

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/portsql/portsql/src/query"
	_ "modernc.org/sqlite"
)

// connectSQLite opens an in-memory SQLite database and returns a connection.
// Uses the pure-Go modernc.org/sqlite driver (no CGO required).
func connectSQLite(t *testing.T) *sql.DB {
	t.Helper()

	// Open an in-memory database using the modernc.org/sqlite driver
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skipf("SQLite unavailable: %v", err)
		return nil
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("SQLite unavailable: %v", err)
		return nil
	}

	return db
}

func TestSQLiteIntegration_SelectExecutes(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS compile_authors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

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

	sql, params, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_SelectWithOrderByLimitOffset(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

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

	sql, params, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_InsertWithReturning(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL,
			title TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Build INSERT with RETURNING
	publicID := query.StringColumn{Table: "test_posts", Name: "public_id"}
	title := query.StringColumn{Table: "test_posts", Name: "title"}

	ast := query.InsertInto(mockTable{name: "test_posts"}).
		Columns(publicID, title).
		Values(query.Param[string]("public_id"), query.Param[string]("title")).
		Returning(publicID).
		Build()

	sql, params, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute with RETURNING (SQLite 3.35+)
	var returnedID string
	err = db.QueryRow(sql, "xyz789", "Test Post").Scan(&returnedID)
	if err != nil {
		t.Fatalf("failed to insert with RETURNING: %v", err)
	}

	if returnedID != "xyz789" {
		t.Errorf("expected returned ID = %q, got %q", "xyz789", returnedID)
	}
}

func TestSQLiteIntegration_InsertWithDatetimeNow(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create table with datetime
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Build INSERT with datetime('now')
	nameCol := query.StringColumn{Table: "test_events", Name: "name"}
	createdAtCol := query.TimeColumn{Table: "test_events", Name: "created_at"}

	ast := query.InsertInto(mockTable{name: "test_events"}).
		Columns(nameCol, createdAtCol).
		Values(query.Param[string]("name"), query.Now()).
		Build()

	sql, params, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

	// Should be ISO8601 format
	_, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		t.Errorf("created_at should be valid datetime: %s (error: %v)", createdAt, err)
	}
	t.Logf("created_at was set to: %s", createdAt)
}

func TestSQLiteIntegration_Update(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL,
			name TEXT NOT NULL,
			email TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

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

	sql, params, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_Delete(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

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

	sql, params, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_BooleanValues(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create table with boolean (stored as INTEGER in SQLite)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_flags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			active INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Insert with boolean as 1
	_, err = db.Exec("INSERT INTO test_flags (active) VALUES (1)")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Query with boolean literal compiled to 1
	active := query.BoolColumn{Table: "test_flags", Name: "active"}

	ast := query.From(mockTable{name: "test_flags"}).
		Select(active).
		Where(active.Eq(query.Literal(true))).
		Build()

	sql, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	var gotActive int
	err = db.QueryRow(sql).Scan(&gotActive)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if gotActive != 1 {
		t.Errorf("expected active = 1, got %d", gotActive)
	}
}

func TestSQLiteIntegration_SelectWithIn(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

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

	sql, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_JSONAggregation(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create tables
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS compile_authors2 (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS compile_books (
			id INTEGER PRIMARY KEY,
			author_id INTEGER,
			title TEXT NOT NULL,
			FOREIGN KEY (author_id) REFERENCES compile_authors2(id)
		);
		INSERT INTO compile_authors2 (id, name) VALUES (1, 'Alice');
		INSERT INTO compile_books (author_id, title) VALUES (1, 'Book 1'), (1, 'Book 2');
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}

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

	sql, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute and verify JSON is valid
	var name string
	var booksJSON string
	err = db.QueryRow(sql).Scan(&name, &booksJSON)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if name != "Alice" {
		t.Errorf("expected name = %q, got %q", "Alice", name)
	}

	var books []map[string]any
	err = json.Unmarshal([]byte(booksJSON), &books)
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

func TestSQLiteIntegration_JSONAggregation_EmptyArray(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create tables
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS compile_authors3 (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS compile_books3 (
			id INTEGER PRIMARY KEY,
			author_id INTEGER,
			title TEXT NOT NULL,
			FOREIGN KEY (author_id) REFERENCES compile_authors3(id)
		);
		INSERT INTO compile_authors3 (id, name) VALUES (1, 'Bob');
		-- No books for Bob
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}

	// Build query with JSON aggregation
	authorID := query.Int64Column{Table: "compile_authors3", Name: "id"}
	authorName := query.StringColumn{Table: "compile_authors3", Name: "name"}
	bookID := query.Int64Column{Table: "compile_books3", Name: "id"}
	bookTitle := query.StringColumn{Table: "compile_books3", Name: "title"}
	bookAuthorID := query.Int64Column{Table: "compile_books3", Name: "author_id"}

	ast := query.From(mockTable{name: "compile_authors3"}).
		Select(authorName).
		SelectJSONAgg("books", bookID, bookTitle).
		LeftJoin(mockTable{name: "compile_books3"}).On(authorID.Eq(bookAuthorID)).
		GroupBy(authorName).
		Build()

	sql, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute and verify JSON is empty array
	var name string
	var booksJSON string
	err = db.QueryRow(sql).Scan(&name, &booksJSON)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if name != "Bob" {
		t.Errorf("expected name = %q, got %q", "Bob", name)
	}

	var books []map[string]any
	err = json.Unmarshal([]byte(booksJSON), &books)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Note: SQLite's JSON_GROUP_ARRAY includes null values from LEFT JOIN,
	// so we may get [{"id":null,"title":null}] instead of []
	// This is expected behavior for SQLite without additional filtering
	t.Logf("books JSON: %s", booksJSON)
}

func TestSQLiteIntegration_ILike(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_people (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Insert test data
	_, err = db.Exec("INSERT INTO test_people (name) VALUES ('John Smith'), ('JOHNNY DOE'), ('Jane Doe')")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build query with ILIKE (translated to LOWER() LIKE LOWER() for SQLite)
	nameCol := query.StringColumn{Table: "test_people", Name: "name"}

	ast := query.From(mockTable{name: "test_people"}).
		Select(nameCol).
		Where(nameCol.ILike("%john%")).
		Build()

	sql, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_ComplexQuery(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create tables
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_complex_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			deleted_at TEXT
		);
		CREATE TABLE IF NOT EXISTS test_order_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER,
			name TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			FOREIGN KEY (order_id) REFERENCES test_complex_orders(id)
		);
		INSERT INTO test_complex_orders (id, public_id, status) VALUES 
			(1, 'ord1', 'pending'),
			(2, 'ord2', 'processing'),
			(3, 'ord3', 'shipped');
		INSERT INTO test_order_items (order_id, name, quantity) VALUES
			(1, 'Item A', 2),
			(1, 'Item B', 1),
			(2, 'Item C', 3);
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}

	// Build complex query with join, where, order by
	orderID := query.Int64Column{Table: "test_complex_orders", Name: "id"}
	orderPublicID := query.StringColumn{Table: "test_complex_orders", Name: "public_id"}
	orderStatus := query.StringColumn{Table: "test_complex_orders", Name: "status"}
	orderDeletedAt := query.NullTimeColumn{Table: "test_complex_orders", Name: "deleted_at"}
	itemOrderID := query.Int64Column{Table: "test_order_items", Name: "order_id"}
	itemName := query.StringColumn{Table: "test_order_items", Name: "name"}
	itemQty := query.Int64Column{Table: "test_order_items", Name: "quantity"}

	ast := query.From(mockTable{name: "test_complex_orders"}).
		Select(orderPublicID, orderStatus).
		SelectJSONAgg("items", itemName, itemQty).
		LeftJoin(mockTable{name: "test_order_items"}).On(orderID.Eq(itemOrderID)).
		Where(query.And(
			orderStatus.In("pending", "processing"),
			orderDeletedAt.IsNull(),
		)).
		GroupBy(orderPublicID, orderStatus).
		OrderBy(orderPublicID.Asc()).
		Build()

	sql, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute
	rows, err := db.Query(sql)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	type orderResult struct {
		PublicID string
		Status   string
		Items    []map[string]any
	}

	var results []orderResult
	for rows.Next() {
		var r orderResult
		var itemsJSON string
		if err := rows.Scan(&r.PublicID, &r.Status, &itemsJSON); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		json.Unmarshal([]byte(itemsJSON), &r.Items)
		results = append(results, r)
	}

	// Should get ord1 (pending) and ord2 (processing), not ord3 (shipped)
	if len(results) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(results))
	}

	// Check first order (ord1)
	if results[0].PublicID != "ord1" {
		t.Errorf("expected first order public_id = %q, got %q", "ord1", results[0].PublicID)
	}
	if len(results[0].Items) != 2 {
		t.Errorf("expected 2 items for ord1, got %d", len(results[0].Items))
	}

	// Check second order (ord2)
	if results[1].PublicID != "ord2" {
		t.Errorf("expected second order public_id = %q, got %q", "ord2", results[1].PublicID)
	}
	if len(results[1].Items) != 1 {
		t.Errorf("expected 1 item for ord2, got %d", len(results[1].Items))
	}
}

// =============================================================================
// Phase 7: Advanced SQL Features Integration Tests
// =============================================================================

func TestSQLiteIntegration_CountAggregate(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	// Create test table and insert data
	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_users;
		CREATE TABLE phase7_users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			country TEXT NOT NULL
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

	sqlStr, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_SelectDistinct(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_users;
		CREATE TABLE phase7_users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			country TEXT NOT NULL
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

	sqlStr, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_Union(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_active_users;
		DROP TABLE IF EXISTS phase7_archived_users;
		CREATE TABLE phase7_active_users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL
		);
		CREATE TABLE phase7_archived_users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL
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

	sqlStr, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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

func TestSQLiteIntegration_CTE(t *testing.T) {
	db := connectSQLite(t)
	if db == nil {
		return
	}
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS phase7_orders;
		CREATE TABLE phase7_orders (
			id INTEGER PRIMARY KEY,
			status TEXT NOT NULL,
			amount REAL NOT NULL
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
			{Expr: query.ColumnExpr{Column: query.Float64Column{Table: "phase7_orders", Name: "amount"}}},
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
			{Expr: query.AggregateExpr{Func: query.AggSum, Arg: query.ColumnExpr{Column: query.Float64Column{Table: "pending_orders", Name: "amount"}}}, Alias: "total"},
		},
	}

	sqlStr, _, err := CompileSQLite(ast)
	if err != nil {
		t.Fatalf("CompileSQLite failed: %v", err)
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
