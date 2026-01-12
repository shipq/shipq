package migrate

import (
	"encoding/json"
	"testing"

	"github.com/portsql/portsql/src/ddl"
)

func TestPlanJSONRoundtrip(t *testing.T) {
	// Create a plan with some data
	plan := NewPlan()
	plan.Schema.Name = "test_schema"

	// Add a table manually
	plan.Schema.Tables["users"] = ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: "bigint", PrimaryKey: true},
			{Name: "name", Type: "string"},
			{Name: "email", Type: "string", Unique: true},
			{Name: "bio", Type: "text", Nullable: true},
		},
		Indexes: []ddl.IndexDefinition{
			{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
		},
	}

	plan.Migrations = []Migration{
		{
			Name: "create_users_table",
			Instructions: MigrationInstructions{
				Postgres: "CREATE TABLE users (...)",
				MySQL:    "CREATE TABLE users (...)",
				Sqlite:   "CREATE TABLE users (...)",
			},
		},
	}

	// Serialize to JSON
	jsonData, err := plan.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize back
	restored, err := PlanFromJSON(jsonData)
	if err != nil {
		t.Fatalf("PlanFromJSON failed: %v", err)
	}

	// Verify schema name
	if restored.Schema.Name != plan.Schema.Name {
		t.Errorf("schema name mismatch: got %q, want %q", restored.Schema.Name, plan.Schema.Name)
	}

	// Verify tables
	if len(restored.Schema.Tables) != len(plan.Schema.Tables) {
		t.Errorf("table count mismatch: got %d, want %d", len(restored.Schema.Tables), len(plan.Schema.Tables))
	}

	users, ok := restored.Schema.Tables["users"]
	if !ok {
		t.Fatal("users table not found")
	}

	if len(users.Columns) != 4 {
		t.Errorf("column count mismatch: got %d, want 4", len(users.Columns))
	}

	// Verify migrations
	if len(restored.Migrations) != 1 {
		t.Errorf("migration count mismatch: got %d, want 1", len(restored.Migrations))
	}
}

func TestPlanFromJSONInvalid(t *testing.T) {
	_, err := PlanFromJSON([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewPlan(t *testing.T) {
	plan := NewPlan()

	if plan.Schema.Tables == nil {
		t.Error("Schema.Tables should be initialized")
	}

	if plan.Migrations == nil {
		t.Error("Migrations should be initialized")
	}

	if len(plan.Schema.Tables) != 0 {
		t.Error("Schema.Tables should be empty")
	}

	if len(plan.Migrations) != 0 {
		t.Error("Migrations should be empty")
	}
}

func TestPlanJSONFormat(t *testing.T) {
	plan := NewPlan()
	plan.Schema.Name = "test"

	jsonData, err := plan.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify it's indented (has newlines)
	if len(jsonData) < 10 {
		t.Error("JSON output seems too short for indented format")
	}
}
