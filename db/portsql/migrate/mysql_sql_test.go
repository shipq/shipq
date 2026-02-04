package migrate

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// =============================================================================
// Autoincrement Primary Key Tests
// =============================================================================

func TestMySQL_CreateTable_AutoincrementPK_Bigint(t *testing.T) {
	tb := ddl.MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("name")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Should emit NOT NULL AUTO_INCREMENT for single bigint PK
	if !strings.Contains(sql, "`id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY") {
		t.Errorf("expected BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_AutoincrementPK_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("users")
	tb.Integer("id").PrimaryKey()
	tb.String("name")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Should emit NOT NULL AUTO_INCREMENT for single integer PK
	if !strings.Contains(sql, "`id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY") {
		t.Errorf("expected INT NOT NULL AUTO_INCREMENT PRIMARY KEY, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_CompositePK_NoAutoincrement(t *testing.T) {
	tb := ddl.MakeEmptyTable("user_roles")
	tb.Bigint("user_id").PrimaryKey()
	tb.Bigint("role_id").PrimaryKey()
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Composite PK should NOT get AUTO_INCREMENT
	if strings.Contains(sql, "AUTO_INCREMENT") {
		t.Errorf("composite PK should not have AUTO_INCREMENT, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_StringPK_NoAutoincrement(t *testing.T) {
	tb := ddl.MakeEmptyTable("settings")
	tb.String("key").PrimaryKey()
	tb.String("value")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// String PK should NOT get AUTO_INCREMENT
	if strings.Contains(sql, "AUTO_INCREMENT") {
		t.Errorf("string PK should not have AUTO_INCREMENT, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_JunctionTable_NoAutoincrement(t *testing.T) {
	tb := ddl.MakeEmptyTable("user_groups")
	tb.Bigint("id").PrimaryKey()
	tb.Bigint("user_id")
	tb.Bigint("group_id")
	table := tb.Build()
	table.IsJunctionTable = true

	sql := generateMySQLCreateTable(table)

	// Junction table should NOT get AUTO_INCREMENT even with single integer PK
	if strings.Contains(sql, "AUTO_INCREMENT") {
		t.Errorf("junction table should not have AUTO_INCREMENT, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_AutoincrementPK_NoDefault(t *testing.T) {
	// Even if a default is specified on the PK column, it should be ignored
	// for autoincrement-eligible PKs
	tb := ddl.MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey().Default(1)
	tb.String("name")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Should have AUTO_INCREMENT but NO DEFAULT clause
	if !strings.Contains(sql, "AUTO_INCREMENT") {
		t.Errorf("expected AUTO_INCREMENT, got:\n%s", sql)
	}
	if strings.Contains(sql, "DEFAULT 1") {
		t.Errorf("autoincrement PK should not have DEFAULT clause, got:\n%s", sql)
	}
}

// =============================================================================
// CREATE TABLE Tests
// =============================================================================

func TestMySQL_CreateTable_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("age")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`age` INT NOT NULL") {
		t.Errorf("expected INT column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Bigint(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bigint("id")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`id` BIGINT NOT NULL") {
		t.Errorf("expected BIGINT column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_String(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name") // Default VARCHAR(255)
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`name` VARCHAR(255) NOT NULL") {
		t.Errorf("expected VARCHAR(255) column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_VarChar(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.VarChar("code", 50)
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`code` VARCHAR(50) NOT NULL") {
		t.Errorf("expected VARCHAR(50) column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Text(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Text("description")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`description` TEXT NOT NULL") {
		t.Errorf("expected TEXT column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Boolean(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// MySQL uses TINYINT(1) for booleans
	if !strings.Contains(sql, "`active` TINYINT(1) NOT NULL") {
		t.Errorf("expected TINYINT(1) column for boolean, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Decimal(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Decimal("price", 10, 2)
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`price` DECIMAL(10, 2) NOT NULL") {
		t.Errorf("expected DECIMAL(10, 2) column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Float(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Float("score")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`score` DOUBLE NOT NULL") {
		t.Errorf("expected DOUBLE column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Datetime(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Datetime("created_at")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`created_at` DATETIME NOT NULL") {
		t.Errorf("expected DATETIME column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Timestamp(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Timestamp("updated_at")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`updated_at` TIMESTAMP NOT NULL") {
		t.Errorf("expected TIMESTAMP column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Binary(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Binary("data")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`data` BLOB NOT NULL") {
		t.Errorf("expected BLOB column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_JSON(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.JSON("metadata")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, "`metadata` JSON NOT NULL") {
		t.Errorf("expected JSON column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Nullable(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("bio").Nullable()
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Nullable columns should NOT have NOT NULL
	if strings.Contains(sql, "`bio` VARCHAR(255) NOT NULL") {
		t.Errorf("nullable column should not have NOT NULL, got:\n%s", sql)
	}
	// Should just be the type without NOT NULL
	if !strings.Contains(sql, "`bio` VARCHAR(255)") {
		t.Errorf("expected VARCHAR(255) column, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Default_String(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("status").Default("pending")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, `DEFAULT 'pending'`) {
		t.Errorf("expected DEFAULT 'pending', got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Default_Integer(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Integer("count").Default(0)
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, `DEFAULT 0`) {
		t.Errorf("expected DEFAULT 0, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Default_Boolean(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bool("active").Default(true)
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// MySQL booleans default to 1 or 0
	if !strings.Contains(sql, `DEFAULT 1`) {
		t.Errorf("expected DEFAULT 1 for true boolean, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_PrimaryKey(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.Bigint("id").PrimaryKey()
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, `PRIMARY KEY`) {
		t.Errorf("expected PRIMARY KEY, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Unique(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("email").Unique()
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Should create a unique index
	if !strings.Contains(sql, `CREATE UNIQUE INDEX`) {
		t.Errorf("expected CREATE UNIQUE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, "`idx_test_table_email`") {
		t.Errorf("expected index name idx_test_table_email, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_Index(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("status").Indexed()
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, `CREATE INDEX`) {
		t.Errorf("expected CREATE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, "`idx_test_table_status`") {
		t.Errorf("expected index name idx_test_table_status, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_CompositeIndex(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	col1 := tb.String("first_name")
	col2 := tb.String("last_name")
	tb.AddIndex(col1.Col(), col2.Col())
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	if !strings.Contains(sql, `CREATE INDEX`) {
		t.Errorf("expected CREATE INDEX, got:\n%s", sql)
	}
	if !strings.Contains(sql, "(`first_name`, `last_name`)") {
		t.Errorf("expected composite index columns, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_BacktickIdentifiers(t *testing.T) {
	tb := ddl.MakeEmptyTable("user_table")
	tb.String("user_name")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Table name should be backtick-quoted
	if !strings.Contains(sql, "CREATE TABLE `user_table`") {
		t.Errorf("expected backtick-quoted table name, got:\n%s", sql)
	}
	// Column name should be backtick-quoted
	if !strings.Contains(sql, "`user_name`") {
		t.Errorf("expected backtick-quoted column name, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_StringDefaultEscaping(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("message").Default("it's a test")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Single quotes in string should be escaped
	if !strings.Contains(sql, `DEFAULT 'it''s a test'`) {
		t.Errorf("expected escaped single quote, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_EngineAndCharset(t *testing.T) {
	tb := ddl.MakeEmptyTable("test_table")
	tb.String("name")
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Should include ENGINE and CHARSET
	if !strings.Contains(sql, "ENGINE=InnoDB") {
		t.Errorf("expected ENGINE=InnoDB, got:\n%s", sql)
	}
	if !strings.Contains(sql, "DEFAULT CHARSET=utf8mb4") {
		t.Errorf("expected DEFAULT CHARSET=utf8mb4, got:\n%s", sql)
	}
}

func TestMySQL_CreateTable_MultipleColumns(t *testing.T) {
	tb := ddl.MakeEmptyTable("users")
	tb.Bigint("id").PrimaryKey()
	tb.String("name")
	tb.String("email").Unique()
	tb.Integer("age").Nullable()
	tb.Bool("active").Default(true)
	table := tb.Build()

	sql := generateMySQLCreateTable(table)

	// Verify all columns are present
	if !strings.Contains(sql, "`id`") {
		t.Errorf("expected id column, got:\n%s", sql)
	}
	if !strings.Contains(sql, "`name`") {
		t.Errorf("expected name column, got:\n%s", sql)
	}
	if !strings.Contains(sql, "`email`") {
		t.Errorf("expected email column, got:\n%s", sql)
	}
	if !strings.Contains(sql, "`age`") {
		t.Errorf("expected age column, got:\n%s", sql)
	}
	if !strings.Contains(sql, "`active`") {
		t.Errorf("expected active column, got:\n%s", sql)
	}
}

// =============================================================================
// ALTER TABLE Tests
// =============================================================================

func TestMySQL_AlterTable_AddColumn(t *testing.T) {
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

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "ALTER TABLE `users` ADD COLUMN `email` VARCHAR(255) NOT NULL") {
		t.Errorf("expected ADD COLUMN statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_DropColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:   ddl.OpDropColumn,
			Column: "legacy_field",
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "ALTER TABLE `users` DROP COLUMN `legacy_field`") {
		t.Errorf("expected DROP COLUMN statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_RenameColumn(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpRenameColumn,
			Column:  "name",
			NewName: "full_name",
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	// MySQL 8.0+ syntax
	if !strings.Contains(sql, "ALTER TABLE `users` RENAME COLUMN `name` TO `full_name`") {
		t.Errorf("expected RENAME COLUMN statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_ChangeType(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpChangeType,
			Column:  "count",
			NewType: ddl.BigintType,
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	// MySQL uses MODIFY COLUMN for type changes
	if !strings.Contains(sql, "ALTER TABLE `users` MODIFY COLUMN `count` BIGINT") {
		t.Errorf("expected MODIFY COLUMN TYPE statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_SetNullable(t *testing.T) {
	nullable := true
	ops := []ddl.TableOperation{
		{
			Type:     ddl.OpChangeNullable,
			Column:   "bio",
			Nullable: &nullable,
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	// MySQL uses MODIFY COLUMN for nullability changes
	if !strings.Contains(sql, "ALTER TABLE `users` MODIFY COLUMN `bio`") && !strings.Contains(sql, "NULL") {
		t.Errorf("expected MODIFY COLUMN with NULL, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_SetNotNull(t *testing.T) {
	nullable := false
	ops := []ddl.TableOperation{
		{
			Type:     ddl.OpChangeNullable,
			Column:   "email",
			Nullable: &nullable,
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	// MySQL uses MODIFY COLUMN for nullability changes
	if !strings.Contains(sql, "ALTER TABLE `users` MODIFY COLUMN `email`") && !strings.Contains(sql, "NOT NULL") {
		t.Errorf("expected MODIFY COLUMN with NOT NULL, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_SetDefault(t *testing.T) {
	defaultVal := "pending"
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpChangeDefault,
			Column:  "status",
			Default: &defaultVal,
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "ALTER TABLE `users` ALTER COLUMN `status` SET DEFAULT 'pending'") {
		t.Errorf("expected SET DEFAULT statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_DropDefault(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:    ddl.OpChangeDefault,
			Column:  "status",
			Default: nil,
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "ALTER TABLE `users` ALTER COLUMN `status` DROP DEFAULT") {
		t.Errorf("expected DROP DEFAULT statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_AddIndex(t *testing.T) {
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

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "CREATE INDEX `idx_users_email` ON `users` (`email`)") {
		t.Errorf("expected CREATE INDEX statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_AddUniqueIndex(t *testing.T) {
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

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "CREATE UNIQUE INDEX `idx_users_email` ON `users` (`email`)") {
		t.Errorf("expected CREATE UNIQUE INDEX statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_DropIndex(t *testing.T) {
	ops := []ddl.TableOperation{
		{
			Type:      ddl.OpDropIndex,
			IndexName: "idx_users_email",
		},
	}

	sql := generateMySQLAlterTable("users", ops)

	// MySQL DROP INDEX requires ON table_name
	if !strings.Contains(sql, "DROP INDEX `idx_users_email` ON `users`") {
		t.Errorf("expected DROP INDEX ON statement, got:\n%s", sql)
	}
}

func TestMySQL_AlterTable_MultipleOperations(t *testing.T) {
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

	sql := generateMySQLAlterTable("users", ops)

	if !strings.Contains(sql, "ADD COLUMN `email`") {
		t.Errorf("expected ADD COLUMN, got:\n%s", sql)
	}
	if !strings.Contains(sql, "DROP COLUMN `legacy`") {
		t.Errorf("expected DROP COLUMN, got:\n%s", sql)
	}
}

// =============================================================================
// DROP TABLE Tests
// =============================================================================

func TestMySQL_DropTable(t *testing.T) {
	sql := generateMySQLDropTable("users")

	expected := "DROP TABLE `users`"
	if sql != expected {
		t.Errorf("expected %q, got %q", expected, sql)
	}
}

func TestMySQL_DropTable_BacktickIdentifier(t *testing.T) {
	sql := generateMySQLDropTable("user_accounts")

	if !strings.Contains(sql, "`user_accounts`") {
		t.Errorf("expected backtick-quoted table name, got: %s", sql)
	}
}
