package codegen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// ─── FilterQueryFields tests ───

func TestFilterQueryFields_ReturnsQueryTagged(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Name: "ListPostsRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Limit", Type: "int", JSONName: "limit", Tags: map[string]string{"query": "limit"}},
				{Name: "Title", Type: "string", JSONName: "title"},
			},
		},
	}
	result := codegen.FilterQueryFields(h)
	if len(result) != 1 {
		t.Fatalf("expected 1 query field, got %d", len(result))
	}
	if result[0].Name != "Limit" {
		t.Errorf("expected field name Limit, got %s", result[0].Name)
	}
}

func TestFilterQueryFields_NilRequest(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: nil,
	}
	result := codegen.FilterQueryFields(h)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFilterQueryFields_NoQueryTags(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Name: "CreatePostRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Title", Type: "string", JSONName: "title"},
				{Name: "Body", Type: "string", JSONName: "body"},
			},
		},
	}
	result := codegen.FilterQueryFields(h)
	if len(result) != 0 {
		t.Errorf("expected 0 query fields, got %d", len(result))
	}
}

func TestFilterQueryFields_MixedPathAndQuery(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		PathParams: []codegen.SerializedPathParam{{Name: "id", Position: 1}},
		Request: &codegen.SerializedStructInfo{
			Name: "ListUserPostsRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "ID", Type: "string", JSONName: "id", Tags: map[string]string{"path": "id"}},
				{Name: "Cursor", Type: "*string", JSONName: "cursor", Tags: map[string]string{"query": "cursor"}},
				{Name: "Title", Type: "string", JSONName: "title"},
			},
		},
	}
	result := codegen.FilterQueryFields(h)
	if len(result) != 1 {
		t.Fatalf("expected 1 query field, got %d", len(result))
	}
	if result[0].Name != "Cursor" {
		t.Errorf("expected field name Cursor, got %s", result[0].Name)
	}
}

func TestFilterQueryFields_MultipleQueryTags(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Name: "SearchRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Q", Type: "string", JSONName: "q", Tags: map[string]string{"query": "q"}},
				{Name: "Limit", Type: "int", JSONName: "limit", Tags: map[string]string{"query": "limit"}},
			},
		},
	}
	result := codegen.FilterQueryFields(h)
	if len(result) != 2 {
		t.Fatalf("expected 2 query fields, got %d", len(result))
	}
	if result[0].Name != "Q" {
		t.Errorf("expected first field Q, got %s", result[0].Name)
	}
	if result[1].Name != "Limit" {
		t.Errorf("expected second field Limit, got %s", result[1].Name)
	}
}

// ─── FilterBodyFields tests ───

func TestFilterBodyFields_ExcludesQueryAndPathFields(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		PathParams: []codegen.SerializedPathParam{{Name: "id", Position: 1}},
		Request: &codegen.SerializedStructInfo{
			Name: "UpdatePostRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "ID", Type: "string", JSONName: "id", Tags: map[string]string{"path": "id"}},
				{Name: "Tag", Type: "string", JSONName: "tag", Tags: map[string]string{"query": "tag"}},
				{Name: "Title", Type: "string", JSONName: "title"},
				{Name: "Body", Type: "string", JSONName: "body"},
			},
		},
	}
	result := codegen.FilterBodyFields(h)
	if len(result) != 2 {
		t.Fatalf("expected 2 body fields, got %d", len(result))
	}
	if result[0].Name != "Title" {
		t.Errorf("expected Title, got %s", result[0].Name)
	}
	if result[1].Name != "Body" {
		t.Errorf("expected Body, got %s", result[1].Name)
	}
}

func TestFilterBodyFields_NilRequest(t *testing.T) {
	h := codegen.SerializedHandlerInfo{Request: nil}
	result := codegen.FilterBodyFields(h)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFilterBodyFields_AllBodyFields(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Name: "CreatePostRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Title", Type: "string", JSONName: "title"},
				{Name: "Body", Type: "string", JSONName: "body"},
			},
		},
	}
	result := codegen.FilterBodyFields(h)
	if len(result) != 2 {
		t.Fatalf("expected 2 body fields, got %d", len(result))
	}
}

func TestFilterBodyFields_OnlyQueryFields(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Name: "SearchRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Q", Type: "string", JSONName: "q", Tags: map[string]string{"query": "q"}},
				{Name: "Limit", Type: "int", JSONName: "limit", Tags: map[string]string{"query": "limit"}},
			},
		},
	}
	result := codegen.FilterBodyFields(h)
	if len(result) != 0 {
		t.Errorf("expected 0 body fields, got %d", len(result))
	}
}

func TestFilterBodyFields_EmptyQueryTag(t *testing.T) {
	// A field with query:"" (empty tag value) should NOT be excluded
	h := codegen.SerializedHandlerInfo{
		Request: &codegen.SerializedStructInfo{
			Name: "TestRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Foo", Type: "string", JSONName: "foo", Tags: map[string]string{"query": ""}},
				{Name: "Bar", Type: "string", JSONName: "bar"},
			},
		},
	}
	result := codegen.FilterBodyFields(h)
	if len(result) != 2 {
		t.Errorf("expected 2 body fields (empty query tag should not exclude), got %d", len(result))
	}
}

func TestGetModulePath(t *testing.T) {
	tests := []struct {
		name         string
		goModContent string
		wantModule   string
		wantErr      bool
	}{
		{
			name:         "simple module",
			goModContent: "module example.com/myapp\n\ngo 1.21\n",
			wantModule:   "example.com/myapp",
			wantErr:      false,
		},
		{
			name:         "module with subdirectory",
			goModContent: "module github.com/user/repo/subdir\n\ngo 1.21\n",
			wantModule:   "github.com/user/repo/subdir",
			wantErr:      false,
		},
		{
			name:         "module with extra whitespace",
			goModContent: "  module   example.com/myapp  \n\ngo 1.21\n",
			wantModule:   "example.com/myapp",
			wantErr:      false,
		},
		{
			name:         "no module declaration",
			goModContent: "go 1.21\n\nrequire something v1.0.0\n",
			wantModule:   "",
			wantErr:      true,
		},
		{
			name:         "empty file",
			goModContent: "",
			wantModule:   "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			goModPath := filepath.Join(tmpDir, "go.mod")

			if err := os.WriteFile(goModPath, []byte(tt.goModContent), 0644); err != nil {
				t.Fatalf("failed to write go.mod: %v", err)
			}

			got, err := codegen.GetModulePath(tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModulePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantModule {
				t.Errorf("GetModulePath() = %q, want %q", got, tt.wantModule)
			}
		})
	}
}

func TestGetModulePath_NoGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := codegen.GetModulePath(tmpDir)
	if err == nil {
		t.Error("GetModulePath() expected error when go.mod doesn't exist")
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Test creating a single directory
	newDir := filepath.Join(tmpDir, "newdir")
	if err := codegen.EnsureDir(newDir); err != nil {
		t.Fatalf("EnsureDir() failed: %v", err)
	}

	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("EnsureDir() did not create directory")
	}

	// Test creating nested directories
	nestedDir := filepath.Join(tmpDir, "a", "b", "c", "d")
	if err := codegen.EnsureDir(nestedDir); err != nil {
		t.Fatalf("EnsureDir() failed for nested: %v", err)
	}

	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("EnsureDir() did not create nested directories")
	}

	// Test that calling on existing directory is idempotent
	if err := codegen.EnsureDir(nestedDir); err != nil {
		t.Errorf("EnsureDir() failed on existing directory: %v", err)
	}
}

func TestWriteFileIfChanged(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("creates new file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "new.txt")
		content := []byte("hello world")

		written, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			t.Fatalf("WriteFileIfChanged() error = %v", err)
		}
		if !written {
			t.Error("WriteFileIfChanged() should return true for new file")
		}

		got, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("file content = %q, want %q", string(got), string(content))
		}
	})

	t.Run("skips unchanged file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "unchanged.txt")
		content := []byte("same content")

		// Write initially
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write initial file: %v", err)
		}

		// Try to write same content
		written, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			t.Fatalf("WriteFileIfChanged() error = %v", err)
		}
		if written {
			t.Error("WriteFileIfChanged() should return false for unchanged file")
		}
	})

	t.Run("updates changed file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "changed.txt")
		oldContent := []byte("old content")
		newContent := []byte("new content")

		// Write initially
		if err := os.WriteFile(filePath, oldContent, 0644); err != nil {
			t.Fatalf("failed to write initial file: %v", err)
		}

		// Write new content
		written, err := codegen.WriteFileIfChanged(filePath, newContent)
		if err != nil {
			t.Fatalf("WriteFileIfChanged() error = %v", err)
		}
		if !written {
			t.Error("WriteFileIfChanged() should return true for changed file")
		}

		got, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(got) != string(newContent) {
			t.Errorf("file content = %q, want %q", string(got), string(newContent))
		}
	})
}
