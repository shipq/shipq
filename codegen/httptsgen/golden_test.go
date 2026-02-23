package httptsgen

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// makeMultiResourceHandlers returns a handler set spanning two packages (posts, comments)
// with full CRUD, admin, and custom handlers for thorough golden file coverage.
func makeMultiResourceHandlers() []codegen.SerializedHandlerInfo {
	posts := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/posts",
			FuncName:    "CreatePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			Request: &codegen.SerializedStructInfo{
				Name: "CreatePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
					{Name: "Tags", Type: "[]string", JSONName: "tags", Required: false},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreatePostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
					{Name: "Tags", Type: "[]string", JSONName: "tags", Required: false},
					{Name: "CreatedAt", Type: "string", JSONName: "created_at", Required: true},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts/:id",
			FuncName:    "GetPost",
			PackagePath: "myapp/api/posts",
			RequireAuth: false,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
			Request: &codegen.SerializedStructInfo{
				Name: "GetPostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "GetPostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
					{Name: "Tags", Type: "[]string", JSONName: "tags", Required: false},
					{Name: "CreatedAt", Type: "string", JSONName: "created_at", Required: true},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts",
			FuncName:    "ListPosts",
			PackagePath: "myapp/api/posts",
			RequireAuth: false,
			Response: &codegen.SerializedStructInfo{
				Name: "ListPostsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Post", JSONName: "items", Required: true},
					{Name: "NextCursor", Type: "string", JSONName: "next_cursor", Required: false},
				},
			},
		},
		{
			Method:      "PATCH",
			Path:        "/posts/:id",
			FuncName:    "UpdatePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
			Request: &codegen.SerializedStructInfo{
				Name: "UpdatePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
					{Name: "Title", Type: "string", JSONName: "title", Required: false},
					{Name: "Body", Type: "string", JSONName: "body", Required: false},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "UpdatePostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
				},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/posts/:id",
			FuncName:    "SoftDeletePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
		},
		{
			Method:      "GET",
			Path:        "/admin/posts",
			FuncName:    "AdminListPosts",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			Response: &codegen.SerializedStructInfo{
				Name: "AdminListPostsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Post", JSONName: "items", Required: true},
					{Name: "NextCursor", Type: "string", JSONName: "next_cursor", Required: false},
				},
			},
		},
		{
			Method:      "POST",
			Path:        "/posts/:id/publish",
			FuncName:    "PublishPost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
			Request: &codegen.SerializedStructInfo{
				Name: "PublishPostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "PublishPostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "PublishedAt", Type: "string", JSONName: "published_at", Required: true},
				},
			},
		},
	}

	comments := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/comments",
			FuncName:    "CreateComment",
			PackagePath: "myapp/api/comments",
			RequireAuth: true,
			Request: &codegen.SerializedStructInfo{
				Name: "CreateCommentRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "PostID", Type: "string", JSONName: "post_id", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreateCommentResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "PostID", Type: "string", JSONName: "post_id", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
					{Name: "CreatedAt", Type: "string", JSONName: "created_at", Required: true},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/comments",
			FuncName:    "ListComments",
			PackagePath: "myapp/api/comments",
			RequireAuth: false,
			Response: &codegen.SerializedStructInfo{
				Name: "ListCommentsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Comment", JSONName: "items", Required: true},
					{Name: "NextCursor", Type: "string", JSONName: "next_cursor", Required: false},
				},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/comments/:id",
			FuncName:    "SoftDeleteComment",
			PackagePath: "myapp/api/comments",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
		},
	}

	var all []codegen.SerializedHandlerInfo
	all = append(all, posts...)
	all = append(all, comments...)
	return all
}

func runGoldenTest(t *testing.T, name string, generate func() ([]byte, error)) {
	t.Helper()

	output, err := generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden", name)

	if *updateGolden {
		dir := filepath.Dir(goldenPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file %s", goldenPath)
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s (run with -update to create): %v", goldenPath, err)
	}

	if string(output) != string(golden) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nWant:\n%s", goldenPath, string(output), string(golden))
	}
}

func TestGolden_HTTPBaseClient(t *testing.T) {
	handlers := makeMultiResourceHandlers()
	runGoldenTest(t, "shipq-api.ts", func() ([]byte, error) {
		return GenerateHTTPTypeScriptClient(handlers)
	})
}

func TestGolden_ReactHooks(t *testing.T) {
	handlers := makeMultiResourceHandlers()
	runGoldenTest(t, "react-shipq-api.ts", func() ([]byte, error) {
		return GenerateReactHooks(handlers)
	})
}

func TestGolden_SvelteHooks(t *testing.T) {
	handlers := makeMultiResourceHandlers()
	runGoldenTest(t, "svelte-shipq-api.ts", func() ([]byte, error) {
		return GenerateSvelteHooks(handlers)
	})
}
