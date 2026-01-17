//go:build integration

package codegen

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// dbConnections holds connections to all three databases for cross-db tests
type dbConnections struct {
	postgres *pgx.Conn
	mysql    *sql.DB
	sqlite   *sql.DB
}

// connectAllDatabases attempts to connect to all three databases.
// Skips the test if any database is unavailable.
func connectAllDBs(t *testing.T) *dbConnections {
	t.Helper()

	postgres := connectPg(t)
	mysql := connectMy(t)
	sqlite := connectSq(t)

	return &dbConnections{
		postgres: postgres,
		mysql:    mysql,
		sqlite:   sqlite,
	}
}

func connectPg(t *testing.T) *pgx.Conn {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Skipf("PostgreSQL unavailable: %v", err)
		return nil
	}
	return conn
}

func connectMy(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_URL")
	if dsn == "" {
		dsn = "root:root@tcp(localhost:3306)/test?parseTime=true"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("MySQL unavailable: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("MySQL unavailable: %v", err)
		return nil
	}
	return db
}

func connectSq(t *testing.T) *sql.DB {
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

// closeAll closes all database connections
func (dbs *dbConnections) closeAll() {
	if dbs.postgres != nil {
		dbs.postgres.Close(context.Background())
	}
	if dbs.mysql != nil {
		dbs.mysql.Close()
	}
	if dbs.sqlite != nil {
		dbs.sqlite.Close()
	}
}

// setupAllDBsSchema creates the test tables in all databases
func setupAllDBsSchema(t *testing.T, dbs *dbConnections) {
	t.Helper()

	// PostgreSQL
	_, err := dbs.postgres.Exec(context.Background(), `
		DROP TABLE IF EXISTS pet_tags CASCADE;
		DROP TABLE IF EXISTS pets CASCADE;
		DROP TABLE IF EXISTS tags CASCADE;
		DROP TABLE IF EXISTS categories CASCADE;
		
		CREATE TABLE categories (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		
		CREATE TABLE pets (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			category_id BIGINT NOT NULL,
			name VARCHAR(255) NOT NULL,
			status VARCHAR(255),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		
		CREATE TABLE tags (
			id BIGSERIAL PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		
		CREATE TABLE pet_tags (
			pet_id BIGINT NOT NULL,
			tag_id BIGINT NOT NULL,
			PRIMARY KEY (pet_id, tag_id)
		);
	`)
	if err != nil {
		t.Fatalf("failed to setup PostgreSQL schema: %v", err)
	}

	// MySQL
	_, err = dbs.mysql.Exec(`DROP TABLE IF EXISTS pet_tags, pets, tags, categories`)
	if err != nil {
		t.Fatalf("failed to drop MySQL tables: %v", err)
	}
	_, err = dbs.mysql.Exec(`
		CREATE TABLE categories (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create MySQL categories table: %v", err)
	}
	_, err = dbs.mysql.Exec(`
		CREATE TABLE pets (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			category_id BIGINT NOT NULL,
			name VARCHAR(255) NOT NULL,
			status VARCHAR(255),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create MySQL pets table: %v", err)
	}
	_, err = dbs.mysql.Exec(`
		CREATE TABLE tags (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			public_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create MySQL tags table: %v", err)
	}
	_, err = dbs.mysql.Exec(`
		CREATE TABLE pet_tags (
			pet_id BIGINT NOT NULL,
			tag_id BIGINT NOT NULL,
			PRIMARY KEY (pet_id, tag_id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create MySQL pet_tags table: %v", err)
	}

	// SQLite
	_, err = dbs.sqlite.Exec(`
		DROP TABLE IF EXISTS pet_tags;
		DROP TABLE IF EXISTS pets;
		DROP TABLE IF EXISTS tags;
		DROP TABLE IF EXISTS categories;
		
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY,
			public_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		
		CREATE TABLE pets (
			id INTEGER PRIMARY KEY,
			public_id TEXT NOT NULL UNIQUE,
			category_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		
		CREATE TABLE tags (
			id INTEGER PRIMARY KEY,
			public_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		
		CREATE TABLE pet_tags (
			pet_id INTEGER NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (pet_id, tag_id)
		);
	`)
	if err != nil {
		t.Fatalf("failed to setup SQLite schema: %v", err)
	}
}

// insertAllDBsData inserts identical test data into all databases
func insertAllDBsData(t *testing.T, dbs *dbConnections) {
	t.Helper()

	// PostgreSQL
	_, err := dbs.postgres.Exec(context.Background(), `
		INSERT INTO categories (id, public_id, name, created_at, updated_at) VALUES
		(1, 'cat-1', 'Dogs', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'cat-2', 'Cats', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
		
		INSERT INTO pets (id, public_id, category_id, name, status, created_at, updated_at) VALUES
		(1, 'pet-1', 1, 'Buddy', 'available', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'pet-2', 1, 'Max', 'pending', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(3, 'pet-3', 2, 'Whiskers', 'available', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
		
		INSERT INTO tags (id, public_id, name, created_at, updated_at) VALUES
		(1, 'tag-1', 'Friendly', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'tag-2', 'Trained', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
		
		INSERT INTO pet_tags (pet_id, tag_id) VALUES
		(1, 1), (1, 2), (2, 1);
	`)
	if err != nil {
		t.Fatalf("failed to insert PostgreSQL data: %v", err)
	}

	// MySQL
	_, err = dbs.mysql.Exec(`
		INSERT INTO categories (id, public_id, name, created_at, updated_at) VALUES
		(1, 'cat-1', 'Dogs', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'cat-2', 'Cats', '2025-01-01 00:00:00', '2025-01-01 00:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert MySQL categories: %v", err)
	}
	_, err = dbs.mysql.Exec(`
		INSERT INTO pets (id, public_id, category_id, name, status, created_at, updated_at) VALUES
		(1, 'pet-1', 1, 'Buddy', 'available', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'pet-2', 1, 'Max', 'pending', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(3, 'pet-3', 2, 'Whiskers', 'available', '2025-01-01 00:00:00', '2025-01-01 00:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert MySQL pets: %v", err)
	}
	_, err = dbs.mysql.Exec(`
		INSERT INTO tags (id, public_id, name, created_at, updated_at) VALUES
		(1, 'tag-1', 'Friendly', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'tag-2', 'Trained', '2025-01-01 00:00:00', '2025-01-01 00:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert MySQL tags: %v", err)
	}
	_, err = dbs.mysql.Exec(`INSERT INTO pet_tags (pet_id, tag_id) VALUES (1, 1), (1, 2), (2, 1)`)
	if err != nil {
		t.Fatalf("failed to insert MySQL pet_tags: %v", err)
	}

	// SQLite
	_, err = dbs.sqlite.Exec(`
		INSERT INTO categories (id, public_id, name, created_at, updated_at) VALUES
		(1, 'cat-1', 'Dogs', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'cat-2', 'Cats', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
		
		INSERT INTO pets (id, public_id, category_id, name, status, created_at, updated_at) VALUES
		(1, 'pet-1', 1, 'Buddy', 'available', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'pet-2', 1, 'Max', 'pending', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(3, 'pet-3', 2, 'Whiskers', 'available', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
		
		INSERT INTO tags (id, public_id, name, created_at, updated_at) VALUES
		(1, 'tag-1', 'Friendly', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
		(2, 'tag-2', 'Trained', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
		
		INSERT INTO pet_tags (pet_id, tag_id) VALUES
		(1, 1), (1, 2), (2, 1);
	`)
	if err != nil {
		t.Fatalf("failed to insert SQLite data: %v", err)
	}
}

// createCrossDBPlan creates a migration plan for cross-DB testing
func createCrossDBPlan() *migrate.MigrationPlan {
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

// normalizeJSON normalizes a JSON structure for comparison:
// - Removes null entries from arrays
// - Converts numeric types consistently
// - Sorts arrays by a deterministic key
func normalizeJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			if v != nil {
				result[k] = normalizeJSON(v)
			}
		}
		return result
	case []interface{}:
		// Filter out nil values
		result := make([]interface{}, 0)
		for _, item := range val {
			if item != nil {
				result = append(result, normalizeJSON(item))
			}
		}
		// Sort by public_id or name for deterministic comparison
		sort.Slice(result, func(i, j int) bool {
			mi, oki := result[i].(map[string]interface{})
			mj, okj := result[j].(map[string]interface{})
			if !oki || !okj {
				return false
			}
			// Try public_id first, then name
			if pi, ok := mi["public_id"].(string); ok {
				if pj, ok := mj["public_id"].(string); ok {
					return pi < pj
				}
			}
			if ni, ok := mi["name"].(string); ok {
				if nj, ok := mj["name"].(string); ok {
					return ni < nj
				}
			}
			return false
		})
		return result
	case float64:
		// Normalize numeric representation
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	default:
		return val
	}
}

// compareJSONResults compares two JSON results for semantic equivalence
func compareJSONResults(t *testing.T, result1, result2 interface{}, label string) bool {
	t.Helper()
	norm1 := normalizeJSON(result1)
	norm2 := normalizeJSON(result2)

	if !reflect.DeepEqual(norm1, norm2) {
		j1, _ := json.MarshalIndent(norm1, "", "  ")
		j2, _ := json.MarshalIndent(norm2, "", "  ")
		t.Errorf("%s: JSON not equivalent\nFirst:\n%s\nSecond:\n%s", label, j1, j2)
		return false
	}
	return true
}

func TestCrossDB_HasMany_SemanticEquivalence(t *testing.T) {
	dbs := connectAllDBs(t)
	defer dbs.closeAll()

	setupAllDBsSchema(t, dbs)
	insertAllDBsData(t, dbs)

	plan := createCrossDBPlan()
	relations := ScanRelations(plan)

	hasManyRel := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasManyRel == nil {
		t.Fatal("expected HasMany relation")
	}

	// Generate SQL for each dialect
	pgSQL := GenerateRelationSQL(plan, *hasManyRel, SQLDialectPostgres)
	mySQL := GenerateRelationSQL(plan, *hasManyRel, SQLDialectMySQL)
	sqSQL := GenerateRelationSQL(plan, *hasManyRel, SQLDialectSQLite)

	t.Logf("PostgreSQL SQL: %s", pgSQL)
	t.Logf("MySQL SQL: %s", mySQL)
	t.Logf("SQLite SQL: %s", sqSQL)

	// Execute on PostgreSQL
	var pgPublicID, pgName, pgCreatedAt, pgUpdatedAt string
	var pgPetsJSON string
	err := dbs.postgres.QueryRow(context.Background(), pgSQL, "cat-1").Scan(
		&pgPublicID, &pgName, &pgCreatedAt, &pgUpdatedAt, &pgPetsJSON)
	if err != nil {
		t.Fatalf("PostgreSQL query failed: %v", err)
	}

	// Execute on MySQL
	var myPublicID, myName, myCreatedAt, myUpdatedAt string
	var myPetsJSON string
	err = dbs.mysql.QueryRow(mySQL, "cat-1").Scan(
		&myPublicID, &myName, &myCreatedAt, &myUpdatedAt, &myPetsJSON)
	if err != nil {
		t.Fatalf("MySQL query failed: %v", err)
	}

	// Execute on SQLite
	var sqPublicID, sqName, sqCreatedAt, sqUpdatedAt string
	var sqPetsJSON string
	err = dbs.sqlite.QueryRow(sqSQL, "cat-1").Scan(
		&sqPublicID, &sqName, &sqCreatedAt, &sqUpdatedAt, &sqPetsJSON)
	if err != nil {
		t.Fatalf("SQLite query failed: %v", err)
	}

	// Parse JSON results
	var pgPets, myPets, sqPets interface{}
	json.Unmarshal([]byte(pgPetsJSON), &pgPets)
	json.Unmarshal([]byte(myPetsJSON), &myPets)
	json.Unmarshal([]byte(sqPetsJSON), &sqPets)

	// Compare results
	compareJSONResults(t, pgPets, myPets, "PostgreSQL vs MySQL HasMany")
	compareJSONResults(t, pgPets, sqPets, "PostgreSQL vs SQLite HasMany")

	// Verify base data is consistent
	if pgName != myName || pgName != sqName || pgName != "Dogs" {
		t.Errorf("name mismatch: pg=%s, my=%s, sq=%s", pgName, myName, sqName)
	}
}
