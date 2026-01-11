package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

// =============================================================================
// SQL Generation Tests
// =============================================================================

func TestGenerateCRUDSQL_Insert_WithAutoFilledColumns(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	tests := []struct {
		name     string
		dialect  Dialect
		wantNow  string
		wantSQL  string
	}{
		{
			name:    "Postgres",
			dialect: DialectPostgres,
			wantNow: "NOW()",
			wantSQL: `INSERT INTO "authors" ("public_id", "name", "email", "created_at", "updated_at") VALUES ($1, $2, $3, NOW(), NOW()) RETURNING "public_id"`,
		},
		{
			name:    "MySQL",
			dialect: DialectMySQL,
			wantNow: "NOW()",
			wantSQL: "INSERT INTO `authors` (`public_id`, `name`, `email`, `created_at`, `updated_at`) VALUES (?, ?, ?, NOW(), NOW())",
		},
		{
			name:    "SQLite",
			dialect: DialectSQLite,
			wantNow: "datetime('now')",
			wantSQL: `INSERT INTO "authors" ("public_id", "name", "email", "created_at", "updated_at") VALUES (?, ?, ?, datetime('now'), datetime('now')) RETURNING "public_id"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, tt.dialect, CRUDOptions{})

			// Check that NOW() is present for timestamps
			if !strings.Contains(sqlSet.InsertSQL, tt.wantNow) {
				t.Errorf("InsertSQL should contain %s for auto-filled timestamps.\nGot: %s", tt.wantNow, sqlSet.InsertSQL)
			}

			// Check exact SQL
			if sqlSet.InsertSQL != tt.wantSQL {
				t.Errorf("InsertSQL mismatch.\nWant: %s\nGot:  %s", tt.wantSQL, sqlSet.InsertSQL)
			}

			// Verify public_id is NOT auto-filled (it's a param)
			// The first placeholder should be for public_id
			if tt.dialect == DialectPostgres {
				if !strings.Contains(sqlSet.InsertSQL, "$1") {
					t.Error("Postgres InsertSQL should have $1 for public_id")
				}
			} else {
				// For MySQL and SQLite, first ? is for public_id
				idx := strings.Index(sqlSet.InsertSQL, "?")
				if idx == -1 {
					t.Error("MySQL/SQLite InsertSQL should have ? placeholder")
				}
			}
		})
	}
}

func TestGenerateCRUDSQL_Update_WithAutoFilledUpdatedAt(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	tests := []struct {
		name     string
		dialect  Dialect
		wantNow  string
		wantSQL  string
	}{
		{
			name:    "Postgres",
			dialect: DialectPostgres,
			wantNow: "NOW()",
			wantSQL: `UPDATE "authors" SET "name" = $1, "email" = $2, "updated_at" = NOW() WHERE "public_id" = $3 AND "deleted_at" IS NULL`,
		},
		{
			name:    "MySQL",
			dialect: DialectMySQL,
			wantNow: "NOW()",
			wantSQL: "UPDATE `authors` SET `name` = ?, `email` = ?, `updated_at` = NOW() WHERE `public_id` = ? AND `deleted_at` IS NULL",
		},
		{
			name:    "SQLite",
			dialect: DialectSQLite,
			wantNow: "datetime('now')",
			wantSQL: `UPDATE "authors" SET "name" = ?, "email" = ?, "updated_at" = datetime('now') WHERE "public_id" = ? AND "deleted_at" IS NULL`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, tt.dialect, CRUDOptions{})

			// Check that NOW() is present for updated_at
			if !strings.Contains(sqlSet.UpdateSQL, tt.wantNow) {
				t.Errorf("UpdateSQL should contain %s for auto-filled updated_at.\nGot: %s", tt.wantNow, sqlSet.UpdateSQL)
			}

			// Check exact SQL
			if sqlSet.UpdateSQL != tt.wantSQL {
				t.Errorf("UpdateSQL mismatch.\nWant: %s\nGot:  %s", tt.wantSQL, sqlSet.UpdateSQL)
			}

			// Verify deleted_at IS NULL is in WHERE clause
			if !strings.Contains(sqlSet.UpdateSQL, "deleted_at") || !strings.Contains(sqlSet.UpdateSQL, "IS NULL") {
				t.Error("UpdateSQL should exclude soft-deleted records")
			}
		})
	}
}

func TestGenerateCRUDSQL_Delete_SoftDelete(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	tests := []struct {
		name     string
		dialect  Dialect
		wantNow  string
		wantSQL  string
	}{
		{
			name:    "Postgres",
			dialect: DialectPostgres,
			wantNow: "NOW()",
			wantSQL: `UPDATE "authors" SET "deleted_at" = NOW() WHERE "public_id" = $1 AND "deleted_at" IS NULL`,
		},
		{
			name:    "MySQL",
			dialect: DialectMySQL,
			wantNow: "NOW()",
			wantSQL: "UPDATE `authors` SET `deleted_at` = NOW() WHERE `public_id` = ? AND `deleted_at` IS NULL",
		},
		{
			name:    "SQLite",
			dialect: DialectSQLite,
			wantNow: "datetime('now')",
			wantSQL: `UPDATE "authors" SET "deleted_at" = datetime('now') WHERE "public_id" = ? AND "deleted_at" IS NULL`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, tt.dialect, CRUDOptions{})

			// Soft delete should be an UPDATE, not DELETE
			if strings.HasPrefix(strings.ToUpper(sqlSet.DeleteSQL), "DELETE") {
				t.Error("DeleteSQL should be UPDATE for soft delete when deleted_at exists")
			}

			// Check that NOW() is present
			if !strings.Contains(sqlSet.DeleteSQL, tt.wantNow) {
				t.Errorf("DeleteSQL should contain %s.\nGot: %s", tt.wantNow, sqlSet.DeleteSQL)
			}

			// Check exact SQL
			if sqlSet.DeleteSQL != tt.wantSQL {
				t.Errorf("DeleteSQL mismatch.\nWant: %s\nGot:  %s", tt.wantSQL, sqlSet.DeleteSQL)
			}
		})
	}
}

func TestGenerateCRUDSQL_Delete_HardDelete_WhenNoDeletedAt(t *testing.T) {
	table := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.TextType},
		},
	}

	tests := []struct {
		name    string
		dialect Dialect
		wantSQL string
	}{
		{
			name:    "Postgres",
			dialect: DialectPostgres,
			wantSQL: `DELETE FROM "settings" WHERE "id" = $1`,
		},
		{
			name:    "MySQL",
			dialect: DialectMySQL,
			wantSQL: "DELETE FROM `settings` WHERE `id` = ?",
		},
		{
			name:    "SQLite",
			dialect: DialectSQLite,
			wantSQL: `DELETE FROM "settings" WHERE "id" = ?`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, tt.dialect, CRUDOptions{})

			// When no deleted_at, delete should be a real DELETE
			if !strings.HasPrefix(strings.ToUpper(sqlSet.DeleteSQL), "DELETE") {
				t.Error("DeleteSQL should be DELETE when no deleted_at column")
			}

			// HardDeleteSQL should be empty (no separate hard delete needed)
			if sqlSet.HardDeleteSQL != "" {
				t.Error("HardDeleteSQL should be empty when no deleted_at column")
			}

			// Check exact SQL
			if sqlSet.DeleteSQL != tt.wantSQL {
				t.Errorf("DeleteSQL mismatch.\nWant: %s\nGot:  %s", tt.wantSQL, sqlSet.DeleteSQL)
			}
		})
	}
}

func TestGenerateCRUDSQL_HardDelete(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	tests := []struct {
		name    string
		dialect Dialect
		wantSQL string
	}{
		{
			name:    "Postgres",
			dialect: DialectPostgres,
			wantSQL: `DELETE FROM "authors" WHERE "public_id" = $1`,
		},
		{
			name:    "MySQL",
			dialect: DialectMySQL,
			wantSQL: "DELETE FROM `authors` WHERE `public_id` = ?",
		},
		{
			name:    "SQLite",
			dialect: DialectSQLite,
			wantSQL: `DELETE FROM "authors" WHERE "public_id" = ?`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, tt.dialect, CRUDOptions{})

			// HardDelete should be a real DELETE
			if !strings.HasPrefix(strings.ToUpper(sqlSet.HardDeleteSQL), "DELETE") {
				t.Errorf("HardDeleteSQL should be DELETE.\nGot: %s", sqlSet.HardDeleteSQL)
			}

			// Check exact SQL
			if sqlSet.HardDeleteSQL != tt.wantSQL {
				t.Errorf("HardDeleteSQL mismatch.\nWant: %s\nGot:  %s", tt.wantSQL, sqlSet.HardDeleteSQL)
			}
		})
	}
}

func TestGenerateCRUDSQL_Get_ExcludesSoftDeleted(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
		t.Run(string(dialect), func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Get should exclude soft-deleted records
			if !strings.Contains(sqlSet.GetSQL, "deleted_at") || !strings.Contains(sqlSet.GetSQL, "IS NULL") {
				t.Errorf("GetSQL should exclude soft-deleted records.\nGot: %s", sqlSet.GetSQL)
			}
		})
	}
}

func TestGenerateCRUDSQL_ParameterOrder(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}

	t.Run("Postgres parameter order", func(t *testing.T) {
		sqlSet := GenerateCRUDSQL(table, DialectPostgres, CRUDOptions{})

		// Insert: public_id ($1), name ($2), email ($3), then NOW() for timestamps
		if !strings.Contains(sqlSet.InsertSQL, "$1, $2, $3, NOW(), NOW()") {
			t.Errorf("Insert params should be $1, $2, $3, NOW(), NOW().\nGot: %s", sqlSet.InsertSQL)
		}

		// Update: name ($1), email ($2), NOW(), public_id ($3)
		if !strings.Contains(sqlSet.UpdateSQL, "$1") &&
			!strings.Contains(sqlSet.UpdateSQL, "$2") &&
			!strings.Contains(sqlSet.UpdateSQL, "$3") {
			t.Errorf("Update should use $1, $2, $3.\nGot: %s", sqlSet.UpdateSQL)
		}
	})

	t.Run("MySQL/SQLite parameter order", func(t *testing.T) {
		for _, dialect := range []Dialect{DialectMySQL, DialectSQLite} {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Count ? placeholders in INSERT
			insertCount := strings.Count(sqlSet.InsertSQL, "?")
			if insertCount != 3 {
				t.Errorf("%s Insert should have 3 ? placeholders (public_id, name, email), got %d.\nSQL: %s",
					dialect, insertCount, sqlSet.InsertSQL)
			}

			// Count ? placeholders in UPDATE
			updateCount := strings.Count(sqlSet.UpdateSQL, "?")
			if updateCount != 3 {
				t.Errorf("%s Update should have 3 ? placeholders (name, email, public_id), got %d.\nSQL: %s",
					dialect, updateCount, sqlSet.UpdateSQL)
			}
		}
	})
}

func TestGenerateCRUDSQL_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}

	opts := CRUDOptions{ScopeColumn: "organization_id"}

	for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
		t.Run(string(dialect), func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, dialect, opts)

			// Get should include scope column in WHERE
			if !strings.Contains(sqlSet.GetSQL, "organization_id") {
				t.Errorf("GetSQL should include scope column.\nGot: %s", sqlSet.GetSQL)
			}

			// List should include scope column in WHERE
			if !strings.Contains(sqlSet.ListSQL, "organization_id") {
				t.Errorf("ListSQL should include scope column.\nGot: %s", sqlSet.ListSQL)
			}

			// Insert should include scope column
			if !strings.Contains(sqlSet.InsertSQL, "organization_id") {
				t.Errorf("InsertSQL should include scope column.\nGot: %s", sqlSet.InsertSQL)
			}

			// Update should include scope column in WHERE
			if !strings.Contains(sqlSet.UpdateSQL, "organization_id") {
				t.Errorf("UpdateSQL should include scope column in WHERE.\nGot: %s", sqlSet.UpdateSQL)
			}
		})
	}
}

// =============================================================================
// CRUD Runner Generation Tests
// =============================================================================

func TestGenerateCRUDRunner_Compiles(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "name", Type: ddl.StringType},
						{Name: "email", Type: ddl.StringType},
						{Name: "created_at", Type: ddl.DatetimeType},
						{Name: "updated_at", Type: ddl.DatetimeType},
						{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
					},
				},
			},
		},
	}

	code, err := GenerateCRUDRunner(plan, "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateCRUDRunner failed: %v", err)
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "runner.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("Generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestGenerateCRUDRunner_ContainsNanoidImport(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "name", Type: ddl.StringType},
					},
				},
			},
		},
	}

	code, err := GenerateCRUDRunner(plan, "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateCRUDRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should import nanoid
	if !strings.Contains(codeStr, "github.com/portsql/nanoid") {
		t.Error("Generated code should import github.com/portsql/nanoid")
	}

	// Should use nanoid.New()
	if !strings.Contains(codeStr, "nanoid.New()") {
		t.Error("Generated code should use nanoid.New() for public_id generation")
	}
}

func TestGenerateCRUDRunner_HandlesDialects(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "name", Type: ddl.StringType},
						{Name: "created_at", Type: ddl.DatetimeType},
						{Name: "updated_at", Type: ddl.DatetimeType},
					},
				},
			},
		},
	}

	code, err := GenerateCRUDRunner(plan, "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateCRUDRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have dialect constants
	if !strings.Contains(codeStr, "Postgres Dialect = iota") {
		t.Error("Generated code should define Postgres dialect")
	}
	if !strings.Contains(codeStr, "MySQL") {
		t.Error("Generated code should define MySQL dialect")
	}
	if !strings.Contains(codeStr, "SQLite") {
		t.Error("Generated code should define SQLite dialect")
	}

	// Should have switch for dialects in constructor
	if !strings.Contains(codeStr, "switch dialect") {
		t.Error("Generated code should switch on dialect in constructor")
	}

	// Should have NOW() for Postgres/MySQL
	if !strings.Contains(codeStr, "NOW()") {
		t.Error("Generated code should contain NOW() for Postgres/MySQL")
	}

	// Should have datetime('now') for SQLite
	if !strings.Contains(codeStr, "datetime('now')") {
		t.Error("Generated code should contain datetime('now') for SQLite")
	}
}

func TestGenerateCRUDRunner_InsertMethodReturnsPublicID(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "name", Type: ddl.StringType},
					},
				},
			},
		},
	}

	code, err := GenerateCRUDRunner(plan, "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateCRUDRunner failed: %v", err)
	}

	codeStr := string(code)

	// InsertAuthor should return (string, error)
	if !strings.Contains(codeStr, "InsertAuthor(ctx context.Context, params InsertAuthorParams) (string, error)") {
		t.Error("InsertAuthor should return (string, error) when public_id exists")
	}

	// Should handle MySQL differently (no RETURNING)
	if !strings.Contains(codeStr, "if r.dialect == MySQL") {
		t.Error("InsertAuthor should handle MySQL dialect differently")
	}
}

func TestGenerateCRUDRunner_WithMultipleTables(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "public_id", Type: ddl.StringType},
						{Name: "name", Type: ddl.StringType},
						{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
					},
				},
				"posts": {
					Name: "posts",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "title", Type: ddl.StringType},
					},
				},
			},
		},
	}

	code, err := GenerateCRUDRunner(plan, "crud", make(map[string]CRUDOptions))
	if err != nil {
		t.Fatalf("GenerateCRUDRunner failed: %v", err)
	}

	codeStr := string(code)

	// Should have methods for both tables
	if !strings.Contains(codeStr, "InsertAuthor") {
		t.Error("Generated code should have InsertAuthor method")
	}
	if !strings.Contains(codeStr, "InsertPost") {
		t.Error("Generated code should have InsertPost method")
	}

	// Authors has deleted_at, should have HardDelete
	if !strings.Contains(codeStr, "HardDeleteAuthor") {
		t.Error("Generated code should have HardDeleteAuthor for tables with deleted_at")
	}

	// Posts doesn't have deleted_at, should NOT have HardDelete
	if strings.Contains(codeStr, "HardDeletePost") {
		t.Error("Generated code should not have HardDeletePost")
	}

	// Verify it compiles
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "runner.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("Generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestGenerateCRUDSQL_TableWithoutPublicID(t *testing.T) {
	table := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.TextType},
		},
	}

	for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
		t.Run(string(dialect), func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Should use "id" instead of "public_id"
			if strings.Contains(sqlSet.GetSQL, "public_id") {
				t.Error("GetSQL should use id when no public_id")
			}
			if !strings.Contains(sqlSet.GetSQL, quoteIdentifier("id", dialect)) {
				t.Errorf("GetSQL should use id column.\nGot: %s", sqlSet.GetSQL)
			}

			// Insert should NOT have public_id column
			if strings.Contains(sqlSet.InsertSQL, "public_id") {
				t.Error("InsertSQL should not have public_id when column doesn't exist")
			}

			// No RETURNING clause for Postgres/SQLite (no public_id to return)
			if dialect == DialectPostgres || dialect == DialectSQLite {
				if strings.Contains(sqlSet.InsertSQL, "RETURNING") {
					t.Error("InsertSQL should not have RETURNING when no public_id")
				}
			}
		})
	}
}

func TestGenerateCRUDSQL_TableWithoutTimestamps(t *testing.T) {
	table := ddl.Table{
		Name: "configs",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.StringType},
		},
	}

	for _, dialect := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
		t.Run(string(dialect), func(t *testing.T) {
			sqlSet := GenerateCRUDSQL(table, dialect, CRUDOptions{})

			// Insert should NOT have NOW() calls
			if strings.Contains(sqlSet.InsertSQL, "NOW()") || strings.Contains(sqlSet.InsertSQL, "datetime") {
				t.Errorf("InsertSQL should not have timestamp functions when no timestamp columns.\nGot: %s", sqlSet.InsertSQL)
			}

			// Update should NOT have NOW() calls
			if strings.Contains(sqlSet.UpdateSQL, "NOW()") || strings.Contains(sqlSet.UpdateSQL, "datetime") {
				t.Errorf("UpdateSQL should not have timestamp functions when no timestamp columns.\nGot: %s", sqlSet.UpdateSQL)
			}
		})
	}
}
