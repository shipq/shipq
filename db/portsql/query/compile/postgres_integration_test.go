//go:build integration

package compile

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shipq/shipq/db/portsql/query"
)

// mockTable implements query.Table for testing
type mockTable struct {
	name string
}

func (m mockTable) TableName() string { return m.name }

// connectPostgres attempts to connect to PostgreSQL and returns a connection.
// Returns nil and skips the test if PostgreSQL is unavailable.
func connectPostgres(t *testing.T) *pgx.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect via Unix socket at /tmp/.s.PGSQL.5432, user "postgres", database "postgres"
	connString := "host=/tmp user=postgres database=postgres"
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		t.Skipf("PostgreSQL unavailable: %v. Please see the README for instructions about how to start all databases.", err)
		return nil
	}

	return conn
}

// setupTestTable creates a test table and returns a cleanup function
func setupTestTable(t *testing.T, conn *pgx.Conn, ddl string) func() {
	t.Helper()

	ctx := context.Background()
	_, err := conn.Exec(ctx, ddl)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	return func() {
		// Cleanup is handled by dropping the table
	}
}

func TestPostgresIntegration_SelectExecutes(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS compile_authors CASCADE;
		CREATE TABLE compile_authors (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS compile_authors`)

	// Insert test data
	_, err = conn.Exec(ctx, `INSERT INTO compile_authors (public_id, name) VALUES ('abc123', 'Alice')`)
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

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute the query
	row := conn.QueryRow(ctx, sql, "abc123")

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

func TestPostgresIntegration_SelectWithOrderByLimitOffset(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_users;
		CREATE TABLE test_users (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_users`)

	// Insert test data
	_, err = conn.Exec(ctx, `
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

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute: get 2 users starting from offset 1
	rows, err := conn.Query(ctx, sql, 2, 1)
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

func TestPostgresIntegration_InsertWithReturning(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_posts;
		CREATE TABLE test_posts (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			title VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_posts`)

	// Build INSERT with RETURNING
	publicID := query.StringColumn{Table: "test_posts", Name: "public_id"}
	title := query.StringColumn{Table: "test_posts", Name: "title"}

	ast := query.InsertInto(mockTable{name: "test_posts"}).
		Columns(publicID, title).
		Values(query.Param[string]("public_id"), query.Param[string]("title")).
		Returning(publicID).
		Build()

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute with RETURNING
	var returnedID string
	err = conn.QueryRow(ctx, sql, "xyz789", "Test Post").Scan(&returnedID)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	if returnedID != "xyz789" {
		t.Errorf("expected returned ID = %q, got %q", "xyz789", returnedID)
	}
}

func TestPostgresIntegration_InsertWithNow(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_events;
		CREATE TABLE test_events (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_events`)

	// Build INSERT with NOW()
	nameCol := query.StringColumn{Table: "test_events", Name: "name"}
	createdAtCol := query.TimeColumn{Table: "test_events", Name: "created_at"}

	ast := query.InsertInto(mockTable{name: "test_events"}).
		Columns(nameCol, createdAtCol).
		Values(query.Param[string]("name"), query.Now()).
		Build()

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	_, err = conn.Exec(ctx, sql, "Test Event")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Verify the timestamp was set (just check it's non-zero)
	var createdAt time.Time
	err = conn.QueryRow(ctx, `SELECT created_at FROM test_events WHERE name = 'Test Event'`).Scan(&createdAt)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	// Verify the timestamp is non-zero (NOW() was executed)
	if createdAt.IsZero() {
		t.Error("created_at should not be zero")
	}
	t.Logf("created_at was set to: %v", createdAt)
}

func TestPostgresIntegration_Update(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_profiles;
		CREATE TABLE test_profiles (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_profiles`)

	// Insert test data
	_, err = conn.Exec(ctx, `INSERT INTO test_profiles (public_id, name, email) VALUES ('pid123', 'Alice', 'alice@example.com')`)
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

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	_, err = conn.Exec(ctx, sql, "Bob", "bob@example.com", "pid123")
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// Verify the update
	var name, email string
	err = conn.QueryRow(ctx, `SELECT name, email FROM test_profiles WHERE public_id = 'pid123'`).Scan(&name, &email)
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

func TestPostgresIntegration_Delete(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_items;
		CREATE TABLE test_items (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_items`)

	// Insert test data
	_, err = conn.Exec(ctx, `INSERT INTO test_items (public_id, name) VALUES ('item1', 'Item 1'), ('item2', 'Item 2')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build DELETE
	publicIDCol := query.StringColumn{Table: "test_items", Name: "public_id"}

	ast := query.Delete(mockTable{name: "test_items"}).
		Where(publicIDCol.Eq(query.Param[string]("public_id"))).
		Build()

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	_, err = conn.Exec(ctx, sql, "item1")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify item1 is deleted
	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM test_items WHERE public_id = 'item1'`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 items with public_id 'item1', got %d", count)
	}

	// Verify item2 still exists
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM test_items WHERE public_id = 'item2'`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 item with public_id 'item2', got %d", count)
	}
}

func TestPostgresIntegration_SelectWithILike(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_people;
		CREATE TABLE test_people (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_people`)

	// Insert test data
	_, err = conn.Exec(ctx, `INSERT INTO test_people (name) VALUES ('John Smith'), ('JOHNNY DOE'), ('Jane Doe')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build query with ILIKE
	nameCol := query.StringColumn{Table: "test_people", Name: "name"}

	ast := query.From(mockTable{name: "test_people"}).
		Select(nameCol).
		Where(nameCol.ILike("%john%")).
		Build()

	sql, params, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)
	t.Logf("Params: %v", params)

	// Execute
	rows, err := conn.Query(ctx, sql)
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

func TestPostgresIntegration_SelectWithIn(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_orders;
		CREATE TABLE test_orders (
			id BIGSERIAL PRIMARY KEY,
			status VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_orders`)

	// Insert test data
	_, err = conn.Exec(ctx, `INSERT INTO test_orders (status) VALUES ('pending'), ('processing'), ('shipped'), ('delivered')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Build query with IN
	statusCol := query.StringColumn{Table: "test_orders", Name: "status"}

	ast := query.From(mockTable{name: "test_orders"}).
		Select(statusCol).
		Where(statusCol.In("pending", "processing")).
		Build()

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute
	rows, err := conn.Query(ctx, sql)
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

func TestPostgresIntegration_JSONAggregation(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create tables
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS compile_books CASCADE;
		DROP TABLE IF EXISTS compile_authors2 CASCADE;
		CREATE TABLE compile_authors2 (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
		CREATE TABLE compile_books (
			id BIGSERIAL PRIMARY KEY,
			author_id BIGINT REFERENCES compile_authors2(id),
			title VARCHAR(255) NOT NULL
		);
		INSERT INTO compile_authors2 (id, name) VALUES (1, 'Alice');
		INSERT INTO compile_books (author_id, title) VALUES (1, 'Book 1'), (1, 'Book 2');
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS compile_books; DROP TABLE IF EXISTS compile_authors2`)

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

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute and verify JSON is valid
	var name string
	var booksJSON []byte
	err = conn.QueryRow(ctx, sql).Scan(&name, &booksJSON)
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

func TestPostgresIntegration_JSONAggregation_EmptyArray(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create tables
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS compile_books3 CASCADE;
		DROP TABLE IF EXISTS compile_authors3 CASCADE;
		CREATE TABLE compile_authors3 (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		);
		CREATE TABLE compile_books3 (
			id BIGSERIAL PRIMARY KEY,
			author_id BIGINT REFERENCES compile_authors3(id),
			title VARCHAR(255) NOT NULL
		);
		INSERT INTO compile_authors3 (id, name) VALUES (1, 'Bob');
		-- No books for Bob
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS compile_books3; DROP TABLE IF EXISTS compile_authors3`)

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

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute and verify JSON is empty array
	var name string
	var booksJSON []byte
	err = conn.QueryRow(ctx, sql).Scan(&name, &booksJSON)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if name != "Bob" {
		t.Errorf("expected name = %q, got %q", "Bob", name)
	}

	var books []map[string]any
	err = json.Unmarshal(booksJSON, &books)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Should be empty array, not null
	if books == nil {
		t.Error("books should be empty array, not nil")
	}
	if len(books) != 0 {
		t.Errorf("expected 0 books, got %d", len(books))
	}
}

func TestPostgresIntegration_ComplexQuery(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create tables
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS test_order_items CASCADE;
		DROP TABLE IF EXISTS test_complex_orders CASCADE;
		CREATE TABLE test_complex_orders (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL,
			status VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP
		);
		CREATE TABLE test_order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT REFERENCES test_complex_orders(id),
			name VARCHAR(255) NOT NULL,
			quantity INT NOT NULL
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
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS test_order_items; DROP TABLE IF EXISTS test_complex_orders`)

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

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	t.Logf("Compiled SQL: %s", sql)

	// Execute
	rows, err := conn.Query(ctx, sql)
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
		var itemsJSON []byte
		if err := rows.Scan(&r.PublicID, &r.Status, &itemsJSON); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		json.Unmarshal(itemsJSON, &r.Items)
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

func TestPostgresIntegration_CountAggregate(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Create test table and insert data
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS phase7_users CASCADE;
		CREATE TABLE phase7_users (
			id BIGSERIAL PRIMARY KEY,
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
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS phase7_users`)

	// Test COUNT(*)
	ast := &query.AST{
		Kind:      query.SelectQuery,
		FromTable: query.TableRef{Name: "phase7_users"},
		SelectCols: []query.SelectExpr{
			{Expr: query.AggregateExpr{Func: query.AggCount, Arg: nil}},
		},
	}

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	t.Logf("COUNT SQL: %s", sql)

	var count int
	err = conn.QueryRow(ctx, sql).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count = 5, got %d", count)
	}
}

func TestPostgresIntegration_SelectDistinct(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	// Reuse table from above or create
	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS phase7_users CASCADE;
		CREATE TABLE phase7_users (
			id BIGSERIAL PRIMARY KEY,
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
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS phase7_users`)

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

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	t.Logf("DISTINCT SQL: %s", sql)

	rows, err := conn.Query(ctx, sql)
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

	// Should get only 2 unique countries: UK and USA (sorted)
	if len(countries) != 2 {
		t.Errorf("expected 2 distinct countries, got %d: %v", len(countries), countries)
	}
}

func TestPostgresIntegration_Union(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS phase7_active_users CASCADE;
		DROP TABLE IF EXISTS phase7_archived_users CASCADE;
		CREATE TABLE phase7_active_users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL
		);
		CREATE TABLE phase7_archived_users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL
		);
		INSERT INTO phase7_active_users (email) VALUES ('alice@test.com'), ('bob@test.com');
		INSERT INTO phase7_archived_users (email) VALUES ('charlie@test.com'), ('alice@test.com');
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS phase7_active_users; DROP TABLE IF EXISTS phase7_archived_users`)

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

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	t.Logf("UNION SQL: %s", sql)

	rows, err := conn.Query(ctx, sql)
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

	// UNION removes duplicates: alice@test.com, bob@test.com, charlie@test.com = 3
	if len(emails) != 3 {
		t.Errorf("expected 3 unique emails (UNION deduplicates), got %d: %v", len(emails), emails)
	}
}

func TestPostgresIntegration_CTE(t *testing.T) {
	conn := connectPostgres(t)
	if conn == nil {
		return
	}
	defer conn.Close(context.Background())

	ctx := context.Background()

	_, err := conn.Exec(ctx, `
		DROP TABLE IF EXISTS phase7_orders CASCADE;
		CREATE TABLE phase7_orders (
			id BIGSERIAL PRIMARY KEY,
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
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS phase7_orders`)

	// CTE: SELECT pending orders, then count them
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

	sql, _, err := NewCompiler(Postgres).Compile(ast)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	t.Logf("CTE SQL: %s", sql)

	var cnt int
	var total float64
	err = conn.QueryRow(ctx, sql).Scan(&cnt, &total)
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
