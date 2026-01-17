//go:build integration

package codegen

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	_ "modernc.org/sqlite"
)

// connectSQLite opens an in-memory SQLite database for testing.
func connectSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skipf("SQLite unavailable: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("SQLite unavailable: %v", err)
		return nil
	}
	return db
}

// setupTestSchema creates test tables for relation testing
func setupTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	// Create categories table
	_, err := db.Exec(`
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY,
			public_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create categories table: %v", err)
	}

	// Create pets table
	_, err = db.Exec(`
		CREATE TABLE pets (
			id INTEGER PRIMARY KEY,
			public_id TEXT NOT NULL UNIQUE,
			category_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create pets table: %v", err)
	}

	// Create tags table
	_, err = db.Exec(`
		CREATE TABLE tags (
			id INTEGER PRIMARY KEY,
			public_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create tags table: %v", err)
	}

	// Create pet_tags junction table
	_, err = db.Exec(`
		CREATE TABLE pet_tags (
			pet_id INTEGER NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (pet_id, tag_id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create pet_tags table: %v", err)
	}
}

// insertTestData inserts sample data for relation testing
func insertTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	// Insert categories
	_, err := db.Exec(`
		INSERT INTO categories (id, public_id, name, created_at, updated_at) VALUES
		(1, 'cat-1', 'Dogs', datetime('now'), datetime('now')),
		(2, 'cat-2', 'Cats', datetime('now'), datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert categories: %v", err)
	}

	// Insert pets
	_, err = db.Exec(`
		INSERT INTO pets (id, public_id, category_id, name, status, created_at, updated_at) VALUES
		(1, 'pet-1', 1, 'Buddy', 'available', datetime('now'), datetime('now')),
		(2, 'pet-2', 1, 'Max', 'pending', datetime('now'), datetime('now')),
		(3, 'pet-3', 2, 'Whiskers', 'available', datetime('now'), datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert pets: %v", err)
	}

	// Insert tags
	_, err = db.Exec(`
		INSERT INTO tags (id, public_id, name, created_at, updated_at) VALUES
		(1, 'tag-1', 'Friendly', datetime('now'), datetime('now')),
		(2, 'tag-2', 'Trained', datetime('now'), datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert tags: %v", err)
	}

	// Insert pet_tags
	_, err = db.Exec(`
		INSERT INTO pet_tags (pet_id, tag_id) VALUES
		(1, 1), (1, 2),
		(2, 1)
	`)
	if err != nil {
		t.Fatalf("failed to insert pet_tags: %v", err)
	}
}

// createTestPlan creates a migration plan matching the test schema
func createTestPlan() *migrate.MigrationPlan {
	plan := migrate.NewPlan()
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "name", Type: ddl.StringType},
			{Name: "status", Type: ddl.StringType, Nullable: true},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	plan.Schema.Tables["tags"] = ddl.Table{
		Name: "tags",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	plan.Schema.Tables["pet_tags"] = ddl.Table{
		Name:            "pet_tags",
		IsJunctionTable: true,
		Columns: []ddl.ColumnDefinition{
			{Name: "pet_id", Type: ddl.BigintType, References: "pets"},
			{Name: "tag_id", Type: ddl.BigintType, References: "tags"},
		},
	}
	return plan
}

func TestRelationQuery_HasMany_SQLite(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	setupTestSchema(t, db)
	insertTestData(t, db)

	// Create the migration plan and scan relations
	plan := createTestPlan()
	relations := ScanRelations(plan)

	hasManyRel := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasManyRel == nil {
		t.Fatal("expected HasMany relation from categories to pets")
	}

	// Generate and execute the SQL
	sqlStr := GenerateRelationSQL(plan, *hasManyRel, SQLDialectSQLite)
	t.Logf("HasMany SQL: %s", sqlStr)

	// Execute query for category_id = 'cat-1' (Dogs)
	row := db.QueryRow(sqlStr, "cat-1")

	var publicID, name string
	var petsJSON string
	var createdAt, updatedAt string

	err := row.Scan(&publicID, &name, &createdAt, &updatedAt, &petsJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify the result
	if publicID != "cat-1" {
		t.Errorf("expected public_id='cat-1', got %q", publicID)
	}
	if name != "Dogs" {
		t.Errorf("expected name='Dogs', got %q", name)
	}

	// Parse and verify the pets JSON array
	var pets []map[string]interface{}
	if err := json.Unmarshal([]byte(petsJSON), &pets); err != nil {
		t.Fatalf("failed to parse pets JSON: %v\nJSON: %s", err, petsJSON)
	}

	if len(pets) != 2 {
		t.Errorf("expected 2 pets in Dogs category, got %d", len(pets))
	}
}

func TestRelationQuery_BelongsTo_SQLite(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	setupTestSchema(t, db)
	insertTestData(t, db)

	plan := createTestPlan()
	relations := ScanRelations(plan)

	belongsToRel := findRelation(relations, "pets", "categories", RelationBelongsTo)
	if belongsToRel == nil {
		t.Fatal("expected BelongsTo relation from pets to categories")
	}

	sqlStr := GenerateRelationSQL(plan, *belongsToRel, SQLDialectSQLite)
	t.Logf("BelongsTo SQL: %s", sqlStr)

	// Execute query for pet_id = 'pet-1' (Buddy)
	row := db.QueryRow(sqlStr, "pet-1")

	var publicID, name string
	var categoryID int64
	var status *string
	var createdAt, updatedAt string
	var categoryJSON string

	err := row.Scan(&publicID, &categoryID, &name, &status, &createdAt, &updatedAt, &categoryJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify the result
	if publicID != "pet-1" {
		t.Errorf("expected public_id='pet-1', got %q", publicID)
	}
	if name != "Buddy" {
		t.Errorf("expected name='Buddy', got %q", name)
	}

	// Parse and verify the category JSON object
	var category map[string]interface{}
	if err := json.Unmarshal([]byte(categoryJSON), &category); err != nil {
		t.Fatalf("failed to parse category JSON: %v\nJSON: %s", err, categoryJSON)
	}

	if category["name"] != "Dogs" {
		t.Errorf("expected category name='Dogs', got %v", category["name"])
	}
}

func TestRelationQuery_ManyToMany_SQLite(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	setupTestSchema(t, db)
	insertTestData(t, db)

	plan := createTestPlan()
	relations := ScanRelations(plan)

	m2mRel := findRelation(relations, "pets", "tags", RelationManyToMany)
	if m2mRel == nil {
		t.Fatal("expected ManyToMany relation from pets to tags")
	}

	sqlStr := GenerateRelationSQL(plan, *m2mRel, SQLDialectSQLite)
	t.Logf("ManyToMany SQL: %s", sqlStr)

	// Execute query for pet_id = 'pet-1' (Buddy - has 2 tags)
	row := db.QueryRow(sqlStr, "pet-1")

	// The query returns all ResultColumns from pets, then the tags JSON
	var publicID string
	var categoryID int64
	var name string
	var status *string
	var createdAt, updatedAt string
	var tagsJSON string

	err := row.Scan(&publicID, &categoryID, &name, &status, &createdAt, &updatedAt, &tagsJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify the result
	if publicID != "pet-1" {
		t.Errorf("expected public_id='pet-1', got %q", publicID)
	}
	if name != "Buddy" {
		t.Errorf("expected name='Buddy', got %q", name)
	}

	// Parse and verify the tags JSON array
	var tags []map[string]interface{}
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		t.Fatalf("failed to parse tags JSON: %v\nJSON: %s", err, tagsJSON)
	}

	if len(tags) != 2 {
		t.Errorf("expected 2 tags for Buddy, got %d", len(tags))
	}
}

func TestRelationQuery_HasMany_EmptyResult_SQLite(t *testing.T) {
	db := connectSQLite(t)
	defer db.Close()

	setupTestSchema(t, db)

	// Insert a category with NO pets
	_, err := db.Exec(`
		INSERT INTO categories (id, public_id, name, created_at, updated_at) VALUES
		(99, 'cat-99', 'Birds', datetime('now'), datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert category: %v", err)
	}

	plan := createTestPlan()
	relations := ScanRelations(plan)

	hasManyRel := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasManyRel == nil {
		t.Fatal("expected HasMany relation from categories to pets")
	}

	sqlStr := GenerateRelationSQL(plan, *hasManyRel, SQLDialectSQLite)

	// Execute query for category with no pets
	row := db.QueryRow(sqlStr, "cat-99")

	var publicID, name string
	var petsJSON string
	var createdAt, updatedAt string

	err = row.Scan(&publicID, &name, &createdAt, &updatedAt, &petsJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify the JSON is an empty array (not [null])
	// The FILTER clause in the SQL ensures NULL rows from LEFT JOIN are excluded
	var pets []map[string]interface{}
	if err := json.Unmarshal([]byte(petsJSON), &pets); err != nil {
		t.Fatalf("failed to parse pets JSON: %v\nJSON: %s", err, petsJSON)
	}

	if len(pets) != 0 {
		t.Errorf("expected empty pets array, got %d items: %v", len(pets), pets)
	}
}
