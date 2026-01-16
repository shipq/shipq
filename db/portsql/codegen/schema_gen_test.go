package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func TestAnalyzeTable_StandardColumns(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "bio", Type: ddl.TextType, Nullable: true},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	analysis := AnalyzeTable(table)

	if !analysis.HasPublicID {
		t.Error("expected HasPublicID = true")
	}
	if !analysis.HasCreatedAt {
		t.Error("expected HasCreatedAt = true")
	}
	if !analysis.HasUpdatedAt {
		t.Error("expected HasUpdatedAt = true")
	}
	if !analysis.HasDeletedAt {
		t.Error("expected HasDeletedAt = true")
	}
	if analysis.PrimaryKey == nil {
		t.Error("expected PrimaryKey to be set")
	} else if analysis.PrimaryKey.Name != "id" {
		t.Errorf("expected PrimaryKey.Name = %q, got %q", "id", analysis.PrimaryKey.Name)
	}

	// UserColumns should only have name, email, bio
	if len(analysis.UserColumns) != 3 {
		t.Errorf("expected 3 UserColumns, got %d", len(analysis.UserColumns))
	} else {
		expectedNames := []string{"name", "email", "bio"}
		for i, col := range analysis.UserColumns {
			if col.Name != expectedNames[i] {
				t.Errorf("expected UserColumns[%d].Name = %q, got %q", i, expectedNames[i], col.Name)
			}
		}
	}

	// ResultColumns should exclude id and deleted_at
	resultNames := make([]string, len(analysis.ResultColumns))
	for i, col := range analysis.ResultColumns {
		resultNames[i] = col.Name
	}
	if contains(resultNames, "id") {
		t.Error("ResultColumns should not contain 'id'")
	}
	if contains(resultNames, "deleted_at") {
		t.Error("ResultColumns should not contain 'deleted_at'")
	}
	if !contains(resultNames, "public_id") {
		t.Error("ResultColumns should contain 'public_id'")
	}
}

func TestAnalyzeTable_NoStandardColumns(t *testing.T) {
	table := ddl.Table{
		Name: "settings",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "key", Type: ddl.StringType},
			{Name: "value", Type: ddl.TextType},
		},
	}

	analysis := AnalyzeTable(table)

	if analysis.HasPublicID {
		t.Error("expected HasPublicID = false")
	}
	if analysis.HasCreatedAt {
		t.Error("expected HasCreatedAt = false")
	}
	if analysis.HasUpdatedAt {
		t.Error("expected HasUpdatedAt = false")
	}
	if analysis.HasDeletedAt {
		t.Error("expected HasDeletedAt = false")
	}

	// UserColumns should have key and value
	if len(analysis.UserColumns) != 2 {
		t.Errorf("expected 2 UserColumns, got %d", len(analysis.UserColumns))
	}
}

func TestMapColumnType(t *testing.T) {
	tests := []struct {
		name       string
		col        ddl.ColumnDefinition
		wantGo     string
		wantColumn string
	}{
		{
			name:       "bigint non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.BigintType, Nullable: false},
			wantGo:     "int64",
			wantColumn: "Int64Column",
		},
		{
			name:       "bigint nullable",
			col:        ddl.ColumnDefinition{Type: ddl.BigintType, Nullable: true},
			wantGo:     "*int64",
			wantColumn: "NullInt64Column",
		},
		{
			name:       "integer non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.IntegerType, Nullable: false},
			wantGo:     "int32",
			wantColumn: "Int32Column",
		},
		{
			name:       "integer nullable",
			col:        ddl.ColumnDefinition{Type: ddl.IntegerType, Nullable: true},
			wantGo:     "*int32",
			wantColumn: "NullInt32Column",
		},
		{
			name:       "decimal non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.DecimalType, Nullable: false},
			wantGo:     "string",
			wantColumn: "DecimalColumn",
		},
		{
			name:       "decimal nullable",
			col:        ddl.ColumnDefinition{Type: ddl.DecimalType, Nullable: true},
			wantGo:     "*string",
			wantColumn: "NullDecimalColumn",
		},
		{
			name:       "float non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.FloatType, Nullable: false},
			wantGo:     "float64",
			wantColumn: "Float64Column",
		},
		{
			name:       "float nullable",
			col:        ddl.ColumnDefinition{Type: ddl.FloatType, Nullable: true},
			wantGo:     "*float64",
			wantColumn: "NullFloat64Column",
		},
		{
			name:       "boolean non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.BooleanType, Nullable: false},
			wantGo:     "bool",
			wantColumn: "BoolColumn",
		},
		{
			name:       "boolean nullable",
			col:        ddl.ColumnDefinition{Type: ddl.BooleanType, Nullable: true},
			wantGo:     "*bool",
			wantColumn: "NullBoolColumn",
		},
		{
			name:       "string non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.StringType, Nullable: false},
			wantGo:     "string",
			wantColumn: "StringColumn",
		},
		{
			name:       "string nullable",
			col:        ddl.ColumnDefinition{Type: ddl.StringType, Nullable: true},
			wantGo:     "*string",
			wantColumn: "NullStringColumn",
		},
		{
			name:       "text non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.TextType, Nullable: false},
			wantGo:     "string",
			wantColumn: "StringColumn",
		},
		{
			name:       "text nullable",
			col:        ddl.ColumnDefinition{Type: ddl.TextType, Nullable: true},
			wantGo:     "*string",
			wantColumn: "NullStringColumn",
		},
		{
			name:       "datetime non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.DatetimeType, Nullable: false},
			wantGo:     "time.Time",
			wantColumn: "TimeColumn",
		},
		{
			name:       "datetime nullable",
			col:        ddl.ColumnDefinition{Type: ddl.DatetimeType, Nullable: true},
			wantGo:     "*time.Time",
			wantColumn: "NullTimeColumn",
		},
		{
			name:       "timestamp non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.TimestampType, Nullable: false},
			wantGo:     "time.Time",
			wantColumn: "TimeColumn",
		},
		{
			name:       "timestamp nullable",
			col:        ddl.ColumnDefinition{Type: ddl.TimestampType, Nullable: true},
			wantGo:     "*time.Time",
			wantColumn: "NullTimeColumn",
		},
		{
			name:       "binary",
			col:        ddl.ColumnDefinition{Type: ddl.BinaryType, Nullable: false},
			wantGo:     "[]byte",
			wantColumn: "BytesColumn",
		},
		{
			name:       "json non-nullable",
			col:        ddl.ColumnDefinition{Type: ddl.JSONType, Nullable: false},
			wantGo:     "json.RawMessage",
			wantColumn: "JSONColumn",
		},
		{
			name:       "json nullable",
			col:        ddl.ColumnDefinition{Type: ddl.JSONType, Nullable: true},
			wantGo:     "json.RawMessage",
			wantColumn: "NullJSONColumn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := MapColumnType(tt.col)
			if mapping.GoType != tt.wantGo {
				t.Errorf("GoType = %q, want %q", mapping.GoType, tt.wantGo)
			}
			if mapping.ColumnType != tt.wantColumn {
				t.Errorf("ColumnType = %q, want %q", mapping.ColumnType, tt.wantColumn)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id", "Id"},
		{"public_id", "PublicId"},
		{"created_at", "CreatedAt"},
		{"author_id", "AuthorId"},
		{"first_name", "FirstName"},
		{"users", "Users"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateTableStruct(t *testing.T) {
	table := ddl.Table{
		Name: "authors",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
			{Name: "bio", Type: ddl.TextType, Nullable: true},
		},
	}

	code, err := GenerateTableStruct(table, "myapp/src/query")
	if err != nil {
		t.Fatalf("GenerateTableStruct failed: %v", err)
	}

	codeStr := string(code)

	// Verify generated code contains expected elements
	expectedStrings := []string{
		"type AuthorsTable struct{}",
		"var Authors = AuthorsTable{}",
		"func (AuthorsTable) Id() query.Int64Column",
		"func (AuthorsTable) Name() query.StringColumn",
		"func (AuthorsTable) Bio() query.NullStringColumn",
		"func (AuthorsTable) TableName() string",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code should contain %q", expected)
		}
	}
}

func TestGenerateSchemaPackage(t *testing.T) {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				"authors": {
					Name: "authors",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "name", Type: ddl.StringType},
					},
				},
				"posts": {
					Name: "posts",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "title", Type: ddl.StringType},
						{Name: "author_id", Type: ddl.BigintType},
					},
				},
			},
		},
	}

	code, err := GenerateSchemaPackage(plan, "myapp/src/query")
	if err != nil {
		t.Fatalf("GenerateSchemaPackage failed: %v", err)
	}

	codeStr := string(code)

	// Verify both tables are generated
	expectedStrings := []string{
		"type AuthorsTable struct{}",
		"var Authors = AuthorsTable{}",
		"type PostsTable struct{}",
		"var Posts = PostsTable{}",
		"func (AuthorsTable) Id() query.Int64Column",
		"func (PostsTable) AuthorId() query.Int64Column",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(codeStr, expected) {
			t.Errorf("generated code should contain %q", expected)
		}
	}
}

func TestGeneratedCodeCompiles(t *testing.T) {
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
						{Name: "bio", Type: ddl.TextType, Nullable: true},
						{Name: "created_at", Type: ddl.DatetimeType},
						{Name: "updated_at", Type: ddl.DatetimeType},
						{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
					},
				},
				"posts": {
					Name: "posts",
					Columns: []ddl.ColumnDefinition{
						{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
						{Name: "author_id", Type: ddl.BigintType},
						{Name: "title", Type: ddl.StringType},
						{Name: "content", Type: ddl.TextType},
						{Name: "published", Type: ddl.BooleanType},
						{Name: "metadata", Type: ddl.JSONType, Nullable: true},
					},
				},
			},
		},
	}

	code, err := GenerateSchemaPackage(plan, "myapp/src/query")
	if err != nil {
		t.Fatalf("GenerateSchemaPackage failed: %v", err)
	}

	// Use go/parser to verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "schema.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

func TestGenerateAllColumnTypes(t *testing.T) {
	// Test that all DDL types generate valid code
	table := ddl.Table{
		Name: "all_types",
		Columns: []ddl.ColumnDefinition{
			{Name: "int_col", Type: ddl.IntegerType},
			{Name: "int_null", Type: ddl.IntegerType, Nullable: true},
			{Name: "bigint_col", Type: ddl.BigintType},
			{Name: "bigint_null", Type: ddl.BigintType, Nullable: true},
			{Name: "decimal_col", Type: ddl.DecimalType},
			{Name: "decimal_null", Type: ddl.DecimalType, Nullable: true},
			{Name: "float_col", Type: ddl.FloatType},
			{Name: "float_null", Type: ddl.FloatType, Nullable: true},
			{Name: "bool_col", Type: ddl.BooleanType},
			{Name: "bool_null", Type: ddl.BooleanType, Nullable: true},
			{Name: "string_col", Type: ddl.StringType},
			{Name: "string_null", Type: ddl.StringType, Nullable: true},
			{Name: "text_col", Type: ddl.TextType},
			{Name: "text_null", Type: ddl.TextType, Nullable: true},
			{Name: "datetime_col", Type: ddl.DatetimeType},
			{Name: "datetime_null", Type: ddl.DatetimeType, Nullable: true},
			{Name: "timestamp_col", Type: ddl.TimestampType},
			{Name: "timestamp_null", Type: ddl.TimestampType, Nullable: true},
			{Name: "binary_col", Type: ddl.BinaryType},
			{Name: "json_col", Type: ddl.JSONType},
			{Name: "json_null", Type: ddl.JSONType, Nullable: true},
		},
	}

	code, err := GenerateTableStruct(table, "myapp/src/query")
	if err != nil {
		t.Fatalf("GenerateTableStruct failed: %v", err)
	}

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "schema.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code should be valid Go: %v\n\nGenerated code:\n%s", err, string(code))
	}
}

// Helper function
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
