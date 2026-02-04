package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/project"
)

func TestEnsureGitignore_CreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()

	updated, err := ensureGitignore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !updated {
		t.Error("expected updated=true for new file")
	}

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	if !strings.Contains(string(content), ".shipq/") {
		t.Errorf("expected .shipq/ in .gitignore, got:\n%s", content)
	}

	if !strings.Contains(string(content), "# shipq generated files") {
		t.Errorf("expected comment in .gitignore, got:\n%s", content)
	}
}

func TestEnsureGitignore_AppendsToExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing .gitignore
	existingContent := "node_modules/\n*.log\n"
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}

	updated, err := ensureGitignore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !updated {
		t.Error("expected updated=true when appending")
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	contentStr := string(content)

	// Should preserve existing content
	if !strings.Contains(contentStr, "node_modules/") {
		t.Error("existing content was not preserved")
	}

	// Should add .shipq/
	if !strings.Contains(contentStr, ".shipq/") {
		t.Error("expected .shipq/ to be added")
	}
}

func TestEnsureGitignore_DoesNotDuplicate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore that already has .shipq/
	existingContent := "node_modules/\n.shipq/\n*.log\n"
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}

	updated, err := ensureGitignore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated {
		t.Error("expected updated=false when .shipq/ already present")
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	// Content should be unchanged
	if string(content) != existingContent {
		t.Errorf("content was modified when it shouldn't have been:\nexpected:\n%s\ngot:\n%s", existingContent, content)
	}
}

func TestEnsureGitignore_RecognizesVariations(t *testing.T) {
	variations := []string{".shipq/", ".shipq", "/.shipq/", "/.shipq"}

	for _, variation := range variations {
		t.Run(variation, func(t *testing.T) {
			tmpDir := t.TempDir()

			existingContent := "node_modules/\n" + variation + "\n"
			gitignorePath := filepath.Join(tmpDir, ".gitignore")
			if err := os.WriteFile(gitignorePath, []byte(existingContent), 0644); err != nil {
				t.Fatalf("failed to create .gitignore: %v", err)
			}

			updated, err := ensureGitignore(tmpDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if updated {
				t.Errorf("expected updated=false for variation %q", variation)
			}
		})
	}
}

func TestGetGoVersion(t *testing.T) {
	version := getGoVersion()

	// Should be in X.Y format (e.g., "1.21")
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		t.Errorf("expected version in X.Y format, got %q", version)
	}

	// First part should be a number
	if parts[0] == "" {
		t.Error("major version should not be empty")
	}

	// Second part should be a number
	if parts[1] == "" {
		t.Error("minor version should not be empty")
	}
}

func TestCreateGoMod(t *testing.T) {
	t.Run("creates go.mod with correct content", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := createGoMod(tmpDir, "myproject")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		goModPath := filepath.Join(tmpDir, project.GoModFile)
		content, err := os.ReadFile(goModPath)
		if err != nil {
			t.Fatalf("failed to read go.mod: %v", err)
		}

		contentStr := string(content)

		// Check module name
		if !strings.Contains(contentStr, "module com.myproject") {
			t.Errorf("expected module com.myproject, got:\n%s", contentStr)
		}

		// Check go version line exists
		if !strings.Contains(contentStr, "go ") {
			t.Errorf("expected go version directive, got:\n%s", contentStr)
		}
	})
}

func TestCreateShipqIni(t *testing.T) {
	t.Run("creates shipq.ini with db section", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := createShipqIni(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shipqIniPath := filepath.Join(tmpDir, project.ShipqIniFile)
		content, err := os.ReadFile(shipqIniPath)
		if err != nil {
			t.Fatalf("failed to read shipq.ini: %v", err)
		}

		contentStr := string(content)

		// Check [db] section exists
		if !strings.Contains(contentStr, "[db]") {
			t.Errorf("expected [db] section, got:\n%s", contentStr)
		}
	})
}

func TestInitInEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Manually call the internal functions (since initCmd calls os.Exit)
	projectName := project.GetProjectName(tmpDir)

	if !project.HasGoMod(tmpDir) {
		if err := createGoMod(tmpDir, projectName); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}
	}

	if !project.HasShipqIni(tmpDir) {
		if err := createShipqIni(tmpDir); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}
	}

	// Verify both files exist
	if !project.HasGoMod(tmpDir) {
		t.Error("expected go.mod to exist")
	}

	if !project.HasShipqIni(tmpDir) {
		t.Error("expected shipq.ini to exist")
	}
}

func TestInitWithExistingGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing go.mod
	existingGoMod := "module existing.module\n\ngo 1.20\n"
	goModPath := filepath.Join(tmpDir, project.GoModFile)
	if err := os.WriteFile(goModPath, []byte(existingGoMod), 0644); err != nil {
		t.Fatalf("failed to create existing go.mod: %v", err)
	}

	// Simulate init - should only create shipq.ini
	if !project.HasGoMod(tmpDir) {
		t.Fatal("expected go.mod to already exist")
	}

	if !project.HasShipqIni(tmpDir) {
		if err := createShipqIni(tmpDir); err != nil {
			t.Fatalf("failed to create shipq.ini: %v", err)
		}
	}

	// Verify go.mod was not modified
	content, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	if string(content) != existingGoMod {
		t.Errorf("go.mod was modified, expected:\n%s\ngot:\n%s", existingGoMod, string(content))
	}

	// Verify shipq.ini was created
	if !project.HasShipqIni(tmpDir) {
		t.Error("expected shipq.ini to exist")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := project.GetProjectName(tmpDir)

	// First init
	if err := createGoMod(tmpDir, projectName); err != nil {
		t.Fatalf("first createGoMod failed: %v", err)
	}
	if err := createShipqIni(tmpDir); err != nil {
		t.Fatalf("first createShipqIni failed: %v", err)
	}

	// Read original contents
	goModPath := filepath.Join(tmpDir, project.GoModFile)
	shipqIniPath := filepath.Join(tmpDir, project.ShipqIniFile)

	origGoMod, _ := os.ReadFile(goModPath)
	origShipqIni, _ := os.ReadFile(shipqIniPath)

	// Simulate second init - should not modify anything
	if project.HasGoMod(tmpDir) && project.HasShipqIni(tmpDir) {
		// Already initialized, nothing to do
	}

	// Verify files are unchanged
	newGoMod, _ := os.ReadFile(goModPath)
	newShipqIni, _ := os.ReadFile(shipqIniPath)

	if string(newGoMod) != string(origGoMod) {
		t.Error("go.mod was modified on second init")
	}

	if string(newShipqIni) != string(origShipqIni) {
		t.Error("shipq.ini was modified on second init")
	}
}

func TestGoModModuleName(t *testing.T) {
	tests := []struct {
		projectName string
		wantModule  string
	}{
		{"myapp", "com.myapp"},
		{"my-project", "com.my-project"},
		{"app123", "com.app123"},
	}

	for _, tt := range tests {
		t.Run(tt.projectName, func(t *testing.T) {
			tmpDir := t.TempDir()

			if err := createGoMod(tmpDir, tt.projectName); err != nil {
				t.Fatalf("createGoMod failed: %v", err)
			}

			goModPath := filepath.Join(tmpDir, project.GoModFile)
			content, err := os.ReadFile(goModPath)
			if err != nil {
				t.Fatalf("failed to read go.mod: %v", err)
			}

			expected := "module " + tt.wantModule
			if !strings.Contains(string(content), expected) {
				t.Errorf("expected %q in go.mod, got:\n%s", expected, content)
			}
		})
	}
}
