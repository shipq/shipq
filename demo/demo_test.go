package demo_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/portsql/portsql/demo/queries"
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

	// Test 3: Verify generated SQL queries are valid
	t.Run("GeneratedQueriesAreValid", func(t *testing.T) {
		// Test GetPetById query compiles
		_, err := db.PrepareContext(ctx, queries.GetPetByIdSQL)
		if err != nil {
			t.Errorf("GetPetByIdSQL is invalid: %v", err)
		}

		// Test FindPetsByStatus query compiles
		_, err = db.PrepareContext(ctx, queries.FindPetsByStatusSQL)
		if err != nil {
			t.Errorf("FindPetsByStatusSQL is invalid: %v", err)
		}

		// Test GetUserByUsername query compiles
		_, err = db.PrepareContext(ctx, queries.GetUserByUsernameSQL)
		if err != nil {
			t.Errorf("GetUserByUsernameSQL is invalid: %v", err)
		}

		// Test GetOrderById query compiles
		_, err = db.PrepareContext(ctx, queries.GetOrderByIdSQL)
		if err != nil {
			t.Errorf("GetOrderByIdSQL is invalid: %v", err)
		}

		// Test ListPetsWithCategory query compiles
		_, err = db.PrepareContext(ctx, queries.ListPetsWithCategorySQL)
		if err != nil {
			t.Errorf("ListPetsWithCategorySQL is invalid: %v", err)
		}
	})

	// Test 4: Insert and query data using QueryRunner methods
	t.Run("InsertAndQueryDataWithRunner", func(t *testing.T) {
		// Create QueryRunner for SQLite
		runner := queries.NewQueryRunner(db, queries.SQLite)

		// Insert a category
		_, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO categories (id, name) VALUES (1, 'Dogs')`,
		)
		if err != nil {
			t.Fatalf("failed to insert category: %v", err)
		}

		// Insert a pet
		_, err = db.ExecContext(ctx,
			`INSERT OR IGNORE INTO pets (id, category_id, name, photo_urls, status) VALUES (1, 1, 'Buddy', '["http://example.com/buddy.jpg"]', 'available')`,
		)
		if err != nil {
			t.Fatalf("failed to insert pet: %v", err)
		}

		// Query using DefineMany query (FindPetsByStatus doesn't include photo_urls which has JSON scanning issues)
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

	// Test 5: Test DefineOne returns nil for non-existent row
	t.Run("OneReturnsNilForMissing", func(t *testing.T) {
		runner := queries.NewQueryRunner(db, queries.SQLite)

		// Query for a pet that doesn't exist
		result, err := runner.GetPetById(ctx, queries.GetPetByIdParams{Id: 99999})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for non-existent pet, got %+v", result)
		}
	})

	// Test 6: Test DefineMany returns slice
	t.Run("ManyReturnsSlice", func(t *testing.T) {
		runner := queries.NewQueryRunner(db, queries.SQLite)

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
		runner := queries.NewQueryRunner(db, queries.SQLite)

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
