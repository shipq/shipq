package migrate

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// CREATE TABLE Tests
// =============================================================================

func TestSQLite_CreateTable_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("age")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `"age" INTEGER NOT NULL`) {
		t.Errorf("expected INTEGER column, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Bigint(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bigint("id")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite uses INTEGER for both int and bigint (64-bit)
	if !strings.Contains(sql, `"id" INTEGER NOT NULL`) {
		t.Errorf("expected INTEGER column for bigint, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_String(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite doesn't have VARCHAR, use TEXT
	if !strings.Contains(sql, `"name" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column for string, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_VarChar(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.VarChar("code", 50)
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite doesn't have VARCHAR(n), use TEXT
	if !strings.Contains(sql, `"code" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column for varchar, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Text(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Text("description")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `"description" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Boolean(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite uses INTEGER for booleans (0=false, 1=true)
	if !strings.Contains(sql, `"active" INTEGER NOT NULL`) {
		t.Errorf("expected INTEGER column for boolean, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Decimal(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Decimal("price", 10, 2)
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite uses REAL for decimals
	if !strings.Contains(sql, `"price" REAL NOT NULL`) {
		t.Errorf("expected REAL column for decimal, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Float(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Float("score")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `"score" REAL NOT NULL`) {
		t.Errorf("expected REAL column, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Datetime(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Datetime("created_at")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite stores datetime as TEXT (ISO8601 format)
	if !strings.Contains(sql, `"created_at" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column for datetime, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Timestamp(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Timestamp("updated_at")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite stores timestamp as TEXT (ISO8601 format)
	if !strings.Contains(sql, `"updated_at" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column for timestamp, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Binary(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Binary("data")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `"data" BLOB NOT NULL`) {
		t.Errorf("expected BLOB column, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_JSON(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.JSON("metadata")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite stores JSON as TEXT
	if !strings.Contains(sql, `"metadata" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column for JSON, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Nullable(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("bio").Nullable()
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Nullable columns should NOT have NOT NULL
	if strings.Contains(sql, `"bio" TEXT NOT NULL`) {
		t.Errorf("nullable column should not have NOT NULL, got:\n%s", sql)
	}
	// Should just be the type without NOT NULL
	if !strings.Contains(sql, `"bio" TEXT`) {
		t.Errorf("expected TEXT column, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Default_String(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("status").Default("pending")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `DEFAULT 'pending'`) {
		t.Errorf("expected DEFAULT 'pending', got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Default_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("count").Default(0)
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `DEFAULT 0`) {
		t.Errorf("expected DEFAULT 0, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Default_Boolean(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active").Default(true)
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite booleans default to 1 or 0
	if !strings.Contains(sql, `DEFAULT 1`) {
		t.Errorf("expected DEFAULT 1 for true boolean, got:\n%s", sql)
	}
}

// TestSQLite_CreateTable_NullableWithDefault tests that a nullable column
// with a default value generates correct SQL (no NOT NULL, but has DEFAULT)
func TestSQLite_CreateTable_NullableWithDefault(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name").Nullable().Default("testdefault")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Should have the DEFAULT clause
	if !strings.Contains(sql, `DEFAULT 'testdefault'`) {
		t.Errorf("expected DEFAULT 'testdefault', got:\n%s", sql)
	}

	// Should NOT have NOT NULL (since column is nullable)
	if strings.Contains(sql, `NOT NULL`) {
		t.Errorf("expected no NOT NULL for nullable column, got:\n%s", sql)
	}

	// Log the full SQL for debugging
	t.Logf("Generated SQL: %s", sql)
}

// TestSQLite_CreateTable_EmptyStringDefault tests that an empty string default
// is properly handled - empty string IS a valid default value
func TestSQLite_CreateTable_EmptyStringDefault(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name").Default("")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Empty string default should still produce a DEFAULT clause: DEFAULT ''
	if !strings.Contains(sql, `DEFAULT ''`) {
		t.Errorf("expected DEFAULT '' for empty string default, got:\n%s", sql)
	}

	t.Logf("Generated SQL: %s", sql)
}

func TestSQLite_CreateTable_PrimaryKey(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bigint("id").PrimaryKey()
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `PRIMARY KEY`) {
		t.Errorf("expected PRIMARY KEY, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Unique(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("email").Unique()
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Should create a unique index
	if !strings.Contains(sql, `CREATE UNIQUE INDEX`) {
		t.Errorf("expected CREATE UNIQUE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"idx_test_table_email"`) {
		t.Errorf("expected index name idx_test_table_email, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_Index(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("status").Indexed()
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `CREATE INDEX`) {
		t.Errorf("expected CREATE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"idx_test_table_status"`) {
		t.Errorf("expected index name idx_test_table_status, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_CompositeIndex(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	col1 := tb.String("first_name")
	col2 := tb.String("last_name")
	tb.AddIndex(col1.Col(), col2.Col())
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	if !strings.Contains(sql, `CREATE INDEX`) {
		t.Errorf("expected CREATE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, `("first_name", "last_name")`) {
		t.Errorf("expected composite index columns, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_QuotesIdentifiers(t *testing.T) {
	tb := ddl.MakeEmptyTable("user_table")
	tb.String("user_name")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// SQLite uses double quotes (like PostgreSQL)
	if !strings.Contains(sql, `CREATE TABLE "user_table"`) {
		t.Errorf("expected double-quoted table name, got:\n%s", sql)
	}
	// Column name should be double-quoted
	if !strings.Contains(sql, `"user_name"`) {
		t.Errorf("expected double-quoted column name, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_StringDefaultEscaping(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("message").Default("it's a test")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Single quotes in string should be escaped
	if !strings.Contains(sql, `DEFAULT 'it''s a test'`) {
		t.Errorf("expected escaped single quote, got:\n%s", sql)
	}
}

func TestSQLite_CreateTable_MultipleColumns(t *testing.T) {
	tb := ddl.MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("name")
	tb.String("email").Unique()
	tb.Integer("age").Nullable()
	tb.Bool("active").Default(true)
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Verify all columns are present
	if !strings.Contains(sql, `"id"`) {
		t.Errorf("expected id column, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"name"`) {
		t.Errorf("expected name column, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"email"`) {
		t.Errorf("expected email column, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"age"`) {
		t.Errorf("expected age column, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"active"`) {
		t.Errorf("expected active column, got:\n%s", sql)
	}
}

// =============================================================================
// ALTER TABLE Tests
// =============================================================================

func TestSQLite_AlterTable_AddColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddColumn,
			ColumnDef: &ddl.ColumnDefinition{
				Name:     "email",
				Type:     ddl.StringType,
				Length:   intPtr(255),
				Nullable: false,
				Default:  nil, // SQLite needs DEFAULT for NOT NULL columns in ADD COLUMN
			},
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	if !strings.Contains(sql, `ALTER TABLE "users" ADD COLUMN "email" TEXT NOT NULL`) {
		t.Errorf("expected ADD COLUMN statement, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_AddColumn_WithDefault(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddColumn,
			ColumnDef: &ddl.ColumnDefinition{
				Name:     "status",
				Type:     ddl.StringType,
				Nullable: false,
				Default:  strPtr("pending"),
			},
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	// SQLite requires DEFAULT for NOT NULL columns in ADD COLUMN
	if !strings.Contains(sql, `ADD COLUMN "status" TEXT NOT NULL DEFAULT 'pending'`) {
		t.Errorf("expected ADD COLUMN with DEFAULT, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_DropColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:   ddl.OpDropColumn,
			Column: "legacy_field",
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	// SQLite 3.35.0+ supports DROP COLUMN
	if !strings.Contains(sql, `ALTER TABLE "users" DROP COLUMN "legacy_field"`) {
		t.Errorf("expected DROP COLUMN statement, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_RenameColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpRenameColumn,
			Column:  "name",
			NewName: "full_name",
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	// SQLite 3.25.0+ syntax
	if !strings.Contains(sql, `ALTER TABLE "users" RENAME COLUMN "name" TO "full_name"`) {
		t.Errorf("expected RENAME COLUMN statement, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_AddIndex(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddIndex,
			IndexDef: &ddl.IndexDefinition{
				Name:    "idx_users_email",
				Columns: []string{"email"},
				Unique:  false,
			},
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	if !strings.Contains(sql, `CREATE INDEX "idx_users_email" ON "users" ("email")`) {
		t.Errorf("expected CREATE INDEX statement, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_AddUniqueIndex(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddIndex,
			IndexDef: &ddl.IndexDefinition{
				Name:    "idx_users_email",
				Columns: []string{"email"},
				Unique:  true,
			},
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	if !strings.Contains(sql, `CREATE UNIQUE INDEX "idx_users_email" ON "users" ("email")`) {
		t.Errorf("expected CREATE UNIQUE INDEX statement, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_DropIndex(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:      ddl.OpDropIndex,
			IndexName: "idx_users_email",
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	// SQLite DROP INDEX doesn't need ON clause (like PostgreSQL)
	if !strings.Contains(sql, `DROP INDEX "idx_users_email"`) {
		t.Errorf("expected DROP INDEX statement, got:\n%s", sql)
	}
}

func TestSQLite_AlterTable_MultipleOperations(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddColumn,
			ColumnDef: &ddl.ColumnDefinition{
				Name:     "email",
				Type:     ddl.StringType,
				Nullable: true,
			},
		},
		{
			Type:   ddl.OpDropColumn,
			Column: "legacy",
		},
	}

	sql := generateSQLiteAlterTable("users", ops, nil)

	if !strings.Contains(sql, `ADD COLUMN "email"`) {
		t.Errorf("expected ADD COLUMN, got:\n%s", sql)
	}
	if !strings.Contains(sql, `DROP COLUMN "legacy"`) {
		t.Errorf("expected DROP COLUMN, got:\n%s", sql)
	}
}

// =============================================================================
// DROP TABLE Tests
// =============================================================================

func TestSQLite_DropTable(t *testing.T) {
	sql := generateSQLiteDropTable("users")

	expected := `DROP TABLE "users"`
	if sql != expected {
		t.Errorf("expected %q, got %q", expected, sql)
	}
}

func TestSQLite_DropTable_QuotesIdentifier(t *testing.T) {
	sql := generateSQLiteDropTable("user_accounts")

	if !strings.Contains(sql, `"user_accounts"`) {
		t.Errorf("expected double-quoted table name, got: %s", sql)
	}
}

// =============================================================================
// SQLite-Specific Tests (Type Mappings)
// =============================================================================

func TestSQLite_TextForStrings(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name")      // Should become TEXT
	tb.VarChar("code", 50) // Should become TEXT (no VARCHAR in SQLite)
	tb.Text("bio")         // Should become TEXT
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// All string types should be TEXT
	if strings.Contains(sql, "VARCHAR") {
		t.Errorf("SQLite should not have VARCHAR, got:\n%s", sql)
	}
	// Count TEXT occurrences (should have 3)
	if strings.Count(sql, "TEXT") != 3 {
		t.Errorf("expected 3 TEXT columns, got:\n%s", sql)
	}
}

func TestSQLite_IntegerForBooleans(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active")
	tb.Bool("verified")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Boolean should be INTEGER (no BOOLEAN type in SQLite)
	if strings.Contains(sql, "BOOLEAN") {
		t.Errorf("SQLite should not have BOOLEAN type, got:\n%s", sql)
	}
	if strings.Contains(sql, "TINYINT") {
		t.Errorf("SQLite should not have TINYINT type, got:\n%s", sql)
	}
}

func TestSQLite_IntegerForBigint(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("count")
	tb.Bigint("big_count")
	table := tb.Build()

	sql := generateSQLiteCreateTable(table)

	// Both should be INTEGER (SQLite INTEGER is 64-bit)
	if strings.Contains(sql, "BIGINT") {
		t.Errorf("SQLite should not have BIGINT type, got:\n%s", sql)
	}
}
