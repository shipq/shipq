package handlergen

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
)

func TestResourceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"posts", "Post"},
		{"users", "User"},
		{"comments", "Comment"},
		{"categories", "Category"},
		{"post_tags", "PostTag"},
		{"user_profiles", "UserProfile"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := resourceName(tt.input)
			if result != tt.expected {
				t.Errorf("resourceName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"hello_world", "HelloWorld"},
		{"created_at", "CreatedAt"},
		{"public_id", "PublicId"},
		{"id", "Id"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello_world", "helloWorld"},
		{"created_at", "createdAt"},
		{"ID", "iD"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toCamelCase(tt.input)
			if result != tt.expected {
				t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToSingular(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"posts", "post"},
		{"users", "user"},
		{"categories", "category"},
		{"boxes", "box"},
		{"churches", "church"},
		{"post", "post"}, // already singular
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSingular(tt.input)
			if result != tt.expected {
				t.Errorf("toSingular(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToEmbedFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"author_id", "author"},
		{"category_id", "category"},
		{"user_id", "user"},
		{"organization_id", "organization"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toEmbedFieldName(tt.input)
			if result != tt.expected {
				t.Errorf("toEmbedFieldName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGoBaseType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{ddl.IntegerType, "int32"},
		{ddl.BigintType, "int64"},
		{ddl.DecimalType, "float64"},
		{ddl.FloatType, "float64"},
		{ddl.BooleanType, "bool"},
		{ddl.StringType, "string"},
		{ddl.TextType, "string"},
		{ddl.DatetimeType, "time.Time"},
		{ddl.TimestampType, "time.Time"},
		{ddl.BinaryType, "[]byte"},
		{ddl.JSONType, "json.RawMessage"},
		{"unknown", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := goBaseType(tt.input)
			if result != tt.expected {
				t.Errorf("goBaseType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGoTypeForColumn(t *testing.T) {
	tests := []struct {
		name     string
		col      ddl.ColumnDefinition
		expected string
	}{
		{
			name:     "non-nullable string",
			col:      ddl.ColumnDefinition{Type: ddl.StringType, Nullable: false},
			expected: "string",
		},
		{
			name:     "nullable string",
			col:      ddl.ColumnDefinition{Type: ddl.StringType, Nullable: true},
			expected: "*string",
		},
		{
			name:     "non-nullable int",
			col:      ddl.ColumnDefinition{Type: ddl.IntegerType, Nullable: false},
			expected: "int32",
		},
		{
			name:     "nullable bigint",
			col:      ddl.ColumnDefinition{Type: ddl.BigintType, Nullable: true},
			expected: "*int64",
		},
		{
			name:     "non-nullable timestamp",
			col:      ddl.ColumnDefinition{Type: ddl.TimestampType, Nullable: false},
			expected: "time.Time",
		},
		{
			name:     "nullable datetime",
			col:      ddl.ColumnDefinition{Type: ddl.DatetimeType, Nullable: true},
			expected: "*time.Time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := goTypeForColumn(tt.col)
			if result != tt.expected {
				t.Errorf("goTypeForColumn() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsAutoColumn(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"id", true},
		{"public_id", true},
		{"created_at", true},
		{"updated_at", true},
		{"deleted_at", true},
		{"title", false},
		{"content", false},
		{"author_id", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isAutoColumn(tt.input)
			if result != tt.expected {
				t.Errorf("isAutoColumn(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetEmbeddableColumns(t *testing.T) {
	table := ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "email", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}

	result := getEmbeddableColumns(table)

	// Should exclude id and deleted_at
	expected := []string{"public_id", "name", "email", "created_at", "updated_at"}

	if len(result) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(result))
	}

	for i, col := range result {
		if col != expected[i] {
			t.Errorf("column %d: expected %q, got %q", i, expected[i], col)
		}
	}
}

func TestAnalyzeRelationships_DirectFK(t *testing.T) {
	schema := map[string]ddl.Table{
		"posts": {
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "author_id", Type: ddl.StringType, References: "users"},
			},
		},
		"users": {
			Name: "users",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
			},
		},
	}

	relations := AnalyzeRelationships(schema["posts"], schema)

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	rel := relations[0]
	if rel.FieldName != "author" {
		t.Errorf("expected FieldName 'author', got %q", rel.FieldName)
	}
	if rel.TargetTable != "users" {
		t.Errorf("expected TargetTable 'users', got %q", rel.TargetTable)
	}
	if rel.IsMany {
		t.Error("expected IsMany false for direct FK")
	}
	if rel.FKColumn != "author_id" {
		t.Errorf("expected FKColumn 'author_id', got %q", rel.FKColumn)
	}
}

func TestAnalyzeRelationships_ManyToMany(t *testing.T) {
	schema := map[string]ddl.Table{
		"posts": {
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
			},
		},
		"tags": {
			Name: "tags",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
			},
		},
		"post_tags": {
			Name:            "post_tags",
			IsJunctionTable: true,
			Columns: []ddl.ColumnDefinition{
				{Name: "post_id", Type: ddl.StringType, References: "posts"},
				{Name: "tag_id", Type: ddl.StringType, References: "tags"},
			},
		},
	}

	relations := AnalyzeRelationships(schema["posts"], schema)

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	rel := relations[0]
	if rel.FieldName != "tags" {
		t.Errorf("expected FieldName 'tags', got %q", rel.FieldName)
	}
	if rel.TargetTable != "tags" {
		t.Errorf("expected TargetTable 'tags', got %q", rel.TargetTable)
	}
	if !rel.IsMany {
		t.Error("expected IsMany true for many-to-many")
	}
}

func TestHasNullableTime(t *testing.T) {
	tests := []struct {
		name     string
		table    ddl.Table
		expected bool
	}{
		{
			name: "has nullable time",
			table: ddl.Table{
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType},
					{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
				},
			},
			expected: true,
		},
		{
			name: "no nullable time",
			table: ddl.Table{
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType},
					{Name: "created_at", Type: ddl.TimestampType, Nullable: false},
				},
			},
			expected: false,
		},
		{
			name: "nullable non-time",
			table: ddl.Table{
				Columns: []ddl.ColumnDefinition{
					{Name: "id", Type: ddl.BigintType},
					{Name: "description", Type: ddl.TextType, Nullable: true},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasNullableTime(tt.table)
			if result != tt.expected {
				t.Errorf("hasNullableTime() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateCreateHandler(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "content", Type: ddl.TextType},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Check package
	if !strings.Contains(code, "package posts") {
		t.Error("expected package posts")
	}

	// Check imports use embedded lib path
	if !strings.Contains(code, `"myapp/shipq/lib/httperror"`) {
		t.Error("expected embedded httperror import")
	}
	if strings.Contains(code, `"github.com/shipq/shipq/`) {
		t.Error("generated code must NOT import from github.com/shipq/shipq")
	}
	if !strings.Contains(code, `"myapp/shipq/queries"`) {
		t.Error("expected queries import")
	}

	// Check request struct
	if !strings.Contains(code, "type CreatePostRequest struct") {
		t.Error("expected CreatePostRequest struct")
	}
	if !strings.Contains(code, "Title") || !strings.Contains(code, `json:"title"`) {
		t.Error("expected Title field")
	}
	if !strings.Contains(code, "Content") || !strings.Contains(code, `json:"content"`) {
		t.Error("expected Content field")
	}

	// Check response struct
	if !strings.Contains(code, "type CreatePostResponse struct") {
		t.Error("expected CreatePostResponse struct")
	}

	// Check handler function
	if !strings.Contains(code, "func CreatePost(ctx context.Context, req *CreatePostRequest)") {
		t.Error("expected CreatePost function")
	}

	// Check it calls the query runner
	if !strings.Contains(code, "runner.CreatePost(ctx") {
		t.Error("expected runner.CreatePost call")
	}
}

func TestGenerateGetOneHandler(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateGetOneHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Check request struct
	if !strings.Contains(code, "type GetPostRequest struct") {
		t.Error("expected GetPostRequest struct")
	}
	if !strings.Contains(code, "ID string") || !strings.Contains(code, `path:"id"`) {
		t.Error("expected ID path parameter")
	}

	// Check response struct
	if !strings.Contains(code, "type GetPostResponse struct") {
		t.Error("expected GetPostResponse struct")
	}

	// Check handler function
	if !strings.Contains(code, "func GetPost(ctx context.Context, req *GetPostRequest)") {
		t.Error("expected GetPost function")
	}

	// Check 404 handling
	if !strings.Contains(code, "httperror.NotFoundf") {
		t.Error("expected NotFoundf error")
	}
}

func TestGenerateListHandler(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Check request struct
	if !strings.Contains(code, "type ListPostsRequest struct") {
		t.Error("expected ListPostsRequest struct")
	}
	if !strings.Contains(code, `Limit  int`) {
		t.Error("expected Limit field")
	}
	if !strings.Contains(code, `Cursor *string`) {
		t.Error("expected Cursor field")
	}

	// Check item struct
	if !strings.Contains(code, "type PostItem struct") {
		t.Error("expected PostItem struct")
	}

	// Check response struct
	if !strings.Contains(code, "type ListPostsResponse struct") {
		t.Error("expected ListPostsResponse struct")
	}
	if !strings.Contains(code, `Items      []PostItem`) {
		t.Error("expected Items field")
	}
	if !strings.Contains(code, `NextCursor *string`) {
		t.Error("expected NextCursor field")
	}

	// Check handler function
	if !strings.Contains(code, "func ListPosts(ctx context.Context, req *ListPostsRequest)") {
		t.Error("expected ListPosts function")
	}

	// Check pagination logic
	if !strings.Contains(code, "if limit <= 0 || limit > 100") {
		t.Error("expected limit validation")
	}
}

func TestGenerateUpdateHandler(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "content", Type: ddl.TextType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateUpdateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Check request struct
	if !strings.Contains(code, "type UpdatePostRequest struct") {
		t.Error("expected UpdatePostRequest struct")
	}
	if !strings.Contains(code, "ID") || !strings.Contains(code, `path:"id"`) {
		t.Error("expected ID path parameter")
	}
	// Fields should be pointers for optional updates
	if !strings.Contains(code, "Title") || !strings.Contains(code, "*string") {
		t.Error("expected Title as pointer type")
	}

	// Check handler function
	if !strings.Contains(code, "func UpdatePost(ctx context.Context, req *UpdatePostRequest)") {
		t.Error("expected UpdatePost function")
	}

	// Check it calls UpdateByPublicID
	if !strings.Contains(code, "runner.UpdatePostByPublicID") {
		t.Error("expected runner.UpdatePostByPublicID call")
	}
}

func TestGenerateSoftDeleteHandler(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateSoftDeleteHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Check request struct
	if !strings.Contains(code, "type SoftDeletePostRequest struct") {
		t.Error("expected SoftDeletePostRequest struct")
	}

	// Check response struct
	if !strings.Contains(code, "type SoftDeletePostResponse struct") {
		t.Error("expected SoftDeletePostResponse struct")
	}
	if !strings.Contains(code, `Success bool`) {
		t.Error("expected Success field")
	}

	// Check handler function
	if !strings.Contains(code, "func SoftDeletePost(ctx context.Context, req *SoftDeletePostRequest)") {
		t.Error("expected SoftDeletePost function")
	}

	// Check it calls SoftDeleteByPublicID
	if !strings.Contains(code, "runner.SoftDeletePostByPublicID") {
		t.Error("expected runner.SoftDeletePostByPublicID call")
	}
}

func TestGenerateRegister(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateRegister(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Check package
	if !strings.Contains(code, "package posts") {
		t.Error("expected package posts")
	}

	// Check import uses embedded lib path
	if !strings.Contains(code, `"myapp/shipq/lib/handler"`) {
		t.Error("expected embedded handler import")
	}
	if strings.Contains(code, `"github.com/shipq/shipq/`) {
		t.Error("generated code must NOT import from github.com/shipq/shipq")
	}

	// Check Register function
	if !strings.Contains(code, "func Register(app *handler.App)") {
		t.Error("expected Register function")
	}

	// Check all CRUD registrations
	expectedRegistrations := []string{
		`app.Post("/posts", CreatePost)`,
		`app.Get("/posts", ListPosts)`,
		`app.Get("/posts/:id", GetPost)`,
		`app.Patch("/posts/:id", UpdatePost)`,
		`app.Delete("/posts/:id", SoftDeletePost)`,
	}

	for _, reg := range expectedRegistrations {
		if !strings.Contains(code, reg) {
			t.Errorf("expected registration: %s", reg)
		}
	}
}

func TestGenerateRegister_WithAuth(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       ddl.Table{Name: "posts"},
		Schema:      make(map[string]ddl.Table),
		RequireAuth: true,
	}

	result, err := GenerateRegister(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// All routes should use the builder pattern with .Auth()
	expectedRegistrations := []string{
		`app.Post("/posts", CreatePost).Auth()`,
		`app.Get("/posts", ListPosts).Auth()`,
		`app.Get("/posts/:id", GetPost).Auth()`,
		`app.Patch("/posts/:id", UpdatePost).Auth()`,
		`app.Delete("/posts/:id", SoftDeletePost).Auth()`,
	}

	for _, reg := range expectedRegistrations {
		if !strings.Contains(code, reg) {
			t.Errorf("expected registration: %s\ngot:\n%s", reg, code)
		}
	}

	// Should NOT contain old-style AuthPost/AuthGet patterns
	if strings.Contains(code, "AuthPost") || strings.Contains(code, "AuthGet") {
		t.Error("should not contain old-style Auth* methods")
	}
}

func TestGenerateHandlerFiles(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	files, err := GenerateHandlerFiles(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := []string{
		"create.go",
		"get_one.go",
		"list.go",
		"update.go",
		"soft_delete.go",
		"register.go",
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("expected %d files, got %d", len(expectedFiles), len(files))
	}

	for _, filename := range expectedFiles {
		if _, ok := files[filename]; !ok {
			t.Errorf("expected file %q to be generated", filename)
		}
	}
}

func TestSortedTableNames(t *testing.T) {
	schema := map[string]ddl.Table{
		"zebras":    {Name: "zebras"},
		"alpacas":   {Name: "alpacas"},
		"monkeys":   {Name: "monkeys"},
		"elephants": {Name: "elephants"},
	}

	result := SortedTableNames(schema)

	expected := []string{"alpacas", "elephants", "monkeys", "zebras"}

	if len(result) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(result))
	}

	for i, name := range result {
		if name != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], name)
		}
	}
}

func TestResponseFieldType(t *testing.T) {
	tests := []struct {
		name     string
		col      ddl.ColumnDefinition
		expected string
	}{
		{
			name:     "timestamp becomes string",
			col:      ddl.ColumnDefinition{Type: ddl.TimestampType},
			expected: "string",
		},
		{
			name:     "datetime becomes string",
			col:      ddl.ColumnDefinition{Type: ddl.DatetimeType},
			expected: "string",
		},
		{
			name:     "string stays string",
			col:      ddl.ColumnDefinition{Type: ddl.StringType},
			expected: "string",
		},
		{
			name:     "nullable string",
			col:      ddl.ColumnDefinition{Type: ddl.StringType, Nullable: true},
			expected: "*string",
		},
		{
			name:     "int maps to int32",
			col:      ddl.ColumnDefinition{Type: ddl.IntegerType},
			expected: "int32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := responseFieldType(tt.col)
			if result != tt.expected {
				t.Errorf("responseFieldType() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestUpdateHandler_NullableFieldTypes(t *testing.T) {
	// Bug 2: Update handler request fields must use double-pointer for nullable
	// columns to match the query runner's UpdateParams convention, where
	// MapColumnType already includes "*" for nullable types and then another "*"
	// is added for PATCH semantics.
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "items",
		Table: ddl.Table{
			Name: "items",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "description", Type: ddl.StringType, Nullable: true},
				{Name: "weight", Type: ddl.BigintType, Nullable: true},
				{Name: "count", Type: ddl.IntegerType, Nullable: false},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateUpdateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Helper to check struct fields (handles tab-aligned formatting from go/format)
	containsField := func(code, fieldName, fieldType string) bool {
		// go/format aligns struct fields with tabs, so we check for the field name
		// followed by whitespace and the type
		for _, line := range strings.Split(code, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, fieldName+" ") || strings.HasPrefix(trimmed, fieldName+"\t") {
				return strings.Contains(trimmed, fieldType)
			}
		}
		return false
	}

	// Non-nullable string -> *string (one pointer for PATCH optional)
	if !containsField(code, "Name", "*string") {
		t.Error("expected Name to be *string for non-nullable string update field")
	}

	// Nullable string -> **string (pointer-to-pointer: nullable + PATCH optional)
	if !containsField(code, "Description", "**string") {
		t.Errorf("expected Description to be **string for nullable string update field, got:\n%s", code)
	}

	// Nullable bigint -> **int64 (pointer-to-pointer: nullable + PATCH optional)
	if !containsField(code, "Weight", "**int64") {
		t.Errorf("expected Weight to be **int64 for nullable bigint update field, got:\n%s", code)
	}

	// Non-nullable int -> *int32 (one pointer for PATCH optional)
	if !containsField(code, "Count", "*int32") {
		t.Errorf("expected Count to be *int32 for non-nullable integer update field, got:\n%s", code)
	}
}

func TestListHandler_FKColumnTypes(t *testing.T) {
	// FK columns in list responses are resolved to the referenced row's
	// public_id (string) via JOIN + SelectAs in the query layer.
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "sessions",
		Table: ddl.Table{
			Name: "sessions",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "account_id", Type: ddl.BigintType, Nullable: false, References: "accounts"},
				{Name: "org_id", Type: ddl.BigintType, Nullable: true, References: "organizations"},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Helper to check struct fields (handles tab-aligned formatting from go/format)
	containsField := func(code, fieldName, fieldType string) bool {
		for _, line := range strings.Split(code, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, fieldName+" ") || strings.HasPrefix(trimmed, fieldName+"\t") {
				return strings.Contains(trimmed, fieldType)
			}
		}
		return false
	}

	// Non-nullable FK -> string (resolved to referenced row's public_id)
	if !containsField(code, "AccountId", "string") {
		t.Errorf("expected AccountId to be string in list item struct (resolved FK), got:\n%s", code)
	}
	if containsField(code, "AccountId", "int64") {
		t.Error("AccountId should NOT be int64 in list item; FK columns resolve to string public_id")
	}

	// Nullable FK -> *string (resolved to referenced row's public_id, nullable)
	if !containsField(code, "OrgId", "*string") {
		t.Errorf("expected OrgId to be *string in list item struct (resolved nullable FK), got:\n%s", code)
	}
	if containsField(code, "OrgId", "*int64") {
		t.Error("OrgId should NOT be *int64 in list item; nullable FK columns resolve to *string public_id")
	}
}

func TestGetOneHandler_WithRelations_UsesStandardGet(t *testing.T) {
	// Bug 4: When relations exist, the get-one handler must still use the standard
	// GetByPublicID method (not a nonexistent WithRelations method).
	schema := map[string]ddl.Table{
		"posts": {
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "author_id", Type: ddl.StringType, References: "users"},
				{Name: "created_at", Type: ddl.TimestampType},
			},
		},
		"users": {
			Name: "users",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
			},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table:      schema["posts"],
		Schema:     schema,
	}

	// AnalyzeRelationships should find the author FK relation
	relations := AnalyzeRelationships(cfg.Table, cfg.Schema)
	if len(relations) == 0 {
		t.Fatal("expected at least one relation for posts -> users")
	}

	result, err := GenerateGetOneHandler(cfg, relations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Must use the standard get method, NOT WithRelations
	if strings.Contains(code, "WithRelations") {
		t.Error("generated code must NOT call WithRelations (method does not exist on runner)")
	}
	if !strings.Contains(code, "runner.GetPostByPublicID(ctx, queries.GetPostByPublicIDParams{") {
		t.Errorf("expected runner.GetPostByPublicID call with params struct, got:\n%s", code)
	}
	if !strings.Contains(code, "PublicId: req.ID,") {
		t.Errorf("expected PublicId: req.ID in params struct, got:\n%s", code)
	}
}

// =============================================================================
// Author Account ID Column Tests
// =============================================================================

func TestIsAutoColumn_IncludesAuthorAccountID(t *testing.T) {
	autoColumns := []string{"id", "public_id", "created_at", "updated_at", "deleted_at", "author_account_id"}
	for _, col := range autoColumns {
		if !isAutoColumn(col) {
			t.Errorf("isAutoColumn(%q) should return true", col)
		}
	}

	nonAutoColumns := []string{"title", "email", "name", "organization_id", "user_id"}
	for _, col := range nonAutoColumns {
		if isAutoColumn(col) {
			t.Errorf("isAutoColumn(%q) should return false", col)
		}
	}
}

func TestIsResponseExcluded_IncludesAuthorAccountID(t *testing.T) {
	excludedColumns := []string{"id", "deleted_at", "author_account_id"}
	for _, col := range excludedColumns {
		if !isResponseExcluded(col) {
			t.Errorf("isResponseExcluded(%q) should return true", col)
		}
	}

	includedColumns := []string{"public_id", "created_at", "updated_at", "title", "email"}
	for _, col := range includedColumns {
		if isResponseExcluded(col) {
			t.Errorf("isResponseExcluded(%q) should return false", col)
		}
	}
}

func TestGenerateCreateHandler_ExcludesAuthorAccountID(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "body", Type: ddl.TextType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("GenerateCreateHandler failed: %v", err)
	}

	code := string(result)

	// Request struct should NOT have AuthorAccountId field (it comes from session context)
	// Extract the request struct portion of the generated code
	reqStructStart := strings.Index(code, "type CreatePostRequest struct")
	reqStructEnd := strings.Index(code[reqStructStart:], "}\n") + reqStructStart
	reqStruct := code[reqStructStart : reqStructEnd+1]
	if strings.Contains(reqStruct, "AuthorAccountId") {
		t.Error("Create request struct should NOT contain AuthorAccountId field")
	}

	// But the create params SHOULD set AuthorAccountId from session context
	if !strings.Contains(code, "AuthorAccountId: accountID") {
		t.Error("Create params should set AuthorAccountId: accountID from session context")
	}

	// Should have Title and Body fields
	if !strings.Contains(code, "Title") {
		t.Error("Create request struct should contain Title field")
	}
	if !strings.Contains(code, "Body") {
		t.Error("Create request struct should contain Body field")
	}
}

func TestGenerateUpdateHandler_ExcludesAuthorAccountID(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "body", Type: ddl.TextType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateUpdateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("GenerateUpdateHandler failed: %v", err)
	}

	code := string(result)

	// Request/response structs should NOT have AuthorAccountId
	if strings.Contains(code, "AuthorAccountId") {
		t.Error("Update handler should NOT contain AuthorAccountId field")
	}

	// Should still have Title and Body
	if !strings.Contains(code, "Title") {
		t.Error("Update handler should contain Title field")
	}
}

// =============================================================================
// Scope Column (Tenancy) Tests
// =============================================================================

func TestGenerateCreateHandler_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "organization_id",
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Request struct should NOT include OrganizationId.
	// The response struct may include it, so check specifically in the request struct section.
	reqStart := strings.Index(code, "type CreatePostRequest struct")
	reqEnd := strings.Index(code[reqStart:], "}")
	reqSection := code[reqStart : reqStart+reqEnd]
	if strings.Contains(reqSection, "OrganizationId") {
		t.Error("Create request struct should NOT contain OrganizationId field when scoped")
	}

	// Should import httputil for OrganizationIDFromContext
	if !strings.Contains(code, `/shipq/lib/httputil"`) {
		t.Error("expected httputil import when scoped")
	}

	// Should call OrganizationIDFromContext
	if !strings.Contains(code, "httputil.OrganizationIDFromContext(ctx)") {
		t.Error("expected OrganizationIDFromContext call")
	}

	// Should pass orgID to create params
	if !strings.Contains(code, "OrganizationId: orgID") {
		t.Errorf("expected OrganizationId: orgID in create params, got:\n%s", code)
	}
}

func TestGenerateListHandler_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "created_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "organization_id",
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Request struct should NOT include organization_id query param
	if strings.Contains(code, `query:"organization_id"`) {
		t.Error("List request struct should NOT contain organization_id query param when scoped")
	}

	// Should call OrganizationIDFromContext
	if !strings.Contains(code, "httputil.OrganizationIDFromContext(ctx)") {
		t.Error("expected OrganizationIDFromContext call")
	}

	// Should pass orgID inside params struct
	if !strings.Contains(code, "OrganizationId: orgID,") {
		t.Error("expected OrganizationId: orgID in list params struct")
	}
}

func TestGenerateGetOneHandler_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "created_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "organization_id",
	}

	result, err := GenerateGetOneHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Should call OrganizationIDFromContext
	if !strings.Contains(code, "httputil.OrganizationIDFromContext(ctx)") {
		t.Error("expected OrganizationIDFromContext call")
	}

	// Should pass orgID in the params struct (not as a separate argument)
	if !strings.Contains(code, "runner.GetPostByPublicID(ctx, queries.GetPostByPublicIDParams{") {
		t.Errorf("expected GetPostByPublicID with params struct, got:\n%s", code)
	}
	if !strings.Contains(code, "OrganizationId: orgID,") {
		t.Errorf("expected OrganizationId: orgID in params struct, got:\n%s", code)
	}
}

func TestGenerateUpdateHandler_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "organization_id",
	}

	result, err := GenerateUpdateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Request struct should NOT include organization_id.
	// The response struct may include it, so check specifically in the request struct section.
	reqStart := strings.Index(code, "type UpdatePostRequest struct")
	reqEnd := strings.Index(code[reqStart:], "}")
	reqSection := code[reqStart : reqStart+reqEnd]
	if strings.Contains(reqSection, "OrganizationId") {
		t.Error("Update request struct should NOT contain OrganizationId when scoped")
	}

	// Should call OrganizationIDFromContext
	if !strings.Contains(code, "httputil.OrganizationIDFromContext(ctx)") {
		t.Error("expected OrganizationIDFromContext call")
	}

	// Should pass orgID in the update params struct (not as a separate argument)
	if !strings.Contains(code, "OrganizationId: orgID,") {
		t.Errorf("expected OrganizationId: orgID in update params struct, got:\n%s", code)
	}
}

func TestGenerateSoftDeleteHandler_WithScopeColumn(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "organization_id",
	}

	result, err := GenerateSoftDeleteHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Should import httputil
	if !strings.Contains(code, `/shipq/lib/httputil"`) {
		t.Error("expected httputil import when scoped")
	}

	// Should call OrganizationIDFromContext
	if !strings.Contains(code, "httputil.OrganizationIDFromContext(ctx)") {
		t.Error("expected OrganizationIDFromContext call")
	}

	// Should pass orgID in the soft delete params struct (not as a separate argument)
	if !strings.Contains(code, "runner.SoftDeletePostByPublicID(ctx, queries.SoftDeletePostByPublicIDParams{") {
		t.Errorf("expected SoftDeletePostByPublicID with params struct, got:\n%s", code)
	}
	if !strings.Contains(code, "OrganizationId: orgID,") {
		t.Errorf("expected OrganizationId: orgID in soft delete params struct, got:\n%s", code)
	}
}

// =============================================================================
// §3.2 — FK String Types, Author Embed, Create Re-fetch Tests
// =============================================================================

func TestGenerateListHandler_FKFieldsAreStrings(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "category_id", Type: ddl.BigintType, References: "categories"},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	containsField := func(code, fieldName, fieldType string) bool {
		for _, line := range strings.Split(code, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, fieldName+" ") || strings.HasPrefix(trimmed, fieldName+"\t") {
				return strings.Contains(trimmed, fieldType)
			}
		}
		return false
	}

	// FK column category_id should be string (resolved public_id)
	if !containsField(code, "CategoryId", "string") {
		t.Errorf("expected CategoryId to be string in list item struct, got:\n%s", code)
	}
	if containsField(code, "CategoryId", "int64") {
		t.Error("CategoryId should NOT be int64 in list item struct; FK columns resolve to string")
	}
}

func TestGenerateGetOneHandler_FKFieldsAreStrings(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "category_id", Type: ddl.BigintType, References: "categories"},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateGetOneHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	containsField := func(code, fieldName, fieldType string) bool {
		for _, line := range strings.Split(code, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, fieldName+" ") || strings.HasPrefix(trimmed, fieldName+"\t") {
				return strings.Contains(trimmed, fieldType)
			}
		}
		return false
	}

	if !containsField(code, "CategoryId", "string") {
		t.Errorf("expected CategoryId to be string in get-one response struct, got:\n%s", code)
	}
	if containsField(code, "CategoryId", "int64") {
		t.Error("CategoryId should NOT be int64 in get-one response; FK columns resolve to string")
	}
}

func TestGenerateCreateHandler_RefetchesAfterInsert(t *testing.T) {
	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "posts",
		Table: ddl.Table{
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "category_id", Type: ddl.BigintType, References: "categories"},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
			},
		},
		Schema: make(map[string]ddl.Table),
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Should call the CREATE method
	if !strings.Contains(code, "runner.CreatePost(ctx") {
		t.Error("expected runner.CreatePost call")
	}

	// Should re-fetch via the GET method after INSERT
	if !strings.Contains(code, "runner.GetPostByPublicID(ctx") {
		t.Error("expected runner.GetPostByPublicID re-fetch call after INSERT")
	}

	// The INSERT result should be discarded (assigned to _)
	if !strings.Contains(code, "_, err := runner.CreatePost(ctx") {
		t.Error("expected INSERT result to be discarded with _, err := pattern")
	}

	// Should generate a publicId before the INSERT
	if !strings.Contains(code, "publicId := nanoid.New()") {
		t.Error("expected publicId := nanoid.New() before INSERT")
	}
}

func TestGenerateListHandler_AuthorEmbed(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// AuthorEmbed struct should NOT be in the handler file (it lives in types.go)
	if strings.Contains(code, "type AuthorEmbed struct") {
		t.Error("AuthorEmbed struct definition should NOT be in list handler (should be in types.go)")
	}

	// Item struct should have Author *AuthorEmbed field
	// go/format aligns struct fields with tabs, so check flexibly
	hasAuthorField := false
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Author") && strings.Contains(trimmed, "*AuthorEmbed") {
			hasAuthorField = true
			break
		}
	}
	if !hasAuthorField {
		t.Error("expected Author *AuthorEmbed field in item struct")
	}

	// Mapping code should populate Author from flat fields
	if !strings.Contains(code, "item.AuthorId") {
		t.Error("expected mapping from item.AuthorId")
	}
	if !strings.Contains(code, "item.AuthorEmail") {
		t.Error("expected mapping from item.AuthorEmail")
	}
	if !strings.Contains(code, "item.AuthorFirstName") {
		t.Error("expected mapping from item.AuthorFirstName")
	}
	if !strings.Contains(code, "item.AuthorLastName") {
		t.Error("expected mapping from item.AuthorLastName")
	}
}

func TestGenerateGetOneHandler_AuthorEmbed(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateGetOneHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// AuthorEmbed struct should NOT be in the handler file (it lives in types.go)
	if strings.Contains(code, "type AuthorEmbed struct") {
		t.Error("AuthorEmbed struct definition should NOT be in get_one handler (should be in types.go)")
	}

	// Response struct should have Author *AuthorEmbed field
	// go/format aligns struct fields with tabs, so check flexibly
	hasAuthorField := false
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Author") && strings.Contains(trimmed, "*AuthorEmbed") {
			hasAuthorField = true
			break
		}
	}
	if !hasAuthorField {
		t.Error("expected Author *AuthorEmbed field in response struct")
	}

	// Mapping code should populate Author from flat result fields
	if !strings.Contains(code, "result.AuthorId") {
		t.Error("expected mapping from result.AuthorId")
	}
	if !strings.Contains(code, "result.AuthorEmail") {
		t.Error("expected mapping from result.AuthorEmail")
	}
	if !strings.Contains(code, "result.AuthorFirstName") {
		t.Error("expected mapping from result.AuthorFirstName")
	}
	if !strings.Contains(code, "result.AuthorLastName") {
		t.Error("expected mapping from result.AuthorLastName")
	}
}

func TestGenerateTypesFile_AuthorEmbed(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateTypesFile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	// Should contain AuthorEmbed struct
	if !strings.Contains(code, "type AuthorEmbed struct") {
		t.Error("expected AuthorEmbed struct definition in types.go")
	}

	// AuthorEmbed should have Id, Email, FirstName, LastName
	if !strings.Contains(code, `Id        string`) {
		t.Error("AuthorEmbed missing Id field")
	}
	if !strings.Contains(code, `Email     string`) {
		t.Error("AuthorEmbed missing Email field")
	}
	if !strings.Contains(code, `FirstName string`) {
		t.Error("AuthorEmbed missing FirstName field")
	}
	if !strings.Contains(code, `LastName  string`) {
		t.Error("AuthorEmbed missing LastName field")
	}

	// Should have correct package declaration
	if !strings.Contains(code, "package posts") {
		t.Error("expected package posts declaration")
	}
}

func TestGenerateHandlerFiles_AuthorEmbed_InTypesFile(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	files, err := GenerateHandlerFiles(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// types.go should be generated
	typesContent, ok := files["types.go"]
	if !ok {
		t.Fatal("expected types.go to be generated for table with author_account_id")
	}

	// types.go should contain AuthorEmbed struct exactly once
	typesCode := string(typesContent)
	if !strings.Contains(typesCode, "type AuthorEmbed struct") {
		t.Error("types.go should contain AuthorEmbed struct definition")
	}

	// Individual handler files should NOT contain AuthorEmbed struct definition
	for _, filename := range []string{"create.go", "get_one.go", "list.go", "update.go"} {
		content, ok := files[filename]
		if !ok {
			t.Errorf("expected %s to be generated", filename)
			continue
		}
		code := string(content)
		if strings.Contains(code, "type AuthorEmbed struct") {
			t.Errorf("%s should NOT contain AuthorEmbed struct definition (it belongs in types.go)", filename)
		}
		// But handlers should still reference AuthorEmbed
		if !strings.Contains(code, "AuthorEmbed") {
			t.Errorf("%s should reference AuthorEmbed", filename)
		}
	}
}

func TestGenerateListHandler_NoAuthorWhenAbsent(t *testing.T) {
	// Table WITHOUT author_account_id should NOT have AuthorEmbed
	table := ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath: "myapp",
		TableName:  "categories",
		Table:      table,
		Schema:     map[string]ddl.Table{"categories": table},
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(result)

	if strings.Contains(code, "AuthorEmbed") {
		t.Error("list handler for table WITHOUT author_account_id should NOT contain AuthorEmbed")
	}
}

func TestGenerateCreateHandler_DiscardsInsertResult(t *testing.T) {
	// Safety-net test for Bug 2: the Create handler must assign the Create
	// result to _ (i.e., `_, err := runner.Create...`). This ensures the
	// internal Id field on CreateResult is never referenced in handler code,
	// even though we now return it in the RETURNING clause for fixture use.
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "organization_id", Type: ddl.BigintType},
			{Name: "author_account_id", Type: ddl.BigintType},
			{Name: "created_at", Type: ddl.TimestampType},
			{Name: "updated_at", Type: ddl.TimestampType},
			{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "organization_id",
		RequireAuth: true,
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("GenerateCreateHandler failed: %v", err)
	}

	code := string(result)

	// The INSERT result must be discarded with the _, err := pattern
	if !strings.Contains(code, "_, err := runner.CreatePost(ctx") {
		t.Error("Create handler must discard INSERT result with '_, err := runner.CreatePost(ctx' pattern")
	}

	// The handler must NOT reference CreatePostResult.Id anywhere
	if strings.Contains(code, "CreatePostResult") {
		t.Error("Create handler must NOT reference CreatePostResult — the INSERT result should be discarded")
	}
}

func TestGenerateListHandler_ExcludesAuthorAccountID(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateListHandler(cfg, nil)
	if err != nil {
		t.Fatalf("GenerateListHandler failed: %v", err)
	}

	code := string(result)

	// List item struct should NOT have AuthorAccountId
	if strings.Contains(code, "AuthorAccountId") {
		t.Error("List handler should NOT contain AuthorAccountId field")
	}
}

func TestGenerateCreateHandler_SetsAuthorAccountId(t *testing.T) {
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "body", Type: ddl.TextType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		RequireAuth: true,
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("GenerateCreateHandler failed: %v", err)
	}

	code := string(result)

	// Should extract account ID from session context
	if !strings.Contains(code, "httputil.SessionAccountIDFromContext(ctx)") {
		t.Error("expected httputil.SessionAccountIDFromContext(ctx) call")
	}

	// Should set AuthorAccountId in Create params
	if !strings.Contains(code, "AuthorAccountId: accountID") {
		t.Error("expected AuthorAccountId: accountID in create params")
	}
}

func TestGenerateCreateHandler_ImportsHttputil_WhenAuthor(t *testing.T) {
	// When a table has author_account_id but NO scope column, httputil should
	// still be imported because we need SessionAccountIDFromContext.
	table := ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "title", Type: ddl.StringType},
			{Name: "author_account_id", Type: ddl.BigintType, References: "accounts"},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
			{Name: "deleted_at", Type: ddl.DatetimeType, Nullable: true},
		},
	}

	cfg := HandlerGenConfig{
		ModulePath:  "myapp",
		TableName:   "posts",
		Table:       table,
		Schema:      map[string]ddl.Table{"posts": table},
		ScopeColumn: "", // No scope column
		RequireAuth: true,
	}

	result, err := GenerateCreateHandler(cfg, nil)
	if err != nil {
		t.Fatalf("GenerateCreateHandler failed: %v", err)
	}

	code := string(result)

	// httputil should be imported even without a scope column
	if !strings.Contains(code, `"myapp/shipq/lib/httputil"`) {
		t.Error("expected httputil import when table has author_account_id, even without scope column")
	}
}
