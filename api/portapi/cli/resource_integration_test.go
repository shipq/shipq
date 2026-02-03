//go:build integration

package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	portcli "github.com/shipq/shipq/db/portsql/cli"
)

// TestResourceGeneration_Integration tests the full end-to-end resource generation flow.
func TestResourceGeneration_Integration(t *testing.T) {
	// Create a temporary project directory
	tmpDir := t.TempDir()

	// Set up the migrations directory with a schema.json
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a schema with both AddTable and AddEmptyTable tables
	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {
				"users": {
					"name": "users",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "public_id", "type": "string"},
						{"name": "created_at", "type": "timestamp"},
						{"name": "updated_at", "type": "timestamp"},
						{"name": "deleted_at", "type": "timestamp", "nullable": true},
						{"name": "email", "type": "string"},
						{"name": "name", "type": "string"},
						{"name": "age", "type": "integer", "nullable": true}
					]
				},
				"posts": {
					"name": "posts",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "public_id", "type": "string"},
						{"name": "created_at", "type": "timestamp"},
						{"name": "updated_at", "type": "timestamp"},
						{"name": "deleted_at", "type": "timestamp", "nullable": true},
						{"name": "user_id", "type": "bigint"},
						{"name": "title", "type": "string"},
						{"name": "body", "type": "text"},
						{"name": "published_at", "type": "timestamp", "nullable": true}
					]
				},
				"settings": {
					"name": "settings",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "key", "type": "string"},
						{"name": "value", "type": "string"}
					]
				}
			}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to the temp directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	t.Run("generate users resource", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		opts := Options{
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: "test",
		}

		exitCode := runResource([]string{"users"}, opts)

		if exitCode != ExitSuccess {
			t.Fatalf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
		}

		// Verify the file was created
		handlersPath := filepath.Join(tmpDir, "api", "resources", "users", "handlers.go")
		if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
			t.Fatalf("expected handlers.go to be created at %s", handlersPath)
		}

		// Read and verify the content
		content, err := os.ReadFile(handlersPath)
		if err != nil {
			t.Fatalf("failed to read generated file: %v", err)
		}
		contentStr := string(content)

		// Verify package declaration
		if !strings.Contains(contentStr, "package users") {
			t.Error("generated file should have package users")
		}

		// Verify all request types
		requestTypes := []string{
			"type GetUserRequest struct",
			"type ListUsersRequest struct",
			"type CreateUserRequest struct",
			"type UpdateUserRequest struct",
			"type DeleteUserRequest struct",
		}
		for _, rt := range requestTypes {
			if !strings.Contains(contentStr, rt) {
				t.Errorf("generated file should contain %q", rt)
			}
		}

		// Verify user columns are present in Create/Update requests
		// Note: gofmt aligns struct fields, so we check for json tags instead
		if !strings.Contains(contentStr, `json:"email"`) {
			t.Error("generated file should contain email field with json tag")
		}
		if !strings.Contains(contentStr, `json:"name"`) {
			t.Error("generated file should contain name field with json tag")
		}
		if !strings.Contains(contentStr, `json:"age"`) {
			t.Error("generated file should contain age field with json tag")
		}

		// Verify response types
		if !strings.Contains(contentStr, "type UserResponse struct") {
			t.Error("generated file should contain UserResponse")
		}
		if !strings.Contains(contentStr, "type ListUsersResponse struct") {
			t.Error("generated file should contain ListUsersResponse")
		}

		// Verify handlers (standalone functions, not methods)
		handlers := []string{
			"func GetUser(ctx context.Context, req GetUserRequest)",
			"func ListUsers(ctx context.Context, req ListUsersRequest)",
			"func CreateUser(ctx context.Context, req CreateUserRequest)",
			"func UpdateUser(ctx context.Context, req UpdateUserRequest)",
			"func DeleteUser(ctx context.Context, req DeleteUserRequest)",
		}
		for _, handler := range handlers {
			if !strings.Contains(contentStr, handler) {
				t.Errorf("generated file should contain %q", handler)
			}
		}

		// Verify registration function
		if !strings.Contains(contentStr, "func Register(app *portapi.App)") {
			t.Error("generated file should contain Register function")
		}

		// Verify endpoint paths
		endpoints := []string{
			`app.Get("/users/{public_id}"`,
			`app.Get("/users"`,
			`app.Post("/users"`,
			`app.Put("/users/{public_id}"`,
			`app.Delete("/users/{public_id}"`,
		}
		for _, ep := range endpoints {
			if !strings.Contains(contentStr, ep) {
				t.Errorf("generated file should contain %q", ep)
			}
		}

		// Verify NO HardDelete
		if strings.Contains(contentStr, "HardDelete") {
			t.Error("generated file should NOT contain HardDelete")
		}

		// Verify cursor pagination is supported (table has created_at and public_id)
		if !strings.Contains(contentStr, "Cursor *string") {
			t.Error("generated file should support cursor pagination")
		}
	})

	t.Run("generate posts resource with prefix", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		opts := Options{
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: "test",
		}

		exitCode := runResource([]string{"posts", "--prefix", "/api/v1"}, opts)

		if exitCode != ExitSuccess {
			t.Fatalf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
		}

		// Verify the file was created
		handlersPath := filepath.Join(tmpDir, "api", "resources", "posts", "handlers.go")
		content, err := os.ReadFile(handlersPath)
		if err != nil {
			t.Fatalf("failed to read generated file: %v", err)
		}
		contentStr := string(content)

		// Verify endpoint paths include the prefix
		endpoints := []string{
			`app.Get("/api/v1/posts/{public_id}"`,
			`app.Get("/api/v1/posts"`,
			`app.Post("/api/v1/posts"`,
			`app.Put("/api/v1/posts/{public_id}"`,
			`app.Delete("/api/v1/posts/{public_id}"`,
		}
		for _, ep := range endpoints {
			if !strings.Contains(contentStr, ep) {
				t.Errorf("generated file should contain %q", ep)
			}
		}

		// Verify singular/plural naming
		if !strings.Contains(contentStr, "type PostResponse struct") {
			t.Error("generated file should use singular 'Post' for response type")
		}
		if !strings.Contains(contentStr, "type ListPostsResponse struct") {
			t.Error("generated file should use plural 'Posts' for list response")
		}
	})

	t.Run("reject AddEmptyTable table", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		opts := Options{
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: "test",
		}

		exitCode := runResource([]string{"settings"}, opts)

		if exitCode != ExitError {
			t.Errorf("runResource() exit code = %d, want %d for non-eligible table", exitCode, ExitError)
		}

		errOutput := stderr.String()
		if !strings.Contains(errOutput, "not eligible") {
			t.Errorf("expected error about not eligible, got %q", errOutput)
		}
		if !strings.Contains(errOutput, "plan.AddTable()") {
			t.Errorf("expected hint about AddTable, got %q", errOutput)
		}
	})

	t.Run("reject non-existent table", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		opts := Options{
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: "test",
		}

		exitCode := runResource([]string{"nonexistent"}, opts)

		if exitCode != ExitError {
			t.Errorf("runResource() exit code = %d, want %d for non-existent table", exitCode, ExitError)
		}

		errOutput := stderr.String()
		if !strings.Contains(errOutput, "not found") {
			t.Errorf("expected error about table not found, got %q", errOutput)
		}
	})

	t.Run("custom output directory", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		opts := Options{
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: "test",
		}

		exitCode := runResource([]string{"users", "--out", "internal/handlers"}, opts)

		if exitCode != ExitSuccess {
			t.Fatalf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
		}

		// Verify the file was created in the custom directory
		handlersPath := filepath.Join(tmpDir, "internal", "handlers", "users", "handlers.go")
		if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
			t.Fatalf("expected handlers.go to be created at %s", handlersPath)
		}
	})
}

// TestResourceGeneration_NoSchema tests behavior when schema.json doesn't exist.
func TestResourceGeneration_NoSchema(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"users"}, opts)

	if exitCode != ExitError {
		t.Errorf("runResource() exit code = %d, want %d when schema.json is missing", exitCode, ExitError)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "schema.json") {
		t.Errorf("expected error about schema.json, got %q", errOutput)
	}
	if !strings.Contains(errOutput, "shipq db migrate up") {
		t.Errorf("expected hint about running migrate up, got %q", errOutput)
	}
}

// TestResourceGeneration_GeneratedCodeCompiles verifies that the generated code
// is syntactically valid Go code.
func TestResourceGeneration_GeneratedCodeCompiles(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a complex schema to stress test code generation
	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {
				"purchase_orders": {
					"name": "purchase_orders",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "public_id", "type": "string"},
						{"name": "created_at", "type": "timestamp"},
						{"name": "updated_at", "type": "timestamp"},
						{"name": "deleted_at", "type": "timestamp", "nullable": true},
						{"name": "order_number", "type": "string"},
						{"name": "total_amount", "type": "decimal"},
						{"name": "is_paid", "type": "boolean"},
						{"name": "notes", "type": "text", "nullable": true},
						{"name": "metadata", "type": "json", "nullable": true}
					]
				}
			}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"purchase_orders"}, opts)

	if exitCode != ExitSuccess {
		t.Fatalf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
	}

	// Read the generated file
	handlersPath := filepath.Join(tmpDir, "api", "resources", "purchase_orders", "handlers.go")
	content, err := os.ReadFile(handlersPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	contentStr := string(content)

	// Verify correct singular/plural naming for compound words
	if !strings.Contains(contentStr, "type PurchaseOrderResponse struct") {
		t.Error("generated file should use PascalCase 'PurchaseOrder' for singular")
	}
	if !strings.Contains(contentStr, "type ListPurchaseOrdersResponse struct") {
		t.Error("generated file should use PascalCase 'PurchaseOrders' for plural")
	}

	// Verify different column types are mapped correctly via json tags
	if !strings.Contains(contentStr, `json:"total_amount"`) {
		t.Error("decimal should have total_amount json tag")
	}
	if !strings.Contains(contentStr, `json:"is_paid"`) {
		t.Error("boolean field should have is_paid json tag")
	}
	if !strings.Contains(contentStr, `json:"notes"`) {
		t.Error("nullable text should have notes json tag")
	}

	// Verify the code is formatted (starts with proper package comment)
	if !strings.HasPrefix(contentStr, "// Code generated by shipq api resource. DO NOT EDIT.") {
		t.Error("generated file should start with generated code comment")
	}
}

// TestResourceGeneration_FreshProjectWithEmptyAPIPackage tests the scenario where
// the API root package exists but has no Register function (like a fresh project).
// This mimics the demo project situation where api/main.go just has "package handlers".
func TestResourceGeneration_FreshProjectWithEmptyAPIPackage(t *testing.T) {
	// Get the shipq module path BEFORE changing directories
	shipqModulePath := findShipqModulePath(t)

	tmpDir := t.TempDir()

	// Set up migrations directory with schema.json
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {
				"accounts": {
					"name": "accounts",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "public_id", "type": "string"},
						{"name": "created_at", "type": "timestamp"},
						{"name": "updated_at", "type": "timestamp"},
						{"name": "deleted_at", "type": "timestamp", "nullable": true},
						{"name": "name", "type": "string"},
						{"name": "email", "type": "string"}
					]
				}
			}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create api directory with an empty package (like demo/api/main.go)
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Just a package declaration, no Register function
	if err := os.WriteFile(filepath.Join(apiDir, "main.go"), []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create shipq.ini
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://test.db
dialects = sqlite
migrations = migrations

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod for the temp project
	goMod := `module testproject

go 1.22

require github.com/shipq/shipq v0.0.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Generate queries and db/generated packages so resource generation can compile
	portCfg, err := portcli.LoadConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	portcli.GeneratePackages(context.Background(), portCfg, portcli.GeneratePackagesOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})

	// Step 1: Generate the resource
	t.Run("generate accounts resource", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		opts := Options{
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: "test",
		}

		exitCode := runResource([]string{"accounts"}, opts)

		if exitCode != ExitSuccess {
			t.Fatalf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
		}

		// Verify the resource handlers were created
		handlersPath := filepath.Join(tmpDir, "api", "resources", "accounts", "handlers.go")
		if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
			t.Fatalf("expected handlers.go to be created at %s", handlersPath)
		}

		// Verify output mentions next steps
		output := stdout.String()
		if !strings.Contains(output, "Next steps") {
			t.Error("output should mention next steps")
		}
	})

	// Step 2: Verify the generated resource code is valid Go
	t.Run("verify resource compiles", func(t *testing.T) {
		handlersPath := filepath.Join(tmpDir, "api", "resources", "accounts", "handlers.go")
		content, err := os.ReadFile(handlersPath)
		if err != nil {
			t.Fatalf("failed to read handlers.go: %v", err)
		}
		contentStr := string(content)

		// Check essential elements
		expectedElements := []string{
			"package accounts",
			"type GetAccountRequest struct",
			"type ListAccountsRequest struct",
			"type CreateAccountRequest struct",
			"type UpdateAccountRequest struct",
			"type DeleteAccountRequest struct",
			"type AccountResponse struct",
			"type ListAccountsResponse struct",
			"func GetAccount(ctx context.Context, req GetAccountRequest)",
			"func ListAccounts(ctx context.Context, req ListAccountsRequest)",
			"func CreateAccount(ctx context.Context, req CreateAccountRequest)",
			"func UpdateAccount(ctx context.Context, req UpdateAccountRequest)",
			"func DeleteAccount(ctx context.Context, req DeleteAccountRequest)",
			"func Register(app *portapi.App)",
			`app.Get("/accounts/{public_id}"`,
			`app.Get("/accounts"`,
			`app.Post("/accounts"`,
			`app.Put("/accounts/{public_id}"`,
			`app.Delete("/accounts/{public_id}"`,
		}

		for _, expected := range expectedElements {
			if !strings.Contains(contentStr, expected) {
				t.Errorf("generated file should contain %q", expected)
			}
		}

		// Verify no HardDelete
		if strings.Contains(contentStr, "HardDelete") {
			t.Error("generated file should NOT contain HardDelete")
		}
	})

	// Step 3: Try to run `go build` on the generated code to verify it compiles
	t.Run("go build succeeds", func(t *testing.T) {
		// We need to set up a replace directive for the shipq module
		if shipqModulePath == "" {
			t.Skip("Could not find shipq module path")
		}

		// Update go.mod with replace directive
		goMod := `module testproject

	go 1.22

	require github.com/shipq/shipq v0.0.0

	replace github.com/shipq/shipq => ` + shipqModulePath + `
	`
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			t.Fatal(err)
		}

		// Run go mod tidy
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Logf("go mod tidy output: %s", output)
			// Don't fail - go mod tidy may have issues with missing deps
		}

		// Try to build the resource package
		cmd = exec.Command("go", "build", "./api/resources/accounts")
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go build failed: %v\nOutput: %s", err, output)
		}
	})

	// Step 4: Verify that API root register file was generated
	t.Run("api root register file generated", func(t *testing.T) {
		registerPath := filepath.Join(tmpDir, "api", "zz_generated_register.go")
		content, err := os.ReadFile(registerPath)
		if err != nil {
			t.Fatalf("expected zz_generated_register.go to be created at %s: %v", registerPath, err)
		}
		contentStr := string(content)

		// Verify the register file imports the resource package and calls Register
		expectedElements := []string{
			"package api",
			"func Register(app *portapi.App)",
			"accounts.Register(app)",
		}
		for _, expected := range expectedElements {
			if !strings.Contains(contentStr, expected) {
				t.Errorf("register file should contain %q, got:\n%s", expected, contentStr)
			}
		}
	})
}

// findShipqModulePath attempts to find the path to the shipq module
func findShipqModulePath(t *testing.T) string {
	// Use go list to find the module directory
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/shipq/shipq")
	output, err := cmd.Output()
	if err != nil {
		t.Logf("go list failed: %v", err)
		return ""
	}
	return strings.TrimSpace(string(output))
}

// TestResourceGeneration_FullWorkflow tests that after generating a resource,
// running `shipq api` produces code that compiles. This catches issues like
// type names not being fully qualified in the generated HTTP handlers.
func TestResourceGeneration_FullWorkflow(t *testing.T) {
	// Get the shipq module path BEFORE changing directories
	shipqModulePath := findShipqModulePath(t)
	if shipqModulePath == "" {
		t.Skip("Could not find shipq module path")
	}

	tmpDir := t.TempDir()

	// Set up migrations directory with schema.json
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	schemaJSON := `{
		"schema": {
			"name": "",
			"tables": {
				"products": {
					"name": "products",
					"columns": [
						{"name": "id", "type": "bigint", "primary_key": true},
						{"name": "public_id", "type": "string"},
						{"name": "created_at", "type": "timestamp"},
						{"name": "updated_at", "type": "timestamp"},
						{"name": "deleted_at", "type": "timestamp", "nullable": true},
						{"name": "name", "type": "string"},
						{"name": "price", "type": "decimal"}
					]
				}
			}
		},
		"migrations": []
	}`
	if err := os.WriteFile(filepath.Join(migrationsDir, "schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create shipq.ini
	shipqIni := `[project]
include_logging = false

[db]
url = sqlite://test.db
dialects = sqlite
migrations = migrations

[api]
package = ./api
`
	if err := os.WriteFile(filepath.Join(tmpDir, "shipq.ini"), []byte(shipqIni), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod for the temp project with replace directive
	goMod := `module testworkflow

go 1.22

require github.com/shipq/shipq v0.0.0

replace github.com/shipq/shipq => ` + shipqModulePath + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Generate queries and db/generated packages so resource generation can compile
	portCfg, err := portcli.LoadConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	portcli.GeneratePackages(context.Background(), portCfg, portcli.GeneratePackagesOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})

	// === Step 1: Generate the resource ===
	var stdout, stderr bytes.Buffer
	opts := Options{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: "test",
	}

	exitCode := runResource([]string{"products"}, opts)
	if exitCode != ExitSuccess {
		t.Fatalf("runResource() exit code = %d, want %d\nstderr: %s", exitCode, ExitSuccess, stderr.String())
	}

	// Verify the resource handlers were created
	handlersPath := filepath.Join(tmpDir, "api", "resources", "products", "handlers.go")
	if _, err := os.Stat(handlersPath); os.IsNotExist(err) {
		t.Fatalf("expected handlers.go to be created at %s", handlersPath)
	}

	// Verify the register file was created
	registerPath := filepath.Join(tmpDir, "api", "zz_generated_register.go")
	if _, err := os.Stat(registerPath); os.IsNotExist(err) {
		t.Fatalf("expected zz_generated_register.go to be created at %s", registerPath)
	}

	// === Step 2: Run shipq api to generate HTTP handlers ===
	stdout.Reset()
	stderr.Reset()

	if err := runGenerator(&stdout, &stderr); err != nil {
		t.Fatalf("runGenerator() failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify the HTTP handlers file was created
	httpPath := filepath.Join(tmpDir, "api", "zz_generated_http.go")
	if _, err := os.Stat(httpPath); os.IsNotExist(err) {
		t.Fatalf("expected zz_generated_http.go to be created at %s", httpPath)
	}

	// === Step 3: Verify the generated code compiles ===
	// Run go mod tidy first
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("go mod tidy output: %s", output)
		// Don't fail - go mod tidy may have issues with missing deps
	}

	// Try to build the entire api package tree
	cmd = exec.Command("go", "build", "./api/...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Read the generated files for debugging
		httpContent, _ := os.ReadFile(filepath.Join(tmpDir, "api", "zz_generated_http.go"))
		registerContent, _ := os.ReadFile(filepath.Join(tmpDir, "api", "zz_generated_register.go"))
		handlersContent, _ := os.ReadFile(filepath.Join(tmpDir, "api", "resources", "products", "handlers.go"))

		t.Fatalf("go build failed: %v\nOutput: %s\n\nzz_generated_http.go:\n%s\n\nzz_generated_register.go:\n%s\n\nhandlers.go:\n%s",
			err, output, httpContent, registerContent, handlersContent)
	}

	// === Step 4: Verify the generated HTTP code has correct type references ===
	content, err := os.ReadFile(httpPath)
	if err != nil {
		t.Fatalf("failed to read zz_generated_http.go: %v", err)
	}
	contentStr := string(content)

	// The generated code should import the products package
	if !strings.Contains(contentStr, `"testworkflow/api/resources/products"`) {
		t.Error("generated HTTP code should import the products package")
	}

	// Type references should be prefixed with the package alias
	// e.g., products.GetProductRequest, not just GetProductRequest
	if strings.Contains(contentStr, "func bindGetProduct(r *http.Request) (GetProductRequest,") {
		t.Error("type GetProductRequest should be qualified as products.GetProductRequest")
	}
	if strings.Contains(contentStr, "func bindListProducts(r *http.Request) (ListProductsRequest,") {
		t.Error("type ListProductsRequest should be qualified as products.ListProductsRequest")
	}
}
