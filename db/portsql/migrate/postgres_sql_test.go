package migrate

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// CREATE TABLE Tests
// =============================================================================

func TestPostgres_CreateTable_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("age")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"age" INTEGER NOT NULL`) {
		t.Errorf("expected INTEGER column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Bigint(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bigint("id")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"id" BIGINT NOT NULL`) {
		t.Errorf("expected BIGINT column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_String(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name") // Default VARCHAR(255)
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"name" VARCHAR(255) NOT NULL`) {
		t.Errorf("expected VARCHAR(255) column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Varchar(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Varchar("code", 50)
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"code" VARCHAR(50) NOT NULL`) {
		t.Errorf("expected VARCHAR(50) column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Text(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Text("description")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"description" TEXT NOT NULL`) {
		t.Errorf("expected TEXT column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Boolean(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"active" BOOLEAN NOT NULL`) {
		t.Errorf("expected BOOLEAN column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Decimal(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Decimal("price", 10, 2)
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"price" DECIMAL(10, 2) NOT NULL`) {
		t.Errorf("expected DECIMAL(10, 2) column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Float(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Float("score")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"score" DOUBLE PRECISION NOT NULL`) {
		t.Errorf("expected DOUBLE PRECISION column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Datetime(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Datetime("created_at")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"created_at" TIMESTAMP WITH TIME ZONE NOT NULL`) {
		t.Errorf("expected TIMESTAMP WITH TIME ZONE column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Timestamp(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Timestamp("updated_at")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"updated_at" TIMESTAMP WITH TIME ZONE NOT NULL`) {
		t.Errorf("expected TIMESTAMP WITH TIME ZONE column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Binary(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Binary("data")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"data" BYTEA NOT NULL`) {
		t.Errorf("expected BYTEA column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_JSON(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.JSON("metadata")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `"metadata" JSONB NOT NULL`) {
		t.Errorf("expected JSONB column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Nullable(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("bio").Nullable()
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	// Nullable columns should NOT have NOT NULL
	if strings.Contains(sql, `"bio" VARCHAR(255) NOT NULL`) {
		t.Errorf("nullable column should not have NOT NULL, got:\n%s", sql)
	}
	// Should just be the type without NOT NULL
	if !strings.Contains(sql, `"bio" VARCHAR(255)`) {
		t.Errorf("expected VARCHAR(255) column, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Default_String(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("status").Default("pending")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `DEFAULT 'pending'`) {
		t.Errorf("expected DEFAULT 'pending', got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Default_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("count").Default(0)
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `DEFAULT 0`) {
		t.Errorf("expected DEFAULT 0, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Default_Boolean(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active").Default(true)
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `DEFAULT TRUE`) {
		t.Errorf("expected DEFAULT TRUE, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_PrimaryKey(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bigint("id").PrimaryKey()
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `PRIMARY KEY`) {
		t.Errorf("expected PRIMARY KEY, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Unique(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("email").Unique()
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	// Should create a unique index
	if !strings.Contains(sql, `CREATE UNIQUE INDEX`) {
		t.Errorf("expected CREATE UNIQUE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"idx_test_table_email"`) {
		t.Errorf("expected index name idx_test_table_email, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_Index(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("status").Indexed()
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `CREATE INDEX`) {
		t.Errorf("expected CREATE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, `"idx_test_table_status"`) {
		t.Errorf("expected index name idx_test_table_status, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_CompositeIndex(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	col1 := tb.String("first_name")
	col2 := tb.String("last_name")
	tb.AddIndex(col1.Col(), col2.Col())
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	if !strings.Contains(sql, `CREATE INDEX`) {
		t.Errorf("expected CREATE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, `("first_name", "last_name")`) {
		t.Errorf("expected composite index columns, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_QuotesIdentifiers(t *testing.T) {
	tb := ddl.MakeEmptyTable("user_table")
	tb.String("user_name")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	// Table name should be double-quoted
	if !strings.Contains(sql, `CREATE TABLE "user_table"`) {
		t.Errorf("expected double-quoted table name, got:\n%s", sql)
	}
	// Column name should be double-quoted
	if !strings.Contains(sql, `"user_name"`) {
		t.Errorf("expected double-quoted column name, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_StringDefaultEscaping(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("message").Default("it's a test")
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

	// Single quotes in string should be escaped
	if !strings.Contains(sql, `DEFAULT 'it''s a test'`) {
		t.Errorf("expected escaped single quote, got:\n%s", sql)
	}
}

func TestPostgres_CreateTable_MultipleColumns(t *testing.T) {
	tb := ddl.MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("name")
	tb.String("email").Unique()
	tb.Integer("age").Nullable()
	tb.Bool("active").Default(true)
	table := tb.Build()

	sql := generatePostgresCreateTable(table)

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

func TestPostgres_AlterTable_AddColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddColumn,
			ColumnDef: &ddl.ColumnDefinition{
				Name:     "email",
				Type:     ddl.StringType,
				Length:   intPtr(255),
				Nullable: false,
			},
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" ADD COLUMN "email" VARCHAR(255) NOT NULL`) {
		t.Errorf("expected ADD COLUMN statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_DropColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:   ddl.OpDropColumn,
			Column: "legacy_field",
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" DROP COLUMN "legacy_field"`) {
		t.Errorf("expected DROP COLUMN statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_RenameColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpRenameColumn,
			Column:  "name",
			NewName: "full_name",
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" RENAME COLUMN "name" TO "full_name"`) {
		t.Errorf("expected RENAME COLUMN statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_ChangeType(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpChangeType,
			Column:  "count",
			NewType: ddl.BigintType,
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" ALTER COLUMN "count" TYPE BIGINT`) {
		t.Errorf("expected ALTER COLUMN TYPE statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_SetNullable(t *testing.T) {
	nullable := true
	ops := []ddl.TableOperation{
		{
			Type:     ddl.OpChangeNullable,
			Column:   "bio",
			Nullable: &nullable,
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" ALTER COLUMN "bio" DROP NOT NULL`) {
		t.Errorf("expected DROP NOT NULL statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_SetNotNull(t *testing.T) {
	nullable := false
	ops := []ddl.TableOperation{
		{
			Type:     ddl.OpChangeNullable,
			Column:   "email",
			Nullable: &nullable,
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" ALTER COLUMN "email" SET NOT NULL`) {
		t.Errorf("expected SET NOT NULL statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_SetDefault(t *testing.T) {
	defaultVal := "pending"
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpChangeDefault,
			Column:  "status",
			Default: &defaultVal,
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" ALTER COLUMN "status" SET DEFAULT 'pending'`) {
		t.Errorf("expected SET DEFAULT statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_DropDefault(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpChangeDefault,
			Column:  "status",
			Default: nil,
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `ALTER TABLE "users" ALTER COLUMN "status" DROP DEFAULT`) {
		t.Errorf("expected DROP DEFAULT statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_AddIndex(t *testing.T) {
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

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `CREATE INDEX "idx_users_email" ON "users" ("email")`) {
		t.Errorf("expected CREATE INDEX statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_AddUniqueIndex(t *testing.T) {
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

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `CREATE UNIQUE INDEX "idx_users_email" ON "users" ("email")`) {
		t.Errorf("expected CREATE UNIQUE INDEX statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_DropIndex(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:      ddl.OpDropIndex,
			IndexName: "idx_users_email",
		},
	}

	sql := generatePostgresAlterTable("users", ops)

	if !strings.Contains(sql, `DROP INDEX "idx_users_email"`) {
		t.Errorf("expected DROP INDEX statement, got:\n%s", sql)
	}
}

func TestPostgres_AlterTable_MultipleOperations(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type: ddl.OpAddColumn,
			ColumnDef: &ddl.ColumnDefinition{
				Name:     "email",
				Type:     ddl.StringType,
				Length:   intPtr(255),
				Nullable: false,
			},
		},
		{
			Type:   ddl.OpDropColumn,
			Column: "legacy",
		},
	}

	sql := generatePostgresAlterTable("users", ops)

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

func TestPostgres_DropTable(t *testing.T) {
	sql := generatePostgresDropTable("users")

	expected := `DROP TABLE "users"`
	if sql != expected {
		t.Errorf("expected %q, got %q", expected, sql)
	}
}

func TestPostgres_DropTable_QuotesIdentifier(t *testing.T) {
	sql := generatePostgresDropTable("user_accounts")

	if !strings.Contains(sql, `"user_accounts"`) {
		t.Errorf("expected double-quoted table name, got: %s", sql)
	}
}
