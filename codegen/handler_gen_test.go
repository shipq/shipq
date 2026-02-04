package codegen

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
		{ddl.IntegerType, "int"},
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
			expected: "int",
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

	// Check imports
	if !strings.Contains(code, `"github.com/shipq/shipq/httperror"`) {
		t.Error("expected httperror import")
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

	// Check import
	if !strings.Contains(code, `"github.com/shipq/shipq/handler"`) {
		t.Error("expected handler import")
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
			name:     "int stays int",
			col:      ddl.ColumnDefinition{Type: ddl.IntegerType},
			expected: "int",
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
