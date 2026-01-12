package demo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/portsql/portsql/demo/queries"
	"github.com/portsql/portsql/demo/queries/sqlite"
)

// TestDemoWorkflow validates the CLI-driven workflow produces working code.
func TestDemoWorkflow(t *testing.T) {
	// Get the demo directory path
	demoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	dbPath := filepath.Join(demoDir, "petstore.db")

	// Verify the database was created by migrate up
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("petstore.db not found - run '../app migrate up' first")
	}

	// Open the database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test 1: Verify tables exist
	t.Run("TablesExist", func(t *testing.T) {
		tables := []string{"categories", "tags", "users", "pets", "orders", "_portsql_migrations"}
		for _, table := range tables {
			var name string
			err := db.QueryRowContext(ctx,
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				table,
			).Scan(&name)
			if err != nil {
				t.Errorf("table %q not found: %v", table, err)
			}
		}
	})

	// Test 2: Verify users table has standard columns from AddTable
	t.Run("UsersHasStandardColumns", func(t *testing.T) {
		standardColumns := []string{"id", "public_id", "created_at", "deleted_at", "updated_at"}
		for _, col := range standardColumns {
			var cid int
			err := db.QueryRowContext(ctx,
				"SELECT cid FROM pragma_table_info('users') WHERE name=?",
				col,
			).Scan(&cid)
			if err != nil {
				t.Errorf("users table missing standard column %q: %v", col, err)
			}
		}
	})

	// Test 3: Verify QueryRunner works (methods are generated correctly)
	t.Run("QueryRunnerMethodsWork", func(t *testing.T) {
		runner := sqlite.NewQueryRunner(db)

		// Test FindPetsByStatus (returns nil slice for no matching data, which is fine)
		results, err := runner.FindPetsByStatus(ctx, queries.FindPetsByStatusParams{Status: "nonexistent"})
		if err != nil {
			t.Errorf("FindPetsByStatus failed: %v", err)
		}
		// Nil slice is fine for no results (common Go pattern)
		if len(results) != 0 {
			t.Errorf("expected no results for nonexistent status, got %d", len(results))
		}

		// Test GetPetById (returns nil for missing pet)
		result, err := runner.GetPetById(ctx, queries.GetPetByIdParams{Id: 99999})
		if err != nil {
			t.Errorf("GetPetById failed: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for non-existent pet, got %+v", result)
		}
	})

	// Test 4: Insert and query data using QueryRunner methods
	t.Run("InsertAndQueryDataWithRunner", func(t *testing.T) {
		// Create QueryRunner for SQLite (new simplified API - no dialect parameter!)
		runner := sqlite.NewQueryRunner(db)

		// Insert a category
		_, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO categories (id, name) VALUES (1, 'Dogs')`,
		)
		if err != nil {
			t.Fatalf("failed to insert category: %v", err)
		}

		// Insert a pet with JSON photo_urls
		_, err = db.ExecContext(ctx,
			`INSERT OR IGNORE INTO pets (id, category_id, name, photo_urls, status) VALUES (1, 1, 'Buddy', '["http://example.com/buddy.jpg"]', 'available')`,
		)
		if err != nil {
			t.Fatalf("failed to insert pet: %v", err)
		}

		// Query using DefineMany query
		results, err := runner.FindPetsByStatus(ctx, queries.FindPetsByStatusParams{Status: "available"})
		if err != nil {
			t.Fatalf("FindPetsByStatus query failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least one result")
		}

		// Find Buddy in the results
		found := false
		for _, result := range results {
			if result.Name == "Buddy" && result.CategoryId == 1 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find Buddy with category_id 1 in results")
		}
	})

	// Test 5: Test GetPetById with JSON columns works on SQLite
	t.Run("JSONColumnScanningWorks", func(t *testing.T) {
		runner := sqlite.NewQueryRunner(db)

		// Query for a pet with JSON photo_urls - this was previously broken on SQLite!
		result, err := runner.GetPetById(ctx, queries.GetPetByIdParams{Id: 1})
		if err != nil {
			t.Fatalf("GetPetById failed: %v", err)
		}
		if result == nil {
			t.Fatal("expected result for pet ID 1, got nil")
		}

		// Verify JSON photo_urls was scanned correctly
		if len(result.PhotoUrls) == 0 {
			t.Error("expected photo_urls to be non-empty JSON")
		}

		// Verify it's valid JSON
		var urls []string
		if err := json.Unmarshal(result.PhotoUrls, &urls); err != nil {
			t.Errorf("photo_urls is not valid JSON: %v", err)
		}
		if len(urls) == 0 || urls[0] != "http://example.com/buddy.jpg" {
			t.Errorf("unexpected photo_urls content: %v", urls)
		}
	})

	// Test 6: Test DefineMany returns slice
	t.Run("ManyReturnsSlice", func(t *testing.T) {
		runner := sqlite.NewQueryRunner(db)

		// Insert another pet
		_, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO pets (id, category_id, name, photo_urls, status) VALUES (2, 1, 'Max', '[]', 'available')`,
		)
		if err != nil {
			t.Fatalf("failed to insert pet: %v", err)
		}

		// Query using DefineMany query
		results, err := runner.FindPetsByStatus(ctx, queries.FindPetsByStatusParams{Status: "available"})
		if err != nil {
			t.Fatalf("FindPetsByStatus query failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("expected at least 1 result, got %d", len(results))
		}
	})

	// Test 7: Test QueryRunner with JOIN query
	t.Run("JoinQueryWithRunner", func(t *testing.T) {
		runner := sqlite.NewQueryRunner(db)

		// Query pets with their category names
		results, err := runner.ListPetsWithCategory(ctx)
		if err != nil {
			t.Fatalf("ListPetsWithCategory query failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("expected at least 1 result, got %d", len(results))
		}

		// Verify we got the category name from the JOIN
		for _, r := range results {
			if r.CategoryName == "" {
				t.Error("expected non-empty category name from JOIN")
			}
		}
	})
}

// TestMigrateReset validates the migrate reset command works on localhost.
func TestMigrateReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping migrate reset test in short mode")
	}

	// Get the demo directory path
	demoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	appPath := filepath.Join(demoDir, "..", "app")

	// Run migrate reset
	cmd := exec.Command(appPath, "migrate", "reset")
	cmd.Dir = demoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate reset failed: %v\nOutput: %s", err, output)
	}

	// Verify tables still exist after reset
	dbPath := filepath.Join(demoDir, "petstore.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database after reset: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count tables: %v", err)
	}

	// Should have 6 tables: categories, tags, users, pets, orders, _portsql_migrations
	if count != 6 {
		t.Errorf("expected 6 tables after reset, got %d", count)
	}
}
