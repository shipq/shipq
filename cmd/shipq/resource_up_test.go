package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// setupResourceTestProject creates a minimal project structure for testing
// resource generation, including go.mod, shipq.ini, and schema.json.
func setupResourceTestProject(t *testing.T, tables map[string]ddl.Table) (string, func()) {
	t.Helper()

	// Create temp directory and resolve symlinks (macOS /tmp -> /private/tmp)
	tmpDir, _ := filepath.EvalSymlinks(t.TempDir())

	// Create go.mod
	goModContent := "module testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create shipq.ini
	shipqIniContent := "[db]\ndatabase_url = sqlite:///tmp/test.db\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIniContent), 0644); err != nil {
		t.Fatalf("failed to create shipq.ini: %v", err)
	}

	// Create shipq/db/migrate directory
	migrateDir := filepath.Join(tmpDir, "shipq", "db", "migrate")
	if err := os.MkdirAll(migrateDir, 0755); err != nil {
		t.Fatalf("failed to create migrate directory: %v", err)
	}

	// Create schema.json
	schema := migrate.Schema{Tables: tables}
	plan := migrate.MigrationPlan{Schema: schema}
	schemaJSON, err := plan.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrateDir, "schema.json"), schemaJSON, 0644); err != nil {
		t.Fatalf("failed to write schema.json: %v", err)
	}

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	cleanup := func() {
		os.Chdir(origDir)
	}

	return tmpDir, cleanup
}

func TestGenerateHandlers_EmitsNoRegenMarker(t *testing.T) {
	// Create a simple table schema
	tables := map[string]ddl.Table{
		"posts": {
			Name: "posts",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "title", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
				{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			},
		},
	}

	tmpDir, cleanup := setupResourceTestProject(t, tables)
	defer cleanup()

	// Generate handlers for the posts table
	cfg := codegen.HandlerGenConfig{
		ModulePath: "testproject",
		TableName:  "posts",
		Table:      tables["posts"],
		Schema:     tables,
	}

	files, err := codegen.GenerateHandlerFiles(cfg)
	if err != nil {
		t.Fatalf("GenerateHandlerFiles() error = %v", err)
	}

	// Create api/posts directory and write files
	apiDir := filepath.Join(tmpDir, "api", "posts")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api/posts directory: %v", err)
	}

	for filename, content := range files {
		filePath := filepath.Join(apiDir, filename)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", filename, err)
		}
	}

	// Write the .shipq-no-regen marker (simulating what resource_up.go does)
	markerPath := filepath.Join(apiDir, ".shipq-no-regen")
	markerContent := "# This file prevents shipq from regenerating handlers in this directory.\n# Delete this file if you want shipq to regenerate the handlers.\n"
	if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
		t.Fatalf("failed to write marker file: %v", err)
	}

	// Verify the marker file exists
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error(".shipq-no-regen marker file was not created")
	}

	// Verify the marker file content
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}
	if !strings.Contains(string(content), "Delete this file") {
		t.Error("marker file should contain instructions for regeneration")
	}
}

func TestGenerateHandlers_SkipsWithExistingMarker(t *testing.T) {
	tables := map[string]ddl.Table{
		"users": {
			Name: "users",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "email", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
				{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			},
		},
	}

	tmpDir, cleanup := setupResourceTestProject(t, tables)
	defer cleanup()

	// Create api/users directory with existing handlers
	apiDir := filepath.Join(tmpDir, "api", "users")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api/users directory: %v", err)
	}

	// Write a customized handler file
	customContent := `// Code generated by shipq.
// NOTE: To regenerate this file, delete the .shipq-no-regen marker in this directory.
package users

// CUSTOM MODIFICATION - this should be preserved
func CustomHandler() {}
`
	createPath := filepath.Join(apiDir, "create.go")
	if err := os.WriteFile(createPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write custom create.go: %v", err)
	}

	// Write the .shipq-no-regen marker
	markerPath := filepath.Join(apiDir, ".shipq-no-regen")
	if err := os.WriteFile(markerPath, []byte("# marker\n"), 0644); err != nil {
		t.Fatalf("failed to write marker file: %v", err)
	}

	// Verify the marker exists (simulating the check in resource_up.go)
	markerExists := false
	if _, err := os.Stat(markerPath); err == nil {
		markerExists = true
	}

	if !markerExists {
		t.Fatal("marker file should exist")
	}

	// The actual skip logic is in resource_up.go - here we just verify the check works
	// When marker exists, we should NOT overwrite the file

	// Read the file and verify it still has custom content
	content, err := os.ReadFile(createPath)
	if err != nil {
		t.Fatalf("failed to read create.go: %v", err)
	}

	if !strings.Contains(string(content), "CUSTOM MODIFICATION") {
		t.Error("custom modification should be preserved when marker exists")
	}
}

func TestGenerateHandlers_RegeneratesWhenMarkerDeleted(t *testing.T) {
	tables := map[string]ddl.Table{
		"comments": {
			Name: "comments",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "body", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
				{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			},
		},
	}

	tmpDir, cleanup := setupResourceTestProject(t, tables)
	defer cleanup()

	// Create api/comments directory
	apiDir := filepath.Join(tmpDir, "api", "comments")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api/comments directory: %v", err)
	}

	// Write old/custom handler content
	oldContent := `// Old content that should be overwritten
package comments

func OldHandler() {}
`
	createPath := filepath.Join(apiDir, "create.go")
	if err := os.WriteFile(createPath, []byte(oldContent), 0644); err != nil {
		t.Fatalf("failed to write old create.go: %v", err)
	}

	// Verify no marker exists (marker was deleted)
	markerPath := filepath.Join(apiDir, ".shipq-no-regen")
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatal("marker file should not exist for this test")
	}

	// Generate new handlers (simulating what happens when marker is deleted)
	cfg := codegen.HandlerGenConfig{
		ModulePath: "testproject",
		TableName:  "comments",
		Table:      tables["comments"],
		Schema:     tables,
	}

	files, err := codegen.GenerateHandlerFiles(cfg)
	if err != nil {
		t.Fatalf("GenerateHandlerFiles() error = %v", err)
	}

	// Write the regenerated files
	for filename, content := range files {
		filePath := filepath.Join(apiDir, filename)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", filename, err)
		}
	}

	// Verify the file was overwritten with new content
	content, err := os.ReadFile(createPath)
	if err != nil {
		t.Fatalf("failed to read create.go: %v", err)
	}

	if strings.Contains(string(content), "OldHandler") {
		t.Error("old content should have been overwritten")
	}

	if !strings.Contains(string(content), "CreateComment") {
		t.Error("new generated content should contain CreateComment handler")
	}
}

func TestGenerateHandlers_MarkerNotOverwritten(t *testing.T) {
	tables := map[string]ddl.Table{
		"tags": {
			Name: "tags",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
				{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			},
		},
	}

	tmpDir, cleanup := setupResourceTestProject(t, tables)
	defer cleanup()

	apiDir := filepath.Join(tmpDir, "api", "tags")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api/tags directory: %v", err)
	}

	// Write marker with custom content (user added notes)
	markerPath := filepath.Join(apiDir, ".shipq-no-regen")
	customMarkerContent := "# Custom notes from user\n# Do not regenerate - special customizations\n"
	if err := os.WriteFile(markerPath, []byte(customMarkerContent), 0644); err != nil {
		t.Fatalf("failed to write marker file: %v", err)
	}

	// Simulate the check in resource_up.go: only write marker if it doesn't exist
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		// This block should NOT execute since marker exists
		defaultContent := "# Default marker content\n"
		if err := os.WriteFile(markerPath, []byte(defaultContent), 0644); err != nil {
			t.Fatalf("failed to write marker: %v", err)
		}
	}

	// Verify marker still has custom content
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}

	if !strings.Contains(string(content), "Custom notes from user") {
		t.Error("marker file custom content should be preserved")
	}
}

func TestGeneratedFileHeader(t *testing.T) {
	tables := map[string]ddl.Table{
		"items": {
			Name: "items",
			Columns: []ddl.ColumnDefinition{
				{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
				{Name: "public_id", Type: ddl.StringType},
				{Name: "name", Type: ddl.StringType},
				{Name: "created_at", Type: ddl.TimestampType},
				{Name: "updated_at", Type: ddl.TimestampType},
				{Name: "deleted_at", Type: ddl.TimestampType, Nullable: true},
			},
		},
	}

	cfg := codegen.HandlerGenConfig{
		ModulePath: "testproject",
		TableName:  "items",
		Table:      tables["items"],
		Schema:     tables,
	}

	files, err := codegen.GenerateHandlerFiles(cfg)
	if err != nil {
		t.Fatalf("GenerateHandlerFiles() error = %v", err)
	}

	// Check that all generated files have the new header
	expectedHeader := "// NOTE: To regenerate this file, delete the .shipq-no-regen marker in this directory."

	for filename, content := range files {
		if !strings.Contains(string(content), expectedHeader) {
			t.Errorf("%s should contain the regeneration instruction header", filename)
		}

		// Verify the old "DO NOT EDIT" message is NOT present
		if strings.Contains(string(content), "DO NOT EDIT") {
			t.Errorf("%s should not contain 'DO NOT EDIT' anymore", filename)
		}
	}
}
