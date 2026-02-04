package codegen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
)

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
